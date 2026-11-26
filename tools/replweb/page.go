package replweb

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"text/template"

	"github.com/Kuniwak/puml-parallel/v6/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/v6/csdfbmc"

	"github.com/Kuniwak/puml-parallel/v6/version"
)

//go:embed assets/page.html.tmpl assets/style.css assets/app.js assets/messages.js assets/wasm_exec.js
var assets embed.FS

// Engine is the exploration engine a page carries: the gzipped WebAssembly
// of ./engine (WasmGz) and the wasm_exec.js of the Go toolchain that built
// it (Glue). A nil Glue is the wasm_exec.js of the Go version go.mod names.
type Engine struct {
	WasmGz []byte
	Glue   []byte
}

// Page is what the page shows: the diagram the session explores (Diagram,
// a .puml text or a .png, with the source of every file a promotion in it
// includes, by the path its !include writes), named Path, the drawing
// PlantUML renders of it, and which of its elements draw what.
type Page struct {
	Title      string
	Path       string
	Diagram    []byte
	Includes   map[string]string
	SVG        []byte
	Highlights Highlights
	// Python, when set, makes the page suggest values as csdfrepl -checks
	// does: it loads Python and runs csdfbmc next in it.
	Python *Python
	// Shapes, when set, is what csdfbmc shapes prints of the diagram: the
	// page shows by each input what the values of its type look like.
	Shapes json.RawMessage
	// Prose are the clauses of other versions of the diagram, written for
	// people, which the page shows in place of its own, by the language
	// they are written in (LanguageOf); "" for those of no language.
	Prose map[string]*Prose
}

// Python is what the page's Python reads to suggest values: Pyodide from
// URL (the directory holding pyodide.js), the checker it asks csdfbmc next
// with, and the files the checker names (Files, by the paths the checker
// writes, relative ones read from the directory csdfreplweb ran in).
type Python struct {
	URL     string
	Checker bmc.Checker
	Files   map[string][]byte
}

// WorkDir is the directory of the page's Python that the checker's
// relative paths are read from, and LibDir the one holding csdfbmc's
// package.
const (
	WorkDir = "/work"
	LibDir  = "/lib"
)

// pythonConfig is Python as the page's script reads it: Files by their
// absolute paths in the page's Python, csdfbmc's package among them.
type pythonConfig struct {
	URL     string            `json:"url"`
	Checker bmc.Checker       `json:"checker"`
	Work    string            `json:"work"`
	Lib     string            `json:"lib"`
	Files   map[string]string `json:"files"`
}

func (p *Python) config() (*pythonConfig, error) {
	if p == nil {
		return nil, nil
	}
	files := map[string]string{}
	for name, text := range p.Files {
		files[hostPath(name)] = string(text)
	}
	names, err := fs.Glob(csdfbmc.Sources, "*.py")
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		bs, err := csdfbmc.Sources.ReadFile(name)
		if err != nil {
			return nil, err
		}
		files[path.Join(LibDir, "csdfbmc", name)] = string(bs)
	}
	return &pythonConfig{URL: p.URL, Checker: p.Checker, Work: WorkDir, Lib: LibDir, Files: files}, nil
}

// hostPath is where the page's Python keeps the file the checker names
// name: an absolute path as it is, a relative one under WorkDir.
func hostPath(name string) string {
	name = filepath.ToSlash(name)
	if path.IsAbs(name) {
		return path.Clean(name)
	}
	return path.Join(WorkDir, name)
}

// config is what the page's script reads.
type config struct {
	Title       string            `json:"title"`
	Path        string            `json:"path"`
	Diagram     []byte            `json:"diagram"`
	Includes    map[string]string `json:"includes,omitempty"`
	Highlights  Highlights        `json:"highlights"`
	Highlighted bool              `json:"highlighted"`
	// Transport names how the script reaches the engine: "wasm", the
	// engine carried by the page.
	Transport string `json:"transport"`
	// Python is what the page's Python reads, when it suggests values.
	Python *pythonConfig `json:"python,omitempty"`
	// Shapes is the page's Shapes.
	Shapes json.RawMessage `json:"shapes,omitempty"`
	// Prose is the page's Prose.
	Prose map[string]*Prose `json:"prose,omitempty"`
}

// pageData fills page.html.tmpl; every field is put in as it is, so each
// is HTML already or text a <script> or <style> holds.
type pageData struct {
	Generator, Title, CSS, SVG, Config, Engine, WasmExec, Messages, App string
}

// WriteStaticPage writes the page as one HTML file that needs nothing but a
// browser: it carries the engine, compiled to WebAssembly, which holds the
// session in the browser.
func WriteStaticPage(w io.Writer, page Page, engine Engine) error {
	if err := writePage(w, page, "wasm", engine); err != nil {
		return fmt.Errorf("replweb.WriteStaticPage: %w", err)
	}
	return nil
}

func writePage(w io.Writer, page Page, transport string, engine Engine) error {
	python, err := page.Python.config()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(config{
		Title:       page.Title,
		Path:        page.Path,
		Diagram:     page.Diagram,
		Includes:    page.Includes,
		Highlights:  page.Highlights,
		Highlighted: len(page.Highlights.States) > 0 || len(page.Highlights.Families) > 0,
		Transport:   transport,
		Python:      python,
		Shapes:      page.Shapes,
		Prose:       page.Prose,
	})
	if err != nil {
		return err
	}
	data := pageData{
		Generator: html.EscapeString("csdfreplweb " + version.Version),
		Title:     html.EscapeString(page.Title),
		SVG:       string(page.SVG),
		// json.Marshal writes <, > and & as \u escapes, so no </script> ends
		// the config early.
		Config:   string(encoded),
		Engine:   base64.StdEncoding.EncodeToString(engine.WasmGz),
		WasmExec: string(engine.Glue),
	}
	if engine.Glue == nil {
		glue, err := assets.ReadFile("assets/wasm_exec.js")
		if err != nil {
			return err
		}
		data.WasmExec = string(glue)
	}
	for path, field := range map[string]*string{
		"assets/style.css":   &data.CSS,
		"assets/messages.js": &data.Messages,
		"assets/app.js":      &data.App,
	} {
		bs, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		*field = string(bs)
	}
	tmpl, err := template.ParseFS(assets, "assets/page.html.tmpl")
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}
