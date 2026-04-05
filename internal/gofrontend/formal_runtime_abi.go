package gofrontend

import (
	"go/ast"

	"github.com/yuanting/MLSE/internal/gofrontend/formalruntime"
)

type formalRuntimeSymbol = formalruntime.Symbol

const (
	formalRuntimeSymbolPanicIndex        formalRuntimeSymbol = formalruntime.PanicIndex
	formalRuntimeSymbolGrowSlice         formalRuntimeSymbol = formalruntime.GrowSlice
	formalRuntimeSymbolMakeSlice         formalRuntimeSymbol = formalruntime.MakeSlice
	formalRuntimeSymbolEqString          formalRuntimeSymbol = formalruntime.EqString
	formalRuntimeSymbolNeqString         formalRuntimeSymbol = formalruntime.NeqString
	formalRuntimeSymbolGoLen             formalRuntimeSymbol = formalruntime.GoLen
	formalRuntimeSymbolGoCap             formalRuntimeSymbol = formalruntime.GoCap
	formalRuntimeSymbolGoIndex           formalRuntimeSymbol = formalruntime.GoIndex
	formalRuntimeSymbolGoAppend          formalRuntimeSymbol = formalruntime.GoAppend
	formalRuntimeSymbolGoAppendSlice     formalRuntimeSymbol = formalruntime.GoAppendSlice
	formalRuntimeSymbolNewObject         formalRuntimeSymbol = formalruntime.NewObject
	formalRuntimeSymbolErrorsNew         formalRuntimeSymbol = formalruntime.ErrorsNew
	formalRuntimeSymbolFmtErrorf         formalRuntimeSymbol = formalruntime.FmtErrorf
	formalRuntimeSymbolFmtPrint          formalRuntimeSymbol = formalruntime.FmtPrint
	formalRuntimeSymbolFmtPrintf         formalRuntimeSymbol = formalruntime.FmtPrintf
	formalRuntimeSymbolFmtPrintln        formalRuntimeSymbol = formalruntime.FmtPrintln
	formalRuntimeSymbolFmtSprint         formalRuntimeSymbol = formalruntime.FmtSprint
	formalRuntimeSymbolFmtSprintf        formalRuntimeSymbol = formalruntime.FmtSprintf
	formalRuntimeSymbolStringsContains   formalRuntimeSymbol = formalruntime.StringsContains
	formalRuntimeSymbolStringsFields     formalRuntimeSymbol = formalruntime.StringsFields
	formalRuntimeSymbolStringsHasPrefix  formalRuntimeSymbol = formalruntime.StringsHasPrefix
	formalRuntimeSymbolStringsHasSuffix  formalRuntimeSymbol = formalruntime.StringsHasSuffix
	formalRuntimeSymbolStringsReplaceAll formalRuntimeSymbol = formalruntime.StringsReplaceAll
	formalRuntimeSymbolStringsSplit      formalRuntimeSymbol = formalruntime.StringsSplit
	formalRuntimeSymbolStringsToLower    formalRuntimeSymbol = formalruntime.StringsToLower
	formalRuntimeSymbolStringsToUpper    formalRuntimeSymbol = formalruntime.StringsToUpper
	formalRuntimeSymbolStringsTrimPrefix formalRuntimeSymbol = formalruntime.StringsTrimPrefix
	formalRuntimeSymbolStringsTrimSpace  formalRuntimeSymbol = formalruntime.StringsTrimSpace
	formalRuntimeSymbolStringsTrimSuffix formalRuntimeSymbol = formalruntime.StringsTrimSuffix
)

func formalRuntimeAnyBoxSymbol(valueTy string) formalRuntimeSymbol {
	return formalruntime.AnyBoxSymbol(formalTypeHelperSuffix(valueTy))
}

func formalRuntimeAddrofSymbol(valueTy string) formalRuntimeSymbol {
	return formalruntime.AddrofSymbol(formalTypeHelperSuffix(valueTy))
}

