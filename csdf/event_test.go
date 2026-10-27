package csdf

import (
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
		"received and chosen fields": {text: "c?x!y", want: "c?x!y", wantFields: []Field{{Received, "x"}, {Sent, "y"}}},
		"whitespace around ? and !":  {text: " c ? x ! y ", want: "c?x!y", wantFields: []Field{{Received, "x"}, {Sent, "y"}}},
		"unicode and snake case":     {text: "AUDIT_BEGIN?監査ID", want: "AUDIT_BEGIN?監査ID", wantFields: []Field{{Received, "監査ID"}}},
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
	for _, text := range []string{"", "?x", "c?", "c?x?", "c!!y", "c x", "c?x y", "f(x)", "a.b", "a-b", "c?x-1"} {
		t.Run(text, func(t *testing.T) {
			if got, err := ParseEvent(text); err == nil {
				t.Errorf("ParseEvent(%q) = %q, want an error", text, got)
			}
		})
	}
}

func TestParseEventRefusesARepeatedFieldName(t *testing.T) {
	// Execute
	_, err := ParseEvent("c?x!x")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "c?x!x repeats the field x") {
		t.Errorf("ParseEvent(c?x!x) error = %v, want the field x refused as repeated", err)
	}
}

func TestParseChannelRefusesTau(t *testing.T) {
	// Execute
	_, err := ParseChannel("tau")

	// Assert
	if err == nil {
		t.Error("ParseChannel(tau) error = nil, want tau refused")
	}
}
