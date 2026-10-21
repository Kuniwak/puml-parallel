package clause

import "testing"

func TestPrintWritesTheSurfaceSyntax(t *testing.T) {
	tests := map[string]struct {
		f    Formula
		want string
	}{
		"true": {
			f:    And{},
			want: "",
		},
		"name holes and text": {
			f:    and(atom(hole(post("在庫")), Text(" = "), hole(pre("在庫")), Text(" - "), hole(pre("数量")))),
			want: "$在庫' = $在庫 - $数量",
		},
		"a space between a hole and a letter": {
			f:    and(atom(hole(pre("在庫")), Text("が"))),
			want: "$在庫 が",
		},
		"a term hole": {
			f:    and(atom(hole(Field{Of: Index{Map: post("m"), Key: pre("k")}, Name: "f"}))),
			want: "${m'[k].f}",
		},
		"conjuncts and builtins": {
			f:    and(eq(pre("x"), post("y")), Builtin{Op: OpAt, Args: []Term{pre("i")}, State: "s"}, Builtin{Op: OpFrame, Args: []Term{post("m"), pre("m"), pre("k")}}),
			want: "@eq(x, y') ∧ @at(i, s) ∧ @frame(m', m, k)",
		},
		"an ∃ keeps its hint when it captures nothing": {
			f:    Exists{Hint: "v", Body: and(eq(Bound{0}, pre("x")))},
			want: "@exists(v) (@eq(v, x))",
		},
		"an ∃ takes a fresh name when its hint would capture": {
			f:    Exists{Hint: "x", Body: and(eq(Bound{0}, pre("x")))},
			want: "@exists(x2) (@eq(x2, x))",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Print(tt.f)

			// Assert
			if err != nil {
				t.Fatalf("Print() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Print() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseOfPrintIsTheFormula(t *testing.T) {
	tests := map[string]Formula{
		"text after a hole that starts with a space and a letter": and(atom(hole(pre("x")), Text(" が"))),
		"text after a hole that starts with a letter":             and(atom(hole(pre("x")), Text("が"))),
		"a prime in text after an unprimed hole":                  and(atom(hole(pre("x")), Text("'s"))),
		"a prime in text after a primed hole":                     and(atom(hole(post("x")), Text("'s"))),
		"a dollar before a digit":                                 and(atom(Text("costs $100"))),
		"a dollar before a letter":                                and(atom(Text("$x is not a hole"))),
		"a dollar before a brace":                                 and(atom(Text("${x}"))),
		"a dollar at the end of text before a hole":               and(atom(Text("$"), hole(pre("x")))),
		"text that starts with @":                                 and(atom(Text("@eq(x, y)"))),
		"@ later in text":                                         and(atom(Text("a @ b"))),
		"a backslash before a hole":                               and(atom(Text(`a\`), hole(pre("x")))),
		"a backslash before a reserved character":                 and(atom(Text(`\;`))),
		"∧ and ; in text":                                         and(atom(Text("a ∧ b; c"))),
		"a name that starts with a digit":                         and(atom(hole(pre("1st")), Text(" > 0"))),
		"parentheses in an ∃ body":                                Exists{Hint: "v", Body: and(atom(Text(") f("), hole(Bound{0}), Text("(")))},
		"a backslash at the end of an ∃ body":                     Exists{Hint: "v", Body: and(atom(hole(Bound{0}), Text(`\`)))},
		"nested binders with one hint":                            Exists{Hint: "v", Body: and(Exists{Hint: "v", Body: and(eq(Bound{0}, Bound{1}))})},
		"a binder whose hint is a free name inside":               Exists{Hint: "x", Body: and(Exists{Hint: "x2", Body: and(eq(Bound{1}, pre("x")), eq(Bound{0}, pre("x2")))})},
		"an environment function":                                 and(atom(hole(App{Fn: "f", Args: []Term{pre("x"), post("y")}}), Text(" > 0"))),
		"a hole after a hole":                                     and(atom(hole(pre("x")), hole(pre("y")))),
	}
	for name, f := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			text, err := Print(f)
			if err != nil {
				t.Fatalf("Print() error = %v", err)
			}

			// Execute
			got, err := Parse(text)

			// Assert
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", text, err)
			}
			if !AlphaEqual(got, f) {
				t.Errorf("Parse(Print(f)) = %#v, want %#v (printed %q)", got, f, text)
			}
		})
	}
}

func TestPrintIsPrintOfTheNormalForm(t *testing.T) {
	// Setup
	f := and(and(atom(Text("a"), Text(""), Text("b"))), atom(hole(pre("x"))))

	// Execute
	got, err := Print(f)

	// Assert
	want, werr := Print(Normalize(f))
	if err != nil || werr != nil || got != want {
		t.Errorf("Print(f) = %q, %v; Print(Normalize(f)) = %q, %v", got, err, want, werr)
	}
}

func TestPrintRefusesAFormulaThatIsNotWellFormed(t *testing.T) {
	tests := map[string]Formula{
		"a dangling index":             and(eq(Bound{0}, pre("x"))),
		"a name that is no identifier": and(atom(hole(pre("a b")))),
		"a hint that is no name":       Exists{Body: and(eq(Bound{0}, pre("x")))},
	}
	for name, f := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Print(f)

			// Assert
			if err == nil {
				t.Errorf("Print() = %q, want an error", got)
			}
		})
	}
}
