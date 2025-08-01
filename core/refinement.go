package core

import (
	"fmt"
	"strings"
)

// ProofObligation represents a proof obligation for refinement verification
type ProofObligation struct {
	ID          string
	Type        ProofObligationType
	Description string
	Premise     string
	Conclusion  string
	Context     RefContext
}

type ProofObligationType string

const (
	GuardWeakening           ProofObligationType = "guard_weakening"
	PostconditionStrength    ProofObligationType = "postcondition_strengthening"
	StateInvariant           ProofObligationType = "state_invariant"
	StableFailuresRefinement ProofObligationType = "stable_failures_refinement"
	TraceInclusion           ProofObligationType = "trace_inclusion"
	RefusalSetInclusion      ProofObligationType = "refusal_set_inclusion"
)

// RefContext provides context for refinement verification
type RefContext struct {
	SpecState StateID
	ImplState StateID
	Event     EventID
	StateVars map[StateID][]Var
}

// RefinementVerifier generates proof obligations for refinement verification
type RefinementVerifier struct {
	specDiagram *Diagram
	implDiagram *Diagram
}

// StableFailuresVerifier extends RefinementVerifier for stable failures refinement
type StableFailuresVerifier struct {
	*RefinementVerifier
	antichain map[string]bool
	processed map[string]bool
}

// NewRefinementVerifier creates a new refinement verifier
func NewRefinementVerifier(spec, impl *Diagram) *RefinementVerifier {
	return &RefinementVerifier{
		specDiagram: spec,
		implDiagram: impl,
	}
}

// NewStableFailuresVerifier creates a new stable failures verifier
func NewStableFailuresVerifier(spec, impl *Diagram) *StableFailuresVerifier {
	return &StableFailuresVerifier{
		RefinementVerifier: NewRefinementVerifier(spec, impl),
		antichain:         make(map[string]bool),
		processed:         make(map[string]bool),
	}
}

// GenerateProofObligations generates all proof obligations for refinement
func (rv *RefinementVerifier) GenerateProofObligations() ([]ProofObligation, error) {
	var obligations []ProofObligation
	
	// Generate obligations for corresponding transitions
	specTransitions := rv.getTransitionMap(rv.specDiagram)
	implTransitions := rv.getTransitionMap(rv.implDiagram)
	
	for key, specEdge := range specTransitions {
		if implEdge, exists := implTransitions[key]; exists {
			// Generate guard weakening obligation
			if specEdge.Guard != True && implEdge.Guard != True {
				guardObligation := ProofObligation{
					ID:          fmt.Sprintf("guard_%s_%s", specEdge.Src, specEdge.Event.ID),
					Type:        GuardWeakening,
					Description: fmt.Sprintf("Guard weakening for transition %s --%s--> %s", specEdge.Src, specEdge.Event.ID, specEdge.Dst),
					Premise:     implEdge.Guard,
					Conclusion:  specEdge.Guard,
					Context: RefContext{
						SpecState: specEdge.Src,
						ImplState: implEdge.Src,
						Event:     specEdge.Event.ID,
						StateVars: rv.getAllStateVars(),
					},
				}
				obligations = append(obligations, guardObligation)
			}
			
			// Generate postcondition strengthening obligation
			if specEdge.Post != True && implEdge.Post != True {
				postObligation := ProofObligation{
					ID:          fmt.Sprintf("post_%s_%s", specEdge.Src, specEdge.Event.ID),
					Type:        PostconditionStrength,
					Description: fmt.Sprintf("Postcondition strengthening for transition %s --%s--> %s", specEdge.Src, specEdge.Event.ID, specEdge.Dst),
					Premise:     specEdge.Post,
					Conclusion:  implEdge.Post,
					Context: RefContext{
						SpecState: specEdge.Src,
						ImplState: implEdge.Src,
						Event:     specEdge.Event.ID,
						StateVars: rv.getAllStateVars(),
					},
				}
				obligations = append(obligations, postObligation)
			}
		}
	}
	
	return obligations, nil
}

// getTransitionMap creates a map of transitions keyed by (src, event)  
func (rv *RefinementVerifier) getTransitionMap(diagram *Diagram) map[string]Edge {
	transitions := make(map[string]Edge)
	
	for _, edge := range diagram.Edges {
		key := fmt.Sprintf("%s_%s", edge.Src, edge.Event.ID)
		transitions[key] = edge
	}
	
	return transitions
}

// getAllStateVars collects all state variables from both diagrams
func (rv *RefinementVerifier) getAllStateVars() map[StateID][]Var {
	stateVars := make(map[StateID][]Var)
	
	for stateID, state := range rv.specDiagram.States {
		stateVars[stateID] = state.Vars
	}
	
	for stateID, state := range rv.implDiagram.States {
		stateVars[stateID] = state.Vars
	}
	
	return stateVars
}

