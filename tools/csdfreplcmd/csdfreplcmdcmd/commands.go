package csdfreplcmdcmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation/proto"
	"github.com/Kuniwak/puml-parallel/tools"
)

// Subcommands returns the csdfreplcmd command tree for tools.NewSubcommandFunc.
func Subcommands() []tools.Subcommand {
	subs := []tools.Subcommand{
		{Name: "session", Description: "manage sessions (new, list, rm)", CommandFunc: tools.NewSubcommandFunc("csdfreplcmd session", "Manage csdfrepld sessions.", sessionSubcommands())},
		{Name: "read", Description: "show the session's current state or value prompt", CommandFunc: tools.NewCommandFunc(parseRead(), newMainFunc())},
		{Name: "select", Description: "select a transition, or list transitions when no index is given", CommandFunc: tools.NewCommandFunc(parseSelect(), newMainFunc())},
		{Name: "statevar", Description: "enter state-variable values as a JSON array", CommandFunc: tools.NewCommandFunc(parseStatevar(), newMainFunc())},
		{Name: "trace", Description: "show the current event trace", CommandFunc: tools.NewCommandFunc(parseTrace(), newMainFunc())},
		{Name: "history", Description: "show the exploration history", CommandFunc: tools.NewCommandFunc(parseHistory(), newMainFunc())},
		{Name: "jump", Description: "jump to a history entry", CommandFunc: tools.NewCommandFunc(parseJump(), newMainFunc())},
		{Name: "serverversion", Description: "print the csdfrepld server version", CommandFunc: tools.NewCommandFunc(parseServerVersion(), newMainFunc())},
	}
	return subs
}

func sessionSubcommands() []tools.Subcommand {
	return []tools.Subcommand{
		{Name: "new", Description: "start a session from a .puml/.png file", CommandFunc: tools.NewCommandFunc(parseSessionNew(), newMainFunc())},
		{Name: "list", Description: "list active sessions", CommandFunc: tools.NewCommandFunc(parseSessionList(), newMainFunc())},
		{Name: "rm", Description: "remove a session", CommandFunc: tools.NewCommandFunc(parseSessionRm(), newMainFunc())},
	}
}

func parseRead() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd read", inout)
		setLeafUsage(flags, "csdfreplcmd read [-s <id>] [-json]",
			"Show the session's current state and its outgoing transitions, or the\nvalue prompt when the session is awaiting state-variable values.\n\nExample:\n  csdfreplcmd read")
		cf := declareConnFlags(flags)
		declareSession(flags, cf)
		declareJSON(flags, cf)
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		if flags.NArg() != 0 {
			return nil, errors.New("read takes no arguments")
		}
		opts.req = proto.Request{Command: proto.CommandRead, Session: cf.session}
		return opts, nil
	}
}

func parseTrace() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd trace", inout)
		setLeafUsage(flags, "csdfreplcmd trace [-s <id>] [-json]",
			"Show the visible event trace of the current path. The internal tau event\nis hidden.")
		cf := declareConnFlags(flags)
		declareSession(flags, cf)
		declareJSON(flags, cf)
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		if flags.NArg() != 0 {
			return nil, errors.New("trace takes no arguments")
		}
		opts.req = proto.Request{Command: proto.CommandTrace, Session: cf.session}
		return opts, nil
	}
}

func parseHistory() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd history", inout)
		setLeafUsage(flags, "csdfreplcmd history [-s <id>] [-json]",
			"Show every explored history entry with its trace and state. Branch from\nan earlier entry with \"csdfreplcmd jump <index>\".")
		cf := declareConnFlags(flags)
		declareSession(flags, cf)
		declareJSON(flags, cf)
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		if flags.NArg() != 0 {
			return nil, errors.New("history takes no arguments")
		}
		opts.req = proto.Request{Command: proto.CommandHistory, Session: cf.session}
		return opts, nil
	}
}

func parseSelect() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd select", inout)
		setLeafUsage(flags, "csdfreplcmd select [<index>] [-s <id>] [-json]",
			"With no index, list the current state's outgoing transitions. With a\nzero-based <index>, take that transition; the session then awaits the\ndestination's state-variable values (enter them with statevar).\n\nExample:\n  csdfreplcmd select        # list transitions\n  csdfreplcmd select 0      # take transition [0]")
		cf := declareConnFlags(flags)
		declareSession(flags, cf)
		declareJSON(flags, cf)
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		req := proto.Request{Command: proto.CommandSelect, Session: cf.session}
		switch flags.NArg() {
		case 0:
		case 1:
			index, err := parseIndex(flags.Arg(0))
			if err != nil {
				return nil, err
			}
			req.Index = &index
		default:
			return nil, errors.New("select takes at most one index")
		}
		opts.req = req
		return opts, nil
	}
}

