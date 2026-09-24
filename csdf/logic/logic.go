// Package logic is first-order logic over opaque predicates: the guards and
// postconditions of a diagram are natural language, so the tools state what
// has to hold of them as formulas whose atoms are never evaluated. It builds
// such formulas, simplifies them without changing what they mean, and spells
// them in a notation of the caller's choosing.
//
// Every domain a variable ranges over is taken to be inhabited - a state
// variable holds some value, an event has some parameters - which is what
// makes binding a variable the body does not read an equivalence.
package logic

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Var is a variable of a formula.
type Var string

// Formula is a formula of first-order logic. It is built only by the
// functions of this package, so every formula is well formed.
type Formula interface {
	// freeVars calls add for every variable occurring free, in order.
	freeVars(bound map[Var]bool, add func(Var))
	// spell writes the formula; rightmost says that nothing follows it up
	// to the end of the enclosing parentheses or of the whole formula.
	spell(n Notation, sb *strings.Builder, rightmost bool)
	// simplify returns an equivalent formula without constants inside it.
	simplify() Formula
	// equal reports whether g is the same formula, structurally.
	equal(g Formula) bool
}

type constant struct{ value bool }

type atom struct {
	name   string
	quoted bool
	args   []Var
}

type not struct{ operand Formula }

// junction is a conjunction or a disjunction of at least two operands.
type junction struct {
	and      bool
	operands []Formula
}

type implies struct{ antecedent, consequent Formula }

// quantifier binds at least one variable.
type quantifier struct {
	forall bool
	vars   []Var
	body   Formula
}

var (
	// True is the formula that always holds.
	True Formula = constant{value: true}
	// False is the formula that never holds.
	False Formula = constant{value: false}
)

// Atom applies the predicate symbol name to args. The name is spelled as it
// is, so it has to be one the reader cannot take for a connective.
func Atom(name string, args ...Var) Formula {
	if name == "" {
		panic("logic.Atom: a predicate needs a name")
	}
	return atom{name: name, args: slices.Clone(args)}
}

// Quoted applies a predicate written in natural language to args. The text
// is spelled as a JSON string, so that nothing it holds reads as a connective.
func Quoted(text string, args ...Var) Formula {
	return atom{name: text, quoted: true, args: slices.Clone(args)}
}

// Not negates f.
func Not(f Formula) Formula {
	mustBeFormula("logic.Not", f)
	return not{operand: f}
}

// And conjoins fs. The empty conjunction is True, and that of one formula is
// the formula itself.
func And(fs ...Formula) Formula { return newJunction("logic.And", true, fs) }

// Or disjoins fs. The empty disjunction is False, and that of one formula is
// the formula itself.
func Or(fs ...Formula) Formula { return newJunction("logic.Or", false, fs) }

func newJunction(caller string, and bool, fs []Formula) Formula {
	for _, f := range fs {
		mustBeFormula(caller, f)
	}
	switch len(fs) {
	case 0:
		return constant{value: and}
	case 1:
		return fs[0]
	}
	return junction{and: and, operands: slices.Clone(fs)}
}

// Implies is the implication from antecedent to consequent.
func Implies(antecedent, consequent Formula) Formula {
	mustBeFormula("logic.Implies", antecedent)
	mustBeFormula("logic.Implies", consequent)
	return implies{antecedent: antecedent, consequent: consequent}
}

// Exists binds vars existentially in body; binding none is body itself.
func Exists(vars []Var, body Formula) Formula {
	return newQuantifier("logic.Exists", false, vars, body)
}

// Forall binds vars universally in body; binding none is body itself.
func Forall(vars []Var, body Formula) Formula { return newQuantifier("logic.Forall", true, vars, body) }

func newQuantifier(caller string, forall bool, vars []Var, body Formula) Formula {
	mustBeFormula(caller, body)
	if len(vars) == 0 {
		return body
	}
	return quantifier{forall: forall, vars: slices.Clone(vars), body: body}
}

func mustBeFormula(caller string, f Formula) {
	if f == nil {
		panic(caller + ": a formula cannot be nil")
	}
}

// IsTrue reports whether f is True itself. It does not decide validity.
func IsTrue(f Formula) bool {
	c, ok := f.(constant)
	return ok && c.value
}

// IsFalse reports whether f is False itself. It does not decide
// unsatisfiability.
func IsFalse(f Formula) bool {
	c, ok := f.(constant)
	return ok && !c.value
}

// Equal reports whether f and g are the same formula, structurally: the same
// connectives over the same atoms in the same order.
func Equal(f, g Formula) bool { return f.equal(g) }

