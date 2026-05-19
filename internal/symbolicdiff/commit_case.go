package symbolicdiff

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	defaultEntryWrapper = "MLSEDiffEntry"
	diffPackageName     = "diffcase"
)

type PrepareOptions struct {
	Repo           string
	OldCommit      string
	NewCommit      string
	File           string
	Function       string
	OldFunction    string
	NewFunction    string
	EmitDir        string
	CaseName       string
	ExpectedStatus string
	SliceLength    int
}

type CaseResult struct {
	CaseName     string     `json:"case_name"`
	CaseDir      string     `json:"case_dir"`
	CasesRoot    string     `json:"cases_root"`
	Function     string     `json:"function"`
	Model        string     `json:"model"`
	Source       SourceInfo `json:"source"`
	ChangedFiles []string   `json:"changed_files"`
}

type SourceInfo struct {
	Repo        string `json:"repo"`
	OldCommit   string `json:"old_commit"`
	NewCommit   string `json:"new_commit"`
	Path        string `json:"path"`
	OldFunction string `json:"old_function"`
	NewFunction string `json:"new_function"`
}

type caseMetadata struct {
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	Function        string       `json:"function"`
	ExpectedStatus  string       `json:"expected_status"`
	ExpectedBlocker string       `json:"expected_blocker,omitempty"`
	Inputs          []paramMeta  `json:"inputs"`
	Results         []resultMeta `json:"results"`
	Source          SourceInfo   `json:"source"`
	CModel          *cModel      `json:"c_model,omitempty"`
	KLEEModel       *kleeModel   `json:"klee_model,omitempty"`
}

type paramMeta struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type resultMeta struct {
	Type string `json:"type"`
}

type cModel struct {
	ReturnType string        `json:"return_type"`
	Params     []cModelParam `json:"params"`
}

type cModelParam struct {
	Name  string `json:"name"`
	CType string `json:"ctype"`
}

type kleeModel struct {
	ABI    string           `json:"abi"`
	Params []kleeModelParam `json:"params"`
	Return kleeModelReturn  `json:"return"`
}

type kleeModelParam struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Length int    `json:"length"`
}

type kleeModelReturn struct {
	Type    string `json:"type"`
	Compare string `json:"compare"`
}

type parsedGoFile struct {
	path    string
	source  string
	fset    *token.FileSet
	file    *ast.File
	funcs   map[string]*functionInfo
	ordered []string
}

type functionInfo struct {
	key  string
	name string
	decl *ast.FuncDecl
	text string
	sig  signatureInfo
}

type signatureInfo struct {
	Params   []paramInfo
	Results  []string
	Variadic bool
}

type paramInfo struct {
	Name string
	Type string
}

type selectedDiff struct {
	path        string
	oldFile     *parsedGoFile
	newFile     *parsedGoFile
	oldSupport  []*parsedGoFile
	newSupport  []*parsedGoFile
	oldFunction *functionInfo
	newFunction *functionInfo
	wrapperName string
}

func PrepareCase(ctx context.Context, opts PrepareOptions) (CaseResult, error) {
	opts = normalizePrepareOptions(opts)
	if err := validatePrepareOptions(opts); err != nil {
		return CaseResult{}, err
	}
	changed, err := changedGoFiles(ctx, opts.Repo, opts.OldCommit, opts.NewCommit)
	if err != nil {
		return CaseResult{}, err
	}
	selected, err := selectDiff(ctx, opts, changed)
	if err != nil {
		return CaseResult{}, err
	}
	metadata, model, err := buildMetadata(opts, selected)
	if err != nil {
		return CaseResult{}, err
	}
	opts.CaseName = metadata.Name
	if err := writeCaseFiles(opts, selected, metadata); err != nil {
		return CaseResult{}, err
	}
	caseDir := caseDir(opts)
	return CaseResult{
		CaseName:     metadata.Name,
		CaseDir:      caseDir,
		CasesRoot:    filepath.Dir(caseDir),
		Function:     metadata.Function,
		Model:        model,
		Source:       metadata.Source,
		ChangedFiles: changed,
	}, nil
}

