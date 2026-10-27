package obligationir

import (
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"strconv"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
)

// LiteralSpeller spells the values of literals in a prover, each case a
// constructor of its value type applied to the parts of the value, already
// spelled for the parts that are values. A value is pure data, so two
// literals are one value exactly when their spellings are.
type LiteralSpeller struct {
	// Int spells an integer, given in decimal.
	Int func(decimal string) string
	// Float spells a float as (-1)^neg · sig · 2^exp in its canonical
	// form (clause.FloatParts): sig odd, or the zero, sig 0, exp 0 and neg false.
	Float func(neg bool, sig uint64, exp int) string
	// String spells a string.
	String func(s string) string
	// Bool spells a truth value.
	Bool func(b bool) string
	// Array spells an array of the values items.
	Array func(items []string) string
	// Object spells an object of the keys, sorted, and their values.
	Object func(keys []string, values []string) string
}

// spellLiteral spells the literal whose canonical text is text by s.
func (s LiteralSpeller) spellLiteral(text string) (string, error) {
	lit, err := clause.ParseLiteral(text)
	if err != nil {
		return "", err
	}
	if lit.Text != text {
		return "", fmt.Errorf("obligationir: the literal %s is not in its canonical form %s", text, lit.Text)
	}
	v, err := lit.Decode()
	if err != nil {
		return "", err
	}
	return s.value(v)
}

func (s LiteralSpeller) value(v any) (string, error) {
	switch v := v.(type) {
	case json.Number:
		if !clause.IsFloatText(string(v)) {
			n, ok := new(big.Int).SetString(string(v), 10)
			if !ok {
				return "", fmt.Errorf("obligationir: %s is no integer", v)
			}
			return s.Int(n.String()), nil
		}
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return "", fmt.Errorf("obligationir: %s is no float: %w", v, err)
		}
		return s.Float(clause.FloatParts(f)), nil
	case string:
		return s.String(v), nil
	case bool:
		return s.Bool(v), nil
	case []any:
		items := make([]string, len(v))
		for i, x := range v {
			sp, err := s.value(x)
			if err != nil {
				return "", err
			}
			items[i] = sp
		}
		return s.Array(items), nil
	case map[string]any:
		keys := slices.Sorted(func(yield func(string) bool) {
			for k := range v {
				if !yield(k) {
					return
				}
			}
		})
		values := make([]string, len(keys))
		for i, k := range keys {
			sp, err := s.value(v[k])
			if err != nil {
				return "", err
			}
			values[i] = sp
		}
		return s.Object(keys, values), nil
	}
	return "", fmt.Errorf("obligationir: %#v is no value of a literal", v)
}
