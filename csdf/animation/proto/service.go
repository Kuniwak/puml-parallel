package proto

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/Kuniwak/puml-parallel/tools"
)

// entry is one live session with its origin label.
type entry struct {
	id        string
	path      string
	session   *animation.Session
	suggester animation.Suggester
	cleanup   func()
}

// Service holds the in-memory animation sessions and maps protocol requests to
// engine operations. A single mutex serializes Handle calls, so it is safe for
// concurrent connections; a web server can reuse Handle verbatim.
type Service struct {
	mu       sync.Mutex
	sessions map[string]*entry
	order    []string // session ids in creation order, for stable listing
	nextID   int
	version  string
	solver   csdf.PostSolver
	debug    bool
	// suggesters builds, per new session, what its value prompts ask; nil
	// when the daemon has no checker.
	suggesters SuggesterFactory
	// environmentCheck refuses a replay whose report was made in an
	// environment the suggesters do not walk.
	environmentCheck EnvironmentCheck
}

// SuggesterFactory builds a session's suggester from its diagram's source
// and, for an expansion, the JSON the checker reads; the cleanup, if any,
// runs when the session is removed.
type SuggesterFactory func(source []byte, diagramJSON string) (animation.Suggester, func(), error)

// SetSuggesters gives the daemon a checker: every session created afterwards
// suggests values at its prompts and accepts statevar picks.
func (s *Service) SetSuggesters(f SuggesterFactory) { s.suggesters = f }

// EnvironmentCheck refuses, before a replay walks, a path whose report's
// environment (nil when the report has none) is not the one the daemon's
// checker walks: the daemon's checker is fixed when it starts, and its
// suggestions after the path would otherwise not be what csdfbmc saw there.
type EnvironmentCheck func(*animation.Environment) error

// AnyEnvironment is the EnvironmentCheck of a daemon whose sessions suggest
// nothing, for which every environment is alike.
func AnyEnvironment(*animation.Environment) error { return nil }

// NewService returns a Service that reports the given version and resolves
// state-variable values with csdf.SolveJSON. When debug is false, errors are
// surfaced as their deepest (prefix-free) message; when true, the full wrapped
// chain is surfaced. environmentCheck refuses a replay of a report made in
// another environment; AnyEnvironment refuses none.
func NewService(version string, debug bool, environmentCheck EnvironmentCheck) *Service {
	return &Service{
		sessions:         map[string]*entry{},
		version:          version,
		solver:           csdf.SolveJSON,
		debug:            debug,
		environmentCheck: environmentCheck,
	}
}

// Handle dispatches one request and returns its response.
func (s *Service) Handle(req Request) Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch req.Command {
	case CommandSessionNew:
		return s.handleSessionNew(req)
	case CommandSessionList:
		return s.handleSessionList()
	case CommandSessionRm:
		return s.handleSessionRm(req)
	case CommandRead:
		return s.handleRead(req)
	case CommandSelect:
		return s.handleSelect(req)
	case CommandStatevar:
		return s.handleStatevar(req)
	case CommandTrace:
		return s.handleTrace(req)
	case CommandHistory:
		return s.handleHistory(req)
	case CommandJump:
		return s.handleJump(req)
	case CommandReplay:
		return s.handleReplay(req)
	case CommandServerVersion:
		return s.handleServerVersion()
	default:
		return errorResponse(fmt.Sprintf("unknown command %q", req.Command))
	}
}

func (s *Service) handleSessionNew(req Request) Response {
	diagram, err := csdf.ParseBytes(req.Content)
	if err != nil {
		return s.errorFromErr(err)
	}
	session, err := animation.NewSession(diagram, s.solver)
	if err != nil {
		return s.errorFromErr(err)
	}
	e := &entry{path: req.Path, session: session}
	if s.suggesters != nil {
		sg, cleanup, err := s.suggesters(req.Content, req.DiagramJSON)
		if err != nil {
			return s.errorFromErr(err)
		}
		e.suggester, e.cleanup = sg, cleanup
	}
	s.nextID++
	e.id = strconv.Itoa(s.nextID)
	s.sessions[e.id] = e
	s.order = append(s.order, e.id)
	return Response{OK: true, Session: e.id, Output: e.id + "\n", Data: mustData(SessionRef{Session: e.id})}
}

