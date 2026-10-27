package tracechk_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "rewrite the expected reports")

// TestWriteMarkdown holds the report of each trace to the expected one in
// testdata, which names the values at each step of a path apart: v@i is the
// variable v after step i, and k#i the field k step i receives.
func TestWriteMarkdown(t *testing.T) {
	testCases := map[string]struct {
		Diagram string
		Trace   []string
		Golden  string
	}{
		"a refusing branch says where and along which path": {
			Diagram: `@startuml
state "A" as A
A : n ; n
state "B" as B
state "C" as C
state "D" as D
[*] --> A : $n' = 0
A --> B : p ; $n = 0
A --> C : p
C --> D : q
@enduml
`,
			Trace:  []string{"p", "q"},
			Golden: "refusing.md",
		},
		"an accepted trace states one obligation per prefix path, each received field bound per step": {
			Diagram: `@startuml
state "a" as a
a : n ; n
state "b" as b
b : n ; n
b : m ; m
[*] --> a : $n' = 0
a --> b : x?k ; $n < $k ; $n' = $n + $k ∧ $m' = 0
b --> a : tau ; $m = 0 ; @eq(n', n)
b --> b : y ; $m > 0 ; @eq(n', n) ∧ @eq(m', m)
a --> a : y ; $n > 0 ; @eq(n', n)
note as channels
  channel x : k
end note
@enduml
`,
			Trace:  []string{"x?k", "y"},
			Golden: "received.md",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			d := read(t, tc.Diagram)
			golden := filepath.Join("testdata", tc.Golden)
			var sb strings.Builder

			// Execute
			err := tracechk.WriteMarkdown(&sb, d, tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Name: "trace.tsv", Events: tc.Trace}))
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, []byte(sb.String()), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			// Assert
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(want), sb.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

// TestWriteMarkdownSpellsWhatEachObligationReads pins, by hand, what the
// obligations of a report read.
func TestWriteMarkdownSpellsWhatEachObligationReads(t *testing.T) {
	testCases := map[string]struct {
		Diagram string
		Trace   []string
		Want    string
	}{
		"a sent field is the value the step holds, and no field of its own": {
			Diagram: "@startuml\nstate \"a\" as a\na : n ; n\n[*] --> a : $n' = 0\na --> a : out!n ; ; @eq(n', n)\nnote as channels\n  channel out : n\nend note\n@enduml\n",
			Trace:   []string{"out!n", "out!n"},
			Want:    "Obligation: ∀ n@0 n@1. \"$_0 = 0\"(n@0) ∧ n@1 = n@0 → ∃ n'. n' = n@1",
		},
		"a received field is a value of its step": {
			Diagram: "@startuml\nstate \"a\" as a\na : n ; n\n[*] --> a : $n' = 0\na --> a : in?k ; ; $n' = $k\nnote as channels\n  channel in : k\nend note\n@enduml\n",
			Trace:   []string{"in?k", "in?k"},
			Want:    "Obligation: ∀ n@0 n@1 k#1. \"$_0 = 0\"(n@0) ∧ \"$_0 = $_1\"(n@1, k#1) → ∃ k n'. \"$_0 = $_1\"(n', k)",
		},
		"a quantifier a disjunct follows is parenthesised": {
			Diagram: "@startuml\nstate \"a\" as a\na : n ; n\nstate \"b\" as b\nb : n ; n\n[*] --> a : $n' = 0\na --> b : tau ; ; $n' = 1\na --> a : go ; ; $n' = 2\nb --> b : go ; ; @eq(n', n)\n@enduml\n",
			Trace:   []string{"go"},
			Want:    "Obligation: ∀ n@0. \"$_0 = 0\"(n@0) → (∃ n'. \"$_0 = 1\"(n')) ∨ ∃ n'. \"$_0 = 2\"(n')",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			d := read(t, tc.Diagram)
			r := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Name: "trace.tsv", Events: tc.Trace})
			var sb strings.Builder

			// Execute
			err := tracechk.WriteMarkdown(&sb, d, r)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(sb.String(), tc.Want) {
				t.Errorf("no %q in\n%s", tc.Want, sb.String())
			}
		})
	}
}
