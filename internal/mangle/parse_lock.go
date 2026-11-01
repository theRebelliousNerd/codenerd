package mangle

import (
	"context"
	"fmt"
	"io"
	"strings"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/parse"
	"golang.org/x/sync/semaphore"
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
// through it. A large unit parse can hold it long enough to exceed an unrelated
// query's deadline, so context-owned atom waiters must be able to withdraw.
var parseMu = semaphore.NewWeighted(1)

// ParseUnit is the serialized, process-wide entry point for the Mangle parser.
// Callers in any package MUST use this instead of parse.Unit directly so the
// shared ANTLR prediction state is never mutated concurrently.
func ParseUnit(reader io.Reader) (parse.SourceUnit, error) {
	if reader == nil {
		return parse.SourceUnit{}, fmt.Errorf("mangle: ParseUnit called with nil reader")
	}
	_ = parseMu.Acquire(context.Background(), 1)
	defer parseMu.Release(1)
	return parse.Unit(reader)
}

// ParseAtom is the serialized, process-wide entry point for parsing a single
// Mangle atom. It shares the same ANTLR prediction state as ParseUnit, so it
// must take the same lock.
func ParseAtom(s string) (ast.Atom, error) {
	return ParseAtomContext(context.Background(), s)
}

// ParseAtomContext preserves ParseAtom's serialized, full-input semantics but
// lets a caller withdraw while another parser owns the shared ANTLR state.
// It does not preempt a parse already in progress or release another owner.
func ParseAtomContext(ctx context.Context, s string) (ast.Atom, error) {
	if err := parseMu.Acquire(ctx, 1); err != nil {
		return ast.Atom{}, err
	}
	defer parseMu.Release(1)
	atom, err := parse.Atom(s)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ast.Atom{}, ctxErr
	}
	if err != nil {
		return atom, err
	}
	// The upstream parser accepts a prefix and silently drops the rest:
	// `foo(1) garbage` parses as foo(1) with no error. In a logic kernel
	// that turns a typo into a different rule, so verify the whole input
	// was consumed by re-encoding the atom and comparing modulo whitespace
	// and one statement-terminating period.
	if !atomConsumesInput(s, atom) {
		return ast.Atom{}, fmt.Errorf("mangle: trailing input after atom in %q", s)
	}
	return atom, nil
}

// atomConsumesInput reports whether atom accounts for all of s.
func atomConsumesInput(s string, atom ast.Atom) bool {
	strip := func(v string) string {
		v = strings.Join(strings.Fields(v), "")
		return strings.TrimSuffix(v, ".")
	}
	return strip(s) == strip(atom.String())
}

// ParseGroundFacts parses Mangle text holding only ground facts (no rules, no
// declarations) into Facts, with arguments converted exactly as the engine's
// query results are, so a parsed row and a queried row of the same fact
// compare equal.
func ParseGroundFacts(text string) ([]Fact, error) {
	unit, err := ParseUnit(strings.NewReader(text))
	if err != nil {
		return nil, err
	}
	facts := make([]Fact, 0, len(unit.Clauses))
	for _, clause := range unit.Clauses {
		if len(clause.Premises) > 0 || clause.Transform != nil {
			return nil, fmt.Errorf("mangle: %s is a rule, not a ground fact", clause.Head.Predicate.Symbol)
		}
		args := make([]any, len(clause.Head.Args))
		for i, arg := range clause.Head.Args {
			if _, isVar := arg.(ast.Variable); isVar {
				return nil, fmt.Errorf("mangle: %s has a variable argument, not a ground fact", clause.Head.Predicate.Symbol)
			}
			args[i] = convertBaseTermToInterface(arg)
		}
		facts = append(facts, Fact{Predicate: clause.Head.Predicate.Symbol, Args: args})
	}
	return facts, nil
}
