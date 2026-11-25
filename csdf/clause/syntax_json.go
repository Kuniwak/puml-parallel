package clause

import "encoding/json"

// The syntax tree of a clause as JSON (docs/SYNTAX.md, "The syntax tree as
// JSON"): each node an object whose "kind" says what it is, and whose
// "offset" is where it starts in the clause text, in code points.

func (a *SynAtom) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string    `json:"kind"`
		Offset int       `json:"offset"`
		Parts  []SynPart `json:"parts"`
	}{"atom", a.Pos, a.Parts})
}

func (e *SynExists) MarshalJSON() ([]byte, error) {
	body := e.Body.Conjuncts
	if body == nil {
		body = []SynConjunct{}
	}
	return json.Marshal(struct {
		Kind   string        `json:"kind"`
		Offset int           `json:"offset"`
		Name   string        `json:"name"`
		Type   string        `json:"type"`
		Body   []SynConjunct `json:"body"`
	}{"exists", e.Pos, e.Name, e.Type, body})
}

func (b *SynBuiltin) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string    `json:"kind"`
		Offset int       `json:"offset"`
		Op     Op        `json:"op"`
		Args   []SynTerm `json:"args"`
		States []string  `json:"states,omitempty"`
	}{"builtin", b.Pos, b.Op, b.Args, b.States})
}

func (t *SynText) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
	}{"text", t.Text})
}

func (h *SynHole) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string  `json:"kind"`
		Offset int     `json:"offset"`
		Braced bool    `json:"braced"`
		Term   SynTerm `json:"term"`
	}{"hole", h.Pos, h.Braced, h.Term})
}

func (v *SynVar) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string `json:"kind"`
		Offset int    `json:"offset"`
		Name   string `json:"name"`
		Primed bool   `json:"primed"`
	}{"var", v.Pos, v.Name, v.Primed})
}

func (l *SynLit) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string `json:"kind"`
		Offset int    `json:"offset"`
		Text   string `json:"text"`
	}{"lit", l.Pos, l.Text})
}

func (i *SynIndex) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string  `json:"kind"`
		Offset int     `json:"offset"`
		Map    SynTerm `json:"map"`
		Key    SynTerm `json:"key"`
	}{"index", i.Pos, i.Map, i.Key})
}

func (f *SynField) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string  `json:"kind"`
		Offset int     `json:"offset"`
		Of     SynTerm `json:"of"`
		Name   string  `json:"name"`
	}{"field", f.Pos, f.Of, f.Name})
}

func (a *SynApp) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string    `json:"kind"`
		Offset int       `json:"offset"`
		Fn     string    `json:"fn"`
		Args   []SynTerm `json:"args"`
	}{"app", a.Pos, a.Fn, a.Args})
}

func (p *SynPrime) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string  `json:"kind"`
		Offset int     `json:"offset"`
		Of     SynTerm `json:"of"`
	}{"prime", p.Pos, p.Of})
}
