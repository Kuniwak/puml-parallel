package animation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
)

// Value is a value bound to a state variable, as JSON decodes it (numbers
// as json.Number): a map of a family's instances is an object from IDs to
// {"at": <state>, "fields": {...}}, as csdfbmc writes it.
type Value struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// State is a state of the diagram with values bound to its variables, in
// their declaration order: the runtime state csdfbmc next reads.
type State struct {
	ID     string  `json:"state_id"`
	Name   string  `json:"state_name"`
	Values []Value `json:"values"`
}

// Target is a state awaiting values: its ID, its label, and the names of
// its variables in declaration order.
type Target struct {
	ID   string   `json:"state_id"`
	Name string   `json:"state_name"`
	Vars []string `json:"vars"`
}

// RefusedValuesError says why entered values do not fit the target state;
// a front end shows it and asks again.
type RefusedValuesError struct {
	Reason string
}

func (e *RefusedValuesError) Error() string { return e.Reason }

func refused(format string, args ...any) error {
	return &RefusedValuesError{Reason: fmt.Sprintf(format, args...)}
}

// StateName is the label of the state id of d, or its ID when it has none.
func StateName(d *core.Diagram, id string) string {
	if l, ok := d.Labels[id]; ok {
		return l
	}
	return id
}

// targetOf is the state id of d awaiting values.
func targetOf(d *core.Diagram, id string) Target {
	return Target{ID: id, Name: StateName(d, id), Vars: slices.Clone(d.Vars[id])}
}

// family is the family whose instances the map name holds, if any.
func family(d *core.Diagram, name string) (core.FamilyStates, bool) {
	i := slices.IndexFunc(d.Families, func(f core.FamilyStates) bool { return f.Map == name })
	if i < 0 {
		return core.FamilyStates{}, false
	}
	return d.Families[i], true
}

// decodeValues reads encoded, a JSON array of the values of target's
// variables in declaration order, and checks each map of a family's
// instances against the family. What does not fit is a RefusedValuesError.
func decodeValues(d *core.Diagram, target Target, encoded string) ([]Value, error) {
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, refused("invalid JSON array: %v", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, refused("unexpected trailing data after JSON array")
	}
	values, ok := clause.UnsignedZeros(decoded).([]any)
	if !ok {
		return nil, refused("invalid JSON array: top-level value must be an array")
	}
	if slices.ContainsFunc(values, containsNull) {
		return nil, refused("null is not a valid state variable value")
	}
	if len(values) != len(target.Vars) {
		return nil, refused("expected %d value(s) for [%s], got %d", len(target.Vars), strings.Join(target.Vars, ", "), len(values))
	}
	out := make([]Value, len(values))
	for i, v := range values {
		name := target.Vars[i]
		if f, ok := family(d, name); ok {
			if err := checkInstances(f, v); err != nil {
				return nil, err
			}
		}
		out[i] = Value{Name: name, Value: v}
	}
	return out, nil
}

// checkInstances refuses v unless it is a map of f's instances: an object
// from IDs to {"at": s, "fields": {...}}, s a state of f other than its
// start state (absence), the fields exactly the variables s holds.
func checkInstances(f core.FamilyStates, v any) error {
	entries, ok := v.(map[string]any)
	if !ok {
		return refused(`%s: a map of instances is a JSON object from IDs to {"at": <state>, "fields": {...}}`, f.Map)
	}
	for _, id := range slices.Sorted(maps.Keys(entries)) {
		at, fields, ok := instance(entries[id])
		if !ok {
			return refused(`%s[%q]: an instance is {"at": <state>, "fields": {...}}`, f.Map, id)
		}
		if at == f.Init {
			return refused("%s[%q]: %q is the start state of the family of %s, which is absence; leave the entry out", f.Map, id, at, f.Map)
		}
		if !slices.Contains(f.States, at) {
			others := slices.DeleteFunc(slices.Clone(f.States), func(s string) bool { return s == f.Init })
			return refused("%s[%q]: %q is no state of the family of %s; it is at one of [%s]", f.Map, id, at, f.Map, strings.Join(others, ", "))
		}
		want := slices.Sorted(slices.Values(f.Vars[at]))
		got := slices.Sorted(maps.Keys(fields))
		if !slices.Equal(want, got) {
			return refused("%s[%q]: an instance at %s holds the fields [%s], got [%s]", f.Map, id, at, strings.Join(want, ", "), strings.Join(got, ", "))
		}
	}
	return nil
}

// instance takes v apart as {"at": <state>, "fields": {...}}.
func instance(v any) (string, map[string]any, bool) {
	obj, ok := v.(map[string]any)
	if !ok || len(obj) != 2 {
		return "", nil, false
	}
	at, ok1 := obj["at"].(string)
	fields, ok2 := obj["fields"].(map[string]any)
	return at, fields, ok1 && ok2
}

func containsNull(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case []any:
		return slices.ContainsFunc(v, containsNull)
	case map[string]any:
		for _, x := range v {
			if containsNull(x) {
				return true
			}
		}
	}
	return false
}
