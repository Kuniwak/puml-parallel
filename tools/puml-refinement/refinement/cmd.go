package refinement

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/core"
)

type Options struct {
	SpecFile string
	ImplFile string
	Help     bool
}

func ParseOptions(args []string, inout *cli.ProcInout) (*Options, error) {
	flags := flag.NewFlagSet("puml-refinement", flag.ContinueOnError)
	flags.SetOutput(inout.Stderr)

	options := &Options{}
	flags.StringVar(&options.SpecFile, "spec", "", "Path to specification PUML file")
	flags.StringVar(&options.ImplFile, "impl", "", "Path to implementation PUML file")

	flags.Usage = func() {
		fmt.Fprintf(inout.Stderr, "Usage: puml-refinement -spec <spec.puml> -impl <impl.puml>\n\n")
		fmt.Fprintf(inout.Stderr, "Stable failures refinement verification tool for PlantUML state diagrams.\n\n")
		fmt.Fprintf(inout.Stderr, "OPTIONS:\n")
		flags.PrintDefaults()
		fmt.Fprintf(inout.Stderr, "\nEXAMPLES:\n")
		fmt.Fprintf(inout.Stderr, "  puml-refinement -spec spec.puml -impl impl.puml\n")
		fmt.Fprintf(inout.Stderr, "  puml-refinement -spec examples/test_cases/spec_guard_fail.puml -impl examples/test_cases/impl_guard_fail.puml\n")
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			options.Help = true
			return options, nil
		}
		return nil, err
	}

	if options.SpecFile == "" || options.ImplFile == "" {
		fmt.Fprintf(inout.Stderr, "error: both -spec and -impl flags are required\n")
		flags.Usage()
		return nil, fmt.Errorf("missing required flags")
	}

	return options, nil
}

func MainCommandByArgs(args []string, inout *cli.ProcInout) int {
	opts, err := ParseOptions(args, inout)
	if err != nil {
		return 1
	}

	if opts.Help {
		return 0
	}

	if err := MainCommandByOptions(opts, inout); err != nil {
		fmt.Fprintf(inout.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

func MainCommandByOptions(opts *Options, inout *cli.ProcInout) error {
	// Parse specification
	specDiagram, err := parsePumlFile(opts.SpecFile)
	if err != nil {
		return fmt.Errorf("parsing specification file %s: %w", opts.SpecFile, err)
	}

	// Parse implementation
	implDiagram, err := parsePumlFile(opts.ImplFile)
	if err != nil {
		return fmt.Errorf("parsing implementation file %s: %w", opts.ImplFile, err)
	}

	// Create stable failures verifier
	verifier := core.NewStableFailuresVerifier(specDiagram, implDiagram)

	// Generate proof obligations
	obligations, err := verifier.GenerateStableFailuresProofObligations()
	if err != nil {
		return fmt.Errorf("generating proof obligations: %w", err)
	}

	// Output proof obligations to stdout
	fmt.Fprint(inout.Stdout, core.FormatProofObligations(obligations))
	return nil
}

func parsePumlFile(filename string) (*core.Diagram, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	parser := core.NewParser(string(content))
	diagram, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse PUML: %w", err)
	}

	return diagram, nil
}
