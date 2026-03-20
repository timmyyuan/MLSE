package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

func emitStmt(stmt ast.Stmt, env *env) string {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		return emitAssignStmt(s, env)
	case *ast.ReturnStmt:
		return emitReturnStmt(s, env)
	case *ast.ExprStmt:
		value, ty, prelude := emitExpr(s.X, env)
		return prelude + fmt.Sprintf("    mlse.expr %s : %s\n", value, ty)
	case *ast.DeclStmt:
		return emitDeclStmt(s, env)
	case *ast.IfStmt:
		return emitIfStmt(s, env)
	case *ast.ForStmt:
		return emitForStmt(s, env)
	case *ast.RangeStmt:
		return emitRangeStmt(s, env)
	case *ast.BranchStmt:
		return fmt.Sprintf("    mlse.branch %q\n", s.Tok.String())
	case *ast.IncDecStmt:
		return emitIncDecStmt(s, env)
	case *ast.SwitchStmt:
		return emitSwitchStmt(s, env)
	case *ast.GoStmt:
		value, ty, prelude := emitExpr(s.Call, env)
		return prelude + fmt.Sprintf("    mlse.go %s : %s\n", value, ty)
	case *ast.DeferStmt:
		value, ty, prelude := emitExpr(s.Call, env)
		return prelude + fmt.Sprintf("    mlse.defer %s : %s\n", value, ty)
	case *ast.LabeledStmt:
		return fmt.Sprintf("    mlse.label @%s\n", sanitizeName(s.Label.Name)) + emitStmt(s.Stmt, env)
	case *ast.EmptyStmt:
		return ""
	default:
		return fmt.Sprintf("    mlse.unsupported_stmt %q\n", shortNodeName(stmt))
	}
}

func emitAssignStmt(s *ast.AssignStmt, env *env) string {
	var buf strings.Builder
	if len(s.Rhs) == 1 && len(s.Lhs) > 1 {
		value, ty, prelude := emitExpr(s.Rhs[0], env)
		buf.WriteString(prelude)
		for i, lhs := range s.Lhs {
			assignedValue := value
			assignedTy := ty
			if i > 0 {
				assignedTy = syntheticAssignType(lhs, env)
				assignedValue = "mlse.zero"
			}
			ident, ok := lhs.(*ast.Ident)
			if ok {
				if ident.Name == "_" {
					continue
				}
				name := env.defineTyped(ident.Name, assignedTy)
				if s.Tok == token.ASSIGN {
					name = env.use(ident.Name)
				}
				buf.WriteString(fmt.Sprintf("    %s = %s : %s\n", name, assignedValue, assignedTy))
				continue
			}
			buf.WriteString(emitStoreTarget(lhs, assignedValue, assignedTy, env))
		}
		return buf.String()
	}
	for i := 0; i < len(s.Lhs) && i < len(s.Rhs); i++ {
		value, ty, prelude := emitExpr(s.Rhs[i], env)
		buf.WriteString(prelude)
		ident, ok := s.Lhs[i].(*ast.Ident)
		if !ok {
			buf.WriteString(emitStoreTarget(s.Lhs[i], value, ty, env))
			continue
		}
		name := env.defineTyped(ident.Name, ty)
		if s.Tok == token.ASSIGN {
			name = env.use(ident.Name)
		}
		buf.WriteString(fmt.Sprintf("    %s = %s : %s\n", name, value, ty))
	}
	if len(s.Lhs) != len(s.Rhs) {
		buf.WriteString(fmt.Sprintf("    mlse.unsupported_stmt %q\n", shortNodeName(s)))
	}
	return buf.String()
}

func emitReturnStmt(s *ast.ReturnStmt, env *env) string {
	if len(s.Results) == 0 {
		return "    return\n"
	}
	var values []string
	var tys []string
	var prelude strings.Builder
	for _, result := range s.Results {
		value, ty, inner := emitExpr(result, env)
		prelude.WriteString(inner)
		if !isAtomicMLIRValue(value) {
			tmp := env.temp("ret")
			prelude.WriteString(fmt.Sprintf("    %s = %s : %s\n", tmp, value, ty))
			value = tmp
		}
		values = append(values, value)
		tys = append(tys, ty)
	}
	return prelude.String() + fmt.Sprintf("    return %s : %s\n", strings.Join(values, ", "), strings.Join(tys, ", "))
}

