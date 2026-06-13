package core

import (
	"io"

	manglepkg "codenerd/internal/mangle"

	"codeberg.org/TauCeti/mangle-go/parse"
)

// parseUnit is the core package's entry point for the Mangle parser. It delegates
// to mangle.ParseUnit, which holds the single PROCESS-WIDE parse lock.
//
// The ANTLR-generated Mangle parser keeps global ATN / DFA prediction state that
// it mutates while parsing, so concurrent parse calls race on it. The kernel
// parses on construction (loadMangleFiles / loadEmbeddedIntentFacts) and on hot
// paths that run from many goroutines (concurrent transactions, shard spawns,
// query pattern parsing) — AND, during construction, it builds a mangle.Engine
// that parses its schema through the same code. Sharing one lock across both
// packages is what actually eliminates the race; a core-only lock did not,
// because core and mangle parsing interleaved under different locks.
func parseUnit(reader io.Reader) (parse.SourceUnit, error) {
	return manglepkg.ParseUnit(reader)
}
