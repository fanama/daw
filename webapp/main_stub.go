//go:build !(js && wasm)

// Package main is the browser build of gorecord. The wasm entry point lives in
// main_js.go; this stub keeps native `go build ./...` and `go test ./...`
// working on machines without a WebAssembly toolchain.
package main

func main() {}
