package animation

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/v6/csdf/clause"
)

func free(name string, primed bool) clause.Term { return clause.Free{Name: name, Primed: primed} }

func TestSpellConjunctShouldSpellEachConjunct(t *testing.T) {
	m, m2, k, v := free("m", false), free("m", true), free("k", false), free("v", false)
	// ∃ n : T. n' = n ∧ n ≠ n₀, whose hint n clashes with the free n
	exists := clause.Exists{Hint: "n", Type: "T", Body: clause.And{Conjuncts: []clause.Formula{
		clause.Builtin{Op: clause.OpEq, Args: []clause.Term{free("n", true), clause.Bound{Index: 0}}},
		clause.Builtin{Op: clause.OpNe, Args: []clause.Term{clause.Bound{Index: 0}, free("n", false)}},
	}}}
	hole := clause.Atom{Parts: []clause.Part{
		clause.Hole{Term: clause.Field{Of: clause.Index{Map: m2, Key: k}, Name: "x"}},
		clause.Text(" が 0"),
	}}
	tests := map[string]struct {
		words Phrases
		f     clause.Formula
		want  string
	}{
		"exists named apart from a variable of the diagram": {English, clause.Exists{Hint: "cash", Type: "T", Body: clause.And{Conjuncts: []clause.Formula{
			clause.Builtin{Op: clause.OpEq, Args: []clause.Term{free("n", true), clause.Bound{Index: 0}}},
		}}}, "for some cash2 of T, n' is cash2"},
		"ne in English":               {English, clause.Builtin{Op: clause.OpNe, Args: []clause.Term{k, v}}, "k is not v"},
		"ne in Japanese":              {Japanese, clause.Builtin{Op: clause.OpNe, Args: []clause.Term{k, v}}, "k は v でない"},
		"eq in English":               {English, clause.Builtin{Op: clause.OpEq, Args: []clause.Term{k, v}}, "k is v"},
		"eq in Japanese":              {Japanese, clause.Builtin{Op: clause.OpEq, Args: []clause.Term{k, v}}, "k は v"},
		"in in English":               {English, clause.Builtin{Op: clause.OpIn, Args: []clause.Term{k, m}}, "m has k"},
		"in in Japanese":              {Japanese, clause.Builtin{Op: clause.OpIn, Args: []clause.Term{k, m}}, "m に k がある"},
		"notin in English":            {English, clause.Builtin{Op: clause.OpNotIn, Args: []clause.Term{k, m}}, "m has no k"},
		"notin in Japanese":           {Japanese, clause.Builtin{Op: clause.OpNotIn, Args: []clause.Term{k, m}}, "m に k がない"},
		"empty in English":            {English, clause.Builtin{Op: clause.OpEmpty, Args: []clause.Term{m}}, "m is empty"},
		"empty in Japanese":           {Japanese, clause.Builtin{Op: clause.OpEmpty, Args: []clause.Term{m}}, "m は空"},
		"update in English":           {English, clause.Builtin{Op: clause.OpUpdate, Args: []clause.Term{m2, m, k, v}}, "m' is m with k set to v"},
		"update in Japanese":          {Japanese, clause.Builtin{Op: clause.OpUpdate, Args: []clause.Term{m2, m, k, v}}, "m' は m の k を v にしたもの"},
		"frame in English":            {English, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m, k, v}}, "m' is m except at k, v"},
		"frame in Japanese":           {Japanese, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m, k, v}}, "m' は k・v のほか m と同じ"},
		"frame of no key in English":  {English, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m}}, "m' is m"},
		"frame of no key in Japanese": {Japanese, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m}}, "m' は m"},
		"at in English":               {English, clause.Builtin{Op: clause.OpAt, Args: []clause.Term{k}, States: []string{"A", "B"}}, "k is at A, B"},
		"at in Japanese":              {Japanese, clause.Builtin{Op: clause.OpAt, Args: []clause.Term{k}, States: []string{"A", "B"}}, "k は A・B にある"},
		"exists in English":           {English, exists, "for some n2 of T, (n' is n2 and n2 is not n)"},
		"exists in Japanese":          {Japanese, exists, "ある T の n2 について (n' は n2 かつ n2 は n でない)"},
		"exists of one conjunct": {English, clause.Exists{Hint: "x", Type: "T", Body: clause.And{Conjuncts: []clause.Formula{
			clause.Builtin{Op: clause.OpEq, Args: []clause.Term{free("n", true), clause.Bound{Index: 0}}},
		}}}, "for some x of T, n' is x"},
		"a hole without braces":                          {Japanese, hole, "$m'[k].x が 0"},
		"a hole before text that would read as its path": {English, clause.Atom{Parts: []clause.Part{clause.Hole{Term: m}, clause.Text(".x and [k]")}}, "$m .x and [k]"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := spellConjunct(tt.f, tt.words, map[string]bool{"cash": true})

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("spellConjunct() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSpellTextShouldJoinTheConjunctsInTheLanguage(t *testing.T) {
	tests := map[string]struct {
		c    clause.And
		want string
	}{
		"no conjunct": {clause.And{}, "(none)"},
		"two conjuncts, one nested": {clause.And{Conjuncts: []clause.Formula{
			clause.Builtin{Op: clause.OpNe, Args: []clause.Term{free("k", false), free("v", false)}},
			clause.And{Conjuncts: []clause.Formula{clause.Builtin{Op: clause.OpEmpty, Args: []clause.Term{free("m", false)}}}},
		}}, "k は v でない かつ m は空"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := SpellText(tt.c, Japanese, nil)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("SpellText() = %q, want %q", got, tt.want)
			}
		})
	}
}
