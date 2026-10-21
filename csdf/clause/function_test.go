package clause

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The cases of csdfbmc/tests/test_atoms.py, so that Go and the walk name an
// environment function alike.
func TestFunctionOfAnAtom(t *testing.T) {
	tests := map[string]struct {
		text     string
		wantName string
		wantArgs []Term
	}{
		"a $name hole and the space that ends it":    {text: "$在庫 が 十分", wantName: "$_ が 十分", wantArgs: []Term{pre("在庫")}},
		"two holes, each argument once, in order":    {text: "$a と $b と $a が 等しい", wantName: "$_ と $_ と $_ が 等しい", wantArgs: []Term{pre("a"), pre("b")}},
		"a hole before text that cannot continue it": {text: "$n>0", wantName: "$_>0", wantArgs: []Term{pre("n")}},
		"a ${name} hole is the same hole":            {text: "${在庫} が 十分", wantName: "$_ が 十分", wantArgs: []Term{pre("在庫")}},
		"$ before a digit is text":                   {text: "$100 を 超える $額", wantName: "$100 を 超える $_", wantArgs: []Term{pre("額")}},
		"an escaped $ is text":                       {text: `\$x と $y`, wantName: `\$x と $_`, wantArgs: []Term{pre("y")}},
		"a term hole before a name gets a space":     {text: "${m[k]}foo", wantName: "$_ foo", wantArgs: []Term{Index{Map: pre("m"), Key: pre("k")}}},
		"a primed hole":                              {text: "$x' = $x", wantName: "$_ = $_", wantArgs: []Term{post("x"), pre("x")}},
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
			want := Function{Name: tc.wantName, Args: tc.wantArgs}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
