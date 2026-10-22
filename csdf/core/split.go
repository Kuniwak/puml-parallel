package core

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

// EventField is a field of an event as SplitEvent takes it apart: sent (!)
// or received (?), and what stands for it.
type EventField struct {
	Send bool
	Name string
}

// FieldRule is what stands for a field of an event: Check accepts it, and
// Name says what it is, for an error of an event it refuses.
type FieldRule struct {
	Name  string
	Check func(string) bool
}

// The FieldRules: a diagram's event names its fields; a trace's event may
// carry their values instead.
var (
	FieldIdentifier  = FieldRule{Name: "an identifier", Check: ident.Is}
	FieldNameOrValue = FieldRule{Name: "a name or a value", Check: func(s string) bool { return s != "" }}
)

// SplitEvent takes apart an event written as a channel, then ?x for each
// field received and !x for each sent, in order, each trimmed of spaces. The
// channel is an identifier, and each x is one field accepts. Neither may
// hold ? or !, which mark the fields.
func SplitEvent(text string, field FieldRule) (string, []EventField, error) {
	i := strings.IndexAny(text, "?!")
	if i < 0 {
		i = len(text)
	}
	channel := strings.TrimSpace(text[:i])
	if !ident.Is(channel) {
		return "", nil, fmt.Errorf("%q: expected a channel, an identifier, then ?field or !field", text)
	}
	var fields []EventField
	for rest := text[i:]; rest != ""; {
		send := rest[0] == '!'
		marker := rest[:1]
		rest = rest[1:]
		j := strings.IndexAny(rest, "?!")
		if j < 0 {
			j = len(rest)
		}
		name := strings.TrimSpace(rest[:j])
		if !field.Check(name) {
			return "", nil, fmt.Errorf("%q: expected %s after %q", text, field.Name, marker)
		}
		fields = append(fields, EventField{Send: send, Name: name})
		rest = rest[j:]
	}
	return channel, fields, nil
}
