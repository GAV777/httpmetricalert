package storage

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/GAV777/httpmetricalert/internal/config"
	"github.com/GAV777/httpmetricalert/internal/model"
)

type MemStorage struct {
	data map[string]model.Metrics
	mu   sync.RWMutex
}

func NewMemStorage() *MemStorage {
	storage := &MemStorage{
		data: make(map[string]model.Metrics),
	}

	if config.ShouldRestore() {
		storage.Load()
	}

	if config.StoreInterval() == 0 {
		// Sync mode — no ticker
		return storage
	}

	// Start periodic save
	go func() {
		ticker := time.NewTicker(time.Duration(config.StoreInterval()) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			storage.Save()
		}
	}()

	return storage
}

func (s *MemStorage) Save() error {
	s.mu.RLock()
	data := make([]model.Metrics, 0, len(s.data))
	for _, v := range s.data {
		data = append(data, v)
	}
	s.mu.RUnlock()

	file, err := os.OpenFile(config.FileStoragePath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("Failed to open file for saving: %v", err)
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		log.Printf("Failed to encode metrics: %v", err)
		return err
	}

	return nil
}

func (s *MemStorage) Load() error {
	file, err := os.Open(config.FileStoragePath())
	if os.IsNotExist(err) {
		log.Printf("Storage file not found: %s", config.FileStoragePath())
		return nil
	} else if err != nil {
		log.Printf("Failed to open storage file: %v", err)
		return err
	}
	defer file.Close()

	var metrics []model.Metrics
	if err := json.NewDecoder(file).Decode(&metrics); err != nil {
		log.Printf("Failed to decode metrics: %v", err)
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range metrics {
		s.data[m.ID] = m
	}

	log.Printf("Loaded %d metrics from storage", len(metrics))
	return nil
}

// Sync Save if needed after updates
func (s *MemStorage) SetGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[name] = model.Metrics{
		ID:    name,
		MType: "gauge",
		Value: &value,
	}

	if config.StoreInterval() == 0 {
		s.Save()
	}
}

func (s *MemStorage) SetCounter(name string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.data[name]
	var newValue int64
	if ok && existing.Delta != nil {
		newValue = *existing.Delta + delta
	} else {
		newValue = delta
	}

	s.data[name] = model.Metrics{
		ID:    name,
		MType: "counter",
		Delta: &newValue,
	}

	if config.StoreInterval() == 0 {
		s.Save()
	}
}

func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.data[name]
	if !ok || m.Value == nil {
		return 0, false
	}
	return *m.Value, true
}

func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.data[name]
	if !ok || m.Delta == nil {
		return 0, false
	}
	return *m.Delta, true
}

func (s *MemStorage) GetAll() []model.Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]model.Metrics, 0, len(s.data))
	for _, v := range s.data {
		result = append(result, v)
	}
	return result
}
