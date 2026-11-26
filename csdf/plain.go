package csdf

import (
	"fmt"
	"strings"
)

// Plain is the plain diagram tree says (docs/SYNTAX.md): its states, their
// variables, its edges, and what its notes assume and declare, each clause
// as written. A tree the plain grammar does not have, such as a global
// diagram's, is refused; a failure is a *ParseError at the element refused,
// and is a *PromotionHintError when that element is a promotion directive
// or holds one: a diagram holding directives is a global one, whose
// edges are not the whole of its behaviour.
func Plain(tree *SyntaxTree) (*Diagram, error) {
	r := refuser{global: HoldsDirectives(tree.Body)}
	d := &Diagram{
		Name:        tree.Name,
		States:      map[StateID]State{},
		Edges:       []Edge{},
		Assumptions: []Assumption{},
	}
	// current is the state whose variables may come next, "" for none.
	current := StateID("")
	ended := false
	for _, e := range tree.Body {
		switch e := e.(type) {
		case *CommentSyntax, *IgnoreSyntax:
			continue
		case *ObserveSyntax:
			d.Observes = append(d.Observes, observeOf(e))
			continue
		}
		if _, ok := e.(*NoteSyntax); ended && !ok {
			return nil, r.refuse(e, e.Position(), "expected @enduml after end edge at line %d, col %d", e.Position().Line, e.Position().Col)
		}
		next := StateID("")
		switch e := e.(type) {
		case *StateSyntax:
			if e.Composite || e.Stereotype != "" {
				return nil, r.refuse(e, e.Pos, "the state %s at line %d, col %d has a stereotype or a block, which a plain diagram's state does not have", e.ID, e.Line, e.Col)
			}
			d.States[StateID(e.ID)] = State{Name: e.Label, Vars: []StateVar{}, Line: e.Line}
			next = StateID(e.ID)
		case *StateVarSyntax:
			if StateID(e.State) != current || current == "" {
				return nil, r.refuse(e, e.Pos, "unexpected syntax at line %d, col %d", e.Line, e.Col)
			}
			s := d.States[current]
			s.Vars = append(s.Vars, StateVar{Name: Var(e.Name), Type: e.Type})
			d.States[current] = s
			next = current
		case *StartEdgeSyntax:
			d.StartEdge = StartEdge{Dst: StateID(e.Dst), Post: e.Post, Line: e.Line}
		case *EdgeSyntax:
			d.Edges = append(d.Edges, PlainEdge(e))
		case *EndEdgeSyntax:
			d.EndEdge = &EndEdge{Src: StateID(e.Src), Guard: e.Guard, Line: e.Line}
			ended = true
		case *NoteSyntax:
			if err := plainNote(d, e, r); err != nil {
				return nil, err
			}
		default:
			return nil, r.refuse(e, e.Position(), "unexpected syntax at line %d, col %d", e.Position().Line, e.Position().Col)
		}
		current = next
	}
	if err := observesWellFormed(d); err != nil {
		return nil, fmt.Errorf("csdf.Plain: %w", err)
	}
	return d, nil
}

// refuser refuses an element of a diagram, global when the diagram holds
// promotion directives.
type refuser struct{ global bool }

// refuse is a refusal at pos of the element of, hinted (PromotionHintError)
// when of is a promotion directive or holds one, or when the diagram is
// global and of is what a global diagram has and a plain one does not: a
// preprocessor line, a state with a stereotype or a block, a note.
func (r refuser) refuse(of Element, pos Pos, format string, args ...any) error {
	var err error = &ParseError{Line: pos.Line, Col: pos.Col, err: fmt.Errorf("csdf.Plain: "+format, args...)}
	if HoldsDirectives([]Element{of}) || r.global && isGlobalSyntax(of) {
		err = &PromotionHintError{err: err}
	}
	return err
}

// isGlobalSyntax reports whether e is what a global diagram has and a
// plain one does not. Keep it in step with the global diagram grammar
// (docs/SYNTAX.md, "Global Diagram Format") and the plain one.
func isGlobalSyntax(e Element) bool {
	switch e.(type) {
	case *PragmaSyntax, *StateSyntax, *NoteSyntax:
		return true
	}
	return false
}

// PlainEdge is the edge e says, its clauses still syntax trees.
func PlainEdge(e *EdgeSyntax) Edge {
	fields := make([]Field, 0, len(e.Event.Fields))
	for _, f := range e.Event.Fields {
		fields = append(fields, Field{Dir: Direction(f.Dir), Name: f.Name})
	}
	return Edge{
		Src:   StateID(e.Src),
		Dst:   StateID(e.Dst),
		Event: NewEvent(e.Event.Channel, fields...),
		Guard: e.Guard,
		Post:  e.Post,
		Line:  e.Line,
	}
}

func observeOf(o *ObserveSyntax) Observe {
	out := Observe{Name: o.Name, Line: o.Line}
	if o.View {
		out.States = []StateID{}
		for _, s := range o.States {
			out.States = append(out.States, StateID(s))
		}
	}
	return out
}

