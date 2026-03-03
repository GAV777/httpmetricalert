package storage

import "sync"

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
	mu       sync.RWMutex
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
}

// UpdateCounter обновляет значение counter метрики (суммирует с предыдущим)
func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += value
}

// GetGauge возвращает значение gauge метрики
func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.gauges[name]
	return value, exists
}

// GetCounter возвращает значение counter метрики
func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.counters[name]
	return value, exists
}

// GetAllMetrics возвращает список всех метрик
func (s *MemStorage) GetAllMetrics() []Metric {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var metrics []Metric

	for name, value := range s.gauges {
		valueCopy := value
		metrics = append(metrics, Metric{
			ID:    name,
			MType: "gauge",
			Value: &valueCopy,
		})
	}

	for name, value := range s.counters {
		valueCopy := value
		metrics = append(metrics, Metric{
			ID:    name,
			MType: "counter",
			Delta: &valueCopy,
		})
	}

	return metrics
}
