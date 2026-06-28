// Package migration предоставляет функции для применения SQL-миграций к базе данных.
//
// Миграции встроены в бинарный файл через go:embed и применяются
// автоматически при подключении к PostgreSQL.
package migration

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var embedMigrations embed.FS

// RunMigrations применяет все миграции к базе данных
func RunMigrations(db *sql.DB) error {
	// Устанавливаем встроенную файловую систему с миграциями
	goose.SetBaseFS(embedMigrations)

	// Применяем миграции из embedded файловой системы
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("не удалось применить миграции: %w", err)
	}

	return nil
}
