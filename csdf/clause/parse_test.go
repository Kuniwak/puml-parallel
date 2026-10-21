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
		"a term hole indexes and selects a field": {
			text: "${locations[from].在庫} > 0",
			want: and(atom(hole(Field{Of: Index{Map: pre("locations"), Key: pre("from")}, Name: "在庫"}), Text(" > 0"))),
		},
		"a trailing prime primes the variable a path starts from": {
			text: "${locations[from].在庫'}",
			want: and(atom(hole(Field{Of: Index{Map: post("locations"), Key: pre("from")}, Name: "在庫"}))),
		},
		"a prime after the root name": {
			text: "${locations'[from]}",
			want: and(atom(hole(Index{Map: post("locations"), Key: pre("from")}))),
		},
		"a term hole holds an environment function": {
			text: "${f(x, y')}",
			want: and(atom(hole(App{Fn: "f", Args: []Term{pre("x"), post("y")}}))),
		},
		"a term hole names a variable starting with a digit": {
			text: "${1st}",
			want: and(atom(hole(pre("1st")))),
		},
		"conjuncts are separated by ∧": {
			text: "$x > 0 ∧ $y > 0",
			want: and(atom(hole(pre("x")), Text(" > 0")), atom(hole(pre("y")), Text(" > 0"))),
		},
		"an escaped ∧ is text": {
			text: `a \∧ b`,
			want: and(atom(Text("a ∧ b"))),
		},
		"a backslash before a reserved character is that character": {
			text: `\; \@ \\ \( \)`,
			want: and(atom(Text(`; @ \ ( )`))),
		},
		"a backslash before anything else is itself": {
			text: `a\b`,
			want: and(atom(Text(`a\b`))),
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

func TestParseReadsBuiltins(t *testing.T) {
	tests := map[string]struct {
		text string
		want Formula
	}{
		"eq":                 {text: "@eq(x, y')", want: Builtin{Op: OpEq, Args: []Term{pre("x"), post("y")}}},
		"ne":                 {text: "@ne(from, to)", want: Builtin{Op: OpNe, Args: []Term{pre("from"), pre("to")}}},
		"in":                 {text: "@in(k, m)", want: Builtin{Op: OpIn, Args: []Term{pre("k"), pre("m")}}},
		"update":             {text: "@update(m', m, k, v)", want: Builtin{Op: OpUpdate, Args: []Term{post("m"), pre("m"), pre("k"), pre("v")}}},
		"frame":              {text: "@frame(m', m, a, b)", want: Builtin{Op: OpFrame, Args: []Term{post("m"), pre("m"), pre("a"), pre("b")}}},
		"frame without keys": {text: "@frame(m', m)", want: Builtin{Op: OpFrame, Args: []Term{post("m"), pre("m")}}},
		"at":                 {text: "@at(locations[from], 倉庫待ち)", want: Builtin{Op: OpAt, Args: []Term{Index{Map: pre("locations"), Key: pre("from")}}, State: "倉庫待ち"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Parse(tt.text)

			// Assert
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.text, err)
			}
			if diff := cmp.Diff(and(tt.want), got); diff != "" {
				t.Errorf("Parse(%q) mismatch (-want +got):\n%s", tt.text, diff)
			}
		})
	}
}

func TestParseReadsExists(t *testing.T) {
	tests := map[string]struct {
		text string
		want Formula
	}{
		"a bound variable is an index": {
			text: "@exists(v) ($v > $x)",
			want: Exists{Hint: "v", Body: and(atom(hole(Bound{Index: 0}), Text(" > "), hole(pre("x"))))},
		},
		"the innermost binder wins": {
			text: "@exists(v) (@exists(v) (@eq(v, w)))",
			want: Exists{Hint: "v", Body: and(Exists{Hint: "v", Body: and(Builtin{Op: OpEq, Args: []Term{Bound{Index: 0}, pre("w")}})})},
		},
		"an outer binder is one index further": {
			text: "@exists(m) (@exists(k) (@in(k, m)))",
			want: Exists{Hint: "m", Body: and(Exists{Hint: "k", Body: and(Builtin{Op: OpIn, Args: []Term{Bound{Index: 0}, Bound{Index: 1}}})})},
		},
		"parentheses in a body's text nest": {
			text: "@exists(v) (f($v) > 0) ∧ $y",
			want: Exists{Hint: "v", Body: and(atom(Text("f("), hole(Bound{Index: 0}), Text(") > 0")))},
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
			if diff := cmp.Diff(tt.want, got.Conjuncts[0]); diff != "" {
				t.Errorf("Parse(%q) mismatch (-want +got):\n%s", tt.text, diff)
			}
		})
	}
}

func TestParseAuthoredRefusesExists(t *testing.T) {
	// Setup
	text := "$x > 0 ∧ @exists(v) (@eq(v, x))"

	// Execute
	got, err := ParseAuthored(text)

	// Assert
	if err == nil {
		t.Errorf("ParseAuthored(%q) = %#v, want an error", text, got)
	}
}

func TestParseRefuses(t *testing.T) {
	tests := map[string]string{
		"an unclosed term hole":          "${x",
		"an empty term hole":             "${}",
		"a path primed twice":            "${m'[k]'}",
		"a function primed":              "${f(x)'}",
		"an unclosed index":              "${m[k}",
		"a field without a name":         "${m.}",
		"an empty conjunct":              "$x ∧  ∧ $y",
		"a trailing ∧":                   "$x ∧ ",
		"@at without a state":            "@at(m)",
		"@at with a term for a state":    "@at(m, s[k])",
		"@eq with one term":              "@eq(x)",
		"@update with three terms":       "@update(m', m, k)",
		"@frame with one map":            "@frame(m')",
		"an unknown tool word":           "@foo(x)",
		"a tool word with text after it": "@eq(x, y) z",
		"a bound variable primed":        "@exists(v) ($v' > 0)",
		"an unclosed body":               "@exists(v) ($v > 0",
		"an empty body":                  "@exists(v) ()",
	}
	for name, text := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Parse(text)

			// Assert
			if err == nil {
				t.Errorf("Parse(%q) = %#v, want an error", text, got)
			}
		})
	}
}
