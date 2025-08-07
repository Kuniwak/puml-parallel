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
	// Stable Failures Model refinement proof obligations
	StableFailuresRefinement ProofObligationType = "stable_failures_refinement"
	TraceInclusion           ProofObligationType = "trace_inclusion"
	RefusalSetInclusion      ProofObligationType = "refusal_set_inclusion"
	
	// Additional stable failures refinement checks
	AlphabetConsistency      ProofObligationType = "alphabet_consistency"
	InitialStateRefinement   ProofObligationType = "initial_state_refinement"
)

// RefContext provides context for CSP refinement verification
type RefContext struct {
	SpecState    StateID
	ImplState    StateID
	Event        EventID
	Trace        []EventID    // Current trace being analyzed
	RefusalSet   []EventID    // Events refused in current state
	IsStable     bool         // Whether current state is stable (no tau transitions)
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

// GenerateProofObligations generates basic CSP refinement proof obligations
func (rv *RefinementVerifier) GenerateProofObligations() ([]ProofObligation, error) {
	var obligations []ProofObligation

	// Generate initial state refinement obligation
	initObligation := ProofObligation{
		ID:          "initial_state_refinement",
		Type:        InitialStateRefinement,
		Description: "Implementation initial state must refine specification initial state",
		Premise:     fmt.Sprintf("impl_init = %s", rv.implDiagram.StartEdge.Dst),
		Conclusion:  fmt.Sprintf("spec_init = %s", rv.specDiagram.StartEdge.Dst),
		Context: RefContext{
			SpecState: rv.specDiagram.StartEdge.Dst,
			ImplState: rv.implDiagram.StartEdge.Dst,
			Trace:     []EventID{},
			IsStable:  true,
		},
	}
	obligations = append(obligations, initObligation)

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

	// Note: Divergence obligations are NOT generated for Stable Failures Model
	// Divergence is only relevant in Failures-Divergences Model

	// Generate alphabet consistency obligations
	alphabetObligations, err := sfv.generateAlphabetConsistencyObligations()
	if err != nil {
		return nil, fmt.Errorf("failed to generate alphabet consistency obligations: %w", err)
	}
	obligations = append(obligations, alphabetObligations...)

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
					Trace: []EventID{},
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
						Trace:     []EventID{},
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
	
	// Find initial state
	initialState := diagram.StartEdge.Dst
	
	// Use depth-limited search to extract traces up to a reasonable limit
	maxDepth := 10 // Limit trace depth to prevent infinite loops
	sfv.extractTracesWithDepthLimit(diagram, initialState, []string{}, &traces, make(map[StateID]bool), 0, maxDepth)
	
	// Add empty trace (initial state is reachable)
	traces = append(traces, "")
	
	// Remove duplicates
	return sfv.removeDuplicateTraces(traces)
}

// extractTracesWithDepthLimit extracts traces with depth limitation to prevent infinite loops
func (sfv *StableFailuresVerifier) extractTracesWithDepthLimit(diagram *Diagram, currentState StateID, currentTrace []string, traces *[]string, pathVisited map[StateID]bool, depth int, maxDepth int) {
	// Stop if we've reached maximum depth
	if depth > maxDepth {
		return
	}
	
	// Stop if we've already visited this state in current path (cycle detection)
	if pathVisited[currentState] {
		// Add current trace as it represents a cycle-terminated path
		if len(currentTrace) > 0 {
			*traces = append(*traces, strings.Join(currentTrace, ","))
		}
		return
	}
	
	// Mark current state as visited in this path
	pathVisited[currentState] = true
	defer func() { delete(pathVisited, currentState) }() // Backtrack on return
	
	// Add current trace if it's not empty
	if len(currentTrace) > 0 {
		*traces = append(*traces, strings.Join(currentTrace, ","))
	}
	
	// Explore all outgoing edges
	for _, edge := range diagram.Edges {
		if edge.Src == currentState {
			if edge.Event.IsTau() {
				// For tau transitions, don't add to trace but continue exploration
				sfv.extractTracesWithDepthLimit(diagram, edge.Dst, currentTrace, traces, pathVisited, depth, maxDepth)
			} else {
				// For visible events, add to trace and continue
				newTrace := make([]string, len(currentTrace))
				copy(newTrace, currentTrace)
				newTrace = append(newTrace, string(edge.Event.ID))
				sfv.extractTracesWithDepthLimit(diagram, edge.Dst, newTrace, traces, pathVisited, depth+1, maxDepth)
			}
		}
	}
}

// removeDuplicateTraces removes duplicate traces from the slice
func (sfv *StableFailuresVerifier) removeDuplicateTraces(traces []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	
	for _, trace := range traces {
		if !seen[trace] {
			seen[trace] = true
			result = append(result, trace)
		}
	}
	
	return result
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

		if len(obligation.Context.RefusalSet) > 0 {
			sb.WriteString(fmt.Sprintf("   Refusal Set: %v\n", obligation.Context.RefusalSet))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Note: generateDivergenceObligations is NOT used in Stable Failures Model
// Stable Failures Model ignores divergent behaviors and focuses only on stable states

// generateAlphabetConsistencyObligations generates obligations for alphabet consistency
// In CSP: alphabets must be consistent for meaningful refinement
func (sfv *StableFailuresVerifier) generateAlphabetConsistencyObligations() ([]ProofObligation, error) {
	var obligations []ProofObligation

	specAlphabet := sfv.extractAlphabet(sfv.specDiagram)
	implAlphabet := sfv.extractAlphabet(sfv.implDiagram)

	// Check if implementation alphabet is subset of specification alphabet
	for _, implEvent := range implAlphabet {
		found := false
		for _, specEvent := range specAlphabet {
			if implEvent == specEvent {
				found = true
				break
			}
		}

		if !found {
			obligation := ProofObligation{
				ID:          fmt.Sprintf("alphabet_%s", implEvent),
				Type:        AlphabetConsistency,
				Description: fmt.Sprintf("Implementation event %s must be in specification alphabet", implEvent),
				Premise:     fmt.Sprintf("impl_alphabet contains %s", implEvent),
				Conclusion:  fmt.Sprintf("spec_alphabet contains %s", implEvent),
				Context: RefContext{
					Event: implEvent,
				},
			}
			obligations = append(obligations, obligation)
		}
	}

	return obligations, nil
}

// Helper methods for Stable Failures Model

// findStableStates identifies states that are stable (no outgoing tau transitions)
func (sfv *StableFailuresVerifier) findStableStates(diagram *Diagram) []StateID {
	var stableStates []StateID

	for stateID := range diagram.States {
		if sfv.isStableStateInDiagram(diagram.States[stateID], diagram) {
			stableStates = append(stableStates, stateID)
		}
	}

	return stableStates
}


// statesCorrespond checks if implementation and specification states correspond
// This is simplified - in practice requires sophisticated state mapping
func (sfv *StableFailuresVerifier) statesCorrespond(implState, specState StateID) bool {
	// Simplified correspondence - in practice this requires state bisimulation or mapping
	return implState == specState
}

// extractAlphabet extracts the set of all events from a diagram
func (sfv *StableFailuresVerifier) extractAlphabet(diagram *Diagram) []EventID {
	eventSet := make(map[EventID]bool)
	var alphabet []EventID

	for _, edge := range diagram.Edges {
		if edge.Event.ID != "tau" { // Exclude internal tau events from alphabet
			if !eventSet[edge.Event.ID] {
				eventSet[edge.Event.ID] = true
				alphabet = append(alphabet, edge.Event.ID)
			}
		}
	}

	return alphabet
}
