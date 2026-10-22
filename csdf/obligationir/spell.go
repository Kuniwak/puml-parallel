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
// prefix of its own, so no two categories meet and none meets the vocabulary
// a back end declares itself (step, init, reachable); the name after it is
// Mangled, which is injective.
//
//	atom_<text>   the predicate an atom is, named by its text with $_i marks
//	fn_<name>     a function of the environment, or a state constant of csdf_at
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
	FieldPrefix = "field_"
	TypePrefix  = "type_"
	IndexSymbol = "csdf_index"
	AxiomPrefix = "csdf_axiom_"
)

// AtomName is the predicate the atom of text is.
func AtomName(text string) string { return AtomPrefix + Mangle(text) }

// TypeName is the predicate that a value is of the type typ.
func TypeName(typ string) string { return TypePrefix + Mangle(typ) }

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
// types range over, sorted by kind and then by name.
func Symbols(types []string, fs ...logic.Tree) []Symbol {
	found := map[string]Symbol{}
	add := func(s Symbol) { found[s.Name] = s }
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
			add(Symbol{Name: FieldPrefix + Mangle(t.Name), Text: t.Name, Arity: 1, Kind: KindField})
		case logic.TermApply:
			kind := KindFunction
			if strings.HasPrefix(t.Name, obligation.StatePrefix) {
				kind = KindState
			}
			add(Symbol{Name: FnPrefix + Mangle(t.Name), Text: t.Name, Arity: len(t.Args), Kind: kind})
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
	out := slices.Collect(maps.Values(found))
	slices.SortFunc(out, func(a, b Symbol) int {
		if a.Kind != b.Kind {
			return int(a.Kind) - int(b.Kind)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// relativized reports whether a quantifier over typ is restricted to the
// values of typ. A type variable of an axiom ('map, 'key, 'value) is every
// value, and so restricts nothing.
func relativized(typ string) bool { return typ != "" && !strings.HasPrefix(typ, "'") }

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
// on the precedences of a prover.
func (n Notation) SpellFormula(f logic.Tree) string { return n.formula(f) }

// SpellVar is the identifier a variable of the logic is.
func SpellVar(v logic.Var) string { return VarName(string(v)) }

// Relativize is that each of vars is of the type at its position, conjoined
// with body for ∃ and assumed of it for ∀: the formula a quantifier over
// typed variables reads as when every value is of one type.
func (n Notation) Relativize(forall bool, vars []string, types []string, body string) string {
	var guards []string
	for i, v := range vars {
		if i < len(types) && relativized(types[i]) {
			guards = append(guards, TypeName(types[i])+" "+v)
		}
	}
	if len(guards) == 0 {
		return body
	}
	if forall {
		return "(" + strings.Join(guards, n.And) + ")" + n.Implies + "(" + body + ")"
	}
	return strings.Join(guards, n.And) + n.And + "(" + body + ")"
}

func (n Notation) formula(t logic.Tree) string {
	switch t.Op {
	case logic.OpTrue:
		return n.True
	case logic.OpFalse:
		return n.False
	case logic.OpAtom:
		name := t.Name
		if t.Quoted {
			name = AtomName(t.Name)
		} else {
			name = PredName(t.Name, len(t.Terms))
		}
		return applied(name, t.Terms)
	case logic.OpEq:
		return spellTerm(t.Terms[0]) + " = " + spellTerm(t.Terms[1])
	case logic.OpNot:
		return n.Not + n.operand(t.Operands[0], true)
	case logic.OpAnd, logic.OpOr, logic.OpImplies:
		sep := map[logic.Op]string{logic.OpAnd: n.And, logic.OpOr: n.Or, logic.OpImplies: n.Implies}[t.Op]
		parts := make([]string, len(t.Operands))
		for i, o := range t.Operands {
			parts[i] = n.operand(o, false)
		}
		return strings.Join(parts, sep)
	case logic.OpExists, logic.OpForall:
		vars := make([]string, len(t.Vars))
		for i, v := range t.Vars {
			vars[i] = SpellVar(v)
		}
		forall := t.Op == logic.OpForall
		body := n.Relativize(forall, vars, t.Types, n.formula(t.Operands[0]))
		if forall {
			return n.Forall(vars, body)
		}
		return n.Exists(vars, body)
	}
	panic(fmt.Sprintf("obligationir.Notation: no formula %q", t.Op))
}

// operand is t as an operand: parenthesised unless it is atomic. Under a
// negation an equality is parenthesised too.
func (n Notation) operand(t logic.Tree, negated bool) string {
	switch t.Op {
	case logic.OpTrue, logic.OpFalse, logic.OpAtom:
		return n.formula(t)
	case logic.OpEq:
		if !negated {
			return n.formula(t)
		}
	}
	return "(" + n.formula(t) + ")"
}

func spellTerm(t logic.TermTree) string {
	switch t.Op {
	case logic.TermVar:
		return SpellVar(logic.Var(t.Name))
	case logic.TermIndex:
		return applied(IndexSymbol, t.Args)
	case logic.TermField:
		return applied(FieldPrefix+Mangle(t.Name), t.Args)
	case logic.TermApply:
		return applied(FnPrefix+Mangle(t.Name), t.Args)
	}
	panic(fmt.Sprintf("obligationir.spellTerm: no term %q", t.Op))
}

// applied is name applied to args, curried as both provers apply: an
// argument that is itself applied to something is parenthesised.
func applied(name string, args []logic.TermTree) string {
	var sb strings.Builder
	sb.WriteString(name)
	for _, a := range args {
		sb.WriteString(" ")
		s := spellTerm(a)
		if a.Op != logic.TermVar && len(a.Args) > 0 {
			s = "(" + s + ")"
		}
		sb.WriteString(s)
	}
	return sb.String()
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
func (ir LivelockFree) Symbols() []Symbol {
	var types []string
	for _, s := range ir.States {
		types = append(types, s.Vars...)
	}
	for _, e := range ir.Edges {
		types = append(types, e.Received...)
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
func InitDisjunct(n Notation, side Side, s State, post logic.Tree, ref func(string) string) string {
	term, vars := stateTerm(side, s, true, ref)
	body := "s' = " + term + n.And + n.Relativize(false, vars, s.Vars, n.SpellFormula(post))
	if len(vars) == 0 {
		return body
	}
	return n.Exists(vars, body)
}

// StepDisjuncts is that some edge of edges steps from the state s to s': the
// disjunction of each edge's, or false when there are none. An edge's is
// that s is its source and s' its destination, holding values of their
// types, and that some values of the fields its event receives, of their
// types, pass its guard and post.
func StepDisjuncts(n Notation, side Side, states map[string]State, edges []Edge, ref func(string) string) string {
	if len(edges) == 0 {
		return n.False
	}
	ds := make([]string, len(edges))
	for i, e := range edges {
		src, before := stateTerm(side, states[e.Src], false, ref)
		dst, after := stateTerm(side, states[e.Dst], true, ref)
		step := n.SpellFormula(e.Step)
		if len(e.Received) > 0 {
			received := make([]string, len(e.Received))
			for j, f := range e.Received {
				received[j] = SpellVar(logic.Var(f))
			}
			step = n.Exists(received, n.Relativize(false, received, e.Received, step))
		}
		body := "s = " + src + n.And + "s' = " + dst + n.And + n.Relativize(false, after, states[e.Dst].Vars, step)
		if vars := append(before, after...); len(vars) > 0 {
			body = n.Exists(vars, body)
		}
		ds[i] = "(" + body + ")"
	}
	return strings.Join(ds, n.Or)
}
