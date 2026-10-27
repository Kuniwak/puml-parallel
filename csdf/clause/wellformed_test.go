package clause

import "testing"

func TestWellFormed(t *testing.T) {
	tests := map[string]struct {
		f    Formula
		want string // a part of the error, or "" for a well-formed formula
	}{
		"a parsed clause":              {f: and(atom(hole(pre("x")), Text(" > 0")), Exists{Hint: "v", Type: "T", Body: and(eq(Bound{0}, pre("x")))}), want: ""},
		"true":                         {f: And{}, want: ""},
		"a name that is no identifier": {f: and(atom(hole(pre("a b")))), want: "the variable: \"a b\" is not an identifier"},
		"an empty name":                {f: and(atom(hole(pre("")))), want: "the variable: \"\" is not an identifier"},
		"a field with no name":         {f: and(atom(hole(Field{Of: pre("r")}))), want: "the field: \"\" is not an identifier"},
		"a function with no name":      {f: and(atom(hole(App{}))), want: "the function: \"\" is not an identifier"},
		"an empty atom":                {f: and(atom()), want: "an empty atom"},
		"empty text":                   {f: and(atom(Text(""), hole(pre("x")))), want: "empty text in an atom"},
		"a hint that is no name":       {f: Exists{Hint: "", Type: "T", Body: and(eq(Bound{0}, pre("x")))}, want: "the hint of an ∃: \"\" is not an identifier"},
		"an ∃ without a type":          {f: Exists{Hint: "v", Body: and(eq(Bound{0}, pre("x")))}, want: "the type \"\" of the ∃ v is no identifier"},
		"an empty ∃ body":              {f: Exists{Hint: "v", Type: "T"}, want: "the ∃ v has an empty body"},
		"a dangling index":             {f: and(eq(Bound{0}, pre("x"))), want: "clause.LocallyClosed: the index 0 is under 0 binders"},
		"a builtin of the wrong arity": {f: and(Builtin{Op: OpEq, Args: []Term{pre("x")}}), want: "@eq takes 2 terms, not 1"},
		"an unknown builtin":           {f: and(Builtin{Op: "foo", Args: []Term{pre("x")}}), want: "an unknown builtin @foo"},
		"@at without a state":          {f: and(Builtin{Op: OpAt, Args: []Term{pre("x")}}), want: "@at: a state"},
		"a state on another builtin":   {f: and(Builtin{Op: OpEq, Args: []Term{pre("x"), pre("y")}, State: "s"}), want: "@eq: a state"},
		"a path from a function":       {f: and(atom(hole(Index{Map: App{Fn: "f"}, Key: pre("k")}))), want: "a path starts from a variable"},
		"a nil term":                   {f: and(atom(Hole{})), want: "an unknown term"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			err := WellFormed(tt.f)

			// Assert
			if tt.want == "" {
				if err != nil {
					t.Errorf("WellFormed() = %v, want nil", err)
				}
				return
			}
			assertErrorContains(t, err, tt.want)
		})
	}
}
