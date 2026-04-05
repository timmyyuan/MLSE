package gofrontend

import (
	"fmt"
	"go/ast"
	"strings"
)

func emitFormalLinef(node ast.Node, env *formalEnv, format string, args ...any) string {
	line := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	loc := formalLocationSuffix(node, env)
	if loc == "" {
		return line
	}
	return strings.TrimSuffix(line, "\n") + " " + loc + "\n"
}

func annotateFormalStructuredOp(text string, node ast.Node, env *formalEnv) string {
	loc := formalLocationSuffix(node, env)
	if loc == "" || text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		lines[i] += " " + loc
		break
	}
	return strings.Join(lines, "\n")
}

func formalLocationSuffix(node ast.Node, env *formalEnv) string {
	if env == nil || env.module == nil {
		return ""
	}
	actual := node
	if actual == nil {
		actual = env.currentNode
	}
	if actual == nil {
		return ""
	}
	file, line, col, ok := env.module.sourcePosition(actual)
	if !ok {
		return ""
	}
	if scope, ok := env.module.scopeForNode(actual); ok {
		return fmt.Sprintf("loc(%q(%q:%d:%d))", scope.Label, file, line, col)
	}
	return fmt.Sprintf("loc(%q:%d:%d)", file, line, col)
}
