// Package csdfreplcmdcmd implements the csdfreplcmd client: each subcommand
// dials the csdfrepld daemon, sends one request, prints the response, and exits.
package csdfreplcmdcmd

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation/proto"
	"github.com/Kuniwak/puml-parallel/tools"
)

// clientFlags are the connection/output flags shared by the subcommands.
type clientFlags struct {
	sock    string
	session string
	json    bool
}

// clientOptions is the parsed result every subcommand produces; the shared main
// function turns req into one daemon round trip.
type clientOptions struct {
	help  bool
	flags *clientFlags
	req   proto.Request
}

func newFlagSet(name string, inout *cli.ProcInout) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(inout.Stderr)
	return flags
}

// declareConnFlags registers -sock on flags; declareSession and declareJSON add
// the optional -s and -json toggles where a subcommand supports them.
func declareConnFlags(flags *flag.FlagSet) *clientFlags {
	cf := &clientFlags{}
	flags.StringVar(&cf.sock, "sock", "", "csdfrepld socket path (default: $"+tools.SocketEnv+", $XDG_RUNTIME_DIR, or tmp)")
	return cf
}

func declareSession(flags *flag.FlagSet, cf *clientFlags) {
	flags.StringVar(&cf.session, "s", "", "session id (optional when there is exactly one session)")
}

func declareJSON(flags *flag.FlagSet, cf *clientFlags) {
	flags.BoolVar(&cf.json, "json", false, "print the structured JSON response instead of text")
}

// parseLeaf runs flags.Parse, mapping -h/-help to a help result.
func parseLeaf(flags *flag.FlagSet, args []string, cf *clientFlags) (*clientOptions, error) {
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return &clientOptions{help: true}, nil
		}
		return nil, err
	}
	return &clientOptions{flags: cf}, nil
}

// newMainFunc is shared by every leaf: it performs the request the parser built.
func newMainFunc() cli.MainFunc[*clientOptions] {
	return func(opts *clientOptions, inout *cli.ProcInout) error {
		if opts.help {
			return nil
		}
		return runRequest(inout, opts.flags, opts.req)
	}
}

func runRequest(inout *cli.ProcInout, flags *clientFlags, req proto.Request) error {
	sock := tools.ResolveSocketPath(flags.sock, inout.Env)
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return fmt.Errorf("cannot connect to csdfrepld at %s: is the daemon running?", sock)
	}
	defer func() { _ = conn.Close() }()

	resp, err := proto.Do(conn, req)
	if err != nil {
		return fmt.Errorf("csdfreplcmdcmd: communicating with csdfrepld: %w", err)
	}
	return printResponse(inout, flags.json, resp)
}

func printResponse(inout *cli.ProcInout, jsonOut bool, resp proto.Response) error {
	if !resp.OK {
		return errors.New(resp.Error)
	}
	if jsonOut {
		if len(resp.Data) > 0 {
			fmt.Fprintln(inout.Stdout, string(resp.Data))
		}
		return nil
	}
	if resp.Output != "" {
		fmt.Fprint(inout.Stdout, resp.Output)
	}
	return nil
}

func parseIndex(arg string) (int, error) {
	index, err := strconv.Atoi(arg)
	if err != nil || index < 0 {
		return 0, fmt.Errorf("not a natural number: %q", arg)
	}
	return index, nil
}
