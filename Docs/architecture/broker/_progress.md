# broker — architecture corpus progress

## 2026-09-09 — Initial build: the single accounting boundary

**Scope.** Replace every independent token-counting and budget authority in the
codebase with one metered boundary backed by provider-reported counts.

### What was found first

An audit of the pre-existing state, recorded in
[02-CURRENT-STATE.md](02-CURRENT-STATE.md):

- 145 LLM completion call sites across 76 files.
- Three independent token-counting implementations, of which the one used for
  every budgeting decision was `charsPerToken = 4.0`.
- Seven budget authorities that could not see each other; two of them each
  believed they owned the whole configured window.
- Provider-exact usage already flowing through `internal/usage` on every
  response, consumed by cost display and **no budgeting decision anywhere**.

That last point is the finding the whole change set turns on.

### What shipped

`internal/broker` (2,320 lines of implementation), installed as a
capability-preserving decorator inside the three constructors in
`internal/perception/client_factory.go`. No completion call site changed.

- Anthropic pre-flight counts come from `POST /v1/messages/count_tokens`,
  LRU-cached, 5s-bounded, degrading to the estimator with a lower confidence
  label rather than failing.
- Every other provider gets a calibrating estimator that corrects itself against
  the provider's reported actual after each response.
- One ledger, segment-attributed, enforcing the window and optional per-purpose
  caps, fail-closed on an uncountable request.
- One receipt per call carrying estimate, actual, confidence, decision, duration,
  and estimate error percentage.

### Findings from the wiring audit

`TestEveryClientConstructionPathIsBrokered` failed on its first run and named
three exported constructors. Two were false positives (delegation through
`newSecondarySlotClient`, which is itself metered — the audit was tightened to
verify that rather than assume it). The third was real:

- **`NewImageClientFromUserConfig` built a Gemini client directly and was
  spending entirely off the books.** Now metered.

`internal/perception`'s factory tests then failed, asserting on concrete client
types that now return wrapped. That is the documented breaking change and the
fix is `broker.Base` — the same call the one affected production site
(`cmd/nerd/chat/model_session_context.go`, the Codex CLI engine tag) now makes.
The tests were updated rather than relaxed to interface checks: they are the
reason the production break was found at all.

### Deletions

Breaking changes, taken deliberately rather than leaving parallel systems:

| Removed | Replaced by |
|---|---|
| `context.TokenCounter.charsPerToken`, `CharsPerTokenEstimator`, `NewTokenCounterWithEstimator` | `broker.TextCounter` |
| `session/semantic_compressor.go` `const maxTokens = 64000` (named tokens, measured characters) | `summarizationCharAllowance()`, derived from the ledger |
| `session.DefaultTokenBudget` const | `DefaultTokenBudget()`, derived from the ledger |
| `init/jit_integration.go` literal `120000` | `PromptBudget(0.75, …)` |
| `articulation` literal `60000` clamp | `PromptBudget(0.3, …)` |

### One change outside the package

`internal/usage/observer.go` plus four lines in `TrackFromContext`. This is how
the broker learns actual usage on paths whose return values carry none —
fifteen lines in plumbing every provider already uses, rather than five methods
across nine provider clients.

### Verification receipts

```
go build ./...                     exit 0
go vet ./internal/broker/          exit 0
go test ./internal/broker/         ok      0.44s   coverage 76.5%
go test -race ./internal/broker/   ok      4.85s
go test ./...                      78 ok, 0 FAIL, 3 no-test-files
```

Regression surface run in full: `internal/usage`, `internal/context`,
`internal/session`, `internal/perception`, `internal/system`,
`internal/articulation`, `internal/init`.

### Scope explicitly not taken

Latency pricing, lossless native round-trip, the typed task graph, economic
rebasing, and lanes. Each is recorded in
[13-ROADMAP-AND-GATES.md](13-ROADMAP-AND-GATES.md) with the evidence that would
justify it. Two of them are gated on histograms that do not exist yet and are
each about a day's work — those should be the next thing anyone does, because
between them they decide months of build.
