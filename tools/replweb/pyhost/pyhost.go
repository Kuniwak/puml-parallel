// Package pyhost suggests values through Python running in the page
// (csdfreplweb -checks): the engine asks csdfbmc next, run in that
// interpreter (csdfbmc.inprocess), as a terminal REPL asks the csdfbmc
// process (bmc.RunCsdfbmc).
package pyhost

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/Kuniwak/puml-parallel/v3/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v3/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
)

// Result is what a command of csdfbmc run in the host exits with and
// prints.
type Result struct {
	Code   int    `json:"code"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// Host is the Python interpreter of the page: it keeps files where the
// commands it runs read them, and runs csdfbmc's commands
// (csdfbmc.inprocess.run); an error says it could not run them at all.
type Host interface {
	WriteFile(path string, data []byte) error
	Remove(path string) error
	Run(args []string) (Result, error)
}

// Find gives the host and the checker it asks with, or false when the page
// has none, so that its sessions suggest nothing.
type Find func() (Host, bmc.Checker, bool)

// Dir is where the host keeps the diagrams of the sessions.
const Dir = "/csdfrepl"

var stored atomic.Int64

// Suggesters is the factory of a session's suggester: the one asking the
// host find gives when the session starts, or animation.NoSuggesters.
func Suggesters(find Find) animation.SuggesterFactory {
	return func(d *core.Diagram) (animation.Suggester, func(), error) {
		host, checker, ok := find()
		if !ok {
			return animation.NoSuggesters(d)
		}
		return checker.SuggestersIn(Store(host), Runner(host))(d)
	}
}

// Store keeps a diagram JSON in the host, a file per session.
func Store(host Host) bmc.Store {
	return func(raw []byte) (string, func(), error) {
		path := fmt.Sprintf("%s/diagram-%d.json", Dir, stored.Add(1))
		if err := host.WriteFile(path, raw); err != nil {
			return "", nil, fmt.Errorf("pyhost.Store: %w", err)
		}
		return path, func() { _ = host.Remove(path) }, nil
	}
}

// Runner runs csdfbmc in the host, failing with what it said on its
// standard error when it exits other than 0.
func Runner(host Host) bmc.Runner {
	return func(args []string) ([]byte, error) {
		r, err := host.Run(args)
		if err != nil {
			return nil, fmt.Errorf("pyhost.Runner: %w", err)
		}
		if r.Code != 0 {
			msg := strings.TrimSpace(r.Stderr)
			if msg == "" {
				msg = fmt.Sprintf("csdfbmc exited %d", r.Code)
			}
			return nil, fmt.Errorf("pyhost.Runner: %s", msg)
		}
		return []byte(r.Stdout), nil
	}
}
