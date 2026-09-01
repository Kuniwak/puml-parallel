package csdf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Op is the tag distinguishing the process expressions of a composition tree.
type Op string

const (
	OpRefer             Op = "REFER"
	OpHide              Op = "HIDE"
	OpInterfaceParallel Op = "INTERFACE_PARALLEL"
)

// Expr is a process expression of a composition tree: a reference to an
// existing diagram, a hiding, or an interface parallel composition.
type Expr interface {
	// Op returns the tag of the expression.
	Op() Op
}

// ReferExpr refers to a Composable State Diagram stored at Path.
type ReferExpr struct {
	Path string `json:"path"`
}

func (e *ReferExpr) Op() Op { return OpRefer }

// HideExpr hides Events of Proc (CSP hiding, P \ A).
type HideExpr struct {
	Events []Event `json:"events"`
	Proc   Expr    `json:"proc"`
}

func (e *HideExpr) Op() Op { return OpHide }

// InterfaceParallelExpr composes Procs in parallel synchronising on Sync
// (CSP interface parallel).
type InterfaceParallelExpr struct {
	Sync  []Event `json:"sync"`
	Procs []Expr  `json:"procs"`
}

func (e *InterfaceParallelExpr) Op() Op { return OpInterfaceParallel }

// ParseExpr parses the JSON representation of a composition tree.
func ParseExpr(bs []byte) (Expr, error) {
	var raw json.RawMessage
	if err := json.Unmarshal(bs, &raw); err != nil {
		return nil, fmt.Errorf("csdf.ParseExpr: %w", err)
	}
	return parseExpr(raw)
}

func parseExpr(raw json.RawMessage) (Expr, error) {
	var tagged struct {
		Op Op `json:"op"`
	}
	if err := json.Unmarshal(raw, &tagged); err != nil {
		return nil, fmt.Errorf("cannot read the op of a process expression: %w", err)
	}

	switch tagged.Op {
	case OpRefer:
		var e ReferExpr
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("cannot read a %s expression: %w", OpRefer, err)
		}
		if e.Path == "" {
			return nil, fmt.Errorf("a %s expression requires a non-empty path", OpRefer)
		}
		return &e, nil

	case OpHide:
		var e struct {
			Events []Event         `json:"events"`
			Proc   json.RawMessage `json:"proc"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("cannot read a %s expression: %w", OpHide, err)
		}
		proc, err := parseExpr(e.Proc)
		if err != nil {
			return nil, fmt.Errorf("in the proc of a %s expression: %w", OpHide, err)
		}
		return &HideExpr{Events: e.Events, Proc: proc}, nil

	case OpInterfaceParallel:
		var e struct {
			Sync  []Event           `json:"sync"`
			Procs []json.RawMessage `json:"procs"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("cannot read an %s expression: %w", OpInterfaceParallel, err)
		}
		if len(e.Procs) < 1 {
			return nil, fmt.Errorf("an %s expression requires at least one proc", OpInterfaceParallel)
		}
		procs := make([]Expr, 0, len(e.Procs))
		for i, rawProc := range e.Procs {
			proc, err := parseExpr(rawProc)
			if err != nil {
				return nil, fmt.Errorf("in procs[%d] of an %s expression: %w", i, OpInterfaceParallel, err)
			}
			procs = append(procs, proc)
		}
		return &InterfaceParallelExpr{Sync: e.Sync, Procs: procs}, nil

	default:
		return nil, fmt.Errorf("unknown op: %q", tagged.Op)
	}
}

// DiagramLoader resolves the path of a REFER expression to a diagram.
type DiagramLoader func(path string) (*Diagram, error)

// NewFileDiagramLoader loads referenced diagrams from the file system,
// resolving relative paths against baseDir.
func NewFileDiagramLoader(baseDir string) DiagramLoader {
	return func(path string) (*Diagram, error) {
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		bs, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read file: %w", err)
		}
		diagram, err := ParseBytes(bs)
		if err != nil {
			return nil, fmt.Errorf("cannot parse file %q: %w", path, err)
		}
		return diagram, nil
	}
}

// ComposeTree evaluates a composition tree into a single diagram, loading the
// diagrams of its REFER leaves with load.
func ComposeTree(expr Expr, load DiagramLoader) (*Diagram, error) {
	diagram, err := composeTree(expr, load)
	if err != nil {
		return nil, fmt.Errorf("csdf.ComposeTree: %w", err)
	}
	return diagram, nil
}

func composeTree(expr Expr, load DiagramLoader) (*Diagram, error) {
	switch e := expr.(type) {
	case *ReferExpr:
		diagram, err := load(e.Path)
		if err != nil {
			return nil, fmt.Errorf("in a %s expression of %q: %w", OpRefer, e.Path, err)
		}
		return diagram, nil

	case *HideExpr:
		proc, err := composeTree(e.Proc, load)
		if err != nil {
			return nil, err
		}
		return Hide(proc, e.Events), nil

	case *InterfaceParallelExpr:
		procs := make([]*Diagram, 0, len(e.Procs))
		for _, procExpr := range e.Procs {
			proc, err := composeTree(procExpr, load)
			if err != nil {
				return nil, err
			}
			procs = append(procs, proc)
		}
		composite, err := ComposeParallel(procs, e.Sync)
		if err != nil {
			return nil, fmt.Errorf("in an %s expression: %w", OpInterfaceParallel, err)
		}
		return composite, nil

	default:
		return nil, fmt.Errorf("unknown process expression: %T", expr)
	}
}
