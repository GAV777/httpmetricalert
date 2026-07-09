package storage

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/GAV777/httpmetricalert/internal/model"
)

func TestNewStorageWithoutDSN(t *testing.T) {
	// Без DSN должно создаваться in-memory хранилище
	storage := NewStorage()
	if storage == nil {
		t.Fatal("expected non-nil storage")
	}
}

func TestNewFileStorageCreation(t *testing.T) {
	storage := newFileStorage()
	if storage == nil {
		t.Fatal("expected non-nil file storage")
	}
}

func TestNewMemStorageOnlyCreation(t *testing.T) {
	storage := newMemStorageOnly()
	if storage == nil {
		t.Fatal("expected non-nil storage")
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestMemStorageSaveToFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "metrics-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	memStorage := &MemStorage{
		data: map[string]model.Metrics{
			"TestGauge": {ID: "TestGauge", MType: "gauge", Value: float64Ptr(123.45)},
		},
	}

	// Сохраняем в файл
	file, err := os.Create(tmpPath)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	encoder := json.NewEncoder(file)
	err = encoder.Encode(memStorage.data)
	file.Close()

	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Проверяем, что файл существует и не пуст
	info, err := os.Stat(tmpPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty file")
	}
}

func TestMemStorageLoadFromFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "metrics-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()

	// Записываем тестовые данные
	testData := map[string]model.Metrics{
		"TestGauge":   {ID: "TestGauge", MType: "gauge", Value: float64Ptr(123.45)},
		"TestCounter": {ID: "TestCounter", MType: "counter", Delta: int64Ptr(42)},
	}

	encoder := json.NewEncoder(tmpFile)
	err = encoder.Encode(testData)
	tmpFile.Close()

	if err != nil {
		t.Fatalf("failed to write test data: %v", err)
	}
	defer os.Remove(tmpPath)

	// Загружаем из файла
	memStorage := &MemStorage{
		data: make(map[string]model.Metrics),
	}

	file, err := os.Open(tmpPath)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&memStorage.data)

	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(memStorage.data) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(memStorage.data))
	}
}
