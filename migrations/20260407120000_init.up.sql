-- Миграция 001: Создание таблиц для хранения метрик

-- Таблица для метрик типа gauge
CREATE TABLE gauges (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    value DOUBLE PRECISION NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Таблица для метрик типа counter
CREATE TABLE counters (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    delta BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Индексы для ускорения поиска по имени
CREATE INDEX idx_gauges_name ON gauges(name);
CREATE INDEX idx_counters_name ON counters(name);
