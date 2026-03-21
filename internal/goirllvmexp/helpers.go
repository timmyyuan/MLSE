package goirllvmexp

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func splitTopLevel(input string) ([]string, error) {
	var out []string
	depth := 0
	start := 0
	inString := false
	escaped := false
	for i, r := range input {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '<', '(', '[':
			depth++
		case '>', ')', ']':
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

func splitTrailingType(input string) (string, string, error) {
	depth := 0
	inString := false
	escaped := false
	splitAt := -1
	for i, r := range input {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '<', '(', '[':
			depth++
		case '>', ')', ']':
			depth--
			if depth < 0 {
				return "", "", fmt.Errorf("unbalanced delimiters in %q", input)
			}
		case ':':
			if depth == 0 && i > 0 && i+1 < len(input) && input[i-1] == ' ' && input[i+1] == ' ' {
				splitAt = i - 1
			}
		}
	}
	if depth != 0 || splitAt < 0 {
		return "", "", fmt.Errorf("missing trailing type in %q", input)
	}
	head := strings.TrimSpace(input[:splitAt])
	ty := strings.TrimSpace(input[splitAt+3:])
	if head == "" || ty == "" {
		return "", "", fmt.Errorf("missing trailing type in %q", input)
	}
	return head, ty, nil
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
		case "int", "uint", "len", "cap":
			return "i32"
		case "byte", "uint8", "int8":
			return "i8"
		case "uint16", "int16":
			return "i16"
		case "uint32", "int32", "rune":
			return "i32"
		case "uint64", "int64", "uintptr":
			return "i64"
		default:
			return "!llvm.ptr"
		}
	default:
		return "!llvm.ptr"
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

func isPointerLLVMType(llvmTy string) bool {
	return llvmTy == "!llvm.ptr"
}

func emitCompareInst(op string, dest string, llvmTy string, lhs string, rhs string) (string, string, error) {
	var pred string
	switch op {
	case "arith.cmpi_eq":
		pred = "eq"
	case "arith.cmpi_ne":
		pred = "ne"
	case "arith.cmpi_gt":
		pred = "sgt"
	case "arith.cmpi_lt":
		pred = "slt"
	case "arith.cmpi_ge":
		pred = "sge"
	case "arith.cmpi_le":
		pred = "sle"
	default:
		return "", "", fmt.Errorf("unsupported compare op %q", op)
	}
	if !isIntegerLLVMType(llvmTy) && !isPointerLLVMType(llvmTy) {
		return "", "", fmt.Errorf("unsupported compare type %q", llvmTy)
	}
	return fmt.Sprintf("%s = llvm.icmp %q %s, %s : %s", dest, pred, lhs, rhs, llvmTy), "i1", nil
}

func reverseLLVMType(llvmTy string) string {
	switch llvmTy {
	case "i1", "i8", "i16", "i32", "i64":
		return llvmTy
	default:
		return "!go.any"
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
	_, ok := parseIntegerLiteral(raw)
	return ok
}

func parseIntegerLiteral(raw string) (*big.Int, bool) {
	if raw == "" {
		return nil, false
	}
	v := new(big.Int)
	parsed, ok := v.SetString(raw, 0)
	if ok {
		return parsed, true
	}
	return nil, false
}

func integerBitWidth(llvmTy string) (int, bool) {
	switch llvmTy {
	case "i1":
		return 1, true
	case "i8":
		return 8, true
	case "i16":
		return 16, true
	case "i32":
		return 32, true
	case "i64":
		return 64, true
	default:
		return 0, false
	}
}

func normalizeIntegerLiteral(raw string, llvmTy string) (string, error) {
	v, ok := parseIntegerLiteral(raw)
	if !ok {
		return "", fmt.Errorf("invalid integer literal %q", raw)
	}
	width, ok := integerBitWidth(llvmTy)
	if !ok {
		return raw, nil
	}
	modulus := new(big.Int).Lsh(big.NewInt(1), uint(width))
	wrapped := new(big.Int).Mod(v, modulus)
	if wrapped.Sign() < 0 {
		wrapped.Add(wrapped, modulus)
	}
	if width > 1 {
		signBit := new(big.Int).Lsh(big.NewInt(1), uint(width-1))
		if wrapped.Cmp(signBit) >= 0 {
			wrapped.Sub(wrapped, modulus)
		}
	}
	return wrapped.String(), nil
}

func (e *funcEmitter) emitTruthiness(value string, llvmTy string, line int) (string, error) {
	switch {
	case llvmTy == "i1":
		return value, nil
	case isIntegerLLVMType(llvmTy), isPointerLLVMType(llvmTy):
		zero, err := e.materializeZero(llvmTy)
		if err != nil {
			if line > 0 {
				return "", fmt.Errorf("line %d: %v", line, err)
			}
			return "", err
		}
		tmp := e.freshValue("truthy")
		e.emitInstruction(fmt.Sprintf("%s = llvm.icmp %q %s, %s : %s", tmp, "ne", value, zero, llvmTy))
		return tmp, nil
	default:
		if line > 0 {
			return "", fmt.Errorf("line %d: unsupported if condition type %q", line, llvmTy)
		}
		return "", fmt.Errorf("unsupported if condition type %q", llvmTy)
	}
}

func alignmentForLLVMType(llvmTy string) int64 {
	switch llvmTy {
	case "i1", "i8":
		return 1
	case "i16":
		return 2
	case "i32":
		return 4
	case "i64", "!llvm.ptr":
		return 8
	default:
		return 8
	}
}

func llvmCallResultText(llvmTy string) string {
	if llvmTy == "void" {
		return "()"
	}
	return llvmTy
}

func isQuotedStringLiteral(raw string) bool {
	return len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"'
}

func unquoteGoStringLiteral(raw string) (string, error) {
	return strconv.Unquote(raw)
}

func quoteLLVMStringBytes(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 32 && c <= 126 && c != '"' && c != '\\' {
			b.WriteByte(c)
			continue
		}
		b.WriteString(fmt.Sprintf("\\%02X", c))
	}
	b.WriteByte('"')
	return b.String()
}

func splitMLSEIndexExpr(raw string) (string, string, bool) {
	text := strings.TrimSpace(strings.TrimPrefix(raw, "mlse.index "))
	end := strings.LastIndexByte(text, ']')
	start := strings.LastIndexByte(text[:end], '[')
	if end < 0 || start < 0 || start+1 > end {
		return "", "", false
	}
	base := strings.TrimSpace(text[:start])
	index := strings.TrimSpace(text[start+1 : end])
	if base == "" || index == "" {
		return "", "", false
	}
	return base, index, true
}
