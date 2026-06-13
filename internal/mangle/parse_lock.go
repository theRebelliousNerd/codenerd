package mangle

import (
	"io"
	"sync"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/parse"
)

// parseMu serializes ALL Mangle source parsing across the process.
//
// The ANTLR-generated Mangle parser (mangle-go/parse) keeps process-global ATN /
// DFA prediction state that it MUTATES while parsing (the ParserATNSimulator's
// adaptivePredict caches DFA states). The library pools lexer/parser objects, but
// that prediction cache is shared by every parser instance, so two goroutines
// calling parse.Unit concurrently race on it — confirmed by the race detector
// during concurrent kernel construction (core.loadMangleFiles parsing at the same
// time as mangle.Engine parsing its schema).
//
// This lock is the single, process-wide chokepoint. The mangle package is the
// lowest layer that owns Mangle parsing, and everything that parses (the engine,
// the schema validator, and — via mangle.ParseUnit — the core kernel) funnels
// through it. Parsing is cheap relative to evaluation, so serializing it is not a
// throughput concern.
var parseMu sync.Mutex

// ParseUnit is the serialized, process-wide entry point for the Mangle parser.
// Callers in any package MUST use this instead of parse.Unit directly so the
// shared ANTLR prediction state is never mutated concurrently.
func ParseUnit(reader io.Reader) (parse.SourceUnit, error) {
	parseMu.Lock()
	defer parseMu.Unlock()
	return parse.Unit(reader)
}

// ParseAtom is the serialized, process-wide entry point for parsing a single
// Mangle atom. It shares the same ANTLR prediction state as ParseUnit, so it
// must take the same lock.
func ParseAtom(s string) (ast.Atom, error) {
	parseMu.Lock()
	defer parseMu.Unlock()
	return parse.Atom(s)
}
