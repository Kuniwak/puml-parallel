package coreio

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	global := `@startuml
state "稼働中" as running {
  state "shipments : (s : s) ⇸ Shipment" as rs <<promote>> {
    !include local/SHIPMENT.puml
  }
}
[*] --> running
@enduml
`
	tool := `@startuml
state "a" as a
[*] --> a
a --> a : tau ; @exists(y : Y) ($y < 1)
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
			name: "a diagram a tool wrote, which may hold ∃",
			path: "TOOL.puml",
			load: func(path string) (string, error) {
				d, err := Load(path)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%d edges", len(d.Edges)), nil
			},
			want: "1 edges",
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
			for path, src := range map[string]string{"G.puml": global, "local/SHIPMENT.puml": shipment, "TOOL.puml": tool} {
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

func TestLoadAll(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		paths   []string
		want    []int
		wantErr string
	}{
		{
			name:  "no files",
			paths: nil,
			want:  []int{},
		},
		{
			name: "each file in the order given",
			files: map[string]string{
				"A.puml": "@startuml\nstate \"a\" as a\n[*] --> a\na --> a : x\n@enduml\n",
				"B.puml": "@startuml\nstate \"b\" as b\n[*] --> b\n@enduml\n",
			},
			paths: []string{"B.puml", "A.puml"},
			want:  []int{0, 1},
		},
		{
			name:    "a file that is missing",
			paths:   []string{"MISSING.puml"},
			wantErr: "cannot parse diagrams",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			dir := t.TempDir()
			for path, src := range c.files {
				if err := os.WriteFile(filepath.Join(dir, path), []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			paths := make([]string, len(c.paths))
			for i, p := range c.paths {
				paths[i] = filepath.Join(dir, p)
			}

			// Execute
			got, err := LoadAll(paths)

			// Assert
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("LoadAll: got %v, want an error containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			edges := make([]int, len(got))
			for i, d := range got {
				edges[i] = len(d.Edges)
			}
			if !slices.Equal(edges, c.want) {
				t.Errorf("LoadAll: edge counts %v, want %v", edges, c.want)
			}
		})
	}
}

func TestReadExpandedBytes(t *testing.T) {
	// Setup
	plain := "@startuml\nstate \"S\" as s\n[*] --> s\n@enduml\n"
	global := `@startuml G
state "Run" as run {
  run : items ; ID ⇸ Item
  state "items : (ID : ID) ⇸ Item" as runItems <<promote>> {
    !include item.puml
  }
}
[*] --> run
@enduml
`
	local := `@startuml item
state "none" as itNone
state "made" as itMade
itMade : n ; n
[*] --> itNone : 準備済み
itNone --> itMade : MAKE ; ; $n' は 0
@enduml
`
	cases := map[string]struct {
		raw          string
		sources      map[string]string
		wantFamilies []string
		wantStates   []string
		wantWarnings int
		wantErr      string
	}{
		"a diagram without promotion directives reads as ReadBytes reads it": {
			raw: plain, wantStates: []string{"s"},
		},
		"a promotion reads as its expansion, families included, with its warnings": {
			raw: global, sources: map[string]string{"item.puml": local},
			wantStates: []string{"run"}, wantFamilies: []string{"items"}, wantWarnings: 1,
		},
		"a promotion whose include cannot be read is refused": {
			raw: global, wantErr: "item.puml",
		},
		"a plain diagram that does not parse is refused with the parse error": {
			raw: "@startuml\n[*] --> \n@enduml\n", wantErr: "coreio.ReadExpandedBytes: coreio.ReadBytes:",
		},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Execute
			d, warnings, err := ReadExpandedBytes([]byte(tt.raw), SourcesIncluder(tt.sources))

			// Assert
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if !slices.Equal(d.States, tt.wantStates) {
				t.Errorf("states = %v, want %v", d.States, tt.wantStates)
			}
			var families []string
			for _, f := range d.Families {
				families = append(families, f.Map)
			}
			if !slices.Equal(families, tt.wantFamilies) {
				t.Errorf("families = %v, want %v", families, tt.wantFamilies)
			}
			if len(warnings) != tt.wantWarnings {
				t.Errorf("warnings = %v, want %d", warnings, tt.wantWarnings)
			}
		})
	}
}
