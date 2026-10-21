package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseClauseTellsTheFormOfAClauseInOnePlace(t *testing.T) {
	tests := map[Predicate]struct {
		want     ParsedClause
		wantText string
	}{
		`x \∧ y`:          {want: ParsedClause{Form: FormText, Body: `x \∧ y`}, wantText: "x ∧ y"},
		"@opaque":         {want: ParsedClause{Form: FormOpaque}},
		"@eq(y, z)":       {want: ParsedClause{Form: FormEq, Eq: [2]string{"y", "z"}}},
		"@[y := x] y > 0": {want: ParsedClause{Form: FormText, Body: "y > 0", Renames: []Rename{{"y", "x"}}}, wantText: "y > 0"},
	}
	for clause, tt := range tests {
		t.Run(string(clause), func(t *testing.T) {
			// Setup: the clause is the table's key.

			// Execute
			got, err := ParseClause(clause)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ParseClause() mismatch (-want +got):\n%s", diff)
			}
			if got.Form == FormText && got.Text() != tt.wantText {
				t.Errorf("Text() = %q, want %q", got.Text(), tt.wantText)
			}
		})
	}
}

func TestParsedClauseWritesBackTheClauseAsWritten(t *testing.T) {
	for _, clause := range []Predicate{`x \∧ y`, "@opaque", "@eq(y, z)", "@[y := x] y > 0"} {
		t.Run(string(clause), func(t *testing.T) {
			// Setup
			parsed, err := ParseClause(clause)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			back := parsed.Predicate()

			// Assert
			if back != clause {
				t.Errorf("Predicate() = %q, want %q", back, clause)
			}
		})
	}
}

func TestParseClauseRefusesWhatNoFormIs(t *testing.T) {
	for _, clause := range []Predicate{"", "@opaq", "x @ y", "@eq(y)", "@[y = x] y", "@[a := b] @eq(a, c)"} {
		t.Run(string(clause), func(t *testing.T) {
			// Execute
			_, err := ParseClause(clause)

			// Assert
			if err == nil {
				t.Errorf("ParseClause(%q) error = nil, want one", clause)
			}
		})
	}
}

func TestParsedClauseRenamed(t *testing.T) {
	tests := map[string]struct {
		clause   ParsedClause
		renames  []Rename
		want     ParsedClause
		wantSays bool
	}{
		"a text clause gets a renaming": {
			clause: TextClause("y > 0", nil), renames: []Rename{{"y", "x"}},
			want: ParsedClause{Form: FormText, Body: "y > 0", Renames: []Rename{{"y", "x"}}}, wantSays: true,
		},
		"a renaming is composed onto one there is": {
			clause: TextClause("z > 0", []Rename{{"z", "y"}}), renames: []Rename{{"y", "x"}},
			want: ParsedClause{Form: FormText, Body: "z > 0", Renames: []Rename{{"y", "x"}, {"z", "x"}}}, wantSays: true,
		},
		"a name come to stand for itself is left out": {
			clause: TextClause("z > 0", []Rename{{"z", "y"}}), renames: []Rename{{"y", "z"}},
			want: ParsedClause{Form: FormText, Body: "z > 0", Renames: []Rename{{"y", "z"}}}, wantSays: true,
		},
		"an equality gets its names rewritten": {
			clause: EqualityClause("y", "z"), renames: []Rename{{"z", "w"}},
			want: EqualityClause("y", "w"), wantSays: true,
		},
		"an equality renamed onto one name says nothing": {
			clause: EqualityClause("y", "z"), renames: []Rename{{"z", "y"}},
			want: EqualityClause("y", "y"), wantSays: false,
		},
		"@opaque reads nothing": {
			clause: OpaqueClause(), renames: []Rename{{"y", "x"}},
			want: OpaqueClause(), wantSays: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, says := tt.clause.Renamed(tt.renames)

			// Assert
			if diff := cmp.Diff(tt.want, got); diff != "" || says != tt.wantSays {
				t.Errorf("Renamed() = %v (-want +got):\n%s", says, diff)
			}
		})
	}
}

func TestTextClauseWritesARenamingThatParsesBack(t *testing.T) {
	// Setup: renames out of order, one onto itself.
	clause := TextClause("y < z", []Rename{{"z", "w"}, {"x", "x"}, {"y", "v"}})

	// Execute
	back, err := ParseClause(clause.Predicate())

	// Assert
	if err != nil {
		t.Fatalf("ParseClause(%q) error = %v", clause.Predicate(), err)
	}
	if diff := cmp.Diff(clause, back); diff != "" {
		t.Errorf("round trip mismatch (-want +got):\n%s", diff)
	}
}

func TestParsedClauseMentions(t *testing.T) {
	tests := map[string]struct {
		clause Predicate
		name   string
		want   bool
	}{
		"a text clause naming it":             {clause: "額 は正", name: "額", want: true},
		"a renaming onto it":                  {clause: "@[y := 額] y は正", name: "額", want: true},
		"a name the renaming binds otherwise": {clause: "@[額 := x] 額 は正", name: "額", want: false},
		"an equality naming it":               {clause: "@eq(額, x)", name: "額", want: true},
		"@opaque":                             {clause: "@opaque", name: "額", want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			parsed, err := ParseClause(tt.clause)
			if err != nil {
				t.Fatal(err)
			}

			// Execute, Assert
			if got := parsed.Mentions(tt.name); got != tt.want {
				t.Errorf("Mentions(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
