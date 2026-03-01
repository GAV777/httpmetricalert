package main

import (
	"io"
	"net/http"
	"net/http/httptest"
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
	// Мок сервера
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

	// Подменяем адрес сервера на мок
	originalAddress := serverAddress
	serverAddress = server.URL // временно

	client := &http.Client{}
	metrics := NewMetrics()
	metrics.SendMetric(client, "gauge", "test_gauge", 42.5)

	// Возвращаем оригинальный адрес
	serverAddress = originalAddress
}

func TestMetrics_SendMetric_Counter(t *testing.T) {
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

	originalAddress := serverAddress
	serverAddress = server.URL

	client := &http.Client{}
	metrics := NewMetrics()
	metrics.SendMetric(client, "counter", "test_counter", int64(5))

	serverAddress = originalAddress
}

func TestMetrics_Report(t *testing.T) {
	reportCalled := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reportCalled++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	originalAddress := serverAddress
	serverAddress = server.URL
	defer func() { serverAddress = originalAddress }()

	metrics := NewMetrics()
	metrics.Collect()
	metrics.Report()

	// Ожидаем минимум 28 gauge + 1 counter = 29 запросов
	if reportCalled < 29 {
		t.Errorf("Expected at least 29 requests, got %d", reportCalled)
	}
}
