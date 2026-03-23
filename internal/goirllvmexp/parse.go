package goirllvmexp

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

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
	if open <= 0 {
		return nil, fmt.Errorf("line %d: malformed function header %q", line.number, line.text)
	}
	close, err := findMatchingParen(rest, open)
	if err != nil {
		return nil, fmt.Errorf("line %d: malformed function header %q", line.number, line.text)
	}

	fn := &function{name: rest[:open]}
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
		results, err := parseResultTypes(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(tail, "-> "), "{")))
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", line.number, err)
		}
		fn.results = results
		return fn, nil
	default:
		return nil, fmt.Errorf("line %d: malformed function header tail %q", line.number, tail)
	}
}

func parseResultTypes(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	if strings.HasPrefix(input, "(") {
		if !strings.HasSuffix(input, ")") {
			return nil, fmt.Errorf("unbalanced delimiters in %q", input)
		}
		return splitTopLevel(strings.TrimSpace(input[1 : len(input)-1]))
	}
	return []string{input}, nil
}

func findMatchingParen(input string, open int) (int, error) {
	if open < 0 || open >= len(input) || input[open] != '(' {
		return -1, errors.New("invalid opening delimiter")
	}

	depth := 0
	inString := false
	escaped := false
	for i := open; i < len(input); i++ {
		switch ch := input[i]; {
		case inString:
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
		case ch == '"':
			inString = true
		case ch == '(':
			depth++
		case ch == ')':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return -1, errors.New("missing closing delimiter")
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
	case strings.HasPrefix(line.text, "mlse.expr "):
		inst, err := parseExpr(line)
		return inst, start + 1, err
	case strings.HasPrefix(line.text, "mlse.label "):
		inst, err := parseLabel(line)
		return inst, start + 1, err
	case strings.HasPrefix(line.text, "mlse.branch "):
		inst, err := parseBranch(line)
		return inst, start + 1, err
	case strings.HasPrefix(line.text, "mlse.++ "), strings.HasPrefix(line.text, "mlse.-- "):
		inst, err := parseIncDec(line)
		return inst, start + 1, err
	case strings.HasPrefix(line.text, "mlse.if "):
		return parseIf(lines, start)
	case strings.HasPrefix(line.text, "mlse.for "):
		return parseFor(lines, start)
	case strings.HasPrefix(line.text, "mlse.switch "):
		return parseSwitch(lines, start)
	case strings.HasPrefix(line.text, "mlse.store_"):
		inst, err := parseStore(line)
		return inst, start + 1, err
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
	return &valueCondition{ref: valueRef{raw: condText, ty: ty}}, nil
}

func parseExpr(line sourceLine) (instruction, error) {
	rest := strings.TrimPrefix(line.text, "mlse.expr ")
	head, ty, err := splitTrailingType(rest)
	if err != nil {
		return nil, fmt.Errorf("line %d: malformed expr %q", line.number, line.text)
	}
	return &exprInst{
		line: line.number,
		ref: valueRef{
			raw: head,
			ty:  ty,
		},
	}, nil
}

func parseLabel(line sourceLine) (instruction, error) {
	label := strings.TrimSpace(strings.TrimPrefix(line.text, "mlse.label "))
	if !strings.HasPrefix(label, "@") || len(label) == 1 {
		return nil, fmt.Errorf("line %d: malformed label %q", line.number, line.text)
	}
	return &labelInst{line: line.number, label: sanitizeSymbol(strings.TrimPrefix(label, "@"))}, nil
}

func parseBranch(line sourceLine) (instruction, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line.text, "mlse.branch "))
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return nil, fmt.Errorf("line %d: malformed branch %q", line.number, line.text)
	}
	kind, err := strconv.Unquote(fields[0])
	if err != nil {
		return nil, fmt.Errorf("line %d: malformed branch %q", line.number, line.text)
	}
	inst := &branchInst{line: line.number, kind: kind}
	if len(fields) > 1 {
		if !strings.HasPrefix(fields[1], "@") || len(fields[1]) == 1 {
			return nil, fmt.Errorf("line %d: malformed branch %q", line.number, line.text)
		}
		inst.label = sanitizeSymbol(strings.TrimPrefix(fields[1], "@"))
	}
	return inst, nil
}

func parseIncDec(line sourceLine) (instruction, error) {
	op := "mlse.++"
	if strings.HasPrefix(line.text, "mlse.-- ") {
		op = "mlse.--"
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line.text, op))
	target, ty, err := splitTrailingType(rest)
	if err != nil {
		return nil, fmt.Errorf("line %d: malformed %s %q", line.number, op, line.text)
	}
	return &incDecInst{
		line: line.number,
		op:   op,
		target: valueRef{
			raw: target,
			ty:  ty,
		},
	}, nil
}

