package core

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/lts"
	"github.com/Kuniwak/puml-parallel/csdf/types"
)

func mustParse(t *testing.T, text string) clause.And {
	t.Helper()
	f, err := clause.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// counter counts x from 0 up to 1 on inc!x and back to 0 on reset.
func counter(t *testing.T) (*Diagram, Env) {
	d := &Diagram{
		States:   []string{"s"},
		Vars:     map[string][]string{"s": {"x"}},
		Types:    map[string]types.Type{"x": types.Named("N")},
		Channels: map[string][]types.Type{"inc": {types.Named("N")}},
		Init:     "s",
		InitPost: mustParse(t, "$x' = 0"),
		Edges: []Edge{
			{Src: "s", Dst: "s", Event: Event{Channel: "inc", Fields: []Field{{Send: true, Name: "x"}}},
				Guard: mustParse(t, "$x < 1"), Post: mustParse(t, "$x' = $x + 1")},
			{Src: "s", Dst: "s", Event: Event{Channel: "reset"},
				Post: mustParse(t, "$x' = 0")},
		},
	}
	env := Env{
		Types: map[string][]Value{"N": {0, 1}},
		Atoms: map[string]func([]Value) bool{
			"#0 = 0":      func(a []Value) bool { return a[0] == 0 },
			"#0 < 1":      func(a []Value) bool { return a[0].(int) < 1 },
			"#0 = #1 + 1": func(a []Value) bool { return a[0] == a[1].(int)+1 },
		},
	}
	return d, env
}

func TestExpandWalksTheValues(t *testing.T) {
	// Setup
	d, env := counter(t)
	want := lts.New(0)
	want.Add(0, "inc.0", 1)
	want.Add(0, "reset", 0)
	want.Add(1, "reset", 0)

	// Execute
	got, err := Expand(d, env)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !lts.StableFailuresEqual(got, want) {
		t.Errorf("not stable-failures equivalent: %#v", got.Trans)
	}
}

func TestExpandRefusesANameWithoutAType(t *testing.T) {
	cases := map[string]struct {
		d    *Diagram
		want string
	}{
		"a state variable": {d: &Diagram{
			States: []string{"s"}, Vars: map[string][]string{"s": {"y"}}, Init: "s", InitPost: mustParse(t, "@eq(y', y')"),
		}, want: "the variable y of s has no type"},
		"a state variable of a type the environment does not give": {d: &Diagram{
			States: []string{"s"}, Vars: map[string][]string{"s": {"y"}}, Types: map[string]types.Type{"y": types.Named("Y")}, Init: "s", InitPost: mustParse(t, "@eq(y', y')"),
		}, want: "no type Y"},
		"a received field": {d: &Diagram{
			States: []string{"s"}, Init: "s",
			Edges: []Edge{{Src: "s", Dst: "s", Event: Event{Channel: "put", Fields: []Field{{Name: "y"}}}}},
		}, want: "put receives y, but declares no type of it"},
		"a variable bound by ∃": {d: &Diagram{
			States: []string{"s"}, Init: "s",
			Edges: []Edge{{Src: "s", Dst: "s", Event: Event{Channel: "go"}, Guard: mustParse(t, "@exists(v : Y) (@eq(v, v))")}},
		}, want: "no type Y"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			env := Env{Types: map[string][]Value{"x": {0}}}

			// Execute
			_, err := Expand(c.d, env)

			// Assert
			assertErrorContains(t, err, c.want)
		})
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want one containing %q", err, want)
	}
}

func TestExpandRefusesAStateItDoesNotDeclare(t *testing.T) {
	cases := map[string]struct {
		d    *Diagram
		want string
	}{
		"the initial state":         {d: &Diagram{States: []string{"s"}, Init: "t"}, want: "t"},
		"the source of an edge":     {d: &Diagram{States: []string{"s"}, Init: "s", Edges: []Edge{{Src: "u", Dst: "s", Event: Event{Channel: "go"}}}}, want: "u"},
		"the target of an edge":     {d: &Diagram{States: []string{"s"}, Init: "s", Edges: []Edge{{Src: "s", Dst: "u", Event: Event{Channel: "go"}}}}, want: "u"},
		"a state holding variables": {d: &Diagram{States: []string{"s"}, Vars: map[string][]string{"u": {"x"}}, Init: "s"}, want: "u"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := Expand(c.d, Env{Types: map[string][]Value{"x": {0}}})

			// Assert
			assertErrorContains(t, err, "no state "+c.want)
		})
	}
}

func TestExpandRefusesAClauseThatIsNotWellFormed(t *testing.T) {
	short := clause.And{Conjuncts: []clause.Formula{clause.Builtin{Op: clause.OpEq, Args: []clause.Term{clause.Free{Name: "x"}}}}}
	dangling := clause.And{Conjuncts: []clause.Formula{clause.Atom{Parts: []clause.Part{clause.Hole{Term: clause.Bound{Index: 0}}, clause.Text(" > 0")}}}}
	cases := map[string]struct {
		d    *Diagram
		want string
	}{
		"a builtin short of arguments in a guard": {d: &Diagram{States: []string{"s"}, Init: "s",
			Edges: []Edge{{Src: "s", Dst: "s", Event: Event{Channel: "go"}, Guard: short}}}, want: "clause.WellFormed"},
		"a builtin short of arguments in the start edge": {d: &Diagram{States: []string{"s"}, Vars: map[string][]string{"s": {"x"}}, Init: "s", InitPost: short},
			want: "clause.WellFormed"},
		"an index no binder binds in a post": {d: &Diagram{States: []string{"s"}, Init: "s",
			Edges: []Edge{{Src: "s", Dst: "s", Event: Event{Channel: "go"}, Post: dangling}}}, want: "the index 0"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := Expand(c.d, Env{Types: map[string][]Value{"x": {0}}})

			// Assert
			assertErrorContains(t, err, c.want)
		})
	}
}
