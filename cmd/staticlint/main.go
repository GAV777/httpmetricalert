/*
Staticlint — мульти-анализатор для статического анализа Go-проекта httpmetricalert.

# Назначение

Staticlint объединяет несколько анализаторов в один инструмент для комплексной
проверки кода на распространённые ошибки, потенциальные баги и стилистические
проблемы.

# Состав анализаторов

Staticlint состоит из четырёх групп анализаторов:

## 1. Стандартные анализаторы golang.org/x/tools/go/analysis/passes

Эти анализаторы входят в стандартную поставку Go tools и проверяют базовые
паттерны кода:

  - **appends** — обнаруживает некорректное использование append с аргументами,
    которые могут перезаписать данные
  - **asmdecl** — проверяет соответствие объявлений в ассемблерных файлах (.s)
    Go-сигнатурам
  - **assign** — обнаруживает бессмысленные присваивания (переменной самой себе)
  - **atomic** — находит гонки данных при использовании атомарных операций
  - **bools** — обнаруживает логические ошибки в булевых выражениях
  - **buildtag** — проверяет корректность build-тегов в комментариях
  - **cgocall** — обнаруживает некорректные вызовы C-кода из Go
  - **composites** — находит литералы структур без именованных полей
  - **copylocks** — обнаруживает копирование мьютексов по значению
  - **defers** — проверяет корректность defer-вызов (паника в аргументах defer)
  - **directive** — проверяет синтаксис go-директив
  - **errorsas** — проверяет корректность вызовов errors.As
  - **framepointer** — обнаруживает проблемы с frame pointer в ассемблере
  - **httpresponse** — обнаруживает незакрытые тела HTTP-ответов
  - **ifaceassert** — проверяет корректность приведения типов интерфейсов
  - **loopclosure** — обнаруживает захват переменных цикла в замыканиях
  - **lostcancel** — обнаруживает потерянные cancel-функции контекста
  - **nilfunc** — проверяет сравнения функций с nil
  - **nilness** — обнаруживает проверки на nil, которые всегда истинны/ложны
  - **printf** — проверяет корректность форматирующих строк в fmt.Printf и др.
  - **shift** — обнаруживает сдвиги на отрицательные или слишком большие значения
  - **sigchany** — обнаруживает передачу канала в signal.Notify без буфера
  - **slog** — проверяет корректность вызовов log/slog
  - **stdmethods** — обнаруживает нестандартные сигнатуры методов интерфейсов
  - **stringintconv** — обнаруживает некорректные преобразования string(int)
  - **structtag** — проверяет синтаксис тегов структур
  - **testinggoroutine** — обнаруживает запуск горутин в тестах без ожидания
  - **tests** — обнаруживает распространённые ошибки в тестах
  - **timeformat** — проверяет использование time.Parse с форматными строками
  - **unmarshal** — обнаруживает незакрытые Unmarshal-операции
  - **unreachable** — находит unreachable-код после return/panic
  - **unusedresult** — обнаруживает игнорирование результатов функций
  - **unusedwrite** — обнаруживает записи в поля, которые никогда не читаются

## 2. Анализаторы класса SA (staticcheck.io)

Анализаторы класса SA — это "style/atomic" и "suspicious" проверки из пакета
staticcheck. Они обнаруживают подозрительные конструкции и потенциальные баги:

- **SA1000** — некорректная регулярное выражение в regexp.MustCompile
- **SA1001** — некорректный синтаксис шаблона в template.Must
- **SA1002** — некорректная директива в go:generate
- **SA1003** — некорректный аргумент в encoding/xml.Unmarshal
- **SA1004** — некорректное использование time.Tick (утечка памяти)
- **SA1005** — некорректный аргумент в fmt.Printf
- **SA1006** — ручное построение строки вместо fmt.Sprintf
- **SA1007** — некорректный URL в net/url.Parse
- **SA1008** — неканоничный ключ в struct tag
- **SA1010** — некорректное использование regexp.FindAll
- **SA1011** — некорректное использование strings.Split
- **SA1012** — nil-контекст, передаваемый в функцию
- **SA1013** — некорректный аргумент в os.Chmod
- **SA1014** — некорректный аргумент в time.Unix
- **SA1015** — использование time.Tick, ведущее к утечке памяти
- **SA1016** — некорректная обработка os.Signal канала
- **SA1017** — некорректный канал для signal.Notify
- **SA1018** — strings.Replace с count < 0 вместо strings.ReplaceAll
- **SA1019** — использование deprecated-функции
- **SA1020** — некорректный net.ListenTCP
- **SA1021** — некорректное использование bytes.Equal
- **SA1023** — модификация io.Reader в non-EOF случае
- **SA1024** — повторное использование strings.Cut
- **SA1025** — некорректное использование time.Timer
- **SA1026** — некорректное использование atomic.AddUint64
- **SA1027** — некорректное использование atomic.LoadUint64
- **SA1028** — некорректное использование sort.Slice
- **SA1029** — некорректное использование context.WithValue
- **SA1030** — некорректное использование regexp.ReplaceAll
- **SA2000** — missing sync.WaitGroup.Add before goroutine
- **SA2001** — пустой блок в коде (возможно, ошибка)
- **SA2002** — вызов t.Parallel() в неправильном месте
- **SA2003** — race condition в тестах
- **SA3000** — функция main не вызывает os.Exit
- **SA3001** — assignment to receiver parameter
- **SA4000** — одинаковые условия в if/else
- **SA4001** — бессмысленное преобразование типов
- **SA4003** — сравнение unsigned с нулём (< 0 всегда false)
- **SA4004** — цикл с постусловием, выполняющийся 0 раз
- **SA4005** — useless field assignment
- **SA4006** — значение канала не используется
- **SA4008** — переменная в цикле не изменяется
- **SA4009** — аргумент функции не изменяется
- **SA4010** — результат append не используется
- **SA4011** — break без эффекта
- **SA4012** — сравнение с NaN
- **SA4013** — двойное отрицание
- **SA4014** — одинаковые ветки в if/else
- **SA4015** — вызов math.Ceil/Float64bits для получения精度的
- **SA4016** — само-присваивание
- **SA4017** — чистая функция без побочного эффекта, результат не используется
- **SA4018** — само-присваивание в composite literal
- **SA4019** — multiple, identical build constraints
- **SA4020** — unreachable case clause
- **SA4021** — x = append(y) вместо x = y
- **SA4022** — сравнение адреса локальной переменной
- **SA4023** — impossible interface comparison
- **SA4024** — nil check after make
- **SA4025** — unused result of sync.Pool.Get
- **SA4026** — negative len in make slice
- **SA4027** — unused result of fmt.Sprint
- **SA4028** — x = append(y) с y == x
- **SA4029** — unused result of atomic operation
- **SA4030** — post-increment in loop condition
- **SA4031** — comparison with runtime.GOOS/GOARCH
- **SA5000** — nil dereference в коде
- **SA5001** — defer в цикле
- **SA5002** — пустой for-loop (бесконечный цикл)
- **SA5003** — defer с вызовом, который никогда не вернётся
- **SA5004** — missing return в функции
- **SA5005** — цикл с условием, которое никогда не выполнится
- **SA5006** — время, которое никогда не обновляется
- **SA5007** — неправильная структура for-select
- **SA5008** — неверный struct tag json
- **SA5009** — некорректный аргумент в fmt.Printf
- **SA5010** — невозможный nil check
- **SA5011** — возможный nil dereference после проверки
- **SA5012** — некорректный аргумент в syscall
- **SA6000** — неэффективный strings.ReplaceAll в цикле
- **SA6001** — missing key in map range over array
- **SA6002** — неэффективное sync.Pool хранение
- **SA6003** — неэффективное преобразование []byte в string
- **SA6004** — неэффективное использование regexp
- **SA6005** — неэффективное использование strings.ToLower/ToUpper
- **SA6006** — defer os.File.Close без проверки ошибки
- **SA6007** — неэффективное использование sync.Once
- **SA9001** — defer в цикле (дубликат)
- **SA9002** — пустой блокирующий канал
- **SA9003** — empty body в if/else
- **SA9004** — только с _ в import
- **SA9005** — useless regex capture group
- **SA9006** — useless default argument
- **SA9007** — missing Module in go.mod
- **SA9008** — else branch with identical if condition
- **SA9009** — useless struct field tag
- **SA9010** — использование == для сравнения срезов
- **SA9011** — unreachable code в функции main

## 3. Анализаторы других классов staticcheck.io

Помимо класса SA, staticcheck предоставляет другие классы анализаторов:

- **ST1000** (стиль) — проверка на наличие doc-комментария в main
- **ST1001** (стиль) — некорректное использование dot-imports
- **ST1003** (стиль) — некорректные имена идентификаторов
- **ST1005** (стиль) — некорректное использование ошибок
- **ST1006** (стиль) — некорректное использование doc-комментариев
- **ST1008** (стиль) — функция должна возвращать (value, error)
- **ST1011** (стиль) — некорректное использование time.Time
- **ST1012** (стиль) — имена переменных для ошибок
- **ST1013** (стиль) — использование HTTP-метода вместо константы
- **ST1015** (стиль) — os.Exit в main без comment
- **ST1016** (стиль) — receiver type inconsistency
- **ST1017** (стиль) — некорректное использование go:generate
- **ST1018** (стиль) — некорректное использование build-тегов
- **ST1019** (стиль) — duplicate import
- **ST1020** (стиль) — некорректный godoc comment
- **ST1021** (стиль) — некорректный godoc format
- **ST1022** (стиль) — некорректный godoc placement
- **ST1023** (стиль) — redundant type conversion

## 4. Публичные анализаторы на выбор

Дополнительно подключены публичные анализаторы:

  - **noosexit** — собственный анализатор, запрещающий os.Exit в main.main
    (подробнее см. ниже)
  - **golang.org/x/tools/go/analysis/passes/nilness** — проверка на nil
  - **golang.org/x/tools/go/analysis/passes/unusedresult** — неиспользованные
    результаты функций

# Запуск

Для запуска multichecker выполните:

	go run ./cmd/staticlint ./...

Или соберите бинарный файл:

	go build -o staticlint ./cmd/staticlint
	./staticlint ./...

# Собственный анализатор: noosexit

Анализатор noosexit запрещает прямой вызов os.Exit в функции main пакета main.

## Почему это важно

Вызов os.Exit прерывает выполнение программы немедленно, не выполняя
defer-функции. Это может привести к:
- утечке ресурсов (файлы, соединения с БД не закрываются);
- незавершённым транзакциям;
- потере данных из буферов логгера.

## Рекомендация

Вместо os.Exit рекомендуется использовать:
- log.Fatal — корректно завершает программу после flush логгера
- возврат ошибки из main — позволяет обработать ошибку на уровне выше

## Примеры

Плохо:

	package main

	func main() {
	    if err := doSomething(); err != nil {
	        os.Exit(1) // нарушение: defer-функции не выполнятся
	    }
	}

Хорошо:

	package main

	import "log"

	func main() {
	    if err := doSomething(); err != nil {
	        log.Fatal(err) // OK: логгер выполнит flush
	    }
	}

Или:

	package main

	func main() {
	    if err := doSomething(); err != nil {
	        panic(err) // OK: panic выполнит defer-функции
	    }
	}
*/
package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/directive"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"

	"github.com/GAV777/httpmetricalert/cmd/staticlint/analyzers/noosexit"
)

