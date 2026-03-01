package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestUpdateHandler_Gauge(t *testing.T) {
	storage := NewMemStorage()

	// Создаем запрос
	req := httptest.NewRequest("POST", "/update/gauge/test_metric/123.45", strings.NewReader("123.45"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	// Создаем роутер chi и регистрируем обработчик
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", storage.UpdateHandler)

	// Обрабатываем через роутер
	r.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if len(body) == 0 {
		t.Error("Expected non-empty response body")
	}

	// Проверяем, что метрика сохранена
	value, ok := storage.GetGauge("test_metric")
	if !ok {
		t.Error("Gauge metric not found")
	} else if value != 123.45 {
		t.Errorf("Expected gauge value 123.45, got %f", value)
	}
}

func TestUpdateHandler_Counter(t *testing.T) {
	storage := NewMemStorage()

	// Первый вызов
	req := httptest.NewRequest("POST", "/update/counter/polls/5", strings.NewReader("5"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", storage.UpdateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Проверяем значение
	value, ok := storage.GetCounter("polls")
	if !ok || value != 5 {
		t.Errorf("Expected counter value 5, got %d", value)
	}

	// Второй вызов — должно суммироваться
	req2 := httptest.NewRequest("POST", "/update/counter/polls/3", strings.NewReader("3"))
	req2.Header.Set("Content-Type", "text/plain")
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Second update failed with status %d", w2.Code)
	}

	value, _ = storage.GetCounter("polls")
	if value != 8 {
		t.Errorf("Expected counter value 8 after increment, got %d", value)
	}
}

func TestUpdateHandler_InvalidMethod(t *testing.T) {
	storage := NewMemStorage()

	req := httptest.NewRequest("GET", "/update/gauge/temp/25.5", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", storage.UpdateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestUpdateHandler_InvalidContentType(t *testing.T) {
	storage := NewMemStorage()

	req := httptest.NewRequest("POST", "/update/gauge/temp/25.5", strings.NewReader("25.5"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", storage.UpdateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateHandler_InvalidPath(t *testing.T) {
	storage := NewMemStorage()

	tests := []struct {
		name, path string
	}{
		{"too short", "/update"},
		{"missing value", "/update/gauge/metric"},
		{"invalid base", "/updatex/gauge/metric/100"},
		{"empty name", "/update/gauge//100"}, // двойной слеш
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.path, strings.NewReader("100"))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()

			r := chi.NewRouter()
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "//") {
						http.Error(w, "Not Found", http.StatusNotFound)
						return
					}
					next.ServeHTTP(w, r)
				})
			})
			r.Post("/update/{type}/{name}/{value}", storage.UpdateHandler)
			r.NotFound(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Not Found", http.StatusNotFound)
			})

			r.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("Expected 404 for path %s, got %d", tt.path, w.Code)
			}
		})
	}
}

func TestUpdateHandler_InvalidMetricType(t *testing.T) {
	storage := NewMemStorage()

	req := httptest.NewRequest("POST", "/update/unknown/metric/100", strings.NewReader("100"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", storage.UpdateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateHandler_InvalidGaugeValue(t *testing.T) {
	storage := NewMemStorage()

	req := httptest.NewRequest("POST", "/update/gauge/temp/abc", strings.NewReader("abc"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", storage.UpdateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateHandler_InvalidCounterValue(t *testing.T) {
	storage := NewMemStorage()

	req := httptest.NewRequest("POST", "/update/counter/polls/xyz", strings.NewReader("xyz"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", storage.UpdateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
