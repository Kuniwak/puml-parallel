package csdfreplwebcmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSplitCommandLine(t *testing.T) {
	tests := map[string]struct {
		line    string
		want    []string
		wantErr bool
	}{
		"words":                {line: "plantuml -pipe -tsvg", want: []string{"plantuml", "-pipe", "-tsvg"}},
		"runs of blanks":       {line: "  plantuml \t -pipe  ", want: []string{"plantuml", "-pipe"}},
		"double quotes":        {line: `java -jar "my dir/plantuml.jar"`, want: []string{"java", "-jar", "my dir/plantuml.jar"}},
		"single quotes":        {line: `java -jar 'a "b" c.jar'`, want: []string{"java", "-jar", `a "b" c.jar`}},
		"escapes":              {line: `a\ b "c\"d" e\\f`, want: []string{"a b", `c"d`, `e\f`}},
		"a quoted empty word":  {line: `a ""`, want: []string{"a", ""}},
		"quotes inside a word": {line: `-D"x y"z`, want: []string{"-Dx yz"}},
		"nothing":              {line: "   ", wantErr: true},
		"an unclosed quote":    {line: `java "a`, wantErr: true},
		"a trailing backslash": {line: `java \`, wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := SplitCommandLine(tt.line)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Errorf("SplitCommandLine(%q) = %q, want an error", tt.line, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SplitCommandLine(%q) mismatch (-want +got):\n%s", tt.line, diff)
			}
		})
	}
}