// GenerateStableFailuresProofObligations generates proof obligations for stable failures refinement
func (sfv *StableFailuresVerifier) GenerateStableFailuresProofObligations() ([]ProofObligation, error) {
	var obligations []ProofObligation
	
	// Generate trace inclusion obligations
	traceObligations, err := sfv.generateTraceInclusionObligations()
	if err != nil {
		return nil, fmt.Errorf("failed to generate trace inclusion obligations: %w", err)
	}
	obligations = append(obligations, traceObligations...)
	
	// Generate refusal set inclusion obligations  
	refusalObligations, err := sfv.generateRefusalSetObligations()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refusal set obligations: %w", err)
	}
	obligations = append(obligations, refusalObligations...)
	
	// Generate existing guard/postcondition obligations
	basicObligations, err := sfv.GenerateProofObligations()
	if err != nil {
		return nil, fmt.Errorf("failed to generate basic obligations: %w", err)
	}
	obligations = append(obligations, basicObligations...)
	
	return obligations, nil
}

// generateTraceInclusionObligations generates obligations for trace inclusion
func (sfv *StableFailuresVerifier) generateTraceInclusionObligations() ([]ProofObligation, error) {
	var obligations []ProofObligation
	
	// For each trace in implementation, verify it exists in specification
	implTraces := sfv.extractTraces(sfv.implDiagram)
	specTraces := sfv.extractTraces(sfv.specDiagram)
	
	for _, implTrace := range implTraces {
		found := false
		for _, specTrace := range specTraces {
			if sfv.tracesMatch(implTrace, specTrace) {
				found = true
				break
			}
		}
		
		if !found {
			obligation := ProofObligation{
				ID:          fmt.Sprintf("trace_inclusion_%s", implTrace),
				Type:        TraceInclusion,
				Description: fmt.Sprintf("Trace inclusion verification for trace: %s", implTrace),
				Premise:     fmt.Sprintf("trace %s exists in implementation", implTrace),
				Conclusion:  fmt.Sprintf("trace %s must exist in specification", implTrace),
				Context: RefContext{
					StateVars: sfv.getAllStateVars(),
				},
			}
			obligations = append(obligations, obligation)
		}
	}
	
	return obligations, nil
}

// generateRefusalSetObligations generates obligations for refusal set inclusion
func (sfv *StableFailuresVerifier) generateRefusalSetObligations() ([]ProofObligation, error) {
	var obligations []ProofObligation
	
	// For each stable state in implementation, verify refusals are included in specification
	for stateID, implState := range sfv.implDiagram.States {
		if specState, exists := sfv.specDiagram.States[stateID]; exists {
			if sfv.isStableState(implState) && sfv.isStableState(specState) {
				obligation := ProofObligation{
					ID:          fmt.Sprintf("refusal_inclusion_%s", stateID),
					Type:        RefusalSetInclusion,
					Description: fmt.Sprintf("Refusal set inclusion for stable state %s", stateID),
					Premise:     fmt.Sprintf("refusals of state %s in implementation", stateID),
					Conclusion:  fmt.Sprintf("refusals must be subset of specification refusals for state %s", stateID),
					Context: RefContext{
						SpecState: stateID,
						ImplState: stateID,
						StateVars: sfv.getAllStateVars(),
					},
				}
				obligations = append(obligations, obligation)
			}
		}
	}
	
	return obligations, nil
}

// Helper methods for stable failures verification
func (sfv *StableFailuresVerifier) extractTraces(diagram *Diagram) []string {
	var traces []string
	
	// Simple trace extraction - in practice this would be more sophisticated
	for _, edge := range diagram.Edges {
		trace := fmt.Sprintf("%s->%s", edge.Src, edge.Event.ID)
		traces = append(traces, trace)
	}
	
	return traces
}

func (sfv *StableFailuresVerifier) tracesMatch(trace1, trace2 string) bool {
	return trace1 == trace2
}

func (sfv *StableFailuresVerifier) isStableState(state State) bool {
	// A state is stable if it has no internal (tau) transitions out of it
	// For simplicity, we assume all states are stable unless they have internal transitions
	return true // This would need proper implementation based on transition analysis
}

// FormatProofObligations formats proof obligations for output
func FormatProofObligations(obligations []ProofObligation) string {
	var sb strings.Builder
	
	sb.WriteString("Stable Failures Refinement Verification\n")
	sb.WriteString("=====================================\n\n")
	
	for i, obligation := range obligations {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, obligation.Description))
		sb.WriteString(fmt.Sprintf("   Type: %s\n", obligation.Type))
		sb.WriteString(fmt.Sprintf("   ID: %s\n", obligation.ID))
		sb.WriteString(fmt.Sprintf("   Prove: (%s) ⇒ (%s)\n", obligation.Premise, obligation.Conclusion))
		
		if obligation.Context.SpecState != "" {
			sb.WriteString(fmt.Sprintf("   Context: State %s", obligation.Context.SpecState))
			if obligation.Context.Event != "" {
				sb.WriteString(fmt.Sprintf(", Event %s", obligation.Context.Event))
			}
			sb.WriteString("\n")
		}
		
		if len(obligation.Context.StateVars) > 0 {
			sb.WriteString("   State Variables:\n")
			for stateID, vars := range obligation.Context.StateVars {
				if len(vars) > 0 {
					sb.WriteString(fmt.Sprintf("     %s: %v\n", stateID, vars))
				}
			}
		}
		sb.WriteString("\n")
	}
	
	return sb.String()
}