func (s *Service) handleSessionList() Response {
	infos := make([]SessionInfo, 0, len(s.order))
	var buf bytes.Buffer
	if len(s.order) > 0 {
		w := csv.NewWriter(&buf)
		w.Comma = '\t'
		_ = w.Write([]string{"ID", "MODE", "STATE", "PATH"})
		for _, id := range s.order {
			info := sessionInfo(s.sessions[id])
			infos = append(infos, info)
			_ = w.Write([]string{info.Session, info.Mode, info.StateName, info.Path})
		}
		w.Flush()
	}
	return Response{OK: true, Output: buf.String(), Data: mustData(SessionListData{Sessions: infos})}
}

func (s *Service) handleSessionRm(req Request) Response {
	e, err := s.resolve(req.Session)
	if err != nil {
		return s.errorFromErr(err)
	}
	delete(s.sessions, e.id)
	s.order = removeString(s.order, e.id)
	if e.cleanup != nil {
		e.cleanup()
	}
	return Response{OK: true, Session: e.id, Output: "removed " + e.id + "\n", Data: mustData(SessionRef{Session: e.id})}
}

func (s *Service) handleSelect(req Request) Response {
	e, err := s.resolve(req.Session)
	if err != nil {
		return s.errorFromErr(err)
	}
	if req.Index == nil {
		// No index: show the current position (the selectable transitions).
		return viewResponse(e)
	}
	if err := e.session.Select(*req.Index); err != nil {
		return s.errorFromErr(err)
	}
	return viewResponse(e)
}

func (s *Service) handleStatevar(req Request) Response {
	e, err := s.resolve(req.Session)
	if err != nil {
		return s.errorFromErr(err)
	}
	if e.session.Mode() != animation.ModeValues {
		return errorResponse("not awaiting values; select a transition first")
	}
	values := req.Values
	if req.Pick != nil {
		if e.suggester == nil {
			return errorResponse("the daemon has no checker to pick a suggestion from; start csdfrepld with -checks and -evaluator or -evaluator-module")
		}
		suggestions, err := e.session.Suggest(e.suggester)
		if err != nil {
			return s.errorFromErr(err)
		}
		picked, _, err := animation.PickSuggestion("#"+strconv.Itoa(*req.Pick), suggestions)
		if err != nil {
			return s.errorFromErr(err)
		}
		values = picked
	}
	result, err := e.session.EnterValues(values)
	if err != nil {
		return s.errorFromErr(err)
	}
	switch result.Kind {
	case csdf.PostSolverResultOK:
		return viewResponse(e)
	case csdf.PostSolverResultNoSolutions:
		return errorResponse("No solutions")
	case csdf.PostSolverResultInvalidStateVarValuesLength:
		if result.Err == nil {
			return errorResponse("state variable values length mismatch")
		}
		return s.errorFromErr(result.Err)
	case csdf.PostSolverResultSyntaxError:
		if result.Err == nil {
			return errorResponse("invalid state variable values")
		}
		return s.errorFromErr(result.Err)
	default:
		return errorResponse("post solver returned an unknown result")
	}
}

func (s *Service) handleTrace(req Request) Response {
	e, err := s.resolve(req.Session)
	if err != nil {
		return s.errorFromErr(err)
	}
	trace := e.session.Trace()
	var buf bytes.Buffer
	animation.RenderTrace(&buf, trace)
	return Response{OK: true, Session: e.id, Output: buf.String(), Data: mustData(TraceData{Trace: trace})}
}

func (s *Service) handleHistory(req Request) Response {
	e, err := s.resolve(req.Session)
	if err != nil {
		return s.errorFromErr(err)
	}
	history := e.session.History()
	var buf bytes.Buffer
	animation.RenderHistory(&buf, history)
	return Response{OK: true, Session: e.id, Output: buf.String(), Data: mustData(HistoryData{History: history})}
}

