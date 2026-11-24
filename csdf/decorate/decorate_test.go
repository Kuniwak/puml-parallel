package decorate

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/coreio"
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
