package gofrontend

import (
	"fmt"
	"go/ast"
	"strings"
)

func emitBoundParams(fields *ast.FieldList, lowerType func(ast.Expr) string, tempName func() string, bindNamed func(string, string) string) []string {
	if fields == nil || len(fields.List) == 0 {
		return nil
	}
	var out []string
	for _, field := range fields.List {
		ty := lowerType(field.Type)
		if len(field.Names) == 0 {
			out = append(out, fmt.Sprintf("%s: %s", tempName(), ty))
			continue
		}
		for _, name := range field.Names {
			if name.Name == "_" {
				out = append(out, fmt.Sprintf("%s: %s", tempName(), ty))
				continue
			}
			out = append(out, fmt.Sprintf("%s: %s", bindNamed(name.Name, ty), ty))
		}
	}
	return out
}

func emitFieldTypes(fields *ast.FieldList, lowerType func(ast.Expr) string) []string {
	if fields == nil || len(fields.List) == 0 {
		return nil
	}
	var out []string
	for _, field := range fields.List {
		ty := lowerType(field.Type)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			out = append(out, ty)
		}
	}
	return out
}

func formatFuncHeader(name string, params []string, results []string) string {
	return formatFuncHeaderWithVisibility(name, params, results, "")
}

func formatPrivateFuncHeader(name string, params []string, results []string) string {
	return formatFuncHeaderWithVisibility(name, params, results, "private ")
}

func formatFuncHeaderWithVisibility(name string, params []string, results []string, visibility string) string {
	switch len(results) {
	case 0:
		return fmt.Sprintf("  func.func %s@%s(%s) {\n", visibility, sanitizeName(name), strings.Join(params, ", "))
	case 1:
		return fmt.Sprintf("  func.func %s@%s(%s) -> %s {\n", visibility, sanitizeName(name), strings.Join(params, ", "), results[0])
	default:
		return fmt.Sprintf("  func.func %s@%s(%s) -> (%s) {\n", visibility, sanitizeName(name), strings.Join(params, ", "), strings.Join(results, ", "))
	}
}

func formatFuncDecl(name string, params []string, results []string) string {
	switch len(results) {
	case 0:
		return fmt.Sprintf("  func.func private @%s(%s)\n", sanitizeName(name), strings.Join(params, ", "))
	case 1:
		return fmt.Sprintf("  func.func private @%s(%s) -> %s\n", sanitizeName(name), strings.Join(params, ", "), results[0])
	default:
		return fmt.Sprintf("  func.func private @%s(%s) -> (%s)\n", sanitizeName(name), strings.Join(params, ", "), strings.Join(results, ", "))
	}
}
