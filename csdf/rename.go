package csdf

import (
	"fmt"
	"slices"
	"strings"
)

// Rename says that a clause's name From stands for the name To of the
// predicate the clause is in: the clause is read with From bound to the value
// of To. Composition writes it when two synchronised events name a field
// differently, so that no clause text is rewritten (docs/SYNTAX.md).
type Rename struct {
	From string
	To   string
}

const (
	renamingOpen  = ToolWordPrefix + "["
	renamingClose = "] "
	renamingBind  = " := "
)

// splitRenaming takes a clause apart into the renaming written before it,
// @[from := to, ...], sorted by From, and the clause it applies to. A clause
// without one has no renames.
func splitRenaming(clause Predicate) ([]Rename, Predicate, error) {
	text := string(clause)
	if !strings.HasPrefix(text, renamingOpen) {
		return nil, clause, nil
	}
	end := strings.Index(text, renamingClose)
	if end < 0 {
		return nil, "", fmt.Errorf("csdf.splitRenaming: %q: expected %q, then the clause it renames", clause, renamingClose)
	}
	var renames []Rename
	for _, part := range strings.Split(text[len(renamingOpen):end], ", ") {
		from, to, ok := strings.Cut(part, renamingBind)
		if !ok || !isID(from) || !isID(to) {
			return nil, "", fmt.Errorf("csdf.splitRenaming: %q: expected @[name := name, ...]", clause)
		}
		if from == to || slices.ContainsFunc(renames, func(r Rename) bool { return r.From == from }) {
			return nil, "", fmt.Errorf("csdf.splitRenaming: %q: each name is renamed once, to another name", clause)
		}
		renames = append(renames, Rename{From: from, To: to})
	}
	body := Predicate(strings.TrimSpace(text[end+len(renamingClose):]))
	if body == "" || IsOpaque(body) {
		return nil, "", fmt.Errorf("csdf.splitRenaming: %q: a renaming applies to a clause that reads names", clause)
	}
	sortRenames(renames)
	return renames, body, nil
}

// RenameClause is clause read with each From of renames bound to To
// (ParsedClause.Renamed), written back; a clause that comes to say nothing is
// the omitted predicate.
func RenameClause(clause Predicate, renames []Rename) (Predicate, error) {
	if len(renames) == 0 {
		return clause, nil
	}
	c, err := ParseClause(clause)
	if err != nil {
		return "", fmt.Errorf("csdf.RenameClause: %w", err)
	}
	renamed, ok := c.Renamed(renames)
	if !ok {
		return "", nil
	}
	return renamed.Predicate(), nil
}

// RenamesMap is renames as a map from each From to its To.
func RenamesMap(renames []Rename) map[string]string {
	if len(renames) == 0 {
		return nil
	}
	m := make(map[string]string, len(renames))
	for _, r := range renames {
		m[r.From] = r.To
	}
	return m
}

// RenamesFromMap is the renames a map from each From to its To says, sorted.
func RenamesFromMap(m map[string]string) []Rename {
	var renames []Rename
	for from, to := range m {
		renames = append(renames, Rename{From: from, To: to})
	}
	sortRenames(renames)
	return renames
}

func sortRenames(renames []Rename) {
	slices.SortFunc(renames, func(a, b Rename) int { return strings.Compare(a.From, b.From) })
}

const (
	equalityOpen  = ToolWordPrefix + "eq("
	equalityClose = ")"
	equalitySep   = ", "
)

// splitEquality reads the tool word @eq(a, b): the clause that the values
// named a and b are equal. Composition writes it when two synchronised events
// both send one field, so that the two agree on its value without a clause in
// any glue's language. ok is false for a clause that is not one.
func splitEquality(clause Predicate) (a, b string, ok bool, err error) {
	text := string(clause)
	if !strings.HasPrefix(text, equalityOpen) {
		return "", "", false, nil
	}
	inner, closed := strings.CutSuffix(text[len(equalityOpen):], equalityClose)
	a, b, cut := strings.Cut(inner, equalitySep)
	if !closed || !cut || !isID(a) || !isID(b) || a == b {
		return "", "", false, fmt.Errorf("csdf.splitEquality: %q: expected @eq(name, name), two names apart", clause)
	}
	return a, b, true, nil
}

// NewEquality writes the clause @eq(a, b).
func NewEquality(a, b string) Predicate {
	return Predicate(equalityOpen + a + equalitySep + b + equalityClose)
}
