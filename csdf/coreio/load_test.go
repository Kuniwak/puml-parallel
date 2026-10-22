package coreio

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	global := `@startuml
state "稼働中" as running {
  state "shipments : s ⇸ Shipment" as rs <<promote>> {
    !include local/SHIPMENT.puml
  }
}
[*] --> running
@enduml
`
	cases := []struct {
		name string
		path string
		// load loads the file at path and says what was read of it.
		load func(path string) (string, error)
		want string
	}{
		{
			name: "a diagram an author wrote",
			path: "local/SHIPMENT.puml",
			load: func(path string) (string, error) {
				d, err := LoadAuthored(path)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%d edges", len(d.Edges)), nil
			},
			want: "2 edges",
		},
		{
			name: "a global diagram, its include read relative to it",
			path: "G.puml",
			load: func(path string) (string, error) {
				p, err := LoadPromotion(path)
				if err != nil {
					return "", err
				}
				return "starts at " + p.Families[0].Local.Init, nil
			},
			want: "starts at none",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "local"), 0o755); err != nil {
				t.Fatal(err)
			}
			for path, src := range map[string]string{"G.puml": global, "local/SHIPMENT.puml": shipment} {
				if err := os.WriteFile(filepath.Join(dir, path), []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			// Execute
			got, err := c.load(filepath.Join(dir, c.path))

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("read %q, want %q", got, c.want)
			}
		})
	}
}
