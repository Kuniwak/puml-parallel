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

// pipeline is the diagram the tools write for expr, run one by one as a
// shell would: csdfparallel for an INTERFACE_PARALLEL, its output read back,
// and csdfhide for a HIDE, each leaf loaded from base.
func pipeline(t *testing.T, base string, expr csdf.Expr) *core.Diagram {
	t.Helper()
	switch e := expr.(type) {
	case *csdf.ReferExpr:
		d, err := Load(filepath.Join(base, e.Path))
		if err != nil {
			t.Fatal(err)
		}
		return d
	case *csdf.HideExpr:
		d, err := core.Hide(pipeline(t, base, e.Proc), e.Channels)
		if err != nil {
			t.Fatal(err)
		}
		return d
	case *csdf.InterfaceParallelExpr:
		var ds []*core.Diagram
		for _, p := range e.Procs {
			ds = append(ds, pipeline(t, base, p))
		}
		composed, err := core.ComposeAll(ds, e.Sync)
		if err != nil {
			t.Fatal(err)
		}
		readBack, err := Read(tools(t, composed))
		if err != nil {
			t.Fatal(err)
		}
		return readBack
	}
	t.Fatalf("no expression %T", expr)
	return nil
}

func TestComposeTreeIsThePipelineOfItsTools(t *testing.T) {
	// Setup
	const base = "../../examples/valid"
	src, err := os.ReadFile(filepath.Join(base, "in_out_tree.json"))
	if err != nil {
		t.Fatal(err)
	}
	expr, err := csdf.ParseExpr(src)
	if err != nil {
		t.Fatal(err)
	}
	want := tools(t, pipeline(t, base, expr))

	// Execute
	got, err := ComposeTree(expr, FileLoader(base))

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, tools(t, got)); diff != "" {
		t.Error(diff)
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
	want := "@startuml\nstate \"((s0, s0), s0)\" as s0_s0_s0\nstate \"((s1, s1), s1)\" as s1_s1_s1\n" +
		"[*] --> s0_s0_s0\ns0_s0_s0 --> s1_s1_s1 : sync\n@enduml\n"
	if diff := cmp.Diff(want, tools(t, got)); diff != "" {
		t.Error(diff)
	}
}

func TestFileLoaderResolvesARelativePathAgainstItsBase(t *testing.T) {
	// Setup: out.puml is in the base, and not in the directory of the test.
	const base = "../../examples/valid"
	abs, err := filepath.Abs(filepath.Join(base, "out.puml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("out.puml"); err == nil {
		t.Fatal("out.puml is in the directory of the test, so a loader ignoring its base would pass")
	}
	want, err := Load(abs)
	if err != nil {
		t.Fatal(err)
	}
	testCases := map[string]string{"a relative path": "out.puml", "an absolute path": abs}

	for name, path := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := FileLoader(base)(path)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tools(t, want), tools(t, got)); diff != "" {
				t.Error(diff)
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
