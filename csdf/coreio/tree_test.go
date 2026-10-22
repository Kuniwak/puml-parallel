package coreio

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/google/go-cmp/cmp"
)

func TestComposeTreeEvaluatesATreeOfFormulas(t *testing.T) {
	// Setup: a producer puts a value its consumer takes, the put hidden.
	diagrams := map[string]string{
		"p.puml": "@startuml\nstate \"p\" as p\np : n\n[*] --> p : $n' = 0\np --> p : put!n ; ; $n' = $n + 1\n@enduml\n",
		"c.puml": "@startuml\nstate \"c\" as c\nc : m\n[*] --> c : $m' = 0\nc --> c : put?k ; $k > $m ; @eq(m', k)\n@enduml\n",
	}
	load := func(path string) (*core.Diagram, error) {
		src, ok := diagrams[path]
		if !ok {
			return nil, fmt.Errorf("no %s", path)
		}
		return Read(src)
	}
	expr, err := csdf.ParseExpr([]byte(`{"op": "HIDE", "channels": ["put"], "proc":
		{"op": "INTERFACE_PARALLEL", "sync": ["put"], "procs": [{"op": "REFER", "path": "p.puml"}, {"op": "REFER", "path": "c.puml"}]}}`))
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	d, err := ComposeTree(expr, load)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := Canonical(d)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Write(canonical)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	// The consumer's k is the value the producer sends, so hiding put binds
	// nothing new.
	want := "@startuml\nstate \"(p, c)\" as p_c\np_c : n\np_c : m\n[*] --> p_c : $n' = 0 ∧ $m' = 0\n" +
		"p_c --> p_c : tau ; $n > $m ; $n > $m ∧ $n' = $n + 1 ∧ @eq(m', n)\n@enduml\n"
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestComposeTreeNamesTheLeafItCannotLoad(t *testing.T) {
	// Setup
	expr, err := csdf.ParseExpr([]byte(`{"op": "REFER", "path": "missing.puml"}`))
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	_, err = ComposeTree(expr, func(path string) (*core.Diagram, error) { return nil, fmt.Errorf("no %s", path) })

	// Assert
	if err == nil {
		t.Error("err = nil, want the missing leaf named")
	}
}

func TestComposeTreeIsThePipelineOfItsTools(t *testing.T) {
	// The tree, read from a file as csdfcomp reads it, against what
	// csdfparallel writes, read back, then hidden as csdfhide does.
	testCases := map[string]struct {
		Tree     string
		Base     string
		Files    []string
		Sync     []string
		Channels []string
	}{
		"in_out_tree.json": {
			Tree: "../../examples/valid/in_out_tree.json", Base: "../../examples/valid",
			Files: []string{"in.puml", "out.puml"}, Sync: []string{"sync"}, Channels: []string{"sync"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			src, err := os.ReadFile(tc.Tree)
			if err != nil {
				t.Fatal(err)
			}
			expr, err := csdf.ParseExpr(src)
			if err != nil {
				t.Fatal(err)
			}
			var ds []*core.Diagram
			for _, f := range tc.Files {
				d, err := Load(filepath.Join(tc.Base, f))
				if err != nil {
					t.Fatal(err)
				}
				ds = append(ds, d)
			}
			composed, err := core.ComposeAll(ds, tc.Sync)
			if err != nil {
				t.Fatal(err)
			}
			want := tools(t, composed)
			readBack, err := Read(want)
			if err != nil {
				t.Fatal(err)
			}
			hidden, err := core.Hide(readBack, tc.Channels)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			got, err := ComposeTree(expr, FileLoader(tc.Base))

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tools(t, hidden), tools(t, got)); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestComposeTreeFoldsMoreThanTwoProcs(t *testing.T) {
	// Setup: three processes synchronised on the one event they share.
	load := func(path string) (*core.Diagram, error) {
		return Read("@startuml\nstate \"s0\" as s0\nstate \"s1\" as s1\n[*] --> s0\ns0 --> s1 : sync\n@enduml\n")
	}
	expr, err := csdf.ParseExpr([]byte(`{"op": "INTERFACE_PARALLEL", "sync": ["sync"], "procs": [{"op": "REFER", "path": "A"}, {"op": "REFER", "path": "B"}, {"op": "REFER", "path": "C"}]}`))
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	got, err := ComposeTree(expr, load)

	// Assert: the three can only move together.
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Edges) != 1 || len(got.States) != 2 {
		t.Errorf("want 1 edge between 2 states, got %s", tools(t, got))
	}
}

func TestFileLoaderResolvesARelativePathAgainstItsBase(t *testing.T) {
	testCases := map[string]string{
		"a relative path":  "in.puml",
		"an absolute path": func() string { p, _ := filepath.Abs("../../examples/valid/in.puml"); return p }(),
	}
	for name, path := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			d, err := FileLoader("../../examples/valid")(path)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if d.Init != "s0" {
				t.Errorf("read %v, want in.puml", d)
			}
		})
	}
}

// tools is d as the tools write it.
func tools(t *testing.T, d *core.Diagram) string {
	t.Helper()
	canonical, err := Canonical(d)
	if err != nil {
		t.Fatal(err)
	}
	text, err := Write(canonical)
	if err != nil {
		t.Fatal(err)
	}
	return text
}
