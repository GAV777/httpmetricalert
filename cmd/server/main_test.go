package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"github.com/go-chi/chi/v5"
)

func TestUpdateHandler(t *testing.T) {
	t.Parallel()

	// Создаём хранилище и хендлер
	storage := storage.NewMemStorage()
	handler := handlers.NewMetricsHandler(storage)

	// Создаём chi-роутер и монтируем маршрут
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler)

	// Создаём запрос
	req := httptest.NewRequest("POST", "/update/gauge/test_gauge/123.45", strings.NewReader("123.45"))
	req.Header.Set("Content-Type", "text/plain")

	rec := httptest.NewRecorder()

	// Обрабатываем через роутер
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetValueHandler(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	storage.SetGauge("test_gauge", 123.45)

	handler := handlers.NewMetricsHandler(storage)

	r := chi.NewRouter()
	r.Get("/value/{type}/{name}", handler.GetValueHandler)

	req := httptest.NewRequest("GET", "/value/gauge/test_gauge", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "123.45" {
		t.Errorf("Expected body '123.45', got '%s'", rec.Body.String())
	}
}

func TestListMetricsHandler(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	storage.SetGauge("test_gauge", 123.45)
	storage.SetCounter("test_counter", 42)

	handler := handlers.NewMetricsHandler(storage)

	r := chi.NewRouter()
	r.Get("/", handler.ListMetricsHandler)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "test_gauge") || !strings.Contains(rec.Body.String(), "test_counter") {
		t.Error("Expected HTML to contain both metrics")
	}
}
