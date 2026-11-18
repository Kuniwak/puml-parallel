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
	"strconv"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
)

// Highlights tells the page which element of the SVG draws what: States,
// the element of each state by its ID; Links, the link of each edge by its
// source line (core.Edge.Line); Start, the link of the start edge. What has
// no element is left out.
type Highlights struct {
	States map[string]string `json:"states"`
	Links  map[int]string    `json:"links"`
	Start  string            `json:"start,omitempty"`
}

// link is a link of the SVG: its element and the entities at its ends.
type link struct {
	id, from, to string
}

// MapSVG maps the states and edges of d to the elements of svg, the SVG
// PlantUML renders d's source as. PlantUML marks every link with the
// 0-based line it read it on and the entities at its ends; a state is the
// entity at the end of an edge to or from it, for its own mark does not
// carry an ID of other than ASCII letters.
func MapSVG(svg []byte, d *core.Diagram) (Highlights, error) {
	links, err := linksByLine(svg)
	if err != nil {
		return Highlights{}, fmt.Errorf("replweb.MapSVG: %w", err)
	}
	h := Highlights{States: map[string]string{}, Links: map[int]string{}}
	if l, ok := links[d.InitLine]; ok && d.InitLine > 0 {
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

// linksByLine reads the links of svg by the 1-based line PlantUML read
// each on.
func linksByLine(svg []byte) (map[int]link, error) {
	links := map[int]link{}
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return links, nil
		}
		if err != nil {
			return nil, fmt.Errorf("the SVG is no XML: %w", err)
		}
		element, ok := token.(xml.StartElement)
		if !ok || element.Name.Local != "g" {
			continue
		}
		attrs := map[string]string{}
		for _, attr := range element.Attr {
			attrs[attr.Name.Local] = attr.Value
		}
		if attrs["class"] != "link" {
			continue
		}
		line, err := strconv.Atoi(attrs["data-source-line"])
		if err != nil {
			continue
		}
		links[line+1] = link{id: attrs["id"], from: attrs["data-entity-1"], to: attrs["data-entity-2"]}
	}
}
