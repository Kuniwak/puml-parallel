package csdf

import (
	"encoding/json"

	"github.com/Kuniwak/puml-parallel/v8/csdf/clause"
)

// The concrete syntax tree of a diagram (docs/SYNTAX.md, docs/PROMOTION.md):
// what Parse reads, every element of the source in the order it is written,
// each with the position it starts at, and nothing of its meaning. A plain
// diagram and a global one read alike; Plain is the plain diagram a tree
// says. Whitespace is not kept.

// SyntaxTreeKind and SyntaxTreeVersion name the JSON of a SyntaxTree,
// apart from core.DiagramDoc's.
const (
	SyntaxTreeKind    = "csdf-syntax-tree"
	SyntaxTreeVersion = 1
)

// Pos is a position in the source: its 1-based line, and its 1-based column
// counted in Unicode code points.
type Pos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

// SyntaxTree is a diagram as written, from @startuml to @enduml.
type SyntaxTree struct {
	Pos
	// Path is the path of the file the source was read from, "" when it is
	// not known, such as for standard input.
	Path string
	// Name is the rest of the @startuml line, as written.
	Name string
	Body []Element
}

// Element is an element of the body of a diagram or of a state block, or a
// line of a note: one of the *...Syntax types of this file. It is a sealed
// sum type, a deliberate exception to preferring function types over
// one-method interfaces: no function type could tell the cases apart, and
// no package outside this one adds a case.
type Element interface{ Position() Pos }

// Position is where the element starts.
func (p Pos) Position() Pos { return p }

// CommentSyntax is a comment: a line comment "' text", or a block comment
// "/' text '/" (Block). A comment written inside a line comes after the
// element of that line, by its position.
type CommentSyntax struct {
	Pos
	Text  string
	Block bool
}

// PragmaSyntax is a line of PlantUML's preprocessor, "!name arg", such as
// !include, whose Arg is the path as written, or !define. Parse does not
// resolve it; an !include resolved (coreio.ResolveIncludes) holds in Tree
// the tree of the file it names, nil until then; no other pragma holds one.
type PragmaSyntax struct {
	Pos
	Name string
	Arg  string
	Tree *SyntaxTree
}

// IgnoreSyntax is a CSDF-IGNORE region: the lines between its markers, as
// written.
type IgnoreSyntax struct {
	Pos
	Lines []string
}

// ObserveSyntax is a CSDF-OBSERVE line: the view Name of States, or, when
// it is no View, the variable Name.
type ObserveSyntax struct {
	Pos
	Name   string
	View   bool
	States []string
}

// StateSyntax is state "Label" as ID, with its stereotype <<Stereotype>>
// if any, and its block { Body } when Composite.
type StateSyntax struct {
	Pos
	ID         string
	Label      string
	Stereotype string
	Composite  bool
	Body       []Element
}

// StateVarSyntax is "State : Name", or "State : Name ; Type".
type StateVarSyntax struct {
	Pos
	State string
	Name  string
	Type  string
}

// ClauseSyntax is a clause as written: its text and its syntax tree.
type ClauseSyntax struct {
	Pos
	Text   string
	Syntax clause.Syntax
}

// EventSyntax is the event of an edge or of a constrain directive.
type EventSyntax struct {
	Pos
	Text    string
	Channel string
	Fields  []FieldSyntax
}

// FieldSyntax is a field of an event: ?Name, received, or !Name, sent.
type FieldSyntax struct {
	Dir  string `json:"dir"`
	Name string `json:"name"`
}

// StartEdgeSyntax is "[*] --> Dst", and ": Post" if written.
type StartEdgeSyntax struct {
	Pos
	Dst  string
	Post *ClauseSyntax
}

// EdgeSyntax is "Src --> Dst : Event ; Guard ; Post"; a guard or post not
// written is nil.
type EdgeSyntax struct {
	Pos
	Src   string
	Dst   string
	Event EventSyntax
	Guard *ClauseSyntax
	Post  *ClauseSyntax
}

// EndEdgeSyntax is "Src --> [*]", and ": Guard" if written.
type EndEdgeSyntax struct {
	Pos
	Src   string
	Guard *ClauseSyntax
}

// NoteSyntax is a note: floating, "note as ID", or anchored, "note Side of
// Of", and its lines up to "end note", blank ones left out, its line
// comments among them where they are written. When its first line but the
// comments opens a directive (assume, channel, sync, constrain), each line
// is read as a directive or prose; otherwise every line is prose.
type NoteSyntax struct {
	Pos
	ID    string
	Side  string
	Of    string
	Lines []Element
}

// Content gives the lines of the note but its comments, in order: what the
// note says.
func (n *NoteSyntax) Content() []Element {
	var out []Element
	for _, l := range n.Lines {
		if _, ok := l.(*CommentSyntax); !ok {
			out = append(out, l)
		}
	}
	return out
}

// AssumeSyntax is "assume Text".
type AssumeSyntax struct {
	Pos
	Text string
}

// ChannelSyntax is "channel Text": "Name" or "Name : Fields[0].Fields[1]…",
// each field's type as written.
type ChannelSyntax struct {
	Pos
	Text   string
	Name   string
	Fields []string
}

// SyncSyntax is "sync Event : Targets ; Guard".
type SyncSyntax struct {
	Pos
	Event   string
	Targets []SyncTargetSyntax
	Guard   *ClauseSyntax
}

