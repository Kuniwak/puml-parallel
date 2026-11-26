package csdfbmc_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v4/cli"
	"github.com/Kuniwak/puml-parallel/v4/csdf/core"
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/tools/csdfpromote/csdfpromotecmd"
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
	run := tools.NewCommandFunc(csdfpromotecmd.NewParseOptionsFunc(), csdfpromotecmd.NewMainFunc())
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			recorded, err := os.ReadFile(tt.json)
			if err != nil {
				t.Fatal(err)
			}
			spy := cli.SpyProcInout()
			spy.Stdin = strings.NewReader("")

			// Execute
			exit := run([]string{"-format", "json", tt.puml}, spy.New())

			// Assert
			if exit != 0 {
				t.Fatalf("want 0, got %d: %s", exit, spy.Stderr.String())
			}
			want, err := core.UnmarshalDiagram(recorded)
			if err != nil {
				t.Fatal(err)
			}
			got, err := core.UnmarshalDiagram([]byte(spy.Stdout.String()))
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
