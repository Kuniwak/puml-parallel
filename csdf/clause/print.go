package clause

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

// Print writes f in the surface syntax (docs/SYNTAX.md, "Clauses"). It
// prints the normal form of f, and Parse(Print(f)) is α-equivalent to f. A
// bound variable is printed with its hint unless that would capture a free
// variable or an enclosing binder's name; then with a fresh name. A formula
// that is not well formed is an error.
func Print(f Formula) (string, error) {
	if err := shaped(f); err != nil {
		return "", fmt.Errorf("clause.Print: %w", err)
	}
	n := Normalize(f)
	if err := WellFormed(n); err != nil {
		return "", fmt.Errorf("clause.Print: %w", err)
	}
	pr := &printer{avoid: map[string]bool{}}
	for _, v := range FreeVars(n) {
		pr.avoid[v.Name] = true
	}
	return pr.and(n, 0)
}

// PrintTerm writes the term t as a hole's content writes it, ${t}: t
// stands under no binder, so an index in it is an error.
func PrintTerm(t Term) (string, error) {
	return (&printer{avoid: map[string]bool{}}).term(t)
}

type printer struct {
	avoid map[string]bool // the free names, which no bound name may take
	scope []string        // the names printed for the enclosing binders, innermost last
	body  int             // how many ∃ bodies the printer is in
}

// and prints the conjuncts of f; after is the rune that follows f, 0 at the
// end and ')' at the end of an ∃ body.
func (pr *printer) and(f And, after rune) (string, error) {
	out := make([]string, len(f.Conjuncts))
	for i, c := range f.Conjuncts {
		next := after
		if i < len(f.Conjuncts)-1 {
			next = ' ' // of " ∧ "
		}
		s, err := pr.conjunct(c, next)
		if err != nil {
			return "", err
		}
		out[i] = s
	}
	return strings.Join(out, conjSep), nil
}

func (pr *printer) conjunct(f Formula, after rune) (string, error) {
	switch f := f.(type) {
	case Exists:
		return pr.exists(f)
	case Builtin:
		return pr.builtin(f)
	case Atom:
		return pr.atom(f, after)
	}
	return "", fmt.Errorf("clause.Print: unknown conjunct %#v", f)
}

func (pr *printer) exists(e Exists) (string, error) {
	taken := map[string]bool{}
	for n := range pr.avoid {
		taken[n] = true
	}
	for _, n := range pr.scope {
		taken[n] = true
	}
	name := Fresh(taken, e.Hint)
	pr.scope = append(pr.scope, name)
	pr.body++
	body, err := pr.and(e.Body, ')')
	pr.scope, pr.body = pr.scope[:len(pr.scope)-1], pr.body-1
	if err != nil {
		return "", err
	}
	return existsWord + name + existsType + e.Type + ") (" + body + ")", nil
}

func (pr *printer) builtin(b Builtin) (string, error) {
	args := make([]string, 0, len(b.Args)+1)
	for _, t := range b.Args {
		s, err := pr.term(t)
		if err != nil {
			return "", err
		}
		args = append(args, s)
	}
	if b.State != "" {
		args = append(args, b.State)
	}
	return string(toolPrefix) + string(b.Op) + "(" + strings.Join(args, ", ") + ")", nil
}

// atom prints the parts of a from the last to the first, since how a part
// is written depends on what follows it; after is the rune after the atom.
func (pr *printer) atom(a Atom, after rune) (string, error) {
	pieces := make([]string, len(a.Parts))
	unmatched := map[int]map[int]bool{}
	if pr.body > 0 {
		unmatched = UnmatchedParens(a)
	}
	next := string(after)
	if after == 0 {
		next = ""
	}
	for i := len(a.Parts) - 1; i >= 0; i-- {
		var s string
		switch p := a.Parts[i].(type) {
		case Text:
			s = EscapeText(string(p), firstRune(next), i == 0, unmatched[i])
		case Hole:
			var err error
			if s, err = pr.hole(p.Term, next); err != nil {
				return "", err
			}
		}
		pieces[i] = s
		next = s + next
	}
	return strings.Join(pieces, ""), nil
}

