package migration

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
)

// RunMigrations применяет все миграции к базе данных
// migrationsDir - путь к директории с SQL файлами миграций
func RunMigrations(db *sql.DB, migrationsDir string) error {
	// Resolve absolute path to migrations directory
	absPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to resolve migrations path: %w", err)
	}

	// Verify migrations directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("migrations directory not found: %s", absPath)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(db, absPath); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
