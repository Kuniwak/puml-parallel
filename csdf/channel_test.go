package csdf

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/csdf/types"
)

func TestParseReadsAChannelNote(t *testing.T) {
	d := mustParseAccepting("@startuml\n[*] --> s0\nnote as channels\n  channel put : ID.(K ⇸ V)\n  channel stop\nend note\n@enduml\n")

	want := []ChannelDecl{{Name: "put", Fields: []types.Type{types.Named("ID"), types.MapOf(types.Named("K"), types.Named("V"))}, Line: 4}, {Name: "stop", Fields: []types.Type{}, Line: 5}}
	if diff := cmp.Diff(want, d.Channels); diff != "" {
		t.Error(diff)
	}
	if len(d.Assumptions) != 0 {
		t.Errorf("Assumptions = %v, want none", d.Assumptions)
	}
}

func TestParseRejectsAMalformedChannelNote(t *testing.T) {
	cases := map[string]struct{ line, want string }{
		"no type after the colon":          {line: "channel put :", want: "no type"},
		"an empty field type":              {line: "channel put : ID..V", want: "no type"},
		"no name":                          {line: "channel", want: "no channel"},
		"an assume beside it":              {line: "channel put\n  assume x", want: "assume"},
		"a parenthesis left open":          {line: "channel put : (K ⇸ V", want: "no type"},
		"text after the name and no colon": {line: "channel put ID", want: "expected"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseAccepting("@startuml\n[*] --> s0\nnote as channels\n  " + c.line + "\nend note\n@enduml\n")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("parseAccepting() error = %v, want one containing %q", err, c.want)
			}
		})
	}
}
