package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

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
	name   string
	params []typedValue
	result string
	body   []instruction
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
	val  *valueRef
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
	params []string
	result string
}

type localSlot struct {
	goTy   string
	llvmTy string
	ptr    string
}

type funcEmitter struct {
	signatures   map[string]*function
	externs      map[string]externDecl
	locals       map[string]localSlot
	lines        []string
	prologue     []string
	resultTy     string
	blockSeq     int
	valueSeq     int
	controlDepth int
	current      string
	terminated   bool
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [input.goir]\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Translate a tiny experimental MLSE GoIR subset to LLVM IR.")
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}

	var (
		data []byte
		err  error
	)
	if flag.NArg() == 1 {
		data, err = os.ReadFile(flag.Arg(0))
	} else {
		data, err = io.ReadAll(bufio.NewReader(os.Stdin))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-goir-llvm-exp: %v\n", err)
		os.Exit(1)
	}

	out, err := translateModule(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-goir-llvm-exp: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}

func translateModule(input string) (string, error) {
	mod, err := parseModule(input)
	if err != nil {
		return "", err
	}
	return emitModule(mod)
}

func parseModule(input string) (*module, error) {
	lines := collectLines(input)
	if len(lines) == 0 {
		return nil, errors.New("empty input")
	}
	if lines[0].text != "module {" {
		return nil, fmt.Errorf("line %d: expected `module {`", lines[0].number)
	}

	mod := &module{}
	for i := 1; i < len(lines); {
		line := lines[i]
		if line.text == "}" {
			if i != len(lines)-1 {
				return nil, fmt.Errorf("line %d: unexpected content after module end", line.number)
			}
			return mod, nil
		}
		if !strings.HasPrefix(line.text, "func.func @") {
			return nil, fmt.Errorf("line %d: unsupported top-level line %q", line.number, line.text)
		}
		fn, next, err := parseFunction(lines, i)
		if err != nil {
			return nil, err
		}
		mod.funcs = append(mod.funcs, fn)
		i = next
	}
	return nil, errors.New("missing module terminator")
}

func collectLines(input string) []sourceLine {
	var out []sourceLine
	scanner := bufio.NewScanner(strings.NewReader(input))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		out = append(out, sourceLine{number: lineNo, text: text})
	}
	return out
}

func parseFunction(lines []sourceLine, start int) (*function, int, error) {
	header := lines[start]
	fn, err := parseFunctionHeader(header)
	if err != nil {
		return nil, 0, err
	}

	body, next, err := parseBlock(lines, start+1)
	if err != nil {
		return nil, 0, err
	}
	if next >= len(lines) {
		return nil, 0, fmt.Errorf("line %d: missing function terminator for %s", header.number, fn.name)
	}
	if lines[next].text != "}" {
		return nil, 0, fmt.Errorf("line %d: malformed function body terminator %q", lines[next].number, lines[next].text)
	}
	fn.body = body
	return fn, next + 1, nil
}

func parseFunctionHeader(line sourceLine) (*function, error) {
	const prefix = "func.func @"
	rest := strings.TrimPrefix(line.text, prefix)
	open := strings.Index(rest, "(")
	close := strings.LastIndex(rest, ")")
	if open <= 0 || close < open {
		return nil, fmt.Errorf("line %d: malformed function header %q", line.number, line.text)
	}

	fn := &function{name: rest[:open], result: "!go.unit"}
	params, err := parseParams(rest[open+1 : close])
	if err != nil {
		return nil, fmt.Errorf("line %d: %v", line.number, err)
	}
	fn.params = params

	tail := strings.TrimSpace(rest[close+1:])
	switch {
	case tail == "{":
		return fn, nil
	case strings.HasPrefix(tail, "-> ") && strings.HasSuffix(tail, "{"):
		result := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(tail, "-> "), "{"))
		if strings.HasPrefix(result, "(") {
			return nil, fmt.Errorf("line %d: multi-result functions are not supported", line.number)
		}
		fn.result = result
		return fn, nil
	default:
		return nil, fmt.Errorf("line %d: malformed function header tail %q", line.number, tail)
	}
}

