package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
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
	flag.StringVar(&serverAddress, "a", "localhost:8080", "HTTP server address (default: localhost:8080)")
	flag.IntVar(&reportInterval, "r", 10, "Report interval in seconds (default: 10)")
	flag.IntVar(&pollInterval, "p", 2, "Poll interval in seconds (default: 2)")
}

// Metrics хранит метрики с мьютексом для потокобезопасности
type Metrics struct {
	Gauge   map[string]float64
	Counter map[string]int64
	mu      sync.RWMutex // защита чтения/записи
}

func NewMetrics() *Metrics {
	return &Metrics{
		Gauge:   make(map[string]float64),
		Counter: make(map[string]int64),
	}
}

// Collect собирает метрики из runtime — требует блокировки на запись
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

// SendMetricWithClient отправляет одну метрику на указанный baseURL
func (m *Metrics) SendMetricWithClient(client *http.Client, baseURL, metricType, name string, value interface{}) {
	var valueStr string
	switch v := value.(type) {
	case int64:
		valueStr = fmt.Sprintf("%d", v)
	case float64:
		valueStr = fmt.Sprintf("%g", v)
	default:
		return
	}

	url := fmt.Sprintf("%s/update/%s/%s/%s", baseURL, metricType, name, valueStr)

	req, err := http.NewRequest("POST", url, strings.NewReader(valueStr))
	if err != nil {
		fmt.Printf("Error creating request for %s: %v\n", name, err)
		return
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending metric %s: %v\n", name, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error response for %s: %s\n", name, resp.Status)
	}
}

// ReportWithBaseURL отправляет все метрики на указанный сервер
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
		m.SendMetricWithClient(client, baseURL, "gauge", name, value)
	}
	for name, value := range counters {
		m.SendMetricWithClient(client, baseURL, "counter", name, value)
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
