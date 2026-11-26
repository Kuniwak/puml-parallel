package animation

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Kuniwak/puml-parallel/v6/csdf/clause"
)

// Phrases are the words a clause is spelled in for people.
type Phrases struct {
	// Builtins spell each builtin, its terms in order as the phrase's
	// operands.
	Builtins map[clause.Op]string
	// Exists spells an ∃ of its name, its type and its body; And joins
	// conjuncts; Separator lists the keys of @frame and the states of @at.
	Exists, And, Separator string
}

// English and Japanese are the phrases of those languages.
var (
	English = Phrases{Builtins: map[clause.Op]string{
		clause.OpEq:     "%[1]s is %[2]s",
		clause.OpNe:     "%[1]s is not %[2]s",
		clause.OpIn:     "%[2]s has %[1]s",
		clause.OpNotIn:  "%[2]s has no %[1]s",
		clause.OpEmpty:  "%[1]s is empty",
		clause.OpUpdate: "%[1]s is %[2]s with %[3]s set to %[4]s",
		clause.OpFrame:  "%[1]s is %[2]s except at %[3]s",
		clause.OpAt:     "%[1]s is at %[2]s",
	}, Exists: "for some %[1]s of %[2]s, %[3]s", And: " and ", Separator: ", "}
	Japanese = Phrases{Builtins: map[clause.Op]string{
		clause.OpEq:     "%[1]s は %[2]s",
		clause.OpNe:     "%[1]s は %[2]s でない",
		clause.OpIn:     "%[2]s に %[1]s がある",
		clause.OpNotIn:  "%[2]s に %[1]s がない",
		clause.OpEmpty:  "%[1]s は空",
		clause.OpUpdate: "%[1]s は %[2]s の %[3]s を %[4]s にしたもの",
		clause.OpFrame:  "%[1]s は %[3]s のほか %[2]s と同じ",
		clause.OpAt:     "%[1]s は %[2]s にある",
	}, Exists: "ある %[2]s の %[1]s について %[3]s", And: " かつ ", Separator: "・"}
)

var byLanguage = map[string]Phrases{"en": English, "ja": Japanese}

// PhrasesOf are the phrases of the language lang names, and whether there
// are any.
func PhrasesOf(lang string) (Phrases, bool) {
	p, ok := byLanguage[lang]
	return p, ok
}

// Languages are the names of the languages there are phrases of, sorted.
func Languages() []string { return slices.Sorted(maps.Keys(byLanguage)) }

// SpellText is c spelled in w as one text, its conjuncts joined by w's
// And, or "(none)", as ClauseText writes it, for a clause of no conjunct.
func SpellText(c clause.And, w Phrases, avoid map[string]bool) (string, error) {
	parts, err := Spell(c, w, avoid)
	if err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return ClauseText(c), nil
	}
	return strings.Join(parts, w.And), nil
}

// Spell is the conjuncts of c, those of nested ∧ in their place, each
// in w's words; a variable an ∃ binds is named apart from avoid. A
// conjunct w has no phrase for, or one that cannot be printed, is an
// error: the words never fall back on the clause syntax.
func Spell(c clause.And, w Phrases, avoid map[string]bool) ([]string, error) {
	var out []string
	for _, f := range c.Conjuncts {
		if inner, ok := f.(clause.And); ok {
			parts, err := Spell(inner, w, avoid)
			if err != nil {
				return nil, err
			}
			out = append(out, parts...)
			continue
		}
		text, err := spellConjunct(f, w, avoid)
		if err != nil {
			return nil, err
		}
		out = append(out, text)
	}
	return out, nil
}

func spellConjunct(f clause.Formula, w Phrases, avoid map[string]bool) (string, error) {
	switch f := f.(type) {
	case clause.Builtin:
		if f.Op == clause.OpFrame && len(f.Args) == 2 {
			// A frame of no key keeps the whole map: m' = m.
			f.Op = clause.OpEq
		}
		phrase, ok := w.Builtins[f.Op]
		if !ok {
			return "", fmt.Errorf("no phrase for @%s", f.Op)
		}
		return phrased(phrase, f, w.Separator)
	case clause.Exists:
		return opened(f, w, avoid)
	case clause.Atom:
		return holed(f)
	}
	return "", fmt.Errorf("no spelling of the conjunct %T", f)
}

// opened is e in w's words, its variable named apart from the names free
// in it and from avoid, and its body in parentheses when it has more than
// one conjunct.
func opened(e clause.Exists, w Phrases, avoid map[string]bool) (string, error) {
	taken := map[string]bool{}
	maps.Copy(taken, avoid)
	for _, v := range clause.FreeVars(e) {
		taken[v.Name] = true
	}
	name := clause.Fresh(taken, e.Hint)
	taken[name] = true
	body, err := clause.Open(e, clause.Free{Name: name})
	if err != nil {
		return "", err
	}
	parts, err := Spell(body, w, taken)
	if err != nil {
		return "", err
	}
	text := strings.Join(parts, w.And)
	if len(parts) > 1 {
		// The body in parentheses, so that a conjunct after the ∃ is not
		// read as a part of it.
		text = "(" + text + ")"
	}
	return fmt.Sprintf(w.Exists, name, e.Type, text), nil
}

// holed is a with each hole written $ and its term, without braces or
// escapes, a space after a hole that text could otherwise continue, as a
// name or as a path.
func holed(a clause.Atom) (string, error) {
	var sb strings.Builder
	afterHole := false
	for _, p := range a.Parts {
		switch p := p.(type) {
		case clause.Text:
			if afterHole && continuesHole(string(p)) {
				sb.WriteString(" ")
			}
			sb.WriteString(string(p))
			afterHole = false
		case clause.Hole:
			s, err := clause.PrintTerm(p.Term)
			if err != nil {
				return "", err
			}
			sb.WriteString("$" + s)
			afterHole = true
		}
	}
	return sb.String(), nil
}

// continuesHole says whether text, after a hole, would read as part of it:
// a name going on, a prime, or a path (.f, [k], (args)).
func continuesHole(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return strings.ContainsRune("_'.[(", r) || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// phrased is b in phrase: its terms are the operands in order, but that the terms of a variadic
// builtin past those it always takes, and the states of @at, are one
// operand, listed with sep.
func phrased(phrase string, b clause.Builtin, sep string) (string, error) {
	printed := make([]string, len(b.Args))
	for i, t := range b.Args {
		s, err := clause.PrintTerm(t)
		if err != nil {
			return "", err
		}
		printed[i] = s
	}
	spec, _ := b.Op.Spec()
	var operands []any
	for _, s := range printed[:min(spec.Terms, len(printed))] {
		operands = append(operands, s)
	}
	if spec.Variadic {
		operands = append(operands, strings.Join(printed[min(spec.Terms, len(printed)):], sep))
	}
	if spec.State {
		operands = append(operands, strings.Join(b.States, sep))
	}
	return fmt.Sprintf(phrase, operands...), nil
}
