package storage

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/GAV777/httpmetricalert/internal/config"
	"github.com/GAV777/httpmetricalert/internal/model"
)

// ErrDatabaseNotAvailable возвращается при попытке пинговать БД,
// когда подключение к PostgreSQL было настроено, но установить его не удалось.
var ErrDatabaseNotAvailable = errors.New("database is not available")

// MemStorage — потокобезопасное in-memory хранилище метрик
// с поддержкой периодического сохранения в файл.
type MemStorage struct {
	data        map[string]model.Metrics
	mu          sync.RWMutex
	dbAttempted bool
}

// NewStorage создаёт хранилище метрик, выбирая тип по конфигурации:
// PostgreSQL при наличии DSN, файловое или in-memory в качестве fallback.
func NewStorage() MetricsStorage {
	dsn := config.DatabaseDSN()

	if dsn != "" {
		storage, err := NewPostgresStorage(dsn)
		if err != nil {
			log.Printf("Failed to connect to PostgreSQL: %v", err)
			log.Println("Falling back to file storage")
			return newFileStorage()
		}
		log.Println("Using PostgreSQL storage")
		return storage
	}

	log.Println("Using in-memory storage")
	return newMemStorageOnly()
}

// initMemStorage инициализирует хранилище: загружает данные и запускает периодическое сохранение
func initMemStorage(dbAttempted bool) *MemStorage {
	storage := &MemStorage{
		data:        make(map[string]model.Metrics),
		dbAttempted: dbAttempted,
	}

	if config.ShouldRestore() {
		_ = storage.Load()
	}

	// В синхронном режиме (interval == 0) сохранение происходит после каждой операции
	if config.StoreInterval() == 0 {
		return storage
	}

	// Запускаем горутину для периодического сохранения
	go func() {
		ticker := time.NewTicker(time.Duration(config.StoreInterval()) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			_ = storage.Save()
		}
	}()

	return storage
}

// newFileStorage создаёт хранилище с сохранением в файл
func newFileStorage() *MemStorage {
	return initMemStorage(true) // БД была настроена, но подключение не удалось
}

// newMemStorageOnly создаёт хранилище только в памяти
func newMemStorageOnly() *MemStorage {
	return initMemStorage(false)
}

// NewMemStorage создаёт пустое in-memory хранилище метрик.
func NewMemStorage() *MemStorage {
	return initMemStorage(false)
}

// Save записывает все метрики в файл, указанный в конфигурации.
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

// Load загружает метрики из файла, указанного в конфигурации.
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

// SetGauge устанавливает значение gauge-метрики.
func (s *MemStorage) SetGauge(name string, value float64) {
	s.mu.Lock()
	s.data[name] = model.Metrics{
		ID:    name,
		MType: "gauge",
		Value: &value,
	}
	needSave := config.StoreInterval() == 0
	s.mu.Unlock()

	if needSave {
		s.Save()
	}
}

// SetCounter устанавливает значение counter-метрики с инкрементом.
func (s *MemStorage) SetCounter(name string, delta int64) {
	s.mu.Lock()
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
	needSave := config.StoreInterval() == 0
	s.mu.Unlock()

	if needSave {
		s.Save()
	}
}

// GetGauge возвращает значение gauge-метрики по имени.
func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.data[name]
	if !ok || m.Value == nil {
		return 0, false
	}
	return *m.Value, true
}

// GetCounter возвращает значение counter-метрики по имени.
func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.data[name]
	if !ok || m.Delta == nil {
		return 0, false
	}
	return *m.Delta, true
}

// GetAll возвращает копию всех метрик из хранилища.
func (s *MemStorage) GetAll() []model.Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]model.Metrics, 0, len(s.data))
	for _, v := range s.data {
		result = append(result, v)
	}
	return result
}

// Ping проверяет доступность хранилища.
func (s *MemStorage) Ping() error {
	// Если БД была настроена, но подключение не удался — возвращаем ошибку
	if s.dbAttempted {
		return ErrDatabaseNotAvailable
	}
	return nil
}

// UpdateBatch обновляет множество метрик пакетом
func (s *MemStorage) UpdateBatch(metrics []model.Metrics) error {
	s.mu.Lock()

	for _, metric := range metrics {
		switch metric.MType {
		case model.Gauge:
			if metric.Value == nil {
				continue
			}
			s.data[metric.ID] = model.Metrics{
				ID:    metric.ID,
				MType: model.Gauge,
				Value: metric.Value,
			}
		case model.Counter:
			if metric.Delta == nil {
				continue
			}
			existing, ok := s.data[metric.ID]
			var newValue int64
			if ok && existing.Delta != nil {
				newValue = *existing.Delta + *metric.Delta
			} else {
				newValue = *metric.Delta
			}
			s.data[metric.ID] = model.Metrics{
				ID:    metric.ID,
				MType: model.Counter,
				Delta: &newValue,
			}
		}
	}

	// В синхронном режиме сохраняем после батча
	if config.StoreInterval() == 0 {
		s.mu.Unlock()
		_ = s.Save()
		s.mu.Lock()
	}

	s.mu.Unlock()
	return nil
}