func emitDeclStmt(s *ast.DeclStmt, env *env) string {
	gen, ok := s.Decl.(*ast.GenDecl)
	if !ok {
		return fmt.Sprintf("    mlse.unsupported_stmt %q\n", shortNodeName(s))
	}
	var buf strings.Builder
	for _, spec := range gen.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			buf.WriteString(fmt.Sprintf("    mlse.unsupported_decl %q\n", shortNodeName(spec)))
			continue
		}
		for i, name := range vs.Names {
			ty := "!go.any"
			if vs.Type != nil {
				ty = goTypeToMLIR(vs.Type)
			}
			ssa := env.defineTyped(name.Name, ty)
			if i < len(vs.Values) {
				value, valueTy, prelude := emitExpr(vs.Values[i], env)
				buf.WriteString(prelude)
				if ty == "!go.any" {
					ty = valueTy
					env.defineTyped(name.Name, ty)
				}
				buf.WriteString(fmt.Sprintf("    %s = %s : %s\n", ssa, value, ty))
			} else {
				buf.WriteString(fmt.Sprintf("    %s = mlse.zero : %s\n", ssa, ty))
			}
		}
	}
	return buf.String()
}

func emitIfStmt(s *ast.IfStmt, env *env) string {
	var buf strings.Builder
	if s.Init != nil {
		buf.WriteString(emitStmt(s.Init, env))
	}
	cond, ty, prelude := emitExpr(s.Cond, env)
	buf.WriteString(prelude)
	buf.WriteString(fmt.Sprintf("    mlse.if %s : %s {\n", cond, ty))
	for _, stmt := range s.Body.List {
		buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
	}
	if s.Else != nil {
		buf.WriteString("    } else {\n")
		switch elseNode := s.Else.(type) {
		case *ast.BlockStmt:
			for _, stmt := range elseNode.List {
				buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
			}
		default:
			buf.WriteString(indentBlock(emitStmt(s.Else, env), 2))
		}
	}
	buf.WriteString("    }\n")
	return buf.String()
}

func emitForStmt(s *ast.ForStmt, env *env) string {
	var buf strings.Builder
	if s.Init != nil {
		buf.WriteString(emitStmt(s.Init, env))
	}
	cond := "true"
	ty := "i1"
	if s.Cond != nil {
		value, valueTy, prelude := emitExpr(s.Cond, env)
		buf.WriteString(prelude)
		cond, ty = value, valueTy
	}
	buf.WriteString(fmt.Sprintf("    mlse.for %s : %s {\n", cond, ty))
	for _, stmt := range s.Body.List {
		buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
	}
	if s.Post != nil {
		buf.WriteString(indentBlock(emitStmt(s.Post, env), 2))
	}
	buf.WriteString("    }\n")
	return buf.String()
}

func emitRangeStmt(s *ast.RangeStmt, env *env) string {
	value, ty, prelude := emitExpr(s.X, env)
	var buf strings.Builder
	buf.WriteString(prelude)
	idx := env.temp("range_idx")
	limit := env.temp("range_len")
	buf.WriteString(fmt.Sprintf("    %s = 0 : i32\n", idx))
	buf.WriteString(fmt.Sprintf("    %s = mlse.call %%len(%s) : i32\n", limit, value))
	buf.WriteString(fmt.Sprintf("    mlse.for arith.cmpi_lt %s, %s : i32 {\n", idx, limit))
	if name, ok := rangeBindingName(s.Key); ok {
		keySSA := env.defineTyped(name, rangeKeyType(ty))
		buf.WriteString(fmt.Sprintf("        %s = %s : %s\n", keySSA, idx, rangeKeyType(ty)))
	}
	if name, ok := rangeBindingName(s.Value); ok {
		valueTy := rangeValueType(ty)
		valueSSA := env.defineTyped(name, valueTy)
		buf.WriteString(fmt.Sprintf("        %s = mlse.index %s[%s] : %s\n", valueSSA, value, idx, valueTy))
	}
	for _, stmt := range s.Body.List {
		buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
	}
	buf.WriteString(fmt.Sprintf("        %s = arith.addi %s, 1 : i32\n", idx, idx))
	buf.WriteString("    }\n")
	return buf.String()
}

