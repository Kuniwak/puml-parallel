package proto

import (
	"testing"
)

func TestHandleReadViewCarriesTheLineOfEachTransition(t *testing.T) {
	// Setup
	service, id := advance(t)

	// Execute
	resp := service.Handle(Request{Command: CommandRead, Session: id})

	// Assert
	view := decodeView(t, resp)
	if len(view.Transitions) != 1 || view.Transitions[0].Line != 7 {
		t.Errorf("transitions = %+v, want one at line 7", view.Transitions)
	}
}
