package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/GAV777/httpmetricalert/internal/agent"
	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/pkg/hash"
	"github.com/GAV777/httpmetricalert/pkg/retry"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	serverAddress  string
	reportInterval int // в секундах
	pollInterval   int // в секундах
	secretKey      string
	rateLimit      int // максимальное кол-во одновременных запросов
)

func init() {
	addr := getEnvOrDefault("ADDRESS", "localhost:8080")
	reportStr := getEnvOrDefault("REPORT_INTERVAL", "10")
	pollStr := getEnvOrDefault("POLL_INTERVAL", "2")
	key := getEnvOrDefault("KEY", "")
	rateLimitStr := getEnvOrDefault("RATE_LIMIT", "1")

	flag.StringVar(&serverAddress, "a", addr, "HTTP server address")
	flag.IntVar(&reportInterval, "r", parseIntOrPanic(reportStr, "REPORT_INTERVAL"), "Report interval in seconds")
	flag.IntVar(&pollInterval, "p", parseIntOrPanic(pollStr, "POLL_INTERVAL"), "Poll interval in seconds")
	flag.StringVar(&secretKey, "k", key, "Secret key for SHA256 hashing")
	flag.IntVar(&rateLimit, "l", parseIntOrPanic(rateLimitStr, "RATE_LIMIT"), "Max concurrent requests")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func parseIntOrPanic(s, context string) int {
	if n, err := strconv.Atoi(s); err == nil {
		if n <= 0 {
			log.Fatalf("%s must be positive, got %d", context, n)
		}
		return n
	}
	log.Fatalf("Invalid value for %s: %s (must be integer)", context, s)
	panic("unreachable")
}

// runtimeCollector собирает метрики runtime
func runtimeCollector(store *agent.Store, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			store.SetGauge("Alloc", float64(m.Alloc))
			store.SetGauge("BuckHashSys", float64(m.BuckHashSys))
			store.SetGauge("Frees", float64(m.Frees))
			store.SetGauge("GCCPUFraction", m.GCCPUFraction)
			store.SetGauge("GCSys", float64(m.GCSys))
			store.SetGauge("HeapAlloc", float64(m.HeapAlloc))
			store.SetGauge("HeapIdle", float64(m.HeapIdle))
			store.SetGauge("HeapInuse", float64(m.HeapInuse))
			store.SetGauge("HeapObjects", float64(m.HeapObjects))
			store.SetGauge("HeapReleased", float64(m.HeapReleased))
			store.SetGauge("HeapSys", float64(m.HeapSys))
			store.SetGauge("LastGC", float64(m.LastGC))
			store.SetGauge("Lookups", float64(m.Lookups))
			store.SetGauge("MCacheInuse", float64(m.MCacheInuse))
			store.SetGauge("MCacheSys", float64(m.MCacheSys))
			store.SetGauge("MSpanInuse", float64(m.MSpanInuse))
			store.SetGauge("MSpanSys", float64(m.MSpanSys))
			store.SetGauge("Mallocs", float64(m.Mallocs))
			store.SetGauge("NextGC", float64(m.NextGC))
			store.SetGauge("NumForcedGC", float64(m.NumForcedGC))
			store.SetGauge("NumGC", float64(m.NumGC))
			store.SetGauge("OtherSys", float64(m.OtherSys))
			store.SetGauge("PauseTotalNs", float64(m.PauseTotalNs))
			store.SetGauge("StackInuse", float64(m.StackInuse))
			store.SetGauge("StackSys", float64(m.StackSys))
			store.SetGauge("Sys", float64(m.Sys))
			store.SetGauge("TotalAlloc", float64(m.TotalAlloc))

			store.SetGauge("RandomValue", rand.Float64())
			store.IncrCounter("PollCount", 1)
		}
	}
}

// gopsutilCollector собирает системные метрики
func gopsutilCollector(store *agent.Store, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			// Память
			v, err := mem.VirtualMemory()
			if err == nil {
				store.SetGauge("TotalMemory", float64(v.Total))
				store.SetGauge("FreeMemory", float64(v.Free))
			}

			// CPU utilization по ядрам
			percentages, err := cpu.Percent(0, true)
			if err == nil {
				for i, p := range percentages {
					name := fmt.Sprintf("CPUutilization%d", i+1)
					store.SetGauge(name, p)
				}
			}
		}
	}
}

