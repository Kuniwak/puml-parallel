package clause

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseReadsAnAtomWithNameHoles(t *testing.T) {
	tests := map[string]struct {
		text string
		want And
	}{
		"a hole, text and a hole": {
			text: "$在庫' = $在庫 - $数量",
			want: and(atom(hole(post("在庫")), Text(" = "), hole(pre("在庫")), Text(" - "), hole(pre("数量")))),
		},
		"a space before a letter belongs to the hole": {
			text: "$在庫 が $数量 以上ある",
			want: and(atom(hole(pre("在庫")), Text("が "), hole(pre("数量")), Text("以上ある"))),
		},
		"a hole ends where its name ends": {
			text: "$x-1",
			want: and(atom(hole(pre("x")), Text("-1"))),
		},
		"$ before a digit is text": {
			text: "costs $100",
			want: and(atom(Text("costs $100"))),
		},
		"an escaped $ is text": {
			text: `\$x`,
			want: and(atom(Text("$x"))),
		},
		"a term hole indexes and selects a field": {
			text: "${locations[from].在庫} > 0",
			want: and(atom(hole(Field{Of: Index{Map: pre("locations"), Key: pre("from")}, Name: "在庫"}), Text(" > 0"))),
		},
		"a trailing prime primes the variable a path starts from": {
			text: "${locations[from].在庫'}",
			want: and(atom(hole(Field{Of: Index{Map: post("locations"), Key: pre("from")}, Name: "在庫"}))),
		},
		"a prime after the root name": {
			text: "${locations'[from]}",
			want: and(atom(hole(Index{Map: post("locations"), Key: pre("from")}))),
		},
		"a term hole holds an environment function": {
			text: "${f(x, y')}",
			want: and(atom(hole(App{Fn: "f", Args: []Term{pre("x"), post("y")}}))),
		},
		"a term hole names a variable starting with a digit": {
			text: "${1st}",
			want: and(atom(hole(pre("1st")))),
		},
		"conjuncts are separated by ∧": {
			text: "$x > 0 ∧ $y > 0",
			want: and(atom(hole(pre("x")), Text(" > 0")), atom(hole(pre("y")), Text(" > 0"))),
		},
		"an escaped ∧ is text": {
			text: `a \∧ b`,
			want: and(atom(Text("a ∧ b"))),
		},
		"a backslash before a reserved character is that character": {
			text: `\; \@ \\ \( \)`,
			want: and(atom(Text(`; @ \ ( )`))),
		},
		"a backslash before anything else is itself": {
			text: `a\b`,
			want: and(atom(Text(`a\b`))),
		},
		"the empty clause is true": {
			text: "",
			want: And{},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Parse(tt.text)

			// Assert
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.text, err)
			}
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported()); diff != "" {
				t.Errorf("Parse(%q) mismatch (-want +got):\n%s", tt.text, diff)
			}
		})
	}
}

func and(cs ...Formula) And { return And{Conjuncts: cs} }
func atom(ps ...Part) Atom  { return Atom{Parts: ps} }
func hole(t Term) Hole      { return Hole{Term: t} }
func pre(name string) Free  { return Free{Name: name} }
func post(name string) Free { return Free{Name: name, Primed: true} }

func TestParseRefuses(t *testing.T) {
	tests := map[string]string{
		"an unclosed term hole":  "${x",
		"an empty term hole":     "${}",
		"a path primed twice":    "${m'[k]'}",
		"a function primed":      "${f(x)'}",
		"an unclosed index":      "${m[k}",
		"a field without a name": "${m.}",
		"an empty conjunct":      "$x ∧  ∧ $y",
		"a trailing ∧":           "$x ∧ ",
	}
	for name, text := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Parse(text)

			// Assert
			if err == nil {
				t.Errorf("Parse(%q) = %#v, want an error", text, got)
			}
		})
	}
}