// FreeVars returns the variables occurring free in f, in the order they first
// occur.
func FreeVars(f Formula) []Var {
	var vars []Var
	seen := make(map[Var]bool)
	f.freeVars(nil, func(v Var) {
		if !seen[v] {
			seen[v] = true
			vars = append(vars, v)
		}
	})
	return vars
}

// Simplify returns a formula equivalent to f in which no constant occurs but,
// possibly, the whole formula. It removes True from conjunctions and False
// from disjunctions, lets False absorb a conjunction and True a disjunction,
// flattens nested conjunctions and disjunctions, cancels double negations,
// reduces implications with a constant side, and unbinds every variable a body
// does not read.
func Simplify(f Formula) Formula { return f.simplify() }

func (c constant) freeVars(map[Var]bool, func(Var)) {}
func (c constant) simplify() Formula                { return c }
func (c constant) equal(g Formula) bool {
	d, ok := g.(constant)
	return ok && c == d
}

func (a atom) freeVars(bound map[Var]bool, add func(Var)) {
	for _, v := range a.args {
		if !bound[v] {
			add(v)
		}
	}
}
func (a atom) simplify() Formula { return a }
func (a atom) equal(g Formula) bool {
	b, ok := g.(atom)
	return ok && a.name == b.name && a.quoted == b.quoted && slices.Equal(a.args, b.args)
}

func (n not) freeVars(bound map[Var]bool, add func(Var)) { n.operand.freeVars(bound, add) }
func (n not) simplify() Formula {
	switch o := n.operand.simplify().(type) {
	case constant:
		return constant{value: !o.value}
	case not:
		return o.operand
	default:
		return not{operand: o}
	}
}
func (n not) equal(g Formula) bool {
	m, ok := g.(not)
	return ok && n.operand.equal(m.operand)
}

func (j junction) freeVars(bound map[Var]bool, add func(Var)) {
	for _, o := range j.operands {
		o.freeVars(bound, add)
	}
}
func (j junction) simplify() Formula {
	// The unit of a conjunction is true and its zero false; a disjunction the
	// other way round.
	var kept []Formula
	for _, o := range j.operands {
		switch s := o.simplify().(type) {
		case constant:
			if s.value != j.and {
				return s
			}
		case junction:
			if s.and == j.and {
				kept = append(kept, s.operands...)
			} else {
				kept = append(kept, s)
			}
		default:
			kept = append(kept, s)
		}
	}
	return newJunction("logic.Simplify", j.and, kept)
}
func (j junction) equal(g Formula) bool {
	k, ok := g.(junction)
	return ok && j.and == k.and && slices.EqualFunc(j.operands, k.operands, Formula.equal)
}

func (i implies) freeVars(bound map[Var]bool, add func(Var)) {
	i.antecedent.freeVars(bound, add)
	i.consequent.freeVars(bound, add)
}
func (i implies) simplify() Formula {
	a, c := i.antecedent.simplify(), i.consequent.simplify()
	switch {
	case IsTrue(a):
		return c
	case IsFalse(a), IsTrue(c):
		return True
	case IsFalse(c):
		return not{operand: a}.simplify()
	}
	return implies{antecedent: a, consequent: c}
}
func (i implies) equal(g Formula) bool {
	j, ok := g.(implies)
	return ok && i.antecedent.equal(j.antecedent) && i.consequent.equal(j.consequent)
}

func (q quantifier) freeVars(bound map[Var]bool, add func(Var)) {
	inner := make(map[Var]bool, len(bound)+len(q.vars))
	for v := range bound {
		inner[v] = true
	}
	for _, v := range q.vars {
		inner[v] = true
	}
	q.body.freeVars(inner, add)
}
func (q quantifier) simplify() Formula {
	body := q.body.simplify()
	if _, ok := body.(constant); ok {
		return body
	}
	read := make(map[Var]bool)
	for _, v := range FreeVars(body) {
		read[v] = true
	}
	var vars []Var
	for _, v := range q.vars {
		if read[v] {
			vars = append(vars, v)
		}
	}
	return newQuantifier("logic.Simplify", q.forall, vars, body)
}
func (q quantifier) equal(g Formula) bool {
	r, ok := g.(quantifier)
	return ok && q.forall == r.forall && slices.Equal(q.vars, r.vars) && q.body.equal(r.body)
}

// Connectives are the spellings of a notation, spaces included.
type Connectives struct {
	And, Or, Not, Implies, Exists, Forall, True, False string
}

// Notation spells formulas. It is made only by NewNotation, which refuses a
// connective spelled as nothing; the zero Notation spells nothing and panics.
type Notation struct {
	c     Connectives
	valid bool
}

