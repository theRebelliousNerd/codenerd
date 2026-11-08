package policy

// Package policy holds the declarative Mangle policy corpus (*.mg) and its
// Go regression tests. It intentionally contains no runtime Go code: the
// kernel loads the .mg files at boot. This file exists so `go build ./...`
// has a non-test Go file in an otherwise data-and-test-only package.
