//go:build js && wasm

// Command engine is the exploration engine of the web page compiled to
// WebAssembly: it holds the sessions in the browser and answers the page's
// requests as proto.HandleJSON does. It sets globalThis.csdfreplEngine to
// an object whose handle(requestJSON) returns the response JSON. A session
// suggests values when the page has set globalThis.csdfreplPython, Python
// running csdfbmc (pyhost.Global), by the time it starts.
package main

import (
	"syscall/js"

	"github.com/Kuniwak/puml-parallel/v8/csdf/animation/proto"
	"github.com/Kuniwak/puml-parallel/v8/tools/replweb/pyhost"
	"github.com/Kuniwak/puml-parallel/v8/version"
)

func main() {
	service := proto.NewService(version.Version, false, proto.AnyEnvironment, pyhost.Suggesters(pyhost.Global("csdfreplPython")))
	handle := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 {
			return `{"ok":false,"error":"handle takes one request"}`
		}
		return string(proto.HandleJSON(service, []byte(args[0].String())))
	})
	engine := js.Global().Get("Object").New()
	engine.Set("handle", handle)
	js.Global().Set("csdfreplEngine", engine)
	select {}
}
