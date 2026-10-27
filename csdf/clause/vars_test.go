package clause

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAtomArgsAreItsHolesInOrder(t *testing.T) {
	// Setup
	a := atom(hole(post("x")), Text(" = "), hole(pre("x")), Text(" + "), hole(pre("x")))

	// Execute
	got := a.Args()

	// Assert
	want := []Term{post("x"), pre("x"), pre("x")}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Args() mismatch (-want +got):\n%s", diff)
	}
}

func TestFreeVars(t *testing.T) {
	tests := map[string]struct {
		text string
		want []Free
	}{
		"in order of appearance, once each": {
			text: "$y > $x ∧ @eq(x, y')",
			want: []Free{pre("y"), pre("x"), post("y")},
		},
		"inside paths and functions": {
			text: "${m[k].f} = ${g(a)}",
			want: []Free{pre("m"), pre("k"), pre("a")},
		},
		"not the bound ones": {
			text: "@exists(v : T) (@eq(v, w))",
			want: []Free{pre("w")},
		},
		"none in true": {
			text: "",
			want: nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			f, err := Parse(tt.text)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			got := FreeVars(f)

			// Assert
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("FreeVars(%q) mismatch (-want +got):\n%s", tt.text, diff)
			}
		})
	}
}

func TestEachTermVisitsTheTermsOfEveryConjunct(t *testing.T) {
	// Setup
	f, err := Parse("${m[k].f} = $x ∧ @eq(y, z) ∧ @exists(v : T) (@at(v[k], s))")
	if err != nil {
		t.Fatal(err)
	}
	var got []Term

	// Execute
	EachTerm(f, func(t Term) { got = append(got, t) })

	// Assert
	want := []Term{
		Field{Of: Index{Map: pre("m"), Key: pre("k")}, Name: "f"}, pre("x"),
		pre("y"), pre("z"),
		Index{Map: Bound{Index: 0}, Key: pre("k")},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("EachTerm mismatch (-want +got):\n%s", diff)
	}
}

func TestPrimedNamesAreTheVariablesNamedAfterTheStep(t *testing.T) {
	// Setup
	f := And{Conjuncts: []Formula{
		Builtin{Op: OpEq, Args: []Term{Free{Name: "n", Primed: true}, Free{Name: "n"}}},
		Builtin{Op: OpEq, Args: []Term{App{Fn: "f", Args: []Term{Free{Name: "m", Primed: true}}}, Free{Name: "o"}}},
		Exists{Type: "T", Body: And{Conjuncts: []Formula{
			Builtin{Op: OpEq, Args: []Term{Free{Name: "n", Primed: true}, Bound{Index: 0}}},
		}}},
	}}

	// Execute
	got := PrimedNames(f)

	// Assert
	if diff := cmp.Diff([]string{"n", "m"}, got); diff != "" {
		t.Error(diff)
	}
}
