package csdf

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/v5/csdf/clause"
)

// UnmarshalSyntaxTree reads the JSON MarshalSyntaxTree writes back into a
// SyntaxTree. A clause is read again from its text, of which its syntax
// tree in the JSON is the reading.
func UnmarshalSyntaxTree(bs []byte) (*SyntaxTree, error) {
	var raw struct {
		Kind    string            `json:"kind"`
		Version int               `json:"version"`
		Path    string            `json:"path"`
		Name    string            `json:"name"`
		Line    int               `json:"line"`
		Col     int               `json:"col"`
		Body    []json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(bs, &raw); err != nil {
		return nil, fmt.Errorf("csdf.UnmarshalSyntaxTree: %w", err)
	}
	if raw.Kind != SyntaxTreeKind || raw.Version != SyntaxTreeVersion {
		return nil, fmt.Errorf("csdf.UnmarshalSyntaxTree: want kind %q version %d, got kind %q version %d", SyntaxTreeKind, SyntaxTreeVersion, raw.Kind, raw.Version)
	}
	body, err := elementsOf(raw.Body)
	if err != nil {
		return nil, fmt.Errorf("csdf.UnmarshalSyntaxTree: %w", err)
	}
	return &SyntaxTree{Pos: Pos{raw.Line, raw.Col}, Path: raw.Path, Name: raw.Name, Body: body}, nil
}

// rawElement is the JSON of any element: the fields of every kind.
type rawElement struct {
	Kind string `json:"kind"`
	Pos
	Text       string             `json:"text"`
	Block      bool               `json:"block"`
	Name       string             `json:"name"`
	Arg        string             `json:"arg"`
	Lines      json.RawMessage    `json:"lines"`
	View       bool               `json:"view"`
	States     []string           `json:"states"`
	ID         string             `json:"id"`
	Label      string             `json:"label"`
	Stereotype string             `json:"stereotype"`
	Body       *[]json.RawMessage `json:"body"`
	State      string             `json:"state"`
	Type       string             `json:"type"`
	Src        string             `json:"src"`
	Dst        string             `json:"dst"`
	Event      json.RawMessage    `json:"event"`
	Guard      *rawClause         `json:"guard"`
	Post       *rawClause         `json:"post"`
	Side       string             `json:"side"`
	Of         string             `json:"of"`
	Fields     []string           `json:"fields"`
	Targets    []SyncTargetSyntax `json:"targets"`
	Tree       json.RawMessage    `json:"tree"`
}

type rawClause struct {
	Pos
	Text string `json:"text"`
}

type rawEvent struct {
	Pos
	Text    string        `json:"text"`
	Channel string        `json:"channel"`
	Fields  []FieldSyntax `json:"fields"`
}

func elementsOf(raws []json.RawMessage) ([]Element, error) {
	out := make([]Element, 0, len(raws))
	for _, r := range raws {
		e, err := elementOf(r)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func elementOf(bs json.RawMessage) (Element, error) {
	var r rawElement
	if err := json.Unmarshal(bs, &r); err != nil {
		return nil, fmt.Errorf("an element: %w", err)
	}
	e, err := r.element()
	if err != nil {
		return nil, fmt.Errorf("the %s at line %d: %w", r.Kind, r.Line, err)
	}
	return e, nil
}

// element is the element r is the JSON of.
func (r *rawElement) element() (Element, error) {
	switch r.Kind {
	case "comment":
		return &CommentSyntax{Pos: r.Pos, Text: r.Text, Block: r.Block}, nil
	case "pragma":
		p := &PragmaSyntax{Pos: r.Pos, Name: r.Name, Arg: r.Arg}
		if r.Tree != nil {
			if r.Name != "include" {
				return nil, fmt.Errorf("!%s holds a tree; only an !include does", r.Name)
			}
			tree, err := UnmarshalSyntaxTree(r.Tree)
			if err != nil {
				return nil, err
			}
			p.Tree = tree
		}
		return p, nil
	case "ignore":
		var lines []string
		if err := json.Unmarshal(r.Lines, &lines); err != nil {
			return nil, err
		}
		return &IgnoreSyntax{Pos: r.Pos, Lines: lines}, nil
	case "observe":
		return &ObserveSyntax{Pos: r.Pos, Name: r.Name, View: r.View, States: r.States}, nil
	case "state":
		s := &StateSyntax{Pos: r.Pos, ID: r.ID, Label: r.Label, Stereotype: r.Stereotype, Composite: r.Body != nil}
		if r.Body != nil {
			body, err := elementsOf(*r.Body)
			if err != nil {
				return nil, err
			}
			s.Body = body
		}
		return s, nil
	case "state_var":
		return &StateVarSyntax{Pos: r.Pos, State: r.State, Name: r.Name, Type: r.Type}, nil
	case "start_edge":
		post, err := r.Post.syntax()
		if err != nil {
			return nil, err
		}
		return &StartEdgeSyntax{Pos: r.Pos, Dst: r.Dst, Post: post}, nil
	case "edge":
		e, err := eventOf(r.Event)
		if err != nil {
			return nil, err
		}
		guard, err := r.Guard.syntax()
		if err != nil {
			return nil, err
		}
		post, err := r.Post.syntax()
		if err != nil {
			return nil, err
		}
		return &EdgeSyntax{Pos: r.Pos, Src: r.Src, Dst: r.Dst, Event: e, Guard: guard, Post: post}, nil
	case "end_edge":
		guard, err := r.Guard.syntax()
		if err != nil {
			return nil, err
		}
		return &EndEdgeSyntax{Pos: r.Pos, Src: r.Src, Guard: guard}, nil
	case "note":
		var raws []json.RawMessage
		if err := json.Unmarshal(r.Lines, &raws); err != nil {
			return nil, err
		}
		lines, err := elementsOf(raws)
		if err != nil {
			return nil, err
		}
		return &NoteSyntax{Pos: r.Pos, ID: r.ID, Side: r.Side, Of: r.Of, Lines: lines}, nil
	case "assume":
		return &AssumeSyntax{Pos: r.Pos, Text: r.Text}, nil
	case "channel":
		// The JSON keeps no text of a channel; it is written again from the
		// name and the field types, not with the spacing of the source.
		text := r.Name
		if len(r.Fields) > 0 {
			text += " : " + strings.Join(r.Fields, ".")
		}
		return &ChannelSyntax{Pos: r.Pos, Text: text, Name: r.Name, Fields: r.Fields}, nil
	case "sync":
		var event string
		if err := json.Unmarshal(r.Event, &event); err != nil {
			return nil, err
		}
		guard, err := r.Guard.syntax()
		if err != nil {
			return nil, err
		}
		return &SyncSyntax{Pos: r.Pos, Event: event, Targets: r.Targets, Guard: guard}, nil
	case "constrain":
		e, err := eventOf(r.Event)
		if err != nil {
			return nil, err
		}
		guard, err := r.Guard.syntax()
		if err != nil {
			return nil, err
		}
		return &ConstrainSyntax{Pos: r.Pos, Event: e, Guard: guard}, nil
	case "text":
		return &ProseSyntax{Pos: r.Pos, Text: r.Text}, nil
	}
	return nil, errors.New("an element of unknown kind")
}

func eventOf(bs json.RawMessage) (EventSyntax, error) {
	var e rawEvent
	if err := json.Unmarshal(bs, &e); err != nil {
		return EventSyntax{}, err
	}
	return EventSyntax{Pos: e.Pos, Text: e.Text, Channel: e.Channel, Fields: e.Fields}, nil
}

// syntax is the clause c, read again from its text; nil when not written.
func (c *rawClause) syntax() (*ClauseSyntax, error) {
	if c == nil {
		return nil, nil
	}
	s, err := clause.ParseSyntax(c.Text)
	if err != nil {
		return nil, fmt.Errorf("the clause %q: %w", c.Text, err)
	}
	return &ClauseSyntax{Pos: c.Pos, Text: c.Text, Syntax: s}, nil
}
