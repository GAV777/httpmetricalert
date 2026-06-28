package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// HTTPObserver — наблюдатель, отправляющий события аудита по HTTP POST
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver создаёт наблюдателя для отправки на удалённый сервер
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url:    url,
		client: &http.Client{},
	}
}

// Notify отправляет событие аудита методом POST на указанный URL
func (o *HTTPObserver) Notify(event AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	resp, err := o.client.Post(o.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Close для HTTPObserver не требует действий
func (o *HTTPObserver) Close() error {
	return nil
}
