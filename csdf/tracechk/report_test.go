package tracechk_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
	"github.com/google/go-cmp/cmp"
)

func TestWriteMarkdownSaysWhereATraceWasRejected(t *testing.T) {
	// Arrange
	d := parse(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : x
b --> a : y
b --> b : z
@enduml
`)
	result := tracechk.Run(tracechk.MatchExact, d, "trace.tsv", []csdf.Event{"x", "w", "y"})

	// Act
	var sb strings.Builder
	if err := tracechk.WriteMarkdown(&sb, d, result); err != nil {
		t.Fatal(err)
	}

	// Assert
	want := `# trace.tsv: REJECTED

Event 2 of 3, ` + "`w`" + ` (row 3 of trace.tsv), cannot be performed even when every
guard is true, so this is not a trace of the diagram.

- trace so far: ` + "`x`" + `
- states the diagram may be in before it: b
- visible events enabled there: ` + "`y`, `z`" + `

`
	if diff := cmp.Diff(want, sb.String()); diff != "" {
		t.Error(diff)
	}
}

func TestWriteMarkdownListsThePredicatesAndTheObligationOfAnAcceptedTrace(t *testing.T) {
	// Arrange
	d := parse(t, `@startuml
state "a" as a
a: n ; Nat
state "b" as b
b: n ; Nat
b: m
[*] --> a : n' = 0
a --> b : x ; n < 3 ; n' = n + 1 ∧ m' = 0
b --> a : tau ; m = 0 ; n' = n
@enduml
`)
	result := tracechk.Run(tracechk.MatchExact, d, "trace.tsv", []csdf.Event{"x", "x"})

	// Act
	var sb strings.Builder
	if err := tracechk.WriteMarkdown(&sb, d, result); err != nil {
		t.Fatal(err)
	}

	// Assert
	want := "# trace.tsv: ACCEPTED when every guard is true\n" +
		"\n" +
		"The diagram performs the trace `x`, `x` along 1 path when every guard is taken\n" +
		"as true. Whether it is a trace of the diagram in fact depends on the\n" +
		"natural-language predicates below, which this tool does not evaluate.\n" +
		"\n" +
		"## Path 1\n" +
		"\n" +
		"| step | event | transition | guard | post |\n" +
		"|-----:|-------|------------|-------|------|\n" +
		"| 0 | (start) | `[*] --> a` (L7) |  | n' = 0 |\n" +
		"| 1 | `x` | `a --> b` (L8) | n < 3 | n' = n + 1 ∧ m' = 0 |\n" +
		"| 2 | `tau` | `b --> a` (L9) | m = 0 | n' = n |\n" +
		"| 3 | `x` | `a --> b` (L8) | n < 3 | n' = n + 1 ∧ m' = 0 |\n" +
		"\n" +
		"Valuations, one per step, over the variables of the state the step reaches:\n" +
		"\n" +
		"- x0: a (n)\n" +
		"- x1: b (n, m)\n" +
		"- x2: a (n)\n" +
		"- x3: b (n, m)\n" +
		"\n" +
		"Obligation:\n" +
		"\n" +
		"∃ x0 x1 x2 x3. post_0(x0) ∧ guard_1(x0) ∧ post_1(x0, x1) ∧ guard_2(x1) ∧ post_2(x1, x2) ∧ guard_3(x2) ∧ post_3(x2, x3)\n" +
		"\n" +
		"where guard_i and post_i are the guard and post of step i. In guard_i the\n" +
		"variables are those of x(i-1); in post_i the unprimed variables are those of\n" +
		"x(i-1) and the primed ones those of xi. A predicate that is exactly `true`\n" +
		"is left out.\n" +
		"\n" +
		"## Prompt\n" +
		"\n" +
		"Decide whether the obligation of at least one path above is satisfiable,\n" +
		"reading every predicate as the natural-language statement in its table.\n" +
		"Answer SATISFIABLE with a witness for every valuation of that path, or\n" +
		"UNSATISFIABLE naming, for every path, the first conjunct that cannot hold\n" +
		"together with the ones before it. A guard is not true because it is written\n" +
		"down: the trace is a trace of the diagram only if the predicates admit it.\n" +
		"\n"
	if diff := cmp.Diff(want, sb.String()); diff != "" {
		t.Error(diff)
	}
}
