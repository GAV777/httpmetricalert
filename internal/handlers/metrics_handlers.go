package handlers

import (
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/GAV777/httpmetricalert/internal/audit"
	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"github.com/go-chi/chi/v5"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// MetricsHandler обрабатывает HTTP-запросы, используя MetricsStorage
type MetricsHandler struct {
	Storage  storage.MetricsStorage
	Tmpl     *template.Template
	Notifier auditNotifier
}

// auditNotifier — минимальный интерфейс для инъекции зависимости аудита
type auditNotifier interface {
	Notify(event audit.AuditEvent)
	HasObservers() bool
}

func (h *MetricsHandler) UpdateGauge(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	valueStr := chi.URLParam(r, "value")
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		http.Error(w, "Invalid value", http.StatusBadRequest)
		return
	}
	h.Storage.SetGauge(name, value)
	w.WriteHeader(http.StatusOK)
}

// NewMetricsHandler создаёт новый обработчик с зависимостями
func NewMetricsHandler(store storage.MetricsStorage, notifier auditNotifier) *MetricsHandler {
	tmpl := `
<!DOCTYPE html>
<html>
<head><title>Метрики</title></head>
<body>
<h1>Список метрик</h1>
<table border="1" style="width:100%; border-collapse: collapse;">
<tr style="background:#eee"><th>Имя</th><th>Тип</th><th>Значение</th></tr>
{{range .}}
<tr>
<td>{{.ID}}</td>
<td>{{.MType}}</td>
<td>{{if .Value}}{{printf "%.6f" .Value}}{{end}}{{if .Delta}}{{.Delta}}{{end}}</td>
</tr>
{{end}}
</table>
</body>
</html>
`

	parsedTmpl := template.Must(template.New("metrics").Parse(tmpl))

	return &MetricsHandler{
		Storage:  store,
		Tmpl:     parsedTmpl,
		Notifier: notifier,
	}
}

// UpdateHandler обрабатывает POST /update/{type}/{name}/{value}
func (h *MetricsHandler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "text/plain" {
		http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
		return
	}

	// Извлекаем параметры из URL
	name := chi.URLParam(r, "name")
	valueStr := chi.URLParam(r, "value")
	metricType := chi.URLParam(r, "type")

	name = strings.TrimSpace(name)
	valueStr = strings.TrimSpace(valueStr)

	if name == "" || valueStr == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	switch metricType {
	case "gauge":
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			http.Error(w, "Invalid value", http.StatusBadRequest)
			return
		}
		h.Storage.SetGauge(name, value)

	case "counter":
		value, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid value", http.StatusBadRequest)
			return
		}
		h.Storage.SetCounter(name, value)

	default:
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}

	// Отправляем событие аудита
	if h.Notifier != nil && h.Notifier.HasObservers() {
		h.Notifier.Notify(audit.AuditEvent{
			Timestamp: time.Now().Unix(),
			Metrics:   []string{name},
			IPAddress: getClientIP(r),
		})
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// GetValueHandler обрабатывает GET /value/{type}/{name}
func (h *MetricsHandler) GetValueHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	name = strings.TrimSpace(name)

	if name == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	switch chi.URLParam(r, "type") {
	case "gauge":
		value, ok := h.Storage.GetGauge(name)
		if !ok {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strconv.FormatFloat(value, 'f', -1, 64)))

	case "counter":
		value, ok := h.Storage.GetCounter(name)
		if !ok {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strconv.FormatInt(value, 10)))

	default:
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}
}

// ListMetricsHandler обрабатывает GET /
func (h *MetricsHandler) ListMetricsHandler(w http.ResponseWriter, r *http.Request) {
	metrics := h.Storage.GetAll()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Tmpl.Execute(w, metrics); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// UpdateJSONHandler обрабатывает POST /update в формате JSON
func (h *MetricsHandler) UpdateJSONHandler(w http.ResponseWriter, r *http.Request) {
	var metric model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch metric.MType {
	case "gauge":
		if metric.Value == nil {
			http.Error(w, "Missing value for gauge", http.StatusBadRequest)
			return
		}
		h.Storage.SetGauge(metric.ID, *metric.Value)

	case "counter":
		if metric.Delta == nil {
			http.Error(w, "Missing delta for counter", http.StatusBadRequest)
			return
		}
		h.Storage.SetCounter(metric.ID, *metric.Delta)

	default:
		http.Error(w, "Unsupported metric type", http.StatusBadRequest)
		return
	}

	// Отправляем событие аудита
	if h.Notifier != nil && h.Notifier.HasObservers() {
		h.Notifier.Notify(audit.AuditEvent{
			Timestamp: time.Now().Unix(),
			Metrics:   []string{metric.ID},
			IPAddress: getClientIP(r),
		})
	}

	w.WriteHeader(http.StatusOK)
	_ = writeJSON(w, metric)
}

// GetValueJSONHandler обрабатывает POST /value в формате JSON
func (h *MetricsHandler) GetValueJSONHandler(w http.ResponseWriter, r *http.Request) {
	var req model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch req.MType {
	case "gauge":
		value, ok := h.Storage.GetGauge(req.ID)
		if !ok {
			http.Error(w, "Metric not found", http.StatusNotFound)
			return
		}
		resp := model.Metrics{
			ID:    req.ID,
			MType: "gauge",
			Value: &value,
		}
		_ = writeJSON(w, resp)

	case "counter":
		value, ok := h.Storage.GetCounter(req.ID)
		if !ok {
			http.Error(w, "Metric not found", http.StatusNotFound)
			return
		}
		resp := model.Metrics{
			ID:    req.ID,
			MType: "counter",
			Delta: &value,
		}
		_ = writeJSON(w, resp)

	default:
		http.Error(w, "Unsupported metric type", http.StatusBadRequest)
		return
	}
}

// UpdateBatchHandler обрабатывает POST /updates/ в формате JSON (список метрик)
func (h *MetricsHandler) UpdateBatchHandler(w http.ResponseWriter, r *http.Request) {
	var metrics []model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metrics); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(metrics) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Проверка корректности метрик перед записью
	for _, metric := range metrics {
		switch metric.MType {
		case model.Gauge:
			if metric.Value == nil {
				http.Error(w, "Missing value for gauge: "+metric.ID, http.StatusBadRequest)
				return
			}
		case model.Counter:
			if metric.Delta == nil {
				http.Error(w, "Missing delta for counter: "+metric.ID, http.StatusBadRequest)
				return
			}
		default:
			http.Error(w, "Unsupported metric type: "+metric.MType, http.StatusBadRequest)
			return
		}
	}

	// Используем пакетное обновление
	if err := h.Storage.UpdateBatch(metrics); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Отправляем событие аудита
	if h.Notifier != nil && h.Notifier.HasObservers() {
		names := make([]string, len(metrics))
		for i, m := range metrics {
			names[i] = m.ID
		}
		h.Notifier.Notify(audit.AuditEvent{
			Timestamp: time.Now().Unix(),
			Metrics:   names,
			IPAddress: getClientIP(r),
		})
	}

	w.WriteHeader(http.StatusOK)
}

// MarshalJSONEncoder сериализует значение в JSON и записывает в ResponseWriter
func writeJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// getClientIP извлекает IP-адрес из запроса
func getClientIP(r *http.Request) string {
	// Проверяем X-Forwarded-For — берём только первый IP до запятой
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fallback: RemoteAddr может быть в формате "ip:port"
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
