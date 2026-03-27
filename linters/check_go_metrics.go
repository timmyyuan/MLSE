package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type violation struct {
	path string
	line int
	msg  string
}

type parsedGoFile struct {
	path      string
	rel       string
	lineCount int
	parsed    *ast.File
}

type localFuncInfo struct {
	name string
	path string
	line int
	decl *ast.FuncDecl
}

func main() {
	root := flag.String("root", ".", "repository root")
	includeCSV := flag.String("include", "cmd,internal,examples,linters", "comma-separated include directories")
	excludeCSV := flag.String("exclude", "testdata,tmp,artifacts,.git", "comma-separated directory names to skip")
	maxParams := flag.Int("max-params", 5, "maximum function parameters")
	maxFunctionLines := flag.Int("max-function-lines", 200, "maximum function length in lines")
	maxFileLines := flag.Int("max-file-lines", 2000, "maximum file length in lines")
	flag.Parse()

	includes := splitCSV(*includeCSV)
	excludes := sliceToSet(splitCSV(*excludeCSV))
	files, err := collectGoFiles(*root, includes, excludes)
	if err != nil {
		exitErr(err)
	}

	violations, err := checkGoFiles(*root, files, *maxParams, *maxFunctionLines, *maxFileLines)
	if err != nil {
		exitErr(err)
	}
	if len(violations) == 0 {
		return
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].path != violations[j].path {
			return violations[i].path < violations[j].path
		}
		if violations[i].line != violations[j].line {
			return violations[i].line < violations[j].line
		}
		return violations[i].msg < violations[j].msg
	})

	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "%s:%d: %s\n", v.path, v.line, v.msg)
	}
	os.Exit(1)
}

func splitCSV(text string) []string {
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func sliceToSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		out[item] = struct{}{}
	}
	return out
}

func collectGoFiles(root string, includes []string, excludes map[string]struct{}) ([]string, error) {
	var files []string
	for _, include := range includes {
		base := filepath.Join(root, include)
		info, err := os.Stat(base)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if shouldSkip(path, root, excludes) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() && filepath.Ext(path) == ".go" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func shouldSkip(path string, root string, excludes map[string]struct{}) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if _, ok := excludes[part]; ok {
			return true
		}
	}
	return false
}

func checkGoFiles(root string, files []string, maxParams int, maxFunctionLines int, maxFileLines int) ([]violation, error) {
	parsedFiles, fset, err := parseGoFiles(root, files)
	if err != nil {
		return nil, err
	}

	out := checkGoFileMetrics(parsedFiles, fset, maxParams, maxFunctionLines, maxFileLines)
	out = append(out, checkSingleUseWrapperViolations(parsedFiles, fset)...)
	return out, nil
}

func parseGoFiles(root string, files []string) ([]parsedGoFile, *token.FileSet, error) {
	fset := token.NewFileSet()
	parsedFiles := make([]parsedGoFile, 0, len(files))
	for _, path := range files {
		data, err := os.Open(path)
		if err != nil {
			return nil, nil, err
		}
		lineCount, countErr := countLines(data)
		closeErr := data.Close()
		if countErr != nil {
			return nil, nil, countErr
		}
		if closeErr != nil {
			return nil, nil, closeErr
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, nil, err
		}

		parsed, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: parse failed: %w", filepath.ToSlash(rel), err)
		}

		parsedFiles = append(parsedFiles, parsedGoFile{
			path:      path,
			rel:       filepath.ToSlash(rel),
			lineCount: lineCount,
			parsed:    parsed,
		})
	}
	return parsedFiles, fset, nil
}

func checkGoFileMetrics(files []parsedGoFile, fset *token.FileSet, maxParams int, maxFunctionLines int, maxFileLines int) []violation {
	var out []violation
	for _, file := range files {
		if file.lineCount > maxFileLines {
			out = append(out, violation{
				path: file.rel,
				line: 1,
				msg:  fmt.Sprintf("file has %d lines, exceeds limit %d", file.lineCount, maxFileLines),
			})
		}

		for _, decl := range file.parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			start := fset.Position(fn.Pos()).Line
			end := fset.Position(fn.End()).Line
			params := countFieldParams(fn.Type.Params)
			length := end - start + 1
			name := fn.Name.Name

			if params > maxParams {
				out = append(out, violation{
					path: file.rel,
					line: start,
					msg:  fmt.Sprintf("function %q has %d parameters, exceeds limit %d", name, params, maxParams),
				})
			}
			if length > maxFunctionLines {
				out = append(out, violation{
					path: file.rel,
					line: start,
					msg:  fmt.Sprintf("function %q has %d lines, exceeds limit %d", name, length, maxFunctionLines),
				})
			}
		}
	}
	return out
}

