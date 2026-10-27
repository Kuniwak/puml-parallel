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
// Key ⇸ Val, a partial map from Key to Val. It is built by Named, MapOf or
// Parse; the zero Type is no type, which a lookup that finds none gives.
type Type struct {
	name     string
	key, val *Type
}

// AnyType is the type AnyName.
var AnyType = Named(AnyName)

// Named is the type named name.
func Named(name string) Type { return Type{name: name} }

// MapOf is the map type key ⇸ val.
func MapOf(key, val Type) Type { return Type{key: &key, val: &val} }

// IsZero says whether t is no type.
func (t Type) IsZero() bool { return t.name == "" && t.key == nil }

// IsMap says whether t is a map type.
func (t Type) IsMap() bool { return t.key != nil }

// IsAny says whether t is AnyName.
func (t Type) IsAny() bool { return t.name == AnyName }

// Name is the name of a named type, "" for a map type.
func (t Type) Name() string { return t.name }

// Key is the type of the keys of a map type, no type for another.
func (t Type) Key() Type {
	if t.key == nil {
		return Type{}
	}
	return *t.key
}

// Val is the type of the values of a map type, no type for another.
func (t Type) Val() Type {
	if t.val == nil {
		return Type{}
	}
	return *t.val
}

// Equal says whether t and o are one type.
func (t Type) Equal(o Type) bool {
	if t.IsMap() != o.IsMap() {
		return false
	}
	if !t.IsMap() {
		return t.name == o.name
	}
	return t.Key().Equal(o.Key()) && t.Val().Equal(o.Val())
}

// String writes t as Parse reads it: ⇸ nests to the right, so a map type
// is parenthesised only as a key.
func (t Type) String() string {
	if !t.IsMap() {
		return t.name
	}
	k := t.Key().String()
	if t.Key().IsMap() {
		k = "(" + k + ")"
	}
	return k + " ⇸ " + t.Val().String()
}

// MarshalText writes t as String does, so that a type in JSON is its text.
func (t Type) MarshalText() ([]byte, error) { return []byte(t.String()), nil }

// UnmarshalText reads t as Parse does.
func (t *Type) UnmarshalText(text []byte) error {
	u, err := Parse(string(text))
	if err != nil {
		return err
	}
	*t = u
	return nil
}

// Fields writes the types of a channel's fields as a channel declaration
// does, each after " : " or ".": a map type in parentheses, so that its .
// reads apart from the next field's.
func Fields(ts []Type) string {
	var b strings.Builder
	for i, t := range ts {
		if i == 0 {
			b.WriteString(" : ")
		} else {
			b.WriteString(".")
		}
		if t.IsMap() {
			b.WriteString("(" + t.String() + ")")
		} else {
			b.WriteString(t.String())
		}
	}
	return b.String()
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
