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
	return strings.Split(Decorate(machine, d, words), "\n")
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

func decoratedBuiltins(t *testing.T, words Words) []string {
	t.Helper()
	d, err := coreio.ReadBytes([]byte(builtins))
	if err != nil {
		t.Fatalf("Setup: coreio.ReadBytes error = %v", err)
	}
	return strings.Split(Decorate(builtins, d, words), "\n")
}

func TestDecorateShouldSpellABuiltinInWords(t *testing.T) {
	// Execute
	got := decoratedBuiltins(t, English)[4]

	// Assert
	if want := `<color:#888>Condition:</color> <color:#0b57d0>cash</color> is not 0\l`; !strings.Contains(got, want) {
		t.Errorf("Decorate() line = %q, want it to hold %q", got, want)
	}
}

func free(name string, primed bool) clause.Term { return clause.Free{Name: name, Primed: primed} }

func TestConjunctTextShouldSpellEachBuiltin(t *testing.T) {
	m, m2, k, v := free("m", false), free("m", true), free("k", false), free("v", false)
	cases := []struct {
		words Words
		b     clause.Builtin
		want  string
	}{
		{English, clause.Builtin{Op: clause.OpNe, Args: []clause.Term{k, v}}, "k is not v"},
		{Japanese, clause.Builtin{Op: clause.OpNe, Args: []clause.Term{k, v}}, "k は v でない"},
		{English, clause.Builtin{Op: clause.OpEq, Args: []clause.Term{k, v}}, "k is v"},
		{Japanese, clause.Builtin{Op: clause.OpEq, Args: []clause.Term{k, v}}, "k は v"},
		{English, clause.Builtin{Op: clause.OpIn, Args: []clause.Term{k, m}}, "m has k"},
		{Japanese, clause.Builtin{Op: clause.OpIn, Args: []clause.Term{k, m}}, "m に k がある"},
		{English, clause.Builtin{Op: clause.OpNotIn, Args: []clause.Term{k, m}}, "m has no k"},
		{Japanese, clause.Builtin{Op: clause.OpNotIn, Args: []clause.Term{k, m}}, "m に k がない"},
		{English, clause.Builtin{Op: clause.OpEmpty, Args: []clause.Term{m}}, "m is empty"},
		{Japanese, clause.Builtin{Op: clause.OpEmpty, Args: []clause.Term{m}}, "m は空"},
		{English, clause.Builtin{Op: clause.OpUpdate, Args: []clause.Term{m2, m, k, v}}, "m' is m with k set to v"},
		{Japanese, clause.Builtin{Op: clause.OpUpdate, Args: []clause.Term{m2, m, k, v}}, "m' は m の k を v にしたもの"},
		{English, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m, k, v}}, "m' is m except at k, v"},
		{Japanese, clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{m2, m, k, v}}, "m' は k・v のほか m と同じ"},
		{English, clause.Builtin{Op: clause.OpAt, Args: []clause.Term{k}, States: []string{"A", "B"}}, "k is at A, B"},
		{Japanese, clause.Builtin{Op: clause.OpAt, Args: []clause.Term{k}, States: []string{"A", "B"}}, "k は A・B にある"},
	}
	for _, c := range cases {
		if got := conjunctText(c.b, c.words); got != c.want {
			t.Errorf("conjunctText(%v) = %q, want %q", c.b, got, c.want)
		}
	}
}
