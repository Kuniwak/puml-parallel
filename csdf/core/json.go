package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

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
// the event's label (Event.Label), its guard and post, and its line.
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

// FieldDoc is a field of an event: Dir "?" for one received, "!" for one
// sent.
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

// FormulaDoc is one conjunct of a clause, exactly one of three:
//
//   - an atom: Atom in the shape a glue receives it (clause.AtomWire),
//     Pieces its text as the atom holds it (Atom.Text without the space the
//     wire puts after a hole), Function the name of the environment function
//     it denotes (clause.FunctionOf), and Args the terms its arguments stand
//     for;
//   - a builtin: Builtin its op, Args its terms, State the state of @at;
//   - ∃: Exists the type it ranges over, Body its conjuncts, where the
//     variable it binds is the bound index 0. A hint carries no meaning and
//     is not written.
type FormulaDoc struct {
	Atom     *clause.AtomWire `json:"atom,omitempty"`
	Pieces   []string         `json:"pieces,omitempty"`
	Function string           `json:"function,omitempty"`
	Builtin  string           `json:"builtin,omitempty"`
	State    string           `json:"state,omitempty"`
	Args     []TermDoc        `json:"args,omitempty"`
	Exists   string           `json:"exists,omitempty"`
	Body     []FormulaDoc     `json:"body,omitempty"`
}

// TermDoc is a term, exactly one of: a free variable (Free, primed when
// Primed), a bound index (Bound), an entry of a map (Index), a field
// (Field), or an environment function applied (App).
type TermDoc struct {
	Free   string    `json:"free,omitempty"`
	Primed bool      `json:"primed,omitempty"`
	Bound  *int      `json:"bound,omitempty"`
	Index  *IndexDoc `json:"index,omitempty"`
	Field  *FieldOf  `json:"field,omitempty"`
	App    *AppDoc   `json:"app,omitempty"`
}

// IndexDoc is m[k].
type IndexDoc struct {
	Map TermDoc `json:"map"`
	Key TermDoc `json:"key"`
}

// FieldOf is t.f.
type FieldOf struct {
	Of   TermDoc `json:"of"`
	Name string  `json:"name"`
}

// AppDoc is f(t, …).
type AppDoc struct {
	Fn   string    `json:"fn"`
	Args []TermDoc `json:"args"`
}

// MarshalDiagram is d as JSON (DiagramDoc). A diagram that is not well
// formed is an error.
func MarshalDiagram(d *Diagram) ([]byte, error) {
	doc, err := DiagramDocOf(d)
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
// an unknown key, a conjunct or term that is not exactly one of its kinds,
// an atom whose wire is not the one its pieces and arguments make, and a
// diagram that is not well formed are errors.
func UnmarshalDiagram(raw []byte) (*Diagram, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc DiagramDoc
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("core.UnmarshalDiagram: %w", err)
	}
	d, err := doc.Diagram()
	if err != nil {
		return nil, fmt.Errorf("core.UnmarshalDiagram: %w", err)
	}
	return d, nil
}

