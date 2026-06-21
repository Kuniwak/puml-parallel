package csdf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// StateValue is a resolved value for a single state variable.
type StateValue struct {
	Name  Var `json:"name"`
	Value any `json:"value"`
}

// RuntimeState is a state group with concrete values bound to its variables.
type RuntimeState struct {
	ID     StateID      `json:"state_id"`
	Name   string       `json:"state_name"`
	Values []StateValue `json:"values"`
}

type PostSolverResultKind int

const (
	PostSolverResultOK PostSolverResultKind = iota
	PostSolverResultNoSolutions
	PostSolverResultInvalidStateVarValuesLength
	PostSolverResultSyntaxError
)

// PostSolverInput is the request to resolve entered values against a post
// state group, given the previous state and the guard/post conditions.
type PostSolverInput struct {
	StateGroup    State
	Previous      *RuntimeState
	Guard         string
	Post          string
	EncodedValues string
}

type PostSolverResult struct {
	Kind  PostSolverResultKind
	State RuntimeState
	Err   error
}

// PostSolver resolves entered state-variable values against a post state group.
// It is interface-independent: a CLI, Web, or RPC front-end can drive the same
// exploration engine through it.
type PostSolver func(PostSolverInput) PostSolverResult

// SolveJSON is a PostSolver that decodes the entered values as a JSON array in
// the post state group's variable declaration order.
func SolveJSON(input PostSolverInput) PostSolverResult {
	decoder := json.NewDecoder(strings.NewReader(input.EncodedValues))
	decoder.UseNumber()

	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return PostSolverResult{Kind: PostSolverResultSyntaxError, Err: fmt.Errorf("csdf.SolveJSON: invalid JSON array: %w", err)}
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return PostSolverResult{Kind: PostSolverResultSyntaxError, Err: fmt.Errorf("csdf.SolveJSON: %w", err)}
	}
	values, ok := decoded.([]any)
	if !ok {
		return PostSolverResult{Kind: PostSolverResultSyntaxError, Err: errors.New("csdf.SolveJSON: invalid JSON array: top-level value must be an array")}
	}
	for _, value := range values {
		if containsNull(value) {
			return PostSolverResult{Kind: PostSolverResultSyntaxError, Err: errors.New("csdf.SolveJSON: null is not a supported JSON value")}
		}
	}
	if len(values) != len(input.StateGroup.Vars) {
		return PostSolverResult{Kind: PostSolverResultInvalidStateVarValuesLength}
	}

	stateValues := make([]StateValue, len(values))
	for i, value := range values {
		stateValues[i] = StateValue{Name: input.StateGroup.Vars[i].Name, Value: value}
	}
	return PostSolverResult{
		Kind: PostSolverResultOK,
		State: RuntimeState{
			ID:     input.StateGroup.ID,
			Name:   input.StateGroup.Name,
			Values: stateValues,
		},
	}
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("csdf.ensureJSONEOF: invalid JSON array: %w", err)
	}
	return errors.New("csdf.ensureJSONEOF: invalid JSON array: multiple JSON values")
}

func containsNull(value any) bool {
	switch value := value.(type) {
	case nil:
		return true
	case []any:
		for _, item := range value {
			if containsNull(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range value {
			if containsNull(item) {
				return true
			}
		}
	}
	return false
}
