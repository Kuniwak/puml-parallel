package csdf

import (
	"reflect"
	"strings"
	"testing"
)

// observed is a local diagram of two live states, A holding x, B holding x
// and y, with the lines extra after its states.
func observed(extra string) string {
	return `@startuml
state "Absent" as S0
state "A" as A
A : x
state "B" as B
B : x
B : y
` + extra + `[*] --> S0
S0 --> A : make
A --> B : go
@enduml
`
}

func TestParseObserveLines(t *testing.T) {
	tests := []struct {
		name  string
		extra string
		want  []Observe
	}{
		{name: "none", extra: "", want: nil},
		{
			name:  "a view and a variable",
			extra: "' CSDF-OBSERVE live = A, B\n' CSDF-OBSERVE x\n",
			want: []Observe{
				{Name: "live", States: []StateID{"A", "B"}, Line: 8},
				{Name: "x", Line: 9},
			},
		},
		{
			name:  "tabs and no spaces around = and ,",
			extra: "\t'\tCSDF-OBSERVE\tone=A\t\n'CSDF-OBSERVE two = A,B\n",
			want: []Observe{
				{Name: "one", States: []StateID{"A"}, Line: 8},
				{Name: "two", States: []StateID{"A", "B"}, Line: 9},
			},
		},
		{
			name:  "a view with no state",
			extra: "' CSDF-OBSERVE live =\n",
			want:  []Observe{{Name: "live", States: []StateID{}, Line: 8}},
		},
		{name: "OBSERVED is a plain comment", extra: "' CSDF-OBSERVED live = A\n", want: nil},
		{name: "OBSERVE-X is a plain comment", extra: "' CSDF-OBSERVE-X live = A\n", want: nil},
		{name: "a marker with nothing after is a plain comment", extra: "' CSDF-OBSERVE\n", want: nil},
		{name: "inside an ignore region", extra: "' CSDF-IGNORE-BEGIN\n' CSDF-OBSERVE live = A\n' CSDF-IGNORE-END\n", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute
			d, err := parseAccepting(observed(tt.extra))

			// Assert
			if err != nil {
				t.Fatalf("parseAccepting() error = %v", err)
			}
			if !reflect.DeepEqual(d.Observes, tt.want) {
				t.Errorf("Observes = %#v, want %#v", d.Observes, tt.want)
			}
		})
	}
}

func TestParseRefusesObserveLines(t *testing.T) {
	tests := []struct {
		name  string
		extra string
		want  string
	}{
		{name: "a marker with no body", extra: "' CSDF-OBSERVE   \n", want: "line 8"},
		{name: "a trailing comma", extra: "' CSDF-OBSERVE live = A,\n", want: "line 8"},
		{name: "two variables", extra: "' CSDF-OBSERVE x y\n", want: "line 8"},
		{name: "a block comment inside", extra: "' CSDF-OBSERVE live = A /' c '/\n", want: "line 8"},
		{name: "a reserved name", extra: "' CSDF-OBSERVE true = A\n", want: "line 8"},
		{name: "a view named like a state", extra: "' CSDF-OBSERVE A = B\n", want: "the view A at line 8 is named like a state"},
		{name: "a view named like a variable", extra: "' CSDF-OBSERVE y = B\n", want: "the view y at line 8 is named like a state variable"},
		{name: "a view declared twice", extra: "' CSDF-OBSERVE v = A\n' CSDF-OBSERVE v = B\n", want: "v at line 9 is declared twice"},
		{name: "a variable declared twice", extra: "' CSDF-OBSERVE x\n' CSDF-OBSERVE x\n", want: "x at line 9 is declared twice"},
		{name: "a view listing the start state", extra: "' CSDF-OBSERVE v = S0\n", want: "the view v at line 8 lists S0, the start state, which is absence; @notin reads it"},
		{name: "a view listing an unknown state", extra: "' CSDF-OBSERVE v = A, C\n", want: "the view v at line 8 lists C, which is no state"},
		{name: "a view listing a state twice", extra: "' CSDF-OBSERVE v = A, A\n", want: "the view v at line 8 lists A twice"},
		{name: "an undeclared variable", extra: "' CSDF-OBSERVE z\n", want: "the CSDF-OBSERVE variable z at line 8 is held by no state"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute
			_, err := parseAccepting(observed(tt.extra))

			// Assert
			if err == nil {
				t.Fatal("parseAccepting() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}
