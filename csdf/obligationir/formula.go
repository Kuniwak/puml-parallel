package obligationir

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
)

// LivelockFree is the livelock-freedom obligation of a diagram whose clauses
// are csdf/clause formulas (docs/CLAUSE_DESIGN.md, O8): no reachable state
// admits an infinite run of tau steps. Each clause is the formula of
// csdf/logic that csdf/obligation states, so a back end spells it rather than
// leaving it opaque: an atom is an uninterpreted predicate applied to its
// holes, a builtin an equality or a csdf_* predicate whose meaning Axioms
// states, and an ∃ a quantifier of its type.
type LivelockFree struct {
	// Version is the version of this encoding, IRVersion; a reader refuses
	// another.
	Version int `json:"version"`
	// Structurally is true when no tau cycle is reachable in the graph of
	// the states, so the obligation holds whatever the clauses say.
	Structurally bool `json:"structurally"`
	// States are the states in the order the diagram declares them, each
	// with its state variables.
	States []State `json:"states"`
	// Init is the start state; InitPost the post of the start edge, over the
	// values after it (x').
	Init     string     `json:"init"`
	InitPost logic.Tree `json:"init_post"`
	Edges    []Edge     `json:"edges"`
	// Axioms are what the csdf_* predicates of the clauses mean
	// (obligation.Axioms); a back end states the theorem under them.
	Axioms []logic.Tree `json:"axioms"`
	// Assumptions are the diagram's assume notes, hypotheses of the theorem
	// like the axioms. Never null, only empty.
	Assumptions []IRAssumption `json:"assumptions"`
}

// IRVersion is the version of the encoding of LivelockFree. It changes
// whenever a field changes meaning, so that an IR stored by another release
// is refused rather than misread.
const IRVersion = 1

// Validate reports, as an error, what makes ir no obligation of a diagram: an
// encoding of another version, a state named twice or by no name, a start
// state or an edge naming a state that is not there, or a field received
// twice, a tree that is no formula, a quantifier without types, or one name
// read as two constants.
func (ir LivelockFree) Validate() error {
	if ir.Version != IRVersion {
		return fmt.Errorf("obligationir.Validate: version %d, want %d", ir.Version, IRVersion)
	}
	states := map[string]bool{}
	for _, s := range ir.States {
		if s.Name == "" || states[s.Name] {
			return fmt.Errorf("obligationir.Validate: the state %q is named twice or by nothing", s.Name)
		}
		states[s.Name] = true
	}
	if !states[ir.Init] {
		return fmt.Errorf("obligationir.Validate: no start state %q", ir.Init)
	}
	for _, e := range ir.Edges {
		if !states[e.Src] || !states[e.Dst] {
			return fmt.Errorf("obligationir.Validate: the edge %s -> %s names a state that is not there", e.Src, e.Dst)
		}
		if e.Channel == "" || e.Tau() && len(e.Fields) > 0 {
			return fmt.Errorf("obligationir.Validate: the edge %s -> %s has no channel, or is a tau with fields", e.Src, e.Dst)
		}
		seen := map[string]bool{}
		for _, f := range e.Received() {
			if f == "" || seen[f] {
				return fmt.Errorf("obligationir.Validate: the edge %s -> %s receives %q twice or by no name", e.Src, e.Dst, f)
			}
			seen[f] = true
		}
	}
	for _, f := range ir.Formulas() {
		if _, err := checking.SpellFormula(f); err != nil {
			return fmt.Errorf("obligationir.Validate: %w", err)
		}
	}
	if _, err := ir.Symbols(); err != nil {
		return fmt.Errorf("obligationir.Validate: %w", err)
	}
	return nil
}

// checking is a notation that only checks what it spells.
var checking = Notation{
	Exists: func([]string, string) string { return "" },
	Forall: func([]string, string) string { return "" },
}

// State is a state and the state variables it holds.
type State struct {
	Name string   `json:"name"`
	Vars []string `json:"vars"`
}

// Edge is a step of the diagram: from Src to Dst, on an event of Channel
// with Fields, internal when the channel is core.Tau. Step is its guard and
// post, over the values before it (x), the fields its event receives and
// the values after it (x').
type Edge struct {
	Src     string     `json:"src"`
	Dst     string     `json:"dst"`
	Channel string     `json:"channel"`
	Fields  []Field    `json:"fields"`
	Step    logic.Tree `json:"step"`
}

// Field is a field of an event: received (?Name), which the step chooses, or
// sent (!Name), a value the step has bound already.
type Field struct {
	Send bool   `json:"send"`
	Name string `json:"name"`
}

// Tau reports whether e is internal.
func (e Edge) Tau() bool { return e.Channel == core.Tau }

// Received are the names of the fields e receives, in order.
func (e Edge) Received() []string {
	names := []string{}
	for _, f := range e.Fields {
		if !f.Send {
			names = append(names, f.Name)
		}
	}
	return names
}

func fieldsOf(e core.Event) []Field {
	fs := make([]Field, len(e.Fields))
	for i, f := range e.Fields {
		fs[i] = Field{Send: f.Send, Name: f.Name}
	}
	return fs
}

// BuildFormulaLivelockFree states the livelock-freedom obligation of d.
func BuildFormulaLivelockFree(d *core.Diagram) (LivelockFree, error) {
	post, err := initPost(d)
	if err != nil {
		return LivelockFree{}, fmt.Errorf("obligationir.BuildFormulaLivelockFree: %w", err)
	}
	clauses := []clause.Formula{d.InitPost}
	edges := make([]Edge, 0, len(d.Edges))
	for _, e := range d.Edges {
		step, err := obligation.Step(*d, e)
		if err != nil {
			return LivelockFree{}, fmt.Errorf("obligationir.BuildFormulaLivelockFree: %w", err)
		}
		edges = append(edges, Edge{Src: e.Src, Dst: e.Dst, Channel: e.Event.Channel, Fields: fieldsOf(e.Event), Step: logic.TreeOf(step)})
		clauses = append(clauses, e.Guard, e.Post)
	}
	states := make([]State, len(d.States))
	for i, s := range d.States {
		states[i] = State{Name: s, Vars: append([]string{}, d.Vars[s]...)}
	}
	assumptions := make([]IRAssumption, 0, len(d.Assumes))
	for _, a := range d.Assumes {
		assumptions = append(assumptions, IRAssumption{ID: a.ID, Text: csdf.Predicate(a.Text)})
	}
	return LivelockFree{
		Version:      IRVersion,
		Structurally: !d.TauCycleReachable(),
		States:       states,
		Init:         d.Init,
		InitPost:     logic.TreeOf(post),
		Edges:        edges,
		Axioms:       logic.Trees(obligation.Axioms(clauses...)),
		Assumptions:  assumptions,
	}, nil
}

// initPost is the post of the start edge, which reads only the values after
// it of the variables the start state holds.
func initPost(d *core.Diagram) (logic.Formula, error) {
	for _, v := range clause.FreeVars(d.InitPost) {
		if !v.Primed || !slices.Contains(d.Vars[d.Init], v.Name) {
			return nil, fmt.Errorf("the start post reads %s, which is no value after the start that %s holds", obligation.Var(v), d.Init)
		}
	}
	f, err := obligation.Formula(d.InitPost)
	if err != nil {
		return nil, fmt.Errorf("the start post: %w", err)
	}
	return logic.Simplify(f), nil
}