func normalizePrepareOptions(opts PrepareOptions) PrepareOptions {
	if opts.ExpectedStatus == "" {
		opts.ExpectedStatus = "equivalent"
	}
	if opts.SliceLength == 0 {
		opts.SliceLength = 1
	}
	if opts.EmitDir == "" {
		opts.EmitDir = filepath.Join("artifacts", "mlse-diff")
	}
	if opts.Function != "" {
		if opts.OldFunction == "" {
			opts.OldFunction = opts.Function
		}
		if opts.NewFunction == "" {
			opts.NewFunction = opts.Function
		}
	}
	return opts
}

func validatePrepareOptions(opts PrepareOptions) error {
	if opts.Repo == "" || opts.OldCommit == "" || opts.NewCommit == "" {
		return errors.New("repo, old commit and new commit are required")
	}
	if opts.ExpectedStatus != "equivalent" && opts.ExpectedStatus != "counterexample" {
		return fmt.Errorf("expected status must be equivalent or counterexample, got %q", opts.ExpectedStatus)
	}
	if opts.SliceLength < 1 {
		return fmt.Errorf("slice length must be positive, got %d", opts.SliceLength)
	}
	return nil
}

func changedGoFiles(ctx context.Context, repo string, oldCommit string, newCommit string) ([]string, error) {
	out, err := gitOutput(ctx, repo, "diff", "--name-only", "--diff-filter=ACMRT", oldCommit, newCommit, "--", "*.go")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) != "" {
			files = append(files, strings.TrimSpace(line))
		}
	}
	sort.Strings(files)
	return files, nil
}

func selectDiff(ctx context.Context, opts PrepareOptions, changed []string) (*selectedDiff, error) {
	files := changed
	if opts.File != "" {
		files = []string{filepath.ToSlash(opts.File)}
	}
	var candidates []*selectedDiff
	for _, path := range files {
		candidate, err := analyzeFileDiff(ctx, opts, path)
		if err == nil {
			candidates = append(candidates, candidate)
			continue
		}
		if opts.File != "" || opts.Function != "" || opts.OldFunction != "" || opts.NewFunction != "" {
			return nil, err
		}
	}
	if len(candidates) == 1 {
		if err := attachSupportFiles(ctx, opts, changed, candidates[0]); err != nil {
			return nil, err
		}
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		return nil, errors.New("no supported Go function-level diff found; pass -file and -function/-old-function/-new-function if the entry function was renamed")
	}
	return nil, fmt.Errorf("found %d candidate function diffs; pass -file and -function to disambiguate", len(candidates))
}

func attachSupportFiles(ctx context.Context, opts PrepareOptions, files []string, selected *selectedDiff) error {
	for _, path := range files {
		if path == selected.path {
			continue
		}
		oldSupport, err := parseSupportFile(ctx, opts.Repo, opts.OldCommit, path, selected.oldFile.file.Name.Name)
		if err != nil {
			return err
		}
		newSupport, err := parseSupportFile(ctx, opts.Repo, opts.NewCommit, path, selected.newFile.file.Name.Name)
		if err != nil {
			return err
		}
		if oldSupport != nil {
			selected.oldSupport = append(selected.oldSupport, oldSupport)
		}
		if newSupport != nil {
			selected.newSupport = append(selected.newSupport, newSupport)
		}
	}
	return nil
}

func parseSupportFile(ctx context.Context, repo string, commit string, path string, packageName string) (*parsedGoFile, error) {
	ok, err := fileExistsAtCommit(ctx, repo, commit, path)
	if err != nil || !ok {
		return nil, err
	}
	source, err := readFileAtCommit(ctx, repo, commit, path)
	if err != nil {
		return nil, err
	}
	parsed, err := parseGoSource(path, source)
	if err != nil {
		return nil, fmt.Errorf("parse support file %s: %w", path, err)
	}
	if parsed.file.Name.Name != packageName {
		return nil, nil
	}
	return parsed, nil
}