func parseStore(line sourceLine) (instruction, error) {
	rest := strings.TrimPrefix(line.text, "mlse.store_")
	space := strings.IndexByte(rest, ' ')
	if space <= 0 {
		return nil, fmt.Errorf("line %d: malformed store %q", line.number, line.text)
	}
	kind := rest[:space]
	body := strings.TrimSpace(rest[space+1:])
	assign := strings.SplitN(body, " = ", 2)
	if len(assign) != 2 {
		return nil, fmt.Errorf("line %d: malformed store %q", line.number, line.text)
	}
	valueHead, valueTy, err := splitTrailingType(assign[1])
	if err != nil {
		return nil, fmt.Errorf("line %d: malformed store %q", line.number, line.text)
	}
	return &storeInst{
		line: line.number,
		kind: kind,
		target: valueRef{
			raw: strings.TrimSpace(assign[0]),
			ty:  "!go.any",
		},
		value: valueRef{
			raw: valueHead,
			ty:  valueTy,
		},
	}, nil
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
	valueText, typeText, err := splitTrailingType(rest)
	if err != nil {
		return nil, fmt.Errorf("line %d: malformed return %q", line.number, line.text)
	}
	values, err := splitTopLevel(valueText)
	if err != nil {
		return nil, fmt.Errorf("line %d: %v", line.number, err)
	}
	types, err := splitTopLevel(typeText)
	if err != nil {
		return nil, fmt.Errorf("line %d: %v", line.number, err)
	}
	if len(values) != len(types) {
		return nil, fmt.Errorf("line %d: return value/type arity mismatch", line.number)
	}
	refs := make([]valueRef, 0, len(values))
	for i := range values {
		refs = append(refs, valueRef{raw: strings.TrimSpace(values[i]), ty: strings.TrimSpace(types[i])})
	}
	return &returnInst{line: line.number, vals: refs}, nil
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
	case rest == "mlse.funclit":
		return &aliasInst{
			line: line.number,
			dest: typedValue{name: dest, ty: "!go.func"},
			src:  valueRef{raw: "mlse.funclit", ty: "!go.func"},
		}, nil
	case strings.HasPrefix(rest, "mlse.zero : "):
		ty := strings.TrimSpace(strings.TrimPrefix(rest, "mlse.zero : "))
		return &aliasInst{line: line.number, dest: typedValue{name: dest, ty: ty}, src: zeroValue(ty)}, nil
	default:
		valueHead, ty, err := splitTrailingType(rest)
		if err != nil {
			return nil, fmt.Errorf("line %d: unsupported assignment %q", line.number, line.text)
		}
		return &aliasInst{
			line: line.number,
			dest: typedValue{name: dest, ty: ty},
			src:  valueRef{raw: valueHead, ty: ty},
		}, nil
	}
}

func parseBinary(lineNo int, dest, rest string) (instruction, error) {
	head, ty, err := splitTrailingType(rest)
	if err != nil {
		return nil, fmt.Errorf("line %d: malformed arithmetic instruction %q", lineNo, rest)
	}
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
	headWithCall, ty, err := splitTrailingType(rest)
	if err != nil {
		return nil, fmt.Errorf("line %d: malformed call instruction %q", lineNo, rest)
	}

	head := strings.TrimPrefix(headWithCall, "mlse.call ")
	open := strings.IndexByte(head, '(')
	close := strings.LastIndexByte(head, ')')
	if open <= 0 || close < open {
		return nil, fmt.Errorf("line %d: malformed call instruction %q", lineNo, rest)
	}

	callee := normalizeCalleeName(head[:open])
	var args []valueRef
	argText := strings.TrimSpace(head[open+1 : close])
	if argText != "" {
		parts, err := splitCallArgs(argText)
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
		dest:   typedValue{name: dest, ty: ty},
		callee: callee,
		args:   args,
	}, nil
}

func normalizeCalleeName(raw string) string {
	text := strings.TrimSpace(raw)
	for strings.HasPrefix(text, "mlse.select ") {
		text = strings.TrimSpace(strings.TrimPrefix(text, "mlse.select "))
	}
	text = strings.TrimPrefix(text, "%")
	if text == "" {
		text = "opaque"
	}
	return sanitizeSymbol(text)
}

func splitCallArgs(input string) ([]string, error) {
	parts, err := splitTopLevel(input)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, nil
	}

	out := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		part := strings.TrimSpace(parts[i])
		if startsInlineBinaryExpr(part) && i+1 < len(parts) {
			part = part + ", " + strings.TrimSpace(parts[i+1])
			i++
		}
		out = append(out, part)
	}
	return out, nil
}

func startsInlineBinaryExpr(input string) bool {
	return strings.HasPrefix(input, "arith.")
}

func sanitizeSymbol(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "opaque"
	}
	return b.String()
}
