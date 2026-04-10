package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
)

func normalizeFormalTopLevelLabels(stmts []ast.Stmt) []ast.Stmt {
	return normalizeFormalTopLevelLabelsWithReserved(stmts, collectFormalReservedNamesFromStmts(stmts))
}

func normalizeFormalTopLevelLabelsWithReserved(stmts []ast.Stmt, reserved map[string]struct{}) []ast.Stmt {
	return normalizeFormalLabelBlock(stmts, reserved)
}

func normalizeFormalLabelBlock(stmts []ast.Stmt, reserved map[string]struct{}) []ast.Stmt {
	if len(stmts) == 0 {
		return stmts
	}

	out := make([]ast.Stmt, 0, len(stmts))
	for _, stmt := range stmts {
		out = append(out, normalizeFormalNestedLabelsStmt(stmt, reserved))
	}
	if len(out) == 0 {
		return out
	}

	for {
		changed := false

		for i := 1; i < len(out); i++ {
			labeled, ok := out[i].(*ast.LabeledStmt)
			if !ok || labeled.Label == nil || labeled.Stmt == nil {
				continue
			}
			label := labeled.Label.Name
			if label == "" {
				continue
			}
			if formalStmtListContainsGotoLabel(out[:i-1], label) || formalStmtListContainsGotoLabel(out[i+1:], label) {
				continue
			}

			prev, localChanged := rewriteFormalForwardGotoTarget(out[i-1], label)
			if !localChanged || formalStmtContainsGotoLabel(prev, label) {
				continue
			}

			out[i-1] = normalizeFormalNestedLabelsStmt(prev, reserved)
			out[i] = normalizeFormalNestedLabelsStmt(labeled.Stmt, reserved)
			changed = true
		}

		backwardChanged := false
		for i := len(out) - 1; i >= 0; i-- {
			labeled, ok := out[i].(*ast.LabeledStmt)
			if !ok || labeled.Label == nil || labeled.Stmt == nil {
				continue
			}
			label := labeled.Label.Name
			if label == "" {
				continue
			}
			if formalStmtListContainsGotoLabel(out[:i], label) {
				continue
			}
			if !formalStmtListContainsGotoLabel(out[i:], label) {
				out[i] = normalizeFormalNestedLabelsStmt(labeled.Stmt, reserved)
				changed = true
				continue
			}

			flagName := allocateFormalGotoRestartFlagName(label, reserved)
			region, ok := rewriteFormalBackwardGotoRegion(append([]ast.Stmt{labeled.Stmt}, out[i+1:]...), label, flagName)
			if !ok {
				continue
			}
			loopRegion, exitSuffix := splitFormalBackwardGotoExitSuffix(region, flagName)
			restartDecl := formalGotoRestartDecl(flagName)
			restartLoop := formalGotoRestartLoop(flagName, loopRegion)
			next := append([]ast.Stmt(nil), out[:i]...)
			next = append(next, restartDecl, restartLoop)
			next = append(next, exitSuffix...)
			out = next
			changed = true
			backwardChanged = true
			break
		}

		if !changed {
			return out
		}
		if backwardChanged {
			continue
		}
	}
}

func normalizeFormalNestedLabelsStmt(stmt ast.Stmt, reserved map[string]struct{}) ast.Stmt {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		clone := *s
		clone.List = normalizeFormalLabelBlock(clone.List, reserved)
		return &clone
	case *ast.IfStmt:
		clone := *s
		if clone.Body != nil {
			body := *clone.Body
			body.List = normalizeFormalLabelBlock(body.List, reserved)
			clone.Body = &body
		}
		switch elseNode := clone.Else.(type) {
		case *ast.BlockStmt:
			elseClone := *elseNode
			elseClone.List = normalizeFormalLabelBlock(elseClone.List, reserved)
			clone.Else = &elseClone
		case *ast.IfStmt:
			clone.Else = normalizeFormalNestedLabelsStmt(elseNode, reserved)
		}
		return &clone
	case *ast.ForStmt:
		clone := *s
		if clone.Body != nil {
			body := *clone.Body
			body.List = normalizeFormalLabelBlock(body.List, reserved)
			clone.Body = &body
		}
		return &clone
	case *ast.RangeStmt:
		clone := *s
		if clone.Body != nil {
			body := *clone.Body
			body.List = normalizeFormalLabelBlock(body.List, reserved)
			clone.Body = &body
		}
		return &clone
	case *ast.LabeledStmt:
		clone := *s
		clone.Stmt = normalizeFormalNestedLabelsStmt(clone.Stmt, reserved)
		return &clone
	default:
		return stmt
	}
}

