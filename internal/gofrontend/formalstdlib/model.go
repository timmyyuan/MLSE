package formalstdlib

import "github.com/yuanting/MLSE/internal/gofrontend/formalruntime"

type ResultKind uint8

const (
	ResultNone ResultKind = iota
	ResultString
	ResultError
	ResultBool
	ResultStringSlice
)

type ArgHintKind uint8

const (
	ArgHintNone ArgHintKind = iota
	ArgHintAllString
	ArgHintFirstString
	ArgHintFormat
)

type ExprKind uint8

const (
	ExprNone ExprKind = iota
	ExprRuntimeDirect
	ExprRuntimeFormat
	ExprRuntimeAnySlice
)

type StmtKind uint8

const (
	StmtNone StmtKind = iota
	StmtRuntimeAnySlice
	StmtRuntimeFormat
)

type CallModel struct {
	ResultKind    ResultKind
	ArgHintKind   ArgHintKind
	ExprKind      ExprKind
	StmtKind      StmtKind
	RuntimeSymbol formalruntime.Symbol
}

var callModels = map[string]CallModel{
	"errors.New": {
		ResultKind:    ResultError,
		ArgHintKind:   ArgHintFirstString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.ErrorsNew,
	},
	"fmt.Errorf": {
		ResultKind:    ResultError,
		ArgHintKind:   ArgHintFormat,
		ExprKind:      ExprRuntimeFormat,
		RuntimeSymbol: formalruntime.FmtErrorf,
	},
	"fmt.Print": {
		StmtKind:      StmtRuntimeAnySlice,
		RuntimeSymbol: formalruntime.FmtPrint,
	},
	"fmt.Println": {
		StmtKind:      StmtRuntimeAnySlice,
		RuntimeSymbol: formalruntime.FmtPrintln,
	},
	"fmt.Printf": {
		ArgHintKind:   ArgHintFormat,
		StmtKind:      StmtRuntimeFormat,
		RuntimeSymbol: formalruntime.FmtPrintf,
	},
	"fmt.Sprint": {
		ResultKind:    ResultString,
		ExprKind:      ExprRuntimeAnySlice,
		RuntimeSymbol: formalruntime.FmtSprint,
	},
	"fmt.Sprintf": {
		ResultKind:    ResultString,
		ArgHintKind:   ArgHintFormat,
		ExprKind:      ExprRuntimeFormat,
		RuntimeSymbol: formalruntime.FmtSprintf,
	},
	"strings.Contains": {
		ResultKind:    ResultBool,
		ArgHintKind:   ArgHintAllString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsContains,
	},
	"strings.Fields": {
		ResultKind:    ResultStringSlice,
		ArgHintKind:   ArgHintFirstString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsFields,
	},
	"strings.HasPrefix": {
		ResultKind:    ResultBool,
		ArgHintKind:   ArgHintAllString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsHasPrefix,
	},
	"strings.HasSuffix": {
		ResultKind:    ResultBool,
		ArgHintKind:   ArgHintAllString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsHasSuffix,
	},
	"strings.ReplaceAll": {
		ResultKind:    ResultString,
		ArgHintKind:   ArgHintAllString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsReplaceAll,
	},
	"strings.Split": {
		ResultKind:    ResultStringSlice,
		ArgHintKind:   ArgHintAllString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsSplit,
	},
	"strings.ToLower": {
		ResultKind:    ResultString,
		ArgHintKind:   ArgHintFirstString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsToLower,
	},
	"strings.ToUpper": {
		ResultKind:    ResultString,
		ArgHintKind:   ArgHintFirstString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsToUpper,
	},
	"strings.TrimPrefix": {
		ResultKind:    ResultString,
		ArgHintKind:   ArgHintAllString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsTrimPrefix,
	},
	"strings.TrimSpace": {
		ResultKind:    ResultString,
		ArgHintKind:   ArgHintFirstString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsTrimSpace,
	},
	"strings.TrimSuffix": {
		ResultKind:    ResultString,
		ArgHintKind:   ArgHintAllString,
		ExprKind:      ExprRuntimeDirect,
		RuntimeSymbol: formalruntime.StringsTrimSuffix,
	},
}

func Lookup(key string) (CallModel, bool) {
	model, ok := callModels[key]
	return model, ok
}
