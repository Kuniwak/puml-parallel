package obligationir

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/logic"
	"github.com/Kuniwak/puml-parallel/v3/csdf/obligation"
)

// LivelockFree is the livelock-freedom obligation of a diagram whose clauses
// are csdf/clause formulas (docs/CLAUSE_DESIGN.md, O8): no reachable state
// admits an infinite run of tau steps. Each clause is the formula of
// csdf/logic that csdf/obligation states, so a back end spells it rather than
// leaving it opaque: an atom is an uninterpreted predicate applied to its
// holes, a builtin an equality or a csdf_* predicate whose meaning Lemmas
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
	// Ends are the end edges: the diagram may terminate from Src when Guard
	// holds of its values. Termination is no tau step, so the livelock
	// obligation reads none of them; a refinement does.
	Ends []End `json:"ends"`
	// Lemmas are what the csdf_* predicates of the clauses mean
	// (obligation.Lemmas). A back end defines the builtins and proves them,
	// so no theorem rests on them as hypotheses.
	Lemmas []logic.Tree `json:"lemmas"`
	// Assumptions are the diagram's assume notes, hypotheses of the theorem
	// like the axioms. Never null, only empty.
	Assumptions []Assumption `json:"assumptions"`
}

// IRVersion is the version of the encoding of LivelockFree. It changes
// whenever a field changes meaning, so that an IR stored by another release
// is refused rather than misread.
const IRVersion = 3

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
		for _, v := range s.Vars {
			if v.Type == "" {
				return fmt.Errorf("obligationir.Validate: the variable %s of %s has no type", v.Name, s.Name)
			}
		}
	}
	if !states[ir.Init] {
		return fmt.Errorf("obligationir.Validate: no start state %q", ir.Init)
	}
	stateMap := ir.StateMap()
	for _, e := range ir.Edges {
		if !states[e.Src] || !states[e.Dst] {
			return fmt.Errorf("obligationir.Validate: the edge %s -> %s names a state that is not there", e.Src, e.Dst)
		}
		if e.Channel == "" || e.Tau() && len(e.Fields) > 0 {
			return fmt.Errorf("obligationir.Validate: the edge %s -> %s has no channel, or is a tau with fields", e.Src, e.Dst)
		}
		if err := fieldsBound(e, stateMap); err != nil {
			return fmt.Errorf("obligationir.Validate: the edge %s -> %s: %w", e.Src, e.Dst, err)
		}
	}
	for _, e := range ir.Ends {
		if !states[e.Src] {
			return fmt.Errorf("obligationir.Validate: an end edge leaves %s, which is not there", e.Src)
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

// fieldsBound reports, as an error, a field of e that is not bound where it
// is read: a received field named twice, by nothing, or like a variable of
// the source or the destination, which it would bind over, as
// obligation.Step refuses; a sent field that is neither a
// variable of the source nor a field e receives, which would be free.
func fieldsBound(e Edge, states map[string]State) error {
	srcState, ok := states[e.Src]
	if !ok {
		return fmt.Errorf("no state %s", e.Src)
	}
	dstState, ok := states[e.Dst]
	if !ok {
		return fmt.Errorf("no state %s", e.Dst)
	}
	src := srcState.Names()
	bound := map[string]bool{}
	for _, v := range src {
		bound[v] = true
	}
	received := map[string]bool{}
	for _, f := range e.Received() {
		if f == "" || received[f] || core.ReceivesOver(f, src, dstState.Names()) {
			return fmt.Errorf("it receives %q twice, by no name or over a variable of %s or %s", f, e.Src, e.Dst)
		}
		received[f] = true
	}
	for _, f := range e.Fields {
		if f.Type == "" {
			return fmt.Errorf("its field %q has no type", f.Name)
		}
	}
	for _, f := range e.Fields {
		if f.Send && !bound[f.Name] && !received[f.Name] {
			return fmt.Errorf("it sends %q, which neither %s holds nor it receives", f.Name, e.Src)
		}
	}
	return nil
}

// checking is a notation that only checks what it spells.
var checking = Notation{
	Exists: func([]string, string) string { return "" },
	Forall: func([]string, string) string { return "" },
	Literal: LiteralSpeller{
		Int:    func(string) string { return "" },
		Float:  func(bool, uint64, int) string { return "" },
		String: func(string) string { return "" },
		Bool:   func(bool) string { return "" },
		Array:  func([]string) string { return "" },
		Object: func([]string, []string) string { return "" },
	},
}

// Assumption is one assume note of a diagram: what its runs as a whole are
// assumed to satisfy, which no clause can say. A back end declares an
// uninterpreted proposition named after its ID (Side.AssumptionName) and
// states the theorem under it.
type Assumption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// State is a state and the state variables it holds.
type State struct {
	Name string `json:"name"`
	Vars []Var  `json:"vars"`
}

// Var is a state variable and the type the diagram declares of it
// (docs/CLAUSE_DESIGN.md, Types): a quantifier over it is relativised to
// its type, to none for the type any.
type Var struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Names are the names of the variables s holds, in order.
func (s State) Names() []string {
	out := make([]string, len(s.Vars))
	for i, v := range s.Vars {
		out[i] = v.Name
	}
	return out
}

// Types are the types of the variables s holds, in order.
func (s State) Types() []string {
	out := make([]string, len(s.Vars))
	for i, v := range s.Vars {
		out[i] = v.Type
	}
	return out
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

// End is an end edge: Src --> [*] : Guard, over the values of Src.
type End struct {
	Src   string     `json:"src"`
	Guard logic.Tree `json:"guard"`
}

// Field is a field of an event, of the type Type its channel declares at
// its position: received (?Name), which the step chooses, or sent (!Name),
// a value the step has bound already.
type Field struct {
	Send bool   `json:"send"`
	Name string `json:"name"`
	Type string `json:"type"`
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

// ReceivedTypes are the types of the fields e receives, in order.
func (e Edge) ReceivedTypes() []string {
	ts := []string{}
	for _, f := range e.Fields {
		if !f.Send {
			ts = append(ts, f.Type)
		}
	}
	return ts
}

// fieldsOf are the fields of e on d, each of the type its channel
// declares at its position.
func fieldsOf(d *core.Diagram, e core.Event) ([]Field, error) {
	fs := make([]Field, len(e.Fields))
	for i, f := range e.Fields {
		t, err := d.FieldType(e, i)
		if err != nil {
			return nil, err
		}
		fs[i] = Field{Send: f.Send, Name: f.Name, Type: t.String()}
	}
	return fs, nil
}

// BuildLivelockFree states the livelock-freedom obligation of d.
func BuildLivelockFree(d *core.Diagram) (LivelockFree, error) {
	post, err := initPost(d)
	if err != nil {
		return LivelockFree{}, fmt.Errorf("obligationir.BuildLivelockFree: %w", err)
	}
	clauses := []clause.Formula{d.InitPost}
	edges := make([]Edge, 0, len(d.Edges))
	for _, e := range d.Edges {
		step, err := obligation.Step(*d, e)
		if err != nil {
			return LivelockFree{}, fmt.Errorf("obligationir.BuildLivelockFree: %w", err)
		}
		fields, err := fieldsOf(d, e.Event)
		if err != nil {
			return LivelockFree{}, fmt.Errorf("obligationir.BuildLivelockFree: %w", err)
		}
		edges = append(edges, Edge{Src: e.Src, Dst: e.Dst, Channel: e.Event.Channel, Fields: fields, Step: logic.TreeOf(step)})
		clauses = append(clauses, e.Guard, e.Post)
	}
	ends := make([]End, 0, len(d.Ends))
	for _, e := range d.Ends {
		guard, err := endGuard(d, e)
		if err != nil {
			return LivelockFree{}, fmt.Errorf("obligationir.BuildLivelockFree: %w", err)
		}
		ends = append(ends, End{Src: e.Src, Guard: logic.TreeOf(guard)})
		clauses = append(clauses, e.Guard)
	}
	states := make([]State, len(d.States))
	for i, s := range d.States {
		states[i] = State{Name: s, Vars: []Var{}}
		for _, v := range d.Vars[s] {
			t, err := d.VarType(v)
			if err != nil {
				return LivelockFree{}, fmt.Errorf("obligationir.BuildLivelockFree: %s: %w", s, err)
			}
			states[i].Vars = append(states[i].Vars, Var{Name: v, Type: t.String()})
		}
	}
	assumptions := make([]Assumption, 0, len(d.Assumes))
	for _, a := range d.Assumes {
		assumptions = append(assumptions, Assumption{ID: a.ID, Text: a.Text})
	}
	return LivelockFree{
		Version:      IRVersion,
		Structurally: !d.TauCycleReachable(),
		States:       states,
		Init:         d.Init,
		InitPost:     logic.TreeOf(post),
		Edges:        edges,
		Ends:         ends,
		Lemmas:       logic.Trees(obligation.Lemmas(clauses...)),
		Assumptions:  assumptions,
	}, nil
}

// endGuard is the guard of an end edge, which reads only the values of the
// state it leaves.
func endGuard(d *core.Diagram, e core.End) (logic.Formula, error) {
	if !slices.Contains(d.States, e.Src) {
		return nil, fmt.Errorf("no state %s, which an end edge leaves", e.Src)
	}
	for _, v := range clause.FreeVars(e.Guard) {
		if v.Primed || !slices.Contains(d.Vars[e.Src], v.Name) {
			return nil, fmt.Errorf("the end edge from %s reads %s, which %s does not hold", e.Src, obligation.Var(v), e.Src)
		}
	}
	f, err := obligation.Formula(e.Guard)
	if err != nil {
		return nil, fmt.Errorf("the end edge from %s: %w", e.Src, err)
	}
	return logic.Simplify(f), nil
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
