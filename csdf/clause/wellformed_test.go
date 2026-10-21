package clause

import "testing"

func TestWellFormed(t *testing.T) {
	tests := map[string]struct {
		f    Formula
		want bool
	}{
		"a parsed clause":              {f: and(atom(hole(pre("x")), Text(" > 0")), Exists{Hint: "v", Body: and(eq(Bound{0}, pre("x")))}), want: true},
		"true":                         {f: And{}, want: true},
		"a name that is no identifier": {f: and(atom(hole(pre("a b")))), want: false},
		"an empty name":                {f: and(atom(hole(pre("")))), want: false},
		"a field with no name":         {f: and(atom(hole(Field{Of: pre("r")}))), want: false},
		"a function with no name":      {f: and(atom(hole(App{}))), want: false},
		"an empty atom":                {f: and(atom()), want: false},
		"empty text":                   {f: and(atom(Text(""), hole(pre("x")))), want: false},
		"a hint that is no name":       {f: Exists{Hint: "", Body: and(eq(Bound{0}, pre("x")))}, want: false},
		"an empty ∃ body":              {f: Exists{Hint: "v"}, want: false},
		"a dangling index":             {f: and(eq(Bound{0}, pre("x"))), want: false},
		"a builtin of the wrong arity": {f: and(Builtin{Op: OpEq, Args: []Term{pre("x")}}), want: false},
		"an unknown builtin":           {f: and(Builtin{Op: "foo", Args: []Term{pre("x")}}), want: false},
		"@at without a state":          {f: and(Builtin{Op: OpAt, Args: []Term{pre("x")}}), want: false},
		"a state on another builtin":   {f: and(Builtin{Op: OpEq, Args: []Term{pre("x"), pre("y")}, State: "s"}), want: false},
		"a path from a function":       {f: and(atom(hole(Index{Map: App{Fn: "f"}, Key: pre("k")}))), want: false},
		"a nil term":                   {f: and(atom(Hole{})), want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			err := WellFormed(tt.f)

			// Assert
			if (err == nil) != tt.want {
				t.Errorf("WellFormed() = %v, want well-formed = %v", err, tt.want)
			}
		})
	}
}
