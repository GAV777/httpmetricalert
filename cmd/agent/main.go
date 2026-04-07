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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/pkg/retry"
)

const (
	contentType = "text/plain"
)

var (
	serverAddress  string
	reportInterval int // в секундах
	pollInterval   int // в секундах
	baseURL        string
)

func init() {
	addr := getEnvOrDefault("ADDRESS", "localhost:8080")
	reportStr := getEnvOrDefault("REPORT_INTERVAL", "10")
	pollStr := getEnvOrDefault("POLL_INTERVAL", "2")

	flag.StringVar(&serverAddress, "a", addr, "HTTP server address")
	flag.IntVar(&reportInterval, "r", parseIntOrPanic(reportStr, "REPORT_INTERVAL"), "Report interval in seconds")
	flag.IntVar(&pollInterval, "p", parseIntOrPanic(pollStr, "POLL_INTERVAL"), "Poll interval in seconds")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func parseIntOrPanic(s, context string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	} else {
		log.Fatalf("Invalid value for %s: %s (must be integer)", context, s)
		panic("unreachable")
	}
}

type Metrics struct {
	Gauge   map[string]float64
	Counter map[string]int64
	Client  *http.Client
	mu      sync.RWMutex
}

func NewMetrics() *Metrics {
	return &Metrics{
		Gauge:   make(map[string]float64),
		Counter: make(map[string]int64),
		Client:  &http.Client{},
	}
}

func (m *Metrics) Collect() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Gauge["Alloc"] = float64(memStats.Alloc)
	m.Gauge["BuckHashSys"] = float64(memStats.BuckHashSys)
	m.Gauge["Frees"] = float64(memStats.Frees)
	m.Gauge["GCCPUFraction"] = memStats.GCCPUFraction
	m.Gauge["GCSys"] = float64(memStats.GCSys)
	m.Gauge["HeapAlloc"] = float64(memStats.HeapAlloc)
	m.Gauge["HeapIdle"] = float64(memStats.HeapIdle)
	m.Gauge["HeapInuse"] = float64(memStats.HeapInuse)
	m.Gauge["HeapObjects"] = float64(memStats.HeapObjects)
	m.Gauge["HeapReleased"] = float64(memStats.HeapReleased)
	m.Gauge["HeapSys"] = float64(memStats.HeapSys)
	m.Gauge["LastGC"] = float64(memStats.LastGC)
	m.Gauge["Lookups"] = float64(memStats.Lookups)
	m.Gauge["MCacheInuse"] = float64(memStats.MCacheInuse)
	m.Gauge["MCacheSys"] = float64(memStats.MCacheSys)
	m.Gauge["MSpanInuse"] = float64(memStats.MSpanInuse)
	m.Gauge["MSpanSys"] = float64(memStats.MSpanSys)
	m.Gauge["Mallocs"] = float64(memStats.Mallocs)
	m.Gauge["NextGC"] = float64(memStats.NextGC)
	m.Gauge["NumForcedGC"] = float64(memStats.NumForcedGC)
	m.Gauge["NumGC"] = float64(memStats.NumGC)
	m.Gauge["OtherSys"] = float64(memStats.OtherSys)
	m.Gauge["PauseTotalNs"] = float64(memStats.PauseTotalNs)
	m.Gauge["StackInuse"] = float64(memStats.StackInuse)
	m.Gauge["StackSys"] = float64(memStats.StackSys)
	m.Gauge["Sys"] = float64(memStats.Sys)
	m.Gauge["TotalAlloc"] = float64(memStats.TotalAlloc)

	m.Gauge["RandomValue"] = rand.Float64()
	m.Counter["PollCount"]++
}

