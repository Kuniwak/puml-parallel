package csdf

import (
	"strings"
	"testing"
)

// A diagram refuses a name the identifier rule refuses: a reserved word, or
// one that starts with a digit, wherever the diagram names something.
func TestParseRefusesIllegalNames(t *testing.T) {
	tests := map[string]struct {
		src  string
		want string
	}{
		"a state ID that is a reserved word": {
			src:  "@startuml\nstate \"Initial\" as true\n[*] --> true\n@enduml\n",
			want: "true is a reserved word",
		},
		"a state ID starting with a digit": {
			src:  "@startuml\nstate \"Initial\" as 0s\n[*] --> 0s\n@enduml\n",
			want: "does not start with a digit, as 0s",
		},
		"a state variable that is a reserved word": {
			src:  "@startuml\nstate \"Initial\" as s0\ns0 : null\n[*] --> s0\n@enduml\n",
			want: "null is a reserved word",
		},
		"a state variable starting with a digit": {
			src:  "@startuml\nstate \"Initial\" as s0\ns0 : 1x\n[*] --> s0\n@enduml\n",
			want: "does not start with a digit, as 1x",
		},
		"a channel that is a reserved word": {
			src:  "@startuml\nstate \"Initial\" as s0\n[*] --> s0\ns0 --> s0 : false\n@enduml\n",
			want: "false is a reserved word",
		},
		"a field starting with a digit": {
			src:  "@startuml\nstate \"Initial\" as s0\n[*] --> s0\ns0 --> s0 : c?2x\n@enduml\n",
			want: "does not start with a digit, as 2x",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseAccepting(tt.src)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseAccepting() error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestParseChannelRefusesAReservedWord(t *testing.T) {
	_, err := ParseChannel("true")
	if err == nil || !strings.Contains(err.Error(), "true is a reserved word") {
		t.Fatalf("ParseChannel(true) error = %v, want true refused as a reserved word", err)
	}
}