func parseParams(input string) ([]typedValue, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	parts, err := splitTopLevel(input)
	if err != nil {
		return nil, err
	}
	out := make([]typedValue, 0, len(parts))
	for _, part := range parts {
		pieces := strings.SplitN(part, ": ", 2)
		if len(pieces) != 2 {
			return nil, fmt.Errorf("malformed parameter %q", part)
		}
		out = append(out, typedValue{name: pieces[0], ty: pieces[1]})
	}
	return out, nil
}

func parseBlock(lines []sourceLine, start int) ([]instruction, int, error) {
	var out []instruction
	for i := start; i < len(lines); {
		line := lines[i]
		if line.text == "}" || line.text == "} else {" {
			return out, i, nil
		}
		inst, next, err := parseInstruction(lines, i)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, inst)
		i = next
	}
	lineNo := 0
	if len(lines) > 0 {
		lineNo = lines[len(lines)-1].number
	}
	return nil, 0, fmt.Errorf("line %d: missing block terminator", lineNo)
}

func parseInstruction(lines []sourceLine, start int) (instruction, int, error) {
	line := lines[start]
	switch {
	case line.text == "return":
		return &returnInst{line: line.number}, start + 1, nil
	case strings.HasPrefix(line.text, "return "):
		inst, err := parseReturn(line)
		return inst, start + 1, err
	case strings.HasPrefix(line.text, "mlse.if "):
		return parseIf(lines, start)
	case strings.HasPrefix(line.text, "mlse.for "):
		return parseFor(lines, start)
	case strings.HasPrefix(line.text, "mlse.switch "):
		return parseSwitch(lines, start)
	case strings.HasPrefix(line.text, "%") && strings.Contains(line.text, " = "):
		inst, err := parseAssignment(line)
		return inst, start + 1, err
	default:
		return nil, 0, fmt.Errorf("line %d: unsupported GoIR instruction %q", line.number, line.text)
	}
}

func parseIf(lines []sourceLine, start int) (instruction, int, error) {
	header := lines[start]
	cond, err := parseConditionHeader(header, "mlse.if ")
	if err != nil {
		return nil, 0, err
	}

	thenBody, next, err := parseBlock(lines, start+1)
	if err != nil {
		return nil, 0, err
	}
	if next >= len(lines) {
		return nil, 0, fmt.Errorf("line %d: missing terminator for mlse.if", header.number)
	}

	inst := &ifInst{line: header.number, cond: cond, thenBody: thenBody}
	switch lines[next].text {
	case "}":
		return inst, next + 1, nil
	case "} else {":
		elseBody, elseEnd, err := parseBlock(lines, next+1)
		if err != nil {
			return nil, 0, err
		}
		if elseEnd >= len(lines) || lines[elseEnd].text != "}" {
			return nil, 0, fmt.Errorf("line %d: missing else terminator for mlse.if", header.number)
		}
		inst.elseBody = elseBody
		return inst, elseEnd + 1, nil
	default:
		return nil, 0, fmt.Errorf("line %d: malformed mlse.if terminator %q", lines[next].number, lines[next].text)
	}
}

func parseFor(lines []sourceLine, start int) (instruction, int, error) {
	header := lines[start]
	cond, err := parseConditionHeader(header, "mlse.for ")
	if err != nil {
		return nil, 0, err
	}

	body, next, err := parseBlock(lines, start+1)
	if err != nil {
		return nil, 0, err
	}
	if next >= len(lines) || lines[next].text != "}" {
		return nil, 0, fmt.Errorf("line %d: missing terminator for mlse.for", header.number)
	}
	return &forInst{line: header.number, cond: cond, body: body}, next + 1, nil
}

func parseSwitch(lines []sourceLine, start int) (instruction, int, error) {
	header := lines[start]
	tag, err := parseSwitchHeader(header)
	if err != nil {
		return nil, 0, err
	}

	inst := &switchInst{line: header.number, tag: tag}
	for i := start + 1; i < len(lines); {
		line := lines[i]
		if line.text == "}" {
			if len(inst.cases) == 0 {
				return nil, 0, fmt.Errorf("line %d: mlse.switch requires at least one case", header.number)
			}
			return inst, i + 1, nil
		}
		caseInst, next, err := parseSwitchCase(lines, i)
		if err != nil {
			return nil, 0, err
		}
		inst.cases = append(inst.cases, caseInst)
		i = next
	}
	return nil, 0, fmt.Errorf("line %d: missing terminator for mlse.switch", header.number)
}

