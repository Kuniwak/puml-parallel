package csdf

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// ClauseConnective joins the clauses of one predicate. A predicate is a
// conjunction of clauses, and the clause is the unit the tools treat as opaque
// text: the connective, with the spaces around it, is grammar, not wording, so
// a clause writes it escaped, \∧ (docs/SYNTAX.md).
const ClauseConnective = " ∧ "

// clauseEscapes are the characters of the tools' vocabulary that a clause
// writes after a backslash to mean the character itself.
const clauseEscapes = "∧;@\\"

// ToolWordPrefix starts every word of the tools' vocabulary in a clause.
const ToolWordPrefix = "@"

func Conjunction(g1, g2 Predicate) Predicate {
	if IsTrue(g1) {
		return g2
	}
	if IsTrue(g2) {
		return g1
	}
	return g1 + ClauseConnective + g2
}

// Clauses splits a predicate into the clauses it conjoins. The omitted
// predicate, and the literal true, have no clause.
func (p Predicate) Clauses() []Predicate {
	if IsTrue(p) {
		return nil
	}
	var clauses []Predicate
	text := string(p)
	start := 0
	for i := 0; i < len(text); {
		if _, size, ok := escapeAt(text, i); ok {
			i += size
			continue
		}
		if strings.HasPrefix(text[i:], ClauseConnective) {
			clauses = append(clauses, Predicate(strings.TrimSpace(text[start:i])))
			i += len(ClauseConnective)
			start = i
			continue
		}
		i++
	}
	return append(clauses, Predicate(strings.TrimSpace(text[start:])))
}

// escapeAt reports whether text holds an escape at i: a backslash before one
// of the characters of the tools' vocabulary. It gives that character and the
// width of the escape. A backslash before anything else escapes nothing.
func escapeAt(text string, i int) (rune, int, bool) {
	if text[i] != '\\' || i+1 >= len(text) {
		return 0, 0, false
	}
	r, size := utf8.DecodeRuneInString(text[i+1:])
	if !strings.ContainsRune(clauseEscapes, r) {
		return 0, 0, false
	}
	return r, 1 + size, true
}

// ClauseText is the clause as its language reads it: the escapes of the
// tools' vocabulary removed. A backslash before any other character stays.
func ClauseText(clause Predicate) string {
	var sb strings.Builder
	text := string(clause)
	for i := 0; i < len(text); {
		if r, size, ok := escapeAt(text, i); ok {
			sb.WriteRune(r)
			i += size
			continue
		}
		sb.WriteByte(text[i])
		i++
	}
	return sb.String()
}

// EscapeClause writes text as one clause, escaping only what would otherwise
// be read as the tools' vocabulary: ; and @, the ∧ of a connective, and a
// backslash that would escape the character after it. So the text of a clause
// that needed no escape is written as it is.
func EscapeClause(text string) Predicate {
	var sb strings.Builder
	for i, r := range text {
		rest := text[i+utf8.RuneLen(r):]
		switch {
		case r == ';' || r == '@':
			sb.WriteByte('\\')
		case r == '∧' && strings.HasSuffix(text[:i], " ") && strings.HasPrefix(rest, " "):
			sb.WriteByte('\\')
		case r == '\\' && rest != "":
			if next, _ := utf8.DecodeRuneInString(rest); strings.ContainsRune(clauseEscapes, next) {
				sb.WriteByte('\\')
			}
		}
		sb.WriteRune(r)
	}
	return Predicate(sb.String())
}

// CheckClauses refuses an empty clause, and a clause that uses a word of the
// tools' vocabulary the tools do not know: an unescaped @ starts one, and
// @opaque is the only one.
func CheckClauses(p Predicate) error {
	for _, c := range p.Clauses() {
		switch {
		case IsOpaque(c):
		case c == "":
			return fmt.Errorf("csdf.CheckClauses: %q has an empty clause, with nothing between two connectives or after one", p)
		case unescapedIndex(string(c), '@') >= 0:
			return fmt.Errorf("csdf.CheckClauses: %q: %s starts a word of the tools, and %s is the only one; write \\@ for the character", c, ToolWordPrefix, PredicateOpaque)
		}
	}
	return nil
}

// unescapedIndex is the index of the first b in text that no backslash
// escapes, or -1.
func unescapedIndex(text string, b byte) int {
	for i := 0; i < len(text); i++ {
		if _, size, ok := escapeAt(text, i); ok {
			i += size - 1
			continue
		}
		if text[i] == b {
			return i
		}
	}
	return -1
}

// ConjoinClauses is the inverse of Clauses: no clause is the literal true.
func ConjoinClauses(clauses []Predicate) Predicate {
	if len(clauses) == 0 {
		return PredicateTrue
	}
	parts := make([]string, len(clauses))
	for i, c := range clauses {
		parts[i] = string(c)
	}
	return Predicate(strings.Join(parts, ClauseConnective))
}

// disjoin combines predicates with a true-aware logical OR. "true" (or the empty
// default) is absorbing: if any disjunct is true the result is "true". Otherwise
// the distinct disjuncts are sorted and joined with " | " for stable output.
func DisjunctionAll(preds []Predicate) Predicate {
	seen := make(map[Predicate]struct{}, len(preds))
	disjuncts := make([]string, 0, len(preds))
	for _, p := range preds {
		if IsTrue(p) {
			return PredicateTrue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		disjuncts = append(disjuncts, string(p))
	}
	if len(disjuncts) == 0 {
		return PredicateTrue
	}
	sort.Strings(disjuncts)
	return Predicate(strings.Join(disjuncts, " ∨ "))
}
