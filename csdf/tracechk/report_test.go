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
A : n
state "B" as B
state "C" as C
state "D" as D
[*] --> A : $n' = 0
A --> B : 1 ; $n = 0
A --> C : 1
C --> D : 2
@enduml
`,
			Trace:  []string{"1", "2"},
			Golden: "refusing.md",
		},
		"an accepted trace states one obligation per prefix path, each received field bound per step": {
			Diagram: `@startuml
state "a" as a
a : n
state "b" as b
b : n
b : m
[*] --> a : $n' = 0
a --> b : x?k ; $n < $k ; $n' = $n + $k ∧ $m' = 0
b --> a : tau ; $m = 0 ; @eq(n', n)
b --> b : y ; $m > 0 ; @eq(n', n) ∧ @eq(m', m)
a --> a : y ; $n > 0 ; @eq(n', n)
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
