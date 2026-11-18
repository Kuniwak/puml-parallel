package proto

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/animation"
	"github.com/google/go-cmp/cmp"
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

func TestHandleErrorsCarryACode(t *testing.T) {
	tests := map[string]struct {
		prepare  []Request
		request  Request
		wantCode string
		wantArgs map[string]string
	}{
		"refused values": {
			request:  Request{Command: CommandStatevar, Values: "[1, 2]"},
			wantCode: ErrorCodeRefusedValues,
			wantArgs: map[string]string{"reason": "expected 1 value(s) for [count], got 2"},
		},
		"an index out of range": {
			prepare:  []Request{{Command: CommandStatevar, Values: "[0]"}},
			request:  Request{Command: CommandSelect, Index: intPtr(5)},
			wantCode: ErrorCodeIndexOutOfRange,
		},
		"a run that ended": {
			prepare: []Request{
				{Command: CommandStatevar, Values: "[0]"},
				{Command: CommandSelect, Index: intPtr(0)},
				{Command: CommandStatevar, Values: `["done"]`},
				{Command: CommandEnd, Index: intPtr(0)},
			},
			request:  Request{Command: CommandSelect, Index: intPtr(0)},
			wantCode: ErrorCodeEnded,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			service := NewService("test-version", false, AnyEnvironment, animation.NoSuggesters)
			diagram := strings.Replace(twoStateDiagram, "note as", "s1 --> [*]\nnote as", 1)
			id := service.Handle(Request{Command: CommandSessionNew, Content: []byte(diagram)}).Session
			for _, req := range tt.prepare {
				req.Session = id
				if resp := service.Handle(req); !resp.OK {
					t.Fatalf("%s failed: %s", req.Command, resp.Error)
				}
			}
			tt.request.Session = id

			// Execute
			resp := service.Handle(tt.request)

			// Assert
			if resp.OK || resp.Error == "" {
				t.Fatalf("response = %+v, want a failure with its text", resp)
			}
			if resp.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", resp.Code, tt.wantCode)
			}
			if diff := cmp.Diff(tt.wantArgs, resp.Args); diff != "" {
				t.Errorf("args mismatch (-want +got):\n%s", diff)
			}
		})
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
