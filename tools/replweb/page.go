package replweb

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"text/template"

	"github.com/Kuniwak/puml-parallel/v3/version"
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
	encoded, err := json.Marshal(config{
		Title:       page.Title,
		Path:        page.Path,
		Diagram:     page.Diagram,
		Includes:    page.Includes,
		Highlights:  page.Highlights,
		Highlighted: len(page.Highlights.States) > 0 || len(page.Highlights.Families) > 0,
		Transport:   transport,
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
