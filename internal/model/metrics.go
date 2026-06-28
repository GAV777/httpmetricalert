// Package model определяет структуры данных для метрик.
//
// Поддерживаются два типа метрик:
//   - counter — счётчик, значение которого только увеличивается
//   - gauge — измеритель, значение которого может изменяться в любую сторону
//
// Структура Metrics использует указатели для полей Delta и Value,
// чтобы отличать отсутствие значения от нуля при сериализации в JSON.
package model

// Counter — тип метрики «счётчик». Значение может только увеличиваться.
const Counter = "counter"

// Gauge — тип метрики «измеритель». Значение может как увеличиваться, так и уменьшаться.
const Gauge = "gauge"

// Metrics представляет единичную метрику.
//
// Поля Delta и Value объявлены через указатели,
// чтобы отличать явно заданный ноль от отсутствующего значения
// и корректно работать с omitempty при сериализации в JSON.
type Metrics struct {
	ID    string   `json:"id"`              // имя метрики
	MType string   `json:"type"`            // тип метрики: "gauge" или "counter"
	Delta *int64   `json:"delta,omitempty"` // значение для counter
	Value *float64 `json:"value,omitempty"` // значение для gauge
	Hash  string   `json:"hash,omitempty"`  // HMAC-SHA256 подпись
}
