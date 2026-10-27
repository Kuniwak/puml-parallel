package csdf

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

// Direction says who chooses the value of an event's field (docs/SYNTAX.md).
type Direction string

const (
	// Received is a field whose value the environment chooses: c?x.
	Received Direction = "?"
	// Sent is a field whose value the event sends: c!y sends the value y has
	// at the step.
	Sent Direction = "!"
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
	if !ident.Is(channel) {
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
	channel, parts, err := core.SplitEvent(text, core.FieldIdentifier)
	if err != nil {
		return "", nil, fmt.Errorf("csdf.ParseEvent: %w", err)
	}
	fields := make([]Field, len(parts))
	for i, p := range parts {
		dir := Received
		if p.Send {
			dir = Sent
		}
		fields[i] = Field{Dir: dir, Name: p.Name}
	}
	return channel, fields, nil
}
