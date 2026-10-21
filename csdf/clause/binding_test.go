package clause

import "testing"

func eq(a, b Term) Builtin { return Builtin{Op: OpEq, Args: []Term{a, b}} }

func TestClose(t *testing.T) {
	tests := map[string]struct {
		f    Formula
		v    Free
		want Exists
	}{
		"the variable becomes the nearest index": {
			f:    and(eq(pre("v"), pre("x"))),
			v:    pre("v"),
			want: Exists{Hint: "v", Body: and(eq(Bound{0}, pre("x")))},
		},
		"under a nested binder it is one index further": {
			f:    and(Exists{Hint: "k", Body: and(Builtin{Op: OpIn, Args: []Term{Bound{0}, pre("m")}})}),
			v:    pre("m"),
			want: Exists{Hint: "m", Body: and(Exists{Hint: "k", Body: and(Builtin{Op: OpIn, Args: []Term{Bound{0}, Bound{1}}})})},
		},
		"the value after the step stays free": {
			f:    and(eq(post("v"), pre("v"))),
			v:    pre("v"),
			want: Exists{Hint: "v", Body: and(eq(post("v"), Bound{0}))},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Close(tt.f, tt.v)

			// Assert
			if err != nil {
				t.Fatalf("Close() error = %v", err)
			}
			if !AlphaEqual(got, tt.want) {
				t.Errorf("Close() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestOpen(t *testing.T) {
	tests := map[string]struct {
		e    Exists
		t    Term
		want And
	}{
		"the nearest index becomes the term": {
			e:    Exists{Body: and(eq(Bound{0}, pre("x")))},
			t:    Field{Of: pre("r"), Name: "f"},
			want: and(eq(Field{Of: pre("r"), Name: "f"}, pre("x"))),
		},
		"under a nested binder, the index one further": {
			e:    Exists{Body: and(Exists{Body: and(eq(Bound{0}, Bound{1}))})},
			t:    pre("m"),
			want: and(Exists{Body: and(eq(Bound{0}, pre("m")))}),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Open(tt.e, tt.t)

			// Assert
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if !AlphaEqual(got, tt.want) {
				t.Errorf("Open() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCloseThenOpenGivesBackTheFormula(t *testing.T) {
	// Setup
	f := and(eq(pre("v"), pre("x")), Exists{Body: and(eq(Bound{0}, pre("v")))})
	e, err := Close(f, pre("v"))
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	got, err := Open(e, pre("v"))

	// Assert
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if !AlphaEqual(got, f) {
		t.Errorf("Open(Close(f)) = %#v, want %#v", got, f)
	}
}

func TestSubstitute(t *testing.T) {
	tests := map[string]struct {
		f    Formula
		v    Free
		t    Term
		want Formula
	}{
		"never captures": {
			f:    Exists{Hint: "v", Body: and(eq(Bound{0}, pre("x")))},
			v:    pre("x"),
			t:    pre("v"),
			want: Exists{Hint: "v", Body: and(eq(Bound{0}, pre("v")))},
		},
		"touches only the given priming": {
			f:    and(eq(post("x"), pre("x"))),
			v:    pre("x"),
			t:    Index{Map: pre("m"), Key: pre("k")},
			want: and(eq(post("x"), Index{Map: pre("m"), Key: pre("k")})),
		},
		"leaves other variables as they were": {
			f:    and(atom(hole(App{Fn: "f"}), Text(" = "), hole(pre("y")))),
			v:    pre("x"),
			t:    pre("z"),
			want: and(atom(hole(App{Fn: "f"}), Text(" = "), hole(pre("y")))),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Substitute(tt.f, tt.v, tt.t)

			// Assert
			if err != nil {
				t.Fatalf("Substitute() error = %v", err)
			}
			if !AlphaEqual(got, tt.want) {
				t.Errorf("Substitute() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestBindingRefusesWhatBreaksItsPreconditions(t *testing.T) {
	dangling := and(eq(Bound{0}, pre("x")))
	tests := map[string]func() error{
		"Close of a formula with a dangling index": func() error { _, err := Close(dangling, pre("v")); return err },
		"Close of a primed variable":               func() error { _, err := Close(and(eq(post("v"), pre("x"))), post("v")); return err },
		"Open of an ∃ with a dangling index":       func() error { _, err := Open(Exists{Body: and(eq(Bound{1}, pre("x")))}, pre("y")); return err },
		"Open with a term that has an index":       func() error { _, err := Open(Exists{Body: and(eq(Bound{0}, pre("x")))}, Bound{0}); return err },
		"Substitute into a dangling index":         func() error { _, err := Substitute(dangling, pre("x"), pre("y")); return err },
		"Substitute a term that has an index":      func() error { _, err := Substitute(and(eq(pre("x"), pre("y"))), pre("x"), Bound{0}); return err },
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			err := call()

			// Assert
			if err == nil {
				t.Error("error = nil, want one")
			}
		})
	}
}

func TestLocallyClosed(t *testing.T) {
	tests := map[string]struct {
		f    Formula
		want bool
	}{
		"free variables only":  {f: and(eq(pre("x"), pre("y"))), want: true},
		"an index its ∃ binds": {f: Exists{Body: and(eq(Bound{0}, pre("y")))}, want: true},
		"a dangling index":     {f: and(eq(Bound{0}, pre("y"))), want: false},
		"an index one too far": {f: Exists{Body: and(eq(Bound{1}, pre("y")))}, want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			err := LocallyClosed(tt.f)

			// Assert
			if (err == nil) != tt.want {
				t.Errorf("LocallyClosed() = %v, want closed = %v", err, tt.want)
			}
		})
	}
}