func analyzeFileDiff(ctx context.Context, opts PrepareOptions, path string) (*selectedDiff, error) {
	oldSource, err := readFileAtCommit(ctx, opts.Repo, opts.OldCommit, path)
	if err != nil {
		return nil, fmt.Errorf("read old file %s: %w", path, err)
	}
	newSource, err := readFileAtCommit(ctx, opts.Repo, opts.NewCommit, path)
	if err != nil {
		return nil, fmt.Errorf("read new file %s: %w", path, err)
	}
	oldFile, err := parseGoSource(path, oldSource)
	if err != nil {
		return nil, fmt.Errorf("parse old file %s: %w", path, err)
	}
	newFile, err := parseGoSource(path, newSource)
	if err != nil {
		return nil, fmt.Errorf("parse new file %s: %w", path, err)
	}
	oldFn, newFn, err := chooseFunctions(opts, oldFile, newFile)
	if err != nil {
		return nil, err
	}
	if oldFn.sig.Variadic || newFn.sig.Variadic {
		return nil, errors.New("variadic entry functions are not supported by the current symbolic diff harness")
	}
	if !sameSignature(oldFn.sig, newFn.sig) {
		return nil, errors.New("old/new entry functions must have the same signature")
	}
	wrapper := ""
	if oldFn.name != newFn.name || oldFn.isMethod() || newFn.isMethod() {
		wrapper = chooseWrapperName(oldFile, newFile)
	}
	return &selectedDiff{
		path:        path,
		oldFile:     oldFile,
		newFile:     newFile,
		oldFunction: oldFn,
		newFunction: newFn,
		wrapperName: wrapper,
	}, nil
}

func chooseFunctions(opts PrepareOptions, oldFile *parsedGoFile, newFile *parsedGoFile) (*functionInfo, *functionInfo, error) {
	if opts.OldFunction != "" || opts.NewFunction != "" {
		oldName := firstNonEmpty(opts.OldFunction, opts.Function)
		newName := firstNonEmpty(opts.NewFunction, opts.Function)
		oldFn, err := resolveFunction(oldFile.funcs, oldName)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve old function %q: %w", oldName, err)
		}
		newFn, err := resolveFunction(newFile.funcs, newName)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve new function %q: %w", newName, err)
		}
		return oldFn, newFn, nil
	}
	common := changedCommonFunctions(oldFile, newFile)
	if len(common) != 1 {
		return nil, nil, fmt.Errorf("expected exactly one changed common function, found %d", len(common))
	}
	key := common[0]
	return oldFile.funcs[key], newFile.funcs[key], nil
}

