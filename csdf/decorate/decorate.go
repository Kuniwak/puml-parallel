// Package decorate writes a diagram's edge labels in PlantUML's Creole for
// people: a line each for the event, the condition, the change and the
// variables kept, one conjunct to a line, the variables and event fields
// in the variables' colour, each followed by where it comes from (before
// or after the step, or the event) when its notation tells, as the web
// animator shows the clauses.
package decorate

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Kuniwak/puml-parallel/v6/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v6/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v6/csdf/core"
)

// Words are the words a decorated label is written with.
type Words struct {
	Event, Condition, Change, Kept, Emptied string
	Before, After, Field                    string
	Separator                               string
	// Builtins spell each builtin they have a phrase for, its terms in
	// order as the phrase's operands; a builtin without one is printed.
	Builtins map[clause.Op]string
	// Exists spells an ∃ of its name, its type and its body; And joins the
	// conjuncts of a body.
	Exists, And string
}

// English and Japanese are the words of those languages.
var (
	English = Words{Event: "Event", Condition: "Condition", Change: "Change", Kept: "Kept", Emptied: "Emptied", Before: "before", After: "after", Field: "event", Separator: ", ", Builtins: map[clause.Op]string{
		clause.OpEq:     "%[1]s is %[2]s",
		clause.OpNe:     "%[1]s is not %[2]s",
		clause.OpIn:     "%[2]s has %[1]s",
		clause.OpNotIn:  "%[2]s has no %[1]s",
		clause.OpEmpty:  "%[1]s is empty",
		clause.OpUpdate: "%[1]s is %[2]s with %[3]s set to %[4]s",
		clause.OpFrame:  "%[1]s is %[2]s except at %[3]s",
		clause.OpAt:     "%[1]s is at %[2]s",
	}, Exists: "for some %[1]s of %[2]s, %[3]s", And: " and "}
	Japanese = Words{Event: "イベント", Condition: "条件", Change: "変化", Kept: "維持", Emptied: "空にする", Before: "前状態", After: "次状態", Field: "イベント", Separator: "・", Builtins: map[clause.Op]string{
		clause.OpEq:     "%[1]s は %[2]s",
		clause.OpNe:     "%[1]s は %[2]s でない",
		clause.OpIn:     "%[2]s に %[1]s がある",
		clause.OpNotIn:  "%[2]s に %[1]s がない",
		clause.OpEmpty:  "%[1]s は空",
		clause.OpUpdate: "%[1]s は %[2]s の %[3]s を %[4]s にしたもの",
		clause.OpFrame:  "%[1]s は %[3]s のほか %[2]s と同じ",
		clause.OpAt:     "%[1]s は %[2]s にある",
	}, Exists: "ある %[2]s の %[1]s について %[3]s", And: " かつ "}
)

// WordsOf are the words of the language lang names (en, ja), and whether
// there are any.
func WordsOf(lang string) (Words, bool) {
	w, ok := byLanguage[lang]
	return w, ok
}

// Languages are the names of the languages there are words of, sorted.
func Languages() []string { return slices.Sorted(maps.Keys(byLanguage)) }

var byLanguage = map[string]Words{"en": English, "ja": Japanese}

const (
	varColour    = "#0b57d0"
	headColour   = "#888"
	originColour = "#555"
	indent       = "    "
)

