package gofrontend

import "strings"

func formatFormalFuncType(params []string, results []string) string {
	paramsText := strings.Join(params, ", ")
	switch len(results) {
	case 0:
		return "(" + paramsText + ") -> ()"
	case 1:
		if isFormalFunctionType(results[0]) {
			return "(" + paramsText + ") -> (" + results[0] + ")"
		}
		return "(" + paramsText + ") -> " + results[0]
	default:
		return "(" + paramsText + ") -> (" + strings.Join(results, ", ") + ")"
	}
}

func isFormalFunctionType(ty string) bool {
	_, ok := parseFormalFuncType(ty)
	return ok
}

func parseFormalFuncType(ty string) (formalFuncSig, bool) {
	ty = strings.TrimSpace(ty)
	if !strings.HasPrefix(ty, "(") {
		return formalFuncSig{}, false
	}

	paramsEnd, ok := consumeFormalGroupedType(ty, 0)
	if !ok {
		return formalFuncSig{}, false
	}

	params := splitFormalTypeList(ty[1 : paramsEnd-1])
	rest := strings.TrimSpace(ty[paramsEnd:])
	if !strings.HasPrefix(rest, "->") {
		return formalFuncSig{}, false
	}
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "->"))
	if rest == "" {
		return formalFuncSig{}, false
	}

	var results []string
	if strings.HasPrefix(rest, "(") {
		resultsEnd, ok := consumeFormalGroupedType(rest, 0)
		if !ok || strings.TrimSpace(rest[resultsEnd:]) != "" {
			return formalFuncSig{}, false
		}
		results = splitFormalTypeList(rest[1 : resultsEnd-1])
	} else {
		results = []string{rest}
	}

	return formalFuncSig{params: params, results: results}, true
}

func splitFormalTypeList(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var (
		out        []string
		start      int
		parenDepth int
		angleDepth int
		inString   bool
		escaped    bool
	)
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '<':
			angleDepth++
		case '>':
			angleDepth--
		case ',':
			if parenDepth == 0 && angleDepth == 0 {
				part := strings.TrimSpace(text[start:i])
				if part != "" {
					out = append(out, part)
				}
				start = i + 1
			}
		}
	}

	part := strings.TrimSpace(text[start:])
	if part != "" {
		out = append(out, part)
	}
	return out
}

func consumeFormalGroupedType(text string, start int) (int, bool) {
	if start >= len(text) || text[start] != '(' {
		return 0, false
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}

	return 0, false
}
