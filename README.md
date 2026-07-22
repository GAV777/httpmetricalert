# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Бенчмарки

Проект содержит бенчмарки для измерения производительности ключевых компонентов:

```bash
# Запуск всех бенчмарков
go test -bench=. -benchmem ./internal/storage/ ./internal/handlers/ ./internal/audit/

# Запуск конкретного бенчмарка
go test -bench=BenchmarkUpdateBatchHandler -benchmem ./internal/handlers/
```

### Результаты бенчмарков (до оптимизации)

| Бенчмарк | Время | Память | Аллокации |
|----------|-------|--------|-----------|
| `BenchmarkMemStorage_SetGauge` | ~25 ns/op | 8 B/op | 1 allocs/op |
| `BenchmarkMemStorage_SetCounter` | ~37 ns/op | 8 B/op | 1 allocs/op |
| `BenchmarkMemStorage_GetGauge` | ~17 ns/op | 0 B/op | 0 allocs/op |
| `BenchmarkMemStorage_UpdateBatch` (100) | ~1950 ns/op | 400 B/op | 50 allocs/op |
| `BenchmarkUpdateBatchHandler_Medium` (100) | ~67000 ns/op | 44294 B/op | 393 allocs/op |
| `BenchmarkUpdateBatchHandler_Large` (1000) | ~655000 ns/op | 318618 B/op | 3549 allocs/op |

## Профилирование памяти

Профили памяти сохранены в директории `profiles/`:
- `base.pprof` — профиль до оптимизации
- `result.pprof` — профиль после оптимизации

### Анализ базового профиля

Основные источники аллокаций:
- `reflect.growslice` (42.12%) — рост слайсов при JSON-декодировании
- `encoding/json.(*Decoder).refill` (33.22%) — чтение JSON из body
- `bufio.NewReaderSize` (8.57%) — создание буфера для чтения

### Оптимизации

1. **`getClientIP`** — замена `strings.Split(xff, ",")` на `strings.IndexByte` — избежание создания слайса при каждом запросе
2. **`UpdateBatchHandler`** — предвыделение слайса `make([]string, len(metrics))` вместо `make([]string, 0, len(metrics))` с последующим `append`
3. **`json.NewEncoder(w).Encode`** → `json.Marshal` + `w.Write` — сокращение аллокаций при сериализации ответа

### Сравнение профилей

```
$ pprof -top -diff_base=profiles/base.pprof profiles/result.pprof

File: handlers.test
Type: alloc_space
Showing nodes accounting for 275.40MB, 77.09% of 357.24MB total
      flat  flat%   sum%        cum   cum%
  108.18MB 30.28% 30.28%   108.18MB 30.28%  reflect.growslice
   93.59MB 26.20% 56.48%    93.59MB 26.20%  encoding/json.(*Decoder).refill
   31.12MB  8.71% 65.19%    31.12MB  8.71%  bufio.NewReaderSize (inline)
      11MB  3.08% 68.27%       11MB  3.08%  reflect.unsafe_New
    9.50MB  2.66% 70.93%    20.50MB  5.74%  encoding/json.(*decodeState).literalStore
    5.50MB  1.54% 72.47%     5.50MB  1.54%  net/textproto.MIMEHeader.Set (inline)
       5MB  1.40% 73.87%        5MB  1.40%  net/http.(*Request).WithContext (inline)
       5MB  1.40% 75.27%        5MB  1.40%  github.com/GAV777/httpmetricalert/internal/storage.(*MemStorage).UpdateBatch
    2.50MB   0.7% 75.97%     2.50MB   0.7%  net/url.parse
       2MB  0.56% 76.53%        2MB  0.56%  encoding/json.NewDecoder (inline)
    1.51MB  0.42% 76.95%     1.51MB  0.42%  runtime.mallocgc
    1.50MB  0.42% 77.37%     1.50MB  0.42%  net/http.Header.Clone (inline)
   -1.50MB  0.42% 76.95%       19MB  5.32%  encoding/json.(*decodeState).object
    1.50MB  0.42% 77.37%     1.50MB  0.42%  io.NopCloser (inline)
      -1MB  0.28% 77.09%     3.50MB  0.98%  net/http.readRequest
    -0.50MB  0.14% 76.95%    40.63MB 11.37%  net/http/httptest.NewRequestWithContext
    0.50MB  0.14% 77.09%   233.27MB 65.30%  github.com/GAV777/httpmetricalert/internal/handlers.(*MetricsHandler).UpdateBatchHandler
```

Отрицательные значения (`-1.50MB`, `-1MB`, `-0.50MB`) показывают уменьшение потребления памяти после оптимизации.
