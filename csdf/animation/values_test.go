package animation

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/google/go-cmp/cmp"
)

func jsonNumber(s string) json.Number { return json.Number(s) }

// promotedDiagram is a diagram a promotion made: its state holds the map
// members, whose instances are at opened, holding count, or at closed.
func promotedDiagram() *core.Diagram {
	return &core.Diagram{
		States: []string{"G"},
		Vars:   map[string][]string{"G": {"members", "total"}},
		Init:   "G",
		Families: []core.FamilyStates{{
			Map: "members", Key: "Member", Init: "none", States: []string{"none", "opened", "closed"},
			Vars: map[string][]string{"opened": {"count"}},
		}},
		InitPost: clause.And{},
	}
}

func TestEnterValuesTakesInstancesOfAFamily(t *testing.T) {
	// Setup
	session, err := NewSession(promotedDiagram(), NoSuggester{})
	if err != nil {
		t.Fatalf("Setup: NewSession() error = %v", err)
	}

	// Execute
	state, err := session.EnterValues(`[{"a": {"at": "opened", "fields": {"count": 3}}, "b": {"at": "closed", "fields": {}}}, 3]`)

	// Assert
	if err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	want := State{ID: "G", Name: "G", Values: []Value{
		{Name: "members", Value: map[string]any{
			"a": map[string]any{"at": "opened", "fields": map[string]any{"count": jsonNumber("3")}},
			"b": map[string]any{"at": "closed", "fields": map[string]any{}},
		}},
		{Name: "total", Value: jsonNumber("3")},
	}}
	if diff := cmp.Diff(want, state); diff != "" {
		t.Errorf("EnterValues() mismatch (-want +got):\n%s", diff)
	}
}

func TestEnterValuesReadsNegativeZeroAsZero(t *testing.T) {
	// Setup
	session, err := NewSession(promotedDiagram(), NoSuggester{})
	if err != nil {
		t.Fatalf("Setup: NewSession() error = %v", err)
	}

	// Execute
	state, err := session.EnterValues(`[{"a": {"at": "opened", "fields": {"count": -0}}}, -0.0]`)

	// Assert
	if err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	want := State{ID: "G", Name: "G", Values: []Value{
		{Name: "members", Value: map[string]any{
			"a": map[string]any{"at": "opened", "fields": map[string]any{"count": jsonNumber("0")}},
		}},
		{Name: "total", Value: jsonNumber("0.0")},
	}}
	if diff := cmp.Diff(want, state); diff != "" {
		t.Errorf("EnterValues() mismatch (-want +got):\n%s", diff)
	}
}

func TestEnterValuesRefusesAnInstanceThatIsNoneOfTheFamilys(t *testing.T) {
	tests := map[string]struct {
		members string
		want    string
	}{
		"not a map": {
			members: `[]`,
			want:    `members: a map of instances is a JSON object from IDs to {"at": <state>, "fields": {...}}`,
		},
		"an instance of no at": {
			members: `{"a": {"fields": {}}}`,
			want:    `members["a"]: an instance is {"at": <state>, "fields": {...}}`,
		},
		"an instance of another key": {
			members: `{"a": {"at": "closed", "fields": {}, "x": 1}}`,
			want:    `members["a"]: an instance is {"at": <state>, "fields": {...}}`,
		},
		"a state of no family": {
			members: `{"a": {"at": "frozen", "fields": {}}}`,
			want:    `members["a"]: "frozen" is no state of the family of members; it is at one of [opened, closed]`,
		},
		"the start state, which is absence": {
			members: `{"a": {"at": "none", "fields": {}}}`,
			want:    `members["a"]: "none" is the start state of the family of members, which is absence; leave the entry out`,
		},
		"a field the state does not hold": {
			members: `{"a": {"at": "opened", "fields": {"count": 1, "owner": "x"}}}`,
			want:    `members["a"]: an instance at opened holds the fields [count], got [count, owner]`,
		},
		"a field left out": {
			members: `{"a": {"at": "opened", "fields": {}}}`,
			want:    `members["a"]: an instance at opened holds the fields [count], got []`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session, err := NewSession(promotedDiagram(), NoSuggester{})
			if err != nil {
				t.Fatalf("Setup: NewSession() error = %v", err)
			}

			// Execute
			_, err = session.EnterValues(`[` + tt.members + `, 0]`)

			// Assert
			var refused *RefusedValuesError
			if !errors.As(err, &refused) || refused.Error() != tt.want {
				t.Fatalf("EnterValues() error = %v, want refused values saying %q", err, tt.want)
			}
		})
	}
}

// eqDiagram holds builtins over its variables alone, which Go decides
// without a glue.
const eqDiagram = `@startuml
state "S" as s0
s0 : a ; a
s0 : b ; b
s0 : m ; m
[*] --> s0 : @eq(a', b') ∧ @empty(m')
s0 --> s0 : tick ; @ne(a, b) ; @eq(a', a) ∧ @eq(m', m)
@enduml
`

