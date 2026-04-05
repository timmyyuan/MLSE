package gofrontend

import (
	"go/ast"
	"testing"

	"github.com/yuanting/MLSE/internal/gofrontend/formalruntime"
	"github.com/yuanting/MLSE/internal/gofrontend/formalstdlib"
)

func TestFormalRuntimeABISymbolBuilders(t *testing.T) {
	if got := formalRuntimeAnyBoxSymbol("!go.string"); got != formalRuntimeSymbol("runtime.any.box.string") {
		t.Fatalf("any box symbol mismatch: got %q", got)
	}
	if got := formalRuntimeAddSymbol("!go.string"); got != formalRuntimeSymbol("runtime.add.string") {
		t.Fatalf("add symbol mismatch: got %q", got)
	}
	if got := formalRuntimeMakeHelperSymbol(`!go.named<"map">`); got != formalRuntimeSymbol("runtime.make.map") {
		t.Fatalf("make helper symbol mismatch: got %q", got)
	}
	if got := formalRuntimeNewHelperSymbol("!go.ptr<i64>"); got != formalRuntimeSymbol("runtime.new.i64") {
		t.Fatalf("new helper symbol mismatch: got %q", got)
	}
	if got := formalRuntimeZeroSymbol(`!go.named<"Resp">`); got != formalRuntimeSymbol("runtime.zero.Resp") {
		t.Fatalf("zero helper symbol mismatch: got %q", got)
	}
	if got := formalRuntimeTypeAssertSymbol(`!go.named<"any">`, "i1"); got != formalRuntimeSymbol("runtime.type.assert.any.to.bool") {
		t.Fatalf("type assert symbol mismatch: got %q", got)
	}
	if got := formalRuntimeRangeLenSymbol(`!go.named<"map">`); got != formalRuntimeSymbol("runtime.range.len.map") {
		t.Fatalf("range len symbol mismatch: got %q", got)
	}
	lit := &ast.CompositeLit{
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: ast.NewIdent("Version")},
		},
	}
	if got := formalRuntimeCompositeHelperSymbol(lit, `!go.named<"Resp">`); got != formalRuntimeSymbol("runtime.composite.Resp.Version") {
		t.Fatalf("composite helper symbol mismatch: got %q", got)
	}
}

func TestFormalStdlibModelsUseRuntimeABIRegistry(t *testing.T) {
	cases := map[string]formalRuntimeSymbol{
		"errors.New":         formalruntime.ErrorsNew,
		"fmt.Errorf":         formalruntime.FmtErrorf,
		"fmt.Print":          formalruntime.FmtPrint,
		"fmt.Printf":         formalruntime.FmtPrintf,
		"fmt.Println":        formalruntime.FmtPrintln,
		"fmt.Sprint":         formalruntime.FmtSprint,
		"fmt.Sprintf":        formalruntime.FmtSprintf,
		"strings.Contains":   formalruntime.StringsContains,
		"strings.Fields":     formalruntime.StringsFields,
		"strings.HasPrefix":  formalruntime.StringsHasPrefix,
		"strings.HasSuffix":  formalruntime.StringsHasSuffix,
		"strings.ReplaceAll": formalruntime.StringsReplaceAll,
		"strings.Split":      formalruntime.StringsSplit,
		"strings.ToLower":    formalruntime.StringsToLower,
		"strings.ToUpper":    formalruntime.StringsToUpper,
		"strings.TrimPrefix": formalruntime.StringsTrimPrefix,
		"strings.TrimSpace":  formalruntime.StringsTrimSpace,
		"strings.TrimSuffix": formalruntime.StringsTrimSuffix,
	}
	for key, want := range cases {
		model, ok := formalstdlib.Lookup(key)
		if !ok {
			t.Fatalf("missing stdlib model for %q", key)
		}
		if model.RuntimeSymbol != want {
			t.Fatalf("runtime symbol mismatch for %q: got %q want %q", key, model.RuntimeSymbol, want)
		}
	}
}
