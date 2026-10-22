package obligationir

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// IRRefinementMode selects the CSP model the refinement is stated in.
type IRRefinementMode string

// The models a refinement is stated in.
const (
	IRRefinementModeTrace              IRRefinementMode = "trace"
	IRRefinementModeStableFailure      IRRefinementMode = "stable-failure"
	IRRefinementModeFailuresDivergence IRRefinementMode = "failures-divergence"
)

// ParseIRRefinementMode is the model s names: trace, stable-failure or
// failures-divergence.
func ParseIRRefinementMode(s string) (IRRefinementMode, error) {
	switch m := IRRefinementMode(s); m {
	case IRRefinementModeTrace, IRRefinementModeStableFailure, IRRefinementModeFailuresDivergence:
		return m, nil
	}
	return "", fmt.Errorf("obligationir.ParseIRRefinementMode: no model %q (want trace, stable-failure or failures-divergence)", s)
}

// Refinement is the obligation that Spec ⊑ Impl in the model Mode, the
// direction FDR checks, of two diagrams whose clauses are formulas
// (docs/CLAUSE_DESIGN.md, O8). Each side is stated as its livelock-freedom
// obligation is, and its events carry the values of their fields: an event
// of a channel is the channel applied to the value of each field, in order,
// so two events that differ in a value are two events.
type Refinement struct {
	Version  int              `json:"version"`
	Mode     IRRefinementMode `json:"mode"`
	Alphabet []Channel        `json:"alphabet"`
	Spec     LivelockFree     `json:"spec"`
	Impl     LivelockFree     `json:"impl"`
}

// Channel is a visible channel of either side and the number of its fields.
type Channel struct {
	Name  string `json:"name"`
	Arity int    `json:"arity"`
}

// BuildRefinement states spec ⊑ impl in mode. A channel with one
// number of fields in one place and another elsewhere is refused, since its
// events would be of no one type. An end edge is SKIP when its guard holds.
func BuildRefinement(mode IRRefinementMode, spec, impl *core.Diagram) (Refinement, error) {
	specIR, err := BuildLivelockFree(spec)
	if err != nil {
		return Refinement{}, fmt.Errorf("obligationir.BuildRefinement: the spec: %w", err)
	}
	implIR, err := BuildLivelockFree(impl)
	if err != nil {
		return Refinement{}, fmt.Errorf("obligationir.BuildRefinement: the impl: %w", err)
	}
	r := Refinement{Version: IRVersion, Mode: mode, Spec: specIR, Impl: implIR}
	alphabet, err := r.channels()
	if err != nil {
		return Refinement{}, fmt.Errorf("obligationir.BuildRefinement: %w", err)
	}
	r.Alphabet = alphabet
	return r, nil
}

// namedSide is a side of a refinement and what an error calls it.
type namedSide struct {
	name string
	ir   LivelockFree
}

func (r Refinement) sides() []namedSide {
	return []namedSide{{"the spec", r.Spec}, {"the impl", r.Impl}}
}

