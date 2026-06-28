// Package noosexit предоставляет анализатор, который запрещает
// прямой вызов os.Exit в функции main пакета main.
//
// Анализатор проверяет AST функцию main пакета main и ищет
// любые вызовы os.Exit. Если такой вызов обнаружен, анализатор
// выдаёт предупреждение, так как os.Exit прерывает выполнение
// программы без выполнения defer-функций, что может привести
// к утечке ресурсов или незавершённым операциям.
package noosexit

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Doc описывает назначение анализатора для multichecker.
const Doc = `noosexit запрещает прямой вызов os.Exit в функции main пакета main.

Вызов os.Exit прерывает выполнение программы немедленно, не выполняя
defer-функции. Это может привести к:
- утечке ресурсов (файлы, соединения с БД не закрываются);
- незавершённым транзакциям;
- потере данных из буферов логгера.

Вместо os.Exit рекомендуется использовать возврат ошибки из main
или log.Fatal, который корректно завершает программу после flush логгера.`

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

	// Ищем функцию main
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(node ast.Node) {
		funcDecl := node.(*ast.FuncDecl)

		// Проверяем, что это функция main
		if funcDecl.Name.Name != "main" {
			return
		}

		// Проверяем, что это не метод (у FuncDecl нет Recv для main)
		if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
			return
		}

		// Обходим тело функции main в поисках os.Exit
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Проверяем, что это вызов os.Exit
			if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				// Проверяем, что X — это идентификатор "os"
				if ident, ok := selExpr.X.(*ast.Ident); ok && ident.Name == "os" {
					// Проверяем, что Selector — это "Exit"
					if selExpr.Sel.Name == "Exit" {
						// Дополнительно проверяем через types.Info, что это действительно os.Exit
						if selObj := pass.TypesInfo.ObjectOf(selExpr.Sel); selObj != nil {
							if pkgName, ok := pass.TypesInfo.ObjectOf(ident).(*types.PkgName); ok {
								if pkgName.Imported().Path() == "os" {
									pass.Reportf(callExpr.Pos(),
										"не используйте os.Exit в функции main — используйте log.Fatal или возврат ошибки")
								}
							}
						}
					}
				}
			}
			return true
		})
	})

	return nil, nil
}
