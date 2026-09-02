package obligationir

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Mangle encodes an arbitrary CSDF name as a run of ASCII letters, digits and
// underscores. CSDF names are not identifiers: an event is any run of
// non-semicolon Unicode characters ("choose(product)"), and even an id may carry
// a hyphen or start with a digit. Emitting them verbatim produced theories that
// do not parse, which fails completeness outright - a diagram that refines itself
// must at least yield a checkable obligation.
//
// The encoding is total and injective, so two distinct names never collide and
// the original is recoverable:
//
//   - an ASCII letter or digit stands for itself;
//   - "_" doubles to "__";
//   - every other rune becomes "_u<hex>_" (lower-case hex of its code point).
//
// The result is NOT an identifier on its own: it may start with a digit, and it
// may spell a keyword of either prover or a name the generator itself declares.
// Every emitted identifier therefore carries a category prefix - EventCtor,
// Side.Ctor, VarName, pred_, guard_/post_/init_ - which is what makes it a legal
// identifier distinct from the generator's own vocabulary. Prefixing every
// category rather than keeping a table of words to avoid is deliberate: a table
// is only as good as its last update, and the categories are finitely many.
//
// Backends must agree on these spellings, so this is their single definition.
// The original name is kept in a comment beside the declaration.
func Mangle(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '_':
			b.WriteString("__")
		case r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
		default:
			b.WriteString("_u")
			b.WriteString(strconv.FormatInt(int64(r), 16))
			b.WriteString("_")
		}
	}
	return b.String()
}

// VarPrefix keeps a state variable out of the generator's own vocabulary. A
// diagram may name a variable "step", "init" or "and", and an unprefixed binder
// of that name shadows the constant being defined or is a keyword outright.
const VarPrefix = "v_"

// VarName is the identifier a state variable is bound by.
func VarName(name string) string { return VarPrefix + Mangle(name) }

// IsMangled reports whether Mangle changed the name, i.e. whether the generated
// identifier needs the original spelling recorded beside it.
func IsMangled(s string) bool { return Mangle(s) != s }

// MangledName pairs a generated identifier with the CSDF name it encodes.
type MangledName struct {
	Identifier string
	Original   string
}

// NameTable collects the names an obligation had to encode, so a backend can
// print the correspondence beside the generated theory. Without it a mangled
// event is unreadable, and the reader cannot tell which diagram element a
// declaration came from.
type NameTable struct {
	m map[string]string
}

func NewNameTable() *NameTable { return &NameTable{m: make(map[string]string)} }

// Add records name if Mangle changed it, and is a no-op otherwise.
func (t *NameTable) Add(name string) {
	if id := Mangle(name); id != name {
		t.m[id] = name
	}
}

// Entries returns the recorded names in identifier order.
func (t *NameTable) Entries() []MangledName {
	res := make([]MangledName, 0, len(t.m))
	for id, orig := range t.m {
		res = append(res, MangledName{Identifier: id, Original: orig})
	}
	slices.SortFunc(res, func(a, b MangledName) int {
		return cmp.Compare(a.Identifier, b.Identifier)
	})
	return res
}

// AddStates records every state id and state variable name of a state space.
func (t *NameTable) AddStates(states map[csdf.StateID]IRState) {
	for id, st := range states {
		t.Add(string(id))
		for _, f := range st.Fields {
			t.Add(f.Name)
		}
	}
}

// AddEvents records every event of a transition list. τ is excluded: it never
// becomes a constructor.
func (t *NameTable) AddEvents(edges []IREdge) {
	for _, e := range edges {
		if e.Event == csdf.Tau {
			continue
		}
		t.Add(string(e.Event))
	}
}

// RefinementNameTable lists every name of a refinement obligation that Mangle
// had to encode.
func RefinementNameTable(ir IRRefinement) []MangledName {
	t := NewNameTable()
	for _, ev := range ir.Alphabet {
		t.Add(string(ev))
	}
	for _, s := range []IRSide{ir.Spec, ir.Impl} {
		t.AddStates(s.States)
	}
	return t.Entries()
}

// LivelockFreeNameTable lists every name of a livelock-freedom obligation that
// Mangle had to encode.
func LivelockFreeNameTable(ir IRLivelockFree) []MangledName {
	t := NewNameTable()
	t.AddStates(ir.States)
	t.AddEvents(ir.Edges)
	return t.Entries()
}