func (m *Metrics) ReportWithBaseURL(baseURL string) {
	client := m.Client

	m.mu.RLock()
	gauges := make(map[string]float64, len(m.Gauge))
	for k, v := range m.Gauge {
		gauges[k] = v
	}
	counters := make(map[string]int64, len(m.Counter))
	for k, v := range m.Counter {
		counters[k] = v
	}
	m.mu.RUnlock()

	// Формируем батч метрик
	var batch []model.Metrics

	for name, value := range gauges {
		v := value
		batch = append(batch, model.Metrics{
			ID:    name,
			MType: "gauge",
			Value: &v,
		})
	}

	for name, value := range counters {
		v := value
		batch = append(batch, model.Metrics{
			ID:    name,
			MType: "counter",
			Delta: &v,
		})
	}

	// Отправляем только если есть метрики
	if len(batch) > 0 {
		m.sendBatchJSON(client, baseURL, batch)
	}
}

// sendJSON отправляет метрику в формате JSON, сжатую gzip, с повторными попытками
func (m *Metrics) sendJSON(client *http.Client, baseURL string, metric model.Metrics) {
	data, err := json.Marshal(metric)
	if err != nil {
		fmt.Printf("Error marshaling metric %s: %v\n", metric.ID, err)
		return
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		fmt.Printf("Error compressing data for %s: %v\n", metric.ID, err)
		return
	}
	if err := gz.Close(); err != nil {
		fmt.Printf("Error closing gzip writer for %s: %v\n", metric.ID, err)
		return
	}

	url := fmt.Sprintf("%s/update", baseURL)

	cfg := retry.DefaultConfig()
	err = retry.Do(context.Background(), cfg, func() error {
		req, err := http.NewRequest("POST", url, bytes.NewReader(buf.Bytes()))
		if err != nil {
			return err // Non-retriable: request creation failure
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		resp, err := client.Do(req)
		if err != nil {
			return err // Potentially retriable (network error)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server responded %d: %s", resp.StatusCode, string(body))
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Failed to send metric %s after retries: %v\n", metric.ID, err)
	}
}

// sendBatchJSON отправляет батч метрик в формате JSON, сжатый gzip, с повторными попытками
func (m *Metrics) sendBatchJSON(client *http.Client, baseURL string, batch []model.Metrics) {
	data, err := json.Marshal(batch)
	if err != nil {
		fmt.Printf("Error marshaling batch: %v\n", err)
		return
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		fmt.Printf("Error compressing batch: %v\n", err)
		return
	}
	if err := gz.Close(); err != nil {
		fmt.Printf("Error closing gzip writer: %v\n", err)
		return
	}

	url := fmt.Sprintf("%s/updates/", baseURL)

	cfg := retry.DefaultConfig()
	err = retry.Do(context.Background(), cfg, func() error {
		req, err := http.NewRequest("POST", url, bytes.NewReader(buf.Bytes()))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server responded %d: %s", resp.StatusCode, string(body))
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Failed to send batch after retries: %v\n", err)
	} else {
		fmt.Printf("Successfully sent batch with %d metrics\n", len(batch))
	}
}

func main() {
	flag.Parse()

	reportDuration := time.Duration(reportInterval) * time.Second
	pollDuration := time.Duration(pollInterval) * time.Second

	if !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}
	baseURL = serverAddress
	fmt.Printf("Starting agent with server address: %s\n", serverAddress)
	fmt.Printf("Report interval: %v, Poll interval: %v\n", reportDuration, pollDuration)

	metrics := NewMetrics()
	tickerPoll := time.NewTicker(pollDuration)
	tickerReport := time.NewTicker(reportDuration)
	defer tickerPoll.Stop()
	defer tickerReport.Stop()

	metrics.Collect()
	fmt.Println("Initial metrics collected")

	for {
		select {
		case <-tickerPoll.C:
			metrics.Collect()
			fmt.Println("Metrics collected")
		case <-tickerReport.C:
			fmt.Println("Sending metrics to server...")
			metrics.ReportWithBaseURL(serverAddress)
		}
	}
}
func newFloat64(v float64) *float64 {
	return &v
}

func newInt64(v int64) *int64 {
	return &v
}
