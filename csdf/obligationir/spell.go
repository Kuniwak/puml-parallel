package obligationir

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
)

// The identifiers a formula's symbols become in a prover. Each category has a
// prefix of its own, no prefix begins another, and none meets the vocabulary
// a back end declares itself (step, init, reachable, St_, assume_); the name
// after it is Mangled, which is injective, so no two symbols meet.
//
//	atom_<text>   the predicate an atom is, named by its text with $_i marks
//	fn_<name>     a function of the environment
//	state_<name>  a state of a local diagram, as csdf_at names it
//	field_<name>  the field name of a record
//	type_<name>   that a value is of the type name
//	v_<name>      a variable, primed or not (Mangle spells the prime)
//
// IndexSymbol is the entry of a map under a key. The csdf_* predicates keep
// their names, but csdf_frame takes the number of its keys after it, since
// a prover's constant has one arity.
const (
	AtomPrefix  = "atom_"
	FnPrefix    = "fn_"
	StatePrefix = "state_"
	FieldPrefix = "field_"
	TypePrefix  = "type_"
	IndexSymbol = "csdf_index"
	AxiomPrefix = "csdf_axiom_"
)

// AtomName is the predicate the atom of text is.
func AtomName(text string) string { return AtomPrefix + Mangle(text) }

// TypeName is the predicate that a value is of the type typ.
func TypeName(typ string) string { return TypePrefix + Mangle(typ) }

// FieldName is the function the field name of a record is.
func FieldName(name string) string { return FieldPrefix + Mangle(name) }

// FunctionName is the constant a function of the logic is: a state of a
// local diagram when csdf/obligation named it so (isState), and otherwise a
// function of the environment.
func FunctionName(fn string) string {
	if isState(fn) {
		return StatePrefix + Mangle(strings.TrimPrefix(fn, obligation.StatePrefix))
	}
	return FnPrefix + Mangle(fn)
}

// isState reports whether fn is a constant obligation.StateConstant names: it
// begins with obligation.StatePrefix and holds a dot, which no function of
// the environment does, since its name is an identifier.
func isState(fn string) bool {
	return strings.HasPrefix(fn, obligation.StatePrefix) && strings.Contains(fn, ".")
}

// AxiomName is the proposition the i-th axiom of an obligation is, from 1.
func AxiomName(i int) string { return fmt.Sprintf("%s%d", AxiomPrefix, i) }

// PredName is the predicate a csdf_* predicate of n arguments is.
func PredName(name string, n int) string {
	if name == obligation.PredFrame {
		return fmt.Sprintf("%s%d", name, n-2)
	}
	return name
}

// Symbol is a constant a formula reads that a back end declares: Name is its
// identifier, Arity the number of its arguments, Text what it was named in
// the clauses, and Pred whether it is a predicate rather than a function.
type Symbol struct {
	Name  string
	Text  string
	Arity int
	Pred  bool
	// Kind is what the symbol is: an atom, a function, a field, the index of
	// a map, a type or a csdf_* predicate, as the Kind* constants say.
	Kind SymbolKind
}

// SymbolKind is what a Symbol is.
type SymbolKind int

// The kinds of a Symbol. Atoms, functions and types are the environment,
// which no obligation interprets; a state is a constant csdf_at names a
// state of a local diagram by (obligation.StateConstant); fields and the
// index are what clauses read of values; the csdf_* predicates have the
// meaning of the axioms.
const (
	KindAtom SymbolKind = iota
	KindFunction
	KindType
	KindState
	KindField
	KindIndex
	KindBuiltin
)

