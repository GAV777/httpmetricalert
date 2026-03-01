package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateHandler_Gauge(t *testing.T) {
	// Создаем тестовое хранилище
	storage := NewMemStorage()

	req := httptest.NewRequest("POST", "/update/gauge/test_metric/123.45", strings.NewReader("123.45"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	// Вызываем метод напрямую
	storage.UpdateHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if string(body) == "" {
		t.Error("Expected non-empty response body")
	}

	// Проверим, что метрика действительно сохранена
	if val, exists := storage.gauges["test_metric"]; !exists || val != 123.45 {
		t.Errorf("Gauge metric not stored correctly: expected 123.45, got %v", val)
	}
}

func TestUpdateHandler_Counter(t *testing.T) {
	storage := NewMemStorage()

	req := httptest.NewRequest("POST", "/update/counter/polls/5", strings.NewReader("5"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	storage.UpdateHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Проверим, что счётчик увеличился
	if val, exists := storage.counters["polls"]; !exists || val != 5 {
		t.Errorf("Counter metric not stored correctly: expected 5, got %d", val)
	}

	// Повторный вызов — значение должно суммироваться
	req2 := httptest.NewRequest("POST", "/update/counter/polls/3", strings.NewReader("3"))
	req2.Header.Set("Content-Type", "text/plain")
	w2 := httptest.NewRecorder()

	storage.UpdateHandler(w2, req2)

	resp2 := w2.Result()
	resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Second update failed with status %d", resp2.StatusCode)
	}

	if val := storage.counters["polls"]; val != 8 {
		t.Errorf("Counter not incremented correctly: expected 8, got %d", val)
	}
}

func TestUpdateHandler_InvalidMethod(t *testing.T) {
	storage := NewMemStorage()

	req := httptest.NewRequest("GET", "/update/gauge/temp/25.5", nil)
	w := httptest.NewRecorder()

	// Метод ожидает POST
	storage.UpdateHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestUpdateHandler_InvalidContentType(t *testing.T) {
	storage := NewMemStorage()

	req := httptest.NewRequest("POST", "/update/gauge/temp/25.5", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	storage.UpdateHandler(w, req)

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
		{"empty name", "/update/gauge//100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.path, strings.NewReader("100"))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()

			// Передаём запрос в UpdateHandler
			// Но! UpdateHandler использует mux.Vars(r), которые пусты при ручном вызове
			// Поэтому нужно смоделировать маршрутизацию
			// → Обернём в router
			r := mux.NewRouter()
			r.HandleFunc("/update/{type}/{name}/{value}", storage.UpdateHandler).Methods("POST")

			// Serve the request through the router
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

	r := mux.NewRouter()
	r.HandleFunc("/update/{type}/{name}/{value}", storage.UpdateHandler).Methods("POST")
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

	r := mux.NewRouter()
	r.HandleFunc("/update/{type}/{name}/{value}", storage.UpdateHandler).Methods("POST")
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

	r := mux.NewRouter()
	r.HandleFunc("/update/{type}/{name}/{value}", storage.UpdateHandler).Methods("POST")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
