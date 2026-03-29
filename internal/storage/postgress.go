package storage

import (
	"database/sql"
	"github.com/GAV777/httpmetricalert/internal/model"
	"log"

	_ "github.com/lib/pq"
)

type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage подключается к PostgreSQL, но не создаёт таблицу
func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	log.Println("✅ Подключено к PostgreSQL (без использования таблицы)")
	return &PostgresStorage{db: db}, nil
}

// Ping проверяет соединение с БД
func (p *PostgresStorage) Ping() error {
	return p.db.Ping()
}

// --- Заглушки: ничего не делаем ---

func (p *PostgresStorage) SetGauge(name string, value float64) {
	// Ничего не делаем
}

func (p *PostgresStorage) SetCounter(name string, delta int64) {
	// Ничего не делаем
}

func (p *PostgresStorage) GetGauge(name string) (float64, bool) {
	return 0, false // не найдено
}

func (p *PostgresStorage) GetCounter(name string) (int64, bool) {
	return 0, false // не найдено
}

func (p *PostgresStorage) GetAll() []model.Metrics {
	return nil // пустой список
}
