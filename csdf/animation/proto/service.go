package proto

import (
	"bytes"
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
	id      string
	path    string
	session *animation.Session
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
}

// NewService returns a Service that reports the given version and resolves
// state-variable values with csdf.SolveJSON. When debug is false, errors are
// surfaced as their deepest (prefix-free) message; when true, the full wrapped
// chain is surfaced.
func NewService(version string, debug bool) *Service {
	return &Service{
		sessions: map[string]*entry{},
		version:  version,
		solver:   csdf.SolveJSON,
		debug:    debug,
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
	case CommandServerVersion:
		return s.handleServerVersion()
	default:
		return errorResponse(fmt.Sprintf("unknown command %q", req.Command))
	}
}

func (s *Service) handleSessionNew(req Request) Response {
	diagram, err := csdf.ParseDiagram(req.Content)
	if err != nil {
		return s.errorFromErr(err)
	}
	session, err := animation.NewSession(diagram, s.solver)
	if err != nil {
		return s.errorFromErr(err)
	}
	s.nextID++
	id := strconv.Itoa(s.nextID)
	s.sessions[id] = &entry{id: id, path: req.Path, session: session}
	s.order = append(s.order, id)
	return Response{OK: true, Session: id, Output: id + "\n", Data: mustData(SessionRef{Session: id})}
}

func (s *Service) handleSessionList() Response {
	infos := make([]SessionInfo, 0, len(s.order))
	var buf bytes.Buffer
	for _, id := range s.order {
		info := sessionInfo(s.sessions[id])
		infos = append(infos, info)
		fmt.Fprintf(&buf, "%s\t%s\t%s\t%s\n", info.Session, info.Mode, info.StateName, info.Path)
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
	result, err := e.session.EnterValues(req.Values)
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
