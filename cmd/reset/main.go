package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// structInfo хранит информацию о структуре, для которой нужно сгенерировать Reset()
type structInfo struct {
	Name   string
	Fields []fieldInfo
}

// fieldInfo хранит информацию об отдельном поле структуры
type fieldInfo struct {
	Name string
	Type string
	// isPointer — true, если поле является указателем
	isPointer bool
	// pointerBase — базовый тип указателя (без *)
	pointerBase string
}

// generatedMethod представляет сгенерированный метод для одной структуры
type generatedMethod struct {
	StructName string
	Body       string
}

// packageMethods хранит все методы для одного пакета
type packageMethods struct {
	Dir         string
	PackageName string
	Methods     []generatedMethod
}

func main() {
	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	allPackages, err := scanPackages(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scan packages: %v\n", err)
		os.Exit(1)
	}

	for _, pkg := range allPackages {
		if len(pkg.Methods) == 0 {
			continue
		}

		if err := writeResetFile(&pkg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write reset file for package %s: %v\n", pkg.PackageName, err)
			os.Exit(1)
		}
		fmt.Printf("Generated reset methods for package %s (%d structs)\n", pkg.PackageName, len(pkg.Methods))
	}
}

// scanPackages обходит все .go файлы начиная с root и находит структуры с комментарием // generate:reset
func scanPackages(root string) ([]packageMethods, error) {
	pkgMap := make(map[string]*packageMethods)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Пропускаем директорию vendor, .git, и сгенерированные файлы
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == ".git" || base == "reset" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Пропускаем уже сгенерированные файлы
		if strings.HasSuffix(path, "reset.gen.go") {
			return nil
		}

		structs, pkgName, err := parseFile(path)
		if err != nil {
			// Не критичная ошибка — возможно файл не парсится из-за build tags
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", path, err)
			return nil
		}

		if len(structs) == 0 {
			return nil
		}

		dir := filepath.Dir(path)
		pkg, exists := pkgMap[dir]
		if !exists {
			pkg = &packageMethods{
				Dir:         dir,
				PackageName: pkgName,
			}
			pkgMap[dir] = pkg
		}

		for _, s := range structs {
			body := generateResetBody(s)
			pkg.Methods = append(pkg.Methods, generatedMethod{
				StructName: s.Name,
				Body:       body,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Собираем результат в слайс
	var result []packageMethods
	for _, pkg := range pkgMap {
		result = append(result, *pkg)
	}
	return result, nil
}

// parseFile парсит один .go файл и возвращает структуры с комментарием // generate:reset
func parseFile(path string) ([]structInfo, string, error) {
	fset := token.NewFileSet()

	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, "", err
	}

	pkgName := node.Name.Name
	var structs []structInfo

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Проверяем комментарий // generate:reset
			if !hasGenerateResetComment(genDecl) {
				continue
			}

			si := structInfo{
				Name: typeSpec.Name.Name,
			}

			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}

				fieldType := exprString(field.Type)
				fi := fieldInfo{
					Name: field.Names[0].Name,
					Type: fieldType,
				}

				// Проверяем, является ли поле указателем
				if starExpr, ok := field.Type.(*ast.StarExpr); ok {
					fi.isPointer = true
					fi.pointerBase = exprString(starExpr.X)
				}

				si.Fields = append(si.Fields, fi)
			}

			structs = append(structs, si)
		}
	}

	return structs, pkgName, nil
}

// hasGenerateResetComment проверяет, есть ли у декларации комментарий // generate:reset
func hasGenerateResetComment(decl *ast.GenDecl) bool {
	if decl.Doc != nil {
		for _, comment := range decl.Doc.List {
			if strings.TrimSpace(comment.Text) == "// generate:reset" {
				return true
			}
		}
	}
	return false
}

// exprString возвращает строковое представление AST выражения типа
func exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprString(e.X)
	case *ast.ArrayType:
		return "[]" + exprString(e.Elt)
	case *ast.MapType:
		return "map[" + exprString(e.Key) + "]" + exprString(e.Value)
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		if e.Dir == ast.SEND {
			return "chan<- " + exprString(e.Value)
		}
		if e.Dir == ast.RECV {
			return "<-chan " + exprString(e.Value)
		}
		return "chan " + exprString(e.Value)
	case *ast.FuncType:
		return "func"
	case *ast.Ellipsis:
		return "..." + exprString(e.Elt)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// generateResetBody генерирует тело метода Reset() для структуры
