package csdf

import (
	"slices"
	"testing"
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
			if got := RenameClause(tt.clause, tt.renames); got != tt.want {
				t.Errorf("RenameClause(%q, %v) = %q, want %q", tt.clause, tt.renames, got, tt.want)
			}
		})
	}
}

func TestARenamedClauseIsCheckedAndReadByItsBody(t *testing.T) {
	p := Predicate(`x > 0 ∧ @[y := x] y \∧ z`)
	if err := CheckClauses(p); err != nil {
		t.Fatalf("CheckClauses(%q) = %v", p, err)
	}
	want := []ClauseJSON{{Text: "x > 0"}, {Text: "y ∧ z", Rename: map[string]string{"y": "x"}}}
	got := p.ClausesJSON()
	if len(got) != 2 || got[0].Text != want[0].Text || got[1].Text != want[1].Text || got[1].Rename["y"] != "x" {
		t.Errorf("ClausesJSON() = %#v, want %#v", got, want)
	}
}