func collectFormalReservedNames(nodes ...ast.Node) map[string]struct{} {
	names := make(map[string]struct{})
	for _, node := range nodes {
		if formalIsNilNode(node) {
			continue
		}
		ast.Inspect(node, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if ok && ident.Name != "" && ident.Name != "_" {
				names[ident.Name] = struct{}{}
			}
			return true
		})
	}
	return names
}

func formalIsNilNode(node ast.Node) bool {
	if node == nil {
		return true
	}
	value := reflect.ValueOf(node)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func collectFormalReservedNamesFromStmts(stmts []ast.Stmt) map[string]struct{} {
	names := make(map[string]struct{})
	for _, stmt := range stmts {
		for name := range collectFormalReservedNames(stmt) {
			names[name] = struct{}{}
		}
	}
	return names
}

func allocateFormalGotoRestartFlagName(label string, reserved map[string]struct{}) string {
	if reserved == nil {
		reserved = make(map[string]struct{})
	}
	base := fmt.Sprintf("__mlse_goto_restart_%s", sanitizeName(label))
	candidate := base
	for suffix := 1; ; suffix++ {
		if _, exists := reserved[candidate]; !exists {
			reserved[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d", base, suffix)
	}
}

func formalGotoRestartDecl(flagName string) ast.Stmt {
	return &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{ast.NewIdent(flagName)},
					Type:  ast.NewIdent("bool"),
				},
			},
		},
	}
}

func formalGotoRestartLoop(flagName string, region []ast.Stmt) ast.Stmt {
	body := make([]ast.Stmt, 0, len(region)+3)
	body = append(body, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(flagName)},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent("false")},
	})
	body = append(body, region...)
	body = append(body, &ast.BranchStmt{Tok: token.BREAK})
	return &ast.ForStmt{
		Cond: ast.NewIdent("true"),
		Body: &ast.BlockStmt{List: body},
	}
}

func formalSyntheticGotoRestartFlagName(s *ast.ForStmt) (string, bool) {
	if s == nil || s.Init != nil || s.Post != nil || s.Body == nil {
		return "", false
	}
	cond, ok := s.Cond.(*ast.Ident)
	if !ok || cond.Name != "true" || len(s.Body.List) < 2 {
		return "", false
	}
	assign, ok := s.Body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return "", false
	}
	flag, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || !strings.HasPrefix(flag.Name, "__mlse_goto_restart_") {
		return "", false
	}
	reset, ok := assign.Rhs[0].(*ast.Ident)
	if !ok || reset.Name != "false" {
		return "", false
	}
	last, ok := s.Body.List[len(s.Body.List)-1].(*ast.BranchStmt)
	if !ok || last.Tok != token.BREAK || last.Label != nil {
		return "", false
	}
	return flag.Name, true
}

func rewriteFormalBackwardGotoRegion(stmts []ast.Stmt, label string, flagName string) ([]ast.Stmt, bool) {
	list, _, ok := rewriteFormalBackwardGotoStmtList(stmts, label, flagName, 0)
	return list, ok
}

func rewriteFormalBackwardGotoStmtList(stmts []ast.Stmt, label string, flagName string, loopDepth int) ([]ast.Stmt, bool, bool) {
	if len(stmts) == 0 {
		return nil, false, true
	}
	out := make([]ast.Stmt, 0, len(stmts)*2)
	changed := false
	for _, stmt := range stmts {
		if formalIsDirectGotoToLabel(stmt, label) {
			out = append(out, formalGotoRestartAssign(flagName))
			out = append(out, formalGotoRestartBranch(loopDepth))
			changed = true
			continue
		}
		next, stmtChanged, ok := rewriteFormalBackwardGotoStmt(stmt, label, flagName, loopDepth)
		if !ok {
			return nil, false, false
		}
		out = append(out, next)
		if stmtChanged {
			changed = true
			out = append(out, formalGotoPropagationStmt(flagName, loopDepth))
		}
	}
	return out, changed, true
}