// SyncTargetSyntax is "Map(ID)" or "Map(ID).Event".
type SyncTargetSyntax struct {
	Map   string `json:"map"`
	ID    string `json:"id"`
	Event string `json:"event,omitempty"`
}

// ConstrainSyntax is "constrain Event ; Guard"; Guard is nil when no ";"
// is written.
type ConstrainSyntax struct {
	Pos
	Event EventSyntax
	Guard *ClauseSyntax
}

// ProseSyntax is a line of a note that is no directive.
type ProseSyntax struct {
	Pos
	Text string
}

// MarshalSyntaxTree is t as JSON (docs/SYNTAX.md, "The syntax tree as
// JSON").
func MarshalSyntaxTree(t *SyntaxTree) ([]byte, error) {
	return json.Marshal(struct {
		Kind    string    `json:"kind"`
		Version int       `json:"version"`
		Path    string    `json:"path,omitempty"`
		Name    string    `json:"name"`
		Line    int       `json:"line"`
		Col     int       `json:"col"`
		Body    []Element `json:"body"`
	}{SyntaxTreeKind, SyntaxTreeVersion, t.Path, t.Name, t.Line, t.Col, nonNil(t.Body)})
}

func nonNil[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

func (c *CommentSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Text  string `json:"text"`
		Block bool   `json:"block"`
	}{"comment", c.Pos, c.Text, c.Block})
}

func (p *PragmaSyntax) MarshalJSON() ([]byte, error) {
	var tree json.RawMessage
	if p.Tree != nil {
		bs, err := MarshalSyntaxTree(p.Tree)
		if err != nil {
			return nil, err
		}
		tree = bs
	}
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Name string          `json:"name"`
		Arg  string          `json:"arg"`
		Tree json.RawMessage `json:"tree,omitempty"`
	}{"pragma", p.Pos, p.Name, p.Arg, tree})
}

func (i *IgnoreSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Lines []string `json:"lines"`
	}{"ignore", i.Pos, nonNil(i.Lines)})
}

func (o *ObserveSyntax) MarshalJSON() ([]byte, error) {
	var states []string
	if o.View {
		states = nonNil(o.States)
	}
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Name   string   `json:"name"`
		View   bool     `json:"view"`
		States []string `json:"states,omitempty"`
	}{"observe", o.Pos, o.Name, o.View, states})
}

func (s *StateSyntax) MarshalJSON() ([]byte, error) {
	var body *[]Element
	if s.Composite {
		b := nonNil(s.Body)
		body = &b
	}
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		ID         string     `json:"id"`
		Label      string     `json:"label"`
		Stereotype string     `json:"stereotype,omitempty"`
		Body       *[]Element `json:"body,omitempty"`
	}{"state", s.Pos, s.ID, s.Label, s.Stereotype, body})
}

func (v *StateVarSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		State string `json:"state"`
		Name  string `json:"name"`
		Type  string `json:"type,omitempty"`
	}{"state_var", v.Pos, v.State, v.Name, v.Type})
}

func (c *ClauseSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Pos
		Text      string               `json:"text"`
		Conjuncts []clause.SynConjunct `json:"conjuncts"`
	}{c.Pos, c.Text, nonNil(c.Syntax.Conjuncts)})
}

func (e EventSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Pos
		Text    string        `json:"text"`
		Channel string        `json:"channel"`
		Fields  []FieldSyntax `json:"fields"`
	}{e.Pos, e.Text, e.Channel, nonNil(e.Fields)})
}

func (e *StartEdgeSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Dst  string        `json:"dst"`
		Post *ClauseSyntax `json:"post"`
	}{"start_edge", e.Pos, e.Dst, e.Post})
}

func (e *EdgeSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Src   string        `json:"src"`
		Dst   string        `json:"dst"`
		Event EventSyntax   `json:"event"`
		Guard *ClauseSyntax `json:"guard"`
		Post  *ClauseSyntax `json:"post"`
	}{"edge", e.Pos, e.Src, e.Dst, e.Event, e.Guard, e.Post})
}

func (e *EndEdgeSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Src   string        `json:"src"`
		Guard *ClauseSyntax `json:"guard"`
	}{"end_edge", e.Pos, e.Src, e.Guard})
}

func (n *NoteSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		ID    string    `json:"id,omitempty"`
		Side  string    `json:"side,omitempty"`
		Of    string    `json:"of,omitempty"`
		Lines []Element `json:"lines"`
	}{"note", n.Pos, n.ID, n.Side, n.Of, nonNil(n.Lines)})
}

func (a *AssumeSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Text string `json:"text"`
	}{"assume", a.Pos, a.Text})
}

func (c *ChannelSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Name   string   `json:"name"`
		Fields []string `json:"fields"`
	}{"channel", c.Pos, c.Name, nonNil(c.Fields)})
}

func (s *SyncSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Event   string             `json:"event"`
		Targets []SyncTargetSyntax `json:"targets"`
		Guard   *ClauseSyntax      `json:"guard"`
	}{"sync", s.Pos, s.Event, nonNil(s.Targets), s.Guard})
}

func (c *ConstrainSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Event EventSyntax   `json:"event"`
		Guard *ClauseSyntax `json:"guard"`
	}{"constrain", c.Pos, c.Event, c.Guard})
}

func (p *ProseSyntax) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Pos
		Text string `json:"text"`
	}{"text", p.Pos, p.Text})
}
