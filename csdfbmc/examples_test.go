package csdfbmc_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfparse/csdfparsecmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfpromote/csdfpromotecmd"
	"github.com/google/go-cmp/cmp"
)

// The JSON of each worked example is what the tools print of its diagram
// today, so the examples csdfbmc walks are the diagrams beside them.
func TestTheExamplesJSONIsWhatTheToolsPrint(t *testing.T) {
	tests := map[string]struct {
		json string
		run  func(args []string, inout *cli.ProcInout) int
		args []string
	}{
		"printer": {json: "../examples/bmc/printer_formal.json", run: tools.NewCommandFunc(csdfparsecmd.NewParseOptionsFunc(), csdfparsecmd.NewMainFunc()),
			args: []string{"../examples/bmc/printer_formal.puml"}},
		"printer_retry": {json: "../examples/bmc/printer_retry_formal.json", run: tools.NewCommandFunc(csdfparsecmd.NewParseOptionsFunc(), csdfparsecmd.NewMainFunc()),
			args: []string{"../examples/bmc/printer_retry_formal.puml"}},
		"attempts": {json: "../examples/bmc/attempts/attempts.json", run: tools.NewCommandFunc(csdfpromotecmd.NewParseOptionsFunc(), csdfpromotecmd.NewMainFunc()),
			args: []string{"-format", "json", "../examples/bmc/attempts/ATTEMPTS.puml"}},
	}
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
			exit := tt.run(tt.args, spy.New())

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
