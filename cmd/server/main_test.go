package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/storage"
)

func TestUpdateHandler(t *testing.T) {
	t.Parallel()

	// Создаём хранилище и хендлер
	storage := storage.NewMemStorage()
	handler := handlers.NewMetricsHandler(storage)

	req := httptest.NewRequest("POST", "/update/gauge/test_metric/123.45", nil)
	rec := httptest.NewRecorder()

	handler.UpdateHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}
}

func TestGetValueHandler(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	storage.UpdateGauge("test_gauge", 123.45)

	handler := handlers.NewMetricsHandler(storage)

	req := httptest.NewRequest("GET", "/value/gauge/test_gauge", nil)
	rec := httptest.NewRecorder()

	handler.GetValueHandler(rec, req)

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
	storage.UpdateGauge("test_gauge", 123.45)
	storage.UpdateCounter("test_counter", 42)

	handler := handlers.NewMetricsHandler(storage)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ListMetricsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "test_gauge") || !strings.Contains(rec.Body.String(), "test_counter") {
		t.Error("Expected HTML to contain both metrics")
	}
}