func emitSwitchStmt(s *ast.SwitchStmt, env *env) string {
	var buf strings.Builder
	if s.Init != nil {
		buf.WriteString(emitStmt(s.Init, env))
	}
	tag := "mlse.unit"
	ty := "!go.any"
	if s.Tag != nil {
		value, valueTy, prelude := emitExpr(s.Tag, env)
		buf.WriteString(prelude)
		tag, ty = value, valueTy
	}
	buf.WriteString(fmt.Sprintf("    mlse.switch %s : %s {\n", tag, ty))
	for _, stmt := range s.Body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			buf.WriteString(indentBlock(fmt.Sprintf("    mlse.unsupported_stmt %q\n", shortNodeName(stmt)), 1))
			continue
		}
		buf.WriteString(indentBlock(emitCaseClause(clause, env), 1))
	}
	buf.WriteString("    }\n")
	return buf.String()
}

func emitCaseClause(clause *ast.CaseClause, env *env) string {
	var buf strings.Builder
	if len(clause.List) == 0 {
		buf.WriteString("    default {\n")
	} else {
		values := make([]string, 0, len(clause.List))
		caseTy := "!go.any"
		var prelude strings.Builder
		for idx, expr := range clause.List {
			value, ty, inner := emitExpr(expr, env)
			prelude.WriteString(inner)
			values = append(values, value)
			if idx == 0 {
				caseTy = ty
			}
		}
		buf.WriteString(prelude.String())
		buf.WriteString(fmt.Sprintf("    case %s : %s {\n", strings.Join(values, ", "), caseTy))
	}
	for _, stmt := range clause.Body {
		buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
	}
	buf.WriteString("    }\n")
	return buf.String()
}

func emitIncDecStmt(s *ast.IncDecStmt, env *env) string {
	ident, ok := s.X.(*ast.Ident)
	if !ok {
		value, ty, prelude := emitExpr(s.X, env)
		return prelude + fmt.Sprintf("    mlse.%s %s : %s\n", strings.ToLower(s.Tok.String()), value, ty)
	}
	name := env.use(ident.Name)
	ty := env.typeOf(ident.Name)
	if !isIntegerLikeType(ty) {
		return fmt.Sprintf("    mlse.%s %s : %s\n", strings.ToLower(s.Tok.String()), name, ty)
	}
	tmp := env.temp("inc")
	op := "arith.addi"
	if s.Tok == token.DEC {
		op = "arith.subi"
	}
	return fmt.Sprintf("    %s = %s %s, 1 : %s\n    %s = %s : %s\n", tmp, op, name, ty, name, tmp, ty)
}

func emitStoreTarget(lhs ast.Expr, value string, ty string, env *env) string {
	switch target := lhs.(type) {
	case *ast.SelectorExpr:
		ref, _, prelude := emitExpr(target, env)
		return prelude + fmt.Sprintf("    mlse.store_select %s = %s : %s\n", ref, value, ty)
	case *ast.IndexExpr:
		ref, _, prelude := emitExpr(target, env)
		return prelude + fmt.Sprintf("    mlse.store_index %s = %s : %s\n", ref, value, ty)
	case *ast.StarExpr:
		ref, _, prelude := emitExpr(target.X, env)
		return prelude + fmt.Sprintf("    mlse.store_deref %s = %s : %s\n", ref, value, ty)
	default:
		return fmt.Sprintf("    mlse.assign_target %q = %s : %s\n", shortNodeName(lhs), value, ty)
	}
}

func syntheticAssignType(lhs ast.Expr, env *env) string {
	ident, ok := lhs.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return "!go.any"
	}
	switch ident.Name {
	case "ok":
		return "i1"
	case "err", "e":
		return "!go.error"
	default:
		if ty := env.typeOf(ident.Name); ty != "!go.any" {
			return ty
		}
		return "!go.any"
	}
}

func indentBlock(text string, levels int) string {
	if text == "" {
		return ""
	}
	indent := strings.Repeat("  ", levels)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