// channels is the sorted union of the visible channels of both sides.
func (r Refinement) channels() ([]Channel, error) {
	arity := map[string]int{}
	for _, side := range r.sides() {
		for _, e := range side.ir.Edges {
			if e.Tau() {
				continue
			}
			if n, ok := arity[e.Channel]; ok && n != len(e.Fields) {
				return nil, fmt.Errorf("the channel %s has %d fields in one place and %d in another", e.Channel, n, len(e.Fields))
			}
			arity[e.Channel] = len(e.Fields)
		}
	}
	out := make([]Channel, 0, len(arity))
	for name, n := range arity {
		out = append(out, Channel{Name: name, Arity: n})
	}
	slices.SortFunc(out, func(a, b Channel) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// Validate reports, as an error, what makes r no refinement obligation: an
// encoding of another version, an unknown model, a side Validate refuses, or
// an alphabet that is not the channels of the sides.
func (r Refinement) Validate() error {
	if r.Version != IRVersion {
		return fmt.Errorf("obligationir.Refinement.Validate: version %d, want %d", r.Version, IRVersion)
	}
	if _, err := ParseIRRefinementMode(string(r.Mode)); err != nil {
		return fmt.Errorf("obligationir.Refinement.Validate: %w", err)
	}
	for _, side := range r.sides() {
		if err := side.ir.Validate(); err != nil {
			return fmt.Errorf("obligationir.Refinement.Validate: %s: %w", side.name, err)
		}
	}
	want, err := r.channels()
	if err != nil {
		return fmt.Errorf("obligationir.Refinement.Validate: %w", err)
	}
	if !slices.Equal(want, r.Alphabet) {
		return fmt.Errorf("obligationir.Refinement.Validate: the alphabet %v is not the channels of the sides, %v", r.Alphabet, want)
	}
	if _, err := r.Symbols(); err != nil {
		return fmt.Errorf("obligationir.Refinement.Validate: %w", err)
	}
	return nil
}

// Symbols are the constants either side reads; the environment is one, so
// an atom both sides read is one constant.
func (r Refinement) Symbols() ([]Symbol, error) {
	return Symbols(slices.Concat(r.Spec.types(), r.Impl.types()), slices.Concat(r.Spec.Formulas(), r.Impl.Formulas())...)
}

// Declarations are the constants either side reads, as a back end declares
// them; see LivelockFree.Declarations.
func (r Refinement) Declarations(todo string) ([]Declaration, error) {
	syms, err := r.Symbols()
	if err != nil {
		return nil, err
	}
	return declarations(syms, todo), nil
}

// Lemmas are what the builtins of either side mean, each once.
func (r Refinement) Lemmas() []logic.Tree {
	var out []logic.Tree
	for _, a := range slices.Concat(r.Spec.Lemmas, r.Impl.Lemmas) {
		if !slices.ContainsFunc(out, func(b logic.Tree) bool { return logic.EqualTrees(a, b) }) {
			out = append(out, a)
		}
	}
	return out
}

// EventArgs are the values an event of e carries, in the order of its
// fields: a received field is the variable the step chooses, and a sent one
// the variable it names, a value the step has bound already.
func EventArgs(e Edge) []string {
	args := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		args[i] = SpellVar(logic.Var(f.Name))
	}
	return args
}

// EdgeSets are what an edge of a process term reads, spelled in n: enabled is
// that the event ev is one the edge performs from the values before it, of
// some received values of their types for which some values after it pass
// its guard and post; next is, of the values after it (after), that ev is
// such an event and they pass its guard and post. event spells the event
// the edge performs, the channel applied to EventArgs.
func EdgeSets(n Notation, states map[string]State, e Edge, ev, event string) (enabled, next string, after []string, err error) {
	dst, ok := states[e.Dst]
	if !ok {
		return "", "", nil, fmt.Errorf("obligationir.EdgeSets: no state %s", e.Dst)
	}
	after = make([]string, len(dst.Vars))
	for i, v := range dst.Vars {
		after[i] = SpellVar(logic.Var(v + "'"))
	}
	step, err := n.SpellFormula(e.Step)
	if err != nil {
		return "", "", nil, fmt.Errorf("obligationir.EdgeSets: %w", err)
	}
	typedAfter, err := n.Relativize(false, after, dst.Vars, step)
	if err != nil {
		return "", "", nil, fmt.Errorf("obligationir.EdgeSets: %w", err)
	}
	withEvent := func(body string) (string, error) {
		received := e.Received()
		vars := make([]string, len(received))
		for i, f := range received {
			vars[i] = SpellVar(logic.Var(f))
		}
		body = ev + " = " + event + n.And + "(" + body + ")"
		if len(vars) == 0 {
			return body, nil
		}
		typed, err := n.Relativize(false, vars, received, body)
		if err != nil {
			return "", err
		}
		return n.Exists(vars, typed), nil
	}
	someAfter := typedAfter
	if len(after) > 0 {
		someAfter = n.Exists(after, typedAfter)
	}
	if enabled, err = withEvent(someAfter); err != nil {
		return "", "", nil, fmt.Errorf("obligationir.EdgeSets: %w", err)
	}
	if next, err = withEvent(typedAfter); err != nil {
		return "", "", nil, fmt.Errorf("obligationir.EdgeSets: %w", err)
	}
	return enabled, next, after, nil
}

// InitSet is, of the values of the start state of ir (vars), that they are
// of their types and pass the start post, spelled in n.
func InitSet(n Notation, ir LivelockFree) (body string, vars []string, err error) {
	start, ok := ir.StateMap()[ir.Init]
	if !ok {
		return "", nil, fmt.Errorf("obligationir.InitSet: no start state %s", ir.Init)
	}
	vars = make([]string, len(start.Vars))
	for i, v := range start.Vars {
		vars[i] = SpellVar(logic.Var(v + "'"))
	}
	post, err := n.SpellFormula(ir.InitPost)
	if err != nil {
		return "", nil, fmt.Errorf("obligationir.InitSet: %w", err)
	}
	body, err = n.Relativize(false, vars, start.Vars, post)
	if err != nil {
		return "", nil, fmt.Errorf("obligationir.InitSet: %w", err)
	}
	return body, vars, nil
}
