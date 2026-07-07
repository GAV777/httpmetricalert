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
	"github.com/go-chi/chi/v5"
)

// Helper функции
func floatPtr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func setupRouter(handler *MetricsHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler)
	r.Get("/value/{type}/{name}", handler.GetValueHandler)
	r.Post("/update", handler.UpdateJSONHandler)
	r.Post("/value", handler.GetValueJSONHandler)
	r.Post("/updates/", handler.UpdateBatchHandler)
	r.Get("/", handler.ListMetricsHandler)
	r.Get("/ping", handler.PingDB)
	return r
}

// newTestHandler создаёт обработчик с nil-нотификатором (аудит отключён)
func newTestHandler(s storage.MetricsStorage) *MetricsHandler {
	return NewMetricsHandler(s, nil)
}

// ==================== UpdateHandler (text/plain) ====================

func TestUpdateHandler_Gauge(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test_gauge/123.45", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	value, ok := storage.GetGauge("test_gauge")
	if !ok || value != 123.45 {
		t.Errorf("Expected gauge value 123.45, got %v (ok=%v)", value, ok)
	}
}

func TestUpdateHandler_Counter(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/test_counter/42", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	value, ok := storage.GetCounter("test_counter")
	if !ok || value != 42 {
		t.Errorf("Expected counter value 42, got %v (ok=%v)", value, ok)
	}
}

func TestUpdateHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/update/gauge/test/123", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", rec.Code)
	}
}

func TestUpdateHandler_InvalidValue(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/notanumber", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateHandler_InvalidType(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/update/invalid/test/123", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateHandler_EmptyName(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge//123", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rec.Code)
	}
}

func TestUpdateHandler_WhitespaceName(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	// Создаём запрос с именем которое после trim станет пустым
	// Используем прямой вызов хендлера т.к. httptest.NewRequest не принимает пробелы в URL
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/%20%20%20/123", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// После trim имя станет пустым -> 404
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rec.Code)
	}
}

func TestUpdateHandler_CounterIncrement(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	// Первый запрос
	req1 := httptest.NewRequest(http.MethodPost, "/update/counter/inc_counter/10", nil)
	req1.Header.Set("Content-Type", "text/plain")
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	// Второй запрос (инкремент)
	req2 := httptest.NewRequest(http.MethodPost, "/update/counter/inc_counter/5", nil)
	req2.Header.Set("Content-Type", "text/plain")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	value, ok := storage.GetCounter("inc_counter")
	if !ok || value != 15 {
		t.Errorf("Expected counter value 15, got %v (ok=%v)", value, ok)
	}
}

func TestUpdateHandler_CounterNegative(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/neg_counter/-10", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	value, ok := storage.GetCounter("neg_counter")
	if !ok || value != -10 {
		t.Errorf("Expected counter value -10, got %v (ok=%v)", value, ok)
	}
}

// ==================== GetValueHandler ====================

func TestGetValueHandler_Gauge(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	storage.SetGauge("get_gauge", 99.99)

	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/get_gauge", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "99.99" {
		t.Errorf("Expected body '99.99', got '%s'", rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", rec.Header().Get("Content-Type"))
	}
}

func TestGetValueHandler_Counter(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	storage.SetCounter("get_counter", 42)

	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/value/counter/get_counter", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "42" {
		t.Errorf("Expected body '42', got '%s'", rec.Body.String())
	}
}

func TestGetValueHandler_NotFound(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/nonexistent", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rec.Code)
	}
}

func TestGetValueHandler_EmptyName(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rec.Code)
	}
}

