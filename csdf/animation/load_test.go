package animation

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/google/go-cmp/cmp"
)

func TestReadDiagramReadsTheSourceOrTheJSONInItsPlace(t *testing.T) {
	promoted, err := core.MarshalDiagram(promotedDiagram())
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		source       string
		diagramJSON  []byte
		wantStates   []string
		wantFamilies int
	}{
		"a diagram of the new builtins": {
			source:     builtinsDiagram,
			wantStates: []string{"s0"},
		},
		"the JSON of an expansion, whose families the source cannot say": {
			source:       "not read",
			diagramJSON:  promoted,
			wantStates:   []string{"G"},
			wantFamilies: 1,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			d, err := ReadDiagram([]byte(tt.source), tt.diagramJSON)

			// Assert
			if err != nil {
				t.Fatalf("ReadDiagram() error = %v", err)
			}
			if diff := cmp.Diff(tt.wantStates, d.States); diff != "" {
				t.Errorf("states mismatch (-want +got):\n%s", diff)
			}
			if len(d.Families) != tt.wantFamilies {
				t.Errorf("families = %d, want %d", len(d.Families), tt.wantFamilies)
			}
		})
	}
}

func TestReadDiagramRefusesWhatCoreioRefuses(t *testing.T) {
	tests := map[string]struct {
		source      string
		diagramJSON []byte
		want        string
	}{
		"a tool word of the old vocabulary": {
			source: "@startuml\nstate \"S\" as s\n[*] --> s\ns --> s : a ; @opaque\n@enduml\n",
			want:   "coreio.ReadBytes: coreio.Read:",
		},
		"a renaming of the old syntax": {
			source: "@startuml\nstate \"S\" as s\ns : n\n[*] --> s : @[y := n] y = 0\n@enduml\n",
			want:   "coreio.ReadBytes: coreio.Read:",
		},
		"JSON that is no diagram": {
			source:      builtinsDiagram,
			diagramJSON: []byte(`{"version": 0}`),
			want:        "core.UnmarshalDiagram:",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := ReadDiagram([]byte(tt.source), tt.diagramJSON)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ReadDiagram() error = %v, want one containing %q", err, tt.want)
			}
		})
	}
}
