package coreio

import (
	"fmt"
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
