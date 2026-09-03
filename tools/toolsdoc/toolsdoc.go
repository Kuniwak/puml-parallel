// Package toolsdoc turns a catalog of CLI tools into documentation. It is
// independent of any one interface, so the same catalog can back a CLI, a
// generated Markdown file, or a tool listing served over a protocol.
package toolsdoc

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

// Entry is one documented CLI.
//
// Run is the tool's command function; Run([]string{"-h"}, inout) writes the
// usage and returns 0. Subs is non-nil only for a command group built with
// tools.NewSubcommandFunc, whose -h prints just the command listing, so each
// subcommand's own usage has to be rendered separately.
type Entry struct {
	Name    string
	Summary string
	Hidden  bool // excluded from a listing of every tool; still shown when named.
	Run     cli.CommandFunc
	Subs    []tools.Subcommand
}

// Lookup returns the entry named name, hidden ones included.
func Lookup(entries []Entry, name string) (Entry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return Entry{}, false
}

// Select returns the entries to document, in catalog order. With no names it
// returns every non-hidden entry; with names it returns the matching entries,
// hidden ones included. Unknown names are ignored, so validate them first.
func Select(entries []Entry, names []string) []Entry {
	if len(names) == 0 {
		selected := make([]Entry, 0, len(entries))
		for _, entry := range entries {
			if !entry.Hidden {
				selected = append(selected, entry)
			}
		}
		return selected
	}

	requested := make(map[string]bool, len(names))
	for _, name := range names {
		requested[name] = true
	}
	selected := make([]Entry, 0, len(names))
	for _, entry := range entries {
		if requested[entry.Name] {
			selected = append(selected, entry)
		}
	}
	return selected
}

// Names returns the name of every entry, in order.
func Names(entries []Entry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

// WriteSummaries writes one "name: summary" line per entry.
func WriteSummaries(w io.Writer, entries []Entry) error {
	for _, entry := range entries {
		if _, err := fmt.Fprintf(w, "%s: %s\n", entry.Name, entry.Summary); err != nil {
			return fmt.Errorf("toolsdoc.WriteSummaries: cannot write: %w", err)
		}
	}
	return nil
}

// WriteMarkdown writes every entry as a Markdown section: its usage under a
// level-1 heading, then one level-2 heading per subcommand of a command group.
// env is handed to each tool so it sees the same environment as the caller, and
// must not be nil.
func WriteMarkdown(w io.Writer, env cli.EnvFunc, entries []Entry) error {
	for _, entry := range entries {
		if err := writeHelp(w, env, "# "+entry.Name, entry.Name, entry.Run); err != nil {
			return err
		}
		for _, sub := range entry.Subs {
			name := entry.Name + " " + sub.Name
			if err := writeHelp(w, env, "## "+name, name, sub.CommandFunc); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeHelp renders one heading and the fenced output of cmdFunc's -h. The help
// is collected first so a failing tool leaves no half-written fence behind.
func writeHelp(w io.Writer, env cli.EnvFunc, heading, name string, cmdFunc cli.CommandFunc) error {
	var help bytes.Buffer
	// A tool writes its usage to stderr and a command group to stdout, so both
	// streams are collected into the same buffer.
	inout := &cli.ProcInout{
		Stdin:  cli.StubStdin(strings.NewReader("")),
		Stdout: cli.StubStdout(&help),
		Stderr: cli.StubStderr(&help),
		Env:    env,
	}
	if exitStatus := cmdFunc([]string{"-h"}, inout); exitStatus != 0 {
		return fmt.Errorf("toolsdoc.writeHelp: %s: help exited with %d", name, exitStatus)
	}

	body := help.String()
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	fence := Fence(body)
	if _, err := fmt.Fprintf(w, "%s\n\n%s\n%s%s\n\n", heading, fence, body, fence); err != nil {
		return fmt.Errorf("toolsdoc.writeHelp: %s: cannot write: %w", name, err)
	}
	return nil
}

// Fence returns a code fence long enough to survive backticks in content: the
// usual three, or one backtick more than content's longest run.
func Fence(content string) string {
	longest := 0
	run := 0
	for _, r := range content {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
			continue
		}
		run = 0
	}
	if longest < 3 {
		return "```"
	}
	return strings.Repeat("`", longest+1)
}
