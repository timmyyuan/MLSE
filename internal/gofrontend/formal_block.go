package gofrontend

import (
	"go/ast"
	"strings"
)

// emitFormalFuncBlock walks a function body and applies returning-region matchers before fallback dispatch.
func emitFormalFuncBlock(stmts []ast.Stmt, env *formalEnv, resultTypes []string) (string, bool) {
	var buf strings.Builder
	for i := 0; i < len(stmts); i++ {
		if ifStmt, ok := stmts[i].(*ast.IfStmt); ok {
			var next ast.Stmt
			if i+1 < len(stmts) {
				next = stmts[i+1]
			}
			if text, consumed, term, ok := emitFormalVoidReturningIfStmt(ifStmt, stmts[i+1:], env); ok {
				buf.WriteString(text)
				if term {
					return buf.String(), true
				}
				i += consumed - 1
				continue
			}
			if text, consumed, term, ok := emitFormalTerminatingIfStmt(ifStmt, next, env, resultTypes); ok {
				buf.WriteString(text)
				if term {
					return buf.String(), true
				}
				i += consumed - 1
				continue
			}
			if text, consumed, term, ok := emitFormalReturningIfStmt(ifStmt, stmts[i+1:], env, resultTypes); ok {
				buf.WriteString(text)
				if term {
					return buf.String(), true
				}
				i += consumed - 1
				continue
			}
		}
		if text, consumed, term, ok := emitFormalReturningLoopStmt(stmts[i], stmts[i+1:], env, resultTypes); ok {
			buf.WriteString(text)
			if term {
				return buf.String(), true
			}
			i += consumed - 1
			continue
		}
		text, term := emitFormalStmt(stmts[i], env, resultTypes)
		buf.WriteString(text)
		if term {
			return buf.String(), true
		}
	}
	return buf.String(), false
}

// emitFormalRegionBlock is the block walker used inside nested `scf.if` and loop regions.
func emitFormalRegionBlock(stmts []ast.Stmt, env *formalEnv) (string, bool) {
	var buf strings.Builder
	for i := 0; i < len(stmts); i++ {
		if ifStmt, ok := stmts[i].(*ast.IfStmt); ok {
			var next ast.Stmt
			if i+1 < len(stmts) {
				next = stmts[i+1]
			}
			if text, consumed, term, ok := emitFormalVoidReturningIfStmt(ifStmt, stmts[i+1:], env); ok {
				buf.WriteString(text)
				if term {
					return buf.String(), true
				}
				i += consumed - 1
				continue
			}
			if text, consumed, term, ok := emitFormalTerminatingIfStmt(ifStmt, next, env, env.resultTypes); ok {
				buf.WriteString(text)
				if term {
					return buf.String(), true
				}
				i += consumed - 1
				continue
			}
			if text, consumed, term, ok := emitFormalReturningIfStmt(ifStmt, stmts[i+1:], env, env.resultTypes); ok {
				buf.WriteString(text)
				if term {
					return buf.String(), true
				}
				i += consumed - 1
				continue
			}
		}
		if text, consumed, term, ok := emitFormalReturningLoopStmt(stmts[i], stmts[i+1:], env, env.resultTypes); ok {
			buf.WriteString(text)
			if term {
				return buf.String(), true
			}
			i += consumed - 1
			continue
		}
		text, term := emitFormalStmt(stmts[i], env, nil)
		buf.WriteString(text)
		if term {
			return buf.String(), true
		}
	}
	return buf.String(), false
}
