package clause

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

// Lit is a literal (docs/CLAUSE_DESIGN.md, "Literals"): a JSON value
// without null, held as its canonical text, so that two literals are one
// value exactly when their texts are equal. An integer is any integer,
// written in decimal; a float is a finite IEEE 754 binary64, written
// shortest with a point or an exponent, so that 1 and 1.0 are two values,
// and never negative zero, which is read as zero, so that two floats are
// one value exactly when they are equal numbers; a string is written with its characters as they
// are, but for those a clause or its edge reads as syntax, which are
// escaped; an array keeps its order; an object has its keys sorted.
type Lit struct{ Text string }

func (Lit) isTerm() {}

// ParseLiteral reads text as one literal.
func ParseLiteral(text string) (Lit, error) {
	src := []rune(text)
	canon, pos, err := readLiteral(src, 0)
	if err != nil {
		return Lit{}, fmt.Errorf("clause.ParseLiteral: %q: %w", text, err)
	}
	if pos < len(src) {
		return Lit{}, fmt.Errorf("clause.ParseLiteral: %q: unexpected %q at %d", text, string(src[pos:]), pos)
	}
	return Lit{Text: canon}, nil
}

// startsLiteral reports whether r starts a literal other than true and
// false, which start as names do.
func startsLiteral(r rune) bool {
	return r == '-' || r == '"' || r == '[' || r == '{' || ('0' <= r && r <= '9')
}

// The words that are literals, never names.
const (
	trueWord  = "true"
	falseWord = "false"
	nullWord  = "null"
)

// IsReserved reports whether a name is a word a term reads as a literal,
// and so is no name of a variable, a field or a function.
func IsReserved(name string) bool { return name == trueWord || name == falseWord || name == nullWord }

// readLiteral reads the literal at pos of src and returns its canonical
// text and the position after it.
func readLiteral(src []rune, pos int) (string, int, error) {
	l := &lexer{src: src, pos: pos}
	s, err := l.value()
	return s, l.pos, err
}

type lexer struct {
	src []rune
	pos int
}

func (l *lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) space() {
	for strings.ContainsRune(" \t\n\r", l.peek()) && l.peek() != 0 {
		l.pos++
	}
}

func (l *lexer) word(w string) bool {
	rs := []rune(w)
	if l.pos+len(rs) > len(l.src) || string(l.src[l.pos:l.pos+len(rs)]) != w {
		return false
	}
	if next := l.pos + len(rs); next < len(l.src) && ident.IsRune(l.src[next]) {
		return false
	}
	l.pos += len(rs)
	return true
}

func (l *lexer) value() (string, error) {
	switch r := l.peek(); {
	case r == '"':
		s, err := l.str()
		if err != nil {
			return "", err
		}
		return quote(s), nil
	case r == '[':
		return l.array()
	case r == '{':
		return l.object()
	case r == '-' || ('0' <= r && r <= '9'):
		return l.number()
	case l.word(trueWord):
		return trueWord, nil
	case l.word(falseWord):
		return falseWord, nil
	case l.word(nullWord):
		return "", fmt.Errorf("null is no value at %d", l.pos-len(nullWord))
	}
	return "", fmt.Errorf("expected a literal at %d", l.pos)
}

func (l *lexer) digits() int {
	n := 0
	for '0' <= l.peek() && l.peek() <= '9' {
		l.pos++
		n++
	}
	return n
}

func (l *lexer) number() (string, error) {
	start := l.pos
	if l.peek() == '-' {
		l.pos++
	}
	first := l.pos
	if l.digits() == 0 {
		return "", fmt.Errorf("expected a digit at %d", l.pos)
	}
	if l.src[first] == '0' && l.pos-first > 1 {
		return "", fmt.Errorf("a number does not start with 0 at %d", start)
	}
	float := false
	if l.peek() == '.' {
		l.pos++
		float = true
		if l.digits() == 0 {
			return "", fmt.Errorf("expected a digit at %d", l.pos)
		}
	}
	if l.peek() == 'e' || l.peek() == 'E' {
		l.pos++
		float = true
		if l.peek() == '+' || l.peek() == '-' {
			l.pos++
		}
		if l.digits() == 0 {
			return "", fmt.Errorf("expected a digit at %d", l.pos)
		}
	}
	if ident.IsRune(l.peek()) {
		end := l.pos
		for end < len(l.src) && ident.IsRune(l.src[end]) {
			end++
		}
		return "", fmt.Errorf("a name does not start with a digit, as %s at %d does", string(l.src[start:end]), start)
	}
	return canonicalNumber(string(l.src[start:l.pos]), float)
}

