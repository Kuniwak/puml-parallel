package logic

import (
	"slices"
	"strings"
)

// Term is what a predicate is applied to: a variable, an entry of a map, a
// field of a record, or a function applied to terms. It is built only by
// the functions of this package and Var.
type Term interface {
	// vars calls add for every variable the term reads, in order.
	vars(add func(Var))
	// spellTerm writes the term.
	spellTerm(sb *strings.Builder)
	// equalTerm reports whether u is the same term, structurally.
	equalTerm(u Term) bool
	// termTree returns the syntax tree of the term.
	termTree() TermTree
}

type index struct{ m, key Term }

type field struct {
	of   Term
	name string
}

type apply struct {
	fn   string
	args []Term
}

// Index is the entry of the map m under key.
func Index(m, key Term) Term {
	mustBeTerm("logic.Index", m)
	mustBeTerm("logic.Index", key)
	return index{m: m, key: key}
}

// Field is the field name of the record of.
func Field(of Term, name string) Term {
	mustBeTerm("logic.Field", of)
	if name == "" {
		panic("logic.Field: a field needs a name")
	}
	return field{of: of, name: name}
}

// Apply applies the function symbol fn to args; applied to nothing, it is a
// constant.
func Apply(fn string, args ...Term) Term {
	if fn == "" {
		panic("logic.Apply: a function needs a name")
	}
	for _, a := range args {
		mustBeTerm("logic.Apply", a)
	}
	return apply{fn: fn, args: slices.Clone(args)}
}

// Pred applies the predicate symbol name to terms; quoted says name is text
// that is spelled as a JSON string, as Quoted does.
func Pred(name string, quoted bool, args ...Term) Formula {
	if name == "" && !quoted {
		panic("logic.Pred: a predicate needs a name")
	}
	for _, a := range args {
		mustBeTerm("logic.Pred", a)
	}
	return atom{name: name, quoted: quoted, args: slices.Clone(args)}
}

func mustBeTerm(caller string, t Term) {
	if t == nil {
		panic(caller + ": a term cannot be nil")
	}
}

func varTerms(vs []Var) []Term {
	ts := make([]Term, len(vs))
	for i, v := range vs {
		ts[i] = v
	}
	return ts
}

func (v Var) vars(add func(Var))            { add(v) }
func (v Var) spellTerm(sb *strings.Builder) { sb.WriteString(string(v)) }
func (v Var) equalTerm(u Term) bool         { w, ok := u.(Var); return ok && v == w }
func (v Var) termTree() TermTree            { return TermTree{Op: TermVar, Name: string(v)} }

func (i index) vars(add func(Var)) { i.m.vars(add); i.key.vars(add) }
func (i index) spellTerm(sb *strings.Builder) {
	i.m.spellTerm(sb)
	sb.WriteString("[")
	i.key.spellTerm(sb)
	sb.WriteString("]")
}
func (i index) equalTerm(u Term) bool {
	j, ok := u.(index)
	return ok && i.m.equalTerm(j.m) && i.key.equalTerm(j.key)
}
func (i index) termTree() TermTree {
	return TermTree{Op: TermIndex, Args: []TermTree{i.m.termTree(), i.key.termTree()}}
}

func (f field) vars(add func(Var)) { f.of.vars(add) }
func (f field) spellTerm(sb *strings.Builder) {
	f.of.spellTerm(sb)
	sb.WriteString("." + f.name)
}
func (f field) equalTerm(u Term) bool {
	g, ok := u.(field)
	return ok && f.name == g.name && f.of.equalTerm(g.of)
}
func (f field) termTree() TermTree {
	return TermTree{Op: TermField, Name: f.name, Args: []TermTree{f.of.termTree()}}
}

func (a apply) vars(add func(Var)) {
	for _, t := range a.args {
		t.vars(add)
	}
}
func (a apply) spellTerm(sb *strings.Builder) {
	sb.WriteString(a.fn)
	spellArgs(sb, a.args)
}
func (a apply) equalTerm(u Term) bool {
	b, ok := u.(apply)
	return ok && a.fn == b.fn && slices.EqualFunc(a.args, b.args, Term.equalTerm)
}
func (a apply) termTree() TermTree {
	return TermTree{Op: TermApply, Name: a.fn, Args: termTrees(a.args)}
}

// spellArgs writes args in parentheses, or nothing when there are none.
func spellArgs(sb *strings.Builder, args []Term) {
	if len(args) == 0 {
		return
	}
	sb.WriteString("(")
	for i, t := range args {
		if i > 0 {
			sb.WriteString(", ")
		}
		t.spellTerm(sb)
	}
	sb.WriteString(")")
}

// TermTree is the syntax tree of a term. Op is one of
//
//	var     the variable Name
//	index   the entry of the map Args[0] under the key Args[1]
//	field   the field Name of the record Args[0]
//	apply   the function Name applied to Args
type TermTree struct {
	Op   TermOp     `json:"op"`
	Name string     `json:"name,omitempty"`
	Args []TermTree `json:"args,omitempty"`
}

// TermOp is what a TermTree is.
type TermOp string

// The TermOps of a TermTree.
const (
	TermVar   TermOp = "var"
	TermIndex TermOp = "index"
	TermField TermOp = "field"
	TermApply TermOp = "apply"
)

func termTrees(ts []Term) []TermTree {
	out := make([]TermTree, len(ts))
	for i, t := range ts {
		out[i] = t.termTree()
	}
	return out
}

// equality says that two terms are the same value.
type equality struct{ left, right Term }

// Eq is the formula that left and right are the same value.
func Eq(left, right Term) Formula {
	mustBeTerm("logic.Eq", left)
	mustBeTerm("logic.Eq", right)
	return equality{left: left, right: right}
}

func (e equality) freeVars(bound map[Var]bool, add func(Var)) {
	read := func(v Var) {
		if !bound[v] {
			add(v)
		}
	}
	e.left.vars(read)
	e.right.vars(read)
}
func (e equality) simplify() Formula { return e }
func (e equality) equal(g Formula) bool {
	f, ok := g.(equality)
	return ok && e.left.equalTerm(f.left) && e.right.equalTerm(f.right)
}
func (e equality) spell(_ Notation, sb *strings.Builder, _ bool) {
	e.left.spellTerm(sb)
	sb.WriteString(" = ")
	e.right.spellTerm(sb)
}
func (e equality) tree() Tree {
	return Tree{Op: OpEq, Terms: []TermTree{e.left.termTree(), e.right.termTree()}}
}
