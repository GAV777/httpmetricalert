package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/GAV777/httpmetricalert/internal/model"
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
)

const (
	contentType = "text/plain"
)

var (
	serverAddress  string
	reportInterval int // в секундах
	pollInterval   int // в секундах
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
	mu      sync.RWMutex
}

func NewMetrics() *Metrics {
	return &Metrics{
		Gauge:   make(map[string]float64),
		Counter: make(map[string]int64),
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
	client := &http.Client{}

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

	for name, value := range gauges {
		metric := model.Metrics{
			ID:    name,
			MType: "gauge",
			Value: &value,
		}
		m.sendJSON(client, baseURL, metric)
	}
	for name, value := range counters {
		delta := value
		metric := model.Metrics{
			ID:    name,
			MType: "counter",
			Delta: &delta,
		}
		m.sendJSON(client, baseURL, metric)
	}
}

// sendJSON отправляет метрику в формате JSON, сжатую gzip
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
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		fmt.Printf("Error creating request for %s: %v\n", metric.ID, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending metric %s: %v\n", metric.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Failed to send metric %s: %d %s", metric.ID, resp.StatusCode, string(body))
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
