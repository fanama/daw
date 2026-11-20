//go:build js && wasm

package main

import "syscall/js"

var jsBindings []js.Func

// bind publishes a Go function on the global object and keeps it alive for the
// lifetime of the page.
func bind(name string, callback func(js.Value, []js.Value) any) {
	function := js.FuncOf(callback)
	jsBindings = append(jsBindings, function)
	js.Global().Set(name, function)
}

func main() {
	core := newCore()
	bind("gorecordCore", core.invoke)
	bind("gorecordSetPCM", core.setPCM)
	bind("gorecordClearPCM", core.clearPCM)
	bind("gorecordExportWAV", core.exportWAV)
	js.Global().Set("gorecordReady", true)
	select {}
}
