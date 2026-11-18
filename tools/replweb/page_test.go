package replweb

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func writeTestPage(t *testing.T, page Page) string {
	t.Helper()
	var buf bytes.Buffer
	engine, err := CommittedEngine()
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteStaticPage(&buf, page, engine); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func scriptOf(t *testing.T, html, id string) string {
	t.Helper()
	m := regexp.MustCompile(`<script type="[^"]+" id="` + id + `">([^<]*)</script>`).FindStringSubmatch(html)
	if m == nil {
		t.Fatalf("no script %s in the page", id)
	}
	return m[1]
}

func TestWriteStaticPageCarriesTheDiagramAndItsDrawing(t *testing.T) {
	// Setup
	page := Page{
		Title:      "vending <machine>",
		Path:       "vm.puml",
		Diagram:    []byte("@startuml\n@enduml\n"),
		Includes:   map[string]string{"a.puml": "x"},
		SVG:        []byte(`<svg id="drawing"></svg>`),
		Highlights: Highlights{States: map[string]string{"s": "ent1"}, Links: map[int]string{3: "lnk2"}},
	}

	// Execute
	html := writeTestPage(t, page)

	// Assert
	if !strings.Contains(html, `<div id="diagram"><svg id="drawing"></svg></div>`) {
		t.Error("the page does not hold the drawing")
	}
	if !strings.Contains(html, "<title>vending &lt;machine&gt;</title>") {
		t.Error("the page does not hold the escaped title")
	}
	var config struct {
		Title       string            `json:"title"`
		Path        string            `json:"path"`
		Diagram     []byte            `json:"diagram"`
		Includes    map[string]string `json:"includes"`
		Highlights  Highlights        `json:"highlights"`
		Highlighted bool              `json:"highlighted"`
		Transport   string            `json:"transport"`
	}
	if err := json.Unmarshal([]byte(scriptOf(t, html, "csdfrepl-config")), &config); err != nil {
		t.Fatal(err)
	}
	if config.Title != page.Title || config.Path != "vm.puml" || string(config.Diagram) != string(page.Diagram) ||
		config.Transport != "wasm" || !config.Highlighted {
		t.Errorf("config = %+v", config)
	}
	if diff := cmp.Diff(page.Includes, config.Includes); diff != "" {
		t.Errorf("includes mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(page.Highlights, config.Highlights); diff != "" {
		t.Errorf("highlights mismatch (-want +got):\n%s", diff)
	}
}

func TestWriteStaticPageCarriesTheEngine(t *testing.T) {
	// Execute
	html := writeTestPage(t, Page{Title: "t", SVG: []byte("<svg></svg>")})

	// Assert
	gz, err := base64.StdEncoding.DecodeString(scriptOf(t, html, "csdfrepl-engine"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatal(err)
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(r, head); err != nil {
		t.Fatal(err)
	}
	if string(head) != "\x00asm" {
		t.Errorf("the engine starts with %q, want the WebAssembly magic", head)
	}
	if !strings.Contains(html, "class Go") && !strings.Contains(html, "globalThis.Go") {
		t.Error("the page does not hold wasm_exec.js")
	}
}

func TestWriteStaticPageSaysWhenNothingIsHighlighted(t *testing.T) {
	// Execute
	html := writeTestPage(t, Page{Title: "t", SVG: []byte("<svg></svg>")})

	// Assert
	if !strings.Contains(scriptOf(t, html, "csdfrepl-config"), `"highlighted":false`) {
		t.Error("the config does not say the drawing is not highlighted")
	}
}

func TestWriteStaticPageKeepsTheConfigInsideItsScript(t *testing.T) {
	// Execute
	html := writeTestPage(t, Page{Title: "</script><b>", Path: "</script>", SVG: []byte("<svg></svg>")})

	// Assert
	if strings.Count(html, "</script>") != 5 {
		t.Errorf("the page closes %d scripts, want 5", strings.Count(html, "</script>"))
	}
}