func formalIsDirectGotoToLabel(stmt ast.Stmt, label string) bool {
	branch, ok := stmt.(*ast.BranchStmt)
	return ok && branch.Tok == token.GOTO && branch.Label != nil && branch.Label.Name == label
}

func formalGotoRestartAssign(flagName string) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(flagName)},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent("true")},
	}
}

func formalGotoRestartBranch(loopDepth int) ast.Stmt {
	tok := token.CONTINUE
	if loopDepth > 0 {
		tok = token.BREAK
	}
	return &ast.BranchStmt{Tok: tok}
}

func rewriteFormalBackwardGotoStmt(stmt ast.Stmt, label string, flagName string, loopDepth int) (ast.Stmt, bool, bool) {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		list, changed, ok := rewriteFormalBackwardGotoStmtList(s.List, label, flagName, loopDepth)
		if !ok {
			return nil, false, false
		}
		if !changed {
			return stmt, false, true
		}
		clone := *s
		clone.List = list
		return &clone, true, true
	case *ast.IfStmt:
		body, bodyChanged, ok := rewriteFormalBackwardGotoStmt(s.Body, label, flagName, loopDepth)
		if !ok {
			return nil, false, false
		}
		var (
			elseStmt    ast.Stmt
			elseChanged bool
		)
		if s.Else != nil {
			elseStmt, elseChanged, ok = rewriteFormalBackwardGotoStmt(s.Else, label, flagName, loopDepth)
			if !ok {
				return nil, false, false
			}
		}
		if !bodyChanged && !elseChanged {
			return stmt, false, true
		}
		clone := *s
		clone.Body = body.(*ast.BlockStmt)
		clone.Else = elseStmt
		return &clone, true, true
	case *ast.ForStmt:
		body, changed, ok := rewriteFormalBackwardGotoStmt(s.Body, label, flagName, loopDepth+1)
		if !ok {
			return nil, false, false
		}
		if !changed {
			return stmt, false, true
		}
		clone := *s
		clone.Body = body.(*ast.BlockStmt)
		return &clone, true, true
	case *ast.RangeStmt:
		body, changed, ok := rewriteFormalBackwardGotoStmt(s.Body, label, flagName, loopDepth+1)
		if !ok {
			return nil, false, false
		}
		if !changed {
			return stmt, false, true
		}
		clone := *s
		clone.Body = body.(*ast.BlockStmt)
		return &clone, true, true
	case *ast.LabeledStmt:
		if s.Label != nil && s.Label.Name == label {
			next, changed, ok := rewriteFormalBackwardGotoStmt(s.Stmt, label, flagName, loopDepth)
			if !ok {
				return nil, false, false
			}
			return next, changed, true
		}
		if formalStmtContainsGotoLabel(s, label) {
			return nil, false, false
		}
		return stmt, false, true
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		if formalStmtContainsGotoLabel(stmt, label) {
			return nil, false, false
		}
		return stmt, false, true
	case *ast.BranchStmt:
		if s.Tok != token.GOTO || s.Label == nil || s.Label.Name != label {
			return stmt, false, true
		}
		return formalGotoRestartAssign(flagName), true, true
	default:
		return stmt, false, true
	}
}

func formalGotoPropagationStmt(flagName string, loopDepth int) ast.Stmt {
	tok := token.CONTINUE
	if loopDepth > 0 {
		tok = token.BREAK
	}
	return &ast.IfStmt{
		Cond: ast.NewIdent(flagName),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.BranchStmt{Tok: tok},
			},
		},
	}
}

func splitFormalBackwardGotoExitSuffix(region []ast.Stmt, flagName string) ([]ast.Stmt, []ast.Stmt) {
	lastPropagation := -1
	for i, stmt := range region {
		if formalIsGotoPropagationStmt(stmt, flagName, token.CONTINUE) {
			lastPropagation = i
		}
	}
	if lastPropagation == -1 || lastPropagation+1 >= len(region) {
		return region, nil
	}
	loopRegion := append([]ast.Stmt(nil), region[:lastPropagation+1]...)
	exitSuffix := append([]ast.Stmt(nil), region[lastPropagation+1:]...)
	return loopRegion, exitSuffix
}

