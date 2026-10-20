package csdf

import "encoding/json"

// ClauseJSON is one clause as JSON, as its language reads it: its text with
// the escapes removed, or the tool word @opaque, which has no text. Rename
// maps a name the text reads to the name of the predicate it stands for.
type ClauseJSON struct {
	Text   string            `json:"text,omitempty"`
	Opaque bool              `json:"opaque,omitempty"`
	Rename map[string]string `json:"rename,omitempty"`
}

// ClausesJSON takes a predicate apart into its clauses, so a reader in another
// language need not split or unescape it again. It is never null.
func (p Predicate) ClausesJSON() []ClauseJSON {
	out := []ClauseJSON{}
	for _, c := range p.Clauses() {
		if IsOpaque(c) {
			out = append(out, ClauseJSON{Opaque: true})
			continue
		}
		renames, body, _ := SplitRenaming(c)
		j := ClauseJSON{Text: ClauseText(body)}
		for _, r := range renames {
			if j.Rename == nil {
				j.Rename = map[string]string{}
			}
			j.Rename[r.From] = r.To
		}
		out = append(out, j)
	}
	return out
}

type startEdgeJSON struct {
	Dst         StateID      `json:"dst"`
	Post        Predicate    `json:"post"`
	PostClauses []ClauseJSON `json:"post_clauses"`
	Line        int          `json:"line"`
}

func (e StartEdge) MarshalJSON() ([]byte, error) {
	return json.Marshal(startEdgeJSON{Dst: e.Dst, Post: e.Post, PostClauses: e.Post.ClausesJSON(), Line: e.Line})
}

// UnmarshalJSON reads the post whole and derives its clauses from it: the
// text wins over post_clauses, which are only a reader's convenience.
func (e *StartEdge) UnmarshalJSON(data []byte) error {
	var j startEdgeJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	*e = StartEdge{Dst: j.Dst, Post: j.Post, Line: j.Line}
	return nil
}

type endEdgeJSON struct {
	Src          StateID      `json:"src"`
	Guard        Predicate    `json:"guard"`
	GuardClauses []ClauseJSON `json:"guard_clauses"`
	Line         int          `json:"line"`
}

func (e EndEdge) MarshalJSON() ([]byte, error) {
	return json.Marshal(endEdgeJSON{Src: e.Src, Guard: e.Guard, GuardClauses: e.Guard.ClausesJSON(), Line: e.Line})
}

// UnmarshalJSON reads the guard whole and derives its clauses from it: the
// text wins over guard_clauses, which are only a reader's convenience.
func (e *EndEdge) UnmarshalJSON(data []byte) error {
	var j endEdgeJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	*e = EndEdge{Src: j.Src, Guard: j.Guard, Line: j.Line}
	return nil
}
