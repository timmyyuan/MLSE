package debugview

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuanting/MLSE/internal/gofrontend"
)

type Snapshot struct {
	SourcePath   string        `json:"sourcePath"`
	SourceName   string        `json:"sourceName"`
	SourceLines  []SourceLine  `json:"sourceLines"`
	Instructions []Instruction `json:"instructions"`
	FormalMLIR   string        `json:"formalMLIR"`
	Summary      Summary       `json:"summary"`
}

type SourceLine struct {
	Number           int    `json:"number"`
	Text             string `json:"text"`
	InstructionCount int    `json:"instructionCount"`
}

type Instruction struct {
	Index  int    `json:"index"`
	Text   string `json:"text"`
	Kind   string `json:"kind"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Scope  string `json:"scope,omitempty"`
	File   string `json:"file,omitempty"`
}

type Summary struct {
	TotalInstructions   int `json:"totalInstructions"`
	LocatedInstructions int `json:"locatedInstructions"`
	TodoInstructions    int `json:"todoInstructions"`
}

var (
	scopedLocPattern = regexp.MustCompile(`loc\("([^"]+)"\("([^"]+)":([0-9]+):([0-9]+)\)\)`)
	plainLocPattern  = regexp.MustCompile(`loc\("([^"]+)":([0-9]+):([0-9]+)\)`)
)

func BuildSnapshot(path string) (Snapshot, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read source: %w", err)
	}
	formal, err := gofrontend.CompileFileFormal(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("compile formal MLIR: %w", err)
	}
	sourceLines := splitSourceLines(string(source))
	instructions := parseInstructions(formal)
	counts := countInstructionsByLine(instructions)
	for i := range sourceLines {
		sourceLines[i].InstructionCount = counts[sourceLines[i].Number]
	}
	return Snapshot{
		SourcePath:   path,
		SourceName:   filepath.Base(path),
		SourceLines:  sourceLines,
		Instructions: instructions,
		FormalMLIR:   formal,
		Summary:      summarizeInstructions(instructions),
	}, nil
}

func NewServer(snapshot Snapshot) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := pageTemplate.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/debug.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(snapshot)
	})
	mux.HandleFunc("/raw.mlir", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(snapshot.FormalMLIR))
	})
	return mux
}

func splitSourceLines(source string) []SourceLine {
	raw := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	out := make([]SourceLine, 0, len(raw))
	for i, text := range raw {
		out = append(out, SourceLine{Number: i + 1, Text: text})
	}
	return out
}

func parseInstructions(formal string) []Instruction {
	lines := strings.Split(strings.ReplaceAll(formal, "\r\n", "\n"), "\n")
	out := make([]Instruction, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		inst := Instruction{Index: len(out), Text: trimmed, Kind: instructionKind(trimmed)}
		fillLocation(&inst, trimmed)
		out = append(out, inst)
	}
	return out
}

func fillLocation(inst *Instruction, text string) {
	if match := scopedLocPattern.FindStringSubmatch(text); len(match) == 5 {
		inst.Scope = match[1]
		inst.File = match[2]
		inst.Line = atoi(match[3])
		inst.Column = atoi(match[4])
		return
	}
	if match := plainLocPattern.FindStringSubmatch(text); len(match) == 4 {
		inst.File = match[1]
		inst.Line = atoi(match[2])
		inst.Column = atoi(match[3])
	}
}

func instructionKind(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "module") {
		return "module"
	}
	if strings.HasPrefix(trimmed, "}") {
		return "region"
	}
	if strings.HasPrefix(trimmed, "return") {
		return "return"
	}
	if strings.Contains(trimmed, " go.todo") || strings.Contains(trimmed, "= go.todo") || strings.HasPrefix(trimmed, "go.todo") {
		return "todo"
	}
	if eq := strings.Index(trimmed, " = "); eq >= 0 {
		trimmed = strings.TrimSpace(trimmed[eq+3:])
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "op"
	}
	op := fields[0]
	if dot := strings.Index(op, "."); dot > 0 {
		return op[:dot]
	}
	return strings.Trim(op, "@")
}

func countInstructionsByLine(instructions []Instruction) map[int]int {
	out := make(map[int]int)
	for _, inst := range instructions {
		if inst.Line > 0 {
			out[inst.Line]++
		}
	}
	return out
}

func summarizeInstructions(instructions []Instruction) Summary {
	summary := Summary{TotalInstructions: len(instructions)}
	for _, inst := range instructions {
		if inst.Line > 0 {
			summary.LocatedInstructions++
		}
		if inst.Kind == "todo" {
			summary.TodoInstructions++
		}
	}
	return summary
}

func atoi(text string) int {
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}
	return value
}

var pageTemplate = template.Must(template.New("debug").Parse(debugHTML))