func changedCommonFunctions(oldFile *parsedGoFile, newFile *parsedGoFile) []string {
	var keys []string
	for key, oldFn := range oldFile.funcs {
		newFn, ok := newFile.funcs[key]
		if ok && oldFn.text != newFn.text {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func resolveFunction(funcs map[string]*functionInfo, name string) (*functionInfo, error) {
	if name == "" {
		return nil, errors.New("function name is empty")
	}
	if fn, ok := funcs[name]; ok {
		return fn, nil
	}
	suffix := name[strings.LastIndex(name, ".")+1:]
	var matches []*functionInfo
	for key, fn := range funcs {
		if key == suffix || strings.HasSuffix(key, "."+suffix) {
			matches = append(matches, fn)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("not found in parsed file")
	}
	return nil, fmt.Errorf("ambiguous suffix %q", suffix)
}

func parseGoSource(path string, source string) (*parsedGoFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	parsed := &parsedGoFile{
		path:   path,
		source: source,
		fset:   fset,
		file:   file,
		funcs:  make(map[string]*functionInfo),
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		key := functionKey(fn)
		sig, err := signatureFromFunc(fset, fn)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		parsed.funcs[key] = &functionInfo{
			key:  key,
			name: fn.Name.Name,
			decl: fn,
			text: nodeString(fset, fn),
			sig:  sig,
		}
		parsed.ordered = append(parsed.ordered, key)
	}
	return parsed, nil
}

func signatureFromFunc(fset *token.FileSet, fn *ast.FuncDecl) (signatureInfo, error) {
	params, variadic, err := paramsFromFields(fset, fn.Type.Params)
	if err != nil {
		return signatureInfo{}, err
	}
	if receiver := receiverParamFromFunc(fset, fn, params); receiver != nil {
		params = append([]paramInfo{*receiver}, params...)
	}
	results := resultsFromFields(fset, fn.Type.Results)
	return signatureInfo{Params: params, Results: results, Variadic: variadic}, nil
}

func paramsFromFields(fset *token.FileSet, fields *ast.FieldList) ([]paramInfo, bool, error) {
	if fields == nil {
		return nil, false, nil
	}
	var params []paramInfo
	var variadic bool
	for _, field := range fields.List {
		if _, ok := field.Type.(*ast.Ellipsis); ok {
			variadic = true
		}
		typ := exprString(fset, field.Type)
		if len(field.Names) == 0 {
			params = append(params, paramInfo{Name: fmt.Sprintf("p%d", len(params)), Type: typ})
			continue
		}
		for _, name := range field.Names {
			paramName := name.Name
			if paramName == "_" || paramName == "" {
				paramName = fmt.Sprintf("p%d", len(params))
			}
			params = append(params, paramInfo{Name: paramName, Type: typ})
		}
	}
	return params, variadic, nil
}

func receiverParamFromFunc(fset *token.FileSet, fn *ast.FuncDecl, params []paramInfo) *paramInfo {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return nil
	}
	field := fn.Recv.List[0]
	name := "recv"
	if len(field.Names) > 0 && field.Names[0].Name != "" && field.Names[0].Name != "_" {
		name = field.Names[0].Name
	}
	return &paramInfo{
		Name: uniqueParamName(name, params),
		Type: exprString(fset, field.Type),
	}
}

func uniqueParamName(base string, params []paramInfo) string {
	used := make(map[string]bool)
	for _, param := range params {
		used[param.Name] = true
	}
	if !used[base] {
		return base
	}
	for i := 0; ; i++ {
		name := fmt.Sprintf("%s%d", base, i)
		if !used[name] {
			return name
		}
	}
}

func resultsFromFields(fset *token.FileSet, fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}
	var results []string
	for _, field := range fields.List {
		typ := exprString(fset, field.Type)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			results = append(results, typ)
		}
	}
	return results
}

func buildMetadata(opts PrepareOptions, selected *selectedDiff) (caseMetadata, string, error) {
	entryName := selected.oldFunction.name
	if selected.wrapperName != "" {
		entryName = selected.wrapperName
	}
	source := SourceInfo{
		Repo:        cleanRepoName(opts.Repo),
		OldCommit:   opts.OldCommit,
		NewCommit:   opts.NewCommit,
		Path:        selected.path,
		OldFunction: selected.oldFunction.key,
		NewFunction: selected.newFunction.key,
	}
	metadata := caseMetadata{
		Name:           caseName(opts, selected),
		Description:    fmt.Sprintf("Generated from %s between %s and %s.", selected.path, shortCommit(opts.OldCommit), shortCommit(opts.NewCommit)),
		Function:       diffPackageName + "." + entryName,
		ExpectedStatus: opts.ExpectedStatus,
		Inputs:         paramsMetadata(selected.oldFunction.sig.Params),
		Results:        resultsMetadata(selected.oldFunction.sig.Results),
		Source:         source,
	}
	model := attachKLEEModel(&metadata, selected.oldFunction.sig, opts.SliceLength)
	return metadata, model, nil
}

func attachKLEEModel(metadata *caseMetadata, sig signatureInfo, sliceLength int) string {
	if scalarModelSupported(sig) {
		model := &cModel{ReturnType: "long"}
		for _, param := range sig.Params {
			model.Params = append(model.Params, cModelParam{Name: param.Name, CType: "long"})
		}
		metadata.CModel = model
		return "c_model"
	}
	if sliceI64ModelSupported(sig) {
		metadata.KLEEModel = &kleeModel{
			ABI: "slice_i64",
			Params: []kleeModelParam{{
				Name:   sig.Params[0].Name,
				Type:   "slice_i64",
				Length: sliceLength,
			}},
			Return: kleeModelReturn{Type: "slice_i64", Compare: "logical"},
		}
		return "klee_model:slice_i64"
	}
	if goLLVMModelSupported(sig) {
		model := &kleeModel{
			ABI:    "go_llvm",
			Return: kleeModelReturn{Type: goKLEEType(sig.Results[0]), Compare: "logical"},
		}
		for _, param := range sig.Params {
			model.Params = append(model.Params, kleeModelParam{
				Name:   param.Name,
				Type:   goKLEEType(param.Type),
				Length: sliceLength,
			})
		}
		metadata.KLEEModel = model
		return "klee_model:go_llvm"
	}
	metadata.ExpectedBlocker = "klee_model_unavailable"
	return "unsupported"
}

func scalarModelSupported(sig signatureInfo) bool {
	if len(sig.Results) != 1 || !isScalarInt(sig.Results[0]) {
		return false
	}
	for _, param := range sig.Params {
		if !isScalarInt(param.Type) {
			return false
		}
	}
	return true
}

func sliceI64ModelSupported(sig signatureInfo) bool {
	return len(sig.Params) == 1 && sig.Params[0].Type == "[]int" &&
		len(sig.Results) == 1 && sig.Results[0] == "[]int"
}

func goLLVMModelSupported(sig signatureInfo) bool {
	if len(sig.Results) != 1 || !goLLVMReturnSupported(sig.Results[0]) {
		return false
	}
	for _, param := range sig.Params {
		if !goLLVMParamSupported(param.Type, sig.Results[0]) {
			return false
		}
	}
	return true
}

func goLLVMParamSupported(typ string, result string) bool {
	if result == "error" {
		switch typ {
		case "int", "int64", "bool", "string":
			return true
		default:
			return false
		}
	}
	switch typ {
	case "bool", "string", "[]string":
		return true
	default:
		return false
	}
}

func goLLVMReturnSupported(typ string) bool {
	switch typ {
	case "string", "[]string", "error":
		return true
	default:
		return false
	}
}

func goKLEEType(typ string) string {
	switch typ {
	case "int", "int64":
		return "i64"
	case "bool":
		return "bool"
	case "string":
		return "string"
	case "[]string":
		return "slice_string"
	case "error":
		return "error"
	default:
		return typ
	}
}

func sameSignature(oldSig signatureInfo, newSig signatureInfo) bool {
	if oldSig.Variadic != newSig.Variadic ||
		len(oldSig.Params) != len(newSig.Params) ||
		len(oldSig.Results) != len(newSig.Results) {
		return false
	}
	for i := range oldSig.Params {
		if oldSig.Params[i].Type != newSig.Params[i].Type {
			return false
		}
	}
	for i := range oldSig.Results {
		if oldSig.Results[i] != newSig.Results[i] {
			return false
		}
	}
	return true
}

func isScalarInt(typ string) bool {
	return typ == "int" || typ == "int64"
}

func writeCaseFiles(opts PrepareOptions, selected *selectedDiff, metadata caseMetadata) error {
	dir := caseDir(opts)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	oldFiles := append([]*parsedGoFile{selected.oldFile}, selected.oldSupport...)
	newFiles := append([]*parsedGoFile{selected.newFile}, selected.newSupport...)
	if err := writeRenderedSource(filepath.Join(dir, "old.go"), oldFiles, selected.oldFunction, selected.wrapperName); err != nil {
		return err
	}
	if err := writeRenderedSource(filepath.Join(dir, "new.go"), newFiles, selected.newFunction, selected.wrapperName); err != nil {
		return err
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "case.json"), append(data, '\n'), 0o644)
}

