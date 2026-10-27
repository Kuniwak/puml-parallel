package clause

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

const (
	toolPrefix = '@'
	holeSigil  = '$'
	escape     = '\\'
	conjSep    = " ∧ "
	// escapable are the characters a backslash makes text; before anything
	// else a backslash is itself.
	escapable = `$∧;@\()`
)

// Parse reads a clause (docs/SYNTAX.md, "Clauses"). The empty text is true,
// And{}.
func Parse(text string) (And, error) {
	p := &parser{src: []rune(text)}
	f, err := p.clause()
	if err != nil {
		return And{}, fmt.Errorf("clause.Parse: %q: %w", text, err)
	}
	if !p.atEnd() {
		return And{}, fmt.Errorf("clause.Parse: %q: unexpected %q at %d", text, string(p.src[p.pos:]), p.pos)
	}
	return f, nil
}

// ParseAuthored reads a clause an author wrote: Parse, refusing ∃, which
// only tools write.
func ParseAuthored(text string) (And, error) {
	f, err := Parse(text)
	if err != nil {
		return And{}, err
	}
	if hasExists(f) {
		return And{}, fmt.Errorf("clause.ParseAuthored: %q: @exists is written by tools only", text)
	}
	return f, nil
}

func hasExists(f And) bool {
	for _, c := range f.Conjuncts {
		if _, ok := c.(Exists); ok {
			return true
		}
	}
	return false
}

type parser struct {
	src   []rune
	pos   int
	scope []string // names bound by enclosing ∃, innermost last
	body  int      // how many ∃ bodies the parser is in
}

func (p *parser) atEnd() bool { return p.pos >= len(p.src) }

func (p *parser) peek() rune { return p.peekAt(0) }

func (p *parser) peekAt(n int) rune {
	if p.pos+n >= len(p.src) {
		return 0
	}
	return p.src[p.pos+n]
}

func (p *parser) hasPrefix(s string) bool {
	return strings.HasPrefix(string(p.src[p.pos:]), s)
}

func (p *parser) expect(s string) error {
	if !p.hasPrefix(s) {
		return fmt.Errorf("expected %q at %d", s, p.pos)
	}
	p.pos += len([]rune(s))
	return nil
}

// clause reads conjuncts up to the end, or, in an ∃ body, up to the ")"
// that closes it.
func (p *parser) clause() (And, error) {
	if p.atEnd() || p.closesBody() {
		return And{}, nil
	}
	var cs []Formula
	for {
		c, err := p.conjunct()
		if err != nil {
			return And{}, err
		}
		cs = append(cs, c)
		if !p.hasPrefix(conjSep) {
			return And{Conjuncts: cs}, nil
		}
		p.pos += len([]rune(conjSep))
	}
}

func (p *parser) conjunct() (Formula, error) {
	if p.hasPrefix(existsWord) {
		return p.exists()
	}
	if p.peek() == toolPrefix {
		return p.builtin()
	}
	return p.atom()
}

const (
	existsWord = "@exists("
	existsType = " : "
)

// exists reads @exists(name : type) (clause); name is Bound{0} in the
// clause.
func (p *parser) exists() (Formula, error) {
	start := p.pos
	p.pos += len([]rune(existsWord))
	hint := p.name()
	if hint == "" {
		return nil, fmt.Errorf("@exists at %d: expected a name", start)
	}
	if err := ident.CheckName(hint); err != nil {
		return nil, fmt.Errorf("@exists at %d: %w", start, err)
	}
	if err := p.expect(existsType); err != nil {
		return nil, fmt.Errorf("@exists at %d: %w", start, err)
	}
	typ := p.name()
	if typ == "" {
		return nil, fmt.Errorf("@exists at %d: expected the type %s ranges over", start, hint)
	}
	if err := p.expect(") ("); err != nil {
		return nil, fmt.Errorf("@exists at %d: %w", start, err)
	}
	p.scope = append(p.scope, hint)
	p.body++
	body, err := p.clause()
	p.scope, p.body = p.scope[:len(p.scope)-1], p.body-1
	if err != nil {
		return nil, err
	}
	if len(body.Conjuncts) == 0 {
		return nil, fmt.Errorf("@exists at %d: an empty body", start)
	}
	if err := p.expect(")"); err != nil {
		return nil, fmt.Errorf("@exists at %d: %w", start, err)
	}
	if !p.atEnd() && !p.hasPrefix(conjSep) && !p.closesBody() {
		return nil, fmt.Errorf("@exists at %d: expected %q or the end at %d", start, conjSep, p.pos)
	}
	return Exists{Hint: hint, Type: typ, Body: body}, nil
}

