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

// SplitEvent takes apart an event written as a channel, then ?x for each
// field received and !x for each sent, in order, each trimmed of spaces. The
// channel is an identifier, and each x is one field accepts: an identifier
// for a diagram's event, a name or a value for a trace's. Neither may hold
// ? or !, which mark the fields.
func SplitEvent(text string, field func(string) bool) (string, []EventField, error) {
	i := strings.IndexAny(text, "?!")
	if i < 0 {
		i = len(text)
	}
	channel := strings.TrimSpace(text[:i])
	if !ident.Is(channel) {
		return "", nil, fmt.Errorf("%q: expected a channel, then ?field or !field", text)
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
		if !field(name) {
			return "", nil, fmt.Errorf("%q: expected a field after %q", text, marker)
		}
		fields = append(fields, EventField{Send: send, Name: name})
		rest = rest[j:]
	}
	return channel, fields, nil
}
