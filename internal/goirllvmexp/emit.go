package goirllvmexp

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

func emitLLVMDialectModule(mod *module, opts LoweringOptions) (string, error) {
	signatures := make(map[string]*function, len(mod.funcs))
	for _, fn := range mod.funcs {
		signatures[fn.name] = fn
	}

	var defs []string
	externs := map[string]externDecl{}
	stringGlobals := map[string]stringGlobalDecl{}
	for _, fn := range mod.funcs {
		text, fnExterns, fnStrings, err := emitLLVMDialectFunction(fn, signatures, opts)
		if err != nil {
			return "", err
		}
		defs = append(defs, text)
		for name, decl := range fnExterns {
			if _, ok := signatures[name]; ok {
				continue
			}
			if existing, ok := externs[name]; ok {
				if !sameSignature(existing, decl) {
					return "", fmt.Errorf("conflicting external signatures for %s", name)
				}
				continue
			}
			externs[name] = decl
		}
		for name, decl := range fnStrings {
			stringGlobals[name] = decl
		}
	}

	var b strings.Builder
	b.WriteString("module {\n")
	if len(stringGlobals) > 0 {
		names := make([]string, 0, len(stringGlobals))
		for name := range stringGlobals {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			decl := stringGlobals[name]
			b.WriteString(fmt.Sprintf("  llvm.mlir.global internal constant @%s(%s) {addr_space = 0 : i32}\n", decl.name, decl.encoded))
		}
		if len(externs) > 0 || len(defs) > 0 {
			b.WriteString("\n")
		}
	}
	if len(externs) > 0 {
		names := make([]string, 0, len(externs))
		for name := range externs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			decl := externs[name]
			b.WriteString(formatExternDecl(decl))
		}
		if len(defs) > 0 {
			b.WriteString("\n")
		}
	}
	for i, def := range defs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(def)
	}
	b.WriteString("}\n")
	return b.String(), nil
}

func formatExternDecl(decl externDecl) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  llvm.func @%s(%s)", decl.name, strings.Join(decl.params, ", ")))
	if decl.result != "void" {
		b.WriteString(" -> ")
		b.WriteString(decl.result)
	}
	b.WriteString("\n")
	return b.String()
}

