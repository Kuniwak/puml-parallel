package coreio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPromotionIncludesRelativeToTheFile(t *testing.T) {
	// Setup
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	global := `@startuml
state "稼働中" as running {
  state "shipments : s" as rs <<promote>> {
    !include local/SHIPMENT.puml
  }
}
[*] --> running
note as n
  create ORDER : shipments
end note
@enduml
`
	for path, src := range map[string]string{"G.puml": global, "local/SHIPMENT.puml": shipment} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Execute
	p, err := LoadPromotion(filepath.Join(dir, "G.puml"))

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Families[0].Local.Init; got != "waiting" {
		t.Errorf("the local diagram starts at %q, want waiting", got)
	}
}

func TestLoadAuthoredReadsAFile(t *testing.T) {
	// Setup
	path := filepath.Join(t.TempDir(), "S.puml")
	if err := os.WriteFile(path, []byte(shipment), 0o644); err != nil {
		t.Fatal(err)
	}

	// Execute
	d, err := LoadAuthored(path)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Edges) != 1 {
		t.Errorf("read %d edges, want 1", len(d.Edges))
	}
}
