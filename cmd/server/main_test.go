package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GAV777/httpmetricalert/internal/config"
	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/storage"
)

func TestSetupRouter(t *testing.T) {
	t.Parallel()

	store := storage.NewMemStorage()
	handler := handlers.NewMetricsHandler(store, nil)
	r := setupRouter(handler, nil, &config.Config{})

	// Проверяем что роутер не nil
	if r == nil {
		t.Fatal("setupRouter returned nil")
	}

	// Проверяем маршрут POST /update
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/42.0", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
}

func TestSetupRouter_NotFound(t *testing.T) {
	t.Parallel()

	store := storage.NewMemStorage()
	handler := handlers.NewMetricsHandler(store, nil)
	r := setupRouter(handler, nil, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rec.Code)
	}
}

func TestSetupRouter_DoubleSlashRejected(t *testing.T) {
	t.Parallel()

	store := storage.NewMemStorage()
	handler := handlers.NewMetricsHandler(store, nil)
	r := setupRouter(handler, nil, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "//value/gauge/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for double slash, got %d", rec.Code)
	}
}

func TestNoDoubleSlashes(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := noDoubleSlashes(next)

	// Запрос с двойным слэшем — должен быть отклонён
	req := httptest.NewRequest(http.MethodGet, "//test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for //test, got %d", rec.Code)
	}

	// Запрос без двойного слэша — должен пройти
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 for /test, got %d", rec.Code)
	}
}

func TestSetupAuditNotifier_NoConfig(t *testing.T) {
	t.Parallel()

	// Без конфигурации аудита нотификатор не должен иметь наблюдателей
	notifier := setupAuditNotifier(&config.Config{})
	if notifier.HasObservers() {
		t.Error("Expected no observers when audit is not configured")
	}
}

func TestAccessLog(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Функция accessLog просто логирует — проверяем что не паникует
	accessLog(req, http.StatusOK, 100, 50)
}

func TestSetupRouter_PingEndpoint(t *testing.T) {
	t.Parallel()

	store := storage.NewMemStorage()
	handler := handlers.NewMetricsHandler(store, nil)
	r := setupRouter(handler, nil, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", rec.Body.String())
	}
}

func TestSetupRouter_UpdateBatch(t *testing.T) {
	t.Parallel()

	store := storage.NewMemStorage()
	handler := handlers.NewMetricsHandler(store, nil)
	r := setupRouter(handler, nil, &config.Config{})

	batch := `[{"id":"batch_g","type":"gauge","value":1.0},{"id":"batch_c","type":"counter","delta":5}]`
	req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(batch))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	// Проверяем что метрики сохранились
	if v, ok := store.GetGauge("batch_g"); !ok || v != 1.0 {
		t.Errorf("Expected gauge value 1.0, got %v", v)
	}
	if v, ok := store.GetCounter("batch_c"); !ok || v != 5 {
		t.Errorf("Expected counter value 5, got %v", v)
	}
}

func TestSetupRouter_UpdateJSON(t *testing.T) {
	t.Parallel()

	store := storage.NewMemStorage()
	handler := handlers.NewMetricsHandler(store, nil)
	r := setupRouter(handler, nil, &config.Config{})

	body := `{"id":"json_test","type":"gauge","value":99.9}`
	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	if v, ok := store.GetGauge("json_test"); !ok || v != 99.9 {
		t.Errorf("Expected gauge value 99.9, got %v", v)
	}
}

func TestSetupRouter_GetValueJSON(t *testing.T) {
	t.Parallel()

	store := storage.NewMemStorage()
	store.SetGauge("get_json", 77.7)
	handler := handlers.NewMetricsHandler(store, nil)
	r := setupRouter(handler, nil, &config.Config{})

	body := `{"id":"get_json","type":"gauge"}`
	req := httptest.NewRequest(http.MethodPost, "/value", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "77.7") {
		t.Errorf("Expected body to contain '77.7', got '%s'", rec.Body.String())
	}
}
