package irjson

import (
	"bytes"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/google/go-cmp/cmp"
)

// TestWriteLivelockFreePinsTheWireForm pins the wire form itself: each clause
// is the syntax tree of its formula, and the text of an atom is written as it
// is, <, > and & included.
func TestWriteLivelockFreePinsTheWireForm(t *testing.T) {
	// Setup
	d, err := coreio.Read("@startuml\nstate \"a\" as a\na : n\n[*] --> a : $n' = 10\na --> a : tau ; $n > 0 ; $n' = $n - 1\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}
	ir, err := obligationir.BuildLivelockFree(d)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer

	// Execute
	err = WriteLivelockFree(&buf, ir)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"structurally":false,"states":[{"name":"a","vars":["n"]}],"init":"a","init_post":{"op":"atom","name":"$_0 = 10","quoted":true,"terms":[{"op":"var","name":"n'"}]},"edges":[{"src":"a","dst":"a","channel":"tau","fields":[],"step":{"op":"and","operands":[{"op":"atom","name":"$_0 > 0","quoted":true,"terms":[{"op":"var","name":"n"}]},{"op":"atom","name":"$_0 = $_1 - 1","quoted":true,"terms":[{"op":"var","name":"n'"},{"op":"var","name":"n"}]}]}}],"ends":[],"axioms":[],"assumptions":[]}
`
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Error(diff)
	}
}
