package gofrontend

import (
	"fmt"
	"go/ast"
	"strings"
)

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

func indentBlock(text string, levels int) string {
	if text == "" {
		return ""
	}
	indent := strings.Repeat("  ", levels)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
