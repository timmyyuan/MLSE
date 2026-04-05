package formalruntime

import "testing"

func TestSymbolBuilders(t *testing.T) {
	if got := AddrofSymbol("slice.string"); got != Symbol("runtime.addrof.slice.string") {
		t.Fatalf("addrof symbol mismatch: got %q", got)
	}
	if got := AddSymbol("string"); got != Symbol("runtime.add.string") {
		t.Fatalf("add symbol mismatch: got %q", got)
	}
	if got := AnyBoxSymbol("string"); got != Symbol("runtime.any.box.string") {
		t.Fatalf("any box symbol mismatch: got %q", got)
	}
	if got := ConvertSymbol("any", "bool"); got != Symbol("runtime.convert.any.to.bool") {
		t.Fatalf("convert symbol mismatch: got %q", got)
	}
	if got := EqSymbol("Result"); got != Symbol("runtime.eq.Result") {
		t.Fatalf("eq symbol mismatch: got %q", got)
	}
	if got := NeqSymbol("Result"); got != Symbol("runtime.neq.Result") {
		t.Fatalf("neq symbol mismatch: got %q", got)
	}
	if got := BinaryOpSymbol("mul", "time.Duration"); got != Symbol("runtime.bin.mul.time.Duration") {
		t.Fatalf("binary op symbol mismatch: got %q", got)
	}
	if got := MakeHelperSymbol("map"); got != Symbol("runtime.make.map") {
		t.Fatalf("make helper symbol mismatch: got %q", got)
	}
	if got := NewHelperSymbol("i64"); got != Symbol("runtime.new.i64") {
		t.Fatalf("new helper symbol mismatch: got %q", got)
	}
	if got := ZeroSymbol("Result"); got != Symbol("runtime.zero.Result") {
		t.Fatalf("zero symbol mismatch: got %q", got)
	}
	if got := DerefSymbol("ptr.i64"); got != Symbol("runtime.deref.ptr.i64") {
		t.Fatalf("deref symbol mismatch: got %q", got)
	}
	if got := TypeAssertSymbol("any", "bool"); got != Symbol("runtime.type.assert.any.to.bool") {
		t.Fatalf("type assert symbol mismatch: got %q", got)
	}
	if got := IndexSymbol("map"); got != Symbol("runtime.index.map") {
		t.Fatalf("index symbol mismatch: got %q", got)
	}
	if got := SelectorSymbol("Owner"); got != Symbol("runtime.selector.Owner") {
		t.Fatalf("selector symbol mismatch: got %q", got)
	}
	if got := RangeLenSymbol("map"); got != Symbol("runtime.range.len.map") {
		t.Fatalf("range len symbol mismatch: got %q", got)
	}
	if got := StoreIndexSymbol("map"); got != Symbol("runtime.store.index.map") {
		t.Fatalf("store index symbol mismatch: got %q", got)
	}
	if got := StoreSelectorSymbol("Owner"); got != Symbol("runtime.store.selector.Owner") {
		t.Fatalf("store selector symbol mismatch: got %q", got)
	}
	if got := StoreDerefSymbol("ptr.i64"); got != Symbol("runtime.store.deref.ptr.i64") {
		t.Fatalf("store deref symbol mismatch: got %q", got)
	}
	if got := CompositeHelperSymbol("Resp", []string{"Version"}); got != Symbol("runtime.composite.Resp.Version") {
		t.Fatalf("composite helper symbol mismatch: got %q", got)
	}
}
