package main

import (
	"github.com/GAV777/httpmetricalert/internal/model"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSendMetric_Gauge(t *testing.T) {
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
	metric := model.Metrics{
		ID:    "test",
		MType: "gauge",
		Value: floatPtr(42.0),
	}
	err := sendMetric(client, server.URL, metric, nil, "", "")
	if err != nil {
		t.Errorf("sendMetric error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !requestReceived {
		t.Error("Expected request to be sent")
	}
}

func TestSendMetric_Counter(t *testing.T) {
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
	metric := model.Metrics{
		ID:    "test_counter",
		MType: "counter",
		Delta: int64Ptr(5),
	}
	err := sendMetric(client, server.URL, metric, nil, "", "")
	if err != nil {
		t.Errorf("sendMetric error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !requestReceived {
		t.Error("Expected request to be sent")
	}
}

func TestSendBatch(t *testing.T) {
	t.Parallel()

	var batchCalled int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		batchCalled++
		mu.Unlock()

		if r.URL.Path != "/updates/" && r.URL.Path != "/updates" {
			t.Errorf("Expected /updates/ or /updates, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{}
	batch := []model.Metrics{
		{ID: "gauge1", MType: "gauge", Value: floatPtr(1.0)},
		{ID: "counter1", MType: "counter", Delta: int64Ptr(5)},
	}
	err := sendBatch(client, server.URL, batch, nil, "", "")
	if err != nil {
		t.Errorf("sendBatch error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if batchCalled < 1 {
		t.Errorf("Expected at least 1 batch request, got %d", batchCalled)
	}
}

func TestWorkerPool(t *testing.T) {
	client := &http.Client{}
	wp := newWorkerPool(2, client, "http://localhost:9999", "", "")
	wp.Start()

	var executed int
	var mu sync.Mutex
	done := make(chan struct{})

	for i := 0; i < 5; i++ {
		wp.Submit(func() {
			mu.Lock()
			executed++
			mu.Unlock()
		})
	}

	// Даём время на выполнение
	go func() {
		time.Sleep(200 * time.Millisecond)
		close(done)
	}()

	<-done
	wp.Stop()

	mu.Lock()
	if executed != 5 {
		t.Errorf("Expected 5 tasks executed, got %d", executed)
	}
	mu.Unlock()
}

func floatPtr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}
