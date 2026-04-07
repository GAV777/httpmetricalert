# Migrations

В данной директории содержатся файлы миграций базы данных, управляемые инструментом [goose](https://github.com/pressly/goose).

## Описание

Миграции базы данных — это скрипты, которые позволяют:

- версионировать изменения схемы базы данных
- применять изменения в правильном порядке
- откатывать изменения при необходимости

## Структура файлов

Файлы миграций следуют соглашению об именовании goose:
- `{timestamp}_{name}.up.sql` — миграция для применения (forward)
- `{timestamp}_{name}.down.sql` — миграция для отката (rollback)

Пример: `20260407120000_init.up.sql`

## Управление миграциями

Миграции применяются автоматически при запуске сервера, если указан `DATABASE_DSN`.

### Ручное управление

Для ручного управления миграциями можно использовать goose CLI:

```bash
# Установить goose
go install github.com/pressly/goose/v3/cmd/goose@latest

# Применить все миграции
 goose postgres "postgres://user:password@localhost:5432/dbname" up

# Откатить последнюю миграцию
goose postgres "postgres://user:password@localhost:5432/dbname" down

# Проверить статус миграций
goose postgres "postgres://user:password@localhost:5432/dbname" status
```

## Создание новых миграций

Для создания новой миграции используйте:

```bash
# Создать новую миграцию
goose create <migration_name> sql -dir migrations/

# Пример:
goose create add_users_table sql -dir migrations/
```

Это создаст два файла:
- `{timestamp}_add_users_table.up.sql`
- `{timestamp}_add_users_table.down.sql`

## Конфигурация

Путь к директории миграций можно настроить через:
- Переменную окружения: `MIGRATIONS_DIR` (по умолчанию: `migrations`)
- Флаг командной строки: `-migrations-dir`
