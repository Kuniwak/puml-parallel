// Package ident is the one identifier rule of CSDF (docs/SYNTAX.md, id):
// state IDs, state variables, channels, fields and the names in clauses all
// follow it.
package ident

import "unicode"

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
