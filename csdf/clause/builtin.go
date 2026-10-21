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

// builtinSpec is what the parser, the printer and the traversal know of a
// builtin: how many terms it takes, whether more may follow, and whether a
// state name follows them.
type builtinSpec struct {
	terms    int
	variadic bool
	state    bool
}

var builtins = map[Op]builtinSpec{
	OpEq:     {terms: 2},
	OpNe:     {terms: 2},
	OpIn:     {terms: 2},
	OpUpdate: {terms: 4},
	OpFrame:  {terms: 2, variadic: true},
	OpAt:     {terms: 1, state: true},
}