// NewNotation makes the notation spelling the connectives as c says. A
// connective spelled as nothing is refused: a negated formula would then read
// as the formula itself, and two conjuncts as one.
func NewNotation(c Connectives) (Notation, error) {
	for name, s := range map[string]string{
		"and": c.And, "or": c.Or, "not": c.Not, "implies": c.Implies,
		"exists": c.Exists, "forall": c.Forall, "true": c.True, "false": c.False,
	} {
		if s == "" {
			return Notation{}, fmt.Errorf("logic.NewNotation: %s is spelled as nothing", name)
		}
	}
	return Notation{c: c, valid: true}, nil
}

// Valid reports whether n was made by NewNotation; the zero Notation is not.
func (n Notation) Valid() bool { return n.valid }

// Spell writes f. Negation binds tightest; a conjunction and a disjunction
// have no precedence over each other, so one inside the other is
// parenthesised; an implication binds loosest, and one inside another is
// parenthesised; a quantifier reaches to the end of what it is in, so it is
// parenthesised exactly when something follows it.
func (n Notation) Spell(f Formula) string {
	if !n.valid {
		panic("logic.Notation.Spell: the zero Notation spells nothing; make one with NewNotation")
	}
	var sb strings.Builder
	f.spell(n, &sb, true)
	return sb.String()
}

func (c constant) spell(n Notation, sb *strings.Builder, _ bool) {
	if c.value {
		sb.WriteString(n.c.True)
	} else {
		sb.WriteString(n.c.False)
	}
}

func (a atom) spell(_ Notation, sb *strings.Builder, _ bool) {
	if a.quoted {
		sb.WriteString(quote(a.name))
	} else {
		sb.WriteString(a.name)
	}
	if len(a.args) == 0 {
		return
	}
	args := make([]string, len(a.args))
	for i, v := range a.args {
		args[i] = string(v)
	}
	sb.WriteString("(" + strings.Join(args, ", ") + ")")
}

func (x not) spell(n Notation, sb *strings.Builder, rightmost bool) {
	sb.WriteString(n.c.Not)
	switch x.operand.(type) {
	case junction, implies:
		parenthesised(n, sb, x.operand)
	default:
		operand(n, sb, x.operand, rightmost)
	}
}

func (j junction) spell(n Notation, sb *strings.Builder, rightmost bool) {
	sep := n.c.Or
	if j.and {
		sep = n.c.And
	}
	for i, o := range j.operands {
		if i > 0 {
			sb.WriteString(sep)
		}
		last := i == len(j.operands)-1
		switch o.(type) {
		case junction, implies:
			// A conjunction inside a conjunction only comes of building
			// one so; it is parenthesised too, to spell what was built.
			parenthesised(n, sb, o)
		default:
			operand(n, sb, o, last && rightmost)
		}
	}
}

func (i implies) spell(n Notation, sb *strings.Builder, rightmost bool) {
	if _, ok := i.antecedent.(implies); ok {
		parenthesised(n, sb, i.antecedent)
	} else {
		operand(n, sb, i.antecedent, false)
	}
	sb.WriteString(n.c.Implies)
	if _, ok := i.consequent.(implies); ok {
		parenthesised(n, sb, i.consequent)
	} else {
		operand(n, sb, i.consequent, rightmost)
	}
}

func (q quantifier) spell(n Notation, sb *strings.Builder, _ bool) {
	if q.forall {
		sb.WriteString(n.c.Forall)
	} else {
		sb.WriteString(n.c.Exists)
	}
	vars := make([]string, len(q.vars))
	for i, v := range q.vars {
		vars[i] = string(v)
	}
	sb.WriteString(strings.Join(vars, " ") + ". ")
	q.body.spell(n, sb, true)
}

// operand writes an operand that needs no parentheses for precedence. A
// quantifier among them still needs them unless nothing follows it.
func operand(n Notation, sb *strings.Builder, f Formula, rightmost bool) {
	if _, ok := f.(quantifier); ok && !rightmost {
		parenthesised(n, sb, f)
		return
	}
	f.spell(n, sb, rightmost)
}

func parenthesised(n Notation, sb *strings.Builder, f Formula) {
	sb.WriteString("(")
	f.spell(n, sb, true)
	sb.WriteString(")")
}

// quote writes text as a JSON string. HTML characters are left as they are:
// a formula is no web page.
func quote(text string) string {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(text); err != nil {
		panic(fmt.Sprintf("logic.quote: a string always encodes: %v", err))
	}
	return strings.TrimSuffix(sb.String(), "\n")
}
