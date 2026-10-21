package clause

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// functionNames are the cases of testdata/function_names.json, which the
// Python tests read too (csdfbmc/tests/test_atoms.py), so that Go and the walk
// name an environment function alike.
type functionNames struct {
	Named []struct {
		Case string   `json:"case"`
		Text string   `json:"text"`
		Name string   `json:"name"`
		Args []string `json:"args"`
	} `json:"named"`
}

func TestFunctionOfAnAtomAsTheSharedCasesSay(t *testing.T) {
	// Setup
	data, err := os.ReadFile("testdata/function_names.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases functionNames
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases.Named {
		t.Run(tc.Case, func(t *testing.T) {
			f, err := Parse(tc.Text)
			if err != nil {
				t.Fatal(err)
			}
			a := firstAtom(f)
			want := Function{Name: tc.Name, Args: []Term{}}
			for _, n := range tc.Args {
				want.Args = append(want.Args, pre(n))
			}

			// Execute
			got, err := FunctionOf(a)

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Holes csdfbmc does not read yet (a path, a value after the step), which the
// shared cases therefore refuse, but which an obligation states.
func TestFunctionOfAnAtomWithHolesOnlyGoReads(t *testing.T) {
	tests := map[string]struct {
		text string
		want Function
	}{
		"a term hole before a name gets a space": {text: "${m[k]}foo", want: Function{Name: "$_0 foo", Args: []Term{Index{Map: pre("m"), Key: pre("k")}}}},
		"a primed hole":                          {text: "$x' = $x", want: Function{Name: "$_0 = $_1", Args: []Term{post("x"), pre("x")}}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			f, err := Parse(tc.text)
			if err != nil {
				t.Fatal(err)
			}
			a := firstAtom(f)

			// Execute
			got, err := FunctionOf(a)

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
