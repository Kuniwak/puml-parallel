package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// DiagramJSONVersion is the version of the shape DiagramDoc writes; a
// reader refuses another.
const DiagramJSONVersion = 1

// DiagramDoc is a diagram in JSON, for a program such as csdfbmc: its
// states in order, the labels of those labelled apart from their IDs, and
// the variables of every state (an empty list for one with none); the start
// state, its post and the line of the start edge; the edges; the end edges;
// and the assume notes. Every clause is a list of its conjuncts
// (FormulaDoc).
type DiagramDoc struct {
	Version  int                 `json:"version"`
	Name     string              `json:"name"`
	States   []string            `json:"states"`
	Labels   map[string]string   `json:"labels"`
	Vars     map[string][]string `json:"vars"`
	Init     string              `json:"init"`
	InitPost []FormulaDoc        `json:"init_post"`
	InitLine int                 `json:"init_line"`
	Edges    []EdgeDoc           `json:"edges"`
	Ends     []EndDoc            `json:"ends"`
	Assumes  []AssumptionDoc     `json:"assumes"`
}

// EdgeDoc is an edge: its event as its channel and its fields in order,
// its guard and post, and its line. Label is derived from the channel and
// the fields (Event.Label) for a reader of the JSON; a reader in Go refuses
// one that is not.
type EdgeDoc struct {
	Src     string       `json:"src"`
	Dst     string       `json:"dst"`
	Channel string       `json:"channel"`
	Fields  []FieldDoc   `json:"fields"`
	Label   string       `json:"label"`
	Guard   []FormulaDoc `json:"guard"`
	Post    []FormulaDoc `json:"post"`
	Line    int          `json:"line"`
}

// FieldDoc is a field of an event, its direction as Field.Dir writes it.
type FieldDoc struct {
	Dir  string `json:"dir"`
	Name string `json:"name"`
}

// EndDoc is an end edge.
type EndDoc struct {
	Src   string       `json:"src"`
	Guard []FormulaDoc `json:"guard"`
}

// AssumptionDoc is an assume note.
type AssumptionDoc struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// FormulaDoc is one conjunct of a clause, a tagged union written with
// "kind" "atom", "builtin" or "exists" beside the fields of that kind: exactly
// one of its fields is set.
type FormulaDoc struct {
	Atom    *AtomDoc
	Builtin *BuiltinDoc
	Exists  *ExistsDoc
}

// AtomDoc is an atom: Pieces its text as the atom holds it (clause.Pieces),
// Holes the argument each hole reads and Args the terms the arguments stand
// for (clause.Wire). These are the atom. Text, Primed and Function are a view
// derived from them for a program that hands atoms to a glue, such as
// csdfbmc: the text as the glue receives it and which arguments are values
// after the step (clause.AtomWire), and the name of the environment
// function the atom denotes (clause.FunctionOf). A reader of the JSON in Go
// ignores them.
type AtomDoc struct {
	Pieces   []string  `json:"pieces"`
	Holes    []int     `json:"holes"`
	Args     []TermDoc `json:"args"`
	Text     []string  `json:"text"`
	Primed   []bool    `json:"primed"`
	Function string    `json:"function"`
}

// BuiltinDoc is a builtin: its op, its terms, and the state of @at.
type BuiltinDoc struct {
	Op    string    `json:"op"`
	Args  []TermDoc `json:"args"`
	State string    `json:"state,omitempty"`
}

// ExistsDoc is ∃: the type it ranges over, and its body, where the variable
// it binds is the bound index 0. A hint carries no meaning and is not
// written.
type ExistsDoc struct {
	Type string       `json:"type"`
	Body []FormulaDoc `json:"body"`
}

// TermDoc is a term, a tagged union written with "kind" "free", "bound",
// "index", "field" or "app" beside the fields of that kind: exactly one of
// its fields is set.
type TermDoc struct {
	Free  *FreeDoc
	Bound *BoundDoc
	Index *IndexDoc
	Field *FieldTermDoc
	App   *AppDoc
}

// FreeDoc is a free variable, primed when it is the value after the step.
type FreeDoc struct {
	Name   string `json:"name"`
	Primed bool   `json:"primed,omitempty"`
}

// BoundDoc is a de Bruijn index: 0 is the nearest enclosing ∃.
type BoundDoc struct {
	Index int `json:"index"`
}

