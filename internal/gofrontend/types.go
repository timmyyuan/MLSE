package gofrontend

import (
	"fmt"
	"go/ast"
	"strings"
)

func rangeBindingName(expr ast.Expr) (string, bool) {
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return "", false
	}
	return ident.Name, true
}

func rangeKeyType(containerTy string) string {
	if strings.HasPrefix(containerTy, "!go.map<") {
		keyTy, _ := splitMapTypes(containerTy)
		if keyTy != "" {
			return keyTy
		}
		return "!go.any"
	}
	return "i32"
}

func rangeValueType(containerTy string) string {
	switch {
	case strings.HasPrefix(containerTy, "!go.slice<"):
		return unwrapSingleTypeArg(containerTy)
	case strings.HasPrefix(containerTy, "!go.array<"):
		return unwrapSingleTypeArg(containerTy)
	case strings.HasPrefix(containerTy, "!go.map<"):
		_, valueTy := splitMapTypes(containerTy)
		if valueTy != "" {
			return valueTy
		}
	}
	return "!go.any"
}

func unwrapSingleTypeArg(ty string) string {
	start := strings.IndexByte(ty, '<')
	end := strings.LastIndexByte(ty, '>')
	if start < 0 || end <= start+1 {
		return "!go.any"
	}
	return ty[start+1 : end]
}

func splitMapTypes(ty string) (string, string) {
	body := unwrapSingleTypeArg(ty)
	if body == "!go.any" {
		return "", ""
	}
	parts := splitTypeArgs(body)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func splitTypeArgs(input string) []string {
	var out []string
	depth := 0
	start := 0
	for i, r := range input {
		switch r {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(input[start:i]))
				start = i + 1
			}
		}
	}
	last := strings.TrimSpace(input[start:])
	if last != "" {
		out = append(out, last)
	}
	return out
}

func goTypeToMLIR(expr ast.Expr) string {
	switch t := expr.(type) {
	case nil:
		return "!go.unit"
	case *ast.Ident:
		switch t.Name {
		case "int":
			return "i32"
		case "bool":
			return "i1"
		case "string":
			return "!go.string"
		case "error":
			return "!go.error"
		case "any", "interface{}":
			return "!go.any"
		default:
			return "!go.named<\"" + sanitizeName(t.Name) + "\">"
		}
	case *ast.StarExpr:
		return "!go.ptr<" + goTypeToMLIR(t.X) + ">"
	case *ast.SelectorExpr:
		return "!go.sel<\"" + sanitizeName(renderSelector(t)) + "\">"
	case *ast.ArrayType:
		if t.Len == nil {
			return "!go.slice<" + goTypeToMLIR(t.Elt) + ">"
		}
		return "!go.array<" + goTypeToMLIR(t.Elt) + ">"
	case *ast.MapType:
		return "!go.map<" + goTypeToMLIR(t.Key) + "," + goTypeToMLIR(t.Value) + ">"
	case *ast.InterfaceType:
		return "!go.interface"
	case *ast.FuncType:
		return "!go.func"
	case *ast.StructType:
		return "!go.struct"
	case *ast.ChanType:
		return "!go.chan<" + goTypeToMLIR(t.Value) + ">"
	case *ast.Ellipsis:
		return "!go.vararg<" + goTypeToMLIR(t.Elt) + ">"
	case *ast.ParenExpr:
		return goTypeToMLIR(t.X)
	default:
		return "!go.any"
	}
}

func renderSelector(s *ast.SelectorExpr) string {
	parts := []string{sanitizeName(s.Sel.Name)}
	for {
		switch x := s.X.(type) {
		case *ast.Ident:
			parts = append([]string{sanitizeName(x.Name)}, parts...)
			return strings.Join(parts, ".")
		case *ast.SelectorExpr:
			parts = append([]string{sanitizeName(x.Sel.Name)}, parts...)
			s = x
		default:
			return strings.Join(parts, ".")
		}
	}
}

func shortNodeName(node any) string {
	name := fmt.Sprintf("%T", node)
	return strings.TrimPrefix(name, "*ast.")
}

func sanitizeName(name string) string {
	if name == "" {
		return "anon"
	}
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