func parseSwitchHeader(line sourceLine) (valueRef, error) {
	const prefix = "mlse.switch "
	if !strings.HasPrefix(line.text, prefix) || !strings.HasSuffix(line.text, "{") {
		return valueRef{}, fmt.Errorf("line %d: malformed switch header %q", line.number, line.text)
	}

	rest := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line.text, prefix), "{"))
	sep := strings.LastIndex(rest, " : ")
	if sep <= 0 {
		return valueRef{}, fmt.Errorf("line %d: malformed switch header %q", line.number, line.text)
	}
	tagText := strings.TrimSpace(rest[:sep])
	if strings.Contains(tagText, " ") {
		return valueRef{}, fmt.Errorf("line %d: unsupported switch tag expression %q", line.number, tagText)
	}
	return valueRef{raw: tagText, ty: strings.TrimSpace(rest[sep+3:])}, nil
}

func parseSwitchCase(lines []sourceLine, start int) (switchCase, int, error) {
	line := lines[start]
	switch {
	case line.text == "default {":
		body, next, err := parseBlock(lines, start+1)
		if err != nil {
			return switchCase{}, 0, err
		}
		if next >= len(lines) || lines[next].text != "}" {
			return switchCase{}, 0, fmt.Errorf("line %d: missing terminator for default case", line.number)
		}
		return switchCase{line: line.number, body: body, isDefault: true}, next + 1, nil
	case strings.HasPrefix(line.text, "case ") && strings.HasSuffix(line.text, "{"):
		rest := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line.text, "case "), "{"))
		sep := strings.LastIndex(rest, " : ")
		if sep <= 0 {
			return switchCase{}, 0, fmt.Errorf("line %d: malformed case header %q", line.number, line.text)
		}
		valuesText := strings.TrimSpace(rest[:sep])
		ty := strings.TrimSpace(rest[sep+3:])
		parts, err := splitTopLevel(valuesText)
		if err != nil {
			return switchCase{}, 0, fmt.Errorf("line %d: %v", line.number, err)
		}
		values := make([]valueRef, 0, len(parts))
		for _, part := range parts {
			raw := strings.TrimSpace(part)
			if strings.Contains(raw, " ") {
				return switchCase{}, 0, fmt.Errorf("line %d: unsupported case value expression %q", line.number, raw)
			}
			values = append(values, valueRef{raw: raw, ty: ty})
		}

		body, next, err := parseBlock(lines, start+1)
		if err != nil {
			return switchCase{}, 0, err
		}
		if next >= len(lines) || lines[next].text != "}" {
			return switchCase{}, 0, fmt.Errorf("line %d: missing terminator for case", line.number)
		}
		return switchCase{line: line.number, ty: ty, values: values, body: body}, next + 1, nil
	default:
		return switchCase{}, 0, fmt.Errorf("line %d: unsupported switch case line %q", line.number, line.text)
	}
}

func parseConditionHeader(line sourceLine, prefix string) (condition, error) {
	if !strings.HasPrefix(line.text, prefix) || !strings.HasSuffix(line.text, "{") {
		return nil, fmt.Errorf("line %d: malformed control-flow header %q", line.number, line.text)
	}

	rest := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line.text, prefix), "{"))
	sep := strings.LastIndex(rest, " : ")
	if sep <= 0 {
		return nil, fmt.Errorf("line %d: malformed control-flow header %q", line.number, line.text)
	}
	condText := strings.TrimSpace(rest[:sep])
	ty := strings.TrimSpace(rest[sep+3:])
	if strings.HasPrefix(condText, "arith.cmpi_") {
		return parseCompareCondition(line.number, condText, ty)
	}
	if strings.Contains(condText, " ") {
		return nil, fmt.Errorf("line %d: unsupported if condition expression %q", line.number, condText)
	}
	return &valueCondition{ref: valueRef{raw: condText, ty: ty}}, nil
}

func parseCompareCondition(lineNo int, head string, ty string) (condition, error) {
	firstSpace := strings.IndexByte(head, ' ')
	if firstSpace < 0 {
		return nil, fmt.Errorf("line %d: malformed compare condition %q", lineNo, head)
	}
	op := head[:firstSpace]
	ops, err := splitTopLevel(strings.TrimSpace(head[firstSpace+1:]))
	if err != nil {
		return nil, fmt.Errorf("line %d: %v", lineNo, err)
	}
	if len(ops) != 2 {
		return nil, fmt.Errorf("line %d: expected two operands in compare condition %q", lineNo, head)
	}
	return &compareCondition{
		op:  op,
		ty:  ty,
		lhs: valueRef{raw: strings.TrimSpace(ops[0])},
		rhs: valueRef{raw: strings.TrimSpace(ops[1])},
	}, nil
}

