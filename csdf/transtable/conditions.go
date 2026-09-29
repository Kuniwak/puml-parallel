package transtable

// Conditions says how much of what holds along a way the condition of an
// outcome keeps.
type Conditions int

const (
	// ConditionsFull keeps every guard and postcondition along the way.
	ConditionsFull Conditions = iota
)
