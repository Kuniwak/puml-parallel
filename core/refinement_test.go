package core

import (
	"fmt"
	"testing"
)

func TestStableFailuresRefinement(t *testing.T) {
	testCases := []struct {
		name         string
		spec         *Diagram
		impl         *Diagram
		expectFail   bool
		expectedType ProofObligationType
	}{
		{
			name: "Case 1: With Internal Transitions",
			spec: createSpecWithInternal(),
			impl: createImplWithInternal(),
			expectFail: false,
		},
		{
			name: "Case 2: No Internal Transitions", 
			spec: createSpecNoInternal(),
			impl: createImplNoInternal(),
			expectFail: false,
		},
		{
			name: "Case 3: Guard Condition Fails",
			spec: createSpecGuardFail(),
			impl: createImplGuardFail(), 
			expectFail: true,
			expectedType: GuardWeakening,
		},
		{
			name: "Case 4: Postcondition Fails",
			spec: createSpecPostFail(),
			impl: createImplPostFail(),
			expectFail: true,
			expectedType: PostconditionStrength,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			verifier := NewStableFailuresVerifier(tc.spec, tc.impl)
			obligations, err := verifier.GenerateStableFailuresProofObligations()
			
			if err != nil {
				t.Fatalf("Failed to generate proof obligations: %v", err)
			}

			fmt.Printf("\n=== %s ===\n", tc.name)
			fmt.Print(FormatProofObligations(obligations))
			
			if tc.expectFail {
				// Check if we have the expected failing obligation type
				found := false
				for _, obligation := range obligations {
					if obligation.Type == tc.expectedType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find obligation of type %s, but didn't", tc.expectedType)
				}
			}
		})
	}
}

// Helper functions to create test diagrams

func createSpecWithInternal() *Diagram {
	return &Diagram{
		States: map[StateID]State{
			"s0": {ID: "s0", Name: "waiting", Vars: []Var{"ready"}},
			"s1": {ID: "s1", Name: "processing", Vars: []Var{"processing"}},
			"s2": {ID: "s2", Name: "done", Vars: []Var{"done"}},
		},
		StartEdge: StartEdge{
			Dst:  "s0",
			Post: "ready = true",
		},
		Edges: []Edge{
			{Src: "s0", Dst: "s1", Event: Event{ID: "start"}, Guard: "ready = true", Post: "ready' = false, processing = true"},
			{Src: "s1", Dst: "s1", Event: Event{ID: "tau"}, Guard: "processing = true", Post: "processing' = true (internal computation)"},
			{Src: "s1", Dst: "s2", Event: Event{ID: "finish"}, Guard: "processing = true", Post: "processing' = false, done = true"},
		},
	}
}

func createImplWithInternal() *Diagram {
	return &Diagram{
		States: map[StateID]State{
			"s0":  {ID: "s0", Name: "waiting", Vars: []Var{"ready"}},
			"s1":  {ID: "s1", Name: "processing", Vars: []Var{"processing"}},
			"s1a": {ID: "s1a", Name: "optimizing", Vars: []Var{"optimizing"}},
			"s2":  {ID: "s2", Name: "done", Vars: []Var{"done"}},
		},
		StartEdge: StartEdge{
			Dst:  "s0",
			Post: "ready = true",
		},
		Edges: []Edge{
			{Src: "s0", Dst: "s1", Event: Event{ID: "start"}, Guard: "ready = true", Post: "ready' = false, processing = true"},
			{Src: "s1", Dst: "s1a", Event: Event{ID: "tau"}, Guard: "processing = true", Post: "processing' = true, optimizing = true"},
			{Src: "s1a", Dst: "s1", Event: Event{ID: "tau"}, Guard: "optimizing = true", Post: "optimizing' = false, processing = true"},
			{Src: "s1", Dst: "s2", Event: Event{ID: "finish"}, Guard: "processing = true", Post: "processing' = false, done = true"},
		},
	}
}

func createSpecNoInternal() *Diagram {
	return &Diagram{
		States: map[StateID]State{
			"s0": {ID: "s0", Name: "idle", Vars: []Var{"initialized"}},
			"s1": {ID: "s1", Name: "active", Vars: []Var{"active"}},
			"s2": {ID: "s2", Name: "completed", Vars: []Var{"completed"}},
		},
		StartEdge: StartEdge{
			Dst:  "s0",
			Post: "initialized = true",
		},
		Edges: []Edge{
			{Src: "s0", Dst: "s1", Event: Event{ID: "activate"}, Guard: "initialized = true", Post: "initialized' = false, active = true"},
			{Src: "s1", Dst: "s2", Event: Event{ID: "complete"}, Guard: "active = true", Post: "active' = false, completed = true"},
		},
	}
}