func parseReturn(line sourceLine) (instruction, error) {
	rest := strings.TrimPrefix(line.text, "return ")
	pieces := strings.SplitN(rest, " : ", 2)
	if len(pieces) != 2 {
		return nil, fmt.Errorf("line %d: malformed return %q", line.number, line.text)
	}
	if strings.Contains(pieces[0], ",") || strings.Contains(pieces[1], ",") {
		return nil, fmt.Errorf("line %d: multi-value returns are not supported", line.number)
	}
	ref := valueRef{raw: strings.TrimSpace(pieces[0]), ty: strings.TrimSpace(pieces[1])}
	return &returnInst{line: line.number, val: &ref}, nil
}

func parseAssignment(line sourceLine) (instruction, error) {
	pieces := strings.SplitN(line.text, " = ", 2)
	dest := strings.TrimSpace(pieces[0])
	rest := pieces[1]

	switch {
	case strings.HasPrefix(rest, "arith."):
		return parseBinary(line.number, dest, rest)
	case strings.HasPrefix(rest, "mlse.call "):
		return parseCall(line.number, dest, rest)
	case strings.HasPrefix(rest, "mlse.zero : "):
		ty := strings.TrimSpace(strings.TrimPrefix(rest, "mlse.zero : "))
		return &aliasInst{line: line.number, dest: typedValue{name: dest, ty: ty}, src: zeroValue(ty)}, nil
	default:
		valuePieces := strings.SplitN(rest, " : ", 2)
		if len(valuePieces) != 2 {
			return nil, fmt.Errorf("line %d: unsupported assignment %q", line.number, line.text)
		}
		ty := strings.TrimSpace(valuePieces[1])
		return &aliasInst{
			line: line.number,
			dest: typedValue{name: dest, ty: ty},
			src:  valueRef{raw: strings.TrimSpace(valuePieces[0]), ty: ty},
		}, nil
	}
}

func parseBinary(lineNo int, dest, rest string) (instruction, error) {
	pieces := strings.SplitN(rest, " : ", 2)
	if len(pieces) != 2 {
		return nil, fmt.Errorf("line %d: malformed arithmetic instruction %q", lineNo, rest)
	}
	head := pieces[0]
	ty := strings.TrimSpace(pieces[1])
	firstSpace := strings.IndexByte(head, ' ')
	if firstSpace < 0 {
		return nil, fmt.Errorf("line %d: malformed arithmetic instruction %q", lineNo, rest)
	}
	op := head[:firstSpace]
	ops, err := splitTopLevel(strings.TrimSpace(head[firstSpace+1:]))
	if err != nil {
		return nil, fmt.Errorf("line %d: %v", lineNo, err)
	}
	if len(ops) != 2 {
		return nil, fmt.Errorf("line %d: expected two operands in %q", lineNo, rest)
	}
	return &binaryInst{
		line: lineNo,
		dest: typedValue{name: dest, ty: ty},
		op:   op,
		lhs:  valueRef{raw: strings.TrimSpace(ops[0])},
		rhs:  valueRef{raw: strings.TrimSpace(ops[1])},
	}, nil
}

func parseCall(lineNo int, dest, rest string) (instruction, error) {
	pieces := strings.SplitN(rest, " : ", 2)
	if len(pieces) != 2 {
		return nil, fmt.Errorf("line %d: malformed call instruction %q", lineNo, rest)
	}

	head := strings.TrimPrefix(pieces[0], "mlse.call ")
	open := strings.IndexByte(head, '(')
	close := strings.LastIndexByte(head, ')')
	if open <= 0 || close < open {
		return nil, fmt.Errorf("line %d: malformed call instruction %q", lineNo, rest)
	}

	callee := strings.TrimPrefix(strings.TrimSpace(head[:open]), "%")
	var args []valueRef
	argText := strings.TrimSpace(head[open+1 : close])
	if argText != "" {
		parts, err := splitTopLevel(argText)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", lineNo, err)
		}
		args = make([]valueRef, 0, len(parts))
		for _, part := range parts {
			args = append(args, valueRef{raw: strings.TrimSpace(part)})
		}
	}

	return &callInst{
		line:   lineNo,
		dest:   typedValue{name: dest, ty: strings.TrimSpace(pieces[1])},
		callee: callee,
		args:   args,
	}, nil
}

