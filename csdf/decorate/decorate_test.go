package decorate

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v7/csdf/coreio"
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

func TestDecorateShouldSpellTheBuiltinsOrRefuseThoseWithoutAPhrase(t *testing.T) {
	tests := map[string]struct {
		words    Words
		wantLine string
		wantErr  string
	}{
		"in words":        {words: English, wantLine: `<color:#888>Condition:</color> <color:#0b57d0>cash</color> is not 0\l`},
		"without phrases": {words: Words{}, wantErr: "@eq"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			d, err := coreio.ReadBytes([]byte(builtins))
			if err != nil {
				t.Fatalf("Setup: coreio.ReadBytes error = %v", err)
			}

			// Execute
			out, err := Decorate(builtins, d, tt.words)

			// Assert
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Decorate() error = %v, want one naming %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := lineOf(t, strings.Split(out, "\n"), "idle --> idle : "); !strings.Contains(got, tt.wantLine) {
				t.Errorf("Decorate() line = %q, want it to hold %q", got, tt.wantLine)
			}
		})
	}
}