// hole writes $name when Parse would read it back as the same hole, and
// ${term} otherwise; next is what is printed after it. Either form takes in
// one space after it before a character that could continue a name or
// another space, so a space is written there when the text starts with such
// a character, and one more when it starts with a space before one.
func (pr *printer) hole(t Term, next string) (string, error) {
	s, err := pr.term(t)
	if err != nil {
		return "", err
	}
	n0, n1 := firstRune(next), firstRune(strings.TrimPrefix(next, string(firstRune(next))))
	space := ""
	if ident.IsRune(n0) || (n0 == ' ' && (ident.IsRune(n1) || n1 == ' ')) {
		space = " "
	}
	name, primed, ok := pr.nameOf(t)
	switch {
	case !ok || !startsHoleName(firstRune(name)):
	case n0 == '\'' && !primed:
	default:
		return "$" + s + space, nil
	}
	return "${" + s + "}" + space, nil
}

// nameOf is the name a variable is printed with, and whether it is primed;
// ok is false for any other term.
func (pr *printer) nameOf(t Term) (name string, primed, ok bool) {
	switch t := t.(type) {
	case Free:
		return t.Name, t.Primed, true
	case Bound:
		if t.Index < len(pr.scope) {
			return pr.scope[len(pr.scope)-1-t.Index], false, true
		}
	}
	return "", false, false
}

func (pr *printer) term(t Term) (string, error) {
	switch t := t.(type) {
	case Free:
		if t.Primed {
			return t.Name + "'", nil
		}
		return t.Name, nil
	case Bound:
		name, _, ok := pr.nameOf(t)
		if !ok {
			return "", fmt.Errorf("clause.Print: the index %d is under %d binders", t.Index, len(pr.scope))
		}
		return name, nil
	case Index:
		m, err := pr.term(t.Map)
		if err != nil {
			return "", err
		}
		k, err := pr.term(t.Key)
		if err != nil {
			return "", err
		}
		return m + "[" + k + "]", nil
	case Field:
		o, err := pr.term(t.Of)
		if err != nil {
			return "", err
		}
		return o + "." + t.Name, nil
	case Lit:
		return t.Text, nil
	case App:
		args := make([]string, len(t.Args))
		for i, a := range t.Args {
			s, err := pr.term(a)
			if err != nil {
				return "", err
			}
			args[i] = s
		}
		return t.Fn + "(" + strings.Join(args, ", ") + ")", nil
	}
	return "", fmt.Errorf("clause.Print: unknown term %#v", t)
}

func firstRune(s string) rune {
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

// UnmatchedParens marks, for each text part of a by its index in a.Parts,
// the runes that are parentheses with no partner in the atom. Inside an ∃
// body Parse nests parentheses in text, so only these need escaping there.
func UnmatchedParens(a Atom) map[int]map[int]bool {
	marks := map[int]map[int]bool{}
	type pos struct{ part, r int }
	var open []pos
	for i, p := range a.Parts {
		t, ok := p.(Text)
		if !ok {
			continue
		}
		marks[i] = map[int]bool{}
		for j, r := range []rune(string(t)) {
			switch r {
			case '(':
				open = append(open, pos{i, j})
			case ')':
				if len(open) == 0 {
					marks[i][j] = true
				} else {
					open = open[:len(open)-1]
				}
			}
		}
	}
	for _, o := range open {
		marks[o.part][o.r] = true
	}
	return marks
}

// EscapeText escapes, in the text s of an atom, what Parse would otherwise
// read as syntax: a backslash before a character it escapes, a $ that would
// start a hole, ∧ and ;, an @ that starts a conjunct (startsConjunct), and
// the runes unmatched marks. after is the rune printed after s, 0 at the end.
func EscapeText(s string, after rune, startsConjunct bool, unmatched map[int]bool) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		next := after
		if i+1 < len(rs) {
			next = rs[i+1]
		}
		var esc bool
		switch r {
		case escape:
			esc = next != 0 && strings.ContainsRune(escapable, next)
		case holeSigil:
			esc = startsHoleName(next) || next == '{'
		case '∧', ';':
			esc = true
		case toolPrefix:
			esc = i == 0 && startsConjunct
		case '(', ')':
			esc = unmatched[i]
		}
		if esc {
			b.WriteRune(escape)
		}
		b.WriteRune(r)
	}
	return b.String()
}