func generateResetBody(si structInfo) string {
	var buf bytes.Buffer

	// Nil check
	recv := receiverName(si.Name)
	buf.WriteString(fmt.Sprintf("\tif %s == nil {\n\t\treturn\n\t}\n", recv))

	for _, field := range si.Fields {
		line := generateFieldReset(field, si.Name)
		if line != "" {
			buf.WriteString(line)
		}
	}

	return buf.String()
}

// receiverName возвращает имя ресивера для метода Reset()
func receiverName(structName string) string {
	// Берём первые две буквы имени структуры в нижнем регистре
	if len(structName) >= 2 {
		return strings.ToLower(structName[:2])
	}
	return strings.ToLower(structName)
}

// generateFieldReset генерирует строку сброса для одного поля
func generateFieldReset(fi fieldInfo, structName string) string {
	recv := receiverName(structName)
	fieldRef := fmt.Sprintf("%s.%s", recv, fi.Name)

	// Определяем тип поля и генерируем соответствующий код сброса
	switch {
	case fi.isPointer:
		return generatePointerReset(fieldRef, fi.pointerBase, recv)
	case strings.HasPrefix(fi.Type, "[]"):
		// Слайс — обрезаем до нулевой длины
		return fmt.Sprintf("\t%s = %s[:0]\n", fieldRef, fieldRef)
	case strings.HasPrefix(fi.Type, "map["):
		// Мапа — очищаем через clear
		return fmt.Sprintf("\tclear(%s)\n", fieldRef)
	case isPrimitive(fi.Type):
		// Примитив — зануляем
		return fmt.Sprintf("\t%s = %s\n", fieldRef, zeroValue(fi.Type))
	default:
		// Вложенная структура — вызываем Reset() напрямую (не указатель)
		return generateDirectReset(fieldRef)
	}
}

// generatePointerReset генерирует сброс для поля-указателя
func generatePointerReset(fieldRef, baseType, recv string) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("\tif %s != nil {\n", fieldRef))

	switch {
	case isPrimitive(baseType):
		buf.WriteString(fmt.Sprintf("\t\t*%s = %s\n", fieldRef, zeroValue(baseType)))
	case strings.HasPrefix(baseType, "[]"):
		buf.WriteString(fmt.Sprintf("\t\t*%s = (*%s)[:0]\n", fieldRef, fieldRef))
	case strings.HasPrefix(baseType, "map["):
		buf.WriteString(fmt.Sprintf("\t\tclear(*%s)\n", fieldRef))
	default:
		// Указатель на структуру — вызываем Reset()
		buf.WriteString(fmt.Sprintf("\t\t%s.Reset()\n", fieldRef))
	}

	buf.WriteString("\t}\n")
	return buf.String()
}

// generateDirectReset генерирует сброс для вложенной структуры (не указатель)
func generateDirectReset(fieldRef string) string {
	// Для не-указательных структур вызываем Reset() напрямую
	return fmt.Sprintf("\t%s.Reset()\n", fieldRef)
}

// isPrimitive возвращает true для примитивных типов
func isPrimitive(t string) bool {
	switch t {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64",
		"string", "bool", "byte", "rune":
		return true
	}
	return false
}

// zeroValue возвращает нулевое значение для типа
func zeroValue(t string) string {
	switch t {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "byte", "rune":
		return "0"
	case "string":
		return `""`
	case "bool":
		return "false"
	default:
		return "nil"
	}
}

const resetFileTemplate = `// Code generated by cmd/reset; DO NOT EDIT.

package {{.PackageName}}
{{range .Methods}}

// Reset сбрасывает состояние {{.StructName}} к начальным значениям.
func ({{receiverName .StructName}} *{{.StructName}}) Reset() {
{{.Body}}}
{{end}}
`

// writeResetFile записывает сгенерированные методы в reset.gen.go
func writeResetFile(pkg *packageMethods) error {
	outputPath := filepath.Join(pkg.Dir, "reset.gen.go")

	tmpl, err := template.New("reset").Funcs(template.FuncMap{
		"receiverName": receiverName,
	}).Parse(resetFileTemplate)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, pkg); err != nil {
		return err
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}
