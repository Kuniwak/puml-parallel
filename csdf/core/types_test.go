package core

import (
	"strings"
	"testing"
)

func TestParseType(t *testing.T) {
	tests := map[string]struct {
		text    string
		want    string
		wantErr string
	}{
		"a named type":                  {text: "ID", want: "ID"},
		"a Unicode name":                {text: "口座ID", want: "口座ID"},
		"any":                           {text: "any", want: "any"},
		"a map type":                    {text: "K ⇸ V", want: "K ⇸ V"},
		"a map type written in ASCII":   {text: "K->>V", want: "K ⇸ V"},
		"a map type nests to the right": {text: "K ⇸ L ⇸ V", want: "K ⇸ L ⇸ V"},
		"a map type as a key":           {text: "(K ⇸ L) ⇸ V", want: "(K ⇸ L) ⇸ V"},
		"redundant parentheses":         {text: "((K)) ⇸ (V)", want: "K ⇸ V"},
		"the empty text":                {text: "", wantErr: "no type"},
		"no identifier":                 {text: "a-b", wantErr: "no type"},
		"an unclosed parenthesis":       {text: "(K ⇸ V", wantErr: "no type"},
		"a missing value type":          {text: "K ⇸", wantErr: "no type"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := ParseType(tt.text)

			// Assert
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseType(%q) = %v, %v; want an error containing %q", tt.text, got, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseType(%q): %v", tt.text, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseType(%q) = %s, want %s", tt.text, got, tt.want)
			}
		})
	}
}

func TestTypeParts(t *testing.T) {
	// Setup
	m := MapOf(Named("K"), MapOf(Named("L"), AnyType))

	// Assert
	if !m.IsMap() || m.Key().String() != "K" || m.Val().String() != "L ⇸ any" {
		t.Errorf("the parts of %s are %s and %s", m, m.Key(), m.Val())
	}
	if !m.Val().Val().IsAny() || Named("K").IsAny() || Named("K").IsMap() {
		t.Errorf("IsAny and IsMap tell the kinds of type apart wrongly")
	}
	if !m.Equal(MapOf(Named("K"), MapOf(Named("L"), Named(AnyName)))) || m.Equal(Named("K")) {
		t.Errorf("Equal is not equality of the written types")
	}
}
