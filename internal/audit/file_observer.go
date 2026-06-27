package audit

import (
	"encoding/json"
	"os"
	"sync"
)

// FileObserver — наблюдатель, записывающий события аудита в файл
type FileObserver struct {
	filepath string
	f        *os.File
	mu       sync.Mutex
}

// NewFileObserver создаёт наблюдателя для записи в файл.
// Файл открывается один раз при создании наблюдателя.
func NewFileObserver(filepath string) (*FileObserver, error) {
	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &FileObserver{
		filepath: filepath,
		f:        f,
	}, nil
}

// Notify записывает событие аудита в конец файла (JSON на новой строке)
func (o *FileObserver) Notify(event AuditEvent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = o.f.Write(append(data, '\n'))
	return err
}

// Close закрывает файловый дескриптор
func (o *FileObserver) Close() error {
	return o.f.Close()
}
