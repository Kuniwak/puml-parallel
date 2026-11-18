//go:build js && wasm

// Command engine is the exploration engine of the web page compiled to
// WebAssembly: it holds the sessions in the browser and answers the page's
// requests as proto.HandleJSON does. It sets globalThis.csdfreplEngine to
// an object whose handle(requestJSON) returns the response JSON.
package main

import (
	"syscall/js"

	"github.com/Kuniwak/puml-parallel/v3/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v3/csdf/animation/proto"
	"github.com/Kuniwak/puml-parallel/v3/version"
)

func main() {
	service := proto.NewService(version.Version, false, proto.AnyEnvironment, animation.NoSuggesters)
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
