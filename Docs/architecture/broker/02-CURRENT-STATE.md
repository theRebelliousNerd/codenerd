# Current state — what the meter looked like before

> Snapshot taken 2026-09-09 on branch `claude/jit-context-management-2mv7tq`.
> This document records the pre-broker reality so the gap analysis has a baseline
> and so nobody re-derives these findings from scratch.

## The sprawl, measured

`145` call sites invoke an LLM completion method across `76` files:

```
grep -rn "\.Complete(\|\.CompleteWithSystem(\|\.CompleteWithTools(\
\|\.CompleteWithStreaming(\|\.CompleteWithToolResults(" --include="*.go" internal/ cmd/ \
  | grep -v "_test.go" | wc -l
```

The heaviest are `internal/system/factory_adapters.go` (9),
`internal/perception/tracing_client.go` (9), `internal/core/scheduled_llm_client.go` (7),
`cmd/nerd/cmd_campaign.go` (6), `internal/autopoiesis/tool_generation.go` (5).

Two of those are already decorators over `types.LLMClient` — `tracing_client.go`
and `scheduled_llm_client.go` — which is the pattern the broker adopts rather
than inventing a new one.

## Three token-counting implementations that disagreed

### 1. `internal/context/tokens.go` — the chars-per-token heuristic

```go
func NewTokenCounter() *TokenCounter {
	return &TokenCounter{
		charsPerToken: 4.0, // Claude's approximate ratio
	}
}
```

Rune count divided by four. Structural overheads for facts and turns were
guessed on top (`4 + predicate`, `+2 for quotes`, `10` base per turn). There was
a `TokenEstimator` interface allowing a real tokenizer to be substituted, and
nothing ever substituted one.

Error against a real tokenizer is roughly ±20% on prose and materially worse on
code and non-English text. Every budget in the compressor was denominated in this
unit.

### 2. `internal/prompt/budget.go` — per-atom render-mode charging

The JIT compiler's `TokenBudgetManager.Fit()` charged atoms by render mode, then
`enforceAssembledBudget()` re-checked after template expansion because
`{{available_specialists}}` interpolates the entire agent registry over a
25-character placeholder. The code comment records why that second stage exists:

> Until this step existed the overshoot was detected in `logCompilationStats` and
> then shipped anyway: the contract said "bounded", the wire said otherwise.

The two-stage check is good engineering. It was still counting in heuristic units.

### 3. Provider `usage` blocks — exact, reported, and largely unused for control

Every provider client already extracts real usage and forwards it:

```go
trackUsage(ctx, c.model, ProviderAnthropic,
	anthropicResp.Usage.InputTokens, anthropicResp.Usage.OutputTokens, usageOpChat)
```

`internal/usage` aggregated this for cost reporting and a status bar. **No
budgeting decision anywhere in the codebase consumed it.** The exact number was
collected, stored, displayed — and then every admission decision was made against
the estimate instead.

That is the single most consequential finding in this audit. The ground truth was
already in the process, already correct, already free, and structurally
disconnected from every component that needed it.

## Budget authorities that could not see each other

| Authority | Budget | Source | Knows about the others? |
|---|---|---|---|
| `context.CompressorConfig.TotalBudget` | 200,000, split 5/30/15/50 | `ContextWindow.MaxTokens` | No |
| `prompt.CompilationContext.TokenBudget` | 200,000, clamped by `GetEffectiveJITConfig` | `ContextWindow.MaxTokens` | No |
| `session.ExecutorConfig.TokenBudget` | `DefaultTokenBudget = 65536` | hard-coded | No |
| `init/jit_integration.go:446` | `120000` | hard-coded literal | No |
| `articulation/prompt_assembler.go` | clamps to `60000` for two shard types | hard-coded literal | No |
| `session/semantic_compressor.go` | `maxTokens = 64000` | hard-coded, unreferenced elsewhere | No |
| `campaign/context_pager.go` | `200000` default | hard-coded literal | No |

Both of the first two read the same configured window and neither subtracted the
other. On paper each could claim the entire window. In practice the collision was
masked by the conservative hard-coded constants further down the table — numbers
nobody derived, chosen by people who could tell the accounting did not reconcile.

