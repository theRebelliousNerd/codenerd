//go:build !unix

package main

// installGoroutineDumpSignal has no signal to listen on here; the debug-self
// stall watch still writes dumps.
func installGoroutineDumpSignal() {}
