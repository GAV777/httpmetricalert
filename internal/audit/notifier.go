package audit

import "encoding/json"

// AuditEvent — событие аудита, отправляемое после обработки метрик
type AuditEvent struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// MarshalJSON сериализует событие в JSON
func (e AuditEvent) MarshalJSON() ([]byte, error) {
	type alias AuditEvent
	return json.Marshal(alias(e))
}

// Observer — интерфейс наблюдателя в паттерне «Наблюдатель»
type Observer interface {
	Notify(event AuditEvent) error
	Close() error
}

// Notifier — субъект, управляющий наблюдателями и рассылающий события
type Notifier struct {
	observers []Observer
}

// NewNotifier создаёт новый нотификатор без наблюдателей
func NewNotifier() *Notifier {
	return &Notifier{}
}

// AddObserver добавляет наблюдателя к нотификатору
func (n *Notifier) AddObserver(o Observer) {
	n.observers = append(n.observers, o)
}

// Notify рассылает событие всем наблюдателям
func (n *Notifier) Notify(event AuditEvent) {
	for _, o := range n.observers {
		_ = o.Notify(event)
	}
}

// Close закрывает всех наблюдателей
func (n *Notifier) Close() {
	for _, o := range n.observers {
		_ = o.Close()
	}
}

// HasObservers возвращает true, если есть хотя бы один наблюдатель
func (n *Notifier) HasObservers() bool {
	return len(n.observers) > 0
}
