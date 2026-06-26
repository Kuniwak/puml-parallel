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
	return append(subs, tools.Subcommand{Name: "help", Description: "show this help", CommandFunc: helpCommand(subs)})
}

func sessionSubcommands() []tools.Subcommand {
	return []tools.Subcommand{
		{Name: "new", Description: "start a session from a .puml/.png file", CommandFunc: tools.NewCommandFunc(parseSessionNew(), newMainFunc())},
		{Name: "list", Description: "list active sessions", CommandFunc: tools.NewCommandFunc(parseSessionList(), newMainFunc())},
		{Name: "rm", Description: "remove a session", CommandFunc: tools.NewCommandFunc(parseSessionRm(), newMainFunc())},
	}
}

func helpCommand(subs []tools.Subcommand) cli.CommandFunc {
	return func(_ []string, inout *cli.ProcInout) int {
		fmt.Fprintln(inout.Stdout, "Usage: csdfreplcmd <command> [options]")
		fmt.Fprintln(inout.Stdout)
		fmt.Fprintln(inout.Stdout, "Commands:")
		for _, sub := range subs {
			fmt.Fprintf(inout.Stdout, "  %-14s %s\n", sub.Name, sub.Description)
		}
		fmt.Fprintf(inout.Stdout, "  %-14s %s\n", "help", "show this help")
		return 0
	}
}

func parseRead() cli.ParseOptionsFunc[*clientOptions] {
	return func(args []string, inout *cli.ProcInout) (*clientOptions, error) {
		flags := newFlagSet("csdfreplcmd read", inout)
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
