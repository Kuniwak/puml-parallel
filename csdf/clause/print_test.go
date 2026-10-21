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
			f:    Exists{Hint: "v", Type: "T", Body: and(eq(Bound{0}, pre("x")))},
			want: "@exists(v : T) (@eq(v, x))",
		},
		"an ∃ takes a fresh name when its hint would capture": {
			f:    Exists{Hint: "x", Type: "T", Body: and(eq(Bound{0}, pre("x")))},
			want: "@exists(x2 : T) (@eq(x2, x))",
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
		"parentheses in an ∃ body":                                Exists{Hint: "v", Type: "T", Body: and(atom(Text(") f("), hole(Bound{0}), Text("(")))},
		"a backslash at the end of an ∃ body":                     Exists{Hint: "v", Type: "T", Body: and(atom(hole(Bound{0}), Text(`\`)))},
		"nested binders with one hint":                            Exists{Hint: "v", Type: "T", Body: and(Exists{Hint: "v", Type: "T", Body: and(eq(Bound{0}, Bound{1}))})},
		"a binder whose hint is a free name inside":               Exists{Hint: "x", Type: "T", Body: and(Exists{Hint: "x2", Type: "T", Body: and(eq(Bound{1}, pre("x")), eq(Bound{0}, pre("x2")))})},
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
	// Setup: nested ∧, an empty text and adjacent texts, which the normal
	// form flattens, drops and merges.
	f := and(and(atom(Text("a"), Text(""), Text("b"))), atom(hole(pre("x"))))
	want := "ab ∧ $x"

	// Execute
	got, err := Print(f)

	// Assert
	if err != nil || got != want {
		t.Errorf("Print(f) = %q, %v, want %q", got, err, want)
	}
}

func TestPrintRefusesAFormulaThatIsNotWellFormed(t *testing.T) {
	tests := map[string]struct {
		f    Formula
		want string
	}{
		"a dangling index":             {f: and(eq(Bound{0}, pre("x"))), want: "clause.Print: clause.LocallyClosed: the index 0 is under 0 binders"},
		"a name that is no identifier": {f: and(atom(hole(pre("a b")))), want: `clause.Print: clause.WellFormed: the variable "a b" is no identifier`},
		"a hint that is no name":       {f: Exists{Type: "T", Body: and(eq(Bound{0}, pre("x")))}, want: `clause.Print: clause.WellFormed: the hint "" of an ∃ is no identifier`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := Print(tt.f)

			// Assert
			assertErrorContains(t, err, tt.want)
		})
	}
}

func TestPrintOfParseIsTheTextAuthorsWrite(t *testing.T) {
	for _, text := range []string{
		"$在庫' = $在庫 - $数量",
		"$在庫 が $数量 以上ある",
		"${locations[from].在庫} > 0 ∧ @ne(from, to)",
		"costs $100",
		"@exists(v : T) (f($v) > 0) ∧ $y",
		"@update(m', m, k, v) ∧ @frame(m', m, k)",
	} {
		t.Run(text, func(t *testing.T) {
			// Setup
			f, err := Parse(text)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			got, err := Print(f)

			// Assert
			if err != nil || got != text {
				t.Errorf("Print(Parse(%q)) = %q, %v", text, got, err)
			}
		})
	}
}
