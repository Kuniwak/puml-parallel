// Package replweb is the web page of a spec animator: a frontend that
// draws a diagram as PlantUML renders it and drives an exploration session
// through the proto API. The page talks to the session only through one
// function that sends a request and answers its response, so the same page
// runs the engine compiled to WebAssembly (csdfreplweb) or talks to a
// server.
package replweb

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/Kuniwak/puml-parallel/v4/csdf/core"
)

// Highlights tells the page which element of the SVG draws what: States,
// the element of each state by its ID; Links, the link of each edge by its
// source line (core.Edge.Line); Start, the link of the start edge;
// Families, for a promoted diagram, the local diagram of each family by its
// map, as a <<promote>> block draws it. What has no element is left out.
type Highlights struct {
	States   map[string]string           `json:"states"`
	Links    map[int]string              `json:"links"`
	Start    string                      `json:"start,omitempty"`
	Families map[string]FamilyHighlights `json:"families,omitempty"`
}

// FamilyHighlights tells which element of the SVG draws what of the local
// diagram of a family: States, the element of each local state by its ID;
// Edges, the link of each local edge by its index (core.Move.Edge); Start,
// the link of the local start edge; Block, the element of the block it is
// drawn in.
type FamilyHighlights struct {
	States map[string]string `json:"states"`
	Edges  map[int]string    `json:"edges"`
	Start  string            `json:"start,omitempty"`
	Block  string            `json:"block,omitempty"`
}

// link is a link of the SVG: its element, the entities at its ends, and
// the 1-based line PlantUML read it on (0 when it tells none).
type link struct {
	id, from, to string
	line         int
}

// entity is an entity of the SVG: its element, its qualified name, and
// whether it is a start pseudo-state.
type entity struct {
	id, name string
	start    bool
}

// drawing is what MapSVG reads of an SVG, in document order.
type drawing struct {
	entities []entity
	links    []link
}

// MapSVG maps the states and edges of d to the elements of svg, the SVG
// PlantUML renders d's source as. PlantUML marks every link with the
// 0-based line it read it on and the entities at its ends; a state is the
// entity at the end of an edge to or from it, for its own mark does not
// carry an ID of other than ASCII letters.
//
// For a promoted diagram, a local state is the entity whose qualified name
// is that of a block of its family, then its ID; a local edge is a link
// between the entities of its ends, told from its parallels by its order
// among them (core.Move.Parallel). A link inside a block carries the line
// of the file the block includes, so none is taken for an edge of d by its
// line.
func MapSVG(svg []byte, d *core.Diagram) (Highlights, error) {
	dr, err := readDrawing(svg)
	if err != nil {
		return Highlights{}, fmt.Errorf("replweb.MapSVG: %w", err)
	}
	h := Highlights{States: map[string]string{}, Links: map[int]string{}}
	local := map[string]bool{}
	for _, f := range d.Families {
		fh, ids, ok := mapFamily(dr, d, f)
		if !ok {
			continue
		}
		if h.Families == nil {
			h.Families = map[string]FamilyHighlights{}
		}
		h.Families[f.Map] = fh
		for id := range ids {
			local[id] = true
		}
	}
	links := map[int]link{}
	for _, l := range dr.links {
		if l.line > 0 && !local[l.from] && !local[l.to] {
			links[l.line] = l
		}
	}
	if l, ok := links[d.InitLine]; ok && d.InitLine > 0 {
		h.Start = l.id
		h.States[d.Init] = l.to
	} else if l, ok := dr.topStart(); ok && d.InitLine == 0 && len(d.Families) > 0 {
		h.Start = l.id
		h.States[d.Init] = l.to
	}
	for _, edge := range d.Edges {
		l, ok := links[edge.Line]
		if !ok || edge.Line == 0 {
			continue
		}
		h.Links[edge.Line] = l.id
		h.States[edge.Src] = l.from
		h.States[edge.Dst] = l.to
	}
	return h, nil
}

// mapFamily maps the local diagram of f, as the first of its blocks that
// draws any of its states does, and answers the entities inside that block.
func mapFamily(dr drawing, d *core.Diagram, f core.FamilyStates) (FamilyHighlights, map[string]bool, bool) {
	for _, mode := range slices.Sorted(maps.Keys(f.Blocks)) {
		prefix := mode + "." + f.Blocks[mode] + "."
		inside := map[string]bool{}
		byName := map[string]string{}
		var starts []string
		block := ""
		for _, e := range dr.entities {
			if e.name+"." == prefix {
				block = e.id
			}
			if !strings.HasPrefix(e.name, prefix) {
				continue
			}
			inside[e.id] = true
			if e.start {
				starts = append(starts, e.id)
			} else {
				byName[strings.TrimPrefix(e.name, prefix)] = e.id
			}
		}
		fh := FamilyHighlights{States: map[string]string{}, Edges: map[int]string{}, Block: block}
		for _, s := range f.States {
			if id, ok := byName[s]; ok {
				fh.States[s] = id
			}
		}
		if len(fh.States) == 0 {
			continue
		}
		parallels := map[[2]string][]string{}
		for _, l := range dr.links {
			if slices.Contains(starts, l.from) && l.to == fh.States[f.Init] {
				fh.Start = l.id
			}
			if inside[l.from] && inside[l.to] {
				pair := [2]string{l.from, l.to}
				parallels[pair] = append(parallels[pair], l.id)
			}
		}
		for _, edge := range d.Edges {
			for _, m := range edge.Moves {
				if m.Map != f.Map {
					continue
				}
				ls := parallels[[2]string{fh.States[m.Src], fh.States[m.Dst]}]
				if m.Parallel < len(ls) {
					fh.Edges[m.Edge] = ls[m.Parallel]
				}
			}
		}
		return fh, inside, true
	}
	return FamilyHighlights{}, nil, false
}

// topStart is the link from the start pseudo-state of the top of the
// diagram.
func (dr drawing) topStart() (link, bool) {
	for _, e := range dr.entities {
		if e.start && e.name == ".start." {
			for _, l := range dr.links {
				if l.from == e.id {
					return l, true
				}
			}
		}
	}
	return link{}, false
}

// readDrawing reads the entities and links of svg.
func readDrawing(svg []byte) (drawing, error) {
	var dr drawing
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return dr, nil
		}
		if err != nil {
			return drawing{}, fmt.Errorf("the SVG is no XML: %w", err)
		}
		element, ok := token.(xml.StartElement)
		if !ok || element.Name.Local != "g" {
			continue
		}
		attrs := map[string]string{}
		for _, attr := range element.Attr {
			attrs[attr.Name.Local] = attr.Value
		}
		switch attrs["class"] {
		case "link":
			l := link{id: attrs["id"], from: attrs["data-entity-1"], to: attrs["data-entity-2"]}
			if line, err := strconv.Atoi(attrs["data-source-line"]); err == nil {
				l.line = line + 1
			}
			dr.links = append(dr.links, l)
		case "entity", "cluster", "start_entity":
			dr.entities = append(dr.entities, entity{id: attrs["id"], name: attrs["data-qualified-name"], start: attrs["class"] == "start_entity"})
		}
	}
}
