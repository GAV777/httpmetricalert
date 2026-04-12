package main

import (
	"github.com/GAV777/httpmetricalert/internal/model"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
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

	var requestReceived bool
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestReceived = true
		mu.Unlock()

		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/update" {
			t.Errorf("Expected URL path /update, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{}
	metrics := NewMetrics()

	// Отправка через JSON — актуальный способ
	metric := model.Metrics{
		ID:    "test",
		MType: "gauge",
		Value: newFloat64(42.0),
	}
	metrics.sendJSON(client, server.URL, metric)

	// Даём время на отправку
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !requestReceived {
		t.Error("Expected request to be sent")
	}
}

func TestMetrics_SendMetric_Counter(t *testing.T) {
	t.Parallel()

	var requestReceived bool
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestReceived = true
		mu.Unlock()

		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/update" {
			t.Errorf("Expected URL path /update, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metrics := NewMetrics()
	metrics.Collect()

	// Отправка counter через JSON
	metric := model.Metrics{
		ID:    "test_counter",
		MType: "counter",
		Delta: newInt64(5),
	}
	metrics.sendJSON(metrics.Client, server.URL, metric)

	// Даём время на отправку
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !requestReceived {
		t.Error("Expected request to be sent")
	}
}

func TestMetrics_Report(t *testing.T) {
	t.Parallel()

	var batchCalled int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		batchCalled++
		mu.Unlock()

		// Проверяем, что это пакетный эндпоинт
		if r.URL.Path != "/updates/" && r.URL.Path != "/updates" {
			t.Errorf("Expected /updates/ or /updates, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metrics := NewMetrics()
	metrics.Collect()
	metrics.ReportWithBaseURL(server.URL)

	// Ожидаем 1 запрос с батчем (вместо 29 отдельных)
	if batchCalled < 1 {
		t.Errorf("Expected at least 1 batch request, got %d", batchCalled)
	}
}
