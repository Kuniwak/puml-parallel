package lint_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/lint"
	"github.com/google/go-cmp/cmp"
)

func TestDefaultRulesReportAPostThatLeavesAVariableFree(t *testing.T) {
	// Setup: an edge whose post never names n', which it then leaves free.
	source := strings.Join([]string{
		"@startuml",
		`state "s0" as s0`,
		"s0 : n ; Nat",
		`state "s1" as s1`,
		"s1 : n ; Nat",
		"[*] --> s0 : @eq(n', 0)",
		"s0 --> s1 : a",
		"@enduml",
		"",
	}, "\n")

	// Execute
	findings := lint.Run(lint.DefaultRules(), lint.NewInput("a.puml", []byte(source)))

	// Assert
	if !slices.ContainsFunc(findings, func(f lint.Finding) bool {
		return f.RuleID == "post-leaves-variable-free" && f.StartLine == 7
	}) {
		t.Errorf("want post-leaves-variable-free on line 7, got %+v", findings)
	}
}

func TestPostLeavesVariableFreeRule(t *testing.T) {
	type testCase struct {
		Source  string
		WantTSV string
	}

	header := "file\tstart_line\tend_line\trule\tseverity\tmessage\n"
	edgeFree := func(line, post, primed, plain, first string) string {
		return "a.puml\t" + line + "\t" + line + "\tpost-leaves-variable-free\tHINT\tThe edge `s0` -> `s1` on `a` has the post `" + post + "`, which does not name " + primed + ", so after the step " + plain + " may be any value of its type. If it should not change, keep it in the post, such as `@eq(" + first + ", " + strings.TrimSuffix(first, "'") + ")`; if the choice is intended, say so with an atom that names it, such as `${" + first + "} is chosen freely`.\n"
	}
	startFree := func(line, post string) string {
		return "a.puml\t" + line + "\t" + line + "\tpost-leaves-variable-free\tHINT\tThe start edge to state `s0` has the post `" + post + "`, which does not name `n'`, so `n` may start as any value of its type. Give it a start value in the post, such as `@eq(n', …)`; if any start value is intended, say so with an atom that names it, such as `${n'} is chosen freely`.\n"
	}
	diagram := func(start string, lines ...string) string {
		return strings.Join(append(append([]string{
			"@startuml", `state "s0" as s0`, "s0 : n ; Nat", "s0 : m ; Nat", `state "s1" as s1`, "s1 : n ; Nat", "[*] --> s0" + start,
		}, lines...), "@enduml", ""), "\n")
	}
	mapDiagram := func(lines ...string) string {
		return strings.Join(append(append([]string{
			"@startuml", `state "s0" as s0`, "s0 : m ; K ⇸ V", "s0 : o ; K ⇸ V", `state "s1" as s1`, "s1 : m ; K ⇸ V", "s1 : o ; K ⇸ V", "[*] --> s0 : @empty(m') ∧ @empty(o')",
		}, lines...), "note as channels", "  channel put : K.V", "  channel drop : K", "end note", "@enduml", ""), "\n")
	}
	const started = " : @eq(n', 0) ∧ @eq(m', 0)"

	testCases := map[string]testCase{
		"an omitted post leaves the target's variable free (representative value)": {
			Source:  diagram(started, "s0 --> s1 : a"),
			WantTSV: header + edgeFree("8", "", "`n'`", "`n`", "n'"),
		},
		"a post naming only the source's other variable leaves the target's free (representative value)": {
			Source:  diagram(started, "s0 --> s1 : a ; ; $m > 0"),
			WantTSV: header + edgeFree("8", "$m > 0", "`n'`", "`n`", "n'"),
		},
		"a post that names none of several variables names them all (upper boundary value)": {
			Source:  diagram(started, "s0 --> s0 : a"),
			WantTSV: header + "a.puml\t8\t8\tpost-leaves-variable-free\tHINT\tThe edge `s0` -> `s0` on `a` has the post ``, which does not name `n'`, `m'`, so after the step `n`, `m` may be any value of its type. If it should not change, keep it in the post, such as `@eq(n', n)`; if the choice is intended, say so with an atom that names it, such as `${n'} is chosen freely`.\n",
		},
		"a start edge without a post leaves the start state's variable free (representative value)": {
			Source:  diagram(" : @eq(m', 0)"),
			WantTSV: header + startFree("7", "@eq(m', 0)"),
		},
		"@eq(v', v) keeps the variable (lower boundary value)": {
			Source:  diagram(started, "s0 --> s1 : a ; ; @eq(n', n)"),
			WantTSV: header,
		},
		"an atom reading v' constrains it (lower boundary value)": {
			Source:  diagram(started, "s0 --> s1 : a ; ; $n' = $n + 1"),
			WantTSV: header,
		},
		"an atom saying the choice is intended silences the rule (lower boundary value)": {
			Source:  diagram(started, "s0 --> s1 : a ; ; ${n'} is chosen freely"),
			WantTSV: header,
		},
		"a primed key or function argument names the variable (lower boundary value)": {
			Source:  diagram(started, "s0 --> s1 : a ; ; @eq(f(n'), 1)"),
			WantTSV: header,
		},
		"@update, @frame and @at name the map they write (lower boundary value)": {
			Source:  mapDiagram("s0 --> s1 : put?k?v ; ; @update(m', m, k, v) ∧ @frame(o', o)", "s1 --> s0 : drop?k ; ; @frame(m', m, k) ∧ @at(m'[k], shut) ∧ @eq(o', o)"),
			WantTSV: header,
		},
		"a frame of another map leaves the map free (upper boundary value)": {
			Source:  mapDiagram("s0 --> s1 : put?k?v ; ; @frame(o', m, k)"),
			WantTSV: header + "a.puml\t9\t9\tpost-leaves-variable-free\tHINT\tThe edge `s0` -> `s1` on `put?k?v` has the post `@frame(o', m, k)`, which does not name `m'`, so after the step `m` may be any value of its type. If it should not change, keep it in the post, such as `@eq(m', m)`; if the choice is intended, say so with an atom that names it, such as `${m'} is chosen freely`.\n",
		},
		"a target without variables has nothing to leave free (lower boundary value)": {
			Source:  strings.Join([]string{"@startuml", `state "s0" as s0`, "[*] --> s0", "s0 --> s0 : a", "@enduml", ""}, "\n"),
			WantTSV: header,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			in := lint.NewInput("a.puml", []byte(testCase.Source))
			if in.ParseErr != nil {
				t.Fatal(in.ParseErr)
			}

			// Execute
			findings := lint.Run([]lint.Rule{lint.PostLeavesVariableFreeRule{}}, in)

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

func TestPostLeavesVariableFreeRuleReportsAPostItCannotPrintAsAnError(t *testing.T) {
	// Setup: a post with a variable bound by no ∃, which no reader produces
	// and Print refuses.
	in := &lint.Input{File: "a.puml", Diagram: &core.Diagram{
		States: []string{"s0"},
		Vars:   map[string][]string{"s0": {"n"}},
		Init:   "s0",
		InitPost: clause.And{Conjuncts: []clause.Formula{
			clause.Builtin{Op: clause.OpEq, Args: []clause.Term{clause.Free{Name: "m", Primed: true}, clause.Bound{Index: 0}}},
		}},
		InitLine: 3,
	}}

	// Execute
	findings := lint.PostLeavesVariableFreeRule{}.Check(in)

	// Assert
	if len(findings) != 1 || findings[0].Severity != lint.SeverityError || findings[0].StartLine != 3 {
		t.Fatalf("want one ERROR on line 3, got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "the index 0 is under 0 binders") {
		t.Errorf("want the reason Print gave, got %q", findings[0].Message)
	}
}