// convertLintAnalyzers преобразует []*lint.Analyzer в []*analysis.Analyzer
func convertLintAnalyzers(lintAnalyzers []*lint.Analyzer) []*analysis.Analyzer {
	result := make([]*analysis.Analyzer, len(lintAnalyzers))
	for i, a := range lintAnalyzers {
		result[i] = a.Analyzer
	}
	return result
}

func main() {
	// Собираем все анализаторы
	var analyzers []*analysis.Analyzer

	// 1. Стандартные анализаторы из golang.org/x/tools
	analyzers = append(analyzers,
		appends.Analyzer,
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		defers.Analyzer,
		directive.Analyzer,
		errorsas.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		printf.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		slog.Analyzer,
		stdmethods.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		testinggoroutine.Analyzer,
		tests.Analyzer,
		timeformat.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unusedresult.Analyzer,
		unusedwrite.Analyzer,
	)

	// 2. Анализаторы класса SA из staticcheck
	analyzers = append(analyzers,
		convertLintAnalyzers(staticcheck.Analyzers)...,
	)

	// 3. Анализаторы класса ST (style) из stylecheck
	analyzers = append(analyzers,
		convertLintAnalyzers(stylecheck.Analyzers)...,
	)

	// 4. Собственный анализатор noosexit
	analyzers = append(analyzers,
		noosexit.Analyzer,
	)

	multichecker.Main(analyzers...)
}
