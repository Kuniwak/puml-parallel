package csdfreplwebcmd

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/replweb"
	"github.com/Kuniwak/puml-parallel/v3/tools/replweb/engineload"
	"github.com/Kuniwak/puml-parallel/v3/usererr"
	"github.com/google/go-cmp/cmp"
)

const vendingMachine = "../../../examples/valid/vending_machine.puml"

// fakeSVG is a drawing whose start edge is the one vending_machine.puml
// reads on its 15th line.
const fakeSVG = `<svg xmlns="http://www.w3.org/2000/svg"><g class="link" data-entity-1="ent0009" data-entity-2="ent0001" data-source-line="14" id="lnk10"><path/></g></svg>`

// fakeEngine gives an engine without building one.
func fakeEngine(*Options) engineload.Source {
	return func() (replweb.Engine, error) { return replweb.Engine{WasmGz: []byte("engine")}, nil }
}

func TestMainWritesThePageOfTheDiagram(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	var gotArgv []string
	var gotDir, gotSource string
	render := func(argv []string, dir string, source []byte) ([]byte, error) {
		gotArgv, gotDir, gotSource = argv, dir, string(source)
		return []byte(fakeSVG), nil
	}
	opts := &Options{Common: &tools.CommonOptions{}, File: vendingMachine, PumlExec: []string{"plantuml", "-pipe", "-tsvg"}}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"plantuml", "-pipe", "-tsvg"}, gotArgv); diff != "" {
		t.Errorf("argv mismatch (-want +got):\n%s", diff)
	}
	if gotDir != filepath.Dir(vendingMachine) || !strings.HasPrefix(gotSource, "@startuml") {
		t.Errorf("render ran in %q on %q", gotDir, gotSource)
	}
	out := spy.Stdout.String()
	if !strings.Contains(out, fakeSVG) || !strings.Contains(out, `"start":"lnk10"`) || !strings.Contains(out, `"vmIdle":"ent0001"`) {
		t.Error("the page does not hold the drawing and its highlights")
	}
}

func TestMainDrawsAPromotionWithoutHighlights(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte(fakeSVG), nil }
	opts := &Options{Common: &tools.CommonOptions{}, File: "../../../examples/promote/MEMBERS.puml", PumlExec: []string{"x"}}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(spy.Stdout.String(), `"highlighted":false`) {
		t.Error("the page highlights a promotion")
	}
}

func TestMainRefusesADiagramItCannotRead(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte(fakeSVG), nil }
	opts := &Options{Common: &tools.CommonOptions{}, File: "no-such.puml", PumlExec: []string{"x"}}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err == nil {
		t.Error("want an error, got nil")
	}
}

func TestRunPlantUMLRendersSVG(t *testing.T) {
	if _, err := exec.LookPath("plantuml"); err != nil {
		t.Skip("plantuml is not on the PATH")
	}
	// Execute
	svg, err := RunPlantUML([]string{"plantuml", "-pipe", "-tsvg"}, ".", []byte("@startuml\nstate a\n[*] --> a\n@enduml\n"))

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(svg), "<svg") {
		t.Errorf("output = %.100q, want SVG", svg)
	}
}

func TestRunPlantUMLSaysWhatTheCommandWrote(t *testing.T) {
	// Execute
	_, err := RunPlantUML([]string{"sh", "-c", "echo broken >&2; exit 3"}, ".", nil)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("error = %v, want one holding the standard error", err)
	}
}

func TestMainRefusesWhenThereIsNoEngine(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte(fakeSVG), nil }
	noEngine := func(*Options) engineload.Source {
		return func() (replweb.Engine, error) { return replweb.Engine{}, errors.New("no engine") }
	}
	opts := &Options{Common: &tools.CommonOptions{}, File: vendingMachine, PumlExec: []string{"x"}}

	// Execute
	err := newMainFunc(render, noEngine)(opts, spy.New())

	// Assert
	if err == nil || spy.Stdout.Len() != 0 {
		t.Errorf("error = %v and %d bytes written, want an error and nothing written", err, spy.Stdout.Len())
	}
}

func TestMainSaysWhatToDoWithoutGo(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte(fakeSVG), nil }
	noGo := func(*Options) engineload.Source {
		return func() (replweb.Engine, error) { return replweb.Engine{}, engineload.ErrNoGo }
	}
	opts := &Options{Common: &tools.CommonOptions{}, File: vendingMachine, PumlExec: []string{"x"}}

	// Execute
	err := newMainFunc(render, noGo)(opts, spy.New())

	// Assert
	if msg := usererr.Message(err); !strings.Contains(msg, "-engine") || !strings.Contains(msg, "release") {
		t.Errorf("message = %q, want one naming -engine and the release binary", msg)
	}
}
