package csdfreplwebcmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v6/cli"
	"github.com/Kuniwak/puml-parallel/v6/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/tools/replweb"
	"github.com/Kuniwak/puml-parallel/v6/tools/replweb/engineload"
	"github.com/Kuniwak/puml-parallel/v6/usererr"
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

// fakePromotionSVG is a drawing of MEMBERS.puml whose block draws the local
// state memActive.
const fakePromotionSVG = `<svg xmlns="http://www.w3.org/2000/svg"><g class="entity" data-qualified-name="running.runningMembers.memActive" id="ent0004"><rect/></g></svg>`

func TestMainHighlightsTheLocalDiagramsOfAPromotion(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte(fakePromotionSVG), nil }
	opts := &Options{Common: &tools.CommonOptions{}, File: "../../../examples/promote/MEMBERS.puml", PumlExec: []string{"x"}}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(spy.Stdout.String(), `"families":{"members":{"states":{"memActive":"ent0004"}`) {
		t.Error("the page does not highlight the local diagram of members")
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

const printerFormal = "../../../examples/bmc/printer_formal.puml"
const printerDefs = "../../../examples/bmc/printer_formal_defs.py"

func TestMainCarriesTheCheckerAndTheFilesItNames(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte("<svg></svg>"), nil }
	checker := bmc.Checker{Checks: printerDefs, EvaluatorModule: "csdfbmc.python_glue " + printerDefs}
	opts := &Options{Common: &tools.CommonOptions{}, File: printerFormal, PumlExec: []string{"x"}, Checker: checker, Pyodide: "https://example.com/py/"}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	out := spy.Stdout.String()
	for _, want := range []string{`"url":"https://example.com/py/"`, `"evaluator_module":"csdfbmc.python_glue ../../../examples/bmc/printer_formal_defs.py"`,
		`"/examples/bmc/printer_formal_defs.py":"\"\"\"The domains of examples/bmc/printer_formal.puml`} {
		if !strings.Contains(out, want) {
			t.Errorf("the page does not hold %s", want)
		}
	}
}

func TestMainCarriesNoPythonWithoutAChecker(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte("<svg></svg>"), nil }
	opts := &Options{Common: &tools.CommonOptions{}, File: printerFormal, PumlExec: []string{"x"}, Pyodide: DefaultPyodide}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(spy.Stdout.String(), `"python":`) {
		t.Error("the page carries Python without a checker")
	}
}

func TestMainFailsOnAChecksModuleItCannotRead(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte("<svg></svg>"), nil }
	checker := bmc.Checker{Checks: "no/such/checks.py", EvaluatorModule: "csdfbmc.python_glue"}
	opts := &Options{Common: &tools.CommonOptions{}, File: printerFormal, PumlExec: []string{"x"}, Checker: checker, Pyodide: DefaultPyodide}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err == nil || !strings.Contains(err.Error(), "no/such/checks.py") {
		t.Errorf("err = %v, want one naming the checks module", err)
	}
}

func TestMainCarriesTheShapes(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte("<svg></svg>"), nil }
	shapes := filepath.Join(t.TempDir(), "shapes.json")
	if err := os.WriteFile(shapes, []byte(`{"types": {"Nat": {"kind": "scalar", "numeric": true}}, "vars": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := &Options{Common: &tools.CommonOptions{}, File: printerFormal, PumlExec: []string{"x"}, Pyodide: DefaultPyodide, Shapes: shapes}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if want := `"shapes":{"types":{"Nat":{"kind":"scalar","numeric":true}},"vars":{}}`; !strings.Contains(spy.Stdout.String(), want) {
		t.Errorf("the page does not hold %s", want)
	}
}

func TestMainRefusesShapesThatAreNoJSON(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte("<svg></svg>"), nil }
	shapes := filepath.Join(t.TempDir(), "shapes.json")
	if err := os.WriteFile(shapes, []byte(`{"types":`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := &Options{Common: &tools.CommonOptions{}, File: printerFormal, PumlExec: []string{"x"}, Pyodide: DefaultPyodide, Shapes: shapes}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err == nil || !strings.Contains(err.Error(), "shapes.json") {
		t.Errorf("err = %v, want one naming the shapes file", err)
	}
}

// writeProse writes printer_formal.puml with its clauses replaced as
// replacements say, as printer_prose.puml, and returns its path.
func writeProse(t *testing.T, replacements ...string) string {
	t.Helper()
	return writeProseAs(t, "printer_prose.puml", replacements...)
}

// writeProseAs is writeProse writing the file name.
func writeProseAs(t *testing.T, name string, replacements ...string) string {
	t.Helper()
	bs, err := os.ReadFile(printerFormal)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(strings.NewReplacer(replacements...).Replace(string(bs))), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMainCarriesTheClausesOfTheProse(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte("<svg></svg>"), nil }
	prose := writeProse(t, "$用紙' == INITIAL", "用紙は初期枚数")
	opts := &Options{Common: &tools.CommonOptions{}, File: printerFormal, PumlExec: []string{"x"}, Pyodide: DefaultPyodide, Prose: map[string]string{"ja": prose}}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if want := `"prose":{"ja":{"init_post":"用紙は初期枚数"`; !strings.Contains(spy.Stdout.String(), want) {
		t.Errorf("the page does not hold %s", want)
	}
}

func TestMainRefusesAProseOfALanguageWithoutPhrases(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte("<svg></svg>"), nil }
	prose := writeProse(t, "$用紙' == INITIAL", "用紙は初期枚数")
	opts := &Options{Common: &tools.CommonOptions{}, File: printerFormal, PumlExec: []string{"x"}, Pyodide: DefaultPyodide, Prose: map[string]string{"fr": prose}}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if msg := usererr.Message(err); !strings.Contains(msg, `"fr"`) || spy.Stdout.Len() != 0 {
		t.Errorf("message = %q, want one naming the language, and nothing written", msg)
	}
}

func TestMainRefusesAProseOfAnotherDiagram(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte("<svg></svg>"), nil }
	prose := writeProse(t, "printing --> idle : 排紙", "printing --> printing : 排紙")
	opts := &Options{Common: &tools.CommonOptions{}, File: printerFormal, PumlExec: []string{"x"}, Pyodide: DefaultPyodide, Prose: map[string]string{"ja": prose}}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if msg := usererr.Message(err); !strings.Contains(msg, "printing --> idle") || !strings.Contains(msg, prose) || spy.Stdout.Len() != 0 {
		t.Errorf("message = %q, want one naming the prose and its edge, and nothing written", msg)
	}
}

func TestMainCarriesTheProseOfEachLanguageByTheLanguageGivenNotItsFileName(t *testing.T) {
	// Setup: each file is named after the other language
	spy := cli.SpyProcInout()
	render := func([]string, string, []byte) ([]byte, error) { return []byte("<svg></svg>"), nil }
	en := writeProseAs(t, "printer_ja.puml", "$用紙' == INITIAL", "the tray holds the initial sheets")
	ja := writeProseAs(t, "printer_en.puml", "$用紙' == INITIAL", "用紙は初期枚数")
	opts := &Options{Common: &tools.CommonOptions{}, File: printerFormal, PumlExec: []string{"x"}, Pyodide: DefaultPyodide, Prose: map[string]string{"en": en, "ja": ja}}

	// Execute
	err := newMainFunc(render, fakeEngine)(opts, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"en":{"init_post":"the tray holds the initial sheets"`, `"ja":{"init_post":"用紙は初期枚数"`} {
		if !strings.Contains(spy.Stdout.String(), want) {
			t.Errorf("the page does not hold %s", want)
		}
	}
}