// worker — отправляет одну метрику
func sendMetric(client *http.Client, baseURL string, metric model.Metrics) error {
	data, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("marshal metric %s: %w", metric.ID, err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		gz.Close()
		return fmt.Errorf("compress metric %s: %w", metric.ID, err)
	}
	gz.Close()

	url := fmt.Sprintf("%s/update", baseURL)

	cfg := retry.DefaultConfig()
	return retry.Do(context.Background(), cfg, func() error {
		req, err := http.NewRequest("POST", url, bytes.NewReader(buf.Bytes()))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		if secretKey != "" {
			h := hash.Sign(string(data), secretKey)
			req.Header.Set("HashSHA256", h)
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server %d: %s", resp.StatusCode, string(body))
		}
		return nil
	})
}

// sendBatch отправляет батч метрик
func sendBatch(client *http.Client, baseURL string, batch []model.Metrics) error {
	if len(batch) == 0 {
		return nil
	}

	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal batch: %w", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		gz.Close()
		return fmt.Errorf("compress batch: %w", err)
	}
	gz.Close()

	url := fmt.Sprintf("%s/updates/", baseURL)

	cfg := retry.DefaultConfig()
	return retry.Do(context.Background(), cfg, func() error {
		req, err := http.NewRequest("POST", url, bytes.NewReader(buf.Bytes()))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		if secretKey != "" {
			h := hash.Sign(string(data), secretKey)
			req.Header.Set("HashSHA256", h)
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server %d: %s", resp.StatusCode, string(body))
		}
		return nil
	})
}

// workerPool — пул воркеров для отправки метрик
type workerPool struct {
	tasks   chan func()
	client  *http.Client
	baseURL string
	size    int
	wg      sync.WaitGroup
	stopCh  chan struct{}
}

func newWorkerPool(size int, client *http.Client, baseURL string) *workerPool {
	return &workerPool{
		tasks:   make(chan func(), size*2),
		client:  client,
		baseURL: baseURL,
		size:    size,
		stopCh:  make(chan struct{}),
	}
}

func (wp *workerPool) Start() {
	for i := 0; i < wp.size; i++ {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			for {
				select {
				case task := <-wp.tasks:
					task()
				case <-wp.stopCh:
					return
				}
			}
		}()
	}
}

func (wp *workerPool) Submit(task func()) {
	wp.tasks <- task
}

func (wp *workerPool) Stop() {
	close(wp.stopCh)
	wp.wg.Wait()
}

// sender — отправляет метрики каждые reportInterval
func sender(store *agent.Store, wp *workerPool, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			gauges, counters := store.Snapshot()

			// Отправляем батч
			var batch []model.Metrics
			batch = append(batch, gauges...)
			batch = append(batch, counters...)

			if len(batch) == 0 {
				continue
			}

			fmt.Printf("Sending batch with %d metrics\n", len(batch))
			wp.Submit(func() {
				if err := sendBatch(wp.client, wp.baseURL, batch); err != nil {
					fmt.Printf("Failed to send batch: %v\n", err)
				} else {
					fmt.Printf("Successfully sent batch with %d metrics\n", len(batch))
				}
			})
		}
	}
}

func main() {
	flag.Parse()

	reportDuration := time.Duration(reportInterval) * time.Second
	pollDuration := time.Duration(pollInterval) * time.Second

	if !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}

	fmt.Printf("Starting agent with server address: %s\n", serverAddress)
	fmt.Printf("Report interval: %v, Poll interval: %v, Rate limit: %d\n", reportDuration, pollDuration, rateLimit)

	store := agent.NewStore()
	client := &http.Client{}

	// Запускаем worker pool
	wp := newWorkerPool(rateLimit, client, serverAddress)
	wp.Start()

	// Каналы остановки
	stopCollect := make(chan struct{})

	// Запускаем сборщики метрик в отдельных горутинах
	go runtimeCollector(store, pollDuration, stopCollect)
	go gopsutilCollector(store, pollDuration, stopCollect)

	// Запускаем отправителя
	go sender(store, wp, reportDuration, stopCollect)

	// Ждём сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down agent...")
	close(stopCollect)
	wp.Stop()
	fmt.Println("Agent stopped gracefully")
}
