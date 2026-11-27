package coreio

import (
	"slices"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v9/csdf"
)

func TestExpandTree(t *testing.T) {
	cases := map[string]struct {
		source     string
		keep       []string
		unresolved bool // the tree is not given to ResolveIncludes
		wantStates []string
		wantErr    string
	}{
		"a plain diagram is lowered as it is": {
			source:     "@startuml\nstate \"S\" as s0\n[*] --> s0\ns0 --> [*]\n@enduml\n",
			wantStates: []string{"s0"},
		},
		"a global diagram is expanded": {
			source:     promoting("[*] --> running\n"),
			wantStates: []string{"running"},
		},
		"an unresolved !include is refused, naming csdfinclude": {
			source:     promoting("[*] --> running\n"),
			unresolved: true,
			wantErr:    "!include S.puml is unresolved in the syntax tree; resolve it by csdfinclude",
		},
		"-only on a plain diagram is refused": {
			source:  "@startuml\nstate \"S\" as s0\n[*] --> s0\n@enduml\n",
			keep:    []string{"orders", "items"},
			wantErr: "orders, items is no promoted map",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			parsed, err := csdf.Parse("", c.source)
			if err != nil {
				t.Fatal(err)
			}
			tree := parsed
			if !c.unresolved {
				tree, err = ResolveIncludes(parsed, files(map[string]string{"S.puml": shipment}))
				if err != nil {
					t.Fatal(err)
				}
			}

			// Execute
			d, _, err := ExpandTree(tree, c.keep)

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
			if !slices.Equal(d.States, c.wantStates) {
				t.Errorf("states = %v, want %v", d.States, c.wantStates)
			}
		})
	}
}