func parseJump() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd jump", inout)
		setLeafUsage(flags, "csdfreplcmd jump <index> [-s <id>] [-json]",
			"Branch from the zero-based history entry <index>: a copy of that entry is\nappended as a new history entry and becomes the current state, so earlier\nhistory is preserved (jump does not rewind or truncate). Use \"csdfreplcmd\nhistory\" to see the indexes.\n\nExample:\n  csdfreplcmd jump 0")
		cf := declareConnFlags(flags)
		declareSession(flags, cf)
		declareJSON(flags, cf)
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		if flags.NArg() != 1 {
			return nil, errors.New("jump requires a history index")
		}
		index, err := parseIndex(flags.Arg(0))
		if err != nil {
			return nil, err
		}
		opts.req = proto.Request{Command: proto.CommandJump, Session: cf.session, Index: &index}
		return opts, nil
	}
}

func parseStatevar() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd statevar", inout)
		setLeafUsage(flags, "csdfreplcmd statevar -json <json-array> | -json-file <file> [-s <id>]",
			"Enter the current post state group's variable values as a JSON array, in\ndeclaration order (run \"csdfreplcmd read\" to see the names and count). Each\nvalue is one array element; JSON null is not accepted.\n\nExample:\n  csdfreplcmd statevar -json '[[\"cola\",\"water\"]]'")
		cf := declareConnFlags(flags)
		declareSession(flags, cf)
		var jsonText, jsonFile string
		flags.StringVar(&jsonText, "json", "", "state variable values as a JSON array")
		flags.StringVar(&jsonFile, "json-file", "", "file containing the JSON array of state variable values")
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		if flags.NArg() != 0 {
			return nil, errors.New("statevar takes no positional arguments; use -json or -json-file")
		}
		values, err := statevarValues(jsonText, jsonFile)
		if err != nil {
			return nil, err
		}
		opts.req = proto.Request{Command: proto.CommandStatevar, Session: cf.session, Values: values}
		return opts, nil
	}
}

func statevarValues(jsonText, jsonFile string) (string, error) {
	switch {
	case jsonText != "" && jsonFile != "":
		return "", errors.New("statevar accepts only one of -json or -json-file")
	case jsonFile != "":
		bs, err := os.ReadFile(jsonFile)
		if err != nil {
			return "", fmt.Errorf("statevar: reading -json-file: %w", err)
		}
		return string(bs), nil
	case jsonText != "":
		return jsonText, nil
	default:
		return "", errors.New("statevar requires -json <json-text> or -json-file <file>")
	}
}

func parseServerVersion() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd serverversion", inout)
		setLeafUsage(flags, "csdfreplcmd serverversion [-json]",
			"Print the running csdfrepld daemon's version (use -version for this\nclient's version).")
		cf := declareConnFlags(flags)
		declareJSON(flags, cf)
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		if flags.NArg() != 0 {
			return nil, errors.New("serverversion takes no arguments")
		}
		opts.req = proto.Request{Command: proto.CommandServerVersion}
		return opts, nil
	}
}

func parseSessionNew() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd session new", inout)
		setLeafUsage(flags, "csdfreplcmd session new <file.puml|file.png> [-json]",
			"Start a new exploration session from a Composable State Diagram file and\nprint its session id on stdout.\n\nExample:\n  SID=$(csdfreplcmd session new examples/valid/vending_machine.puml)")
		cf := declareConnFlags(flags)
		declareJSON(flags, cf)
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		if flags.NArg() != 1 {
			return nil, errors.New("session new requires exactly one file (.puml or .png)")
		}
		path := flags.Arg(0)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("session new: cannot read file: %v", err)
		}
		opts.req = proto.Request{Command: proto.CommandSessionNew, Path: path, Content: content}
		return opts, nil
	}
}

func parseSessionList() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd session list", inout)
		setLeafUsage(flags, "csdfreplcmd session list [-json]",
			"List active sessions as id, mode, current state, and source path.")
		cf := declareConnFlags(flags)
		declareJSON(flags, cf)
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		if flags.NArg() != 0 {
			return nil, errors.New("session list takes no arguments")
		}
		opts.req = proto.Request{Command: proto.CommandSessionList}
		return opts, nil
	}
}

func parseSessionRm() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd session rm", inout)
		setLeafUsage(flags, "csdfreplcmd session rm [-s <id>] [-json]",
			"Remove a session. When exactly one session is active, -s is optional.")
		cf := declareConnFlags(flags)
		declareSession(flags, cf)
		declareJSON(flags, cf)
		opts, err := parseLeaf(flags, args, cf)
		if err != nil || opts.help {
			return opts, err
		}
		if flags.NArg() != 0 {
			return nil, errors.New("session rm takes no arguments; use -s to choose the session")
		}
		opts.req = proto.Request{Command: proto.CommandSessionRm, Session: cf.session}
		return opts, nil
	}
}
