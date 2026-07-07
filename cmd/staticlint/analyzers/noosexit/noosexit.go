// Package noosexit предоставляет анализатор, который проверяет корректность
// завершения программы в пакете main.
//
// Анализатор выявляет три категории проблем:
//   - Вызов os.Exit в функции main — прерывает выполнение без defer
//   - Вызов panic в функции main — аварийное завершение без обработки
//   - Вызов os.Exit или log.Fatal вне функции main — некорректное размещение
package noosexit

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Doc описывает назначение анализатора для multichecker.
const Doc = `noosexit проверяет корректность завершения программы в пакете main.

Вызов os.Exit прерывает выполнение программы немедленно, не выполняя
defer-функции. panic в main также приводит к аварийному завершению
без graceful shutdown. Вызов os.Exit или log.Fatal вне функции main
нарушает инкапсуляцию и усложняет тестирование.

Рекомендации:
- в main: используйте возврат ошибки вместо os.Exit/panic
- вне main: возвращайте error вместо вызова log.Fatal/os.Exit`

// Analyzer предоставляет конфигурацию анализатора noosexit.
var Analyzer = &analysis.Analyzer{
	Name: "noosexit",
	Doc:  Doc,
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Пропускаем только пакеты main
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	// Пропускаем тестовые пакеты (они имеют суффикс .test)
	if strings.HasSuffix(pass.Pkg.Path(), ".test") {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	var mainFuncPos token.Pos

	// Первый проход: находим функцию main и проверяем os.Exit/panic внутри неё
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(node ast.Node) {
		funcDecl := node.(*ast.FuncDecl)

		// Проверяем, что это функция main
		if funcDecl.Name.Name != "main" {
			return
		}

		// Проверяем, что это не метод
		if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
			return
		}

		mainFuncPos = funcDecl.Pos()

		// Обходим тело функции main в поисках os.Exit и panic
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Проверяем os.Exit
			if isCallTo(callExpr, pass, "os", "Exit") {
				pass.Reportf(callExpr.Pos(),
					"не используйте os.Exit в функции main — используйте log.Fatal или возврат ошибки")
			}

			// Проверяем panic
			if isCallTo(callExpr, pass, "", "panic") {
				pass.Reportf(callExpr.Pos(),
					"не используйте panic в функции main — используйте возврат ошибки")
			}

			return true
		})
	})

	// Второй проход: проверяем os.Exit и log.Fatal вне функции main
	insp.Preorder(nodeFilter, func(node ast.Node) {
		funcDecl := node.(*ast.FuncDecl)

		// Пропускаем саму функцию main
		if funcDecl.Name.Name == "main" && mainFuncPos != 0 {
			if funcDecl.Pos() == mainFuncPos {
				return
			}
		}

		// Обходим тело функции в поисках os.Exit и log.Fatal
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			if isCallTo(callExpr, pass, "os", "Exit") {
				pass.Reportf(callExpr.Pos(),
					"не используйте os.Exit вне функции main — возвращайте ошибку")
			}

			if isCallTo(callExpr, pass, "log", "Fatal") ||
				isCallTo(callExpr, pass, "log", "Fatalf") ||
				isCallTo(callExpr, pass, "log", "Fatalln") {
				pass.Reportf(callExpr.Pos(),
					"не используйте log.Fatal вне функции main — возвращайте ошибку")
			}

			return true
		})
	})

	return nil, nil
}

// isCallTo проверяет, является ли вызов обращением к pkg.Func
func isCallTo(callExpr *ast.CallExpr, pass *analysis.Pass, pkg, fn string) bool {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	if selExpr.Sel.Name != fn {
		return false
	}

	if pkg == "" {
		// builtin (panic) — достаточно имени
		return ident.Name == ""
	}

	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}

	return pkgName.Imported().Path() == pkg
}
