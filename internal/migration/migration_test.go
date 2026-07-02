package migration

import (
	"testing"
)

func TestEmbeddedMigrationsExist(t *testing.T) {
	// Проверяем, что embedded migrations существуют
	files, err := embedMigrations.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	if len(files) == 0 {
		t.Error("no migration files found in embedded filesystem")
	}

	// Логируем найденные файлы
	t.Logf("Found %d migration files", len(files))
	for _, f := range files {
		t.Logf("  - %s", f.Name())
	}
}
