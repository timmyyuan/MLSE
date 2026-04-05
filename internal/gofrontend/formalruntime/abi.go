package formalruntime

import "strings"

type Symbol string

func (s Symbol) String() string {
	return string(s)
}

func (s Symbol) IsZero() bool {
	return s == ""
}

const (
	PanicIndex        Symbol = "runtime.panic.index"
	GrowSlice         Symbol = "runtime.growslice"
	MakeSlice         Symbol = "runtime.makeslice"
	EqString          Symbol = "runtime.eq.string"
	NeqString         Symbol = "runtime.neq.string"
	GoLen             Symbol = "runtime.go.len"
	GoCap             Symbol = "runtime.go.cap"
	GoIndex           Symbol = "runtime.go.index"
	GoAppend          Symbol = "runtime.go.append"
	GoAppendSlice     Symbol = "runtime.go.append_slice"
	NewObject         Symbol = "runtime.newobject"
	ErrorsNew         Symbol = "runtime.errors.New"
	FmtErrorf         Symbol = "runtime.fmt.Errorf"
	FmtPrint          Symbol = "runtime.fmt.Print"
	FmtPrintf         Symbol = "runtime.fmt.Printf"
	FmtPrintln        Symbol = "runtime.fmt.Println"
	FmtSprint         Symbol = "runtime.fmt.Sprint"
	FmtSprintf        Symbol = "runtime.fmt.Sprintf"
	StringsContains   Symbol = "runtime.strings.Contains"
	StringsFields     Symbol = "runtime.strings.Fields"
	StringsHasPrefix  Symbol = "runtime.strings.HasPrefix"
	StringsHasSuffix  Symbol = "runtime.strings.HasSuffix"
	StringsReplaceAll Symbol = "runtime.strings.ReplaceAll"
	StringsSplit      Symbol = "runtime.strings.Split"
	StringsToLower    Symbol = "runtime.strings.ToLower"
	StringsToUpper    Symbol = "runtime.strings.ToUpper"
	StringsTrimPrefix Symbol = "runtime.strings.TrimPrefix"
	StringsTrimSpace  Symbol = "runtime.strings.TrimSpace"
	StringsTrimSuffix Symbol = "runtime.strings.TrimSuffix"
)

const (
	anyBoxPrefix        = "runtime.any.box."
	addrofPrefix        = "runtime.addrof."
	addPrefix           = "runtime.add."
	convertPrefix       = "runtime.convert."
	eqPrefix            = "runtime.eq."
	neqPrefix           = "runtime.neq."
	binPrefix           = "runtime.bin."
	makePrefix          = "runtime.make."
	compositePrefix     = "runtime.composite."
	newPrefix           = "runtime.new."
	zeroPrefix          = "runtime.zero."
	derefPrefix         = "runtime.deref."
	typeAssertPrefix    = "runtime.type.assert."
	indexPrefix         = "runtime.index."
	selectorPrefix      = "runtime.selector."
	rangeLenPrefix      = "runtime.range.len."
	storeIndexPrefix    = "runtime.store.index."
	storeSelectorPrefix = "runtime.store.selector."
	storeDerefPrefix    = "runtime.store.deref."
)

func AnyBoxSymbol(typeSuffix string) Symbol {
	return Symbol(anyBoxPrefix + typeSuffix)
}

func AddrofSymbol(typeSuffix string) Symbol {
	return Symbol(addrofPrefix + typeSuffix)
}

func AddSymbol(typeSuffix string) Symbol {
	return Symbol(addPrefix + typeSuffix)
}

func ConvertSymbol(fromSuffix string, toSuffix string) Symbol {
	return Symbol(convertPrefix + fromSuffix + ".to." + toSuffix)
}

func EqSymbol(typeSuffix string) Symbol {
	return Symbol(eqPrefix + typeSuffix)
}

func NeqSymbol(typeSuffix string) Symbol {
	return Symbol(neqPrefix + typeSuffix)
}

func BinaryOpSymbol(opName string, typeSuffix string) Symbol {
	return Symbol(binPrefix + opName + "." + typeSuffix)
}

func MakeHelperSymbol(typeSuffix string) Symbol {
	return Symbol(makePrefix + typeSuffix)
}

func CompositeHelperSymbol(typeSuffix string, fieldKeys []string) Symbol {
	base := compositePrefix + typeSuffix
	if len(fieldKeys) == 0 {
		return Symbol(base)
	}
	return Symbol(base + "." + strings.Join(fieldKeys, "."))
}

func NewHelperSymbol(typeSuffix string) Symbol {
	return Symbol(newPrefix + typeSuffix)
}

func ZeroSymbol(typeSuffix string) Symbol {
	return Symbol(zeroPrefix + typeSuffix)
}

func DerefSymbol(typeSuffix string) Symbol {
	return Symbol(derefPrefix + typeSuffix)
}

func TypeAssertSymbol(fromSuffix string, toSuffix string) Symbol {
	return Symbol(typeAssertPrefix + fromSuffix + ".to." + toSuffix)
}

func IndexSymbol(typeSuffix string) Symbol {
	return Symbol(indexPrefix + typeSuffix)
}

func SelectorSymbol(field string) Symbol {
	return Symbol(selectorPrefix + field)
}

func RangeLenSymbol(typeSuffix string) Symbol {
	return Symbol(rangeLenPrefix + typeSuffix)
}

func StoreIndexSymbol(typeSuffix string) Symbol {
	return Symbol(storeIndexPrefix + typeSuffix)
}

func StoreSelectorSymbol(field string) Symbol {
	return Symbol(storeSelectorPrefix + field)
}

func StoreDerefSymbol(typeSuffix string) Symbol {
	return Symbol(storeDerefPrefix + typeSuffix)
}