## Session context windows: three of them, three policies

**Chat level** (`cmd/nerd/chat/process.go`) — token-budget driven, binary switch.
`IsCompressionActive()` gates it: below threshold, all raw history is sent; at or
above, a recent window plus a compressed block. `chat_history_parity_test.go`
statically audits the package so no new path can inject raw history without the
gate, which indicates this has failed before.

**Session executor** (`internal/session/executor.go`) — not token-driven at all.
`appendToHistory` clamps each turn to `maxHistoryTurnChars = 8000` and thoughts
to `2000`, then truncates to the last 50 messages. At send time it takes
`DefaultHistoryTurnWindow = 6` messages within `DefaultHistoryCharBudget = 24000`
characters. A character budget, not a token budget, and unaware of the compressor.

**Compressor** (`internal/context/`) — the real machinery. Compression triggers
on utilization ≥ 0.60, explicitly not on turn count. Last 5 turns verbatim,
older turns collapse into a rolling summary of Mangle atoms.

## Two latent defects in a dangling field

`prompt.CompilationContext` declares:

```go
// ActivatedFacts maps fact string representation to activation score (0.0-1.0).
// Populated by the compression system's GetActivationScores().
ActivatedFacts map[string]float64
ActivationThreshold float64
```

Nothing in production writes `ActivatedFacts` and nothing reads it. The only
non-test reference is the declaration. `Compressor.GetActivationScores()` exists;
its only in-repo caller is a sibling function in the same file.

Wiring it naively creates two new defects:

1. `Hash()` writes `activation_threshold` but not `ActivatedFacts`. Two turns
   with entirely different hot facts collide on one cache entry and the model is
   served a stale prompt. No error is raised.
2. `Clone()` does `clone := *cc` and deep-copies `Frameworks` and `AvailableTools`
   only. The map is shared by reference. Compilation runs under `singleflight`
   with an `errgroup`, and `activation_race_test.go` already exists — concurrent
   compiles mutating a shared map is a data race, not a hypothetical.

The complete fix is three coordinated changes — populate, hash, deep-copy — and
any subset introduces a new defect. Recorded here because it is the cheapest
available test of the meaning-driven-compilation premise, and because the trap is
easy to walk into. Tracked in [TODO.md](TODO.md).

## Untracked text appended after compilation

`cmd/nerd/chat/process.go` queries `final_system_prompt` from the kernel, then:

```go
systemPrompt += "\n\n" + stevenMoorePersona
```

Small in tokens. Fatal to the claim that the kernel governs what the model sees,
and invisible to any accounting done upstream of that line. The broker counts at
the outbound boundary specifically so that appends like this are charged rather
than missed.

## Truncation was already tail-aware — a correction

An earlier revision of this corpus claimed that `prompt.ClampText` /
`ClampHead` truncate from the front and decapitate Go test output. **That was
wrong**, and the record is corrected here rather than quietly edited away.

`ClampText` has been head+tail since it was written, and `internal/prompt/limits.go`
documents the exact reasoning the claim attributed to its absence: *"a stack
trace ends with the panic, `go test` output ends with FAIL and the failing
package list … Head-only truncation on those inputs removes exactly the line the
turn exists to act on."* Tool results and conversation turns use `ClampText`.
`ClampHead` is head-only by design and every one of its ten callers is on
single-line or head-dominant content — a specialist entry, one diagnostic, a
session-context line.

The real defect in that area was narrower and is fixed on this branch:
`ClampText` cut at a byte offset, so the surviving ends routinely began or ended
mid-line. A half-line reads to a model as a whole record — `FAIL
github.com/example/parser` cut after `…/pars` names a package that does not
exist. Cuts now snap to a line boundary when the snap costs less than an eighth
of the budget, and fall back to the raw cut past that so a single-line minified
bundle is not discarded chasing a newline that never comes.

Separately, `ClampLines` — written for exactly this line-oriented case — had
tests and **zero production callers**. Making `ClampText` line-aware serves every
caller instead of migrating them one at a time.
