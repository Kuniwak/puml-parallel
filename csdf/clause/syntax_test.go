package clause

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseSyntaxKeepsTheNamesAsWritten(t *testing.T) {
	// Act
	got, err := ParseSyntax("@exists(x : Nat) ($x < $n') ∧ ${m[x].f'} ok")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := Syntax{Conjuncts: []SynConjunct{
		&SynExists{Name: "x", Type: "Nat", Pos: 0, Body: Syntax{Conjuncts: []SynConjunct{
			&SynAtom{Pos: 18, Parts: []SynPart{
				&SynHole{Pos: 18, Term: &SynVar{Name: "x", Pos: 19}},
				&SynText{Text: " < "},
				&SynHole{Pos: 23, Term: &SynVar{Name: "n", Primed: true, Pos: 24}},
			}},
		}}},
		&SynAtom{Pos: 30, Parts: []SynPart{
			&SynHole{Pos: 30, Braced: true, Term: &SynPrime{Pos: 32, Of: &SynField{Pos: 32, Name: "f",
				Of: &SynIndex{Pos: 32, Map: &SynVar{Name: "m", Pos: 32}, Key: &SynVar{Name: "x", Pos: 34}}}}},
			&SynText{Text: "ok"},
		}},
	}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestFormulaOfIsWhatParseReads(t *testing.T) {
	texts := []string{
		"$在庫' = $在庫 - $数量",
		"@exists(x : Nat) ($x < $n) ∧ ${m[k].f'} > 0",
		"@eq(a, b) ∧ @at(k, s0, s1)",
		"",
	}
	for _, text := range texts {
		t.Run(text, func(t *testing.T) {
			// Arrange
			s, err := ParseSyntax(text)
			if err != nil {
				t.Fatal(err)
			}
			want, err := Parse(text)
			if err != nil {
				t.Fatal(err)
			}

			// Act
			got, err := FormulaOf(s)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}