// IndexDoc is m[k].
type IndexDoc struct {
	Map TermDoc `json:"map"`
	Key TermDoc `json:"key"`
}

// FieldTermDoc is t.f.
type FieldTermDoc struct {
	Of   TermDoc `json:"of"`
	Name string  `json:"name"`
}

// AppDoc is f(t, …).
type AppDoc struct {
	Fn   string    `json:"fn"`
	Args []TermDoc `json:"args"`
}

// MarshalJSON writes f with its kind.
func (f FormulaDoc) MarshalJSON() ([]byte, error) {
	return marshalTagged("conjunct", []variant{
		{"atom", f.Atom != nil, f.Atom}, {"builtin", f.Builtin != nil, f.Builtin}, {"exists", f.Exists != nil, f.Exists},
	})
}

// UnmarshalJSON reads a conjunct of the kind it names, with the fields of
// that kind only.
func (f *FormulaDoc) UnmarshalJSON(raw []byte) error {
	var a AtomDoc
	var b BuiltinDoc
	var e ExistsDoc
	kind, err := unmarshalTagged(raw, "conjunct", []variant{{"atom", true, &a}, {"builtin", true, &b}, {"exists", true, &e}})
	if err != nil {
		return err
	}
	*f = FormulaDoc{}
	switch kind {
	case "atom":
		f.Atom = &a
	case "builtin":
		f.Builtin = &b
	default:
		f.Exists = &e
	}
	return nil
}

// MarshalJSON writes t with its kind.
func (t TermDoc) MarshalJSON() ([]byte, error) {
	return marshalTagged("term", []variant{
		{"free", t.Free != nil, t.Free}, {"bound", t.Bound != nil, t.Bound}, {"index", t.Index != nil, t.Index},
		{"field", t.Field != nil, t.Field}, {"app", t.App != nil, t.App},
	})
}

// UnmarshalJSON reads a term of the kind it names, with the fields of that
// kind only.
func (t *TermDoc) UnmarshalJSON(raw []byte) error {
	var fr FreeDoc
	var bd BoundDoc
	var ix IndexDoc
	var fd FieldTermDoc
	var ap AppDoc
	kind, err := unmarshalTagged(raw, "term", []variant{{"free", true, &fr}, {"bound", true, &bd}, {"index", true, &ix}, {"field", true, &fd}, {"app", true, &ap}})
	if err != nil {
		return err
	}
	*t = TermDoc{}
	switch kind {
	case "free":
		t.Free = &fr
	case "bound":
		t.Bound = &bd
	case "index":
		t.Index = &ix
	case "field":
		t.Field = &fd
	default:
		t.App = &ap
	}
	return nil
}

// variant is one kind of a tagged union: its name, whether it is the one
// set, and its fields (a pointer to them).
type variant struct {
	kind string
	set  bool
	v    any
}

