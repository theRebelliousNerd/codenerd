// Package transparency provides visibility into codeNERD's internal operations.
//
// The transparency layer makes the "magic" visible to users on demand:
//
//   - Shard execution phases: Which shard is running, what phase, how long
//   - Safety gate explanations: Why constitutional rules blocked an action
//   - JIT explain mode: Which prompt atoms were selected and why
//   - Proof trees: Derivation chains for Mangle facts
//   - Operation summaries: What happened after each significant operation
//   - Error categorization: Typed errors with remediation suggestions
//
// Key Design Principles:
//
//  1. Opt-in: All transparency features are toggled via TransparencyConfig
//  2. Non-intrusive: Does not modify the core execution path
//  3. Lazy: Expensive computations (proof trees) only run when requested
//  4. Informative: Explains "why" not just "what"
//
// See the noble-sprouting-emerson.md plan file for full architecture details.
package transparency
