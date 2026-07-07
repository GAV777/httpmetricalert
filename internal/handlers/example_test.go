package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"github.com/go-chi/chi/v5"
)

// newTestRouter создаёт тестовый роутер с хендлерами
func newTestRouter(s storage.MetricsStorage) http.Handler {
	h := handlers.NewMetricsHandler(s, nil)
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.GetValueHandler)
	r.Post("/update", h.UpdateJSONHandler)
	r.Post("/value", h.GetValueJSONHandler)
	r.Post("/updates", h.UpdateBatchHandler)
	r.Get("/", h.ListMetricsHandler)
	r.Get("/ping", h.PingDB)
	return r
}

func Example_updateGauge() {
	s := storage.NewMemStorage()
	router := newTestRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/12345.67", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Printf("Status: %d\n", rec.Code)

	val, _ := s.GetGauge("Alloc")
	fmt.Printf("Alloc = %.2f\n", val)

	// Output:
	// Status: 200
	// Alloc = 12345.67
}

func Example_updateCounter() {
	s := storage.NewMemStorage()
	router := newTestRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/42", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Printf("Status: %d\n", rec.Code)

	val, _ := s.GetCounter("PollCount")
	fmt.Printf("PollCount = %d\n", val)

	// Output:
	// Status: 200
	// PollCount = 42
}

func Example_updateJSON() {
	s := storage.NewMemStorage()
	router := newTestRouter(s)

	metric := model.Metrics{
		ID:    "TotalAlloc",
		MType: model.Gauge,
		Value: ptrFloat64(99999.99),
	}
	body, _ := json.Marshal(metric)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Printf("Status: %d\n", rec.Code)

	val, _ := s.GetGauge("TotalAlloc")
	fmt.Printf("TotalAlloc = %.2f\n", val)

	// Output:
	// Status: 200
	// TotalAlloc = 99999.99
}

func Example_getValueJSON() {
	s := storage.NewMemStorage()
	s.SetGauge("FreeMemory", 4096.5)
	router := newTestRouter(s)

	req := model.Metrics{ID: "FreeMemory", MType: model.Gauge}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, httpReq)

	var resp model.Metrics
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	fmt.Printf("Status: %d\n", rec.Code)
	fmt.Printf("FreeMemory = %.1f\n", *resp.Value)

	// Output:
	// Status: 200
	// FreeMemory = 4096.5
}

func Example_updateBatch() {
	s := storage.NewMemStorage()
	router := newTestRouter(s)

	batch := []model.Metrics{
		{ID: "BuckAlloc", MType: model.Gauge, Value: ptrFloat64(1024.0)},
		{ID: "HeapAlloc", MType: model.Gauge, Value: ptrFloat64(2048.0)},
		{ID: "Mallocs", MType: model.Counter, Delta: ptrInt64(500)},
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Printf("Status: %d\n", rec.Code)

	g1, _ := s.GetGauge("BuckAlloc")
	g2, _ := s.GetGauge("HeapAlloc")
	c1, _ := s.GetCounter("Mallocs")

	fmt.Printf("BuckAlloc = %.0f\n", g1)
	fmt.Printf("HeapAlloc = %.0f\n", g2)
	fmt.Printf("Mallocs = %d\n", c1)

	// Output:
	// Status: 200
	// BuckAlloc = 1024
	// HeapAlloc = 2048
	// Mallocs = 500
}

func Example_pingDB() {
	s := storage.NewMemStorage()
	router := newTestRouter(s)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Printf("Status: %d\n", rec.Code)
	fmt.Printf("Body: %s\n", strings.TrimSpace(rec.Body.String()))

	// Output:
	// Status: 200
	// Body: OK
}

func Example_listMetrics() {
	s := storage.NewMemStorage()
	s.SetGauge("Sys", 100.0)
	s.SetCounter("NumGC", 5)
	router := newTestRouter(s)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Printf("Status: %d\n", rec.Code)
	fmt.Printf("Contains Sys: %v\n", strings.Contains(rec.Body.String(), "Sys"))
	fmt.Printf("Contains NumGC: %v\n", strings.Contains(rec.Body.String(), "NumGC"))

	// Output:
	// Status: 200
	// Contains Sys: true
	// Contains NumGC: true
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }
