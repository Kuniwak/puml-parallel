package obligationir

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Mangle encodes an arbitrary CSDF name as an identifier both Lean 4 and
// Isabelle accept. CSDF names are not identifiers: an event is any run of
// non-semicolon Unicode characters ("choose(product)"), and even an id may carry
// a hyphen, start with a digit, or spell a keyword of either prover. Emitting
// them verbatim produced theories that do not parse, which fails completeness
// outright - a diagram that refines itself must at least yield a checkable
// obligation.
//
// The encoding is total and injective, so two distinct names never collide and
// the original is recoverable:
//
//   - an ASCII letter or digit stands for itself;
//   - "_" doubles to "__";
//   - every other rune becomes "_u<hex>_" (lower-case hex of its code point);
//   - a result that would not start with an ASCII letter gets a "u_" prefix,
//     which no other input can produce because a leading "u" is never escaped;
//   - a result that spells a reserved word of either prover gets a "_" suffix,
//     which no other input can produce because a trailing "_" is always doubled.
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

	res := b.String()
	if res == "" || !isASCIILetter(rune(res[0])) {
		res = "u_" + res
	}
	if _, reserved := reservedWords[res]; reserved {
		res += "_"
	}
	return res
}

// IsMangled reports whether Mangle changed the name, i.e. whether the generated
// identifier needs the original spelling recorded beside it.
func IsMangled(s string) bool { return Mangle(s) != s }

func isASCIILetter(r rune) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

// reservedWords is the union of the Lean 4 and Isabelle/Isar words that cannot be
// an identifier. It is deliberately generous: a false positive only appends an
// underscore, whereas a miss produces a theory that does not parse.
var reservedWords = func() map[string]struct{} {
	words := strings.Fields(`
		abbrev at attribute axiom axiomatization by calc case class corollary
		deriving do else end example exists extends for forall from fun have
		if import in inductive instance is lemma let macro match mutual
		namespace noncomputable notation obtain of opaque open partial
		primrec private proof protected qed section set_option show sorry
		structure syntax theorem then this unsafe universe using variable where
		with

		apply assumes begin consts context datatype definition done fixes
		imports locale nonterminal notes obtains oops shows theory ALL EX SOME
		THE

		Prop Sort Type
	`)
	res := make(map[string]struct{}, len(words))
	for _, w := range words {
		res[w] = struct{}{}
	}
	return res
}()

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
