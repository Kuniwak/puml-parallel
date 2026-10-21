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
