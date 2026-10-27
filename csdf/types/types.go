// Package types is the types a diagram declares (docs/CLAUSE_DESIGN.md,
// Types): of its state variables, and of the fields of its channels.
package types

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

// AnyName is the type that constrains nothing (docs/CLAUSE_DESIGN.md,
// Types): a value of it may be any value, so a prover quantifies over it
// without a guard, and a tool that enumerates the values of a type refuses
// it.
const AnyName = ident.AnyWord

// Type is the type of a variable or of an event's field: a named type,
// which the environment gives the values of (or AnyName), or a map type
// Key ⇸ Val, a partial map from Key to Val. The zero Type is no type.
type Type struct {
	Name     string
	Key, Val *Type
}

// AnyType is the type AnyName.
var AnyType = Named(AnyName)

// Named is the type named name.
func Named(name string) Type { return Type{Name: name} }

// MapOf is the map type key ⇸ val.
func MapOf(key, val Type) Type { return Type{Key: &key, Val: &val} }

// IsZero says whether t is no type.
func (t Type) IsZero() bool { return t.Name == "" && t.Key == nil }

// IsMap says whether t is a map type.
func (t Type) IsMap() bool { return t.Key != nil }

// IsAny says whether t is AnyName.
func (t Type) IsAny() bool { return t.Name == AnyName }

// Equal says whether t and o are one type.
func (t Type) Equal(o Type) bool { return t.String() == o.String() }

// String writes t as Parse reads it: ⇸ nests to the right, so a map
// type is parenthesised only as a key.
func (t Type) String() string {
	if !t.IsMap() {
		return t.Name
	}
	k := t.Key.String()
	if t.Key.IsMap() {
		k = "(" + k + ")"
	}
	return k + " ⇸ " + t.Val.String()
}

// Parse reads a type: a name (an identifier), a map type K ⇸ V (or
// K ->> V) that nests to the right, or a type in parentheses.
func Parse(text string) (Type, error) {
	r := typeReader{src: text}
	t, err := r.typ()
	if err == nil {
		r.space()
		if r.pos < len(r.src) {
			err = fmt.Errorf("%q follows the type", r.src[r.pos:])
		}
	}
	if err != nil {
		return Type{}, fmt.Errorf("types.Parse: %q is no type: %w", text, err)
	}
	return t, nil
}

type typeReader struct {
	src string
	pos int
}

func (r *typeReader) space() {
	for r.pos < len(r.src) && (r.src[r.pos] == ' ' || r.src[r.pos] == '\t') {
		r.pos++
	}
}

func (r *typeReader) eat(s string) bool {
	r.space()
	if strings.HasPrefix(r.src[r.pos:], s) {
		r.pos += len(s)
		return true
	}
	return false
}

func (r *typeReader) typ() (Type, error) {
	key, err := r.atom()
	if err != nil {
		return Type{}, err
	}
	if r.eat("⇸") || r.eat("->>") {
		val, err := r.typ()
		if err != nil {
			return Type{}, err
		}
		return MapOf(key, val), nil
	}
	return key, nil
}

func (r *typeReader) atom() (Type, error) {
	if r.eat("(") {
		t, err := r.typ()
		if err != nil {
			return Type{}, err
		}
		if !r.eat(")") {
			return Type{}, fmt.Errorf("a ( is not closed")
		}
		return t, nil
	}
	r.space()
	start := r.pos
	for r.pos < len(r.src) {
		c, n := utf8.DecodeRuneInString(r.src[r.pos:])
		if !ident.IsRune(c) {
			break
		}
		r.pos += n
	}
	if start == r.pos {
		return Type{}, fmt.Errorf("no name of a type at %d", start)
	}
	return Named(r.src[start:r.pos]), nil
}
