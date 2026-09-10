// Package observation encodes what a tool observed into the structure the
// reasoning actually needs, and keeps the raw observation reachable.
//
// The problem it exists for is specific. A code search returns a wall of
// matching lines. Those lines enter the model's context verbatim, cost a great
// many tokens, and carry almost no structure: the model has to re-derive
// "which symbols are these, and what depends on what" from text it has already
// paid for, on every turn the transcript survives. The wall is also the worst
// possible thing to carry forward, because most of it is incidental — the same
// identifier in a comment, a vendored copy, a test fixture.
//
// A codec answers that by paying once. It projects the observation into
// symbols and dependency edges at the moment the tool runs, when the source is
// at hand, and hands the model the projection. Nothing is thrown away: the raw
// observation is retained under a handle, and the handle is redeemable.
//
// Two boundaries are load-bearing and are the reason this package is separate
// from both the tools that produce observations and the retention that holds
// them:
//
//  1. Retention is not duplicated. internal/retain already does
//     content-addressed storage with TTL, byte and entry ceilings and an
//     eviction hook, and internal/mcp already composes it with an MCP-specific
//     projection. This package is the second composition, not the second
//     store. Two retention layers means two eviction policies and two ways for
//     a published citation to expire.
//
//  2. Hydration reads the retained bytes and cannot do anything else. Re-running
//     a search to expand it would answer from a world that has moved: an agent
//     that reasoned about a symbol at line 40, then expanded and found
//     something else there, has been handed a contradiction it cannot
//     diagnose — and the re-run may not even be a pure read. The guarantee is
//     structural rather than documented: CodeSearch holds a retain.Store and
//     nothing else, projection takes its source reader as a call argument, and
//     Hydrate therefore has no reader to call. It is not possible to make
//     Hydrate touch a file without changing the type.
package observation
