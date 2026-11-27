package csdfhelpcmd

import (
	"github.com/Kuniwak/puml-parallel/v9/tools"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfbmc/csdfbmccmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfcomp/csdfcompcmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfdecorate/csdfdecoratecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfevents/csdfeventscmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfhide/csdfhidecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfinclude/csdfincludecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdflint/csdflintcmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdflivelockfree/csdflivelockfreecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfparallel/csdfparallelcmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfparse/csdfparsecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfpromote/csdfpromotecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfrefinement/csdfrefinementcmd"
	replcmd "github.com/Kuniwak/puml-parallel/v9/tools/csdfrepl/csdfreplcmd"
	clientcmd "github.com/Kuniwak/puml-parallel/v9/tools/csdfreplcmd/csdfreplcmdcmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfrepld/csdfrepldcmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfreplweb/csdfreplwebcmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfslice/csdfslicecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfsort/csdfsortcmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdftracechk/csdftracechkcmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdftranstable/csdftranstablecmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/obligationirc/obligationirccmd"
	"github.com/Kuniwak/puml-parallel/v9/tools/toolsdoc"
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
			Summary: "Parses a Composable State Diagram and prints its concrete syntax tree as JSON.",
			Run:     tools.NewCommandFunc(csdfparsecmd.NewParseOptionsFunc(), csdfparsecmd.NewMainFunc()),
		},
		{
			Name:    "csdfinclude",
			Summary: "Resolves the !include lines of the syntax tree csdfparse prints, putting the tree of each file included in its !include.",
			Run:     tools.NewCommandFunc(csdfincludecmd.NewParseOptionsFunc(), csdfincludecmd.NewMainFunc()),
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
			Summary: "Expands the promotion directives of a Composable State Diagram, read as its syntax tree with the !include lines resolved, into plain PlantUML or the JSON csdfbmc reads.",
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
			Name:    "csdfdecorate",
			Summary: "Prints a Composable State Diagram with its edge labels written for people in PlantUML's Creole, for drawing only.",
			Run:     tools.NewCommandFunc(csdfdecoratecmd.NewParseOptionsFunc(), csdfdecoratecmd.NewMainFunc()),
		},
		{
			Name:    "csdfslice",
			Summary: "Prints the part of a Composable State Diagram around some of its states, for rendering only.",
			Run:     tools.NewCommandFunc(csdfslicecmd.NewParseOptionsFunc(), csdfslicecmd.NewMainFunc()),
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
			Name:    "csdfreplweb",
			Summary: "Writes a web page, one HTML file, that explores a Composable State Diagram graphically in the browser.",
			Run:     tools.NewCommandFunc(csdfreplwebcmd.NewParseOptionsFunc(), csdfreplwebcmd.NewMainFunc()),
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
