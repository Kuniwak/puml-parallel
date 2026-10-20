package csdf

import (
	"reflect"
	"testing"
)

func TestClausesSplitsAConjunctionOnTheCoreConnective(t *testing.T) {
	tests := []struct {
		name string
		p    Predicate
		want []Predicate
	}{
		{name: "the omitted predicate has no clause", p: "", want: nil},
		{name: "true has no clause", p: PredicateTrue, want: nil},
		{name: "a predicate without the connective is one clause", p: "x > 0", want: []Predicate{"x > 0"}},
		{name: "the connective separates clauses", p: "x > 0 ∧ y = x' ∧ z", want: []Predicate{"x > 0", "y = x'", "z"}},
		{name: "a natural-language and is part of the clause", p: "a、かつ b", want: []Predicate{"a、かつ b"}},
		{name: "a bare connective without spaces is part of the clause", p: "a∧b", want: []Predicate{"a∧b"}},
		{name: "Conjunction round-trips", p: Conjunction("a", "b"), want: []Predicate{"a", "b"}},
		{name: "an escaped connective is part of the clause", p: `a \∧ b ∧ c`, want: []Predicate{`a \∧ b`, "c"}},
		{name: "an escaped backslash does not escape the connective", p: `a \\ ∧ b`, want: []Predicate{`a \\`, "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Clauses(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Clauses() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConjoinClausesIsTheInverseOfClauses(t *testing.T) {
	if got := ConjoinClauses(nil); got != PredicateTrue {
		t.Errorf("ConjoinClauses(nil) = %q, want %q", got, PredicateTrue)
	}
	if got := ConjoinClauses([]Predicate{"a", "b"}); got != "a ∧ b" {
		t.Errorf("ConjoinClauses([a b]) = %q, want %q", got, "a ∧ b")
	}
}

func TestIsOpaqueTellsTheReservedClauseOfAClauseNotFormalisedYet(t *testing.T) {
	tests := []struct {
		clause Predicate
		want   bool
	}{
		{clause: PredicateOpaque, want: true},
		{clause: " @opaque ", want: true},
		{clause: "opaque", want: false},
		{clause: `\@opaque`, want: false},
		{clause: "@opaque ∧ x > 0", want: false},
		{clause: "x is @opaque", want: false},
	}
	for _, tt := range tests {
		t.Run(string(tt.clause), func(t *testing.T) {
			if got := IsOpaque(tt.clause); got != tt.want {
				t.Errorf("IsOpaque(%q) = %v, want %v", tt.clause, got, tt.want)
			}
		})
	}
}

func TestFormalClausesLeaveOutTheOpaqueOnes(t *testing.T) {
	tests := []struct {
		p    Predicate
		want []Predicate
	}{
		{p: "@opaque", want: nil},
		{p: "@opaque ∧ x > 0", want: []Predicate{"x > 0"}},
		{p: "x > 0 ∧ y", want: []Predicate{"x > 0", "y"}},
		{p: PredicateTrue, want: nil},
	}
	for _, tt := range tests {
		t.Run(string(tt.p), func(t *testing.T) {
			if got := tt.p.FormalClauses(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FormalClauses() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClauseTextRemovesTheEscapes(t *testing.T) {
	tests := map[Predicate]string{
		"x > 0":             "x > 0",
		`a \∧ b`:            "a ∧ b",
		`f(x) \; g`:         "f(x) ; g",
		`mail \@ host`:      "mail @ host",
		`a \\ b`:            `a \ b`,
		`\n stays as it is`: `\n stays as it is`,
	}
	for clause, want := range tests {
		t.Run(string(clause), func(t *testing.T) {
			if got := ClauseText(clause); got != want {
				t.Errorf("ClauseText(%q) = %q, want %q", clause, got, want)
			}
		})
	}
}

func TestCheckClausesRefusesAnUnknownToolWord(t *testing.T) {
	for p, ok := range map[Predicate]bool{
		"x > 0":                true,
		"@opaque ∧ x > 0":      true,
		`mail \@ host`:         true,
		"@opaq":                false,
		"x > 0 ∧ opaque @here": false,
		"OPAQUE":               true,
	} {
		t.Run(string(p), func(t *testing.T) {
			if err := CheckClauses(p); (err == nil) != ok {
				t.Errorf("CheckClauses(%q) = %v, want ok %v", p, err, ok)
			}
		})
	}
}

func TestEscapeClauseIsTheInverseOfClauseText(t *testing.T) {
	for _, text := range []string{"x > 0", "a ∧ b", "f(x) ; g", "mail @ host", `a \ b`, `a \∧ b`, `\n`} {
		t.Run(text, func(t *testing.T) {
			escaped := EscapeClause(text)
			if got := ClauseText(escaped); got != text {
				t.Errorf("ClauseText(EscapeClause(%q)) = %q (escaped %q)", text, got, escaped)
			}
			if clauses := escaped.Clauses(); len(clauses) != 1 {
				t.Errorf("EscapeClause(%q) = %q is %d clauses, want 1", text, escaped, len(clauses))
			}
			if err := CheckClauses(escaped); err != nil {
				t.Errorf("CheckClauses(EscapeClause(%q)) = %v", text, err)
			}
		})
	}
}