func checkSingleUseWrapperViolations(files []parsedGoFile, fset *token.FileSet) []violation {
	funcsByPackage := collectPackageFuncs(files, fset)
	callCounts := collectDirectCallCounts(files, funcsByPackage)

	var out []violation
	for pkgKey, funcs := range funcsByPackage {
		for _, fn := range funcs {
			callee, ok := forwardedWrapperCallee(fn.decl)
			if !ok {
				continue
			}
			if _, ok := funcs[callee]; !ok {
				continue
			}
			if callCounts[pkgKey][fn.name] != 1 || callCounts[pkgKey][callee] != 1 {
				continue
			}
			out = append(out, violation{
				path: fn.path,
				line: fn.line,
				msg:  fmt.Sprintf("function %q is a single-use wrapper around single-use callee %q; inline one of them", fn.name, callee),
			})
		}
	}
	return out
}

func collectPackageFuncs(files []parsedGoFile, fset *token.FileSet) map[string]map[string]localFuncInfo {
	out := make(map[string]map[string]localFuncInfo)
	for _, file := range files {
		key := packageKey(file)
		if _, ok := out[key]; !ok {
			out[key] = make(map[string]localFuncInfo)
		}
		for _, decl := range file.parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || ast.IsExported(fn.Name.Name) {
				continue
			}
			out[key][fn.Name.Name] = localFuncInfo{
				name: fn.Name.Name,
				path: file.rel,
				line: fset.Position(fn.Pos()).Line,
				decl: fn,
			}
		}
	}
	return out
}

func collectDirectCallCounts(files []parsedGoFile, funcsByPackage map[string]map[string]localFuncInfo) map[string]map[string]int {
	counts := make(map[string]map[string]int)
	for _, file := range files {
		key := packageKey(file)
		known := funcsByPackage[key]
		if len(known) == 0 {
			continue
		}
		if _, ok := counts[key]; !ok {
			counts[key] = make(map[string]int)
		}
		ast.Inspect(file.parsed, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			if _, ok := known[ident.Name]; ok {
				counts[key][ident.Name]++
			}
			return true
		})
	}
	return counts
}

func packageKey(file parsedGoFile) string {
	return filepath.ToSlash(filepath.Dir(file.rel)) + ":" + file.parsed.Name.Name
}

func forwardedWrapperCallee(fn *ast.FuncDecl) (string, bool) {
	if fn == nil || fn.Recv != nil || ast.IsExported(fn.Name.Name) || fn.Body == nil || len(fn.Body.List) != 1 {
		return "", false
	}
	call, ok := forwardedCallFromStmt(fn.Body.List[0], fn.Type.Results)
	if !ok {
		return "", false
	}
	if call.Ellipsis.IsValid() {
		return "", false
	}
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ast.IsExported(ident.Name) || ident.Name == fn.Name.Name {
		return "", false
	}
	if !forwardedArgsMatchParams(call.Args, fn.Type.Params) {
		return "", false
	}
	return ident.Name, true
}

func forwardedCallFromStmt(stmt ast.Stmt, results *ast.FieldList) (*ast.CallExpr, bool) {
	switch node := stmt.(type) {
	case *ast.ReturnStmt:
		if countFieldParams(results) != 1 || len(node.Results) != 1 {
			return nil, false
		}
		call, ok := node.Results[0].(*ast.CallExpr)
		return call, ok
	case *ast.ExprStmt:
		if countFieldParams(results) != 0 {
			return nil, false
		}
		call, ok := node.X.(*ast.CallExpr)
		return call, ok
	default:
		return nil, false
	}
}

func forwardedArgsMatchParams(args []ast.Expr, params *ast.FieldList) bool {
	paramNames, ok := collectParamNames(params)
	if !ok || len(args) != len(paramNames) {
		return false
	}
	for i, arg := range args {
		ident, ok := arg.(*ast.Ident)
		if !ok || ident.Name != paramNames[i] {
			return false
		}
	}
	return true
}

func collectParamNames(fields *ast.FieldList) ([]string, bool) {
	if fields == nil || len(fields.List) == 0 {
		return nil, true
	}
	names := make([]string, 0, countFieldParams(fields))
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			return nil, false
		}
		for _, name := range field.Names {
			if name.Name == "_" {
				return nil, false
			}
			names = append(names, name.Name)
		}
	}
	return names, true
}

func countLines(file *os.File) (int, error) {
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

func countFieldParams(fields *ast.FieldList) int {
	if fields == nil {
		return 0
	}
	count := 0
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			count++
			continue
		}
		count += len(field.Names)
	}
	return count
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