// closesBody reports whether the parser is at the ")" that closes the ∃
// body it is in.
func (p *parser) closesBody() bool { return p.body > 0 && p.peek() == ')' }

// builtin reads @word(term, …), by the table of builtins.
func (p *parser) builtin() (Formula, error) {
	start := p.pos
	p.pos++ // @
	op := Op(p.name())
	spec, ok := op.Spec()
	if !ok {
		return nil, fmt.Errorf("unknown tool word @%s at %d", op, start)
	}
	if err := p.expect("("); err != nil {
		return nil, err
	}
	b := Builtin{Op: op, Args: []Term{}}
	for i := 0; i < spec.Terms || (spec.Variadic && !p.hasPrefix(")")); i++ {
		if i > 0 {
			if err := p.expect(", "); err != nil {
				return nil, fmt.Errorf("@%s at %d takes %d terms: %w", op, start, spec.Terms, err)
			}
		}
		t, err := p.term()
		if err != nil {
			return nil, fmt.Errorf("@%s at %d: %w", op, start, err)
		}
		b.Args = append(b.Args, t)
	}
	if spec.State {
		if err := p.expect(", "); err != nil {
			return nil, fmt.Errorf("@%s at %d takes a state: %w", op, start, err)
		}
		if b.State = p.name(); b.State == "" {
			return nil, fmt.Errorf("@%s at %d: expected a state at %d", op, start, p.pos)
		}
	}
	if err := p.expect(")"); err != nil {
		return nil, fmt.Errorf("@%s at %d: %w", op, start, err)
	}
	if !p.atEnd() && !p.hasPrefix(conjSep) && !p.closesBody() {
		return nil, fmt.Errorf("@%s at %d: expected %q or the end at %d", op, start, conjSep, p.pos)
	}
	return b, nil
}

// atom reads text and holes up to an unescaped " ∧ ", the end, or, in an ∃
// body, a ")" that closes more than the atom opened.
func (p *parser) atom() (Formula, error) {
	var parts []Part
	var text []rune
	depth := 0
	for !p.atEnd() && !p.hasPrefix(conjSep) && !(depth == 0 && p.closesBody()) {
		r := p.peek()
		if p.body > 0 && r == '(' {
			depth++
		} else if p.body > 0 && r == ')' {
			depth--
		}
		switch {
		case r == escape && p.peekAt(1) != 0 && strings.ContainsRune(escapable, p.peekAt(1)):
			text = append(text, p.peekAt(1))
			p.pos += 2
		case p.atHole():
			if len(text) > 0 {
				parts, text = append(parts, Text(text)), nil
			}
			t, err := p.hole()
			if err != nil {
				return nil, err
			}
			parts = append(parts, Hole{Term: t})
		default:
			text = append(text, r)
			p.pos++
		}
	}
	if len(text) > 0 {
		parts = append(parts, Text(text))
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty conjunct at %d", p.pos)
	}
	return Atom{Parts: parts}, nil
}

// atHole reports whether a hole starts here: $ before a brace, or before
// what may start a name.
func (p *parser) atHole() bool {
	return p.peek() == holeSigil && (p.peekAt(1) == '{' || startsHoleName(p.peekAt(1)))
}

func (p *parser) hole() (Term, error) {
	if p.peekAt(1) == '{' {
		return p.termHole()
	}
	return p.nameHole()
}

// nameHole reads $name or $name', and the space after it that belongs to
// it.
func (p *parser) nameHole() (Term, error) {
	p.pos++ // $
	name := p.name()
	if err := ident.CheckName(name); err != nil {
		return nil, err
	}
	primed := p.prime()
	p.spaceAfterHole()
	return p.variable(name, primed)
}

// variable resolves name against the enclosing binders.
func (p *parser) variable(name string, primed bool) (Term, error) {
	for i := len(p.scope) - 1; i >= 0; i-- {
		if p.scope[i] == name {
			if primed {
				return nil, fmt.Errorf("%s is bound by @exists and has no value after the step", name)
			}
			return Bound{Index: len(p.scope) - 1 - i}, nil
		}
	}
	return Free{Name: name, Primed: primed}, nil
}

// startsHoleName reports whether r may start the name of a $name hole: an
// identifier character that is not a digit, so that $100 stays text.
func startsHoleName(r rune) bool { return ident.IsRune(r) && !unicode.IsDigit(r) }

