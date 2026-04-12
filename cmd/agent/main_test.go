package main

import (
	"github.com/GAV777/httpmetricalert/internal/model"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestMetricsStore_SetGauge(t *testing.T) {
	store := NewMetricsStore()
	store.SetGauge("test_gauge", 42.0)

	gauges, _ := store.Snapshot()
	found := false
	for _, m := range gauges {
		if m.ID == "test_gauge" && m.Value != nil && *m.Value == 42.0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected test_gauge with value 42.0 in snapshot")
	}
}

func TestMetricsStore_IncrCounter(t *testing.T) {
	store := NewMetricsStore()
	store.IncrCounter("test_counter", 5)
	store.IncrCounter("test_counter", 3)

	_, counters := store.Snapshot()
	found := false
	for _, m := range counters {
		if m.ID == "test_counter" && m.Delta != nil && *m.Delta == 8 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected test_counter with delta 8 in snapshot")
	}
}

func TestMetricsStore_ConcurrentAccess(t *testing.T) {
	store := NewMetricsStore()
	var wg sync.WaitGroup

	// Множество горутин пишут одновременно
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			store.SetGauge("gauge", float64(i))
		}(i)
		go func(i int) {
			defer wg.Done()
			store.IncrCounter("counter", 1)
		}(i)
	}

	wg.Wait()
	_, counters := store.Snapshot()
	for _, m := range counters {
		if m.ID == "counter" && m.Delta != nil && *m.Delta == 100 {
			return
		}
	}
	t.Error("Expected counter value 100")
}

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
	err := sendMetric(client, server.URL, metric)
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
	err := sendMetric(client, server.URL, metric)
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
	err := sendBatch(client, server.URL, batch)
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
	wp := newWorkerPool(2, client, "http://localhost:9999")
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
