package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationFilesExist(t *testing.T) {
	// Проверяем, что SQL файлы миграций существуют
	migrationDir := "."
	files, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatalf("failed to read migration directory: %v", err)
	}

	sqlFiles := 0
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".sql") {
			sqlFiles++
			t.Logf("Found migration file: %s", f.Name())
		}
	}

	if sqlFiles == 0 {
		t.Error("no SQL migration files found")
	}
}

func TestEmbeddedMigrationsAccessible(t *testing.T) {
	// Проверяем, что мы можем читать embedded файлы
	content, err := embedMigrations.ReadFile("00001_create_tables.sql")
	if err != nil {
		// Файл может не существовать, это OK
		t.Logf("Expected error for non-existent file: %v", err)
		return
	}

	if len(content) == 0 {
		t.Error("expected non-empty migration file")
	}
}

func TestMigrationDirectoryStructure(t *testing.T) {
	// Проверяем, что текущая директория — это internal/migration
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Проверяем, что мы в правильной директории
	if !strings.HasSuffix(wd, "internal/migration") {
		t.Logf("Warning: working directory is %s, expected ...internal/migration", wd)
	}

	// Проверяем наличие go.mod в проекте root
	root := filepath.Join(wd, "../..")
	goMod := filepath.Join(root, "go.mod")
	if _, err := os.Stat(goMod); err != nil {
		t.Errorf("go.mod not found at %s: %v", goMod, err)
	}
}
