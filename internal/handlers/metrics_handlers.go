package handlers

import (
	"encoding/json"
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
	Storage storage.MetricsStorage
	Tmpl    *template.Template
}

// NewMetricsHandler создаёт новый обработчик с зависимостями
func NewMetricsHandler(storage storage.MetricsStorage) *MetricsHandler {
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
		Storage: storage,
		Tmpl:    parsedTmpl,
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
		h.Storage.UpdateGauge(name, value)

	case "counter":
		value, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid value", http.StatusBadRequest)
			return
		}
		h.Storage.UpdateCounter(name, value)

	default:
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
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
	metrics := h.Storage.GetAllMetrics()

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
		h.Storage.UpdateGauge(metric.ID, *metric.Value)

	case "counter":
		if metric.Delta == nil {
			http.Error(w, "Missing delta for counter", http.StatusBadRequest)
			return
		}
		h.Storage.UpdateCounter(metric.ID, *metric.Delta)

	default:
		http.Error(w, "Unsupported metric type", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(metric)
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
		_ = json.NewEncoder(w).Encode(resp)

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
		_ = json.NewEncoder(w).Encode(resp)

	default:
		http.Error(w, "Unsupported metric type", http.StatusBadRequest)
		return
	}
}
