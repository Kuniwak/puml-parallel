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
		antichain:          make(map[string]bool),
		processed:          make(map[string]bool),
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
			traceDisplay := implTrace
			if traceDisplay == "" {
				traceDisplay = "⟨⟩" // Empty trace notation
			} else {
				traceDisplay = fmt.Sprintf("⟨%s⟩", traceDisplay)
			}
			obligation := ProofObligation{
				ID:          fmt.Sprintf("trace_inclusion_%s", strings.ReplaceAll(implTrace, ",", "_")),
				Type:        TraceInclusion,
				Description: fmt.Sprintf("Trace inclusion verification for trace: %s", traceDisplay),
				Premise:     fmt.Sprintf("initial state is reachable AND trace %s exists in implementation", traceDisplay),
				Conclusion:  fmt.Sprintf("trace %s must exist in specification", traceDisplay),
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
			if sfv.isStableStateInDiagram(implState, sfv.implDiagram) && sfv.isStableStateInDiagram(specState, sfv.specDiagram) {
				implRefusalDesc := sfv.generateRefusalSetDescription(stateID, sfv.implDiagram)
				specRefusalDesc := sfv.generateRefusalSetDescription(stateID, sfv.specDiagram)
				
				obligation := ProofObligation{
					ID:          fmt.Sprintf("refusal_inclusion_%s", stateID),
					Type:        RefusalSetInclusion,
					Description: fmt.Sprintf("Refusal set inclusion for stable state %s", stateID),
					Premise:     fmt.Sprintf("state %s is reachable from initial state AND implementation %s", stateID, implRefusalDesc),
					Conclusion:  fmt.Sprintf("implementation refusal set ⊆ specification refusal set, where specification %s", specRefusalDesc),
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
	visited := make(map[string]bool)
	
	// Find initial state
	initialState := diagram.StartEdge.Dst
	
	// Extract all possible traces from initial state
	sfv.extractTracesFromState(diagram, initialState, []string{}, &traces, visited)
	
	// Add empty trace (initial state is reachable)
	traces = append(traces, "")
	
	return traces
}

// extractTracesFromState recursively extracts traces from a given state
func (sfv *StableFailuresVerifier) extractTracesFromState(diagram *Diagram, currentState StateID, currentTrace []string, traces *[]string, visited map[string]bool) {
	// Create a key for the current state and trace to avoid infinite loops
	key := fmt.Sprintf("%s:%s", currentState, strings.Join(currentTrace, ","))
	if visited[key] {
		return
	}
	visited[key] = true
	
	// Add current trace if it's not empty
	if len(currentTrace) > 0 {
		*traces = append(*traces, strings.Join(currentTrace, ","))
	}
	
	// Explore all outgoing edges
	for _, edge := range diagram.Edges {
		if edge.Src == currentState {
			if edge.Event.IsTau() {
				// For tau transitions, don't add to trace but continue exploration
				sfv.extractTracesFromState(diagram, edge.Dst, currentTrace, traces, visited)
			} else {
				// For visible events, add to trace and continue
				newTrace := make([]string, len(currentTrace))
				copy(newTrace, currentTrace)
				newTrace = append(newTrace, string(edge.Event.ID))
				sfv.extractTracesFromState(diagram, edge.Dst, newTrace, traces, visited)
			}
		}
	}
}

func (sfv *StableFailuresVerifier) tracesMatch(trace1, trace2 string) bool {
	return trace1 == trace2
}

func (sfv *StableFailuresVerifier) extractSourceState(trace string) string {
	// For event sequence traces, we need to determine reachability from initial state
	// This is a simplified version - in practice, we'd need full path analysis
	return string(sfv.implDiagram.StartEdge.Dst)
}

func (sfv *StableFailuresVerifier) isStableState(state State) bool {
	// A state is stable if it has no internal (tau) transitions out of it
	return sfv.hasNoTauTransitions(state.ID, sfv.implDiagram)
}

func (sfv *StableFailuresVerifier) isStableStateInDiagram(state State, diagram *Diagram) bool {
	// A state is stable if it has no internal (tau) transitions out of it
	return sfv.hasNoTauTransitions(state.ID, diagram)
}

// hasNoTauTransitions checks if a state has no outgoing tau transitions
func (sfv *StableFailuresVerifier) hasNoTauTransitions(stateID StateID, diagram *Diagram) bool {
	for _, edge := range diagram.Edges {
		if edge.Src == stateID && edge.Event.IsTau() {
			return false
		}
	}
	return true
}

// getAllVisibleEvents collects all visible (non-tau) events from both spec and impl diagrams
func (sfv *StableFailuresVerifier) getAllVisibleEvents() []string {
	eventSet := make(map[string]bool)
	
	// Collect events from specification
	for _, edge := range sfv.specDiagram.Edges {
		if !edge.Event.IsTau() {
			eventSet[string(edge.Event.ID)] = true
		}
	}
	
	// Collect events from implementation
	for _, edge := range sfv.implDiagram.Edges {
		if !edge.Event.IsTau() {
			eventSet[string(edge.Event.ID)] = true
		}
	}
	
	// Convert to sorted slice for consistent output
	var events []string
	for event := range eventSet {
		events = append(events, event)
	}
	
	return events
}

// generateRefusalSetDescription generates a human-readable description for computing refusal sets
func (sfv *StableFailuresVerifier) generateRefusalSetDescription(stateID StateID, diagram *Diagram) string {
	// Find all outgoing transitions from this state
	var outgoingTransitions []Edge
	for _, edge := range diagram.Edges {
		if edge.Src == stateID && !edge.Event.IsTau() {
			outgoingTransitions = append(outgoingTransitions, edge)
		}
	}
	
	if len(outgoingTransitions) == 0 {
		return "refusals of all visible events (no outgoing transitions)"
	}
	
	var conditions []string
	
	// Add conditions for each outgoing transition
	for _, edge := range outgoingTransitions {
		eventStr := string(edge.Event.ID)
		if edge.Guard == "" || edge.Guard == "true" || edge.Guard == True {
			// Always available, so not refused
			continue
		} else {
			// Refused when guard is false
			conditions = append(conditions, fmt.Sprintf("refuse %s when ¬(%s)", eventStr, edge.Guard))
		}
	}
	
	if len(conditions) == 0 {
		return "refusals of events not available from this state"
	}
	
	return strings.Join(conditions, "; ") + "; plus refusals of events not available from this state"
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
