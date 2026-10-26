package clause

import "testing"

func TestContradictory(t *testing.T) {
	tests := map[string]struct {
		text string
		want bool
	}{
		"@ne of a term with itself (lower boundary value)":                     {text: "@ne(n, n)", want: true},
		"@ne of two different terms (upper boundary value)":                    {text: "@ne(n, m)", want: false},
		"@eq beside @ne of the same terms, swapped (representative value)":     {text: "@eq(n, m) ∧ @ne(m, n)", want: true},
		"@in beside @notin of the same key and map (representative value)":     {text: "@in(k, m) ∧ @notin(k, m)", want: true},
		"@empty beside @in of the same map (representative value)":             {text: "@empty(m) ∧ @in(k, m)", want: true},
		"an atom is never evaluated (representative value)":                    {text: "$n > 0 ∧ $n < 0", want: false},
		"a map before the step is not the map after it (representative value)": {text: "@in(k, m') ∧ @notin(k, m)", want: false},
		"facts on the map after the step (representative value)":               {text: "@in(k, m') ∧ @notin(k, m')", want: true},
		"@ne of one bound variable with itself (representative value)":         {text: "@exists(x : T) (@ne(x, x))", want: true},
		"@ne of two nested bound variables (representative value)":             {text: "@exists(x : T) (@exists(y : T) (@ne(x, y)))", want: false},
		"an inner binder shadowing an outer one of the same name (representative value)": {
			text: "@exists(x : T) (@exists(x : T) (@ne(x, x)))", want: true,
		},
		"an outer fact beside an inner one is not read together (representative value)": {
			text: "@in(k, m) ∧ @exists(v : T) (@notin(k, m))", want: false,
		},
		"facts in the body of an ∃ under an outer fact (representative value)": {
			text: "@in(k, m) ∧ @exists(v : T) (@in(v, m) ∧ @notin(v, m))", want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f, err := Parse(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			if got := Contradictory(f); got != tt.want {
				t.Errorf("Contradictory(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}
