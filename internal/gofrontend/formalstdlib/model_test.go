package formalstdlib

import (
	"testing"

	"github.com/yuanting/MLSE/internal/gofrontend/formalruntime"
)

func TestRegistryUsesRuntimeABI(t *testing.T) {
	cases := map[string]formalruntime.Symbol{
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
		model, ok := Lookup(key)
		if !ok {
			t.Fatalf("missing stdlib model for %q", key)
		}
		if model.RuntimeSymbol != want {
			t.Fatalf("runtime symbol mismatch for %q: got %q want %q", key, model.RuntimeSymbol, want)
		}
	}
}
