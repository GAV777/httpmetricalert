-- Миграция 001: Откат создания таблиц

DROP INDEX IF EXISTS idx_counters_name;
DROP INDEX IF EXISTS idx_gauges_name;
DROP TABLE IF EXISTS counters;
DROP TABLE IF EXISTS gauges;
