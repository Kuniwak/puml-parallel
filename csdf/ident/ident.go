// Package ident is the one identifier rule of CSDF (docs/SYNTAX.md, id):
// state IDs, state variables, channels, fields and the names in clauses all
// follow it.
package ident

import (
	"fmt"
	"unicode"
)

// Pattern is the rule as a regular expression, for parsers built on regexp.
const Pattern = `[\p{L}\p{N}_]+`

// IsRune reports whether r may appear in an identifier: a Unicode letter
// (category L), a Unicode number (category N, as Pattern's \p{N}) or _.
// Letters and numbers of any script are accepted so that identifiers can be
// written in any language; PlantUML accepts them too, which keeps CSDF a
// subset of PlantUML.
func IsRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_'
}

// Is reports whether s is an identifier.
func Is(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !IsRune(r) {
			return false
		}
	}
	return true
}

// The words that are literals, never names.
const (
	TrueWord  = "true"
	FalseWord = "false"
	NullWord  = "null"
)

// IsReserved reports whether s is a word a term reads as a literal, and so
// is no name.
func IsReserved(s string) bool { return s == TrueWord || s == FalseWord || s == NullWord }

// CheckName refuses a name that is a reserved word, or that starts with a
// digit, as a literal number does. It does not check the characters; Is
// does, and Check does both.
func CheckName(name string) error {
	if IsReserved(name) {
		return fmt.Errorf("%s is a reserved word, a literal, and names nothing", name)
	}
	if r := []rune(name); len(r) > 0 && unicode.IsDigit(r[0]) {
		return fmt.Errorf("a name does not start with a digit, as %s does", name)
	}
	return nil
}

// Check refuses s unless it is a name: an identifier (Is) that CheckName
// accepts. State IDs, state variables, channels, fields, the names in
// clauses and in a global diagram's directives are all names.
func Check(s string) error {
	if !Is(s) {
		return fmt.Errorf("%q is not an identifier, which is letters, digits and _", s)
	}
	return CheckName(s)
}

// CheckAll refuses the first of names that Check refuses.
func CheckAll(names ...string) error {
	for _, n := range names {
		if err := Check(n); err != nil {
			return err
		}
	}
	return nil
}
