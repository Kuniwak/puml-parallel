package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lint"
	"github.com/google/go-cmp/cmp"
)

func TestSelfTransitionDivergenceRule(t *testing.T) {
	type testCase struct {
		Source  string
		WantTSV string
	}

	header := "file\tstart_line\tend_line\trule\tseverity\tmessage\n"
	diverges := func(line, cycle, src, dst string) string {
		return "a.puml\t" + line + "\t" + line + "\tself-transition-divergence\tWARN\tThe tau edge `" + src + "` -> `" + dst + "` lies on the cycle of tau edges " + cycle + ", which the start reaches, and its guard is not false on its face, so the diagram may diverge: the cycle may run forever while nothing observable happens. If it cannot, prove the diagram livelock-free with csdflivelockfree; if it can, write a guard that excludes it, or make one of its edges observable.\n"
	}
	diagram := func(lines ...string) string {
		return strings.Join(append(append([]string{"@startuml"}, lines...), "@enduml", ""), "\n")
	}

	testCases := map[string]testCase{
		"a tau self-loop diverges (representative value)": {
			Source:  diagram(`state "s0" as s0`, "[*] --> s0", "s0 --> s0 : tau"),
			WantTSV: header + diverges("4", "`s0` -> `s0`", "s0", "s0"),
		},
		"every edge of a longer tau cycle diverges (representative value)": {
			Source: diagram(`state "s0" as s0`, `state "s1" as s1`, "[*] --> s0", "s0 --> s1 : tau", "s1 --> s0 : tau"),
			WantTSV: header +
				diverges("5", "`s0` -> `s1` -> `s0`", "s0", "s1") +
				diverges("6", "`s1` -> `s0` -> `s1`", "s1", "s0"),
		},
		"a tau self-loop whose guard is a natural-language atom diverges (representative value)": {
			Source:  diagram(`state "s0" as s0`, "s0 : n ; Nat", "[*] --> s0", "s0 --> s0 : tau ; $n > 0 ; @eq(n', n)"),
			WantTSV: header + diverges("5", "`s0` -> `s0`", "s0", "s0"),
		},
		"a self-loop on an observable event does not diverge (lower boundary value)": {
			Source:  diagram(`state "s0" as s0`, "[*] --> s0", "s0 --> s0 : a"),
			WantTSV: header,
		},
		"a tau edge on no cycle does not diverge (lower boundary value)": {
			Source:  diagram(`state "s0" as s0`, `state "s1" as s1`, "[*] --> s0", "s0 --> s1 : tau", "s1 --> s0 : a"),
			WantTSV: header,
		},
		"a tau cycle the start does not reach does not diverge (lower boundary value)": {
			Source:  diagram(`state "s0" as s0`, `state "s1" as s1`, "[*] --> s0", "s1 --> s1 : tau"),
			WantTSV: header,
		},
		"a guard @ne of a term with itself is false on its face (lower boundary value)": {
			Source:  diagram(`state "s0" as s0`, "s0 : n ; Nat", "[*] --> s0", "s0 --> s0 : tau ; @ne(n, n)"),
			WantTSV: header,
		},
		"a guard with @eq and @ne of the same terms is false on its face (lower boundary value)": {
			Source:  diagram(`state "s0" as s0`, "s0 : n ; Nat", "[*] --> s0", "s0 --> s0 : tau ; @eq(n, 0) ∧ @ne(0, n)"),
			WantTSV: header,
		},
		"a guard with @in and @notin of the same key is false on its face (lower boundary value)": {
			Source:  diagram(`state "s0" as s0`, "s0 : m ; Map", "[*] --> s0", "s0 --> s0 : tau ; @in(k, m) ∧ @notin(k, m)"),
			WantTSV: header,
		},
		"a guard with @empty and @in of the same map is false on its face (lower boundary value)": {
			Source:  diagram(`state "s0" as s0`, "s0 : m ; Map", "[*] --> s0", "s0 --> s0 : tau ; @empty(m) ∧ @in(k, m)"),
			WantTSV: header,
		},
		"a guard whose ∃ body is false on its face is false on its face (lower boundary value)": {
			Source:  diagram(`state "s0" as s0`, "[*] --> s0", "s0 --> s0 : tau ; @exists(v : T) (@ne(v, v))"),
			WantTSV: header,
		},
		"a cycle broken by one guard false on its face does not diverge (lower boundary value)": {
			Source:  diagram(`state "s0" as s0`, "s0 : n ; Nat", `state "s1" as s1`, "s1 : n ; Nat", "[*] --> s0", "s0 --> s1 : tau", "s1 --> s0 : tau ; @ne(n, n)"),
			WantTSV: header,
		},
		"@ne of two different terms is not false on its face (upper boundary value)": {
			Source:  diagram(`state "s0" as s0`, "s0 : n ; Nat", "[*] --> s0", "s0 --> s0 : tau ; @ne(n, n')"),
			WantTSV: header + diverges("5", "`s0` -> `s0`", "s0", "s0"),
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			in := lint.NewInput("a.puml", []byte(testCase.Source))
			if in.ParseErr != nil {
				t.Fatal(in.ParseErr)
			}

			// Act
			findings := lint.Run([]lint.Rule{lint.SelfTransitionDivergenceRule{}}, in)

			// Assert
			var sb strings.Builder
			if err := lint.WriteTSV(&sb, findings); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(testCase.WantTSV, sb.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}
