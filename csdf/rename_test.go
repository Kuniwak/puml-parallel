package csdf

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSplitRenaming(t *testing.T) {
	tests := map[Predicate]struct {
		want []Rename
		body Predicate
	}{
		"y < 5":                   {body: "y < 5"},
		"@[y := x] y < 5":         {want: []Rename{{From: "y", To: "x"}}, body: "y < 5"},
		"@[z := w, y := x] y < z": {want: []Rename{{From: "y", To: "x"}, {From: "z", To: "w"}}, body: "y < z"},
		"@[監査ID := id] 監査ID は有効":  {want: []Rename{{From: "監査ID", To: "id"}}, body: "監査ID は有効"},
	}
	for clause, tt := range tests {
		t.Run(string(clause), func(t *testing.T) {
			got, body, err := SplitRenaming(clause)
			if err != nil || !slices.Equal(got, tt.want) || body != tt.body {
				t.Errorf("SplitRenaming(%q) = %v, %q, %v, want %v, %q", clause, got, body, err, tt.want, tt.body)
			}
		})
	}
}

func TestSplitRenamingRefusesAMalformedRenaming(t *testing.T) {
	for _, clause := range []Predicate{"@[y := x]", "@[y = x] y", "@[y := x y", "@[y := x, y := z] y", "@[] y", "@[y := x] @opaque", "@[y := y] y"} {
		t.Run(string(clause), func(t *testing.T) {
			if _, _, err := SplitRenaming(clause); err == nil {
				t.Errorf("SplitRenaming(%q) error = nil, want one", clause)
			}
		})
	}
}

func TestRenameClause(t *testing.T) {
	tests := []struct {
		name    string
		clause  Predicate
		renames []Rename
		want    Predicate
	}{
		{"a plain clause", "y < 5", []Rename{{"y", "x"}}, "@[y := x] y < 5"},
		{"nothing to rename", "y < 5", nil, "y < 5"},
		{"onto a renaming, which is composed", "@[z := y] z < 5", []Rename{{"y", "x"}}, "@[y := x, z := x] z < 5"},
		{"an identity left out", "@[z := y] z < 5", []Rename{{"y", "z"}}, "@[y := z] z < 5"},
		{"an opaque clause reads nothing", "@opaque", []Rename{{"y", "x"}}, "@opaque"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := RenameClause(tt.clause, tt.renames); err != nil || got != tt.want {
				t.Errorf("RenameClause(%q, %v) = %q, %v, want %q", tt.clause, tt.renames, got, err, tt.want)
			}
		})
	}
}

func TestRenameClauseRefusesAMalformedRenaming(t *testing.T) {
	// Execute
	_, err := RenameClause("@[y = x] y", []Rename{{"y", "x"}})

	// Assert
	if err == nil {
		t.Error("RenameClause() error = nil, want the malformed renaming refused")
	}
}

func TestARenamedClauseIsCheckedAndReadByItsBody(t *testing.T) {
	// Setup
	p := Predicate(`x > 0 ∧ @[y := x] y \∧ z`)

	// Execute
	err := CheckClauses(p)
	got, jsonErr := p.ClausesJSON()

	// Assert
	if err != nil || jsonErr != nil {
		t.Fatalf("CheckClauses(%q) = %v, ClausesJSON() error = %v", p, err, jsonErr)
	}
	want := []ClauseJSON{{Text: "x > 0"}, {Text: "y ∧ z", Rename: map[string]string{"y": "x"}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ClausesJSON() mismatch (-want +got):\n%s", diff)
	}
}

func TestAnEscapedClauseHoldsNoRenaming(t *testing.T) {
	// Execute
	renames, body, err := SplitRenaming(EscapeClause("@[y := x] y"))

	// Assert
	if err != nil || renames != nil || body != `\@[y := x] y` {
		t.Errorf("SplitRenaming(EscapeClause(...)) = %v, %q, %v, want no renaming", renames, body, err)
	}
}

func TestSplitEquality(t *testing.T) {
	tests := map[Predicate]struct {
		a, b string
		ok   bool
	}{
		"@eq(y, z)":     {a: "y", b: "z", ok: true},
		"@eq(監査ID, id)": {a: "監査ID", b: "id", ok: true},
		"y = z":         {},
	}
	for clause, tt := range tests {
		t.Run(string(clause), func(t *testing.T) {
			// Execute
			a, b, ok, err := SplitEquality(clause)

			// Assert
			if err != nil || ok != tt.ok || a != tt.a || b != tt.b {
				t.Errorf("SplitEquality(%q) = %q, %q, %v, %v, want %q, %q, %v", clause, a, b, ok, err, tt.a, tt.b, tt.ok)
			}
		})
	}
}

func TestSplitEqualityRefusesAMalformedEquality(t *testing.T) {
	for _, clause := range []Predicate{"@eq(y)", "@eq(y, z", "@eq(y,z)", "@eq(y, y)", "@eq(y, z) ∧"} {
		t.Run(string(clause), func(t *testing.T) {
			// Execute
			_, _, _, err := SplitEquality(clause)

			// Assert
			if err == nil {
				t.Errorf("SplitEquality(%q) error = nil, want one", clause)
			}
		})
	}
}

func TestAnEqualityIsCheckedRenamedAndWrittenAsJSON(t *testing.T) {
	// Setup
	p := Predicate("x > 0 ∧ @eq(y, z)")

	// Execute
	err := CheckClauses(p)
	renamed, renameErr := RenameClause("@eq(y, z)", []Rename{{"z", "w"}})
	got, jsonErr := p.ClausesJSON()

	// Assert
	if err != nil || renameErr != nil || jsonErr != nil {
		t.Fatalf("errors = %v, %v, %v", err, renameErr, jsonErr)
	}
	if renamed != "@eq(y, w)" {
		t.Errorf("RenameClause() = %q, want @eq(y, w): a tool word is the tools' to rename", renamed)
	}
	if diff := cmp.Diff([]ClauseJSON{{Text: "x > 0"}, {Eq: []string{"y", "z"}}}, got); diff != "" {
		t.Errorf("ClausesJSON() mismatch (-want +got):\n%s", diff)
	}
}
