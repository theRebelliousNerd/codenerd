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
// Producers (who feeds what):
//
//   - ShardManager (internal/core/shards) calls StartShard / UpdateShardPhase /
//     EndShard / RecordOperation through types.TransparencyManager, which is
//     what makes `/transparency` Active Operations agree with the Glass Box
//     shard lines instead of rendering an empty list.
//   - VirtualStore (internal/core) calls transparency.ReportDeny on every
//     constitutional / permitted refusal and RecordOperation on every routed
//     action, plus ToolEvent + CategoryRouting Glass Box events.
//   - The JIT prompt compiler (internal/prompt) calls transparency.EmitJIT
//     when the JITExplain flag is on.
//
// The last two go through the process-wide handles in process.go because they
// are constructed before, or without, a manager reference. See that file for
// why that indirection exists and how to override it in tests.
//
// Config flags that no producer reads are labelled experimental in GetStatus.
// Do not remove that label without wiring the flag — a status table that
// claims features nobody implements is the failure this package exists to
// prevent.
package transparency