func formalRuntimeAddSymbol(valueTy string) formalRuntimeSymbol {
	return formalruntime.AddSymbol(formalTypeHelperSuffix(valueTy))
}

func formalRuntimeConvertSymbol(valueTy string, targetTy string) formalRuntimeSymbol {
	return formalruntime.ConvertSymbol(formalTypeHelperSuffix(valueTy), formalTypeHelperSuffix(targetTy))
}

func formalRuntimeEqSymbol(valueTy string) formalRuntimeSymbol {
	return formalruntime.EqSymbol(formalTypeHelperSuffix(valueTy))
}

func formalRuntimeNeqSymbol(valueTy string) formalRuntimeSymbol {
	return formalruntime.NeqSymbol(formalTypeHelperSuffix(valueTy))
}

func formalRuntimeBinaryOpSymbol(opName string, valueTy string) formalRuntimeSymbol {
	return formalruntime.BinaryOpSymbol(opName, formalTypeHelperSuffix(valueTy))
}

func formalRuntimeMakeHelperSymbol(targetTy string) formalRuntimeSymbol {
	return formalruntime.MakeHelperSymbol(formalTypeHelperSuffix(targetTy))
}

func formalRuntimeCompositeHelperSymbol(lit *ast.CompositeLit, targetTy string) formalRuntimeSymbol {
	return formalruntime.CompositeHelperSymbol(formalTypeHelperSuffix(targetTy), formalCompositeHelperKeys(lit))
}

func formalRuntimeNewHelperSymbol(resultTy string) formalRuntimeSymbol {
	suffix := formalTypeHelperSuffix(resultTy)
	if isFormalPointerType(resultTy) {
		suffix = formalTypeHelperSuffix(formalDerefType(resultTy))
	}
	return formalruntime.NewHelperSymbol(suffix)
}

func formalRuntimeZeroSymbol(valueTy string) formalRuntimeSymbol {
	return formalruntime.ZeroSymbol(formalTypeHelperSuffix(valueTy))
}

func formalRuntimeDerefSymbol(ptrTy string) formalRuntimeSymbol {
	return formalruntime.DerefSymbol(formalTypeHelperSuffix(ptrTy))
}

func formalRuntimeTypeAssertSymbol(valueTy string, targetTy string) formalRuntimeSymbol {
	return formalruntime.TypeAssertSymbol(formalTypeHelperSuffix(valueTy), formalTypeHelperSuffix(targetTy))
}

func formalRuntimeIndexSymbol(sourceTy string) formalRuntimeSymbol {
	return formalruntime.IndexSymbol(formalTypeHelperSuffix(sourceTy))
}

func formalRuntimeSelectorSymbol(field string) formalRuntimeSymbol {
	return formalruntime.SelectorSymbol(sanitizeName(field))
}

func formalRuntimeRangeLenSymbol(sourceTy string) formalRuntimeSymbol {
	return formalruntime.RangeLenSymbol(formalTypeHelperSuffix(sourceTy))
}

func formalRuntimeStoreIndexSymbol(sourceTy string) formalRuntimeSymbol {
	return formalruntime.StoreIndexSymbol(formalTypeHelperSuffix(sourceTy))
}

func formalRuntimeStoreSelectorSymbol(field string) formalRuntimeSymbol {
	return formalruntime.StoreSelectorSymbol(sanitizeName(field))
}

func formalRuntimeStoreDerefSymbol(ptrTy string) formalRuntimeSymbol {
	return formalruntime.StoreDerefSymbol(formalTypeHelperSuffix(ptrTy))
}

func formalCompositeHelperKeys(lit *ast.CompositeLit) []string {
	if lit == nil {
		return nil
	}
	keys := make([]string, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		switch key := kv.Key.(type) {
		case *ast.Ident:
			keys = append(keys, sanitizeName(key.Name))
		case *ast.SelectorExpr:
			keys = append(keys, sanitizeName(renderSelector(key)))
		}
	}
	return keys
}