// marshalTagged writes the one variant of a union of what (a word for
// errors) that is set: its fields, and "kind" its name.
func marshalTagged(what string, variants []variant) ([]byte, error) {
	var chosen *variant
	for i := range variants {
		if variants[i].set {
			if chosen != nil {
				return nil, fmt.Errorf("a %s is exactly one of %s; more than one is set", what, kindNames(variants))
			}
			chosen = &variants[i]
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("a %s is exactly one of %s; not one is set", what, kindNames(variants))
	}
	raw, err := json.Marshal(chosen.v)
	if err != nil {
		return nil, err
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	kind, err := json.Marshal(chosen.kind)
	if err != nil {
		return nil, err
	}
	fields["kind"] = kind
	return json.Marshal(fields)
}

// unmarshalTagged reads raw into the fields of the variant its "kind"
// names, refusing a key that variant has no field for, and returns the kind.
func unmarshalTagged(raw []byte, what string, variants []variant) (string, error) {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return "", err
	}
	k, ok := fields["kind"]
	if !ok {
		return "", fmt.Errorf("%s has no kind", raw)
	}
	var kind string
	if err := json.Unmarshal(k, &kind); err != nil {
		return "", fmt.Errorf("the kind of %s: %w", raw, err)
	}
	delete(fields, "kind")
	for _, v := range variants {
		if v.kind != kind {
			continue
		}
		rest, err := json.Marshal(fields)
		if err != nil {
			return "", err
		}
		if err := strict(rest, v.v); err != nil {
			return "", fmt.Errorf("a %s of kind %s: %w", what, kind, err)
		}
		return kind, nil
	}
	return "", fmt.Errorf("the kind %q is none of %s", kind, kindNames(variants))
}

func kindNames(variants []variant) string {
	names := make([]string, len(variants))
	for i, v := range variants {
		names[i] = v.kind
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// strict decodes raw into v, refusing a key v has no field for.
func strict(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// MarshalDiagram is d as JSON (DiagramDoc). A diagram that is not well
// formed is an error.
func MarshalDiagram(d *Diagram) ([]byte, error) {
	doc, err := diagramDocOf(d)
	if err != nil {
		return nil, fmt.Errorf("core.MarshalDiagram: %w", err)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("core.MarshalDiagram: %w", err)
	}
	return raw, nil
}

// UnmarshalDiagram reads the diagram MarshalDiagram wrote. Another version,
// an unknown key, a conjunct or term of no kind it knows or with a key of
// another kind, a field of no direction, an atom whose pieces and holes do
// not fit, what a document derives (an edge's label, an atom's text,
// primed and function) other than what the rest makes, and a diagram that
// is not well formed are errors.
func UnmarshalDiagram(raw []byte) (*Diagram, error) {
	var doc DiagramDoc
	if err := strict(raw, &doc); err != nil {
		return nil, fmt.Errorf("core.UnmarshalDiagram: %w", err)
	}
	d, err := doc.diagram()
	if err != nil {
		return nil, fmt.Errorf("core.UnmarshalDiagram: %w", err)
	}
	return d, nil
}

// DiagramDocOf is d as a DiagramDoc. A diagram that is not well formed is
// an error.
func DiagramDocOf(d *Diagram) (DiagramDoc, error) {
	doc, err := diagramDocOf(d)
	if err != nil {
		return DiagramDoc{}, fmt.Errorf("core.DiagramDocOf: %w", err)
	}
	return doc, nil
}

func diagramDocOf(d *Diagram) (DiagramDoc, error) {
	if err := d.wellFormed(); err != nil {
		return DiagramDoc{}, err
	}
	doc := DiagramDoc{
		Version: DiagramJSONVersion, Name: d.Name, States: append([]string{}, d.States...),
		Labels: map[string]string{}, Vars: map[string][]string{},
		Init: d.Init, InitLine: d.InitLine,
		Edges: []EdgeDoc{}, Ends: []EndDoc{}, Assumes: []AssumptionDoc{},
	}
	for _, s := range d.States {
		if l, ok := d.Labels[s]; ok {
			doc.Labels[s] = l
		}
		doc.Vars[s] = append([]string{}, d.Vars[s]...)
	}
	var err error
	if doc.InitPost, err = formulaDocsOf(d.InitPost); err != nil {
		return DiagramDoc{}, fmt.Errorf("the start edge: %w", err)
	}
	for _, e := range d.Edges {
		ed := EdgeDoc{Src: e.Src, Dst: e.Dst, Channel: e.Event.Channel, Fields: []FieldDoc{}, Label: e.Event.Label(), Line: e.Line}
		for _, f := range e.Event.Fields {
			ed.Fields = append(ed.Fields, FieldDoc{Dir: f.Dir(), Name: f.Name})
		}
		if ed.Guard, err = formulaDocsOf(e.Guard); err != nil {
			return DiagramDoc{}, fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, ed.Label, err)
		}
		if ed.Post, err = formulaDocsOf(e.Post); err != nil {
			return DiagramDoc{}, fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, ed.Label, err)
		}
		doc.Edges = append(doc.Edges, ed)
	}
	for _, e := range d.Ends {
		guard, err := formulaDocsOf(e.Guard)
		if err != nil {
			return DiagramDoc{}, fmt.Errorf("the end edge from %s: %w", e.Src, err)
		}
		doc.Ends = append(doc.Ends, EndDoc{Src: e.Src, Guard: guard})
	}
	for _, a := range d.Assumes {
		doc.Assumes = append(doc.Assumes, AssumptionDoc{ID: a.ID, Text: a.Text})
	}
	return doc, nil
}

// Diagram is the diagram doc writes. What doc derives (an edge's label, an
// atom's text, primed and function) must be what the rest makes.
func (doc DiagramDoc) Diagram() (*Diagram, error) {
	d, err := doc.diagram()
	if err != nil {
		return nil, fmt.Errorf("core.DiagramDoc.Diagram: %w", err)
	}
	return d, nil
}

func (doc DiagramDoc) diagram() (*Diagram, error) {
	if doc.Version != DiagramJSONVersion {
		return nil, fmt.Errorf("version %d, but this reads version %d", doc.Version, DiagramJSONVersion)
	}
	d := &Diagram{Name: doc.Name, States: append([]string{}, doc.States...), Vars: map[string][]string{},
		Init: doc.Init, InitLine: doc.InitLine}
	if len(doc.Labels) > 0 {
		d.Labels = make(map[string]string, len(doc.Labels))
		for s, l := range doc.Labels {
			d.Labels[s] = l
		}
	}
	for s, vs := range doc.Vars {
		if len(vs) > 0 {
			d.Vars[s] = append([]string{}, vs...)
		}
	}
	var err error
	if d.InitPost, err = formulaOfDocs(doc.InitPost); err != nil {
		return nil, fmt.Errorf("the start edge: %w", err)
	}
	for _, ed := range doc.Edges {
		e := Edge{Src: ed.Src, Dst: ed.Dst, Event: Event{Channel: ed.Channel}, Line: ed.Line}
		for _, fd := range ed.Fields {
			f, err := fieldOfDir(fd.Dir, fd.Name)
			if err != nil {
				return nil, fmt.Errorf("the edge on %s: %w", ed.Channel, err)
			}
			e.Event.Fields = append(e.Event.Fields, f)
		}
		if ed.Label != e.Event.Label() {
			return nil, fmt.Errorf("the edge on %s is labelled %q, but its channel and fields make %q", ed.Channel, ed.Label, e.Event.Label())
		}
		if e.Guard, err = formulaOfDocs(ed.Guard); err != nil {
			return nil, fmt.Errorf("%s --> %s : %s: %w", ed.Src, ed.Dst, e.Event.Label(), err)
		}
		if e.Post, err = formulaOfDocs(ed.Post); err != nil {
			return nil, fmt.Errorf("%s --> %s : %s: %w", ed.Src, ed.Dst, e.Event.Label(), err)
		}
		d.Edges = append(d.Edges, e)
	}
	for _, ed := range doc.Ends {
		guard, err := formulaOfDocs(ed.Guard)
		if err != nil {
			return nil, fmt.Errorf("the end edge from %s: %w", ed.Src, err)
		}
		d.Ends = append(d.Ends, End{Src: ed.Src, Guard: guard})
	}
	for _, a := range doc.Assumes {
		d.Assumes = append(d.Assumes, Assumption{ID: a.ID, Text: a.Text})
	}
	if err := d.wellFormed(); err != nil {
		return nil, err
	}
	return d, nil
}

// FormulaDocOf is f as the list of its conjuncts, normalised
// (clause.Normalize). It is never nil; a formula that is not well formed
// (clause.WellFormed) is an error.
func FormulaDocOf(f clause.And) ([]FormulaDoc, error) {
	docs, err := formulaDocsOf(f)
	if err != nil {
		return nil, fmt.Errorf("core.FormulaDocOf: %w", err)
	}
	return docs, nil
}

func formulaDocsOf(f clause.And) ([]FormulaDoc, error) {
	if err := clause.WellFormed(f); err != nil {
		return nil, err
	}
	return formulaDocs(clause.Normalize(f))
}

func formulaDocs(f clause.And) ([]FormulaDoc, error) {
	out := []FormulaDoc{}
	for _, c := range f.Conjuncts {
		switch c := c.(type) {
		case clause.Atom:
			w, terms, err := clause.Wire(c)
			if err != nil {
				return nil, err
			}
			fn, err := clause.FunctionOf(c)
			if err != nil {
				return nil, err
			}
			out = append(out, FormulaDoc{Atom: &AtomDoc{
				Pieces: clause.Pieces(c), Holes: w.Holes, Args: termDocs(terms),
				Text: w.Text, Primed: w.Primed, Function: fn.Name,
			}})
		case clause.Builtin:
			out = append(out, FormulaDoc{Builtin: &BuiltinDoc{Op: string(c.Op), Args: termDocs(c.Args), State: c.State}})
		case clause.Exists:
			body, err := formulaDocs(c.Body)
			if err != nil {
				return nil, err
			}
			out = append(out, FormulaDoc{Exists: &ExistsDoc{Type: c.Type, Body: body}})
		}
	}
	return out, nil
}

func formulaOfDocs(docs []FormulaDoc) (clause.And, error) {
	f := clause.And{}
	for _, doc := range docs {
		c, err := doc.formula()
		if err != nil {
			return clause.And{}, err
		}
		f.Conjuncts = append(f.Conjuncts, c)
	}
	return f, nil
}

func (doc FormulaDoc) formula() (clause.Formula, error) {
	switch {
	case doc.Atom != nil:
		args, err := termsOfDocs(doc.Atom.Args)
		if err != nil {
			return nil, err
		}
		a, err := clause.AtomOf(doc.Atom.Pieces, doc.Atom.Holes, args)
		if err != nil {
			return nil, err
		}
		w, terms, err := clause.Wire(a)
		if err != nil {
			return nil, err
		}
		fn, err := clause.FunctionOf(a)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(w.Holes, doc.Atom.Holes) || !reflect.DeepEqual(terms, args) {
			return nil, fmt.Errorf("the atom %s: its holes and arguments are not those it reads", fn.Name)
		}
		if !slices.Equal(w.Text, doc.Atom.Text) || !slices.Equal(w.Primed, doc.Atom.Primed) || fn.Name != doc.Atom.Function {
			return nil, fmt.Errorf("the atom %s: its text, primed or function is not what its pieces, holes and arguments make (%q, %v, %q)",
				fn.Name, doc.Atom.Text, doc.Atom.Primed, doc.Atom.Function)
		}
		return a, nil
	case doc.Builtin != nil:
		args, err := termsOfDocs(doc.Builtin.Args)
		if err != nil {
			return nil, err
		}
		return clause.Builtin{Op: clause.Op(doc.Builtin.Op), Args: args, State: doc.Builtin.State}, nil
	default:
		body, err := formulaOfDocs(doc.Exists.Body)
		if err != nil {
			return nil, err
		}
		return clause.Exists{Type: doc.Exists.Type, Body: body}, nil
	}
}

func termDocs(ts []clause.Term) []TermDoc {
	out := make([]TermDoc, len(ts))
	for i, t := range ts {
		out[i] = termDoc(t)
	}
	return out
}

func termDoc(t clause.Term) TermDoc {
	switch t := t.(type) {
	case clause.Free:
		return TermDoc{Free: &FreeDoc{Name: t.Name, Primed: t.Primed}}
	case clause.Bound:
		return TermDoc{Bound: &BoundDoc{Index: t.Index}}
	case clause.Index:
		return TermDoc{Index: &IndexDoc{Map: termDoc(t.Map), Key: termDoc(t.Key)}}
	case clause.Field:
		return TermDoc{Field: &FieldTermDoc{Of: termDoc(t.Of), Name: t.Name}}
	case clause.App:
		return TermDoc{App: &AppDoc{Fn: t.Fn, Args: termDocs(t.Args)}}
	}
	panic(fmt.Sprintf("core.termDoc: unknown term %#v", t))
}

func termsOfDocs(docs []TermDoc) ([]clause.Term, error) {
	out := make([]clause.Term, 0, len(docs))
	for _, doc := range docs {
		t, err := doc.term()
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (doc TermDoc) term() (clause.Term, error) {
	switch {
	case doc.Free != nil:
		return clause.Free{Name: doc.Free.Name, Primed: doc.Free.Primed}, nil
	case doc.Bound != nil:
		return clause.Bound{Index: doc.Bound.Index}, nil
	case doc.Index != nil:
		m, err := doc.Index.Map.term()
		if err != nil {
			return nil, err
		}
		k, err := doc.Index.Key.term()
		if err != nil {
			return nil, err
		}
		return clause.Index{Map: m, Key: k}, nil
	case doc.Field != nil:
		o, err := doc.Field.Of.term()
		if err != nil {
			return nil, err
		}
		return clause.Field{Of: o, Name: doc.Field.Name}, nil
	case doc.App != nil:
		args, err := termsOfDocs(doc.App.Args)
		if err != nil {
			return nil, err
		}
		return clause.App{Fn: doc.App.Fn, Args: args}, nil
	}
	return nil, fmt.Errorf("a term of no kind")
}