func writeRenderedSource(path string, files []*parsedGoFile, target *functionInfo, wrapper string) error {
	source, err := renderSource(files, target, wrapper)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(source), 0o644)
}

func renderSource(files []*parsedGoFile, target *functionInfo, wrapper string) (string, error) {
	var builder strings.Builder
	builder.WriteString("package ")
	builder.WriteString(diffPackageName)
	builder.WriteString("\n\n")
	imports := collectImports(files)
	if len(imports) > 0 {
		builder.WriteString("import (\n")
		for _, item := range imports {
			builder.WriteString("\t")
			builder.WriteString(item)
			builder.WriteString("\n")
		}
		builder.WriteString(")\n\n")
	}
	decls := renderNonImportDecls(files)
	for i, decl := range decls {
		if i > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(decl)
	}
	if wrapper != "" {
		text, err := wrapperSource(wrapper, target)
		if err != nil {
			return "", err
		}
		if len(decls) > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(text)
	}
	return builder.String() + "\n", nil
}

func collectImports(files []*parsedGoFile) []string {
	seen := make(map[string]bool)
	var imports []string
	for _, file := range files {
		for _, decl := range file.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.IMPORT {
				continue
			}
			for _, spec := range gen.Specs {
				item := formatImport(spec.(*ast.ImportSpec))
				if !seen[item] {
					seen[item] = true
					imports = append(imports, item)
				}
			}
		}
	}
	sort.Strings(imports)
	return imports
}

func formatImport(spec *ast.ImportSpec) string {
	if spec.Name == nil {
		return spec.Path.Value
	}
	return spec.Name.Name + " " + spec.Path.Value
}

func renderNonImportDecls(files []*parsedGoFile) []string {
	var decls []string
	for _, file := range files {
		for _, decl := range file.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if ok && gen.Tok == token.IMPORT {
				continue
			}
			decls = append(decls, nodeString(file.fset, decl))
		}
	}
	return decls
}

