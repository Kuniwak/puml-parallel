package csdf

import (
	"fmt"
	"slices"
	"strings"
)

// ClauseForm is what a clause is (docs/SYNTAX.md): text for whatever reads
// the clause language, or one of the tools' words.
type ClauseForm string

const (
	// FormText is a clause in its language, perhaps under a renaming.
	FormText ClauseForm = "text"
	// FormOpaque is the tool word @opaque, a clause not formalised yet.
	FormOpaque ClauseForm = "opaque"
	// FormEq is the tool word @eq(a, b).
	FormEq ClauseForm = "eq"
)

// ParsedClause is one clause taken apart into its form. It is what every tool
// that reads or writes a clause goes through, so that a tool word is told
// apart in one place.
type ParsedClause struct {
	Form ClauseForm
	// Body is a text clause as written, its escapes in it.
	Body Predicate
	// Renames is the renaming before a text clause, sorted by From.
	Renames []Rename
	// Eq names the two values an equality says are equal.
	Eq [2]string
}

// TextClause is the clause its language reads as text, under renames. The
// renames are sorted and a name renamed onto itself is left out, as
// ParseClause would read them, so the clause writes back to one that parses.
func TextClause(text string, renames []Rename) ParsedClause {
	renames = slices.DeleteFunc(slices.Clone(renames), func(r Rename) bool { return r.From == r.To })
	sortRenames(renames)
	if len(renames) == 0 {
		renames = nil
	}
	return ParsedClause{Form: FormText, Body: EscapeClause(text), Renames: renames}
}

// OpaqueClause is the tool word @opaque.
func OpaqueClause() ParsedClause { return ParsedClause{Form: FormOpaque} }

// EqualityClause is the tool word @eq(a, b).
func EqualityClause(a, b string) ParsedClause {
	return ParsedClause{Form: FormEq, Eq: [2]string{a, b}}
}

// ParseClause takes one clause of a predicate apart. It refuses an empty
// clause, a malformed renaming or equality, and an unescaped @ that starts no
// word the tools know.
func ParseClause(clause Predicate) (ParsedClause, error) {
	if IsOpaque(clause) {
		return OpaqueClause(), nil
	}
	if a, b, ok, err := splitEquality(clause); err != nil {
		return ParsedClause{}, err
	} else if ok {
		return EqualityClause(a, b), nil
	}
	renames, body, err := splitRenaming(clause)
	if err != nil {
		return ParsedClause{}, err
	}
	switch {
	case body == "":
		return ParsedClause{}, fmt.Errorf("csdf.ParseClause: an empty clause, with nothing between two connectives or after one")
	case unescapedIndex(string(body), '@') >= 0:
		return ParsedClause{}, fmt.Errorf("csdf.ParseClause: %q: %s starts a word of the tools, and %s, @eq(a, b) and a renaming are the only ones; write \\@ for the character", body, ToolWordPrefix, PredicateOpaque)
	}
	return ParsedClause{Form: FormText, Body: body, Renames: renames}, nil
}

// Text is a text clause as its language reads it, its escapes removed.
func (c ParsedClause) Text() string { return ClauseText(c.Body) }

// Predicate writes the clause back. A text clause is written as it was, so a
// tool rewrites no clause text.
func (c ParsedClause) Predicate() Predicate {
	switch c.Form {
	case FormOpaque:
		return PredicateOpaque
	case FormEq:
		return NewEquality(c.Eq[0], c.Eq[1])
	case FormText:
	default:
		panic(fmt.Sprintf("csdf.ParsedClause.Predicate: unknown form %q", c.Form))
	}
	if len(c.Renames) == 0 {
		return c.Body
	}
	parts := make([]string, len(c.Renames))
	for i, r := range c.Renames {
		parts[i] = r.From + renamingBind + r.To
	}
	return Predicate(renamingOpen + strings.Join(parts, ", ") + renamingClose + string(c.Body))
}

// JSON is the clause as csdfparse writes it.
func (c ParsedClause) JSON() ClauseJSON {
	switch c.Form {
	case FormOpaque:
		return ClauseJSON{Form: FormOpaque}
	case FormEq:
		return ClauseJSON{Form: FormEq, Eq: []string{c.Eq[0], c.Eq[1]}}
	case FormText:
		return ClauseJSON{Form: FormText, Text: c.Text(), Rename: RenamesMap(c.Renames)}
	}
	panic(fmt.Sprintf("csdf.ParsedClause.JSON: unknown form %q", c.Form))
}

// Mentions reports whether the clause names name, in the names of the
// predicate it is in: an equality by either of its names, a text clause by
// a renaming onto name or, its language being unread, by the name appearing
// in its text unrenamed.
func (c ParsedClause) Mentions(name string) bool {
	switch c.Form {
	case FormEq:
		return c.Eq[0] == name || c.Eq[1] == name
	case FormText:
		for _, r := range c.Renames {
			if r.To == name {
				return true
			}
		}
		return !slices.ContainsFunc(c.Renames, func(r Rename) bool { return r.From == name }) && strings.Contains(c.Text(), name)
	}
	return false
}

// Renamed is the clause read with each From of renames bound to To, and
// whether it still says something: false for an equality renamed onto one
// name, which is the omitted predicate. A text clause gets its renaming composed
// with renames, leaving out a name that comes to stand for itself. An
// equality is the tools' word, so its names are rewritten, and one renamed
// onto a single name says nothing. @opaque reads nothing.
func (c ParsedClause) Renamed(renames []Rename) (ParsedClause, bool) {
	to := RenamesMap(renames)
	outer := func(name string) string {
		if r, ok := to[name]; ok {
			return r
		}
		return name
	}
	switch c.Form {
	case FormOpaque:
		return c, true
	case FormEq:
		a, b := outer(c.Eq[0]), outer(c.Eq[1])
		return EqualityClause(a, b), a != b
	}
	var composed []Rename
	for _, r := range c.Renames {
		composed = append(composed, Rename{From: r.From, To: outer(r.To)})
	}
	for _, r := range renames {
		if !slices.ContainsFunc(c.Renames, func(i Rename) bool { return i.From == r.From }) {
			composed = append(composed, r)
		}
	}
	composed = slices.DeleteFunc(composed, func(r Rename) bool { return r.From == r.To })
	sortRenames(composed)
	if len(composed) == 0 {
		composed = nil
	}
	c.Renames = composed
	return c, true
}
