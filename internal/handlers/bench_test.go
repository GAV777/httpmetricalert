package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/internal/storage"
)

// === Бенчмарки handlers ===

func BenchmarkUpdateHandler_Gauge(b *testing.B) {
	s := storage.NewMemStorage()
	h := NewMetricsHandler(s, nil)
	r := setupRouter(h)

	body := strings.NewReader("123.45")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/update/gauge/bench_gauge/123.45", body)
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkUpdateHandler_Counter(b *testing.B) {
	s := storage.NewMemStorage()
	h := NewMetricsHandler(s, nil)
	r := setupRouter(h)

	body := strings.NewReader("42")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/update/counter/bench_counter/42", body)
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkUpdateJSONHandler_Gauge(b *testing.B) {
	s := storage.NewMemStorage()
	h := NewMetricsHandler(s, nil)
	r := setupRouter(h)

	metric := model.Metrics{ID: "bench_gauge", MType: "gauge"}
	val := 123.45
	metric.Value = &val
	bodyBytes, _ := json.Marshal(metric)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkUpdateJSONHandler_Counter(b *testing.B) {
	s := storage.NewMemStorage()
	h := NewMetricsHandler(s, nil)
	r := setupRouter(h)

	metric := model.Metrics{ID: "bench_counter", MType: "counter"}
	delta := int64(42)
	metric.Delta = &delta
	bodyBytes, _ := json.Marshal(metric)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkUpdateBatchHandler_Small(b *testing.B) {
	s := storage.NewMemStorage()
	h := NewMetricsHandler(s, nil)
	r := setupRouter(h)

	batch := makeBatch(10)
	bodyBytes, _ := json.Marshal(batch)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkUpdateBatchHandler_Medium(b *testing.B) {
	s := storage.NewMemStorage()
	h := NewMetricsHandler(s, nil)
	r := setupRouter(h)

	batch := makeBatch(100)
	bodyBytes, _ := json.Marshal(batch)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkUpdateBatchHandler_Large(b *testing.B) {
	s := storage.NewMemStorage()
	h := NewMetricsHandler(s, nil)
	r := setupRouter(h)

	batch := makeBatch(1000)
	bodyBytes, _ := json.Marshal(batch)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkGetValueJSONHandler(b *testing.B) {
	s := storage.NewMemStorage()
	s.SetGauge("bench_gauge", 99.99)
	h := NewMetricsHandler(s, nil)
	r := setupRouter(h)

	req := model.Metrics{ID: "bench_gauge", MType: "gauge"}
	bodyBytes, _ := json.Marshal(req)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkListMetricsHandler(b *testing.B) {
	s := storage.NewMemStorage()
	for i := 0; i < 100; i++ {
		s.SetGauge("gauge_"+string(rune('A'+i%26)), float64(i))
		s.SetCounter("counter_"+string(rune('A'+i%26)), int64(i))
	}
	h := NewMetricsHandler(s, nil)
	r := setupRouter(h)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

// makeBatch создаёт батч из n метрик
func makeBatch(n int) []model.Metrics {
	metrics := make([]model.Metrics, 0, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			v := float64(i)
			metrics = append(metrics, model.Metrics{
				ID:    "bench_gauge",
				MType: model.Gauge,
				Value: &v,
			})
		} else {
			d := int64(i)
			metrics = append(metrics, model.Metrics{
				ID:    "bench_counter",
				MType: model.Counter,
				Delta: &d,
			})
		}
	}
	return metrics
}
