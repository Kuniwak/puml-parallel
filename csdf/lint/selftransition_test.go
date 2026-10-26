package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/lint"
	"github.com/google/go-cmp/cmp"
)

func TestSelfTransitionPostBreaksGuardRule(t *testing.T) {
	type testCase struct {
		Source  string
		WantTSV string
	}

	header := "file\tstart_line\tend_line\trule\tseverity\tmessage\n"
	breaks := func(line, event, guard, post, vars string) string {
		return "a.puml\t" + line + "\t" + line + "\tself-transition-post-breaks-guard\tHINT\tState `s0` has a self-transition on `" + event + "` guarded by `" + guard + "` whose post `" + post + "` does not keep " + vars + ", which the guard reads, so the guard may be false after the step. Assume the guard holds, apply the post, and evaluate the guard again: if it can be false, `s0` is two states - the one where the guard holds and the one after the post - and the edge should go to a second state. If the guard always holds again, keep what it reads with @eq(v', v) in the post, or leave the loop as it is.\n"
	}
	diagram := func(lines ...string) string {
		return strings.Join(append(append([]string{"@startuml", `state "s0" as s0`, "s0 : n ; Nat"}, lines...), "@enduml", ""), "\n")
	}

	testCases := map[string]testCase{
		"a post that changes what the guard reads may break it (representative value)": {
			Source:  diagram("[*] --> s0", "s0 --> s0 : a ; $n > 0 ; $n' = $n - 1"),
			WantTSV: header + breaks("5", "a", "$n > 0", "$n' = $n - 1", "`n`"),
		},
		"an omitted post keeps nothing (representative value)": {
			Source:  diagram("[*] --> s0", "s0 --> s0 : a ; $n > 0"),
			WantTSV: header + breaks("5", "a", "$n > 0", "", "`n`"),
		},
		"a field of the event is not the state's (lower boundary value)": {
			Source:  diagram("[*] --> s0", "s0 --> s0 : a?x ; $x > $n ; @eq(n', n)"),
			WantTSV: header,
		},
		"a post that keeps what the guard reads keeps the guard (lower boundary value)": {
			Source:  diagram("[*] --> s0", "s0 --> s0 : a ; $n > 0 ; @eq(n', n)"),
			WantTSV: header,
		},
		"a post that keeps a map by @frame without keys keeps the guard (lower boundary value)": {
			Source:  diagram("[*] --> s0", "s0 --> s0 : a ; @in(1, n) ; @frame(n', n)"),
			WantTSV: header,
		},
		"a guard on literals alone cannot be broken (lower boundary value)": {
			Source:  diagram("s0 : n ; Nat", "[*] --> s0", "s0 --> s0 : a ; @ne(1, 0) ; $n' = $n + 1"),
			WantTSV: header,
		},
		"a guard that reads nothing cannot be broken (lower boundary value)": {
			Source:  diagram("[*] --> s0", "s0 --> s0 : a ; the door is open ; $n' = $n + 1"),
			WantTSV: header,
		},
		"an omitted guard cannot be broken (lower boundary value)": {
			Source:  diagram("[*] --> s0", "s0 --> s0 : a ; ; $n' = $n + 1"),
			WantTSV: header,
		},
		"an edge to another state is not a self-transition (lower boundary value)": {
			Source:  diagram(`state "s1" as s1`, "s1 : n ; Nat", "[*] --> s0", "s0 --> s1 : a ; $n > 0 ; $n' = $n - 1"),
			WantTSV: header,
		},
		"a frame of the map at some keys does not keep it (upper boundary value)": {
			Source:  diagram("[*] --> s0", "s0 --> s0 : a ; @in(1, n) ; @frame(n', n, 2)"),
			WantTSV: header + breaks("5", "a", "@in(1, n)", "@frame(n', n, 2)", "`n`"),
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
			findings := lint.Run([]lint.Rule{lint.SelfTransitionPostBreaksGuardRule{}}, in)

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

func TestSelfTransitionPostBreaksGuardRuleReportsAClauseItCannotPrintAsAnError(t *testing.T) {
	// Setup: a guard reading n beside a variable bound by no ∃, which no
	// reader produces and Print refuses.
	in := &lint.Input{File: "a.puml", Diagram: &core.Diagram{
		States: []string{"s0"},
		Vars:   map[string][]string{"s0": {"n"}},
		Init:   "s0",
		Edges: []core.Edge{{
			Src: "s0", Dst: "s0", Event: core.Event{Channel: "a"}, Line: 5,
			Guard: clause.And{Conjuncts: []clause.Formula{
				clause.Builtin{Op: clause.OpEq, Args: []clause.Term{clause.Free{Name: "n"}, clause.Bound{Index: 0}}},
			}},
		}},
	}}

	// Execute
	findings := lint.SelfTransitionPostBreaksGuardRule{}.Check(in)

	// Assert
	if len(findings) != 1 || findings[0].Severity != lint.SeverityError || findings[0].StartLine != 5 {
		t.Fatalf("want one ERROR on line 5, got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "the index 0 is under 0 binders") {
		t.Errorf("want the reason Print gave, got %q", findings[0].Message)
	}
}
