-- +goose Up
-- Миграция 001: Создание таблиц для хранения метрик

CREATE TABLE gauges (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    value DOUBLE PRECISION NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE counters (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    delta BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_gauges_name ON gauges(name);
CREATE INDEX idx_counters_name ON counters(name);

-- +goose Down
DROP INDEX IF EXISTS idx_counters_name;
DROP INDEX IF EXISTS idx_gauges_name;
DROP TABLE IF EXISTS counters;
DROP TABLE IF EXISTS gauges;
