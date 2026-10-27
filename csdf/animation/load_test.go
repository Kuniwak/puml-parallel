package animation

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/coreio"
	"github.com/google/go-cmp/cmp"
)

func TestReadDiagram(t *testing.T) {
	// Setup
	promotion, err := os.ReadFile("../../examples/promote/MEMBERS.puml")
	if err != nil {
		t.Fatalf("Setup: os.ReadFile() error = %v", err)
	}
	tests := map[string]struct {
		source       string
		wantStates   []string
		wantFamilies int
		wantErr      string
	}{
		"a diagram of the new builtins": {
			source:     builtinsDiagram,
			wantStates: []string{"s0"},
		},
		"a promotion is read as its expansion, families included": {
			source:       string(promotion),
			wantStates:   []string{"running"},
			wantFamilies: 1,
		},
		"a tool word of the old vocabulary is refused as coreio refuses it": {
			source:  "@startuml\nstate \"S\" as s\n[*] --> s\ns --> s : a ; @opaque\n@enduml\n",
			wantErr: "coreio.ReadBytes: coreio.Read:",
		},
		"a renaming of the old syntax is refused as coreio refuses it": {
			source:  "@startuml\nstate \"S\" as s\ns : n ; n\n[*] --> s : @[y := n] y = 0\n@enduml\n",
			wantErr: "coreio.ReadBytes: coreio.Read:",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			d, _, err := ReadDiagram([]byte(tt.source), coreio.FileIncluder("../../examples/promote"))

			// Assert
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ReadDiagram() error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
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