func (s *Service) handleJump(req Request) Response {
	e, err := s.resolve(req.Session)
	if err != nil {
		return s.errorFromErr(err)
	}
	if req.Index == nil {
		return errorResponse("jump requires a history index")
	}
	if err := e.session.Jump(*req.Index); err != nil {
		return s.errorFromErr(err)
	}
	return viewResponse(e)
}

// handleReplay walks a fresh session along the recorded path and shows where
// it got to. A step that cannot be taken is an error, and the session is left
// at the last state reached, so the user can look at it.
func (s *Service) handleReplay(req Request) Response {
	e, err := s.resolve(req.Session)
	if err != nil {
		return s.errorFromErr(err)
	}
	if req.Replay == nil {
		return errorResponse("replay requires a recorded path")
	}
	if err := s.environmentCheck(req.Environment); err != nil {
		return s.errorFromErr(err)
	}
	if err := e.session.Replay(*req.Replay); err != nil {
		return s.errorFromErr(err)
	}
	return viewResponse(e)
}

// handleRead resolves the session and renders its current position.
func (s *Service) handleRead(req Request) Response {
	e, err := s.resolve(req.Session)
	if err != nil {
		return s.errorFromErr(err)
	}
	return viewResponse(e)
}

func (s *Service) handleServerVersion() Response {
	return Response{OK: true, Output: s.version + "\n", Data: mustData(VersionData{Version: s.version})}
}

// resolve returns the requested session, or the single session when id is empty.
func (s *Service) resolve(id string) (*entry, error) {
	if id == "" {
		switch len(s.sessions) {
		case 0:
			return nil, errors.New("no sessions")
		case 1:
			return s.sessions[s.order[len(s.order)-1]], nil
		default:
			return nil, errors.New("multiple sessions; specify -s")
		}
	}
	e, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("no such session: %q", id)
	}
	return e, nil
}

func viewResponse(e *entry) Response {
	sess := e.session
	var buf bytes.Buffer
	view := View{}
	if sess.Mode() == animation.ModeValues {
		group, guard, post, _, prev := sess.Pending()
		animation.RenderStateValuePrompt(&buf, prev, group, guard, post)
		view.Mode = "values"
		view.Pending = &Pending{Previous: prev, Group: group, Guard: guard, Post: post}
		if e.suggester != nil {
			suggestions, err := sess.Suggest(e.suggester)
			if err != nil {
				animation.RenderError(&buf, err.Error())
			} else {
				animation.RenderSuggestions(&buf, suggestions)
				view.Pending.Suggestions = suggestions
			}
		}
	} else {
		current, _ := sess.Current()
		animation.RenderState(&buf, sess.Diagram(), current)
		view.Mode = "command"
		view.State = &current
		view.Transitions = transitionsOf(sess)
	}
	return Response{OK: true, Session: e.id, Output: buf.String(), Data: mustData(view)}
}

func transitionsOf(sess *animation.Session) []Transition {
	edges := sess.Transitions()
	out := make([]Transition, len(edges))
	for i, edge := range edges {
		out[i] = Transition{
			Index:   i,
			Event:   edge.Event,
			Dst:     edge.Dst,
			DstName: sess.Diagram().States[edge.Dst].Name,
			Guard:   edge.Guard,
			Post:    edge.Post,
		}
	}
	return out
}

func sessionInfo(e *entry) SessionInfo {
	info := SessionInfo{Session: e.id, Path: e.path}
	if e.session.Mode() == animation.ModeValues {
		group, _, _, _, _ := e.session.Pending()
		info.Mode = "values"
		info.StateID = group.ID
		info.StateName = group.Name
	} else {
		current, _ := e.session.Current()
		info.Mode = "command"
		info.StateID = current.ID
		info.StateName = current.Name
	}
	return info
}

func errorResponse(message string) Response {
	return Response{OK: false, Error: message}
}

// errorFromErr surfaces err with the deepest (prefix-free) message by default,
// or the full wrapped chain when the service is in debug mode.
func (s *Service) errorFromErr(err error) Response {
	return errorResponse(tools.UserFacingError(err, s.debug))
}

func removeString(items []string, target string) []string {
	out := items[:0]
	for _, item := range items {
		if item != target {
			out = append(out, item)
		}
	}
	return out
}