func emitModule(mod *module) (string, error) {
	signatures := make(map[string]*function, len(mod.funcs))
	for _, fn := range mod.funcs {
		signatures[fn.name] = fn
	}

	var defs []string
	externs := map[string]externDecl{}
	for _, fn := range mod.funcs {
		text, fnExterns, err := emitFunction(fn, signatures)
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
	}

	var b strings.Builder
	b.WriteString("; Experimental translation from MLSE GoIR-like text to LLVM IR.\n")
	b.WriteString("; This path only supports a tiny additive subset and is not canonical lowering.\n\n")
	if len(externs) > 0 {
		names := make([]string, 0, len(externs))
		for name := range externs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			decl := externs[name]
			b.WriteString(fmt.Sprintf("declare %s @%s(%s)\n", decl.result, decl.name, strings.Join(decl.params, ", ")))
		}
		b.WriteString("\n")
	}
	for i, def := range defs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(def)
	}
	return b.String(), nil
}

func emitFunction(fn *function, signatures map[string]*function) (string, map[string]externDecl, error) {
	emitter := &funcEmitter{
		signatures: signatures,
		externs:    map[string]externDecl{},
		locals:     map[string]localSlot{},
		resultTy:   mustLLVMType(fn.result),
	}

	paramDefs := make([]string, 0, len(fn.params))
	for _, param := range fn.params {
		llvmTy := mustLLVMType(param.ty)
		paramDefs = append(paramDefs, fmt.Sprintf("%s %s", llvmTy, param.name))
		if err := emitter.bindParam(param); err != nil {
			return "", nil, err
		}
	}

	emitter.startBlock("entry")
	for _, inst := range fn.body {
		if err := inst.emit(emitter); err != nil {
			return "", nil, err
		}
	}

	switch {
	case emitter.current == "":
	case emitter.terminated:
	case emitter.resultTy == "void":
		emitter.emitTerminator("ret void")
	default:
		return "", nil, fmt.Errorf("function %s falls off the end without returning %s", fn.name, emitter.resultTy)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("define %s @%s(%s) {\n", emitter.resultTy, fn.name, strings.Join(paramDefs, ", ")))
	for i, line := range emitter.lines {
		b.WriteString(line)
		b.WriteString("\n")
		if i == 0 {
			for _, prologue := range emitter.prologue {
				b.WriteString("  ")
				b.WriteString(prologue)
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("}\n")
	return b.String(), emitter.externs, nil
}

func (i *aliasInst) emit(e *funcEmitter) error {
	value, llvmTy, err := e.resolveTyped(i.src, i.dest.ty)
	if err != nil {
		return fmt.Errorf("line %d: %v", i.line, err)
	}
	return e.storeLocal(i.dest.name, i.dest.ty, llvmTy, value)
}

func (i *binaryInst) emit(e *funcEmitter) error {
	llvmTy := mustLLVMType(i.dest.ty)
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

	var emitted string
	switch i.op {
	case "arith.addi":
		emitted = fmt.Sprintf("%s = add %s %s, %s", destValue, llvmTy, lhs, rhs)
	case "arith.subi":
		emitted = fmt.Sprintf("%s = sub %s %s, %s", destValue, llvmTy, lhs, rhs)
	case "arith.muli":
		emitted = fmt.Sprintf("%s = mul %s %s, %s", destValue, llvmTy, lhs, rhs)
	case "arith.divsi":
		emitted = fmt.Sprintf("%s = sdiv %s %s, %s", destValue, llvmTy, lhs, rhs)
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
	retTy := mustLLVMType(i.dest.ty)
	args := make([]string, 0, len(i.args))
	argTys := make([]string, 0, len(i.args))
	for _, arg := range i.args {
		goTy := arg.ty
		if goTy == "" {
			goTy = e.typeOfValue(arg.raw)
		}
		llvmTy := mustLLVMType(goTy)
		value, actualTy, err := e.resolveTyped(arg, goTy)
		if err != nil {
			return fmt.Errorf("line %d: %v", i.line, err)
		}
		if actualTy != llvmTy {
			return fmt.Errorf("line %d: call arg %q has LLVM type %s, expected %s", i.line, arg.raw, actualTy, llvmTy)
		}
		args = append(args, fmt.Sprintf("%s %s", llvmTy, value))
		argTys = append(argTys, llvmTy)
	}

	if fn, ok := e.signatures[i.callee]; ok {
		if len(fn.params) != len(argTys) {
			return fmt.Errorf("line %d: call to %s has %d args, expected %d", i.line, i.callee, len(argTys), len(fn.params))
		}
	} else {
		e.externs[i.callee] = externDecl{name: i.callee, params: argTys, result: retTy}
	}

	callTmp := e.freshValue("call")
	e.emitInstruction(fmt.Sprintf("%s = call %s @%s(%s)", callTmp, retTy, i.callee, strings.Join(args, ", ")))
	return e.storeLocal(i.dest.name, i.dest.ty, retTy, callTmp)
}

func (i *returnInst) emit(e *funcEmitter) error {
	if i.val == nil {
		if e.resultTy != "void" {
			return fmt.Errorf("line %d: function must return %s", i.line, e.resultTy)
		}
		e.emitTerminator("ret void")
		return nil
	}
	value, llvmTy, err := e.resolveTyped(*i.val, i.val.ty)
	if err != nil {
		return fmt.Errorf("line %d: %v", i.line, err)
	}
	if llvmTy != e.resultTy {
		return fmt.Errorf("line %d: return value has LLVM type %s, expected %s", i.line, llvmTy, e.resultTy)
	}
	e.emitTerminator(fmt.Sprintf("ret %s %s", e.resultTy, value))
	return nil
}

func (c *valueCondition) emit(e *funcEmitter, line int) (string, error) {
	value, llvmTy, err := e.resolveTyped(c.ref, c.ref.ty)
	if err != nil {
		return "", fmt.Errorf("line %d: %v", line, err)
	}
	if llvmTy != "i1" {
		return "", fmt.Errorf("line %d: unsupported if condition type %q", line, c.ref.ty)
	}
	return value, nil
}

func (c *compareCondition) emit(e *funcEmitter, line int) (string, error) {
	llvmTy := mustLLVMType(c.ty)
	if !isIntegerLLVMType(llvmTy) {
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

	e.emitTerminator(fmt.Sprintf("br i1 %s, label %%%s, label %%%s", cond, thenLabel, falseLabel))

	e.startBlock(thenLabel)
	if err := e.emitControlBody(i.thenBody); err != nil {
		return err
	}
	thenFallsThrough := e.current != "" && !e.terminated
	if thenFallsThrough {
		if mergeLabel == "" {
			mergeLabel = e.freshBlock("if.end")
		}
		e.emitTerminator(fmt.Sprintf("br label %%%s", mergeLabel))
	}

	if len(i.elseBody) > 0 {
		e.startBlock(falseLabel)
		if err := e.emitControlBody(i.elseBody); err != nil {
			return err
		}
		elseFallsThrough := e.current != "" && !e.terminated
		if elseFallsThrough {
			if mergeLabel == "" {
				mergeLabel = e.freshBlock("if.end")
			}
			e.emitTerminator(fmt.Sprintf("br label %%%s", mergeLabel))
		}
	}

	if mergeLabel != "" {
		e.startBlock(mergeLabel)
		return nil
	}

	e.current = ""
	e.terminated = true
	return nil
}

func (i *forInst) emit(e *funcEmitter) error {
	condLabel := e.freshBlock("for.cond")
	bodyLabel := e.freshBlock("for.body")
	endLabel := e.freshBlock("for.end")

	e.emitTerminator(fmt.Sprintf("br label %%%s", condLabel))

	e.startBlock(condLabel)
	cond, err := i.cond.emit(e, i.line)
	if err != nil {
		return err
	}
	e.emitTerminator(fmt.Sprintf("br i1 %s, label %%%s, label %%%s", cond, bodyLabel, endLabel))

	e.startBlock(bodyLabel)
	if err := e.emitControlBody(i.body); err != nil {
		return err
	}
	if e.current != "" && !e.terminated {
		e.emitTerminator(fmt.Sprintf("br label %%%s", condLabel))
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
		if mustLLVMType(switchCase.ty) != tagLLVM {
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
		e.emitTerminator(fmt.Sprintf("br label %%%s", checkLabels[0]))
	} else {
		e.emitTerminator(fmt.Sprintf("br label %%%s", defaultLabel))
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
		e.emitInstruction(fmt.Sprintf("%s = icmp eq %s %s, %s", cmp, tagLLVM, tagValue, caseValue))
		e.emitTerminator(fmt.Sprintf("br i1 %s, label %%%s, label %%%s", cmp, bodyLabels[idx], falseLabel))

		e.startBlock(bodyLabels[idx])
		if err := e.emitControlBody(switchCase.body); err != nil {
			return err
		}
		if e.current != "" && !e.terminated {
			needsEnd = true
			e.emitTerminator(fmt.Sprintf("br label %%%s", endLabel))
		}
	}

	if defaultCase >= 0 {
		e.startBlock(defaultLabel)
		if err := e.emitControlBody(i.cases[defaultCase].body); err != nil {
			return err
		}
		if e.current != "" && !e.terminated {
			needsEnd = true
			e.emitTerminator(fmt.Sprintf("br label %%%s", endLabel))
		}
	}

	if needsEnd {
		e.startBlock(endLabel)
		return nil
	}

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
	llvmTy := mustLLVMType(param.ty)
	slot, err := e.ensureLocal(param.name, param.ty, llvmTy)
	if err != nil {
		return err
	}
	e.prologue = append(e.prologue, fmt.Sprintf("store %s %s, ptr %s", llvmTy, param.name, slot.ptr))
	return nil
}

func (e *funcEmitter) storeLocal(name string, goTy string, llvmTy string, value string) error {
	slot, err := e.ensureLocal(name, goTy, llvmTy)
	if err != nil {
		return err
	}
	e.emitInstruction(fmt.Sprintf("store %s %s, ptr %s", llvmTy, value, slot.ptr))
	return nil
}

func (e *funcEmitter) ensureLocal(name string, goTy string, llvmTy string) (localSlot, error) {
	if llvmTy == "void" {
		return localSlot{}, fmt.Errorf("cannot materialize local %s with void type", name)
	}
	if slot, ok := e.locals[name]; ok {
		if slot.llvmTy != llvmTy {
			return localSlot{}, fmt.Errorf("%s changes type from %s to %s", name, slot.llvmTy, llvmTy)
		}
		return slot, nil
	}
	if e.controlDepth > 0 {
		return localSlot{}, fmt.Errorf("new locals inside control flow are not supported: %s", name)
	}
	slot := localSlot{
		goTy:   goTy,
		llvmTy: llvmTy,
		ptr:    e.freshValue("slot"),
	}
	e.locals[name] = slot
	e.prologue = append(e.prologue, fmt.Sprintf("%s = alloca %s", slot.ptr, llvmTy))
	return slot, nil
}

func (e *funcEmitter) startBlock(label string) {
	e.lines = append(e.lines, label+":")
	e.current = label
	e.terminated = false
}

func (e *funcEmitter) ensureBlock() {
	if e.current == "" || e.terminated {
		e.startBlock(e.freshBlock("dead"))
	}
}

func (e *funcEmitter) emitInstruction(line string) {
	e.ensureBlock()
	e.lines = append(e.lines, "  "+line)
}

func (e *funcEmitter) emitTerminator(line string) {
	e.ensureBlock()
	e.lines = append(e.lines, "  "+line)
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

func (e *funcEmitter) resolveTyped(ref valueRef, hintTy string) (string, string, error) {
	if ref.raw == "" {
		return "", "", errors.New("empty value")
	}
	if strings.HasPrefix(ref.raw, "%") {
		slot, ok := e.locals[ref.raw]
		if !ok {
			return "", "", fmt.Errorf("unknown value %s", ref.raw)
		}
		tmp := e.freshValue("load")
		e.emitInstruction(fmt.Sprintf("%s = load %s, ptr %s", tmp, slot.llvmTy, slot.ptr))
		return tmp, slot.llvmTy, nil
	}
	if ref.raw == "true" || ref.raw == "false" || ref.raw == "mlse.nil" || isIntegerLiteral(ref.raw) {
		llvmTy := mustLLVMType(hintTy)
		return llvmLiteral(ref.raw, llvmTy), llvmTy, nil
	}
	return "", "", fmt.Errorf("unsupported value %q", ref.raw)
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
	case isIntegerLiteral(raw):
		return "i32"
	default:
		return "!go.any"
	}
}

func sameSignature(a, b externDecl) bool {
	if a.name != b.name || a.result != b.result || len(a.params) != len(b.params) {
		return false
	}
	for i := range a.params {
		if a.params[i] != b.params[i] {
			return false
		}
	}
	return true
}

func splitTopLevel(input string) ([]string, error) {
	var out []string
	depth := 0
	start := 0
	for i, r := range input {
		switch r {
		case '<', '(':
			depth++
		case '>', ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced delimiters in %q", input)
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(input[start:i]))
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced delimiters in %q", input)
	}
	last := strings.TrimSpace(input[start:])
	if last != "" {
		out = append(out, last)
	}
	return out, nil
}

func mustLLVMType(goTy string) string {
	switch {
	case goTy == "" || goTy == "!go.unit":
		return "void"
	case goTy == "i1", goTy == "i8", goTy == "i16", goTy == "i32", goTy == "i64":
		return goTy
	case strings.HasPrefix(goTy, "!go.named<"):
		name := strings.TrimSuffix(strings.TrimPrefix(goTy, "!go.named<\""), "\">")
		switch name {
		case "bool":
			return "i1"
		case "byte", "uint8", "int8":
			return "i8"
		case "uint16", "int16":
			return "i16"
		case "uint32", "int32", "rune":
			return "i32"
		case "uint64", "int64", "uintptr":
			return "i64"
		default:
			return "ptr"
		}
	default:
		return "ptr"
	}
}

func isIntegerLLVMType(llvmTy string) bool {
	switch llvmTy {
	case "i1", "i8", "i16", "i32", "i64":
		return true
	default:
		return false
	}
}

func emitCompareInst(op string, dest string, llvmTy string, lhs string, rhs string) (string, string, error) {
	if !isIntegerLLVMType(llvmTy) {
		return "", "", fmt.Errorf("unsupported compare type %q", llvmTy)
	}
	switch op {
	case "arith.cmpi_eq":
		return fmt.Sprintf("%s = icmp eq %s %s, %s", dest, llvmTy, lhs, rhs), "i1", nil
	case "arith.cmpi_ne":
		return fmt.Sprintf("%s = icmp ne %s %s, %s", dest, llvmTy, lhs, rhs), "i1", nil
	case "arith.cmpi_gt":
		return fmt.Sprintf("%s = icmp sgt %s %s, %s", dest, llvmTy, lhs, rhs), "i1", nil
	case "arith.cmpi_lt":
		return fmt.Sprintf("%s = icmp slt %s %s, %s", dest, llvmTy, lhs, rhs), "i1", nil
	case "arith.cmpi_ge":
		return fmt.Sprintf("%s = icmp sge %s %s, %s", dest, llvmTy, lhs, rhs), "i1", nil
	case "arith.cmpi_le":
		return fmt.Sprintf("%s = icmp sle %s %s, %s", dest, llvmTy, lhs, rhs), "i1", nil
	default:
		return "", "", fmt.Errorf("unsupported compare op %q", op)
	}
}

func reverseLLVMType(llvmTy string) string {
	switch llvmTy {
	case "i1", "i8", "i16", "i32", "i64":
		return llvmTy
	default:
		return "!go.any"
	}
}

func llvmLiteral(raw string, llvmTy string) string {
	switch raw {
	case "true":
		return "1"
	case "false":
		return "0"
	case "mlse.nil":
		return "null"
	default:
		if llvmTy == "ptr" && raw == "0" {
			return "null"
		}
		return raw
	}
}

func zeroValue(goTy string) valueRef {
	switch mustLLVMType(goTy) {
	case "i1":
		return valueRef{raw: "false", ty: goTy}
	case "i8", "i16", "i32", "i64":
		return valueRef{raw: "0", ty: goTy}
	default:
		return valueRef{raw: "mlse.nil", ty: goTy}
	}
}

func isIntegerLiteral(raw string) bool {
	if raw == "" {
		return false
	}
	if raw[0] == '-' {
		raw = raw[1:]
	}
	_, err := strconv.ParseInt(raw, 10, 64)
	return err == nil
}
