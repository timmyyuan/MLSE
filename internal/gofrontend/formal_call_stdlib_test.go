package gofrontend

import (
	"go/ast"
	"path/filepath"
	"sort"
	"testing"
)

func TestFormalStdlibCallModelUsesImportPath(t *testing.T) {
	path := filepath.Join("testdata", "stdlib_alias_calls.go")
	file, funcs, typed, err := parseModule(path)
	if err != nil {
		t.Fatalf("parse module %q: %v", path, err)
	}
	module := newFormalModuleContext(file, funcs, typed)

	var got []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if _, ok := lookupFormalStdlibCallModel(call.Fun, module); !ok {
			return true
		}
		got = append(got, formalStdlibCallKey(call.Fun, module))
		return true
	})
	sort.Strings(got)

	want := []string{
		"errors.New",
		"fmt.Errorf",
		"fmt.Sprintf",
		"strings.Contains",
		"strings.ReplaceAll",
		"strings.Split",
	}
	if len(got) != len(want) {
		t.Fatalf("stdlib call keys mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stdlib call keys mismatch: got %v want %v", got, want)
		}
	}
}
