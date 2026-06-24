package audit

import (
	"encoding/json"
	"os"
	"sync"
)

// FileObserver — наблюдатель, записывающий события аудита в файл
type FileObserver struct {
	filepath string
	mu       sync.Mutex
}

// NewFileObserver создаёт наблюдателя для записи в файл
func NewFileObserver(filepath string) *FileObserver {
	return &FileObserver{
		filepath: filepath,
	}
}

// Notify записывает событие аудита в конец файла (JSON на новой строке)
func (o *FileObserver) Notify(event AuditEvent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Открываем файл в режиме append, создаём при необходимости
	f, err := os.OpenFile(o.filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}

// Close для FileObserver не требует действий
func (o *FileObserver) Close() error {
	return nil
}