func formalIsGotoPropagationStmt(stmt ast.Stmt, flagName string, tok token.Token) bool {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok || ifStmt.Init != nil || ifStmt.Else != nil {
		return false
	}
	cond, ok := ifStmt.Cond.(*ast.Ident)
	if !ok || cond.Name != flagName || len(ifStmt.Body.List) != 1 {
		return false
	}
	branch, ok := ifStmt.Body.List[0].(*ast.BranchStmt)
	return ok && branch.Tok == tok && branch.Label == nil
}

func formalStmtListContainsGotoLabel(stmts []ast.Stmt, label string) bool {
	for _, stmt := range stmts {
		if formalStmtContainsGotoLabel(stmt, label) {
			return true
		}
	}
	return false
}

func formalStmtContainsGotoLabel(stmt ast.Stmt, label string) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found || n == nil {
			return !found
		}
		switch node := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if node.Tok == token.GOTO && node.Label != nil && node.Label.Name == label {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func rewriteFormalForwardGotoTarget(stmt ast.Stmt, label string) (ast.Stmt, bool) {
	switch stmt.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return rewriteFormalGotoAsBreak(stmt, label)
	default:
		return rewriteFormalGotoAsNoOp(stmt, label)
	}
}

func rewriteFormalGotoAsNoOp(stmt ast.Stmt, label string) (ast.Stmt, bool) {
	return rewriteFormalGotoStmt(stmt, label, formalGotoRewriteModeNoOp, false)
}

func rewriteFormalGotoAsBreak(stmt ast.Stmt, label string) (ast.Stmt, bool) {
	return rewriteFormalGotoStmt(stmt, label, formalGotoRewriteModeBreak, false)
}

type formalGotoRewriteMode int

const (
	formalGotoRewriteModeNoOp formalGotoRewriteMode = iota
	formalGotoRewriteModeBreak
)

func rewriteFormalGotoStmt(stmt ast.Stmt, label string, mode formalGotoRewriteMode, inTargetLoop bool) (ast.Stmt, bool) {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		changed := false
		list := make([]ast.Stmt, len(s.List))
		for i, child := range s.List {
			next, childChanged := rewriteFormalGotoStmt(child, label, mode, inTargetLoop)
			list[i] = next
			changed = changed || childChanged
		}
		if !changed {
			return stmt, false
		}
		clone := *s
		clone.List = list
		return &clone, true
	case *ast.IfStmt:
		body, bodyChanged := rewriteFormalGotoStmt(s.Body, label, mode, inTargetLoop)
		var (
			elseStmt    ast.Stmt
			elseChanged bool
		)
		if s.Else != nil {
			elseStmt, elseChanged = rewriteFormalGotoStmt(s.Else, label, mode, inTargetLoop)
		}
		if !bodyChanged && !elseChanged {
			return stmt, false
		}
		clone := *s
		clone.Body = body.(*ast.BlockStmt)
		clone.Else = elseStmt
		return &clone, true
	case *ast.LabeledStmt:
		next, changed := rewriteFormalGotoStmt(s.Stmt, label, mode, inTargetLoop)
		if !changed {
			return stmt, false
		}
		clone := *s
		clone.Stmt = next
		return &clone, true
	case *ast.ForStmt:
		if mode == formalGotoRewriteModeNoOp || inTargetLoop {
			return stmt, false
		}
		body, changed := rewriteFormalGotoStmt(s.Body, label, mode, true)
		if !changed {
			return stmt, false
		}
		clone := *s
		clone.Body = body.(*ast.BlockStmt)
		return &clone, true
	case *ast.RangeStmt:
		if mode == formalGotoRewriteModeNoOp || inTargetLoop {
			return stmt, false
		}
		body, changed := rewriteFormalGotoStmt(s.Body, label, mode, true)
		if !changed {
			return stmt, false
		}
		clone := *s
		clone.Body = body.(*ast.BlockStmt)
		return &clone, true
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return stmt, false
	case *ast.BranchStmt:
		if s.Tok != token.GOTO || s.Label == nil || s.Label.Name != label {
			return stmt, false
		}
		switch mode {
		case formalGotoRewriteModeNoOp:
			if inTargetLoop {
				return stmt, false
			}
			return &ast.EmptyStmt{Semicolon: s.TokPos, Implicit: true}, true
		case formalGotoRewriteModeBreak:
			if !inTargetLoop {
				return stmt, false
			}
			return &ast.BranchStmt{TokPos: s.TokPos, Tok: token.BREAK}, true
		default:
			return stmt, false
		}
	default:
		return stmt, false
	}
}