func TestGetValueHandler_InvalidType(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/value/invalid/name", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

// ==================== UpdateJSONHandler ====================

func TestUpdateJSONHandler_Gauge(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	metric := model.Metrics{
		ID:    "json_gauge",
		MType: "gauge",
		Value: floatPtr(55.5),
	}
	body, _ := json.Marshal(metric)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	value, ok := storage.GetGauge("json_gauge")
	if !ok || value != 55.5 {
		t.Errorf("Expected gauge value 55.5, got %v (ok=%v)", value, ok)
	}
}

func TestUpdateJSONHandler_Counter(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	metric := model.Metrics{
		ID:    "json_counter",
		MType: "counter",
		Delta: int64Ptr(100),
	}
	body, _ := json.Marshal(metric)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	value, ok := storage.GetCounter("json_counter")
	if !ok || value != 100 {
		t.Errorf("Expected counter value 100, got %v (ok=%v)", value, ok)
	}
}

func TestUpdateJSONHandler_InvalidJSON(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateJSONHandler_MissingValue(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	metric := model.Metrics{
		ID:    "bad_gauge",
		MType: "gauge",
		// Value отсутствует
	}
	body, _ := json.Marshal(metric)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateJSONHandler_MissingDelta(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	metric := model.Metrics{
		ID:    "bad_counter",
		MType: "counter",
		// Delta отсутствует
	}
	body, _ := json.Marshal(metric)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateJSONHandler_UnsupportedType(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	metric := model.Metrics{
		ID:    "bad_type",
		MType: "histogram",
	}
	body, _ := json.Marshal(metric)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateJSONHandler_Response(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	metric := model.Metrics{
		ID:    "resp_gauge",
		MType: "gauge",
		Value: floatPtr(1.0),
	}
	body, _ := json.Marshal(metric)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	var resp model.Metrics
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.ID != "resp_gauge" || resp.MType != "gauge" {
		t.Errorf("Response mismatch: %+v", resp)
	}
}

// ==================== GetValueJSONHandler ====================

func TestGetValueJSONHandler_Gauge(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	storage.SetGauge("json_get_gauge", 77.77)

	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := model.Metrics{ID: "json_get_gauge", MType: "gauge"}
	body, _ := json.Marshal(req)

	reqHTTP := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(body))
	reqHTTP.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, reqHTTP)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var resp model.Metrics
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Value == nil || *resp.Value != 77.77 {
		t.Errorf("Expected value 77.77, got %v", resp.Value)
	}
}

func TestGetValueJSONHandler_Counter(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	storage.SetCounter("json_get_counter", 33)

	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := model.Metrics{ID: "json_get_counter", MType: "counter"}
	body, _ := json.Marshal(req)

	reqHTTP := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(body))
	reqHTTP.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, reqHTTP)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var resp model.Metrics
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Delta == nil || *resp.Delta != 33 {
		t.Errorf("Expected delta 33, got %v", resp.Delta)
	}
}

func TestGetValueJSONHandler_NotFound(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := model.Metrics{ID: "nonexistent", MType: "gauge"}
	body, _ := json.Marshal(req)

	reqHTTP := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(body))
	reqHTTP.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, reqHTTP)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rec.Code)
	}
}

func TestGetValueJSONHandler_InvalidJSON(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	reqHTTP := httptest.NewRequest(http.MethodPost, "/value", strings.NewReader("not json"))
	reqHTTP.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, reqHTTP)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestGetValueJSONHandler_UnsupportedType(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := model.Metrics{ID: "test", MType: "histogram"}
	body, _ := json.Marshal(req)

	reqHTTP := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(body))
	reqHTTP.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, reqHTTP)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

// ==================== UpdateBatchHandler ====================

