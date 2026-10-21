package csdf

import "encoding/json"

// ClauseJSON is one clause as JSON, as its language reads it: its text with
// the escapes removed, or the tool word @opaque, which has no text. Rename
// maps a name the text reads to the name of the predicate it stands for.
type ClauseJSON struct {
	Text   string            `json:"text,omitempty"`
	Opaque bool              `json:"opaque,omitempty"`
	Rename map[string]string `json:"rename,omitempty"`
	// Eq names the two values the tool word @eq(a, b) says are equal.
	Eq []string `json:"eq,omitempty"`
}

// ClausesJSON takes a predicate apart into its clauses, so a reader in another
// language need not split or unescape it again. It is never null; a clause
// whose renaming does not parse is an error.
func (p Predicate) ClausesJSON() ([]ClauseJSON, error) {
	out := []ClauseJSON{}
	for _, c := range p.Clauses() {
		if IsOpaque(c) {
			out = append(out, ClauseJSON{Opaque: true})
			continue
		}
		if a, b, ok, err := SplitEquality(c); err != nil {
			return nil, err
		} else if ok {
			out = append(out, ClauseJSON{Eq: []string{a, b}})
			continue
		}
		renames, body, err := SplitRenaming(c)
		if err != nil {
			return nil, err
		}
		out = append(out, ClauseJSON{Text: ClauseText(body), Rename: RenamesMap(renames)})
	}
	return out, nil
}

type startEdgeJSON struct {
	Dst         StateID      `json:"dst"`
	Post        Predicate    `json:"post"`
	PostClauses []ClauseJSON `json:"post_clauses"`
	Line        int          `json:"line"`
}

func (e StartEdge) MarshalJSON() ([]byte, error) {
	post, err := e.Post.ClausesJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(startEdgeJSON{Dst: e.Dst, Post: e.Post, PostClauses: post, Line: e.Line})
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
	guard, err := e.Guard.ClausesJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(endEdgeJSON{Src: e.Src, Guard: e.Guard, GuardClauses: guard, Line: e.Line})
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
