package decorate

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v6/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v6/csdf/coreio"
)

const machine = `@startuml
state "idle" as idle
idle : cash ; Nat
state "paying" as paying
paying : cash ; Nat
paying : credit ; Nat
[*] --> idle : $cash' is zero
idle --> paying : insert?coin ; $coin is small ; $credit' is $coin ∧ $cash' is $cash
paying --> idle : cancel ; ; @eq(cash', cash)
paying --> [*] : $credit is zero
note as channels
  channel insert : Nat
end note
@enduml
`

func decorated(t *testing.T, words Words) []string {
	t.Helper()
	d, err := coreio.ReadBytes([]byte(machine))
	if err != nil {
		t.Fatalf("Setup: coreio.ReadBytes error = %v", err)
	}
	out, err := Decorate(machine, d, words)
	if err != nil {
		t.Fatalf("Decorate() error = %v", err)
	}
	return strings.Split(out, "\n")
}

func TestDecorateShouldKeepEveryLineInItsPlace(t *testing.T) {
	// Execute
	got := decorated(t, English)

	// Assert
	if want := strings.Split(machine, "\n"); len(got) != len(want) {
		t.Fatalf("Decorate() has %d lines, want %d", len(got), len(want))
	}
}

func TestDecorateShouldLabelTheEventTheConditionAndTheChange(t *testing.T) {
	// Execute
	got := decorated(t, English)[7]

	// Assert
	for _, want := range []string{
		"idle --> paying : ",
		"<color:#888>Event:</color> <b>insert?<color:#0b57d0>coin</color></b>",
		`\l<color:#888>Condition:</color> <color:#0b57d0>coin</color><size:9><color:#555>event</color></size> is small`,
		`\l<color:#888>Change:</color> <color:#0b57d0>credit</color><size:9><color:#555>after</color></size> is <color:#0b57d0>coin</color><size:9><color:#555>event</color></size>`,
		`\l    <color:#0b57d0>cash</color><size:9><color:#555>after</color></size> is <color:#0b57d0>cash</color><size:9><color:#555>before</color></size>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Decorate() line = %q, want it to hold %q", got, want)
		}
	}
}

func TestDecorateShouldLeaveNoBlankLineBetweenSections(t *testing.T) {
	// Execute
	got := decorated(t, English)[7]

	// Assert
	if strings.Contains(got, `\l\l`) {
		t.Errorf("Decorate() line = %q, want no blank line", got)
	}
}

func TestDecorateShouldListTheKeptVariables(t *testing.T) {
	// Execute
	got := decorated(t, Japanese)[8]

	// Assert
	want := `paying --> idle : <color:#888>イベント:</color> <b>cancel</b>\l<color:#888>維持:</color> <color:#0b57d0>cash</color>\l`
	if got != want {
		t.Errorf("Decorate() line = %q, want %q", got, want)
	}
}

func TestDecorateShouldLabelTheStartAndTheEnd(t *testing.T) {
	// Execute
	got := decorated(t, English)

	// Assert
	if want := `[*] --> idle : <color:#888>Change:</color> <color:#0b57d0>cash</color><size:9><color:#555>after</color></size> is zero\l`; got[6] != want {
		t.Errorf("start line = %q, want %q", got[6], want)
	}
	if want := `paying --> [*] : <color:#888>Condition:</color> <color:#0b57d0>credit</color><size:9><color:#555>before</color></size> is zero\l`; got[9] != want {
		t.Errorf("end line = %q, want %q", got[9], want)
	}
}

func TestDecorateShouldEscapeCreole(t *testing.T) {
	// Execute
	got := escape("a // b < c -- d")

	// Assert
	if want := "a ~// b ~< c ~-- d"; got != want {
		t.Errorf("escape() = %q, want %q", got, want)
	}
}

const builtins = `@startuml
state "idle" as idle
idle : cash ; Nat
[*] --> idle : @eq(cash', 0)
idle --> idle : refund ; @ne(cash, 0) ; @eq(cash', 0)
@enduml
`

// lineOf is the line of lines that starts with prefix.
func lineOf(t *testing.T, lines []string, prefix string) string {
	t.Helper()
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	t.Fatalf("Setup: no line starts with %q", prefix)
	return ""
}

func TestDecorateShouldSpellABuiltinInWords(t *testing.T) {
	// Setup
	d, err := coreio.ReadBytes([]byte(builtins))
	if err != nil {
		t.Fatalf("Setup: coreio.ReadBytes error = %v", err)
	}

	// Execute
	out, err := Decorate(builtins, d, English)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got := lineOf(t, strings.Split(out, "\n"), "idle --> idle : ")
	if want := `<color:#888>Condition:</color> <color:#0b57d0>cash</color> is not 0\l`; !strings.Contains(got, want) {
		t.Errorf("Decorate() line = %q, want it to hold %q", got, want)
	}
}

func TestDecorateShouldRefuseABuiltinItHasNoPhraseFor(t *testing.T) {
	// Setup
	d, err := coreio.ReadBytes([]byte(builtins))
	if err != nil {
		t.Fatalf("Setup: coreio.ReadBytes error = %v", err)
	}

	// Execute
	_, err = Decorate(builtins, d, Words{})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "@eq") {
		t.Errorf("Decorate() error = %v, want one naming @eq", err)
	}
}

