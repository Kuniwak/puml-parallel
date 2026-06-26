package animation

import (
	"bytes"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/google/go-cmp/cmp"
)

func TestRenderStateValuePromptForInitialState(t *testing.T) {
	var buf bytes.Buffer
	RenderStateValuePrompt(&buf, nil, csdf.State{
		ID:   "initial",
		Name: "Initial",
		Vars: []csdf.StateVar{
			{Name: "count", Type: "number"},
			{Name: "metadata"},
		},
	}, "", "")

	want := "" +
		"State: (none)\n" +
		"\n" +
		"Guard:\n" +
		"  true\n" +
		"\n" +
		"Post State Group:\n" +
		"  Initial\n" +
		"    count' as number\n" +
		"    metadata' as any\n" +
		"\n" +
		"Post Condition:\n" +
		"  true\n" +
		"\n" +
		"Enter 2 values as a JSON array in declaration order: [<count>, <metadata>].\n" +
		"\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Error(diff)
	}
}

func TestRenderStateWithTransitions(t *testing.T) {
	diagram := &csdf.Diagram{
		States: map[csdf.StateID]csdf.State{
			"review":   {ID: "review", Name: "Review order"},
			"approved": {ID: "approved", Name: "Approved"},
		},
		Edges: []csdf.Edge{
			{Src: "review", Dst: "approved", Event: "approve", Guard: "count > 0", Post: `status' = "approved"`},
		},
	}
	state := csdf.RuntimeState{
		ID:     "review",
		Name:   "Review order",
		Values: []csdf.StateValue{{Name: "count", Value: 2}},
	}

	var buf bytes.Buffer
	RenderState(&buf, diagram, state)

	want := "" +
		"State: Review order (review)\n" +
		"\n" +
		"Values:\n" +
		"  count = 2\n" +
		"\n" +
		"Transitions:\n" +
		"  [0] approve -> Approved (approved)\n" +
		"      Guard: count > 0\n" +
		`      Post: status' = "approved"` + "\n" +
		"\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Error(diff)
	}
}

func TestRenderStateDeadlock(t *testing.T) {
	var buf bytes.Buffer
	RenderState(&buf, &csdf.Diagram{}, csdf.RuntimeState{ID: "end", Name: "End"})

	want := "" +
		"State: End (end)\n" +
		"Values:\n" +
		"  (none)\n" +
		"\n" +
		"Deadlock: no outgoing transitions.\n" +
		"\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Error(diff)
	}
}

func TestRenderTrace(t *testing.T) {
	var buf bytes.Buffer
	RenderTrace(&buf, []csdf.Event{"submit(order)", "approve"})

	want := "" +
		"Trace:\n" +
		"  [\n" +
		`    "submit(order)",` + "\n" +
		`    "approve"` + "\n" +
		"  ]\n" +
		"\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Error(diff)
	}
}

func TestRenderHelp(t *testing.T) {
	var buf bytes.Buffer
	RenderHelp(&buf)

	want := "" +
		"Commands:\n" +
		"  l         list the current state and transitions\n" +
		"  t         display the current trace\n" +
		"  h         display history\n" +
		"  s INDEX   select a transition\n" +
		"  j INDEX   jump to a history entry\n" +
		"  ?, help   display this help\n" +
		"\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Error(diff)
	}
}