// termHole reads ${term}, and the space after it that belongs to it.
func (p *parser) termHole() (Term, error) {
	p.pos += 2 // ${
	t, err := p.term()
	if err != nil {
		return nil, err
	}
	if err := p.expect("}"); err != nil {
		return nil, err
	}
	p.spaceAfterHole()
	return t, nil
}

// spaceAfterHole takes in the one space after a hole that belongs to it:
// one before a character that could continue a name, or before another
// space, so that text after a hole may start with a space before either.
func (p *parser) spaceAfterHole() {
	if p.peek() == ' ' && (ident.IsRune(p.peekAt(1)) || p.peekAt(1) == ' ') {
		p.pos++
	}
}

// term reads a literal, a variable with its path, or an environment
// function applied to its arguments:
//
//	term = literal / id ["'"] *("[" term "]" / "." id) ["'"] / id "(" [term *(", " term)] ")"
//
// A prime after the path primes the variable the path starts from:
// m[k].f' is the entry of m after the step, its key read before it.
func (p *parser) term() (Term, error) {
	start := p.pos
	if p.atLiteral() {
		return p.literal()
	}
	name := p.name()
	if name == "" {
		return nil, fmt.Errorf("expected a term at %d", p.pos)
	}
	if err := ident.CheckName(name); err != nil {
		return nil, fmt.Errorf("at %d: %w", start, err)
	}
	if p.hasPrefix("(") {
		return p.app(name)
	}
	primed := p.prime()
	t, err := p.variable(name, false)
	if err != nil {
		return nil, err
	}
	path := false
	for {
		if p.hasPrefix("[") {
			p.pos++
			k, err := p.term()
			if err != nil {
				return nil, err
			}
			if err := p.expect("]"); err != nil {
				return nil, err
			}
			t, path = Index{Map: t, Key: k}, true
		} else if p.hasPrefix(".") {
			p.pos++
			f := p.name()
			if f == "" {
				return nil, fmt.Errorf("expected a field at %d", p.pos)
			}
			if err := ident.CheckName(f); err != nil {
				return nil, fmt.Errorf("at %d: %w", p.pos-len([]rune(f)), err)
			}
			t, path = Field{Of: t, Name: f}, true
		} else {
			break
		}
	}
	if p.prime() {
		if primed || !path {
			return nil, fmt.Errorf("the term at %d is primed twice", start)
		}
		primed = true
	}
	if !primed {
		return t, nil
	}
	primedT, err := PrimeRoot(t)
	if err != nil {
		return nil, fmt.Errorf("the term at %d starts from a variable bound by @exists, which has no value after the step", start)
	}
	return primedT, nil
}

// atLiteral reports whether a literal starts here: a word a term reads as
// one, or what starts no name.
func (p *parser) atLiteral() bool {
	if startsLiteral(p.peek()) {
		return true
	}
	save := p.pos
	w := p.name()
	p.pos = save
	return ident.IsReserved(w) && !strings.HasPrefix(string(p.src[p.pos+len([]rune(w)):]), "(")
}

// literal reads a literal, which is a whole term: nothing is read of it
// and it has no value after the step.
func (p *parser) literal() (Term, error) {
	start := p.pos
	text, pos, err := readLiteral(p.src, p.pos)
	if err != nil {
		return nil, err
	}
	p.pos = pos
	switch p.peek() {
	case '\'':
		return nil, fmt.Errorf("a literal at %d has no value after the step", start)
	case '[', '.':
		return nil, fmt.Errorf("a path starts from a variable, not from the literal %s at %d", text, start)
	}
	return Lit{Text: text}, nil
}

func (p *parser) app(fn string) (Term, error) {
	p.pos++ // (
	args := []Term{}
	for !p.hasPrefix(")") {
		if len(args) > 0 {
			if err := p.expect(", "); err != nil {
				return nil, err
			}
		}
		a, err := p.term()
		if err != nil {
			return nil, err
		}
		args = append(args, a)
	}
	p.pos++
	if p.peek() == '\'' {
		return nil, fmt.Errorf("an environment function at %d has no value after the step", p.pos)
	}
	return App{Fn: fn, Args: args}, nil
}

func (p *parser) prime() bool {
	if p.peek() == '\'' {
		p.pos++
		return true
	}
	return false
}

// name reads an identifier, or nothing.
func (p *parser) name() string {
	start := p.pos
	for !p.atEnd() && ident.IsRune(p.peek()) {
		p.pos++
	}
	return string(p.src[start:p.pos])
}
