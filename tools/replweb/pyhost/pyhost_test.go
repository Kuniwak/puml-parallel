package pyhost_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v6/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v6/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/v6/csdf/core"
	"github.com/Kuniwak/puml-parallel/v6/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v6/tools/replweb/pyhost"
	"github.com/google/go-cmp/cmp"
)

// fakeHost keeps files in a map and answers every run with answer.
type fakeHost struct {
	files  map[string]string
	ran    [][]string
	answer pyhost.Result
	err    error
}

func newFakeHost(answer pyhost.Result) *fakeHost {
	return &fakeHost{files: map[string]string{}, answer: answer}
}

func (h *fakeHost) WriteFile(path string, data []byte) error {
	h.files[path] = string(data)
	return nil
}

func (h *fakeHost) Remove(path string) error {
	delete(h.files, path)
	return nil
}

func (h *fakeHost) Run(args []string) (pyhost.Result, error) {
	h.ran = append(h.ran, args)
	return h.answer, h.err
}

var testChecker = bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue c.py"}

func testDiagram() *core.Diagram {
	d, err := coreio.ReadBytes([]byte("@startuml\nstate \"S\" as s0\ns0 : n ; n\n[*] --> s0 : @eq(n', 0)\n@enduml\n"))
	if err != nil {
		panic(err)
	}
	return d
}

func TestSuggestersAskNothingWithoutAHost(t *testing.T) {
	// Setup
	factory := pyhost.Suggesters(func() (pyhost.Host, bmc.Checker, bool) { return nil, bmc.Checker{}, false })

	// Execute
	s, _, err := factory(testDiagram())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Initial(); !errors.Is(err, animation.ErrNoSuggester) {
		t.Errorf("Initial() error = %v, want ErrNoSuggester", err)
	}
}

func TestSuggestersRunCsdfbmcNextInTheHostOnTheDiagramItKeeps(t *testing.T) {
	// Setup
	host := newFakeHost(pyhost.Result{Stdout: `{"candidates":[{"params":{},"values":[3]}]}`})
	factory := pyhost.Suggesters(func() (pyhost.Host, bmc.Checker, bool) { return host, testChecker, true })

	// Execute
	s, cleanup, err := factory(testDiagram())
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Initial()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(`[3]`, string(got[0].Values)); diff != "" {
		t.Errorf("Initial() mismatch (-want +got):\n%s", diff)
	}
	if len(host.ran) != 1 || len(host.files) != 1 {
		t.Fatalf("ran %v with files %v, want one run and one file", host.ran, host.files)
	}
	path := host.ran[0][1]
	want := append(testChecker.NextArgs(path), "--initial")
	if diff := cmp.Diff(want, host.ran[0]); diff != "" {
		t.Errorf("args mismatch (-want +got):\n%s", diff)
	}
	if !strings.Contains(host.files[path], `"s0"`) {
		t.Errorf("the kept diagram %q is not the session's", host.files[path])
	}
	cleanup()
	if len(host.files) != 0 {
		t.Errorf("files after cleanup = %v, want none", host.files)
	}
}

func TestEachSessionKeepsItsDiagramApart(t *testing.T) {
	// Setup
	host := newFakeHost(pyhost.Result{})
	factory := pyhost.Suggesters(func() (pyhost.Host, bmc.Checker, bool) { return host, testChecker, true })

	// Execute
	_, _, _ = factory(testDiagram())
	_, _, _ = factory(testDiagram())

	// Assert
	if len(host.files) != 2 {
		t.Errorf("files = %v, want two", host.files)
	}
}

func TestAFailedRunIsWhatItSaidOnTheStandardError(t *testing.T) {
	tests := map[string]struct {
		answer pyhost.Result
		err    error
		want   string
	}{
		"exit code": {answer: pyhost.Result{Code: 2, Stderr: "csdfbmc next: no TYPES\n"}, want: "csdfbmc next: no TYPES"},
		"host":      {err: errors.New("Python is not loaded"), want: "Python is not loaded"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			host := newFakeHost(tc.answer)
			host.err = tc.err
			s, _, err := pyhost.Suggesters(func() (pyhost.Host, bmc.Checker, bool) { return host, testChecker, true })(testDiagram())
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			_, err = s.Initial()

			// Assert
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Initial() error = %v, want one saying %q", err, tc.want)
			}
		})
	}
}
