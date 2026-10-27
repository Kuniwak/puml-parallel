package csdfhelpcmd

import (
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfbmc/csdfbmccmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfcomp/csdfcompcmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfevents/csdfeventscmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfhide/csdfhidecmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdflint/csdflintcmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdflivelockfree/csdflivelockfreecmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfparallel/csdfparallelcmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfparse/csdfparsecmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfpromote/csdfpromotecmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfrefinement/csdfrefinementcmd"
	replcmd "github.com/Kuniwak/puml-parallel/v3/tools/csdfrepl/csdfreplcmd"
	clientcmd "github.com/Kuniwak/puml-parallel/v3/tools/csdfreplcmd/csdfreplcmdcmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfrepld/csdfrepldcmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfsort/csdfsortcmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdftracechk/csdftracechkcmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdftranstable/csdftranstablecmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/obligationirc/obligationirccmd"
	"github.com/Kuniwak/puml-parallel/v3/tools/toolsdoc"
)

// Registry returns every tool csdfhelp bundles, ordered as the README
// introduces them. It is a function rather than a variable so that csdfhelp's
// own entry, whose command function lives in this package, can be built
// without an initialization cycle, and so that no caller can mutate the
// catalog.
func Registry() []toolsdoc.Entry {
	return []toolsdoc.Entry{
		{
			Name:    "csdfparse",
			Summary: "Parses a Composable State Diagram and prints the parsed structure as JSON.",
			Run:     tools.NewCommandFunc(csdfparsecmd.NewParseOptionsFunc(), csdfparsecmd.NewMainFunc()),
		},
		{
			Name:    "csdfevents",
			Summary: "Prints the channels used across one or more Composable State Diagrams.",
			Run:     tools.NewCommandFunc(csdfeventscmd.NewParseOptionsFunc(), csdfeventscmd.NewMainFunc()),
		},
		{
			Name:    "csdfparallel",
			Summary: "Composes Composable State Diagrams in parallel following CSP interface parallel semantics.",
			Run:     tools.NewCommandFunc(csdfparallelcmd.NewParseOptionsFunc(), csdfparallelcmd.NewMainFunc()),
		},
		{
			Name:    "csdfhide",
			Summary: "Hides channels of a Composable State Diagram following CSP hiding semantics.",
			Run:     tools.NewCommandFunc(csdfhidecmd.NewParseOptionsFunc(), csdfhidecmd.NewMainFunc()),
		},
		{
			Name:    "csdfpromote",
			Summary: "Expands the promotion directives of a global Composable State Diagram into plain PlantUML.",
			Run:     tools.NewCommandFunc(csdfpromotecmd.NewParseOptionsFunc(), csdfpromotecmd.NewMainFunc()),
		},
		{
			Name:    "csdfcomp",
			Summary: "Composes the Composable State Diagrams referred to by a composition tree.",
			Run:     tools.NewCommandFunc(csdfcompcmd.NewParseOptionsFunc(), csdfcompcmd.NewMainFunc()),
		},
		{
			Name:    "csdfsort",
			Summary: "Prints a Composable State Diagram in canonical order without changing its meaning.",
			Run:     tools.NewCommandFunc(csdfsortcmd.NewParseOptionsFunc(), csdfsortcmd.NewMainFunc()),
		},
		{
			Name:    "csdflint",
			Summary: "Checks Composable State Diagrams and reports what it finds as TSV.",
			Run:     tools.NewCommandFunc(csdflintcmd.NewParseOptionsFunc(), csdflintcmd.NewMainFunc()),
		},
		{
			Name:    "csdflivelockfree",
			Summary: "Compiles a livelock-freedom proof obligation as JSON IR, Isabelle/HOL, or Lean 4.",
			Run:     tools.NewCommandFunc(csdflivelockfreecmd.NewParseOptionsFunc(), csdflivelockfreecmd.NewMainFunc()),
		},
		{
			Name:    "csdftracechk",
			Summary: "Checks whether event-sequence TSVs are traces of a Composable State Diagram and states the obligation on its predicates.",
			Run:     tools.NewCommandFunc(csdftracechkcmd.NewParseOptionsFunc(), csdftracechkcmd.NewMainFunc()),
		},
		{
			Name:    "csdftranstable",
			Summary: "Writes the state transition table of a Composable State Diagram as TSV, showing where each event is refused.",
			Run:     tools.NewCommandFunc(csdftranstablecmd.NewParseOptionsFunc(), csdftranstablecmd.NewMainFunc()),
		},
		{
			Name:    "csdfrefinement",
			Summary: "Compiles a refinement proof obligation for two Composable State Diagrams as JSON IR, Isabelle/HOL, or Lean 4.",
			Run:     tools.NewCommandFunc(csdfrefinementcmd.NewParseOptionsFunc(), csdfrefinementcmd.NewMainFunc()),
		},
		{
			Name:    "obligationirc",
			Summary: "Compiles the livelock-freedom proof-obligation IR to JSON IR, Isabelle/HOL, or Lean 4.",
			Run:     tools.NewCommandFunc(obligationirccmd.NewParseOptionsFunc(), obligationirccmd.NewMainFunc()),
		},
		{
			Name:    "csdfbmc",
			Summary: "Runs the bounded model checker over a diagram whose clauses a glue reads, within finite domains (Python, carried by the binary).",
			Run:     tools.NewCommandFunc(csdfbmccmd.NewParseOptionsFunc(), csdfbmccmd.NewMainFunc()),
		},
		{
			Name:    "csdfrepl",
			Summary: "Interactively explores a Composable State Diagram.",
			Run:     tools.NewCommandFunc(replcmd.NewParseOptionsFunc(), replcmd.NewMainFunc()),
		},
		{
			Name:    "csdfrepld",
			Summary: "Runs the CSDF REPL daemon, holding exploration sessions behind a Unix domain socket.",
			Run:     tools.NewCommandFunc(csdfrepldcmd.NewParseOptionsFunc(), csdfrepldcmd.NewMainFunc()),
		},
		{
			Name:    "csdfreplcmd",
			Summary: "One-shot client for the csdfrepld daemon, for headless exploration by agents.",
			Run:     tools.NewSubcommandFunc("csdfreplcmd", clientcmd.Description, clientcmd.Subcommands()),
			Subs:    clientcmd.Subcommands(),
		},
		{
			Name:    "csdfhelp",
			Summary: "Prints the help of every tool in this repository as one Markdown document.",
			Hidden:  true,
			Run:     NewCommandFunc(),
		},
	}
}
