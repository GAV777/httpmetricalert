package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// MetricsStorage определяет интерфейс для работы с хранилищем метрик
type MetricsStorage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
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

// MetricsHandler обрабатывает запросы на обновление метрик
type MetricsHandler struct {
	storage MetricsStorage
}

// NewMetricsHandler создает новый обработчик метрик
func NewMetricsHandler(storage MetricsStorage) *MetricsHandler {
	return &MetricsHandler{
		storage: storage,
	}
}

// UpdateHandler обрабатывает POST /update/:type/:name/:value
func (h *MetricsHandler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем Content-Type
	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	// Разбираем путь
	pathParts := strings.Split(r.URL.Path, "/")
	// Ожидаем формат: /update/{type}/{name}/{value}
	if len(pathParts) != 5 || pathParts[1] != "update" {
		http.NotFound(w, r)
		return
	}

	metricType := pathParts[2]
	metricName := pathParts[3]
	metricValue := pathParts[4]

	// Проверяем наличие имени метрики
	if metricName == "" {
		http.Error(w, "Metric name is required", http.StatusNotFound)
		return
	}

	// Обрабатываем в зависимости от типа метрики
	switch metricType {
	case "gauge":
		value, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			http.Error(w, "Invalid gauge value", http.StatusBadRequest)
			return
		}
		h.storage.UpdateGauge(metricName, value)

	case "counter":
		value, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			http.Error(w, "Invalid counter value", http.StatusBadRequest)
			return
		}
		h.storage.UpdateCounter(metricName, value)

	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	// Успешный ответ
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Metric updated successfully")
}

func main() {
	// Создаем хранилище
	storage := NewMemStorage()

	// Создаем обработчик
	handler := NewMetricsHandler(storage)

	// Регистрируем обработчик для всех путей, начинающихся с /update/
	http.HandleFunc("/update/", handler.UpdateHandler)

	// Запускаем сервер
	addr := "localhost:8080"
	fmt.Printf("Starting metrics server on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}
