package csdf

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Direction says who chooses the value of an event's field (docs/SYNTAX.md).
type Direction string

const (
	// Received is a field whose value the environment chooses: c?x.
	Received Direction = "?"
	// Chosen is a field whose value the diagram chooses internally: c!y.
	Chosen Direction = "!"
)

// Field is one field of an event: its direction and the name its clauses read
// the value by. An event carries names only, never values or expressions.
type Field struct {
	Dir  Direction `json:"dir"`
	Name string    `json:"name"`
}

// ParseEvent reads an event of the grammar channel { ("?" | "!") ident }, and
// writes it without the whitespace around ? and !.
func ParseEvent(text string) (Event, error) {
	channel, fields, err := splitEvent(text)
	if err != nil {
		return "", err
	}
	return NewEvent(channel, fields...), nil
}

// ParseChannel reads a channel: an event's name without its fields, which is
// what a composition synchronises on.
func ParseChannel(text string) (string, error) {
	channel := strings.TrimSpace(text)
	if !isID(channel) {
		return "", fmt.Errorf("csdf.ParseChannel: %q is not a channel, which is an identifier with no ?field or !field", text)
	}
	if Event(channel) == Tau {
		return "", fmt.Errorf("csdf.ParseChannel: tau is the internal event, which is neither synchronised nor hidden")
	}
	return channel, nil
}

// NewEvent writes the event on channel with the given fields.
func NewEvent(channel string, fields ...Field) Event {
	var sb strings.Builder
	sb.WriteString(channel)
	for _, f := range fields {
		sb.WriteString(string(f.Dir))
		sb.WriteString(f.Name)
	}
	return Event(sb.String())
}

// Channel is the name the event synchronises on. Channel and Fields take
// apart an event of the grammar, as ParseEvent, NewEvent, the parser and an
// edge's JSON make them; they read the text each time, since an event is short.
func (e Event) Channel() string {
	channel, _, _ := splitEvent(string(e))
	return channel
}

// Fields are the event's fields in the order they are written.
func (e Event) Fields() []Field {
	_, fields, _ := splitEvent(string(e))
	return fields
}

func splitEvent(text string) (string, []Field, error) {
	i := strings.IndexAny(text, "?!")
	if i < 0 {
		i = len(text)
	}
	channel := strings.TrimSpace(text[:i])
	if !isID(channel) {
		return "", nil, fmt.Errorf("csdf.ParseEvent: %q: expected a channel, then ?field or !field, each an identifier", text)
	}
	var fields []Field
	for rest := text[i:]; rest != ""; {
		dir := Direction(rest[:1])
		rest = rest[1:]
		j := strings.IndexAny(rest, "?!")
		if j < 0 {
			j = len(rest)
		}
		name := strings.TrimSpace(rest[:j])
		if !isID(name) {
			return "", nil, fmt.Errorf("csdf.ParseEvent: %q: expected an identifier after %q", text, dir)
		}
		fields = append(fields, Field{Dir: dir, Name: name})
		rest = rest[j:]
	}
	return channel, fields, nil
}

func isID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isIDRune(r) {
			return false
		}
	}
	return true
}

// edgeJSON is an edge as JSON: the event is written whole and also taken
// apart, so a reader in another language need not parse it again.
type edgeJSON struct {
	Src          StateID      `json:"src"`
	Dst          StateID      `json:"dst"`
	Event        Event        `json:"event"`
	Channel      string       `json:"channel"`
	Fields       []Field      `json:"fields"`
	Guard        Predicate    `json:"guard"`
	GuardClauses []ClauseJSON `json:"guard_clauses"`
	Post         Predicate    `json:"post"`
	PostClauses  []ClauseJSON `json:"post_clauses"`
	Line         int          `json:"line"`
}

func (e Edge) MarshalJSON() ([]byte, error) {
	fields := e.Event.Fields()
	if fields == nil {
		fields = []Field{}
	}
	guard, err := e.Guard.ClausesJSON()
	if err != nil {
		return nil, err
	}
	post, err := e.Post.ClausesJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(edgeJSON{Src: e.Src, Dst: e.Dst, Event: e.Event, Channel: e.Event.Channel(), Fields: fields, Guard: e.Guard, GuardClauses: guard, Post: e.Post, PostClauses: post, Line: e.Line})
}

// UnmarshalJSON reads the event, guard and post whole, the event by the
// grammar, and derives the rest from them: the texts win over the channel,
// fields and clauses, which are only a reader's convenience.
func (e *Edge) UnmarshalJSON(data []byte) error {
	var j edgeJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	event, err := ParseEvent(string(j.Event))
	if err != nil {
		return err
	}
	*e = Edge{Src: j.Src, Dst: j.Dst, Event: event, Guard: j.Guard, Post: j.Post, Line: j.Line}
	return nil
}