func TestEnterValuesWithoutACheckerRefusesValuesThatFalsifyABuiltin(t *testing.T) {
	tests := map[string]struct {
		prev   string
		values string
		want   string
	}{
		"@eq of the variables after the start edge": {
			values: `[1, 2, {}]`,
			want:   "the post does not hold of these values: @eq(a', b')",
		},
		"@empty of a map holding an entry": {
			values: `[1, 1, {"x": 1}]`,
			want:   "the post does not hold of these values: @empty(m')",
		},
		"a guard the previous state falsifies": {
			prev:   `[1, 1, {}]`,
			values: `[1, 0, {}]`,
			want:   "the guard does not hold of these values: @ne(a, b)",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := newSession(t, eqDiagram)
			if tt.prev != "" {
				if _, err := session.EnterValues(tt.prev); err != nil {
					t.Fatalf("Setup: EnterValues() error = %v", err)
				}
				if err := session.Select(0); err != nil {
					t.Fatalf("Setup: Select() error = %v", err)
				}
			}

			// Execute
			_, err := session.EnterValues(tt.values)

			// Assert
			var refused *RefusedValuesError
			if !errors.As(err, &refused) || refused.Error() != tt.want {
				t.Fatalf("EnterValues() error = %v, want refused values saying %q", err, tt.want)
			}
		})
	}
}

func TestEnterValuesWithoutACheckerTakesWhatNoBuiltinFalsifies(t *testing.T) {
	tests := map[string]struct {
		source string
		values string
	}{
		"values every builtin of the post holds of": {
			source: eqDiagram,
			values: `[1, 1, {}]`,
		},
		"an atom, which is not evaluated": {
			source: twoStateDiagram,
			values: `[5]`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := newSession(t, tt.source)

			// Execute
			_, err := session.EnterValues(tt.values)

			// Assert
			if err != nil {
				t.Fatalf("EnterValues() error = %v, want the values taken", err)
			}
		})
	}
}

func TestEnterValuesWithoutACheckerSkipsABuiltinReadingAField(t *testing.T) {
	// Setup
	session := newSession(t, builtinsDiagram)
	if _, err := session.EnterValues(`[{}, 0]`); err != nil {
		t.Fatalf("Setup: EnterValues() error = %v", err)
	}
	if err := session.Select(0); err != nil {
		t.Fatalf("Setup: Select() error = %v", err)
	}

	// Execute
	_, err := session.EnterValues(`[{"7": 1}, 7]`)

	// Assert
	if err != nil {
		t.Fatalf("EnterValues() error = %v, want the values taken: put's builtins read its field x", err)
	}
}

type admittingOnly struct {
	NoSuggester
	values string
}

func (a admittingOnly) Admit(_ *core.Diagram, _ Pending, values []Value) error {
	return AmongSuggestions([]Suggestion{{Values: json.RawMessage(a.values)}}, values)
}

func TestEnterValuesTakesOnlyWhatTheSuggesterAdmits(t *testing.T) {
	tests := map[string]struct {
		values  string
		wantErr string
	}{
		"values the suggester admits":                    {values: `[{}, 0]`},
		"the same values written otherwise":              {values: ` [ {} , 0 ] `},
		"a float where the suggester has an integer":     {values: `[{}, 0.0]`, wantErr: "the checker admits none of these values; enter one of its suggestions"},
		"values the suggester does not admit":            {values: `[{}, 2]`, wantErr: "the checker admits none of these values; enter one of its suggestions"},
		"a truth value where the suggester has a number": {values: `[{}, false]`, wantErr: "the checker admits none of these values; enter one of its suggestions"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session, err := NewSession(mustRead(t, builtinsDiagram), admittingOnly{values: `[{}, 0]`})
			if err != nil {
				t.Fatalf("Setup: NewSession() error = %v", err)
			}

			// Execute
			_, err = session.EnterValues(tt.values)

			// Assert
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("EnterValues() error = %v, want the values taken", err)
				}
				return
			}
			var refused *RefusedValuesError
			if !errors.As(err, &refused) || refused.Error() != tt.wantErr {
				t.Fatalf("EnterValues() error = %v, want refused values saying %q", err, tt.wantErr)
			}
		})
	}
}

func TestAmongSuggestionsComparesValuesAsLiterals(t *testing.T) {
	tests := map[string]struct {
		suggested, entered string
		admitted           bool
	}{
		"0.0 is 0.0":                    {suggested: `[0.0]`, entered: "0.0", admitted: true},
		"negative zero is zero":         {suggested: `[0.0]`, entered: "-0.0", admitted: true},
		"a float written otherwise":     {suggested: `[1.5]`, entered: "15e-1", admitted: true},
		"an integer beyond the floats":  {suggested: `[12345678901234567890]`, entered: "12345678901234567891", admitted: false},
		"an object of the same entries": {suggested: `[{"a": 1, "b": 2}]`, entered: `{"b": 2, "a": 1}`, admitted: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			var v any
			dec := json.NewDecoder(strings.NewReader(tt.entered))
			dec.UseNumber()
			if err := dec.Decode(&v); err != nil {
				t.Fatal(err)
			}

			// Execute
			err := AmongSuggestions([]Suggestion{{Values: json.RawMessage(tt.suggested)}}, []Value{{Name: "x", Value: v}})

			// Assert
			if got := err == nil; got != tt.admitted {
				t.Errorf("AmongSuggestions(%s, %s) = %v, want admitted %v", tt.suggested, tt.entered, err, tt.admitted)
			}
		})
	}
}
