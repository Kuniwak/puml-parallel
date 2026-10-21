package clause

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

const (
	holeSigil = '$'
	escape    = '\\'
	conjSep   = " ∧ "
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
	return f, nil
}

type parser struct {
	src []rune
	pos int
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

func (p *parser) clause() (And, error) {
	if p.atEnd() {
		return And{}, nil
	}
	var cs []Formula
	for {
		c, err := p.atom()
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

// atom reads text and holes up to an unescaped " ∧ " or the end.
func (p *parser) atom() (Formula, error) {
	var parts []Part
	var text []rune
	for !p.atEnd() && !p.hasPrefix(conjSep) {
		r := p.peek()
		switch {
		case r == escape && p.peekAt(1) != 0 && strings.ContainsRune(escapable, p.peekAt(1)):
			text = append(text, p.peekAt(1))
			p.pos += 2
		case r == holeSigil && p.peekAt(1) == '{':
			if len(text) > 0 {
				parts, text = append(parts, Text(text)), nil
			}
			t, err := p.termHole()
			if err != nil {
				return nil, err
			}
			parts = append(parts, Hole{Term: t})
		case r == holeSigil && startsHoleName(p.peekAt(1)):
			if len(text) > 0 {
				parts, text = append(parts, Text(text)), nil
			}
			parts = append(parts, Hole{Term: p.nameHole()})
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

// nameHole reads $name or $name'. One space after it belongs to the hole
// when the next character could continue the name.
func (p *parser) nameHole() Term {
	p.pos++ // $
	v := Free{Name: p.name()}
	if p.peek() == '\'' {
		v.Primed = true
		p.pos++
	}
	if p.peek() == ' ' && ident.IsRune(p.peekAt(1)) {
		p.pos++
	}
	return v
}

// startsHoleName reports whether r may start the name of a $name hole: an
// identifier character that is not a digit, so that $100 stays text.
func startsHoleName(r rune) bool { return ident.IsRune(r) && !unicode.IsDigit(r) }

// termHole reads ${term}.
func (p *parser) termHole() (Term, error) {
	p.pos += 2 // ${
	t, err := p.term()
	if err != nil {
		return nil, err
	}
	return t, p.expect("}")
}

// term reads a variable with its path, or an environment function applied
// to its arguments:
//
//	term = id ["'"] *("[" term "]" / "." id) ["'"] / id "(" [term *(", " term)] ")"
//
// A prime after the path primes the variable the path starts from:
// m[k].f' is the entry of m after the step, its key read before it.
func (p *parser) term() (Term, error) {
	start := p.pos
	name := p.name()
	if name == "" {
		return nil, fmt.Errorf("expected a term at %d", p.pos)
	}
	if p.hasPrefix("(") {
		return p.app(name)
	}
	primed := p.prime()
	var t Term = Free{Name: name}
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
	if primed {
		return primeRoot(t), nil
	}
	return t, nil
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

// primeRoot primes the variable a path starts from.
func primeRoot(t Term) Term {
	switch t := t.(type) {
	case Index:
		return Index{Map: primeRoot(t.Map), Key: t.Key}
	case Field:
		return Field{Of: primeRoot(t.Of), Name: t.Name}
	case Free:
		return Free{Name: t.Name, Primed: true}
	}
	panic("clause.primeRoot: a path starts from a variable")
}