func createImplNoInternal() *Diagram {
	return &Diagram{
		States: map[StateID]State{
			"s0": {ID: "s0", Name: "idle", Vars: []Var{"initialized"}},
			"s1": {ID: "s1", Name: "active", Vars: []Var{"active"}},
			"s2": {ID: "s2", Name: "completed", Vars: []Var{"completed"}},
		},
		StartEdge: StartEdge{
			Dst:  "s0",
			Post: "initialized = true",
		},
		Edges: []Edge{
			{Src: "s0", Dst: "s1", Event: Event{ID: "activate"}, Guard: "initialized = true", Post: "initialized' = false, active = true"},
			{Src: "s1", Dst: "s2", Event: Event{ID: "complete"}, Guard: "active = true", Post: "active' = false, completed = true"},
		},
	}
}

func createSpecGuardFail() *Diagram {
	return &Diagram{
		States: map[StateID]State{
			"s0": {ID: "s0", Name: "ready", Vars: []Var{"balance"}},
			"s1": {ID: "s1", Name: "working", Vars: []Var{}},
			"s2": {ID: "s2", Name: "finished", Vars: []Var{"confirmed"}},
		},
		StartEdge: StartEdge{
			Dst:  "s0",
			Post: "balance = 100",
		},
		Edges: []Edge{
			{Src: "s0", Dst: "s1", Event: Event{ID: "withdraw", Params: []Var{"amount"}}, Guard: "balance >= amount", Post: "balance' = balance - amount"},
			{Src: "s1", Dst: "s2", Event: Event{ID: "confirm"}, Guard: "always", Post: "confirmed = true"},
		},
	}
}

func createImplGuardFail() *Diagram {
	return &Diagram{
		States: map[StateID]State{
			"s0": {ID: "s0", Name: "ready", Vars: []Var{"balance"}},
			"s1": {ID: "s1", Name: "working", Vars: []Var{}},
			"s2": {ID: "s2", Name: "finished", Vars: []Var{"confirmed"}},
		},
		StartEdge: StartEdge{
			Dst:  "s0",
			Post: "balance = 100",
		},
		Edges: []Edge{
			{Src: "s0", Dst: "s1", Event: Event{ID: "withdraw", Params: []Var{"amount"}}, Guard: "balance > 0", Post: "balance' = balance - amount"},
			{Src: "s1", Dst: "s2", Event: Event{ID: "confirm"}, Guard: "always", Post: "confirmed = true"},
		},
	}
}

func createSpecPostFail() *Diagram {
	return &Diagram{
		States: map[StateID]State{
			"s0": {ID: "s0", Name: "prepared", Vars: []Var{"counter"}},
			"s1": {ID: "s1", Name: "executing", Vars: []Var{"executing"}}, 
			"s2": {ID: "s2", Name: "completed", Vars: []Var{"result"}},
		},
		StartEdge: StartEdge{
			Dst:  "s0",
			Post: "counter = 0",
		},
		Edges: []Edge{ 
			{Src: "s0", Dst: "s1", Event: Event{ID: "execute"}, Guard: "always", Post: "executing = true"},
			{Src: "s1", Dst: "s2", Event: Event{ID: "finish"}, Guard: "always", Post: "counter' = counter + 1, result = success"},
		},
	}
}

func createImplPostFail() *Diagram {
	return &Diagram{
		States: map[StateID]State{
			"s0": {ID: "s0", Name: "prepared", Vars: []Var{"counter"}},
			"s1": {ID: "s1", Name: "executing", Vars: []Var{"executing"}},
			"s2": {ID: "s2", Name: "completed", Vars: []Var{"result"}},
		},
		StartEdge: StartEdge{
			Dst:  "s0",
			Post: "counter = 0",
		},
		Edges: []Edge{
			{Src: "s0", Dst: "s1", Event: Event{ID: "execute"}, Guard: "always", Post: "executing = true"},
			{Src: "s1", Dst: "s2", Event: Event{ID: "finish"}, Guard: "always", Post: "result = maybe_success"},
		},
	}
}