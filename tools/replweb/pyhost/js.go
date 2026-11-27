//go:build js && wasm

package pyhost

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/Kuniwak/puml-parallel/v9/csdf/animation/bmc"
)

// Global finds the host the page sets as globalThis[name] once Python is
// ready: an object whose checker is the bmc.Checker as JSON,
// writeFile(path, text) and remove(path) keep files, and run(args) answers
// the Result as JSON. Without it, or before the page sets it, there is no
// host.
func Global(name string) Find {
	return func() (Host, bmc.Checker, bool) {
		v := js.Global().Get(name)
		if v.IsUndefined() || v.IsNull() {
			return nil, bmc.Checker{}, false
		}
		var c bmc.Checker
		if err := json.Unmarshal([]byte(v.Get("checker").String()), &c); err != nil {
			return nil, bmc.Checker{}, false
		}
		return jsHost{v}, c, true
	}
}

type jsHost struct{ v js.Value }

func (h jsHost) WriteFile(path string, data []byte) error {
	_, err := call(h.v, "writeFile", path, string(data))
	return err
}

func (h jsHost) Remove(path string) error {
	_, err := call(h.v, "remove", path)
	return err
}

func (h jsHost) Run(args []string) (Result, error) {
	words := make([]any, len(args))
	for i, a := range args {
		words[i] = a
	}
	out, err := call(h.v, "run", words)
	if err != nil {
		return Result{}, err
	}
	var r Result
	if err := json.Unmarshal([]byte(out.String()), &r); err != nil {
		return Result{}, fmt.Errorf("the page's Python answered something that is not a result: %w", err)
	}
	return r, nil
}

// call calls v's method, an exception it throws becoming an error.
func call(v js.Value, method string, args ...any) (out js.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("the page's Python failed: %v", r)
		}
	}()
	return v.Call(method, args...), nil
}
