package tracechk_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
	"github.com/google/go-cmp/cmp"
)

func TestWriteMarkdown(t *testing.T) {
	type testCase struct {
		Diagram string
		Trace   []csdf.Event
		Want    string
	}

	testCases := map[string]testCase{
		"a refusing branch says where and along which path": {
			Diagram: `@startuml
state "A" as A
A: n
state "B" as B
state "C" as C
state "D" as D
[*] --> A : n' = 0
A --> B : 1 ; n = 0 ; true
A --> C : 1
C --> D : 2
@enduml
`,
			Trace: []csdf.Event{"1", "2"},
			Want: "# trace.tsv: REJECTED\n" +
				"\n" +
				"After `1` (1 of 2 events) the diagram may be in B, C.\n" +
				"B is stable and has no edge for `2`, so the diagram may refuse `2` there even\n" +
				"when every guard is true: this is not a trace of the diagram.\n" +
				"\n" +
				"- visible events enabled at the refusing states: (none)\n" +
				"\n" +
				"The refusal is real unless every path below is infeasible, that is, unless\n" +
				"the predicates along it cannot all hold.\n" +
				"\n" +
				"## Path 1: reaches B\n" +
				"\n" +
				"| step | event | transition | guard | post |\n" +
				"|-----:|-------|------------|-------|------|\n" +
				"| 0 | (start) | `[*] --> A` (L7) |  | n' = 0 |\n" +
				"| 1 | `1` | `A --> B` (L8) | n = 0 | true |\n" +
				"\n" +
				"- x0: A (n)\n" +
				"- x1: B (no variables)\n" +
				"\n" +
				"Infeasible iff: ¬∃ x0 x1. post_0(x0) ∧ guard_1(x0)\n" +
				"\n",
		},
		"an accepted trace states one obligation per prefix path": {
			Diagram: `@startuml
state "a" as a
a: n ; Nat
state "b" as b
b: n ; Nat
b: m
[*] --> a : n' = 0
a --> b : x ; n < 3 ; n' = n + 1 ∧ m' = 0
b --> a : tau ; m = 0 ; n' = n
b --> b : y ; m > 0
a --> a : y ; n > 0
@enduml
`,
			Trace: []csdf.Event{"x", "y"},
			Want: "# trace.tsv: ACCEPTED when every guard is true\n" +
				"\n" +
				"After every prefix of the trace, every stable state the diagram may be in can\n" +
				"perform the next event when every guard is taken as true. Whether it can in\n" +
				"fact depends on the natural-language predicates below, which this tool does not\n" +
				"evaluate. A path never revisits a state within one run of `tau` edges, so\n" +
				"paths going round a `tau` cycle, and their obligations, are not listed.\n" +
				"\n" +
				"- trace: `x`, `y`\n" +
				"- the start edge `[*] --> a` (L7) must admit a valuation: ∃ x0. post_0(x0), where post_0 is `n' = 0`\n" +
				"\n" +
				"## Event 1: `x` after the empty prefix\n" +
				"\n" +
				"### Path 1.1: reaches a\n" +
				"\n" +
				"| step | event | transition | guard | post |\n" +
				"|-----:|-------|------------|-------|------|\n" +
				"| 0 | (start) | `[*] --> a` (L7) |  | n' = 0 |\n" +
				"\n" +
				"- x0: a (n)\n" +
				"\n" +
				"| edge out of the reached state | transition | guard | post |\n" +
				"|-------------------------------|------------|-------|------|\n" +
				"| next | `a --> b` (L8) | n < 3 | n' = n + 1 ∧ m' = 0 |\n" +
				"\n" +
				"Obligation: ∀ x0. post_0(x0) → (guard_L8(x0) ∧ ∃ x'. post_L8(x0, x'))\n" +
				"\n" +
				"## Event 2: `y` after `x`\n" +
				"\n" +
				"### Path 2.1: reaches b\n" +
				"\n" +
				"| step | event | transition | guard | post |\n" +
				"|-----:|-------|------------|-------|------|\n" +
				"| 0 | (start) | `[*] --> a` (L7) |  | n' = 0 |\n" +
				"| 1 | `x` | `a --> b` (L8) | n < 3 | n' = n + 1 ∧ m' = 0 |\n" +
				"\n" +
				"- x0: a (n)\n" +
				"- x1: b (n, m)\n" +
				"\n" +
				"| edge out of the reached state | transition | guard | post |\n" +
				"|-------------------------------|------------|-------|------|\n" +
				"| tau | `b --> a` (L9) | m = 0 | n' = n |\n" +
				"| next | `b --> b` (L10) | m > 0 | true |\n" +
				"\n" +
				"Obligation: ∀ x0 x1. post_0(x0) ∧ guard_1(x0) ∧ post_1(x0, x1) → (guard_L9(x1) ∧ ∃ x'. post_L9(x1, x')) ∨ guard_L10(x1)\n" +
				"\n" +
				"### Path 2.2: reaches a\n" +
				"\n" +
				"| step | event | transition | guard | post |\n" +
				"|-----:|-------|------------|-------|------|\n" +
				"| 0 | (start) | `[*] --> a` (L7) |  | n' = 0 |\n" +
				"| 1 | `x` | `a --> b` (L8) | n < 3 | n' = n + 1 ∧ m' = 0 |\n" +
				"| 2 | `tau` | `b --> a` (L9) | m = 0 | n' = n |\n" +
				"\n" +
				"- x0: a (n)\n" +
				"- x1: b (n, m)\n" +
				"- x2: a (n)\n" +
				"\n" +
				"| edge out of the reached state | transition | guard | post |\n" +
				"|-------------------------------|------------|-------|------|\n" +
				"| next | `a --> a` (L11) | n > 0 | true |\n" +
				"\n" +
				"Obligation: ∀ x0 x1 x2. post_0(x0) ∧ guard_1(x0) ∧ post_1(x0, x1) ∧ guard_2(x1) ∧ post_2(x1, x2) → guard_L11(x2)\n" +
				"\n" +
				"## Prompt\n" +
				"\n" +
				"Decide whether every obligation above holds, reading each predicate as the\n" +
				"natural-language statement in its table. Answer HOLDS, or FAILS naming the\n" +
				"first obligation that does not hold and valuations that satisfy its premise\n" +
				"but not its conclusion. A guard is not true because it is written down: the\n" +
				"trace is a trace of the diagram only if the predicates make every obligation\n" +
				"hold. FAILS settles that it is not; HOLDS settles it only for the listed\n" +
				"paths, since paths going round a `tau` cycle are not listed.\n" +
				"\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			d := parse(t, testCase.Diagram)
			result := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Name: "trace.tsv", Events: testCase.Trace})

			// Act
			var sb strings.Builder
			if err := tracechk.WriteMarkdown(&sb, d, result); err != nil {
				t.Fatal(err)
			}

			// Assert
			if diff := cmp.Diff(testCase.Want, sb.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}
