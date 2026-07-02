package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GAV777/httpmetricalert/internal/agent"
	"github.com/GAV777/httpmetricalert/internal/model"
)

func TestRuntimeCollector(t *testing.T) {
	store := agent.NewStore()
	stopCh := make(chan struct{})

	// Запускаем коллектор на короткое время
	go runtimeCollector(store, 100*time.Millisecond, stopCh)

	// Ждём сбора метрик
	time.Sleep(200 * time.Millisecond)

	// Останавливаем
	close(stopCh)

	// Проверяем, что метрики собраны
	gauges, counters := store.Snapshot()

	if len(gauges) == 0 {
		t.Error("expected some gauge metrics to be collected")
	}

	// Проверяем PollCount
	pollCountFound := false
	for _, c := range counters {
		if c.ID == "PollCount" {
			pollCountFound = true
			if c.Delta == nil || *c.Delta == 0 {
				t.Error("expected PollCount to be incremented")
			}
		}
	}

	if !pollCountFound {
		t.Error("expected PollCount metric")
	}
}

func TestSender(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := agent.NewStore()
	store.SetGauge("TestMetric", 100.0)

	client := &http.Client{}
	wp := newWorkerPool(1, client, server.URL)
	wp.Start()
	defer wp.Stop()

	stopCh := make(chan struct{})

	// Запускаем отправитель
	go sender(store, wp, 100*time.Millisecond, stopCh, nil)

	// Ждём отправки
	time.Sleep(250 * time.Millisecond)

	// Останавливаем
	close(stopCh)

	if requestCount == 0 {
		t.Error("expected at least one request to be sent")
	}
}

func TestSendMetricWithGZIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Errorf("expected gzip encoding, got %s", r.Header.Get("Content-Encoding"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metric := model.Metrics{
		ID:    "TestMetric",
		MType: "gauge",
		Value: floatPtr(42.0),
	}

	err := sendMetric(server.Client(), server.URL, metric, nil)
	if err != nil {
		t.Errorf("sendMetric failed: %v", err)
	}
}

func TestSendBatchWithGZIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Errorf("expected gzip encoding, got %s", r.Header.Get("Content-Encoding"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	batch := []model.Metrics{
		{ID: "Metric1", MType: "gauge", Value: floatPtr(1.0)},
		{ID: "Metric2", MType: "counter", Delta: int64Ptr(5)},
	}

	err := sendBatch(server.Client(), server.URL, batch, nil)
	if err != nil {
		t.Errorf("sendBatch failed: %v", err)
	}
}