func wrapperSource(wrapper string, target *functionInfo) (string, error) {
	params := formatParams(target.sig.Params)
	results := formatResults(target.sig.Results)
	call := wrapperCall(target)
	if len(target.sig.Results) == 0 {
		return fmt.Sprintf("func %s(%s) {\n\t%s\n}\n", wrapper, params, call), nil
	}
	return fmt.Sprintf("func %s(%s) %s {\n\treturn %s\n}\n", wrapper, params, results, call), nil
}

func wrapperCall(target *functionInfo) string {
	if !target.isMethod() {
		return fmt.Sprintf("%s(%s)", target.name, formatArgs(target.sig.Params))
	}
	receiver := target.sig.Params[0].Name
	args := formatArgs(target.sig.Params[1:])
	return fmt.Sprintf("%s.%s(%s)", receiver, target.name, args)
}

func formatParams(params []paramInfo) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, param.Name+" "+param.Type)
	}
	return strings.Join(parts, ", ")
}

func formatResults(results []string) string {
	if len(results) == 0 {
		return ""
	}
	if len(results) == 1 {
		return results[0]
	}
	return "(" + strings.Join(results, ", ") + ")"
}

func formatArgs(params []paramInfo) string {
	args := make([]string, 0, len(params))
	for _, param := range params {
		args = append(args, param.Name)
	}
	return strings.Join(args, ", ")
}

func chooseWrapperName(oldFile *parsedGoFile, newFile *parsedGoFile) string {
	name := defaultEntryWrapper
	for i := 0; ; i++ {
		if i > 0 {
			name = fmt.Sprintf("%s%d", defaultEntryWrapper, i)
		}
		if oldFile.funcs[name] == nil && newFile.funcs[name] == nil {
			return name
		}
	}
}

func paramsMetadata(params []paramInfo) []paramMeta {
	items := make([]paramMeta, 0, len(params))
	for _, param := range params {
		items = append(items, paramMeta(param))
	}
	return items
}

func resultsMetadata(results []string) []resultMeta {
	items := make([]resultMeta, 0, len(results))
	for _, result := range results {
		items = append(items, resultMeta{Type: result})
	}
	return items
}

func caseDir(opts PrepareOptions) string {
	return filepath.Join(opts.EmitDir, "cases", opts.CaseName)
}

func caseName(opts PrepareOptions, selected *selectedDiff) string {
	if opts.CaseName != "" {
		return opts.CaseName
	}
	hash := sha1.Sum([]byte(strings.Join([]string{
		opts.Repo,
		opts.OldCommit,
		opts.NewCommit,
		selected.path,
		selected.oldFunction.key,
		selected.newFunction.key,
	}, "\x00")))
	prefix := strings.TrimSuffix(filepath.Base(selected.path), ".go")
	name := fmt.Sprintf("%s-%s-%s", prefix, selected.oldFunction.name, hex.EncodeToString(hash[:])[:8])
	return sanitizeName(name)
}

func cleanRepoName(repo string) string {
	cleaned := filepath.Clean(repo)
	if base := filepath.Base(cleaned); base != "." && base != string(filepath.Separator) {
		return base
	}
	return repo
}

func shortCommit(commit string) string {
	if len(commit) <= 12 {
		return commit
	}
	return commit[:12]
}

func sanitizeName(text string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9_.-]+`)
	text = re.ReplaceAllString(text, "-")
	text = strings.Trim(text, "-.")
	if text == "" {
		return "case"
	}
	return text
}

func readFileAtCommit(ctx context.Context, repo string, commit string, path string) (string, error) {
	return gitOutput(ctx, repo, "show", commit+":"+filepath.ToSlash(path))
}

func fileExistsAtCommit(ctx context.Context, repo string, commit string, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "cat-file", "-e", commit+":"+filepath.ToSlash(path))
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func gitOutput(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func functionKey(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return receiverName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func receiverName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverName(value.X)
	case *ast.IndexExpr:
		return receiverName(value.X)
	case *ast.IndexListExpr:
		return receiverName(value.X)
	default:
		return "receiver"
	}
}

func nodeString(fset *token.FileSet, node ast.Node) string {
	var buf strings.Builder
	_ = printer.Fprint(&buf, fset, node)
	return buf.String()
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	return nodeString(fset, expr)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (fn *functionInfo) isMethod() bool {
	return fn.decl.Recv != nil && len(fn.decl.Recv.List) > 0
}
