package coreio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/google/go-cmp/cmp"
)

func TestComposeTree(t *testing.T) {
	diagrams := map[string]string{
		"p.puml":   "@startuml\nstate \"p\" as p\np : n\n[*] --> p : $n' = 0\np --> p : put!n ; ; $n' = $n + 1\n@enduml\n",
		"c.puml":   "@startuml\nstate \"c\" as c\nc : m\n[*] --> c : $m' = 0\nc --> c : put?k ; $k > $m ; @eq(m', k)\n@enduml\n",
		"in.puml":  "@startuml\nstate \"s0\" as s0\nstate \"s1\" as s1\nstate \"s2\" as s2\n[*] --> s0\ns0 --> s1 : in\ns1 --> s2 : sync\n@enduml\n",
		"out.puml": "@startuml\nstate \"s0\" as s0\nstate \"s1\" as s1\nstate \"s2\" as s2\n[*] --> s0\ns0 --> s1 : sync\ns1 --> s2 : out\n@enduml\n",
		"one.puml": "@startuml\nstate \"s0\" as s0\nstate \"s1\" as s1\n[*] --> s0\ns0 --> s1 : sync\n@enduml\n",
	}
	load := func(path string) (*core.Diagram, error) {
		src, ok := diagrams[path]
		if !ok {
			return nil, fmt.Errorf("no %s", path)
		}
		return Read(src)
	}
	testCases := map[string]struct {
		Expr string
		Want string
	}{
		// The consumer's k is the value the producer sends, so hiding put
		// binds nothing new.
		"a hidden channel of formulas": {
			Expr: `{"op": "HIDE", "channels": ["put"], "proc": {"op": "INTERFACE_PARALLEL", "sync": ["put"], "procs": [{"op": "REFER", "path": "p.puml"}, {"op": "REFER", "path": "c.puml"}]}}`,
			Want: "@startuml\nstate \"(p, c)\" as p_c\np_c : n\np_c : m\n[*] --> p_c : $n' = 0 ∧ $m' = 0\n" +
				"p_c --> p_c : tau ; $n > $m ; $n > $m ∧ $n' = $n + 1 ∧ @eq(m', n)\n@enduml\n",
		},
		"the tree of in_out_tree.json": {
			Expr: `{"op": "HIDE", "channels": ["sync"], "proc": {"op": "INTERFACE_PARALLEL", "sync": ["sync"], "procs": [{"op": "REFER", "path": "in.puml"}, {"op": "REFER", "path": "out.puml"}]}}`,
			Want: "@startuml\nstate \"(s0, s0)\" as s0_s0\nstate \"(s1, s0)\" as s1_s0\nstate \"(s2, s1)\" as s2_s1\nstate \"(s2, s2)\" as s2_s2\n" +
				"[*] --> s0_s0\ns0_s0 --> s1_s0 : in\ns1_s0 --> s2_s1 : tau\ns2_s1 --> s2_s2 : out\n@enduml\n",
		},
		"three processes, which can only move together": {
			Expr: `{"op": "INTERFACE_PARALLEL", "sync": ["sync"], "procs": [{"op": "REFER", "path": "one.puml"}, {"op": "REFER", "path": "one.puml"}, {"op": "REFER", "path": "one.puml"}]}`,
			Want: "@startuml\nstate \"((s0, s0), s0)\" as s0_s0_s0\nstate \"((s1, s1), s1)\" as s1_s1_s1\n" +
				"[*] --> s0_s0_s0\ns0_s0_s0 --> s1_s1_s1 : sync\n@enduml\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			expr, err := csdf.ParseExpr([]byte(tc.Expr))
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			got, err := ComposeTree(expr, load)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.Want, tools(t, got)); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestComposeTreeNamesTheLeafItCannotLoad(t *testing.T) {
	// Setup
	expr, err := csdf.ParseExpr([]byte(`{"op": "REFER", "path": "missing.puml"}`))
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	_, err = ComposeTree(expr, func(path string) (*core.Diagram, error) { return nil, fmt.Errorf("cannot read") })

	// Assert
	if err == nil || !strings.Contains(err.Error(), "missing.puml") {
		t.Errorf("err = %v, want missing.puml named", err)
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
