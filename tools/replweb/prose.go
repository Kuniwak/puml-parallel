package replweb

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
)

// Prose is the clauses of another version of a diagram, written for people,
// which the page shows in place of the diagram's own: the post of the start
// edge, and the clauses of the edges and the guards of the end edges of
// each state, by the ID of the state, in the order the engine numbers its
// outgoing ones. Each is written as animation.ClauseText writes it.
type Prose struct {
	InitPost string               `json:"init_post"`
	Edges    map[string][]Clauses `json:"edges,omitempty"`
	Ends     map[string][]string  `json:"ends,omitempty"`
}

// Clauses are the guard and the post of an edge.
type Clauses struct {
	Guard string `json:"guard"`
	Post  string `json:"post"`
}

// ProseOf is the clauses of prose, another version of d, by the edges of d
// they belong to.
func ProseOf(d, prose *core.Diagram) (*Prose, error) {
	if err := correspond(d, prose); err != nil {
		return nil, err
	}
	pairs, err := pair(d, prose)
	if err != nil {
		return nil, err
	}
	out := &Prose{InitPost: animation.ClauseText(prose.InitPost)}
	for i, e := range d.Edges {
		if out.Edges == nil {
			out.Edges = map[string][]Clauses{}
		}
		p := prose.Edges[pairs[i]]
		out.Edges[e.Src] = append(out.Edges[e.Src], Clauses{Guard: animation.ClauseText(p.Guard), Post: animation.ClauseText(p.Post)})
	}
	for _, e := range prose.Ends {
		if out.Ends == nil {
			out.Ends = map[string][]string{}
		}
		out.Ends[e.Src] = append(out.Ends[e.Src], animation.ClauseText(e.Guard))
	}
	return out, nil
}

// MismatchError says how the prose version differs from the diagram, apart
// from its clauses.
type MismatchError struct{ Difference string }

func (e MismatchError) Error() string {
	return "the prose version is not the same diagram: " + e.Difference
}
func (MismatchError) UserFacing() {}

// correspond says the first way prose differs from d, apart from its
// clauses, or nil when it does not.
func correspond(d, prose *core.Diagram) error {
	if !slices.Equal(d.States, prose.States) {
		return MismatchError{fmt.Sprintf("the states are %s here, %s in the prose", strings.Join(d.States, ", "), strings.Join(prose.States, ", "))}
	}
	for _, st := range d.States {
		if !slices.Equal(d.Vars[st], prose.Vars[st]) {
			return MismatchError{fmt.Sprintf("the variables of %s are %s here, %s in the prose", st, strings.Join(d.Vars[st], ", "), strings.Join(prose.Vars[st], ", "))}
		}
	}
	if !slices.Equal(channelNames(d), channelNames(prose)) {
		return MismatchError{fmt.Sprintf("the channels are %s here, %s in the prose", strings.Join(channelNames(d), ", "), strings.Join(channelNames(prose), ", "))}
	}
	for _, ch := range channelNames(d) {
		if typesText(d.Channels[ch]) != typesText(prose.Channels[ch]) {
			return MismatchError{fmt.Sprintf("the channel %s carries %s here, %s in the prose", ch, typesText(d.Channels[ch]), typesText(prose.Channels[ch]))}
		}
	}
	if d.Init != prose.Init {
		return MismatchError{fmt.Sprintf("the start state is %s here, %s in the prose", d.Init, prose.Init)}
	}
	if len(d.Edges) != len(prose.Edges) {
		return MismatchError{fmt.Sprintf("the diagram has %d edges, the prose %d", len(d.Edges), len(prose.Edges))}
	}
	for i := range min(len(d.Ends), len(prose.Ends)) {
		if d.Ends[i].Src != prose.Ends[i].Src {
			return MismatchError{fmt.Sprintf("the %s end edge leaves %s here, %s in the prose", ordinal(i+1), d.Ends[i].Src, prose.Ends[i].Src)}
		}
	}
	if len(d.Ends) != len(prose.Ends) {
		return MismatchError{fmt.Sprintf("the diagram has %d end edges, the prose %d", len(d.Ends), len(prose.Ends))}
	}
	return nil
}

// pair is, for each edge of d, the index of the edge of prose it is: the
// next one of the same shape (edgeShape). Edges of one shape keep their
// order, and the moves of a promotion's edge tell it apart from the edges
// its clauses may reorder it among in an expansion's canonical order.
func pair(d, prose *core.Diagram) ([]int, error) {
	waiting := map[string][]int{}
	for j, p := range prose.Edges {
		waiting[edgeShape(p)] = append(waiting[edgeShape(p)], j)
	}
	out := make([]int, len(d.Edges))
	for i, e := range d.Edges {
		shape := edgeShape(e)
		if len(waiting[shape]) == 0 {
			return nil, MismatchError{fmt.Sprintf("the %s edge goes %s here, and no edge of the prose goes so", ordinal(i+1), shape)}
		}
		out[i], waiting[shape] = waiting[shape][0], waiting[shape][1:]
	}
	return out, nil
}

// channelNames are the channels of d, sorted.
func channelNames(d *core.Diagram) []string {
	return slices.Sorted(maps.Keys(d.Channels))
}

// typesText is ts as the diagram writes them, separated by commas.
func typesText(ts []types.Type) string {
	words := make([]string, len(ts))
	for i, t := range ts {
		words[i] = t.String()
	}
	return strings.Join(words, ", ")
}

// edgeShape is e without its clauses: for a promotion, with the local
// edges it moves, each as map#index.
func edgeShape(e core.Edge) string {
	shape := fmt.Sprintf("%s --> %s : %s", e.Src, e.Dst, e.Event.Label())
	if len(e.Moves) == 0 {
		return shape
	}
	moves := make([]string, len(e.Moves))
	for i, m := range e.Moves {
		moves[i] = fmt.Sprintf("%s#%d", m.Map, m.Edge)
	}
	return shape + " moving " + strings.Join(moves, ", ")
}

// ordinal is n in English: 1st, 2nd, 3rd, 4th, ...
func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return fmt.Sprintf("%d%s", n, suffix)
}
