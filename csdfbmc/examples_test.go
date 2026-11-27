package csdfbmc_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v9/cli"
	"github.com/Kuniwak/puml-parallel/v9/csdf/core"
	"github.com/Kuniwak/puml-parallel/v9/tools"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfinclude/csdfincludecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfparse/csdfparsecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfpromote/csdfpromotecmd"
	"github.com/google/go-cmp/cmp"
)

// The JSON of each worked example is what the tools print of its diagram
// today, so the examples csdfbmc walks are the diagrams beside them.
func TestTheExamplesJSONIsWhatTheToolsPrint(t *testing.T) {
	tests := map[string]struct{ json, puml string }{
		"printer":       {json: "../examples/bmc/printer_formal.json", puml: "../examples/bmc/printer_formal.puml"},
		"printer_retry": {json: "../examples/bmc/printer_retry_formal.json", puml: "../examples/bmc/printer_retry_formal.puml"},
		"attempts":      {json: "../examples/bmc/attempts/attempts.json", puml: "../examples/bmc/attempts/ATTEMPTS.puml"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			recorded, err := os.ReadFile(tt.json)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			tree := pipe(t, tools.NewCommandFunc(csdfparsecmd.NewParseOptionsFunc(), csdfparsecmd.NewMainFunc()), []string{tt.puml}, "")
			resolved := pipe(t, tools.NewCommandFunc(csdfincludecmd.NewParseOptionsFunc(), csdfincludecmd.NewMainFunc()), nil, tree)
			out := pipe(t, tools.NewCommandFunc(csdfpromotecmd.NewParseOptionsFunc(), csdfpromotecmd.NewMainFunc()), []string{"-format", "json"}, resolved)

			// Assert
			want, err := core.UnmarshalDiagram(recorded)
			if err != nil {
				t.Fatal(err)
			}
			got, err := core.UnmarshalDiagram([]byte(out))
			if err != nil {
				t.Fatal(err)
			}
			// The title names the command line, which names the path.
			want.Name, got.Name = "", ""
			if d := cmp.Diff(want, got); d != "" {
				t.Errorf("%s is not what the tools print; regenerate it (-want +got):\n%s", tt.json, d)
			}
		})
	}
}

// pipe is what run prints with args and stdin, as a stage of a shell
// pipeline; a nonzero exit fails the test.
func pipe(t *testing.T, run func([]string, *cli.ProcInout) int, args []string, stdin string) string {
	t.Helper()
	spy := cli.SpyProcInout()
	spy.Stdin = strings.NewReader(stdin)
	if exit := run(args, spy.New()); exit != 0 {
		t.Fatalf("want 0, got %d: %s", exit, spy.Stderr.String())
	}
	return spy.Stdout.String()
}
