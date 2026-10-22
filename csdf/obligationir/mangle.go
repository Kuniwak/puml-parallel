package obligationir

import (
	"strconv"
	"strings"
	"unicode"
)

// Mangle encodes an arbitrary CSDF name as a run of ASCII letters, digits and
// underscores. CSDF names are not identifiers: an event is any run of
// non-semicolon Unicode characters ("choose?product"), and even an id may carry
// a hyphen or start with a digit. Emitting them verbatim produced theories that
// do not parse, which fails completeness outright - a diagram that refines itself
// must at least yield a checkable obligation.
//
// The encoding is total and injective, so two distinct names never collide and
// the original is recoverable:
//
//   - an ASCII letter or digit stands for itself;
//   - "_" doubles to "__", which keeps composed state ids readable
//     (ComposeStateIDs joins with "_", so underscores are common);
//   - every other rune becomes "_u<hex>u", the lower-case hex of its code point
//     between a "_u" introducer and a "u" terminator. "u" is not a hex digit, so
//     the terminator is unambiguous, and the character after a "_" says which of
//     the two forms it is.
//
// A "_" that ends the name takes the escaped form instead of doubling, because
// Isabelle rejects a name ending in "_" ("Bad name"). Decoding is unaffected -
// both forms decode to "_" - so the encoding stays injective, and no encoded name
// can end in "_".
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
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range runes {
		switch {
		case r == '_' && i < len(runes)-1:
			b.WriteString("__")
		case r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
		default:
			b.WriteString("_u")
			b.WriteString(strconv.FormatInt(int64(r), 16))
			b.WriteString("u")
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
