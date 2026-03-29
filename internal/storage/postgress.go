package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/GAV777/httpmetricalert/internal/model"
	"log"

	_ "github.com/lib/pq"
)

var ErrNotFound = errors.New("metric not found")

type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage подключается к PostgreSQL и создаёт таблицы
func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Создаём таблицы
	if err := createTables(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	log.Println("✅ Подключено к PostgreSQL")
	return &PostgresStorage{db: db}, nil
}

// createTables создаёт необходимые таблицы в БД
func createTables(db *sql.DB) error {
	schema := `
	-- Таблица для метрик типа gauge
	CREATE TABLE IF NOT EXISTS gauges (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL UNIQUE,
		value DOUBLE PRECISION NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	-- Таблица для метрик типа counter
	CREATE TABLE IF NOT EXISTS counters (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL UNIQUE,
		delta BIGINT NOT NULL DEFAULT 0,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	-- Индексы для ускорения поиска по имени
	CREATE INDEX IF NOT EXISTS idx_gauges_name ON gauges(name);
	CREATE INDEX IF NOT EXISTS idx_counters_name ON counters(name);
	`

	_, err := db.Exec(schema)
	return err
}

// Ping проверяет соединение с БД
func (p *PostgresStorage) Ping() error {
	return p.db.Ping()
}

// SetGauge устанавливает значение gauge метрики
func (p *PostgresStorage) SetGauge(name string, value float64) {
	query := `
		INSERT INTO gauges (name, value, updated_at) 
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (name) 
		DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP
	`
	_, err := p.db.Exec(query, name, value)
	if err != nil {
		log.Printf("❌ Ошибка записи gauge %s: %v", name, err)
	}
}

// SetCounter устанавливает значение counter метрики (инкремент)
func (p *PostgresStorage) SetCounter(name string, delta int64) {
	query := `
		INSERT INTO counters (name, delta, updated_at) 
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (name) 
		DO UPDATE SET delta = counters.delta + $2, updated_at = CURRENT_TIMESTAMP
	`
	_, err := p.db.Exec(query, name, delta)
	if err != nil {
		log.Printf("❌ Ошибка записи counter %s: %v", name, err)
	}
}

// GetGauge получает значение gauge метрики
func (p *PostgresStorage) GetGauge(name string) (float64, bool) {
	var value float64
	query := `SELECT value FROM gauges WHERE name = $1`
	err := p.db.QueryRow(query, name).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false
		}
		log.Printf("❌ Ошибка чтения gauge %s: %v", name, err)
		return 0, false
	}
	return value, true
}

// GetCounter получает значение counter метрики
func (p *PostgresStorage) GetCounter(name string) (int64, bool) {
	var delta int64
	query := `SELECT delta FROM counters WHERE name = $1`
	err := p.db.QueryRow(query, name).Scan(&delta)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false
		}
		log.Printf("❌ Ошибка чтения counter %s: %v", name, err)
		return 0, false
	}
	return delta, true
}

// GetAll получает все метрики из БД
func (p *PostgresStorage) GetAll() []model.Metrics {
	var metrics []model.Metrics

	// Получаем все gauge
	rows, err := p.db.Query(`SELECT name, value FROM gauges`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var value float64
			if err := rows.Scan(&name, &value); err == nil {
				metrics = append(metrics, model.Metrics{
					ID:    name,
					MType: model.Gauge,
					Value: &value,
				})
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("❌ Ошибка чтения gauges: %v", err)
		}
	}

	// Получаем все counter
	rows, err = p.db.Query(`SELECT name, delta FROM counters`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var delta int64
			if err := rows.Scan(&name, &delta); err == nil {
				metrics = append(metrics, model.Metrics{
					ID:    name,
					MType: model.Counter,
					Delta: &delta,
				})
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("❌ Ошибка чтения counters: %v", err)
		}
	}

	return metrics
}

// Close закрывает соединение с БД
func (p *PostgresStorage) Close() error {
	return p.db.Close()
}

// SetCounterAbs устанавливает абсолютное значение counter (для восстановления из файла)
func (p *PostgresStorage) SetCounterAbs(name string, delta int64) {
	query := `
		INSERT INTO counters (name, delta, updated_at) 
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (name) 
		DO UPDATE SET delta = $2, updated_at = CURRENT_TIMESTAMP
	`
	_, err := p.db.Exec(query, name, delta)
	if err != nil {
		log.Printf("❌ Ошибка записи counter %s: %v", name, err)
	}
}

// Restore восстанавливает метрики из БД при старте (для совместимости)
func (p *PostgresStorage) Restore() error {
	// В PostgreSQL данные сохраняются постоянно, восстановление не требуется
	return nil
}

// Save сохраняет метрики (для совместимости интерфейса)
func (p *PostgresStorage) Save() error {
	// В PostgreSQL данные сохраняются сразу, отдельное сохранение не требуется
	return nil
}

// Load загружает метрики (для совместимости интерфейса)
func (p *PostgresStorage) Load() error {
	return nil
}

// SetContext используется для совместимости с контекстным интерфейсом
func (p *PostgresStorage) SetContext(ctx context.Context) {
	// Не используется в текущей реализации
}

// UpdateBatch обновляет множество метрик в рамках одной транзакции
func (p *PostgresStorage) UpdateBatch(metrics []model.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	gaugeQuery := `
		INSERT INTO gauges (name, value, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (name)
		DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP
	`
	counterQuery := `
		INSERT INTO counters (name, delta, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (name)
		DO UPDATE SET delta = counters.delta + $2, updated_at = CURRENT_TIMESTAMP
	`

	for _, metric := range metrics {
		switch metric.MType {
		case model.Gauge:
			if metric.Value == nil {
				continue
			}
			if _, err := tx.Exec(gaugeQuery, metric.ID, *metric.Value); err != nil {
				return err
			}
		case model.Counter:
			if metric.Delta == nil {
				continue
			}
			if _, err := tx.Exec(counterQuery, metric.ID, *metric.Delta); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