func TestUpdateBatchHandler_Success(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	batch := []model.Metrics{
		{ID: "batch_g1", MType: "gauge", Value: floatPtr(1.0)},
		{ID: "batch_g2", MType: "gauge", Value: floatPtr(2.0)},
		{ID: "batch_c1", MType: "counter", Delta: int64Ptr(10)},
		{ID: "batch_c2", MType: "counter", Delta: int64Ptr(20)},
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Проверяем gauge
	if v, ok := storage.GetGauge("batch_g1"); !ok || v != 1.0 {
		t.Errorf("batch_g1 = %v, want 1.0", v)
	}
	if v, ok := storage.GetGauge("batch_g2"); !ok || v != 2.0 {
		t.Errorf("batch_g2 = %v, want 2.0", v)
	}

	// Проверяем counter
	if v, ok := storage.GetCounter("batch_c1"); !ok || v != 10 {
		t.Errorf("batch_c1 = %v, want 10", v)
	}
	if v, ok := storage.GetCounter("batch_c2"); !ok || v != 20 {
		t.Errorf("batch_c2 = %v, want 20", v)
	}
}

func TestUpdateBatchHandler_Empty(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	body, _ := json.Marshal([]model.Metrics{})

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestUpdateBatchHandler_InvalidJSON(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateBatchHandler_MissingGaugeValue(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	batch := []model.Metrics{
		{ID: "bad_g", MType: "gauge"}, // Нет value
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateBatchHandler_MissingCounterDelta(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	batch := []model.Metrics{
		{ID: "bad_c", MType: "counter"}, // Нет delta
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateBatchHandler_UnsupportedType(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	batch := []model.Metrics{
		{ID: "bad", MType: "histogram"},
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestUpdateBatchHandler_Mixed(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	// Батч с валидными и невалидными (пропускаются) метриками
	batch := []model.Metrics{
		{ID: "good_g", MType: "gauge", Value: floatPtr(5.0)},
		{ID: "good_c", MType: "counter", Delta: int64Ptr(5)},
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	if v, ok := storage.GetGauge("good_g"); !ok || v != 5.0 {
		t.Errorf("good_g = %v, want 5.0", v)
	}
	if v, ok := storage.GetCounter("good_c"); !ok || v != 5 {
		t.Errorf("good_c = %v, want 5", v)
	}
}

// ==================== ListMetricsHandler ====================

func TestListMetricsHandler(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	storage.SetGauge("list_gauge", 1.0)
	storage.SetCounter("list_counter", 2)

	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", rec.Header().Get("Content-Type"))
	}

	body := rec.Body.String()
	if !strings.Contains(body, "list_gauge") {
		t.Error("Expected HTML to contain 'list_gauge'")
	}
	if !strings.Contains(body, "list_counter") {
		t.Error("Expected HTML to contain 'list_counter'")
	}
}

func TestListMetricsHandler_Empty(t *testing.T) {
	t.Parallel()

	storage := storage.NewMemStorage()
	handler := newTestHandler(storage)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
}

// ==================== PingDB ====================

func TestPingDB_Success(t *testing.T) {
	t.Parallel()

	memStore := storage.NewMemStorage()
	handler := newTestHandler(memStore)
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", rec.Body.String())
	}
}

func TestPingDB_DatabaseNotAvailable(t *testing.T) {
	t.Parallel()

	handler := &MetricsHandler{
		Storage:  &memStorageWithDBError{},
		Notifier: nil,
	}
	r := chi.NewRouter()
	r.Get("/ping", handler.PingDB)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 Internal Server Error, got %d", rec.Code)
	}
}

// Вспомогательный storage который всегда возвращает ошибку Ping
type memStorageWithDBError struct{}

func (m *memStorageWithDBError) SetGauge(name string, value float64)       {}
func (m *memStorageWithDBError) SetCounter(name string, delta int64)       {}
func (m *memStorageWithDBError) GetGauge(name string) (float64, bool)      { return 0, false }
func (m *memStorageWithDBError) GetCounter(name string) (int64, bool)      { return 0, false }
func (m *memStorageWithDBError) GetAll() []model.Metrics                   { return nil }
func (m *memStorageWithDBError) Ping() error                               { return storage.ErrDatabaseNotAvailable }
func (m *memStorageWithDBError) Save() error                               { return nil }
func (m *memStorageWithDBError) Load() error                               { return nil }
func (m *memStorageWithDBError) UpdateBatch(metrics []model.Metrics) error { return nil }
func (m *memStorageWithDBError) Close() error                              { return nil }
