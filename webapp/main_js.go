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
	bind("fanamixCore", core.invoke)
	bind("fanamixSetPCM", core.setPCM)
	bind("fanamixClearPCM", core.clearPCM)
	bind("fanamixExportWAV", core.exportWAV)
	js.Global().Set("fanamixReady", true)
	select {}
}