// plainNote adds to d what the note n assumes or declares. A plain diagram's
// note floats and holds one assume directive, or channel declarations; a
// note for the picture goes in a CSDF-IGNORE region, as every other
// PlantUML-only line does.
func plainNote(d *Diagram, n *NoteSyntax, r refuser) error {
	const hint = "a note holds one assume directive (\"note as <id>\", \"assume <text>\", \"end note\") or channel declarations (\"channel <name> : <type>.<type>\"); a note for the picture goes in a CSDF-IGNORE region"
	if n.ID == "" {
		return r.refuse(n, n.Pos, "a note is floating, not anchored to a state, at line %d, col %d: %s", n.Line, n.Col, hint)
	}
	if len(n.Lines) == 0 {
		return r.refuse(n, n.Pos, "the note at line %d holds no assume directive and no channel declaration: %s", n.Line, hint)
	}
	var assumption *Assumption
	var channels []ChannelDecl
	for _, l := range n.Lines {
		switch l := l.(type) {
		case *AssumeSyntax:
			if assumption != nil {
				return r.refuse(n, l.Pos, "the note at line %d holds a second assume directive at line %d; write one note per assumption", n.Line, l.Line)
			}
			if channels != nil {
				return r.refuse(n, l.Pos, "the note at line %d declares channels and holds an assume directive at line %d; write the assume directive in a note of its own", n.Line, l.Line)
			}
			if strings.TrimSpace(l.Text) == "" {
				return r.refuse(n, l.Pos, "the assume directive at line %d has no text", l.Line)
			}
			assumption = &Assumption{ID: n.ID, Text: Predicate(l.Text), Line: l.Line}
		case *ChannelSyntax:
			if assumption != nil {
				return r.refuse(n, l.Pos, "the note at line %d holds an assume directive and declares a channel at line %d; write the assume directive in a note of its own", n.Line, l.Line)
			}
			decl, err := ParseChannelDecl(l.Text, l.Line)
			if err != nil {
				return &ParseError{Line: l.Line, Col: l.Col, err: fmt.Errorf("csdf.Plain: %w", err)}
			}
			channels = append(channels, decl)
		default:
			return r.refuse(n, l.Position(), "unexpected line %d in the note at line %d: %s", l.Position().Line, n.Line, hint)
		}
	}
	if assumption != nil {
		d.Assumptions = append(d.Assumptions, *assumption)
	}
	d.Channels = append(d.Channels, channels...)
	return nil
}

// HoldsDirectives reports whether body holds a promotion directive (a
// global diagram's, docs/PROMOTION.md): a
// <<promote>> block, an !include, or a note holding a sync or constrain
// directive.
func HoldsDirectives(body []Element) bool {
	for _, e := range body {
		switch e := e.(type) {
		case *StateSyntax:
			if e.Stereotype == "promote" || HoldsDirectives(e.Body) {
				return true
			}
		case *PragmaSyntax:
			if e.Name == "include" {
				return true
			}
		case *NoteSyntax:
			for _, l := range e.Lines {
				switch l.(type) {
				case *SyncSyntax, *ConstrainSyntax:
					return true
				}
			}
		}
	}
	return false
}

// observesWellFormed refuses an observation of d that docs/SYNTAX.md
// refuses: a view named like a state or a variable, one listing a state
// d lacks, its start state or a state twice, a variable no state holds,
// and a view or variable declared twice.
func observesWellFormed(d *Diagram) error {
	vars := map[string]bool{}
	for _, s := range d.States {
		for _, v := range s.Vars {
			vars[string(v.Name)] = true
		}
	}
	declared := map[string]bool{}
	for _, o := range d.Observes {
		if declared[o.Name] {
			return fmt.Errorf("the CSDF-OBSERVE %s at line %d is declared twice", o.Name, o.Line)
		}
		declared[o.Name] = true
		if !o.IsView() {
			if !vars[o.Name] {
				return fmt.Errorf("the CSDF-OBSERVE variable %s at line %d is held by no state", o.Name, o.Line)
			}
			continue
		}
		if _, ok := d.States[StateID(o.Name)]; ok {
			return fmt.Errorf("the view %s at line %d is named like a state", o.Name, o.Line)
		}
		if vars[o.Name] {
			return fmt.Errorf("the view %s at line %d is named like a state variable", o.Name, o.Line)
		}
		listed := map[StateID]bool{}
		for _, s := range o.States {
			if _, ok := d.States[s]; !ok {
				return fmt.Errorf("the view %s at line %d lists %s, which is no state", o.Name, o.Line, s)
			}
			if s == d.StartEdge.Dst {
				return fmt.Errorf("the view %s at line %d lists %s, the start state, which is absence; @notin reads it", o.Name, o.Line, s)
			}
			if listed[s] {
				return fmt.Errorf("the view %s at line %d lists %s twice", o.Name, o.Line, s)
			}
			listed[s] = true
		}
	}
	return nil
}