func emitLLVMDialectFunction(fn *function, signatures map[string]*function, opts LoweringOptions) (string, map[string]externDecl, map[string]stringGlobalDecl, error) {
	resultGoTys := append([]string(nil), fn.results...)
	resultLLVMTys := make([]string, 0, len(fn.results))
	for _, goTy := range fn.results {
		resultLLVMTys = append(resultLLVMTys, mustLLVMTypeWithOptions(goTy, opts))
	}
	emitter := &funcEmitter{
		signatures:    signatures,
		externs:       map[string]externDecl{},
		locals:        map[string]localSlot{},
		constants:     map[string]string{},
		options:       opts,
		sliceLayout:   newSliceLayout(opts.SliceModel),
		resultGoTys:   resultGoTys,
		resultLLVMTys: resultLLVMTys,
		resultTy:      llvmFunctionResultType(resultLLVMTys),
		stringGlobals: map[string]stringGlobalDecl{},
	}

	paramDefs := make([]string, 0, len(fn.params))
	for _, param := range fn.params {
		llvmTy := mustLLVMTypeWithOptions(param.ty, opts)
		paramDefs = append(paramDefs, fmt.Sprintf("%s: %s", param.name, llvmTy))
		if err := emitter.bindParam(param); err != nil {
			return "", nil, nil, err
		}
	}

	emitter.startEntryBlock()
	for _, inst := range fn.body {
		if err := inst.emit(emitter); err != nil {
			return "", nil, nil, err
		}
	}

	switch {
	case !emitter.hasCurrent:
	case emitter.terminated:
	case emitter.resultTy == "void":
		emitter.emitTerminator("llvm.return")
	default:
		if err := emitter.emitImplicitZeroReturn(); err != nil {
			return "", nil, nil, fmt.Errorf("function %s falls off the end without returning %s: %v", fn.name, emitter.resultTy, err)
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("  llvm.func @%s(%s)", fn.name, strings.Join(paramDefs, ", ")))
	if emitter.resultTy != "void" {
		b.WriteString(" -> ")
		b.WriteString(emitter.resultTy)
	}
	b.WriteString(" {\n")
	for _, line := range emitter.prologue {
		b.WriteString("    ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	for _, line := range emitter.lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("  }\n")
	return b.String(), emitter.externs, emitter.stringGlobals, nil
}

func (e *funcEmitter) llvmType(goTy string) string {
	return mustLLVMTypeWithOptions(goTy, e.options)
}

func (i *aliasInst) emit(e *funcEmitter) error {
	storeGoTy := e.preferredLocalGoType(i.dest.name, i.dest.ty)
	value, llvmTy, err := e.resolveTyped(i.src, storeGoTy)
	if err != nil {
		return fmt.Errorf("line %d: %v", i.line, err)
	}
	if i.dest.name == "%_" {
		return nil
	}
	return e.storeLocal(i.dest.name, storeGoTy, llvmTy, value)
}

func (i *binaryInst) emit(e *funcEmitter) error {
	llvmTy := e.llvmType(i.dest.ty)
	lhs, _, err := e.resolveTyped(i.lhs, i.dest.ty)
	if err != nil {
		return fmt.Errorf("line %d: %v", i.line, err)
	}
	rhs, _, err := e.resolveTyped(i.rhs, i.dest.ty)
	if err != nil {
		return fmt.Errorf("line %d: %v", i.line, err)
	}

	destValue := e.freshValue("tmp")
	resultTy := llvmTy

	if isPointerLLVMType(llvmTy) {
		switch i.op {
		case "arith.addi", "arith.subi", "arith.muli", "arith.divsi":
			zero, err := e.materializeZero(llvmTy)
			if err != nil {
				return fmt.Errorf("line %d: %v", i.line, err)
			}
			return e.storeLocal(i.dest.name, i.dest.ty, llvmTy, zero)
		}
	}

	var emitted string
	switch i.op {
	case "arith.addi":
		emitted = fmt.Sprintf("%s = llvm.add %s, %s : %s", destValue, lhs, rhs, llvmTy)
	case "arith.subi":
		emitted = fmt.Sprintf("%s = llvm.sub %s, %s : %s", destValue, lhs, rhs, llvmTy)
	case "arith.muli":
		emitted = fmt.Sprintf("%s = llvm.mul %s, %s : %s", destValue, lhs, rhs, llvmTy)
	case "arith.divsi":
		emitted = fmt.Sprintf("%s = llvm.sdiv %s, %s : %s", destValue, lhs, rhs, llvmTy)
	case "arith.cmpi_eq", "arith.cmpi_ne", "arith.cmpi_gt", "arith.cmpi_lt", "arith.cmpi_ge", "arith.cmpi_le":
		emitted, resultTy, err = emitCompareInst(i.op, destValue, llvmTy, lhs, rhs)
	default:
		return fmt.Errorf("line %d: unsupported arithmetic op %q", i.line, i.op)
	}
	if err != nil {
		return fmt.Errorf("line %d: %v", i.line, err)
	}

	e.emitInstruction(emitted)
	return e.storeLocal(i.dest.name, reverseLLVMType(resultTy), resultTy, destValue)
}

func (i *callInst) emit(e *funcEmitter) error {
	if handled, err := i.emitBuiltin(e); handled || err != nil {
		return err
	}

	retTy := e.llvmType(i.dest.ty)
	argValues := make([]string, 0, len(i.args))
	argTys := make([]string, 0, len(i.args))
	for _, arg := range i.args {
		goTy := arg.ty
		if goTy == "" {
			goTy = e.typeOfValue(arg.raw)
		}
		llvmTy := e.llvmType(goTy)
		value, actualTy, err := e.resolveTyped(arg, goTy)
		if err != nil {
			return fmt.Errorf("line %d: %v", i.line, err)
		}
		if actualTy != llvmTy {
			return fmt.Errorf("line %d: call arg %q has LLVM type %s, expected %s", i.line, arg.raw, actualTy, llvmTy)
		}
		argValues = append(argValues, value)
		argTys = append(argTys, llvmTy)
	}

	callee := i.callee
	if fn, ok := e.signatures[i.callee]; ok {
		matches := len(fn.params) == len(argTys) && len(fn.results) <= 1
		if matches {
			switch len(fn.results) {
			case 0:
				matches = retTy == "void"
			case 1:
				matches = e.llvmType(fn.results[0]) == retTy
			}
		}
		if matches {
			for idx, param := range fn.params {
				if e.llvmType(param.ty) != argTys[idx] {
					matches = false
					break
				}
			}
		}
		if !matches {
			callee = e.resolveExternSymbol(i.callee, argTys, retTy)
		}
	} else {
		callee = e.resolveExternSymbol(i.callee, argTys, retTy)
	}

	callText := fmt.Sprintf("llvm.call @%s(%s) : (%s) -> %s", callee, strings.Join(argValues, ", "), strings.Join(argTys, ", "), llvmCallResultText(retTy))
	if retTy == "void" {
		e.emitInstruction(callText)
		return nil
	}

	callTmp := e.freshValue("call")
	e.emitInstruction(fmt.Sprintf("%s = %s", callTmp, callText))
	return e.storeLocal(i.dest.name, i.dest.ty, retTy, callTmp)
}

func (i *callInst) emitBuiltin(e *funcEmitter) (bool, error) {
	switch i.callee {
	case "len":
		return i.emitBuiltinLen(e)
	case "cap":
		return i.emitBuiltinCap(e)
	default:
		return false, nil
	}
}

func (i *callInst) emitBuiltinLen(e *funcEmitter) (bool, error) {
	if len(i.args) != 1 {
		return false, nil
	}

	argGoTy := i.args[0].ty
	if argGoTy == "" {
		argGoTy = e.typeOfValue(i.args[0].raw)
	}
	if !isGoSliceType(argGoTy) {
		return false, nil
	}

	value, llvmTy, err := e.resolveTyped(i.args[0], argGoTy)
	if err != nil {
		return true, fmt.Errorf("line %d: %v", i.line, err)
	}
	if llvmTy != e.sliceLayout.llvmType() {
		return true, fmt.Errorf("line %d: len expects slice value, got %s", i.line, llvmTy)
	}

	length, err := e.extractSliceLength(value, llvmTy)
	if err != nil {
		return true, fmt.Errorf("line %d: %v", i.line, err)
	}
	if i.dest.name == "%_" {
		return true, nil
	}
	if err := e.storeLocal(i.dest.name, i.dest.ty, "i32", length); err != nil {
		return true, err
	}
	return true, nil
}

func (i *callInst) emitBuiltinCap(e *funcEmitter) (bool, error) {
	if !e.sliceLayout.hasCap() || len(i.args) != 1 {
		return false, nil
	}

	argGoTy := i.args[0].ty
	if argGoTy == "" {
		argGoTy = e.typeOfValue(i.args[0].raw)
	}
	if !isGoSliceType(argGoTy) {
		return false, nil
	}

	value, llvmTy, err := e.resolveTyped(i.args[0], argGoTy)
	if err != nil {
		return true, fmt.Errorf("line %d: %v", i.line, err)
	}
	if llvmTy != e.sliceLayout.llvmType() {
		return true, fmt.Errorf("line %d: cap expects slice value, got %s", i.line, llvmTy)
	}

	capacity, err := e.extractSliceCapacity(value, llvmTy)
	if err != nil {
		return true, fmt.Errorf("line %d: %v", i.line, err)
	}
	if i.dest.name == "%_" {
		return true, nil
	}
	if err := e.storeLocal(i.dest.name, i.dest.ty, "i32", capacity); err != nil {
		return true, err
	}
	return true, nil
}

func (i *returnInst) emit(e *funcEmitter) error {
	if len(i.vals) == 0 {
		if e.resultTy != "void" {
			return fmt.Errorf("line %d: function must return %s", i.line, e.resultTy)
		}
		e.emitTerminator("llvm.return")
		return nil
	}

	if len(i.vals) != len(e.resultGoTys) {
		return fmt.Errorf("line %d: return arity mismatch: got %d values, expected %d", i.line, len(i.vals), len(e.resultGoTys))
	}
	if len(i.vals) == 1 {
		value, llvmTy, err := e.resolveTyped(i.vals[0], e.resultGoTys[0])
		if err != nil {
			return fmt.Errorf("line %d: %v", i.line, err)
		}
		if llvmTy != e.resultTy {
			return fmt.Errorf("line %d: return value has LLVM type %s, expected %s", i.line, llvmTy, e.resultTy)
		}
		e.emitTerminator(fmt.Sprintf("llvm.return %s : %s", value, e.resultTy))
		return nil
	}

	aggregate := e.freshValue("retagg")
	e.emitInstruction(fmt.Sprintf("%s = llvm.mlir.undef : %s", aggregate, e.resultTy))
	for idx, ref := range i.vals {
		value, llvmTy, err := e.resolveTyped(ref, e.resultGoTys[idx])
		if err != nil {
			return fmt.Errorf("line %d: %v", i.line, err)
		}
		if llvmTy != e.resultLLVMTys[idx] {
			return fmt.Errorf("line %d: return value %d has LLVM type %s, expected %s", i.line, idx, llvmTy, e.resultLLVMTys[idx])
		}
		next := e.freshValue("retagg")
		e.emitInstruction(fmt.Sprintf("%s = llvm.insertvalue %s, %s[%d] : %s", next, value, aggregate, idx, e.resultTy))
		aggregate = next
	}
	e.emitTerminator(fmt.Sprintf("llvm.return %s : %s", aggregate, e.resultTy))
	return nil
}

func (i *exprInst) emit(e *funcEmitter) error {
	_, _, err := e.resolveTyped(i.ref, i.ref.ty)
	return err
}

func (i *labelInst) emit(e *funcEmitter) error {
	if e.hasCurrent && !e.terminated {
		e.emitTerminator(fmt.Sprintf("llvm.br ^%s", i.label))
	}
	e.startBlock(i.label)
	return nil
}

func (i *branchInst) emit(e *funcEmitter) error {
	switch i.kind {
	case "continue":
		if len(e.loopStack) == 0 {
			return fmt.Errorf("line %d: %s used outside loop", i.line, i.kind)
		}
		labels := e.loopStack[len(e.loopStack)-1]
		e.emitTerminator(fmt.Sprintf("llvm.br ^%s", labels.continueLabel))
		return nil
	case "break":
		if len(e.loopStack) == 0 {
			return fmt.Errorf("line %d: %s used outside loop", i.line, i.kind)
		}
		labels := e.loopStack[len(e.loopStack)-1]
		e.emitTerminator(fmt.Sprintf("llvm.br ^%s", labels.breakLabel))
		return nil
	case "goto":
		if i.label == "" {
			return fmt.Errorf("line %d: goto without label", i.line)
		}
		e.emitTerminator(fmt.Sprintf("llvm.br ^%s", i.label))
		return nil
	default:
		return fmt.Errorf("line %d: unsupported branch %q", i.line, i.kind)
	}
}

func (i *incDecInst) emit(e *funcEmitter) error {
	storeGoTy := i.target.ty
	if storeGoTy == "" {
		storeGoTy = e.typeOfValue(i.target.raw)
	}

	value, llvmTy, err := e.resolveTyped(i.target, storeGoTy)
	if err != nil {
		return fmt.Errorf("line %d: %v", i.line, err)
	}

	if !strings.HasPrefix(i.target.raw, "%") || !isIntegerLLVMType(llvmTy) {
		return nil
	}

	one, err := e.materializeLiteral("1", llvmTy)
	if err != nil {
		return fmt.Errorf("line %d: %v", i.line, err)
	}
	tmp := e.freshValue("incdec")
	op := "llvm.add"
	if i.op == "mlse.--" {
		op = "llvm.sub"
	}
	e.emitInstruction(fmt.Sprintf("%s = %s %s, %s : %s", tmp, op, value, one, llvmTy))
	return e.storeLocal(i.target.raw, storeGoTy, llvmTy, tmp)
}

func (i *storeInst) emit(e *funcEmitter) error {
	_, _, err := e.resolveTyped(i.value, i.value.ty)
	return err
}

func (c *valueCondition) emit(e *funcEmitter, line int) (string, error) {
	value, llvmTy, err := e.resolveTyped(c.ref, c.ref.ty)
	if err != nil {
		return "", fmt.Errorf("line %d: %v", line, err)
	}
	return e.emitTruthiness(value, llvmTy, line)
}

func (c *compareCondition) emit(e *funcEmitter, line int) (string, error) {
	llvmTy := e.llvmType(c.ty)
	if !isIntegerLLVMType(llvmTy) && !isPointerLLVMType(llvmTy) {
		return "", fmt.Errorf("line %d: unsupported compare condition type %q", line, c.ty)
	}
	lhs, _, err := e.resolveTyped(c.lhs, c.ty)
	if err != nil {
		return "", fmt.Errorf("line %d: %v", line, err)
	}
	rhs, _, err := e.resolveTyped(c.rhs, c.ty)
	if err != nil {
		return "", fmt.Errorf("line %d: %v", line, err)
	}
	name := e.freshValue("ifcond")
	inst, _, err := emitCompareInst(c.op, name, llvmTy, lhs, rhs)
	if err != nil {
		return "", fmt.Errorf("line %d: %v", line, err)
	}
	e.emitInstruction(inst)
	return name, nil
}

func (i *ifInst) emit(e *funcEmitter) error {
	cond, err := i.cond.emit(e, i.line)
	if err != nil {
		return err
	}

	thenLabel := e.freshBlock("if.then")
	mergeLabel := ""
	falseLabel := ""
	if len(i.elseBody) > 0 {
		falseLabel = e.freshBlock("if.else")
	} else {
		mergeLabel = e.freshBlock("if.end")
		falseLabel = mergeLabel
	}

	e.emitTerminator(fmt.Sprintf("llvm.cond_br %s, ^%s, ^%s", cond, thenLabel, falseLabel))

	e.startBlock(thenLabel)
	if err := e.emitControlBody(i.thenBody); err != nil {
		return err
	}
	thenFallsThrough := e.hasCurrent && !e.terminated
	if thenFallsThrough {
		if mergeLabel == "" {
			mergeLabel = e.freshBlock("if.end")
		}
		e.emitTerminator(fmt.Sprintf("llvm.br ^%s", mergeLabel))
	}

	if len(i.elseBody) > 0 {
		e.startBlock(falseLabel)
		if err := e.emitControlBody(i.elseBody); err != nil {
			return err
		}
		elseFallsThrough := e.hasCurrent && !e.terminated
		if elseFallsThrough {
			if mergeLabel == "" {
				mergeLabel = e.freshBlock("if.end")
			}
			e.emitTerminator(fmt.Sprintf("llvm.br ^%s", mergeLabel))
		}
	}

	if mergeLabel != "" {
		e.startBlock(mergeLabel)
		return nil
	}

	e.hasCurrent = false
	e.entryActive = false
	e.current = ""
	e.terminated = true
	return nil
}

func (i *forInst) emit(e *funcEmitter) error {
	condLabel := e.freshBlock("for.cond")
	bodyLabel := e.freshBlock("for.body")
	endLabel := e.freshBlock("for.end")

	e.emitTerminator(fmt.Sprintf("llvm.br ^%s", condLabel))

	e.startBlock(condLabel)
	cond, err := i.cond.emit(e, i.line)
	if err != nil {
		return err
	}
	e.emitTerminator(fmt.Sprintf("llvm.cond_br %s, ^%s, ^%s", cond, bodyLabel, endLabel))

	e.startBlock(bodyLabel)
	e.loopStack = append(e.loopStack, loopLabels{continueLabel: condLabel, breakLabel: endLabel})
	if err := e.emitControlBody(i.body); err != nil {
		e.loopStack = e.loopStack[:len(e.loopStack)-1]
		return err
	}
	e.loopStack = e.loopStack[:len(e.loopStack)-1]
	if e.hasCurrent && !e.terminated {
		e.emitTerminator(fmt.Sprintf("llvm.br ^%s", condLabel))
	}

	e.startBlock(endLabel)
	return nil
}

func (i *switchInst) emit(e *funcEmitter) error {
	tagValue, tagLLVM, err := e.resolveTyped(i.tag, i.tag.ty)
	if err != nil {
		return fmt.Errorf("line %d: %v", i.line, err)
	}
	if !isIntegerLLVMType(tagLLVM) {
		return fmt.Errorf("line %d: unsupported switch tag type %q", i.line, i.tag.ty)
	}

	defaultCase := -1
	caseIndices := make([]int, 0, len(i.cases))
	for idx, switchCase := range i.cases {
		if switchCase.isDefault {
			if defaultCase >= 0 {
				return fmt.Errorf("line %d: multiple default cases are not supported", switchCase.line)
			}
			defaultCase = idx
			continue
		}
		if len(switchCase.values) != 1 {
			return fmt.Errorf("line %d: only single-value switch cases are supported", switchCase.line)
		}
		if e.llvmType(switchCase.ty) != tagLLVM {
			return fmt.Errorf("line %d: switch case type %q does not match tag type %q", switchCase.line, switchCase.ty, i.tag.ty)
		}
		caseIndices = append(caseIndices, idx)
	}

	endLabel := e.freshBlock("switch.end")
	defaultLabel := endLabel
	needsEnd := defaultCase < 0
	if defaultCase >= 0 {
		defaultLabel = e.freshBlock("switch.default")
	}

	checkLabels := make([]string, len(caseIndices))
	bodyLabels := make([]string, len(caseIndices))
	for idx := range caseIndices {
		checkLabels[idx] = e.freshBlock("switch.case.check")
		bodyLabels[idx] = e.freshBlock("switch.case.body")
	}

	if len(checkLabels) > 0 {
		e.emitTerminator(fmt.Sprintf("llvm.br ^%s", checkLabels[0]))
	} else {
		e.emitTerminator(fmt.Sprintf("llvm.br ^%s", defaultLabel))
	}

	for idx, caseIdx := range caseIndices {
		switchCase := i.cases[caseIdx]
		falseLabel := defaultLabel
		if idx+1 < len(checkLabels) {
			falseLabel = checkLabels[idx+1]
		}

		e.startBlock(checkLabels[idx])
		caseValue, caseLLVM, err := e.resolveTyped(switchCase.values[0], switchCase.ty)
		if err != nil {
			return fmt.Errorf("line %d: %v", switchCase.line, err)
		}
		if caseLLVM != tagLLVM {
			return fmt.Errorf("line %d: switch case value has LLVM type %s, expected %s", switchCase.line, caseLLVM, tagLLVM)
		}
		cmp := e.freshValue("switchcmp")
		e.emitInstruction(fmt.Sprintf("%s = llvm.icmp %q %s, %s : %s", cmp, "eq", tagValue, caseValue, tagLLVM))
		e.emitTerminator(fmt.Sprintf("llvm.cond_br %s, ^%s, ^%s", cmp, bodyLabels[idx], falseLabel))

		e.startBlock(bodyLabels[idx])
		if err := e.emitControlBody(switchCase.body); err != nil {
			return err
		}
		if e.hasCurrent && !e.terminated {
			needsEnd = true
			e.emitTerminator(fmt.Sprintf("llvm.br ^%s", endLabel))
		}
	}

	if defaultCase >= 0 {
		e.startBlock(defaultLabel)
		if err := e.emitControlBody(i.cases[defaultCase].body); err != nil {
			return err
		}
		if e.hasCurrent && !e.terminated {
			needsEnd = true
			e.emitTerminator(fmt.Sprintf("llvm.br ^%s", endLabel))
		}
	}

	if needsEnd {
		e.startBlock(endLabel)
		return nil
	}

	e.hasCurrent = false
	e.entryActive = false
	e.current = ""
	e.terminated = true
	return nil
}

func (e *funcEmitter) emitControlBody(body []instruction) error {
	e.controlDepth++
	defer func() {
		e.controlDepth--
	}()
	for _, inst := range body {
		if err := inst.emit(e); err != nil {
			return err
		}
	}
	return nil
}

func (e *funcEmitter) bindParam(param typedValue) error {
	llvmTy := e.llvmType(param.ty)
	slot, err := e.ensureLocal(param.name, param.ty, llvmTy)
	if err != nil {
		return err
	}
	align := alignmentForLLVMType(llvmTy)
	e.prologue = append(e.prologue, fmt.Sprintf("llvm.store %s, %s {alignment = %d : i64} : %s, !llvm.ptr", param.name, slot.ptr, align, llvmTy))
	return nil
}

func (e *funcEmitter) storeLocal(name string, goTy string, llvmTy string, value string) error {
	slot, err := e.ensureLocal(name, goTy, llvmTy)
	if err != nil {
		return err
	}
	if slot.llvmTy != llvmTy {
		value, err = e.materializeZero(slot.llvmTy)
		if err != nil {
			return err
		}
		llvmTy = slot.llvmTy
	}
	align := alignmentForLLVMType(llvmTy)
	e.emitInstruction(fmt.Sprintf("llvm.store %s, %s {alignment = %d : i64} : %s, !llvm.ptr", value, slot.ptr, align, llvmTy))
	return nil
}

func (e *funcEmitter) ensureLocal(name string, goTy string, llvmTy string) (localSlot, error) {
	if llvmTy == "void" {
		return localSlot{}, fmt.Errorf("cannot materialize local %s with void type", name)
	}
	if slot, ok := e.locals[name]; ok {
		return slot, nil
	}
	count, err := e.materializeLiteral("1", "i32")
	if err != nil {
		return localSlot{}, err
	}
	slot := localSlot{
		goTy:   goTy,
		llvmTy: llvmTy,
		ptr:    e.freshValue("slot"),
	}
	e.locals[name] = slot
	e.prologue = append(e.prologue, fmt.Sprintf("%s = llvm.alloca %s x %s {alignment = %d : i64} : (i32) -> !llvm.ptr", slot.ptr, count, llvmTy, alignmentForLLVMType(llvmTy)))
	return slot, nil
}

func (e *funcEmitter) startEntryBlock() {
	e.hasCurrent = true
	e.entryActive = true
	e.current = "entry"
	e.terminated = false
}

func (e *funcEmitter) startBlock(label string) {
	e.lines = append(e.lines, "  ^"+label+":")
	e.hasCurrent = true
	e.entryActive = false
	e.current = label
	e.terminated = false
}

func (e *funcEmitter) ensureBlock() {
	if !e.hasCurrent || e.terminated {
		e.startBlock(e.freshBlock("dead"))
	}
}

func (e *funcEmitter) emitInstruction(line string) {
	e.ensureBlock()
	e.lines = append(e.lines, "    "+line)
}

func (e *funcEmitter) emitTerminator(line string) {
	e.ensureBlock()
	e.lines = append(e.lines, "    "+line)
	e.terminated = true
}

func (e *funcEmitter) freshBlock(prefix string) string {
	name := fmt.Sprintf("%s%d", prefix, e.blockSeq)
	e.blockSeq++
	return name
}

func (e *funcEmitter) freshValue(prefix string) string {
	name := fmt.Sprintf("%%%s%d", prefix, e.valueSeq)
	e.valueSeq++
	return name
}

func (e *funcEmitter) materializeZero(llvmTy string) (string, error) {
	if llvmTy == "void" {
		return "", errors.New("cannot materialize zero for void")
	}
	key := "zero:" + llvmTy
	if existing, ok := e.constants[key]; ok {
		return existing, nil
	}
	name := e.freshValue("zero")
	e.constants[key] = name
	e.prologue = append(e.prologue, fmt.Sprintf("%s = llvm.mlir.zero : %s", name, llvmTy))
	return name, nil
}

func (e *funcEmitter) materializeLiteral(raw string, llvmTy string) (string, error) {
	if llvmTy == "void" {
		return "", errors.New("cannot materialize literal for void")
	}
	switch {
	case raw == "false", raw == "0", raw == "mlse.nil":
		return e.materializeZero(llvmTy)
	case isQuotedStringLiteral(raw):
		if llvmTy == "!llvm.ptr" {
			return e.materializeStringLiteral(raw)
		}
		return e.materializeZero(llvmTy)
	}

	normalizedRaw := raw
	if isIntegerLiteral(raw) && !isPointerLLVMType(llvmTy) {
		normalized, err := normalizeIntegerLiteral(raw, llvmTy)
		if err != nil {
			return "", err
		}
		normalizedRaw = normalized
	}

	key := "lit:" + llvmTy + ":" + normalizedRaw
	if existing, ok := e.constants[key]; ok {
		return existing, nil
	}

	name := e.freshValue("c")
	e.constants[key] = name

	switch {
	case raw == "true":
		e.prologue = append(e.prologue, fmt.Sprintf("%s = llvm.mlir.constant(true) : i1", name))
	case isIntegerLiteral(raw):
		if isPointerLLVMType(llvmTy) {
			delete(e.constants, key)
			return e.materializeZero(llvmTy)
		}
		e.prologue = append(e.prologue, fmt.Sprintf("%s = llvm.mlir.constant(%s : %s) : %s", name, normalizedRaw, llvmTy, llvmTy))
	default:
		return e.materializeZero(llvmTy)
	}

	return name, nil
}

func (e *funcEmitter) emitImplicitZeroReturn() error {
	if e.resultTy == "void" {
		e.emitTerminator("llvm.return")
		return nil
	}
	if len(e.resultGoTys) == 0 {
		e.emitTerminator(fmt.Sprintf("llvm.return %s : %s", mustZeroValue(e, e.resultTy), e.resultTy))
		return nil
	}
	if len(e.resultGoTys) == 1 {
		zero, err := e.materializeZero(e.resultTy)
		if err != nil {
			return err
		}
		e.emitTerminator(fmt.Sprintf("llvm.return %s : %s", zero, e.resultTy))
		return nil
	}
	aggregate := e.freshValue("retagg")
	e.emitInstruction(fmt.Sprintf("%s = llvm.mlir.undef : %s", aggregate, e.resultTy))
	for idx, llvmTy := range e.resultLLVMTys {
		zero, err := e.materializeZero(llvmTy)
		if err != nil {
			return err
		}
		next := e.freshValue("retagg")
		e.emitInstruction(fmt.Sprintf("%s = llvm.insertvalue %s, %s[%d] : %s", next, zero, aggregate, idx, e.resultTy))
		aggregate = next
	}
	e.emitTerminator(fmt.Sprintf("llvm.return %s : %s", aggregate, e.resultTy))
	return nil
}

func mustZeroValue(e *funcEmitter, llvmTy string) string {
	zero, err := e.materializeZero(llvmTy)
	if err != nil {
		panic(err)
	}
	return zero
}

func (e *funcEmitter) materializeStringLiteral(raw string) (string, error) {
	unquoted, err := unquoteGoStringLiteral(raw)
	if err != nil {
		return "", err
	}
	encoded := quoteLLVMStringBytes(unquoted + "\x00")
	key := "str:" + encoded
	if existing, ok := e.constants[key]; ok {
		return existing, nil
	}
	name := fmt.Sprintf("str%d", len(e.stringGlobals))
	e.stringGlobals[name] = stringGlobalDecl{name: name, encoded: encoded, length: len(unquoted) + 1}
	addr := e.freshValue("str")
	e.constants[key] = addr
	e.prologue = append(e.prologue,
		fmt.Sprintf("%s = llvm.mlir.addressof @%s : !llvm.ptr", addr, name),
	)
	return addr, nil
}

func (e *funcEmitter) extractSliceData(value string, llvmTy string) (string, error) {
	return e.sliceLayout.extract(e, value, llvmTy, sliceFieldData)
}

func (e *funcEmitter) extractSliceLength(value string, llvmTy string) (string, error) {
	return e.sliceLayout.extract(e, value, llvmTy, sliceFieldLen)
}

func (e *funcEmitter) extractSliceCapacity(value string, llvmTy string) (string, error) {
	return e.sliceLayout.extract(e, value, llvmTy, sliceFieldCap)
}

func (e *funcEmitter) buildSliceValue(parts sliceValueParts) (string, string, error) {
	return e.sliceLayout.build(e, parts)
}

func (e *funcEmitter) resolveSliceBound(raw string, fallback string) (string, error) {
	if raw == "" {
		return fallback, nil
	}
	value, llvmTy, err := e.resolveTyped(valueRef{raw: raw, ty: "i32"}, "i32")
	if err != nil {
		return "", err
	}
	if llvmTy != "i32" {
		return "", fmt.Errorf("expected i32 slice bound, got %s", llvmTy)
	}
	return value, nil
}

func (e *funcEmitter) resolveSlice(raw string, hintTy string) (string, string, error) {
	baseRaw, lowRaw, highRaw, hasBounds, ok := splitMLSESliceExpr(raw)
	if !ok {
		value, err := e.materializeZero(e.llvmType(hintTy))
		if err != nil {
			return "", "", err
		}
		return value, e.llvmType(hintTy), nil
	}

	baseGoTy := e.typeOfValue(baseRaw)
	resultGoTy := hintTy
	if resultGoTy == "" || resultGoTy == "!go.any" {
		resultGoTy = sliceExprResultType(baseGoTy)
	}
	if !hasBounds {
		if isGoSliceType(baseGoTy) || strings.HasPrefix(baseGoTy, "!go.string") {
			return e.resolveTyped(valueRef{raw: baseRaw, ty: resultGoTy}, resultGoTy)
		}
		value, err := e.materializeZero(e.llvmType(resultGoTy))
		if err != nil {
			return "", "", err
		}
		return value, e.llvmType(resultGoTy), nil
	}
	if lowRaw == "" && highRaw == "" {
		return e.resolveTyped(valueRef{raw: baseRaw, ty: resultGoTy}, resultGoTy)
	}
	if strings.HasPrefix(baseGoTy, "!go.string") {
		return e.resolveTyped(valueRef{raw: baseRaw, ty: resultGoTy}, resultGoTy)
	}
	if !isGoSliceType(baseGoTy) {
		value, err := e.materializeZero(e.llvmType(resultGoTy))
		if err != nil {
			return "", "", err
		}
		return value, e.llvmType(resultGoTy), nil
	}

	base, baseLLVMTy, err := e.resolveTyped(valueRef{raw: baseRaw, ty: baseGoTy}, baseGoTy)
	if err != nil {
		return "", "", err
	}
	if baseLLVMTy != e.sliceLayout.llvmType() {
		return "", "", fmt.Errorf("expected slice value, got %s", baseLLVMTy)
	}

	data, err := e.extractSliceData(base, baseLLVMTy)
	if err != nil {
		return "", "", err
	}
	length, err := e.extractSliceLength(base, baseLLVMTy)
	if err != nil {
		return "", "", err
	}
	capacity := ""
	if e.sliceLayout.hasCap() {
		capacity, err = e.extractSliceCapacity(base, baseLLVMTy)
		if err != nil {
			return "", "", err
		}
	}
	zero, err := e.materializeLiteral("0", "i32")
	if err != nil {
		return "", "", err
	}

	low, err := e.resolveSliceBound(lowRaw, zero)
	if err != nil {
		value, zeroErr := e.materializeZero(e.llvmType(resultGoTy))
		if zeroErr != nil {
			return "", "", err
		}
		return value, e.llvmType(resultGoTy), nil
	}
	high, err := e.resolveSliceBound(highRaw, length)
	if err != nil {
		value, zeroErr := e.materializeZero(e.llvmType(resultGoTy))
		if zeroErr != nil {
			return "", "", err
		}
		return value, e.llvmType(resultGoTy), nil
	}

	newData := data
	if lowRaw != "" {
		newData = e.freshValue("slice.subdata")
		e.emitInstruction(fmt.Sprintf("%s = llvm.getelementptr %s[%s] : (!llvm.ptr, i32) -> !llvm.ptr", newData, data, low))
	}

	newLen := high
	if lowRaw != "" {
		newLen = e.freshValue("slice.sublen")
		e.emitInstruction(fmt.Sprintf("%s = llvm.sub %s, %s : i32", newLen, high, low))
	}

	newCap := capacity
	if e.sliceLayout.hasCap() && lowRaw != "" {
		newCap = e.freshValue("slice.subcap")
		e.emitInstruction(fmt.Sprintf("%s = llvm.sub %s, %s : i32", newCap, capacity, low))
	}

	return e.buildSliceValue(sliceValueParts{
		Data:     newData,
		Length:   newLen,
		Capacity: newCap,
	})
}

func (e *funcEmitter) resolveIndex(raw string, hintTy string) (string, string, error) {
	baseRaw, indexRaw, ok := splitMLSEIndexExpr(raw)
	if !ok {
		value, err := e.materializeZero(e.llvmType(hintTy))
		if err != nil {
			return "", "", err
		}
		return value, e.llvmType(hintTy), nil
	}
	baseGoTy := e.typeOfValue(baseRaw)
	if baseGoTy == "" {
		baseGoTy = "!go.any"
	}
	base, baseTy, err := e.resolveTyped(valueRef{raw: baseRaw, ty: baseGoTy}, baseGoTy)
	if err != nil {
		return "", "", err
	}
	if isGoSliceType(baseGoTy) {
		base, err = e.extractSliceData(base, baseTy)
		if err != nil {
			return "", "", err
		}
		baseTy = "!llvm.ptr"
	}
	idx, idxTy, err := e.resolveTyped(valueRef{raw: indexRaw, ty: "i32"}, "i32")
	if err != nil {
		return "", "", err
	}
	if !isPointerLLVMType(baseTy) || !isIntegerLLVMType(idxTy) {
		value, err := e.materializeZero(e.llvmType(hintTy))
		if err != nil {
			return "", "", err
		}
		return value, e.llvmType(hintTy), nil
	}
	resultTy := e.llvmType(hintTy)
	if !isIntegerLLVMType(resultTy) && resultTy != "!llvm.ptr" {
		value, err := e.materializeZero(resultTy)
		if err != nil {
			return "", "", err
		}
		return value, resultTy, nil
	}
	ptr := e.freshValue("idxptr")
	e.emitInstruction(fmt.Sprintf("%s = llvm.getelementptr %s[%s] : (!llvm.ptr, i32) -> !llvm.ptr, %s", ptr, base, idx, resultTy))
	if resultTy == "!llvm.ptr" {
		return ptr, resultTy, nil
	}
	loaded := e.freshValue("idx")
	align := alignmentForLLVMType(resultTy)
	e.emitInstruction(fmt.Sprintf("%s = llvm.load %s {alignment = %d : i64} : !llvm.ptr -> %s", loaded, ptr, align, resultTy))
	return loaded, resultTy, nil
}

func (e *funcEmitter) resolveTyped(ref valueRef, hintTy string) (string, string, error) {
	if ref.raw == "" {
		return "", "", errors.New("empty value")
	}
	llvmTy := e.llvmType(hintTy)
	if llvmTy == "void" {
		llvmTy = "!llvm.ptr"
	}
	if strings.HasPrefix(ref.raw, "%") {
		slot, ok := e.locals[ref.raw]
		if !ok {
			value, err := e.materializeZero(llvmTy)
			if err != nil {
				return "", "", err
			}
			return value, llvmTy, nil
		}
		if llvmTy, ok := e.coercedLocalLLVMType(ref.raw, slot, hintTy); ok {
			value, err := e.materializeZero(llvmTy)
			if err != nil {
				return "", "", err
			}
			return value, llvmTy, nil
		}
		tmp := e.freshValue("load")
		align := alignmentForLLVMType(slot.llvmTy)
		e.emitInstruction(fmt.Sprintf("%s = llvm.load %s {alignment = %d : i64} : !llvm.ptr -> %s", tmp, slot.ptr, align, slot.llvmTy))
		return tmp, slot.llvmTy, nil
	}

	if ref.raw == "true" || ref.raw == "false" || ref.raw == "mlse.nil" || isIntegerLiteral(ref.raw) || isQuotedStringLiteral(ref.raw) {
		value, err := e.materializeLiteral(ref.raw, llvmTy)
		if err != nil {
			return "", "", err
		}
		return value, llvmTy, nil
	}

	switch {
	case strings.HasPrefix(ref.raw, "mlse.not "):
		inner := valueRef{raw: strings.TrimSpace(strings.TrimPrefix(ref.raw, "mlse.not ")), ty: hintTy}
		value, innerTy, err := e.resolveTyped(inner, hintTy)
		if err != nil {
			return "", "", err
		}
		cond, err := e.emitTruthiness(value, innerTy, 0)
		if err != nil {
			return "", "", err
		}
		zero, err := e.materializeZero("i1")
		if err != nil {
			return "", "", err
		}
		notValue := e.freshValue("not")
		e.emitInstruction(fmt.Sprintf("%s = llvm.icmp %q %s, %s : i1", notValue, "eq", cond, zero))
		return notValue, "i1", nil
	case strings.HasPrefix(ref.raw, "mlse.index "):
		return e.resolveIndex(ref.raw, hintTy)
	case strings.HasPrefix(ref.raw, "mlse.addr "):
		innerRaw := strings.TrimSpace(strings.TrimPrefix(ref.raw, "mlse.addr "))
		if strings.HasPrefix(innerRaw, "%") {
			if slot, ok := e.locals[innerRaw]; ok {
				return slot.ptr, "!llvm.ptr", nil
			}
		}
		value, err := e.materializeZero(llvmTy)
		if err != nil {
			return "", "", err
		}
		return value, llvmTy, nil
	case strings.HasPrefix(ref.raw, "mlse.load "):
		inner := valueRef{raw: strings.TrimSpace(strings.TrimPrefix(ref.raw, "mlse.load ")), ty: "!go.any"}
		base, baseTy, err := e.resolveTyped(inner, "!go.any")
		if err != nil {
			return "", "", err
		}
		if baseTy != "!llvm.ptr" {
			base, err = e.materializeZero("!llvm.ptr")
			if err != nil {
				return "", "", err
			}
		}
		if llvmTy == "void" {
			llvmTy = "!llvm.ptr"
		}
		tmp := e.freshValue("deref")
		align := alignmentForLLVMType(llvmTy)
		e.emitInstruction(fmt.Sprintf("%s = llvm.load %s {alignment = %d : i64} : !llvm.ptr -> %s", tmp, base, align, llvmTy))
		return tmp, llvmTy, nil
	case strings.HasPrefix(ref.raw, "mlse.neg "):
		inner := valueRef{raw: strings.TrimSpace(strings.TrimPrefix(ref.raw, "mlse.neg ")), ty: hintTy}
		value, innerTy, err := e.resolveTyped(inner, hintTy)
		if err != nil {
			return "", "", err
		}
		if !isIntegerLLVMType(innerTy) {
			zero, err := e.materializeZero(innerTy)
			if err != nil {
				return "", "", err
			}
			return zero, innerTy, nil
		}
		zero, err := e.materializeZero(innerTy)
		if err != nil {
			return "", "", err
		}
		tmp := e.freshValue("neg")
		e.emitInstruction(fmt.Sprintf("%s = llvm.sub %s, %s : %s", tmp, zero, value, innerTy))
		return tmp, innerTy, nil
	case strings.HasPrefix(ref.raw, "mlse.slice "):
		return e.resolveSlice(ref.raw, hintTy)
	case strings.HasPrefix(ref.raw, "mlse.select "),
		strings.HasPrefix(ref.raw, "mlse.typeassert "),
		strings.HasPrefix(ref.raw, "mlse.composite "),
		strings.HasPrefix(ref.raw, "mlse.kv "),
		strings.HasPrefix(ref.raw, "mlse.unsupported_expr("):
		value, err := e.materializeZero(llvmTy)
		if err != nil {
			return "", "", err
		}
		return value, llvmTy, nil
	}

	value, err := e.materializeZero(llvmTy)
	if err != nil {
		return "", "", err
	}
	return value, llvmTy, nil
}

func (e *funcEmitter) typeOfValue(raw string) string {
	if strings.HasPrefix(raw, "%") {
		if slot, ok := e.locals[raw]; ok && slot.goTy != "" {
			return slot.goTy
		}
		if slot, ok := e.locals[raw]; ok {
			return reverseLLVMType(slot.llvmTy)
		}
		return "!go.any"
	}
	switch {
	case raw == "true" || raw == "false":
		return "i1"
	case raw == "mlse.nil":
		return "!go.nil"
	case isQuotedStringLiteral(raw):
		return "!go.string"
	case strings.HasPrefix(raw, "mlse.not "):
		return "i1"
	case strings.HasPrefix(raw, "mlse.slice "):
		baseRaw, _, _, _, ok := splitMLSESliceExpr(raw)
		if !ok {
			return "!go.any"
		}
		return sliceExprResultType(e.typeOfValue(baseRaw))
	case isIntegerLiteral(raw):
		return "i32"
	default:
		return "!go.any"
	}
}

func (e *funcEmitter) preferredLocalGoType(name string, declared string) string {
	if declared != "" && declared != "!go.any" {
		return declared
	}
	if slot, ok := e.locals[name]; ok && slot.goTy != "" && slot.goTy != "!go.any" {
		return slot.goTy
	}
	return declared
}

func (e *funcEmitter) coercedLocalLLVMType(name string, slot localSlot, hintTy string) (string, bool) {
	hintLLVM := e.llvmType(hintTy)
	if hintLLVM == "void" || hintLLVM == slot.llvmTy {
		return "", false
	}
	if slot.goTy == "" || slot.goTy == "!go.any" {
		return hintLLVM, true
	}
	if strings.HasPrefix(name, "%range_idx") || strings.HasPrefix(name, "%range_len") {
		return hintLLVM, true
	}
	if (isPointerLLVMType(slot.llvmTy) && isIntegerLLVMType(hintLLVM)) || (isIntegerLLVMType(slot.llvmTy) && isPointerLLVMType(hintLLVM)) {
		return hintLLVM, true
	}
	if isIntegerLLVMType(slot.llvmTy) && isIntegerLLVMType(hintLLVM) {
		return hintLLVM, true
	}
	return "", false
}

func (e *funcEmitter) resolveExternSymbol(base string, params []string, result string) string {
	decl := externDecl{name: base, base: base, params: append([]string(nil), params...), result: result}
	if _, shadowedByInternal := e.signatures[base]; !shadowedByInternal && !mustMangleExternBase(base) {
		if existing, ok := e.externs[base]; ok && sameSignature(existing, decl) {
			return base
		}
		if _, ok := e.externs[base]; !ok {
			e.externs[base] = decl
			return base
		}
	}

	symbol := mangleExternSymbol(base, params, result)
	decl.name = symbol
	e.externs[symbol] = decl
	return symbol
}

func llvmFunctionResultType(resultTys []string) string {
	switch len(resultTys) {
	case 0:
		return "void"
	case 1:
		return resultTys[0]
	default:
		return "!llvm.struct<(" + strings.Join(resultTys, ", ") + ")>"
	}
}

func mangleExternSymbol(base string, params []string, result string) string {
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("__")
	if len(params) == 0 {
		b.WriteString("void")
	} else {
		for i, param := range params {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteString(param)
		}
	}
	b.WriteString("__ret_")
	b.WriteString(result)
	return sanitizeSymbol(b.String())
}

func sameSignature(a, b externDecl) bool {
	if a.base != b.base || a.result != b.result || len(a.params) != len(b.params) {
		return false
	}
	for i := range a.params {
		if a.params[i] != b.params[i] {
			return false
		}
	}
	return true
}

func mustMangleExternBase(base string) bool {
	switch base {
	case "append":
		return true
	}
	return strings.Contains(base, ".") || strings.HasPrefix(base, "funclit")
}