// canonicalNumber is the canonical text of the number text, which is a
// float when float says so, and an integer otherwise.
func canonicalNumber(text string, float bool) (string, error) {
	if !float {
		n, ok := new(big.Int).SetString(text, 10)
		if !ok {
			return "", fmt.Errorf("%s is no integer", text)
		}
		return n.String(), nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if errors.Is(err, strconv.ErrRange) && math.IsInf(f, 0) {
		return "", fmt.Errorf("%s is beyond the floats, which are finite binary64", text)
	}
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return "", fmt.Errorf("%s is no float: %w", text, err)
	}
	return CanonicalFloat(f), nil
}

// CanonicalFloat is the canonical text of the finite float f: shortest,
// with a point or an exponent, and negative zero written as zero, from
// which nothing in the language can tell it.
func CanonicalFloat(f float64) string {
	if f == 0 {
		f = 0
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}

// UnsignedZeros is v, a value as encoding/json decodes it (with or
// without UseNumber), with every negative zero in it written as zero: a
// float64 -0.0 as 0.0, and a number whose text is a zero with a sign as the
// canonical text of zero, "0" or "0.0". Nothing in the language can tell
// negative zero from zero, so a value read from outside passes through it
// before it is kept, shown or sent on; any other number keeps its text.
func UnsignedZeros(v any) any {
	switch v := v.(type) {
	case float64:
		if v == 0 {
			return 0.0
		}
	case json.Number:
		text := string(v)
		if strings.HasPrefix(text, "-") {
			if c, err := canonicalNumber(text, IsFloatText(text)); err == nil && (c == "0" || c == "0.0") {
				return json.Number(c)
			}
		}
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = UnsignedZeros(x)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, x := range v {
			out[k] = UnsignedZeros(x)
		}
		return out
	}
	return v
}

// FloatParts is the canonical form of the finite float f: f = (-1)^neg · sig
// · 2^exp with sig odd, or neg false and sig and exp 0 for either zero.
// Every such float has one, so two floats are one exactly when their forms
// are.
func FloatParts(f float64) (neg bool, sig uint64, exp int) {
	if f == 0 {
		return false, 0, 0
	}
	bits := math.Float64bits(f)
	neg = bits>>63 == 1
	e := int(bits>>52) & 0x7ff
	sig = bits & (1<<52 - 1)
	if e == 0 {
		exp = -1074
	} else {
		sig |= 1 << 52
		exp = e - 1075
	}
	for sig%2 == 0 {
		sig /= 2
		exp++
	}
	return neg, sig, exp
}

// IsFloatText reports whether the number text is a float: it has a point
// or an exponent; any other number is an integer.
func IsFloatText(text string) bool { return strings.ContainsAny(text, ".eE") }

func (l *lexer) str() (string, error) {
	start := l.pos
	l.pos++ // "
	var b strings.Builder
	for {
		if l.pos >= len(l.src) {
			return "", fmt.Errorf("an unterminated string at %d", start)
		}
		r := l.src[l.pos]
		switch {
		case r == '"':
			l.pos++
			return b.String(), nil
		case r < 0x20:
			return "", fmt.Errorf("a control character in the string at %d; escape it", l.pos)
		case r == '\\':
			e, err := l.escape()
			if err != nil {
				return "", err
			}
			b.WriteRune(e)
		default:
			b.WriteRune(r)
			l.pos++
		}
	}
}

var escapes = map[rune]rune{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}

// escape reads the escape at the backslash at pos.
func (l *lexer) escape() (rune, error) {
	at := l.pos
	l.pos++
	c := l.peek()
	if e, ok := escapes[c]; ok {
		l.pos++
		return e, nil
	}
	if c != 'u' {
		return 0, fmt.Errorf(`an unknown escape \%c at %d`, c, at)
	}
	l.pos++
	r, err := l.hex4()
	if err != nil {
		return 0, err
	}
	if !utf16.IsSurrogate(r) {
		return r, nil
	}
	if r < 0xdc00 && l.peek() == '\\' && l.pos+1 < len(l.src) && l.src[l.pos+1] == 'u' {
		save := l.pos
		l.pos += 2
		lo, err := l.hex4()
		if err == nil {
			if c := utf16.DecodeRune(r, lo); c != utf8.RuneError {
				return c, nil
			}
		}
		l.pos = save
	}
	return 0, fmt.Errorf("a lone surrogate at %d", at)
}

func (l *lexer) hex4() (rune, error) {
	if l.pos+4 > len(l.src) {
		return 0, fmt.Errorf("expected four hexadecimal digits at %d", l.pos)
	}
	n, err := strconv.ParseUint(string(l.src[l.pos:l.pos+4]), 16, 32)
	if err != nil {
		return 0, fmt.Errorf("expected four hexadecimal digits at %d", l.pos)
	}
	l.pos += 4
	return rune(n), nil
}

func (l *lexer) array() (string, error) {
	l.pos++ // [
	var items []string
	l.space()
	if l.peek() == ']' {
		l.pos++
		return "[]", nil
	}
	for {
		l.space()
		v, err := l.value()
		if err != nil {
			return "", err
		}
		items = append(items, v)
		l.space()
		switch l.peek() {
		case ',':
			l.pos++
		case ']':
			l.pos++
			return "[" + strings.Join(items, ",") + "]", nil
		default:
			return "", fmt.Errorf(`expected "," or "]" at %d`, l.pos)
		}
	}
}

func (l *lexer) object() (string, error) {
	l.pos++ // {
	fields := map[string]string{}
	l.space()
	if l.peek() == '}' {
		l.pos++
		return "{}", nil
	}
	for {
		l.space()
		if l.peek() != '"' {
			return "", fmt.Errorf("expected a string, the key, at %d", l.pos)
		}
		at := l.pos
		k, err := l.str()
		if err != nil {
			return "", err
		}
		if _, dup := fields[k]; dup {
			return "", fmt.Errorf("the key %q twice in an object, the second at %d", k, at)
		}
		l.space()
		if l.peek() != ':' {
			return "", fmt.Errorf(`expected ":" at %d`, l.pos)
		}
		l.pos++
		l.space()
		v, err := l.value()
		if err != nil {
			return "", err
		}
		fields[k] = v
		l.space()
		switch l.peek() {
		case ',':
			l.pos++
		case '}':
			l.pos++
			return object(fields), nil
		default:
			return "", fmt.Errorf(`expected "," or "}" at %d`, l.pos)
		}
	}
}

// object is the canonical text of an object whose fields have the
// canonical texts of fields, keys sorted.
func object(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = quote(k) + ":" + fields[k]
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// quote is the canonical text of the string s: escaped where JSON must
// escape, and where a clause or its edge would read the character as
// syntax, ; ending a clause and / beginning a comment.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '/':
			b.WriteString(`\/`)
		case ';':
			b.WriteString(`\u003b`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// CanonicalValue is the canonical text of v, a value as encoding/json
// decodes it with UseNumber (json.Number, string, bool, []any and
// map[string]any), or a float64 or an int: the text of the literal that
// is v. null, a float that is not finite and a string that is not UTF-8
// are no values.
func CanonicalValue(v any) (string, error) {
	switch v := v.(type) {
	case json.Number:
		lit, err := ParseLiteral(string(v))
		if err != nil {
			return "", err
		}
		return lit.Text, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", fmt.Errorf("clause.CanonicalValue: %v is no finite float", v)
		}
		return CanonicalFloat(v), nil
	case int:
		return strconv.Itoa(v), nil
	case string:
		if !utf8.ValidString(v) {
			return "", fmt.Errorf("clause.CanonicalValue: %q is not UTF-8", v)
		}
		return quote(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case []any:
		items := make([]string, len(v))
		for i, x := range v {
			s, err := CanonicalValue(x)
			if err != nil {
				return "", err
			}
			items[i] = s
		}
		return "[" + strings.Join(items, ",") + "]", nil
	case map[string]any:
		fields := make(map[string]string, len(v))
		for k, x := range v {
			if !utf8.ValidString(k) {
				return "", fmt.Errorf("clause.CanonicalValue: the key %q is not UTF-8", k)
			}
			s, err := CanonicalValue(x)
			if err != nil {
				return "", err
			}
			fields[k] = s
		}
		return object(fields), nil
	case nil:
		return "", fmt.Errorf("clause.CanonicalValue: null is no value")
	}
	return "", fmt.Errorf("clause.CanonicalValue: %#v is no JSON value", v)
}

// Decode is the value l is, as encoding/json decodes its text with
// UseNumber.
func (l Lit) Decode() (any, error) {
	dec := json.NewDecoder(strings.NewReader(l.Text))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("clause.Lit.Decode: %s: %w", l.Text, err)
	}
	return v, nil
}