// Symbols are the constants fs read, and the types their quantifiers and
// types range over, sorted by kind and then by name. One name read as two
// constants, of two arities or kinds, is refused: a prover's constant has
// one.
func Symbols(types []string, fs ...logic.Tree) ([]Symbol, error) {
	found := map[string]Symbol{}
	var conflict error
	add := func(s Symbol) {
		if t, ok := found[s.Name]; ok && (t.Arity != s.Arity || t.Kind != s.Kind || t.Pred != s.Pred) && conflict == nil {
			conflict = fmt.Errorf("obligationir.Symbols: %s is read with %d and with %d arguments", s.Name, t.Arity, s.Arity)
		}
		found[s.Name] = s
	}
	for _, typ := range types {
		if relativized(typ) {
			add(Symbol{Name: TypeName(typ), Text: typ, Arity: 1, Pred: true, Kind: KindType})
		}
	}
	var term func(t logic.TermTree)
	term = func(t logic.TermTree) {
		switch t.Op {
		case logic.TermIndex:
			add(Symbol{Name: IndexSymbol, Arity: 2, Kind: KindIndex})
		case logic.TermField:
			add(Symbol{Name: FieldName(t.Name), Text: t.Name, Arity: 1, Kind: KindField})
		case logic.TermApply:
			kind := KindFunction
			if isState(t.Name) {
				kind = KindState
			}
			add(Symbol{Name: FunctionName(t.Name), Text: t.Name, Arity: len(t.Args), Kind: kind})
		}
		for _, a := range t.Args {
			term(a)
		}
	}
	var formula func(t logic.Tree)
	formula = func(t logic.Tree) {
		switch t.Op {
		case logic.OpAtom:
			if t.Quoted {
				add(Symbol{Name: AtomName(t.Name), Text: t.Name, Arity: len(t.Terms), Pred: true, Kind: KindAtom})
			} else {
				add(Symbol{Name: PredName(t.Name, len(t.Terms)), Arity: len(t.Terms), Pred: true, Kind: KindBuiltin})
			}
		case logic.OpExists, logic.OpForall:
			for _, typ := range t.Types {
				if relativized(typ) {
					add(Symbol{Name: TypeName(typ), Text: typ, Arity: 1, Pred: true, Kind: KindType})
				}
			}
		}
		for _, a := range t.Terms {
			term(a)
		}
		for _, o := range t.Operands {
			formula(o)
		}
	}
	for _, f := range fs {
		formula(f)
	}
	if conflict != nil {
		return nil, conflict
	}
	out := slices.Collect(maps.Values(found))
	slices.SortFunc(out, func(a, b Symbol) int {
		if a.Kind != b.Kind {
			return int(a.Kind) - int(b.Kind)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

// relativized reports whether a quantifier over typ is restricted to the
// values of typ. A type variable of an axiom ('map, 'key, 'value) is every
// value, and so restricts nothing.
func relativized(typ string) bool { return !strings.HasPrefix(typ, "'") }

// Notation is how a prover spells the connectives and the binders. Every
// value is of one type, ValType; a quantifier over a type is relativized to
// the values of that type (TypeName), so that the obligation reads as the
// clauses do whatever a prover makes of types.
type Notation struct {
	And, Or, Not, Implies, True, False string
	// Exists and Forall spell a quantifier binding vars, each of the type
	// the prover gives every value, over body.
	Exists, Forall func(vars []string, body string) string
}

// SpellFormula writes f in the notation n. Every operand that is no atom, no
// equality and no constant is parenthesised, so the spelling does not rest
// on the precedences of a prover. A tree that is no formula is refused.
func (n Notation) SpellFormula(f logic.Tree) (string, error) { return n.formula(f) }

// SpellVar is the identifier a variable of the logic is.
func SpellVar(v logic.Var) string { return VarName(string(v)) }

// Relativize is that each of vars is of the type at its position, conjoined
// with body for ∃ and assumed of it for ∀: the formula a quantifier over
// typed variables reads as when every value is of one type. Each variable
// needs a type: a quantifier left unrestricted would range over values the
// clauses never take, and so would state another obligation.
func (n Notation) Relativize(forall bool, vars []string, types []string, body string) (string, error) {
	if len(types) != len(vars) {
		return "", fmt.Errorf("obligationir.Relativize: %d variables with %d types", len(vars), len(types))
	}
	var guards []string
	for i, v := range vars {
		if types[i] == "" {
			return "", fmt.Errorf("obligationir.Relativize: %s has no type", v)
		}
		if relativized(types[i]) {
			guards = append(guards, TypeName(types[i])+" "+v)
		}
	}
	if len(guards) == 0 {
		return body, nil
	}
	if forall {
		return "(" + strings.Join(guards, n.And) + ")" + n.Implies + "(" + body + ")", nil
	}
	return strings.Join(guards, n.And) + n.And + "(" + body + ")", nil
}

func (n Notation) formula(t logic.Tree) (string, error) {
	operands := func(want int) error {
		if want >= 0 && len(t.Operands) != want || want < 0 && len(t.Operands) < 2 {
			return fmt.Errorf("obligationir.Notation: %s with %d operands", t.Op, len(t.Operands))
		}
		return nil
	}
	switch t.Op {
	case logic.OpTrue:
		return n.True, nil
	case logic.OpFalse:
		return n.False, nil
	case logic.OpAtom:
		if t.Name == "" {
			return "", fmt.Errorf("obligationir.Notation: an atom with no name")
		}
		name := AtomName(t.Name)
		if !t.Quoted {
			name = PredName(t.Name, len(t.Terms))
		}
		return applied(name, t.Terms)
	case logic.OpEq:
		if len(t.Terms) != 2 {
			return "", fmt.Errorf("obligationir.Notation: an equality of %d terms", len(t.Terms))
		}
		l, err := spellTerm(t.Terms[0])
		if err != nil {
			return "", err
		}
		r, err := spellTerm(t.Terms[1])
		if err != nil {
			return "", err
		}
		return l + " = " + r, nil
	case logic.OpNot:
		if err := operands(1); err != nil {
			return "", err
		}
		o, err := n.operand(t.Operands[0], true)
		return n.Not + o, err
	case logic.OpAnd, logic.OpOr, logic.OpImplies:
		want := -1
		if t.Op == logic.OpImplies {
			want = 2
		}
		if err := operands(want); err != nil {
			return "", err
		}
		sep := map[logic.Op]string{logic.OpAnd: n.And, logic.OpOr: n.Or, logic.OpImplies: n.Implies}[t.Op]
		parts := make([]string, len(t.Operands))
		for i, o := range t.Operands {
			p, err := n.operand(o, false)
			if err != nil {
				return "", err
			}
			parts[i] = p
		}
		return strings.Join(parts, sep), nil
	case logic.OpExists, logic.OpForall:
		if err := operands(1); err != nil {
			return "", err
		}
		if len(t.Vars) == 0 {
			return "", fmt.Errorf("obligationir.Notation: %s binding nothing", t.Op)
		}
		vars := make([]string, len(t.Vars))
		for i, v := range t.Vars {
			vars[i] = SpellVar(v)
		}
		forall := t.Op == logic.OpForall
		inner, err := n.formula(t.Operands[0])
		if err != nil {
			return "", err
		}
		body, err := n.Relativize(forall, vars, t.Types, inner)
		if err != nil {
			return "", err
		}
		if forall {
			return n.Forall(vars, body), nil
		}
		return n.Exists(vars, body), nil
	}
	return "", fmt.Errorf("obligationir.Notation: no formula %q", t.Op)
}

// operand is t as an operand: parenthesised unless it is atomic. Under a
// negation an equality is parenthesised too.
func (n Notation) operand(t logic.Tree, negated bool) (string, error) {
	s, err := n.formula(t)
	if err != nil {
		return "", err
	}
	switch t.Op {
	case logic.OpTrue, logic.OpFalse, logic.OpAtom:
		return s, nil
	case logic.OpEq:
		if !negated {
			return s, nil
		}
	}
	return "(" + s + ")", nil
}

func spellTerm(t logic.TermTree) (string, error) {
	args := func(want int) error {
		if len(t.Args) != want {
			return fmt.Errorf("obligationir.Notation: %s of %d terms", t.Op, len(t.Args))
		}
		return nil
	}
	switch t.Op {
	case logic.TermVar:
		if t.Name == "" || len(t.Args) > 0 {
			return "", fmt.Errorf("obligationir.Notation: a variable %q of %d terms", t.Name, len(t.Args))
		}
		return SpellVar(logic.Var(t.Name)), nil
	case logic.TermIndex:
		if err := args(2); err != nil {
			return "", err
		}
		return applied(IndexSymbol, t.Args)
	case logic.TermField:
		if err := args(1); err != nil {
			return "", err
		}
		return applied(FieldName(t.Name), t.Args)
	case logic.TermApply:
		if t.Name == "" {
			return "", fmt.Errorf("obligationir.Notation: a function with no name")
		}
		return applied(FunctionName(t.Name), t.Args)
	}
	return "", fmt.Errorf("obligationir.Notation: no term %q", t.Op)
}

// applied is name applied to args, curried as both provers apply: an
// argument that is itself applied to something is parenthesised.
func applied(name string, args []logic.TermTree) (string, error) {
	var sb strings.Builder
	sb.WriteString(name)
	for _, a := range args {
		sb.WriteString(" ")
		s, err := spellTerm(a)
		if err != nil {
			return "", err
		}
		if a.Op != logic.TermVar && len(a.Args) > 0 {
			s = "(" + s + ")"
		}
		sb.WriteString(s)
	}
	return sb.String(), nil
}

// Formulas are every formula ir states: the start post, the step of each
// edge and the axioms.
func (ir LivelockFree) Formulas() []logic.Tree {
	fs := []logic.Tree{ir.InitPost}
	for _, e := range ir.Edges {
		fs = append(fs, e.Step)
	}
	return append(fs, ir.Axioms...)
}

// Symbols are the constants ir reads, the types of its state variables and
// received fields among them: each is of the type named like it, the
// convention until they carry types of their own.
func (ir LivelockFree) Symbols() ([]Symbol, error) {
	var types []string
	for _, s := range ir.States {
		types = append(types, s.Vars...)
	}
	for _, e := range ir.Edges {
		types = append(types, e.Received()...)
	}
	return Symbols(types, ir.Formulas()...)
}

// Uninterpreted reports whether s is of the environment, which no obligation
// interprets and a back end marks TODO(csdf): an atom, a function or a type.
func Uninterpreted(s Symbol) bool {
	return s.Kind == KindAtom || s.Kind == KindFunction || s.Kind == KindType
}

// VarsComment is the state variables vars as a comment beside a constructor
// lists them, or nothing when there are none.
func VarsComment(vars []string) string {
	if len(vars) == 0 {
		return ""
	}
	return " (" + strings.Join(vars, ", ") + ")"
}

// stateTerm is the state s holding the variables vars, primed when after: the
// constructor of s, spelled by ref, applied to them.
func stateTerm(side Side, s State, after bool, ref func(string) string) (string, []string) {
	vars := make([]string, len(s.Vars))
	for i, v := range s.Vars {
		if after {
			v += "'"
		}
		vars[i] = SpellVar(logic.Var(v))
	}
	return strings.Join(append([]string{ref(side.Ctor(csdf.StateID(s.Name)))}, vars...), " "), vars
}

// InitDisjunct is that the state s' is the start state s, holding values
// after the start of their types that post admits.
func InitDisjunct(n Notation, side Side, s State, post logic.Tree, ref func(string) string) (string, error) {
	term, vars := stateTerm(side, s, true, ref)
	spelled, err := n.SpellFormula(post)
	if err != nil {
		return "", fmt.Errorf("obligationir.InitDisjunct: %w", err)
	}
	typed, err := n.Relativize(false, vars, s.Vars, spelled)
	if err != nil {
		return "", fmt.Errorf("obligationir.InitDisjunct: %w", err)
	}
	body := "s' = " + term + n.And + typed
	if len(vars) == 0 {
		return body, nil
	}
	return n.Exists(vars, body), nil
}

// StepDisjuncts is that some edge of edges steps from the state s to s': the
// disjunction of each edge's, or false when there are none. An edge's is
// that s is its source and s' its destination, holding values of their
// types, and that some values of the fields its event receives, of their
// types, pass its guard and post.
func StepDisjuncts(n Notation, side Side, states map[string]State, edges []Edge, ref func(string) string) (string, error) {
	if len(edges) == 0 {
		return n.False, nil
	}
	ds := make([]string, len(edges))
	for i, e := range edges {
		d, err := stepDisjunct(n, side, states, e, ref)
		if err != nil {
			return "", fmt.Errorf("obligationir.StepDisjuncts: the edge %s -> %s: %w", e.Src, e.Dst, err)
		}
		ds[i] = "(" + d + ")"
	}
	return strings.Join(ds, n.Or), nil
}

func stepDisjunct(n Notation, side Side, states map[string]State, e Edge, ref func(string) string) (string, error) {
	srcState, ok := states[e.Src]
	if !ok {
		return "", fmt.Errorf("no state %s", e.Src)
	}
	dstState, ok := states[e.Dst]
	if !ok {
		return "", fmt.Errorf("no state %s", e.Dst)
	}
	src, before := stateTerm(side, srcState, false, ref)
	dst, after := stateTerm(side, dstState, true, ref)
	step, err := n.SpellFormula(e.Step)
	if err != nil {
		return "", err
	}
	if fields := e.Received(); len(fields) > 0 {
		received := make([]string, len(fields))
		for j, f := range fields {
			received[j] = SpellVar(logic.Var(f))
		}
		typed, err := n.Relativize(false, received, fields, step)
		if err != nil {
			return "", err
		}
		step = n.Exists(received, typed)
	}
	typed, err := n.Relativize(false, after, dstState.Vars, step)
	if err != nil {
		return "", err
	}
	body := "s = " + src + n.And + "s' = " + dst + n.And + typed
	if vars := slices.Concat(before, after); len(vars) > 0 {
		body = n.Exists(vars, body)
	}
	return body, nil
}

// TauEdges are the tau edges of ir.
func (ir LivelockFree) TauEdges() []Edge {
	var taus []Edge
	for _, e := range ir.Edges {
		if e.Tau() {
			taus = append(taus, e)
		}
	}
	return taus
}

// StateMap is the states of ir by name.
func (ir LivelockFree) StateMap() map[string]State {
	m := make(map[string]State, len(ir.States))
	for _, s := range ir.States {
		m[s.Name] = s
	}
	return m
}

// AxiomNames are the propositions the axioms of ir are, in order.
func (ir LivelockFree) AxiomNames() []string {
	names := make([]string, len(ir.Axioms))
	for i := range ir.Axioms {
		names[i] = AxiomName(i + 1)
	}
	return names
}

// Declaration is a constant a back end declares: the lines of the comment
// above it, todo first when the constant is uninterpreted, then the text it
// was named by.
type Declaration struct {
	Comments []string
	Symbol
}

// Declarations are the constants ir reads, as a back end declares them, todo
// marking those of the environment.
func (ir LivelockFree) Declarations(todo string) ([]Declaration, error) {
	syms, err := ir.Symbols()
	if err != nil {
		return nil, err
	}
	ds := make([]Declaration, len(syms))
	for i, s := range syms {
		var comments []string
		if Uninterpreted(s) {
			comments = append(comments, todo)
		}
		if s.Text != "" {
			comments = append(comments, s.Text)
		}
		ds[i] = Declaration{Comments: comments, Symbol: s}
	}
	return ds, nil
}
