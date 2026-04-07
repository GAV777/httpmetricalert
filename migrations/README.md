# Migrations

В данной директории содержатся файлы миграций базы данных, управляемые инструментом [goose](https://github.com/pressly/goose).

SQL-файлы миграций находятся в пакете `internal/migration/` и встроены в бинарный файл через `//go:embed`.

## Управление миграциями

Миграции применяются автоматически при запуске сервера, если указан `DATABASE_DSN`.

### Ручное управление

Для ручного управления миграциями можно использовать goose CLI:

```bash
# Установить goose
go install github.com/pressly/goose/v3/cmd/goose@latest

# Применить все миграции
goose postgres "postgres://user:password@localhost:5432/dbname" up -dir internal/migration

# Откатить последнюю миграцию
goose postgres "postgres://user:password@localhost:5432/dbname" down -dir internal/migration

# Проверить статус миграций
goose postgres "postgres://user:password@localhost:5432/dbname" status -dir internal/migration
```

## Создание новых миграций

Для создания новой миграции используйте:

```bash
# Создать новую миграцию
goose create -dir internal/migration <migration_name> sql

# Пример:
goose create -dir internal/migration add_users_table sql
```

Это создаст два файла в `internal/migration/`:
- `{timestamp}_add_users_table.up.sql`
- `{timestamp}_add_users_table.down.sql`
