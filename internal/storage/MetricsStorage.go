package storage

import "github.com/GAV777/httpmetricalert/internal/model"

// MetricsStorage — интерфейс для хранилища метрик
type MetricsStorage interface {
	SetGauge(name string, value float64) error
	SetCounter(name string, delta int64) error
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAll() []model.Metrics
	Ping() error
	Save() error
	Load() error
	UpdateBatch(metrics []model.Metrics) error
	Close() error
}
