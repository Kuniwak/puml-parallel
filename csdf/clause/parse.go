package clause

import (
	"fmt"
	"unicode"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

const (
	holeSigil = '$'
	escape    = '\\'
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

func (p *parser) clause() (And, error) {
	if p.atEnd() {
		return And{}, nil
	}
	a, err := p.atom()
	if err != nil {
		return And{}, err
	}
	return And{Conjuncts: []Formula{a}}, nil
}

// atom reads text and holes up to the end.
func (p *parser) atom() (Formula, error) {
	var parts []Part
	var text []rune
	for !p.atEnd() {
		r := p.peek()
		switch {
		case r == escape && p.peekAt(1) == holeSigil:
			text = append(text, holeSigil)
			p.pos += 2
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
	return Atom{Parts: parts}, nil
}

// nameHole reads $name or $name'. One space after it belongs to the hole
// when the next character could continue the name.
func (p *parser) nameHole() Term {
	p.pos++ // $
	start := p.pos
	for !p.atEnd() && ident.IsRune(p.peek()) {
		p.pos++
	}
	v := Free{Name: string(p.src[start:p.pos])}
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
