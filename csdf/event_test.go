package csdf

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestParseEvent(t *testing.T) {
	tests := map[string]struct {
		text       string
		want       Event
		wantFields []Field
	}{
		"a channel alone":            {text: "reset", want: "reset"},
		"received and chosen fields": {text: "c?x!y", want: "c?x!y", wantFields: []Field{{Received, "x"}, {Chosen, "y"}}},
		"whitespace around ? and !":  {text: " c ? x ! y ", want: "c?x!y", wantFields: []Field{{Received, "x"}, {Chosen, "y"}}},
		"unicode and kebab case":     {text: "AUDIT-BEGIN?監査ID", want: "AUDIT-BEGIN?監査ID", wantFields: []Field{{Received, "監査ID"}}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ParseEvent(tt.text)
			if err != nil || got != tt.want {
				t.Fatalf("ParseEvent(%q) = %q, %v, want %q", tt.text, got, err, tt.want)
			}
			if !slices.Equal(got.Fields(), tt.wantFields) {
				t.Errorf("Fields() = %#v, want %#v", got.Fields(), tt.wantFields)
			}
			if again := NewEvent(got.Channel(), got.Fields()...); again != got {
				t.Errorf("NewEvent(Channel(), Fields()...) = %q, want %q", again, got)
			}
		})
	}
}

func TestParseEventRefusesTextOutsideTheGrammar(t *testing.T) {
	for _, text := range []string{"", "?x", "c?", "c?x?", "c!!y", "c x", "c?x y", "f(x)", "a.b"} {
		t.Run(text, func(t *testing.T) {
			if got, err := ParseEvent(text); err == nil {
				t.Errorf("ParseEvent(%q) = %q, want an error", text, got)
			}
		})
	}
}

func TestEdgeJSON(t *testing.T) {
	tests := map[string]struct {
		edge Edge
		want string
	}{
		"fields taken apart": {
			edge: Edge{Src: "a", Dst: "b", Event: "c?x!y", Line: 3},
			want: `{"src":"a","dst":"b","event":"c?x!y","channel":"c","fields":[{"dir":"?","name":"x"},{"dir":"!","name":"y"}],"guard":"","guard_clauses":[],"post":"","post_clauses":[],"line":3}`,
		},
		"no field is an empty list": {
			edge: Edge{Src: "a", Dst: "b", Event: "tau", Line: 3},
			want: `{"src":"a","dst":"b","event":"tau","channel":"tau","fields":[],"guard":"","guard_clauses":[],"post":"","post_clauses":[],"line":3}`,
		},
		"clauses as their language reads them": {
			edge: Edge{Src: "a", Dst: "b", Event: "go", Guard: `x \∧ y ∧ @opaque`, Post: "z", Line: 3},
			want: `{"src":"a","dst":"b","event":"go","channel":"go","fields":[],"guard":"x \\∧ y ∧ @opaque","guard_clauses":[{"text":"x ∧ y"},{"opaque":true}],"post":"z","post_clauses":[{"text":"z"}],"line":3}`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := json.Marshal(tt.edge)
			if err != nil || string(got) != tt.want {
				t.Fatalf("json.Marshal() = %s, %v, want %s", got, err, tt.want)
			}
			var back Edge
			if err := json.Unmarshal(got, &back); err != nil || back != tt.edge {
				t.Errorf("json.Unmarshal() = %#v, %v, want %#v", back, err, tt.edge)
			}
		})
	}
}

func TestEdgeJSONRefusesAnEventOutsideTheGrammar(t *testing.T) {
	var e Edge
	err := json.Unmarshal([]byte(`{"src":"a","dst":"b","event":"f(x)"}`), &e)
	if err == nil || !strings.Contains(err.Error(), "f(x)") {
		t.Errorf("json.Unmarshal() error = %v, want one naming f(x)", err)
	}
}

func TestStartAndEndEdgeJSONCarryTheirClauses(t *testing.T) {
	start, _ := json.Marshal(StartEdge{Dst: "a", Post: "n = 0 ∧ @opaque", Line: 1})
	if want := `{"dst":"a","post":"n = 0 ∧ @opaque","post_clauses":[{"text":"n = 0"},{"opaque":true}],"line":1}`; string(start) != want {
		t.Errorf("json.Marshal(StartEdge) = %s, want %s", start, want)
	}
	end, _ := json.Marshal(EndEdge{Src: "a", Guard: "done", Line: 2})
	if want := `{"src":"a","guard":"done","guard_clauses":[{"text":"done"}],"line":2}`; string(end) != want {
		t.Errorf("json.Marshal(EndEdge) = %s, want %s", end, want)
	}
	var back StartEdge
	if err := json.Unmarshal(start, &back); err != nil || back.Post != "n = 0 ∧ @opaque" {
		t.Errorf("json.Unmarshal(StartEdge) = %#v, %v", back, err)
	}
}
