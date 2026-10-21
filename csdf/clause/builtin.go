package clause

// Op is a builtin of the skeleton, named by its tool word without "@".
type Op string

const (
	// OpEq says Args[0] and Args[1] are equal.
	OpEq Op = "eq"
	// OpNe says Args[0] and Args[1] differ.
	OpNe Op = "ne"
	// OpIn says the key Args[0] is in the domain of the map Args[1].
	OpIn Op = "in"
	// OpNotIn says the key Args[0] is not in the domain of the map Args[1]:
	// the instance it names does not exist.
	OpNotIn Op = "notin"
	// OpEmpty says the map Args[0] has no entries.
	OpEmpty Op = "empty"
	// OpUpdate says the map Args[0] is Args[1] with the key Args[2] mapped
	// to Args[3].
	OpUpdate Op = "update"
	// OpFrame says the map Args[0] agrees with Args[1] everywhere but at the
	// keys Args[2:].
	OpFrame Op = "frame"
	// OpAt says the instance Args[0] is at State of its local diagram. A
	// state is a position, not a value, so it is a name and never a term.
	OpAt Op = "at"
)

// Builtin is a fact a tool states without any clause language
// (docs/CLAUSE_DESIGN.md, P7).
type Builtin struct {
	Op    Op
	Args  []Term
	State string // OpAt only
}

func (Builtin) isFormula() {}

// BuiltinSpec is what is known of a builtin's shape: how many terms it
// takes, whether more may follow, and whether a state name follows them.
type BuiltinSpec struct {
	Terms    int
	Variadic bool
	State    bool
}

var builtins = map[Op]BuiltinSpec{
	OpEq:     {Terms: 2},
	OpNe:     {Terms: 2},
	OpIn:     {Terms: 2},
	OpNotIn:  {Terms: 2},
	OpEmpty:  {Terms: 1},
	OpUpdate: {Terms: 4},
	OpFrame:  {Terms: 2, Variadic: true},
	OpAt:     {Terms: 1, State: true},
}

// Spec is the shape of the builtin o; ok is false for a word that names no
// builtin.
func (o Op) Spec() (spec BuiltinSpec, ok bool) {
	spec, ok = builtins[o]
	return spec, ok
}
