// cmd/server/main.go
package main

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// === Интерфейсы и хранилище ===

// Metric представляет метрику
type Metric struct {
	ID    string
	MType string   // "gauge" или "counter"
	Delta *int64   // для counter
	Value *float64 // для gauge
}

// MetricsStorage определяет интерфейс для работы с хранилищем метрик
type MetricsStorage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAllMetrics() []Metric
}

// MemStorage реализует хранилище метрик в памяти
type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
}

// NewMemStorage создает новое хранилище метрик
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// UpdateGauge обновляет значение gauge метрики
func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.gauges[name] = value
}

// UpdateCounter обновляет значение counter метрики (суммирует с предыдущим)
func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.counters[name] += value
}

// GetGauge возвращает значение gauge метрики
func (s *MemStorage) GetGauge(name string) (float64, bool) {
	value, exists := s.gauges[name]
	return value, exists
}

// GetCounter возвращает значение counter метрики
func (s *MemStorage) GetCounter(name string) (int64, bool) {
	value, exists := s.counters[name]
	return value, exists
}

// GetAllMetrics возвращает список всех метрик
func (s *MemStorage) GetAllMetrics() []Metric {
	var metrics []Metric

	for name, value := range s.gauges {
		metrics = append(metrics, Metric{
			ID:    name,
			MType: "gauge",
			Value: &value,
		})
	}

	for name, value := range s.counters {
		metrics = append(metrics, Metric{
			ID:    name,
			MType: "counter",
			Delta: &value,
		})
	}

	return metrics
}

// === HTTP Хендлеры ===

// UpdateHandler обрабатывает POST /update/{type}/{name}/{value}
func (s *MemStorage) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверка Content-Type
	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	metricType := vars["type"]
	name := vars["name"]
	valueStr := vars["value"]

	// 🔥 Критично: имя метрики не должно быть пустым
	if name == "" {
		http.Error(w, "Metric name is required", http.StatusNotFound)
		return
	}

	switch metricType {
	case "gauge":
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			http.Error(w, "Invalid gauge value", http.StatusBadRequest)
			return
		}
		s.UpdateGauge(name, value)

	case "counter":
		value, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid counter value", http.StatusBadRequest)
			return
		}
		s.UpdateCounter(name, value)

	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// GetValueHandler обрабатывает GET /value/{type}/{name}
func (s *MemStorage) GetValueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	metricType := vars["type"]
	name := vars["name"]

	switch metricType {
	case "gauge":
		value, ok := s.GetGauge(name)
		if !ok {
			http.Error(w, "Metric not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strconv.FormatFloat(value, 'f', -1, 64)))

	case "counter":
		value, ok := s.GetCounter(name)
		if !ok {
			http.Error(w, "Metric not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strconv.FormatInt(value, 10)))

	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}
}

// ListMetricsHandler обрабатывает GET /
func (s *MemStorage) ListMetricsHandler(w http.ResponseWriter, r *http.Request) {
	metrics := s.GetAllMetrics()

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
<td style="padding:8px">{{.ID}}</td>
<td style="padding:8px">{{.MType}}</td>
<td style="padding:8px; text-align:right">
{{if .Value}}{{printf "%.6f" .Value}}{{end}}
{{if .Delta}}{{.Delta}}{{end}}
</td>
</tr>
{{end}}
</table>
</body>
</html>
`

	t := template.Must(template.New("metrics").Parse(tmpl))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.Execute(w, metrics)
}

// === Основная функция ===

func main() {
	storage := NewMemStorage()
	r := mux.NewRouter()

	// Регистрация маршрутов
	r.HandleFunc("/update/{type}/{name}/{value}", storage.UpdateHandler).Methods("POST")
	r.HandleFunc("/value/{type}/{name}", storage.GetValueHandler).Methods("GET")
	r.HandleFunc("/", storage.ListMetricsHandler).Methods("GET")

	// 🔁 ВАЖНО: НЕ ДОБАВЛЯЕМ PathPrefix для /update/ — это ломает 404!

	addr := "localhost:8080"
	log.Printf("Starting server on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
