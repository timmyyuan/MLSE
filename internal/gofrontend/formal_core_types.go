package gofrontend

import "go/ast"

type formalBinding struct {
	current string
	ty      string
	funcSig *formalFuncSig
}

type formalFuncSig struct {
	params  []string
	results []string
}

type formalExternDecl struct {
	symbol  string
	params  []string
	results []string
}

type formalScopeEntry struct {
	ID     int
	Label  string
	Parent int
	Kind   string
	Name   string
	File   string
	Line   int
	Column int
}

type formalFuncBodySpec struct {
	name      string
	recv      *ast.FieldList
	fnType    *ast.FuncType
	body      *ast.BlockStmt
	private   bool
	scopeNode ast.Node
}

type formalHelperCallSpec struct {
	base       string
	args       []string
	argTys     []string
	resultTy   string
	tempPrefix string
}

func formalFuncSigForType(ty string) *formalFuncSig {
	sig, ok := parseFormalFuncType(ty)
	if !ok {
		return nil
	}
	return cloneFormalFuncSig(sig)
}

func cloneFormalFuncSig(sig formalFuncSig) *formalFuncSig {
	return &formalFuncSig{
		params:  append([]string(nil), sig.params...),
		results: append([]string(nil), sig.results...),
	}
}
