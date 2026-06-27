package agent

import (
	"sync"

	"github.com/GAV777/httpmetricalert/internal/model"
)

// Store — потокобезопасное хранилище метрик агента
type Store struct {
	gauges   map[string]float64
	counters map[string]int64
	mu       sync.RWMutex
}

// NewStore создаёт новое хранилище метрик
func NewStore() *Store {
	return &Store{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// SetGauge устанавливает значение gauge-метрики
func (s *Store) SetGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
}

// IncrCounter инкрементирует counter-метрику
func (s *Store) IncrCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += value
}

// Snapshot возвращает снимок всех метрик для отправки
func (s *Store) Snapshot() ([]model.Metrics, []model.Metrics) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var gauges []model.Metrics
	for name, value := range s.gauges {
		v := value
		gauges = append(gauges, model.Metrics{
			ID:    name,
			MType: "gauge",
			Value: &v,
		})
	}

	var counters []model.Metrics
	for name, value := range s.counters {
		v := value
		counters = append(counters, model.Metrics{
			ID:    name,
			MType: "counter",
			Delta: &v,
		})
	}

	return gauges, counters
}
