package obligationir

import (
	"strconv"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Side names one diagram inside an obligation theory. Everything derived from a
// source location has to be side-qualified once two diagrams share a theory,
// because state ids and line numbers collide between two input files; the empty
// side is the single-diagram case and keeps the unqualified spelling.
//
// Predicate names (pred_<id>) are deliberately *not* qualified: the id hashes the
// text and argument types, so a predicate occurring in both diagrams dedupes into
// one placeholder that is filled once. Every backend must agree on these
// spellings.
type Side struct {
	Suffix     string // appended to declaration names: "" | "_S" | "_I"
	CtorPrefix string // prepended to state constructors: "" | "S_" | "I_"
}

var (
	SideSingle = Side{}
	SideSpec   = Side{Suffix: "_S", CtorPrefix: "S_"}
	SideImpl   = Side{Suffix: "_I", CtorPrefix: "I_"}
)

// EventCtor is the event datatype constructor standing for a visible event.
// It is not side-qualified: the two diagrams are compared over one alphabet.
func EventCtor(e csdf.Event) string { return "Ev_" + string(e) }

// Ctor is the state datatype constructor standing for the state id.
func (s Side) Ctor(id csdf.StateID) string { return s.CtorPrefix + string(id) }

// Qualify side-qualifies a declaration name shared by both sides (st, step,
// tau_step, reachable, init, ...).
func (s Side) Qualify(name string) string { return name + s.Suffix }

// GuardName is the alias of an edge's guard predicate, named after its source line.
func (s Side) GuardName(line int) string { return s.alias("guard", line) }

// PostName is the alias of an edge's post predicate, named after its source line.
func (s Side) PostName(line int) string { return s.alias("post", line) }

func (s Side) alias(kind string, line int) string {
	return kind + s.Suffix + "_L" + strconv.Itoa(line)
}
