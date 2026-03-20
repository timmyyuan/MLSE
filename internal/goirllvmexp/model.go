package goirllvmexp

type sourceLine struct {
	number int
	text   string
}

type typedValue struct {
	name string
	ty   string
}

type valueRef struct {
	raw string
	ty  string
}

type instruction interface {
	emit(*funcEmitter) error
}

type module struct {
	funcs []*function
}

type function struct {
	name    string
	params  []typedValue
	results []string
	body    []instruction
}

type aliasInst struct {
	line int
	dest typedValue
	src  valueRef
}

type binaryInst struct {
	line int
	dest typedValue
	op   string
	lhs  valueRef
	rhs  valueRef
}

type callInst struct {
	line   int
	dest   typedValue
	callee string
	args   []valueRef
}

type returnInst struct {
	line int
	vals []valueRef
}

type exprInst struct {
	line int
	ref  valueRef
}

type branchInst struct {
	line int
	kind string
}

type storeInst struct {
	line   int
	kind   string
	target valueRef
	value  valueRef
}

type condition interface {
	emit(*funcEmitter, int) (string, error)
}

type valueCondition struct {
	ref valueRef
}

type compareCondition struct {
	op  string
	ty  string
	lhs valueRef
	rhs valueRef
}

type ifInst struct {
	line     int
	cond     condition
	thenBody []instruction
	elseBody []instruction
}

type forInst struct {
	line int
	cond condition
	body []instruction
}

type switchCase struct {
	line      int
	ty        string
	values    []valueRef
	body      []instruction
	isDefault bool
}

type switchInst struct {
	line  int
	tag   valueRef
	cases []switchCase
}

type externDecl struct {
	name   string
	base   string
	params []string
	result string
}

type localSlot struct {
	goTy   string
	llvmTy string
	ptr    string
}

type funcEmitter struct {
	signatures    map[string]*function
	externs       map[string]externDecl
	locals        map[string]localSlot
	constants     map[string]string
	lines         []string
	prologue      []string
	resultGoTys   []string
	resultLLVMTys []string
	resultTy      string
	blockSeq      int
	valueSeq      int
	controlDepth  int
	current       string
	hasCurrent    bool
	entryActive   bool
	terminated    bool
	loopStack     []loopLabels
}

type loopLabels struct {
	continueLabel string
	breakLabel    string
}
