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

// SplitRenaming takes a clause apart into the renaming written before it,
// @[from := to, ...], sorted by From, and the clause it applies to. A clause
// without one has no renames.
func SplitRenaming(clause Predicate) ([]Rename, Predicate, error) {
	text := string(clause)
	if !strings.HasPrefix(text, renamingOpen) {
		return nil, clause, nil
	}
	end := strings.Index(text, renamingClose)
	if end < 0 {
		return nil, "", fmt.Errorf("csdf.SplitRenaming: %q: expected %q, then the clause it renames", clause, renamingClose)
	}
	var renames []Rename
	for _, part := range strings.Split(text[len(renamingOpen):end], ", ") {
		from, to, ok := strings.Cut(part, renamingBind)
		if !ok || !isID(from) || !isID(to) {
			return nil, "", fmt.Errorf("csdf.SplitRenaming: %q: expected @[name := name, ...]", clause)
		}
		if from == to || slices.ContainsFunc(renames, func(r Rename) bool { return r.From == from }) {
			return nil, "", fmt.Errorf("csdf.SplitRenaming: %q: each name is renamed once, to another name", clause)
		}
		renames = append(renames, Rename{From: from, To: to})
	}
	body := Predicate(strings.TrimSpace(text[end+len(renamingClose):]))
	if body == "" || IsOpaque(body) {
		return nil, "", fmt.Errorf("csdf.SplitRenaming: %q: a renaming applies to a clause that reads names", clause)
	}
	sortRenames(renames)
	return renames, body, nil
}

// RenameClause is clause read with each From of renames bound to To. A clause
// that already has a renaming gets the two composed into one; a name that
// comes to stand for itself is left out. @opaque reads nothing, so it is not
// renamed.
func RenameClause(clause Predicate, renames []Rename) (Predicate, error) {
	if len(renames) == 0 || IsOpaque(clause) {
		return clause, nil
	}
	inner, body, err := SplitRenaming(clause)
	if err != nil {
		return "", fmt.Errorf("csdf.RenameClause: %w", err)
	}
	outer := func(name string) string {
		for _, r := range renames {
			if r.From == name {
				return r.To
			}
		}
		return name
	}
	var composed []Rename
	for _, r := range inner {
		composed = append(composed, Rename{From: r.From, To: outer(r.To)})
	}
	for _, r := range renames {
		if !slices.ContainsFunc(inner, func(i Rename) bool { return i.From == r.From }) {
			composed = append(composed, r)
		}
	}
	composed = slices.DeleteFunc(composed, func(r Rename) bool { return r.From == r.To })
	if len(composed) == 0 {
		return body, nil
	}
	sortRenames(composed)
	parts := make([]string, len(composed))
	for i, r := range composed {
		parts[i] = r.From + renamingBind + r.To
	}
	return Predicate(renamingOpen + strings.Join(parts, ", ") + renamingClose + string(body)), nil
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
