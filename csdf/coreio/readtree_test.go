package coreio

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v5/csdf"
)

func TestReadTreeJSONTellsJSONByItsOpeningBrace(t *testing.T) {
	// Setup
	tree, err := csdf.Parse("", "@startuml\nstate \"S\" as s0\n[*] --> s0\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}
	bs, err := csdf.MarshalSyntaxTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		raw     string
		wantErr string
	}{
		"JSON":                       {raw: string(bs)},
		"JSON after whitespace":      {raw: " \n\t" + string(bs)},
		"JSON after a UTF-8 BOM":     {raw: "\xef\xbb\xbf" + string(bs)},
		"PlantUML after a UTF-8 BOM": {raw: "\xef\xbb\xbf@startuml\nstate \"S\" as s0\n[*] --> s0\n@enduml\n", wantErr: "not PlantUML; run csdfparse on the diagram first"},
		"JSON of another kind":       {raw: `{"version":3,"name":""}`, wantErr: `want kind "csdf-syntax-tree" version 1, got kind "" version 3`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := ReadTreeJSON([]byte(c.raw))

			// Assert
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("got %v, want an error containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Body) != 2 {
				t.Errorf("body = %v, want the state and the start edge", got.Body)
			}
		})
	}
}
