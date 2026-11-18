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

func TestHandleBackDropsThePendingTransition(t *testing.T) {
	// Setup
	service, id := advance(t)
	if resp := service.Handle(Request{Command: CommandSelect, Session: id, Index: intPtr(0)}); !resp.OK {
		t.Fatalf("select failed: %s", resp.Error)
	}

	// Execute
	resp := service.Handle(Request{Command: CommandBack, Session: id})

	// Assert
	if !resp.OK {
		t.Fatalf("back failed: %s", resp.Error)
	}
	view := decodeView(t, resp)
	if view.Mode != "command" || view.State == nil || view.State.ID != "s0" {
		t.Errorf("back view = %+v, want command mode at s0", view)
	}
}

func TestHandleBackRefusesWithNoHistory(t *testing.T) {
	// Setup
	service, id := newSession(t)

	// Execute
	resp := service.Handle(Request{Command: CommandBack, Session: id})

	// Assert
	if resp.OK {
		t.Error("back OK = true, want failure")
	}
}
