package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	contentType = "text/plain"
)

var (
	serverAddress string
	reportInt     time.Duration
	pollInt       time.Duration
)

func init() {
	// Определяем флаги
	flag.StringVar(&serverAddress, "a", "localhost:8080", "HTTP server address (default: localhost:8080)")
	flag.DurationVar(&reportInt, "r", 10*time.Second, "Report interval (default: 10s)")
	flag.DurationVar(&pollInt, "p", 2*time.Second, "Poll interval (default: 2s)")
}

// Структура для хранения метрик
type Metrics struct {
	Gauge   map[string]float64
	Counter map[string]int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		Gauge:   make(map[string]float64),
		Counter: make(map[string]int64),
	}
}

// Сбор метрик из runtime
func (m *Metrics) Collect() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

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

	// Дополнительные метрики
	m.Gauge["RandomValue"] = rand.Float64()
	m.Counter["PollCount"]++
}

// Отправка одной метрики на сервер
func (m *Metrics) SendMetric(client *http.Client, metricType, name string, value interface{}) {
	var valueStr string
	switch v := value.(type) {
	case int64:
		valueStr = fmt.Sprintf("%d", v)
	case float64:
		valueStr = fmt.Sprintf("%g", v)
	default:
		return
	}

	url := fmt.Sprintf("http://%s/update/%s/%s/%s", serverAddress, metricType, name, valueStr)
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

// Отправка всех метрик
func (m *Metrics) Report() {
	client := &http.Client{}
	for name, value := range m.Gauge {
		m.SendMetric(client, "gauge", name, value)
	}
	for name, value := range m.Counter {
		m.SendMetric(client, "counter", name, value)
	}
}

func main() {
	flag.Parse()

	// Если в -a не указан протокол, добавим http://
	if !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}

	metrics := NewMetrics()
	tickerPoll := time.NewTicker(pollInt)
	tickerReport := time.NewTicker(reportInt)
	defer tickerPoll.Stop()
	defer tickerReport.Stop()

	// Первичный сбор
	metrics.Collect()

	for {
		select {
		case <-tickerPoll.C:
			metrics.Collect()
		case <-tickerReport.C:
			metrics.Report()
		}
	}
}
