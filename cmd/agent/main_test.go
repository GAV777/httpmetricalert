package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestMetrics_Collect(t *testing.T) {
	metrics := NewMetrics()
	metrics.Collect()

	// Проверяем, что все gauge-метрики собраны
	gaugeNames := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys",
		"HeapAlloc", "HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased",
		"HeapSys", "LastGC", "Lookups", "MCacheInuse", "MCacheSys",
		"MSpanInuse", "MSpanSys", "Mallocs", "NextGC", "NumForcedGC",
		"NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
		"Sys", "TotalAlloc", "RandomValue",
	}

	for _, name := range gaugeNames {
		if _, exists := metrics.Gauge[name]; !exists {
			t.Errorf("Expected gauge metric %q to be collected", name)
		}
	}

	// Проверяем PollCount
	if metrics.Counter["PollCount"] != 1 {
		t.Errorf("Expected PollCount = 1, got %d", metrics.Counter["PollCount"])
	}

	// Собираем ещё раз — счётчик должен увеличиться
	metrics.Collect()
	if metrics.Counter["PollCount"] != 2 {
		t.Errorf("Expected PollCount = 2 after second collect, got %d", metrics.Counter["PollCount"])
	}
}

// SendMetricWithClient отправляет метрику, используя указанный базовый URL
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

// ReportWithBaseURL отправляет метрики на указанный URL
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

func TestMetrics_SendMetric_Gauge(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("Expected Content-Type: text/plain, got %s", r.Header.Get("Content-Type"))
		}
		if r.URL.Path == "/update/gauge/test_gauge/42.5" {
			w.WriteHeader(http.StatusOK)
		} else {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &http.Client{}
	metrics := NewMetrics()
	metrics.SendMetricWithClient(client, server.URL, "gauge", "test_gauge", 42.5)
}

func TestMetrics_SendMetric_Counter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/update/counter/test_counter/5" {
			body, _ := io.ReadAll(r.Body)
			if string(body) != "5" {
				t.Errorf("Expected body '5', got '%s'", string(body))
			}
			w.WriteHeader(http.StatusOK)
		} else {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &http.Client{}
	metrics := NewMetrics()
	metrics.SendMetricWithClient(client, server.URL, "counter", "test_counter", int64(5))
}

func TestMetrics_Report(t *testing.T) {
	t.Parallel()

	reportCalled := 0
	var mu sync.Mutex // защищаем счётчик при параллельных запросах

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reportCalled++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metrics := NewMetrics()
	metrics.Collect()
	metrics.ReportWithBaseURL(server.URL)

	// Ожидаем минимум 28 gauge + 1 counter = 29 запросов
	if reportCalled < 29 {
		t.Errorf("Expected at least 29 requests, got %d", reportCalled)
	}
}