func free(name string, primed bool) clause.Term { return clause.Free{Name: name, Primed: primed} }

func TestConjunctTextShouldSpellEachConjunct(t *testing.T) {
	m, m2, k, v := free("m", false), free("m", true), free("k", false), free("v", false)
	// ∃ n : T. n' = n ∧ n ≠ n₀, whose hint n clashes with the free n
	exists := clause.Exists{Hint: "n", Type: "T", Body: clause.And{Conjuncts: []clause.Formula{
		clause.Builtin{Op: clause.OpEq, Args: []clause.Term{free("n", true), clause.Bound{Index: 0}}},
		clause.Builtin{Op: clause.OpNe, Args: []clause.Term{clause.Bound{Index: 0}, free("n", false)}},
	}}}
	hole := clause.Atom{Parts: []clause.Part{
		clause.Hole{Term: clause.Field{Of: clause.Index{Map: m2, Key: k}, Name: "x"}},
		clause.Text(" が 0"),
	}}
	tests := map[string]struct {
		words Words
		f     clause.Formula
		want  string
	}{
		"exists named apart from a variable of the diagram": {English, clause.Exists{Hint: "cash", Type: "T", Body: clause.And{Conjuncts: []clause.Formula{
			clause.Builtin{Op: clause.OpEq, Args: []clause.Term{free("n", true), clause.Bound{Index: 0}}},
		}}}, "for some cash2 of T, n' is cash2"},
		"ne in English":               {English, clause.Builtin{Op: clause.OpNe, Args: []clause.Term{k, v}}, "k is not v"},
		"ne in Japanese":              {Japanese, clause.Builtin{Op: clause.OpNe, Args: []clause.Term{k, v}}, "k は v でない"},
		"eq in English":               {English, clause.Builtin{Op: clause.OpEq, Args: []clause.Term{k, v}}, "k is v"},
		"eq in Japanese":              {Japanese, clause.Builtin{Op: clause.OpEq, Args: []clause.Term{k, v}}, "k は v"},
		"in in English":               {English, clause.Builtin{Op: clause.OpIn, Args: []clause.Term{k, m}}, "m has k"},
		"in in Japanese":              {Japanese, clause.Builtin{Op: clause.OpIn, Args: []clause.Term{k, m}}, "m に k がある"},
		"notin in English":            {English, clause.Builtin{Op: clause.OpNotIn, Args: []clause.Term{k, m}}, "m has no k"},
		"notin in Japanese":           {Japanese, clause.Builtin{Op: clause.OpNotIn, Args: []clause.Term{k, m}}, "m に k がない"},
		"empty in English":            {English, clause.Builtin{Op: clause.OpEmpty, Args: []clause.Term{m}}, "m is empty"},
		"empty in Japanese":           {Japanese, clause.Builtin{Op: clause.OpEmpty, Args: []clause.Term{m}}, "m は空"},
		"update in English":           {English, clause.Builtin{Op: clause.OpUpdate, Args: []clause.Term{m2, m, k, v}}, "m' is m with k set to v"},
		"update in Japanese":          {Japanese, clause.Builtin{Op: clause.OpUpdate, Args: []clause.Term{m2, m, k, v}}, "m' は m の k を v にしたもの"},
		"frame in English":            {English, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m, k, v}}, "m' is m except at k, v"},
		"frame in Japanese":           {Japanese, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m, k, v}}, "m' は k・v のほか m と同じ"},
		"frame of no key in English":  {English, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m}}, "m' is m"},
		"frame of no key in Japanese": {Japanese, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m}}, "m' は m"},
		"at in English":               {English, clause.Builtin{Op: clause.OpAt, Args: []clause.Term{k}, States: []string{"A", "B"}}, "k is at A, B"},
		"at in Japanese":              {Japanese, clause.Builtin{Op: clause.OpAt, Args: []clause.Term{k}, States: []string{"A", "B"}}, "k は A・B にある"},
		"exists in English":           {English, exists, "for some n2 of T, (n' is n2 and n2 is not n)"},
		"exists in Japanese":          {Japanese, exists, "ある T の n2 について (n' は n2 かつ n2 は n でない)"},
		"exists of one conjunct": {English, clause.Exists{Hint: "x", Type: "T", Body: clause.And{Conjuncts: []clause.Formula{
			clause.Builtin{Op: clause.OpEq, Args: []clause.Term{free("n", true), clause.Bound{Index: 0}}},
		}}}, "for some x of T, n' is x"},
		"a hole without braces":                          {Japanese, hole, "$m'[k].x が 0"},
		"a hole before text that would read as its path": {English, clause.Atom{Parts: []clause.Part{clause.Hole{Term: m}, clause.Text(".x and [k]")}}, "$m .x and [k]"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := conjunctText(tt.f, tt.words, map[string]bool{"cash": true})

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("conjunctText() = %q, want %q", got, tt.want)
			}
		})
	}
}
