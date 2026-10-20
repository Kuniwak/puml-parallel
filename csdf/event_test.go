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
			edge: Edge{Src: "a", Dst: "b", Event: "c?x!y", Guard: "true", Post: "true", Line: 3},
			want: `{"src":"a","dst":"b","event":"c?x!y","channel":"c","fields":[{"dir":"?","name":"x"},{"dir":"!","name":"y"}],"guard":"true","post":"true","line":3}`,
		},
		"no field is an empty list": {
			edge: Edge{Src: "a", Dst: "b", Event: "tau", Guard: "true", Post: "true", Line: 3},
			want: `{"src":"a","dst":"b","event":"tau","channel":"tau","fields":[],"guard":"true","post":"true","line":3}`,
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
