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
		"all":                {text: "@all(locations', 未開設)", want: Builtin{Op: OpAll, Args: []Term{Free{Name: "locations", Primed: true}}, State: "未開設"}},
		"notin":              {text: "@notin(k, m)", want: Builtin{Op: OpNotIn, Args: []Term{pre("k"), pre("m")}}},
		"empty":              {text: "@empty(m')", want: Builtin{Op: OpEmpty, Args: []Term{post("m")}}},
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
		"a bound variable is an index, of the type it is given": {
			text: "@exists(v : T) ($v > $x)",
			want: Exists{Hint: "v", Type: "T", Body: and(atom(hole(Bound{Index: 0}), Text(" > "), hole(pre("x"))))},
		},
		"the innermost binder wins": {
			text: "@exists(v : T) (@exists(v : T) (@eq(v, w)))",
			want: Exists{Hint: "v", Type: "T", Body: and(Exists{Hint: "v", Type: "T", Body: and(Builtin{Op: OpEq, Args: []Term{Bound{Index: 0}, pre("w")}})})},
		},
		"an outer binder is one index further": {
			text: "@exists(m : T) (@exists(k : T) (@in(k, m)))",
			want: Exists{Hint: "m", Type: "T", Body: and(Exists{Hint: "k", Type: "T", Body: and(Builtin{Op: OpIn, Args: []Term{Bound{Index: 0}, Bound{Index: 1}}})})},
		},
		"parentheses in a body's text nest": {
			text: "@exists(v : T) (f($v) > 0) ∧ $y",
			want: Exists{Hint: "v", Type: "T", Body: and(atom(Text("f("), hole(Bound{Index: 0}), Text(") > 0")))},
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
	text := "$x > 0 ∧ @exists(v : T) (@eq(v, x))"

	// Execute
	_, err := ParseAuthored(text)

	// Assert
	assertErrorContains(t, err, "@exists is written by tools only")
}

func TestParseRefuses(t *testing.T) {
	tests := map[string]struct {
		text string
		want string
	}{
		"an unclosed term hole":          {text: "${x", want: `expected "}" at 3`},
		"an empty term hole":             {text: "${}", want: "expected a term at 2"},
		"a path primed twice":            {text: "${m'[k]'}", want: "the term at 2 is primed twice"},
		"a function primed":              {text: "${f(x)'}", want: "an environment function at 6 has no value after the step"},
		"an unclosed index":              {text: "${m[k}", want: `expected "]" at 5`},
		"a field without a name":         {text: "${m.}", want: "expected a field at 4"},
		"an empty conjunct":              {text: "$x ∧  ∧ $y", want: "empty conjunct at 5"},
		"a trailing ∧":                   {text: "$x ∧ ", want: "empty conjunct at 5"},
		"@at without a state":            {text: "@at(m)", want: "@at at 0 takes a state"},
		"@at with a term for a state":    {text: "@at(m, s[k])", want: `@at at 0: expected ")" at 8`},
		"@eq with one term":              {text: "@eq(x)", want: "@eq at 0 takes 2 terms"},
		"@update with three terms":       {text: "@update(m', m, k)", want: "@update at 0 takes 4 terms"},
		"@frame with one map":            {text: "@frame(m')", want: "@frame at 0 takes 2 terms"},
		"an unknown tool word":           {text: "@foo(x)", want: "unknown tool word @foo at 0"},
		"a tool word with text after it": {text: "@eq(x, y) z", want: `@eq at 0: expected " ∧ " or the end at 9`},
		"a bound variable primed":        {text: "@exists(v : T) ($v' > 0)", want: "v is bound by @exists and has no value after the step"},
		"an unclosed body":               {text: "@exists(v : T) ($v > 0", want: `expected ")"`},
		"a binder without its type":      {text: "@exists(v) ($v > 0)", want: `expected " : "`},
		"an empty body":                  {text: "@exists(v : T) ()", want: "@exists at 0: an empty body"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := Parse(tt.text)

			// Assert
			assertErrorContains(t, err, tt.want)
		})
	}
}
