package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
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
	"syscall"
	"time"

	"github.com/GAV777/httpmetricalert/internal/agent"
	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/pkg/crypto"
	"github.com/GAV777/httpmetricalert/pkg/hash"
	"github.com/GAV777/httpmetricalert/pkg/retry"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

var (
	serverAddress  string
	reportInterval int // в секундах
	pollInterval   int // в секундах
	secretKey      string
	rateLimit      int    // максимальное кол-во одновременных запросов
	cryptoKeyPath  string // путь к файлу публичного ключа
)

func init() {
	addr := getEnvOrDefault("ADDRESS", "localhost:8080")
	reportStr := getEnvOrDefault("REPORT_INTERVAL", "10")
	pollStr := getEnvOrDefault("POLL_INTERVAL", "2")
	key := getEnvOrDefault("KEY", "")
	rateLimitStr := getEnvOrDefault("RATE_LIMIT", "1")
	cKey := getEnvOrDefault("CRYPTO_KEY", "")

	flag.StringVar(&serverAddress, "a", addr, "HTTP server address")
	flag.IntVar(&reportInterval, "r", parseIntOrPanic(reportStr, "REPORT_INTERVAL"), "Report interval in seconds")
	flag.IntVar(&pollInterval, "p", parseIntOrPanic(pollStr, "POLL_INTERVAL"), "Poll interval in seconds")
	flag.StringVar(&secretKey, "k", key, "Secret key for SHA256 hashing")
	flag.IntVar(&rateLimit, "l", parseIntOrPanic(rateLimitStr, "RATE_LIMIT"), "Max concurrent requests")
	flag.StringVar(&cryptoKeyPath, "crypto-key", cKey, "Path to RSA public key file for request encryption")
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
func sendMetric(client *http.Client, baseURL string, metric model.Metrics, pubKey *rsa.PublicKey) error {
	data, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("marshal metric %s: %w", metric.ID, err)
	}

	var bodyData []byte
	useCrypto := pubKey != nil

	if useCrypto {
		encrypted, err := crypto.Encrypt(pubKey, data)
		if err != nil {
			return fmt.Errorf("encrypt metric %s: %w", metric.ID, err)
		}
		bodyData = encrypted
	} else {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(data); err != nil {
			gz.Close()
			return fmt.Errorf("compress metric %s: %w", metric.ID, err)
		}
		gz.Close()
		bodyData = buf.Bytes()
	}

	url := fmt.Sprintf("%s/update", baseURL)

	cfg := retry.DefaultConfig()
	return retry.Do(context.Background(), cfg, func() error {
		req, err := http.NewRequest("POST", url, bytes.NewReader(bodyData))
		if err != nil {
			return err
		}

		if useCrypto {
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("X-Crypto", "rsa")
		} else {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")
		}
		req.Header.Set("Accept-Encoding", "gzip")

		if secretKey != "" && !useCrypto {
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
func sendBatch(client *http.Client, baseURL string, batch []model.Metrics, pubKey *rsa.PublicKey) error {
	if len(batch) == 0 {
		return nil
	}

	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal batch: %w", err)
	}

	var bodyData []byte
	useCrypto := pubKey != nil

	if useCrypto {
		encrypted, err := crypto.Encrypt(pubKey, data)
		if err != nil {
			return fmt.Errorf("encrypt batch: %w", err)
		}
		bodyData = encrypted
	} else {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(data); err != nil {
			gz.Close()
			return fmt.Errorf("compress batch: %w", err)
		}
		gz.Close()
		bodyData = buf.Bytes()
	}

	url := fmt.Sprintf("%s/updates/", baseURL)

	cfg := retry.DefaultConfig()
	return retry.Do(context.Background(), cfg, func() error {
		req, err := http.NewRequest("POST", url, bytes.NewReader(bodyData))
		if err != nil {
			return err
		}

		if useCrypto {
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("X-Crypto", "rsa")
		} else {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")
		}
		req.Header.Set("Accept-Encoding", "gzip")

		if secretKey != "" && !useCrypto {
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

// sender — отправляет метрики каждые reportInterval
func sender(store *agent.Store, wp *workerPool, interval time.Duration, stopCh <-chan struct{}, pubKey *rsa.PublicKey) {
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
				if err := sendBatch(wp.client, wp.baseURL, batch, pubKey); err != nil {
					fmt.Printf("Failed to send batch: %v\n", err)
				} else {
					fmt.Printf("Successfully sent batch with %d metrics\n", len(batch))
				}
			})
		}
	}
}

// sendFinalMetricses отправляет финальный снимок метрик перед остановкой
func sendFinalMetricses(store *agent.Store, wp *workerPool, pubKey *rsa.PublicKey) {
	gauges, counters := store.Snapshot()

	var batch []model.Metrics
	batch = append(batch, gauges...)
	batch = append(batch, counters...)

	if len(batch) == 0 {
		fmt.Println("No metrics to send on shutdown")
		return
	}

	fmt.Printf("Sending final batch with %d metrics\n", len(batch))
	if err := sendBatch(wp.client, wp.baseURL, batch, pubKey); err != nil {
		fmt.Printf("Failed to send final batch: %v\n", err)
	} else {
		fmt.Printf("Successfully sent final batch with %d metrics\n", len(batch))
	}
}

func main() {
	printBuildInfo()

	flag.Parse()

	reportDuration := time.Duration(reportInterval) * time.Second
	pollDuration := time.Duration(pollInterval) * time.Second

	if !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}

	fmt.Printf("Starting agent with server address: %s\n", serverAddress)
	fmt.Printf("Report interval: %v, Poll interval: %v, Rate limit: %d\n", reportDuration, pollDuration, rateLimit)

	// Загружаем публичный ключ, если указан
	var pubKey *rsa.PublicKey
	if cryptoKeyPath != "" {
		var err error
		pubKey, err = crypto.LoadPublicKey(cryptoKeyPath)
		if err != nil {
			log.Fatalf("Failed to load public key: %v", err)
		}
		fmt.Println("RSA encryption enabled")
	}

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
	go sender(store, wp, reportDuration, stopCollect, pubKey)

	// Ждём сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-sigCh

	fmt.Println("\nShutting down agent...")

	// Останавливаем сборщики — они сделают финальный снимок при выходе из ticker
	close(stopCollect)

	// Даём горутинам завершиться
	time.Sleep(100 * time.Millisecond)

	// Отправляем финальный снимок метрик, накопленных к моменту сигнала
	sendFinalMetricses(store, wp, pubKey)

	// Останавливаем worker pool, дожидаясь pending задач
	wp.Stop()

	fmt.Println("Agent stopped gracefully")
}

func printBuildInfo() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}