// DiagramDocOf is d as a DiagramDoc. A diagram that is not well formed is
// an error.
func DiagramDocOf(d *Diagram) (DiagramDoc, error) {
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
	if doc.InitPost, err = FormulaDocOf(d.InitPost); err != nil {
		return DiagramDoc{}, fmt.Errorf("the start edge: %w", err)
	}
	for _, e := range d.Edges {
		ed := EdgeDoc{Src: e.Src, Dst: e.Dst, Channel: e.Event.Channel, Fields: []FieldDoc{}, Label: e.Event.Label(), Line: e.Line}
		for _, f := range e.Event.Fields {
			dir := "?"
			if f.Send {
				dir = "!"
			}
			ed.Fields = append(ed.Fields, FieldDoc{Dir: dir, Name: f.Name})
		}
		if ed.Guard, err = FormulaDocOf(e.Guard); err != nil {
			return DiagramDoc{}, fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, ed.Label, err)
		}
		if ed.Post, err = FormulaDocOf(e.Post); err != nil {
			return DiagramDoc{}, fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, ed.Label, err)
		}
		doc.Edges = append(doc.Edges, ed)
	}
	for _, e := range d.Ends {
		guard, err := FormulaDocOf(e.Guard)
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

// Diagram is the diagram doc writes.
func (doc DiagramDoc) Diagram() (*Diagram, error) {
	if doc.Version != DiagramJSONVersion {
		return nil, fmt.Errorf("version %d, but this reads version %d", doc.Version, DiagramJSONVersion)
	}
	d := &Diagram{Name: doc.Name, States: append([]string{}, doc.States...), Vars: map[string][]string{},
		Init: doc.Init, InitLine: doc.InitLine}
	for s, l := range doc.Labels {
		if d.Labels == nil {
			d.Labels = map[string]string{}
		}
		d.Labels[s] = l
	}
	for s, vs := range doc.Vars {
		if len(vs) > 0 {
			d.Vars[s] = append([]string{}, vs...)
		}
	}
	var err error
	if d.InitPost, err = formulaOfDoc(doc.InitPost); err != nil {
		return nil, fmt.Errorf("the start edge: %w", err)
	}
	for _, ed := range doc.Edges {
		e := Edge{Src: ed.Src, Dst: ed.Dst, Event: Event{Channel: ed.Channel}, Line: ed.Line}
		for _, f := range ed.Fields {
			switch f.Dir {
			case "?", "!":
				e.Event.Fields = append(e.Event.Fields, Field{Send: f.Dir == "!", Name: f.Name})
			default:
				return nil, fmt.Errorf("the edge on %s: the field %s has the direction %q, neither ? nor !", ed.Channel, f.Name, f.Dir)
			}
		}
		if e.Event.Label() != ed.Label {
			return nil, fmt.Errorf("the edge on %s is labelled %q, but its channel and fields make %q", ed.Channel, ed.Label, e.Event.Label())
		}
		if e.Guard, err = formulaOfDoc(ed.Guard); err != nil {
			return nil, fmt.Errorf("%s --> %s : %s: %w", ed.Src, ed.Dst, ed.Label, err)
		}
		if e.Post, err = formulaOfDoc(ed.Post); err != nil {
			return nil, fmt.Errorf("%s --> %s : %s: %w", ed.Src, ed.Dst, ed.Label, err)
		}
		d.Edges = append(d.Edges, e)
	}
	for _, ed := range doc.Ends {
		guard, err := formulaOfDoc(ed.Guard)
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
	if err := clause.WellFormed(f); err != nil {
		return nil, fmt.Errorf("core.FormulaDocOf: %w", err)
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
			pieces := []string{""}
			for _, p := range c.Parts {
				switch p := p.(type) {
				case clause.Text:
					pieces[len(pieces)-1] += string(p)
				case clause.Hole:
					pieces = append(pieces, "")
				}
			}
			out = append(out, FormulaDoc{Atom: &w, Pieces: pieces, Function: fn.Name, Args: termDocs(terms)})
		case clause.Builtin:
			out = append(out, FormulaDoc{Builtin: string(c.Op), Args: termDocs(c.Args), State: c.State})
		case clause.Exists:
			body, err := formulaDocs(c.Body)
			if err != nil {
				return nil, err
			}
			out = append(out, FormulaDoc{Exists: c.Type, Body: body})
		}
	}
	return out, nil
}

func formulaOfDoc(docs []FormulaDoc) (clause.And, error) {
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
	kinds := 0
	for _, is := range []bool{doc.Atom != nil, doc.Builtin != "", doc.Exists != ""} {
		if is {
			kinds++
		}
	}
	if kinds != 1 {
		return nil, fmt.Errorf("a conjunct is exactly one of an atom, a builtin and ∃, not %+v", doc)
	}
	args, err := termsOfDocs(doc.Args)
	if err != nil {
		return nil, err
	}
	switch {
	case doc.Atom != nil:
		if doc.Builtin != "" || doc.State != "" || doc.Exists != "" || doc.Body != nil {
			return nil, fmt.Errorf("an atom carries no builtin, state or body: %+v", doc)
		}
		if len(doc.Pieces) != len(doc.Atom.Holes)+1 {
			return nil, fmt.Errorf("an atom has one piece more than holes, not %d for %d", len(doc.Pieces), len(doc.Atom.Holes))
		}
		parts := []clause.Part{clause.Text(doc.Pieces[0])}
		for i, h := range doc.Atom.Holes {
			if h < 0 || h >= len(args) {
				return nil, fmt.Errorf("hole %d reads argument %d of %d", i, h, len(args))
			}
			parts = append(parts, clause.Hole{Term: args[h]}, clause.Text(doc.Pieces[i+1]))
		}
		a := clause.Normalize(clause.And{Conjuncts: []clause.Formula{clause.Atom{Parts: parts}}}).Conjuncts[0].(clause.Atom)
		w, terms, err := clause.Wire(a)
		if err != nil {
			return nil, err
		}
		fn, err := clause.FunctionOf(a)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(w, *doc.Atom) || !reflect.DeepEqual(terms, args) || fn.Name != doc.Function {
			return nil, fmt.Errorf("the atom %q: its wire and function are not those its pieces and arguments make", fn.Name)
		}
		return a, nil
	case doc.Builtin != "":
		if doc.Pieces != nil || doc.Function != "" || doc.Body != nil {
			return nil, fmt.Errorf("a builtin carries no pieces, function or body: %+v", doc)
		}
		return clause.Builtin{Op: clause.Op(doc.Builtin), Args: args, State: doc.State}, nil
	default:
		if doc.Pieces != nil || doc.Function != "" || doc.State != "" || doc.Args != nil {
			return nil, fmt.Errorf("∃ carries no pieces, function, state or arguments: %+v", doc)
		}
		body, err := formulaOfDoc(doc.Body)
		if err != nil {
			return nil, err
		}
		return clause.Exists{Type: doc.Exists, Body: body}, nil
	}
}

func termDocs(ts []clause.Term) []TermDoc {
	if len(ts) == 0 {
		return nil
	}
	out := make([]TermDoc, len(ts))
	for i, t := range ts {
		out[i] = termDoc(t)
	}
	return out
}

func termDoc(t clause.Term) TermDoc {
	switch t := t.(type) {
	case clause.Free:
		return TermDoc{Free: t.Name, Primed: t.Primed}
	case clause.Bound:
		i := t.Index
		return TermDoc{Bound: &i}
	case clause.Index:
		return TermDoc{Index: &IndexDoc{Map: termDoc(t.Map), Key: termDoc(t.Key)}}
	case clause.Field:
		return TermDoc{Field: &FieldOf{Of: termDoc(t.Of), Name: t.Name}}
	case clause.App:
		return TermDoc{App: &AppDoc{Fn: t.Fn, Args: termDocs(t.Args)}}
	}
	panic(fmt.Sprintf("core.termDoc: unknown term %#v", t))
}

func termsOfDocs(docs []TermDoc) ([]clause.Term, error) {
	var out []clause.Term
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
	kinds := 0
	for _, is := range []bool{doc.Free != "", doc.Bound != nil, doc.Index != nil, doc.Field != nil, doc.App != nil} {
		if is {
			kinds++
		}
	}
	if kinds != 1 || doc.Primed && doc.Free == "" {
		return nil, fmt.Errorf("a term is exactly one of free, bound, index, field and app, not %+v", doc)
	}
	switch {
	case doc.Free != "":
		return clause.Free{Name: doc.Free, Primed: doc.Primed}, nil
	case doc.Bound != nil:
		return clause.Bound{Index: *doc.Bound}, nil
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
	default:
		args, err := termsOfDocs(doc.App.Args)
		if err != nil {
			return nil, err
		}
		return clause.App{Fn: doc.App.Fn, Args: args}, nil
	}
}