// Decorate is source, the text d was read from, with the label of each
// edge, the start edge and each end edge decorated, every line in its
// place; an error when a clause cannot be spelled in w.
func Decorate(source string, d *core.Diagram, w Words) (string, error) {
	lines := strings.Split(source, "\n")
	relabel := func(line int, label string, err error) error {
		if err != nil {
			return fmt.Errorf("decorate.Decorate: line %d: %w", line, err)
		}
		if line < 1 || line > len(lines) {
			return nil
		}
		if at := strings.Index(lines[line-1], " : "); at >= 0 {
			lines[line-1] = lines[line-1][:at+3] + label
		}
		return nil
	}
	label, err := block(w, nil, d.Init, "", nil, clause.And{}, d.InitPost)
	if err := relabel(d.InitLine, label, err); err != nil {
		return "", err
	}
	for _, e := range d.Edges {
		label, err := block(w, &e.Event, e.Src, e.Dst, d.Vars, e.Guard, e.Post)
		if err := relabel(e.Line, label, err); err != nil {
			return "", err
		}
	}
	for i, line := range lines {
		if src, ok := endOf(line); ok {
			if end := endFrom(d, src); end != nil {
				label, err := block(w, nil, src, "", d.Vars, end.Guard, clause.And{})
				if err := relabel(i+1, label, err); err != nil {
					return "", err
				}
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

var endLine = regexp.MustCompile(`^\s*(\S+)\s*-+>\s*\[\*\]\s*:`)

func endOf(line string) (string, bool) {
	m := endLine.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func endFrom(d *core.Diagram, src string) *core.End {
	for i := range d.Ends {
		if d.Ends[i].Src == src {
			return &d.Ends[i]
		}
	}
	return nil
}

// block is the decorated label of an edge from src to dst on ev (none for
// the start and the end edges), with guard and post.
func block(w Words, ev *core.Event, src, dst string, vars map[string][]string, guard, post clause.And) (string, error) {
	n := names{vars: map[string]bool{}, fields: map[string]bool{}}
	for _, st := range []string{src, dst} {
		for _, v := range vars[st] {
			n.vars[v] = true
		}
	}
	var sb strings.Builder
	if ev != nil {
		for _, f := range ev.Fields {
			n.fields[f.Name] = true
		}
		sb.WriteString(head(w.Event) + "<b>" + escape(ev.Channel))
		for _, f := range ev.Fields {
			sb.WriteString(f.Dir() + coloured(f.Name))
		}
		sb.WriteString("</b>")
	}
	emptied, unchanged, rest := animation.SplitFrame(post)
	avoid := map[string]bool{}
	for nm := range n.vars {
		avoid[nm] = true
	}
	for nm := range n.fields {
		avoid[nm] = true
	}
	conditions, err := spelled(guard, w, avoid)
	if err != nil {
		return "", err
	}
	changes, err := spelled(rest, w, avoid)
	if err != nil {
		return "", err
	}
	section(&sb, w.Condition, conditions, w, n)
	section(&sb, w.Change, changes, w, n)
	listed(&sb, w.Emptied, emptied, w)
	listed(&sb, w.Kept, unchanged, w)
	return sb.String(), nil
}

// names are the state variables and the event fields a label may mention.
type names struct{ vars, fields map[string]bool }

func head(word string) string {
	return fmt.Sprintf("<color:%s>%s:</color> ", headColour, word)
}

func coloured(name string) string {
	return fmt.Sprintf("<color:%s>%s</color>", varColour, escape(name))
}

// spelled are the conjuncts of c, those of nested ∧ in their place, each
// in w's words; a variable an ∃ binds is named apart from avoid. A
// conjunct w has no phrase for, or one that cannot be printed, is an
// error: the words never fall back on the clause syntax.
func spelled(c clause.And, w Words, avoid map[string]bool) ([]string, error) {
	var out []string
	for _, f := range c.Conjuncts {
		if inner, ok := f.(clause.And); ok {
			parts, err := spelled(inner, w, avoid)
			if err != nil {
				return nil, err
			}
			out = append(out, parts...)
			continue
		}
		text, err := conjunctText(f, w, avoid)
		if err != nil {
			return nil, err
		}
		out = append(out, text)
	}
	return out, nil
}

func conjunctText(f clause.Formula, w Words, avoid map[string]bool) (string, error) {
	switch f := f.(type) {
	case clause.Builtin:
		if f.Op == clause.OpFrame && len(f.Args) == 2 {
			// A frame of no key keeps the whole map: m' = m.
			f.Op = clause.OpEq
		}
		phrase, ok := w.Builtins[f.Op]
		if !ok {
			return "", fmt.Errorf("no phrase for @%s", f.Op)
		}
		return phrased(phrase, f, w.Separator)
	case clause.Exists:
		return opened(f, w, avoid)
	case clause.Atom:
		return holed(f)
	}
	return "", fmt.Errorf("no spelling of the conjunct %T", f)
}

// opened is e in w's words, its variable named apart from the names free
// in it and from avoid, and its body in parentheses when it has more than
// one conjunct.
func opened(e clause.Exists, w Words, avoid map[string]bool) (string, error) {
	taken := maps.Clone(avoid)
	for _, v := range clause.FreeVars(e) {
		taken[v.Name] = true
	}
	name := clause.Fresh(taken, e.Hint)
	taken[name] = true
	body, err := clause.Open(e, clause.Free{Name: name})
	if err != nil {
		return "", err
	}
	parts, err := spelled(body, w, taken)
	if err != nil {
		return "", err
	}
	text := strings.Join(parts, w.And)
	if len(parts) > 1 {
		// The body in parentheses, so that a conjunct after the ∃ is not
		// read as a part of it.
		text = "(" + text + ")"
	}
	return fmt.Sprintf(w.Exists, name, e.Type, text), nil
}

// holed is a with each hole written $ and its term, without braces or
// escapes, a space after a hole that text could otherwise continue, as a
// name or as a path.
func holed(a clause.Atom) (string, error) {
	var sb strings.Builder
	afterHole := false
	for _, p := range a.Parts {
		switch p := p.(type) {
		case clause.Text:
			if afterHole && continuesHole(string(p)) {
				sb.WriteString(" ")
			}
			sb.WriteString(string(p))
			afterHole = false
		case clause.Hole:
			s, err := clause.PrintTerm(p.Term)
			if err != nil {
				return "", err
			}
			sb.WriteString("$" + s)
			afterHole = true
		}
	}
	return sb.String(), nil
}

// continuesHole says whether text, after a hole, would read as part of it:
// a name going on, a prime, or a path (.f, [k], (args)).
func continuesHole(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return strings.ContainsRune("_'.[(", r) || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// phrased is b in phrase: its terms are the operands in order, but that the terms of a variadic
// builtin past those it always takes, and the states of @at, are one
// operand, listed with sep.
func phrased(phrase string, b clause.Builtin, sep string) (string, error) {
	printed := make([]string, len(b.Args))
	for i, t := range b.Args {
		s, err := clause.PrintTerm(t)
		if err != nil {
			return "", err
		}
		printed[i] = s
	}
	spec, _ := b.Op.Spec()
	var operands []any
	for _, s := range printed[:min(spec.Terms, len(printed))] {
		operands = append(operands, s)
	}
	if spec.Variadic {
		operands = append(operands, strings.Join(printed[min(spec.Terms, len(printed)):], sep))
	}
	if spec.State {
		operands = append(operands, strings.Join(b.States, sep))
	}
	return fmt.Sprintf(phrase, operands...), nil
}

// section writes a line headed word for the first of parts and an
// indented line for each of the rest; nothing when there are none.
func section(sb *strings.Builder, word string, parts []string, w Words, n names) {
	for i, p := range parts {
		lead := indent
		if i == 0 {
			lead = head(word)
		}
		lineBreak(sb)
		sb.WriteString(lead + marked(p, w, n))
	}
	if len(parts) > 0 && !strings.HasSuffix(sb.String(), `\l`) {
		sb.WriteString(`\l`)
	}
}

// lineBreak ends the line sb is on, unless it is empty or ended.
func lineBreak(sb *strings.Builder) {
	if sb.Len() > 0 && !strings.HasSuffix(sb.String(), `\l`) {
		sb.WriteString(`\l`)
	}
}

func listed(sb *strings.Builder, word string, vars []string, w Words) {
	if len(vars) == 0 {
		return
	}
	lineBreak(sb)
	parts := make([]string, len(vars))
	for i, v := range vars {
		parts[i] = coloured(v)
	}
	sb.WriteString(head(word) + strings.Join(parts, escape(w.Separator)) + `\l`)
}

var name = regexp.MustCompile(`(\$?)([\p{L}\p{N}_]+)(')?`)

// marked is text with each name it mentions coloured and followed by its
// origin, the rest escaped: after the step when primed, the event's when
// the event carries it, before the step when written $x, none otherwise.
func marked(text string, w Words, n names) string {
	var sb strings.Builder
	at := 0
	for _, m := range name.FindAllStringSubmatchIndex(text, -1) {
		dollar := m[3] > m[2]
		nm := text[m[4]:m[5]]
		primed := m[6] >= 0
		if !dollar && !n.vars[nm] && !n.fields[nm] {
			continue
		}
		sb.WriteString(escape(text[at:m[0]]))
		sb.WriteString(coloured(nm))
		if o := origin(dollar, nm, primed, w, n); o != "" {
			sb.WriteString(fmt.Sprintf("<size:9><color:%s>%s</color></size>", originColour, o))
		}
		at = m[1]
	}
	sb.WriteString(escape(text[at:]))
	return sb.String()
}

func origin(dollar bool, nm string, primed bool, w Words, n names) string {
	switch {
	case primed:
		return w.After
	case n.fields[nm]:
		return w.Field
	case dollar:
		return w.Before
	}
	return ""
}

// creole are the marks Creole reads in a label, each escaped by a ~.
var creole = []string{"~", "<", "**", "//", `""`, "--", "__", "^^", "==", "##"}

// escape is text with every Creole mark in it escaped, so that it shows as
// written.
func escape(text string) string {
	var sb strings.Builder
	for i := 0; i < len(text); {
		j := slices.IndexFunc(creole, func(m string) bool { return strings.HasPrefix(text[i:], m) })
		if j < 0 {
			sb.WriteByte(text[i])
			i++
			continue
		}
		sb.WriteString("~" + creole[j])
		i += len(creole[j])
	}
	return sb.String()
}
