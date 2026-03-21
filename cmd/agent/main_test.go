package main

import (
	"github.com/GAV777/httpmetricalert/internal/model"
	"io"
	"net/http"
	"net/http/httptest"
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
	metrics.ReportWithBaseURL("http://localhost:8080")
	// Отправка через JSON — актуальный способ
	metric := model.Metrics{
		ID:    "test",
		MType: "gauge",
		Value: newFloat64(42.0),
	}
	metrics.sendJSON(client, baseURL, metric)
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

	metrics := NewMetrics()
	metrics.ReportWithBaseURL("http://localhost:8080")
}

func TestMetrics_Report(t *testing.T) {
	t.Parallel()

	var reportCalled int
	var mu sync.Mutex

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
