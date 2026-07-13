# Articulation: one response, two trustworthy channels

> Corpus: `articulation` | Live owner: `internal/articulation` | Verified: 2026-07-13

## In one minute

Articulation is the boundary between a model's free-form response and codeNERD's
deterministic runtime. The Piggyback envelope carries a user-facing
`surface_response` beside a typed `control_packet` containing intent metadata,
candidate Mangle updates, memory/context feedback, specialist requests, and tool
requests. Articulation also assembles outbound shard prompts from kernel context
and JIT atoms.

The visible outcome is graceful: users see useful prose even when an envelope is
markdown-wrapped or truncated, while control data is parsed, capped, filtered,
and handed to downstream policy rather than displayed or executed as text.

`VERIFIED CURRENT` — the inbound authority is
`internal/articulation/emitter.go#ResponseProcessor.Process`; the outbound bridge
is `internal/articulation/prompt_assembler.go#PromptAssembler.AssembleSystemPrompt`.

## Its place in codeNERD

The LLM creatively proposes an answer and structured control hints. Articulation
separates and validates those channels. It is not the executive: a syntactically
acceptable `mangle_update` or tool request does not imply
`permitted(Action, Target, Payload)`. Session/core policy must validate the exact
effect before assertion or execution.

```text
kernel + session + JIT atoms -> PromptAssembler -> model
model bytes -> JSON/markdown/embedded recovery -> PiggybackEnvelope
  surface_response -----------------------------> user
  control_packet -> caps + syntax filters -> session/kernel exact gate
                                      -> effect -> articulated result
```

This makes articulation a transduction boundary in both directions, not a second
planner and not a hidden authorization path.

## A representative journey

Suppose a coder shard returns an envelope saying, “I updated the retry helper,”
plus one `mangle_update` and a required `run_tests` request.

1. `ResponseProcessor.Process` tries direct JSON, markdown JSON, and bounded
   embedded candidates. Last-match extraction resists a leading decoy object.
2. Protocol types tolerate documented provider coercions, then `applyCaps` bounds
   the surface, reasoning, updates, memory operations, tools, and knowledge calls.
   Mangle-looking strings with invalid shape or shell metacharacters are dropped.
3. The user-facing surface remains separate. If JSON was truncated, the processor
   salvages `surface_response` or emits a friendly truncation explanation rather
   than dumping raw control data.
4. Session consumes the control packet. It must validate and authorize exact
   facts/tools; articulation's filters are defense in depth, not permission.
5. Parse method, confidence, warnings, and processor counters provide diagnosis.

`VERIFIED CURRENT` — hostile update filtering is covered by
`internal/articulation/emitter_boundary_test.go#TestApplyCaps_MangleUpdates_RejectsShellMetachars`;
decoy scanning is covered in `internal/articulation/json_scanner_test.go`; package
tests pass with `go test -count=1 ./internal/articulation/...`.

## What exists today

| Applicability lane | Evidence-backed answer |
|---|---|
| Mangle | `VERIFIED CURRENT` — control packets carry candidate atom strings and `applyCaps` performs bounded syntax/metacharacter filtering. Outbound `GetKernelContext` queries prompt/context predicates. `PARTIAL` — this package does not prove declarations, arity, provenance, or `permitted/3`; consumers must use the executive gate. |
| Permission and safety | `VERIFIED CURRENT` — response/control caps, bounded JSON depth/candidate size, strict unknown-field mode, decoy-resistant scanning, and constitutional override helpers exist. `PARTIAL` — tolerant fallback intentionally returns surface without control, and not every consumer is proven to share one exact control-application gate. |
| Fact flow | `VERIFIED CURRENT` — model output becomes surface plus `ControlPacket`; session may validate/assert updates, execute requests, and return an articulated result. In the reverse direction kernel/session/JIT context becomes a model system prompt. |
| JIT and agents | `VERIFIED CURRENT` — `PromptAssembler` maps session/intent state into a prompt compilation context, uses JIT by default when wired, appends the Piggyback protocol, and retains bounded emergency templates. `PARTIAL` — hard-coded fallback identities can drift from canonical atoms. |
| Wiring | `VERIFIED CURRENT` — session, chat, shards, perception schema builders, and system factory consume articulation APIs. `PARTIAL` — memory operations, feedback, knowledge re-entry, and constitutional surface override do not have proven parity across every LLM entrypoint. |
| State and concurrency | `VERIFIED CURRENT` — processor statistics are now mutex-protected and race-tested; `PromptAssembler` has lock/race coverage. `PARTIAL` — `StreamParser` is intentionally stateful but lacks an enforced single-owner contract or synchronization. |
| Recovery | `VERIFIED CURRENT` — direct, markdown, embedded, partial-surface salvage, friendly truncation, tolerant field coercion, and plain-text fallback are explicit tiers. `PARTIAL` — fallback loses the executive channel and there is no built-in schema repair/retry policy. |
| Observability | `VERIFIED CURRENT` — parse method, confidence, warnings, timers, structured category logs, and `ProcessorStats` exist. `PARTIAL` — stats are process-local and no durable receipt joins parsed control, downstream permission, actual effect, and final surface. |
| Testing | `VERIFIED CURRENT` — unit, boundary, malformed-input, decoy, benchmark, fuzz entrypoint, focused race, prompt assembly, and stream chunk tests exist. `PARTIAL` — full consumer parity and production control-application paths need cross-system conformance tests. |

The exact types and recovery state machine are in [Implemented Spec](IMPLEMENTED_SPEC.md)
and [Internal Architecture](05-INTERNAL-ARCHITECTURE.md).

## North star

Every model response should produce an inspectable, redacted articulation receipt:
which parser/recovery tier ran, which schema version was used, what was capped or
rejected and why, which exact control operations reached the executive, what was
permitted or denied, what effect occurred, and which surface the user saw. All
entrypoints should consume one versioned protocol and one exact-effect gate.

Non-goals:

- Articulation never authorizes tools or Mangle effects.
- Tolerant parsing never means silent control execution.
- Hidden reasoning and raw sensitive payloads are not persisted for convenience.
- Streaming the surface must not expose the control packet.
- Emergency prompt templates remain minimal recovery, not a parallel prompt
  authority.

## Improvement frontier

1. `VERIFIED CURRENT` — make shared `ResponseProcessor` statistics race-safe and
   preserve UTF-8 at byte caps; focused and race regressions cover both repairs.
2. `PROPOSED UPLIFT` — route every control consumer through one typed validation,
   declaration/provenance, constitutional permission, and application contract.
3. `PROPOSED UPLIFT` — generate Go types, provider schemas, prompt instructions,
   and consumer conformance fixtures from one versioned Piggyback protocol model.
4. `PROPOSED UPLIFT` — give streaming an explicit bounded state machine with
   single-owner enforcement or synchronization, decoy handling, and completion
   status; never buffer unbounded model output.
5. `PROPOSED UPLIFT` — correlate a redacted articulation receipt with prompt,
   permission, tool/fact outcome, and user response.
6. `DEFERRED` — shadow candidate protocol/parser versions against a fixed hostile
   response corpus before promotion, with no live control application.

Acceptance, negative cases, and rollback live in [TODO](TODO.md).

## Choose a reading route

| Time | Route |
|---|---|
| 90 seconds | This README, then [Current State](02-CURRENT-STATE.md) and [Gap Analysis](03-GAP-ANALYSIS.md). |
| 10 minutes | [Internal Architecture](05-INTERNAL-ARCHITECTURE.md), [Wiring](08-WIRING-AND-INTEGRATION.md), [Safety](09-SAFETY-AND-INVARIANTS.md), and [Failure Modes](12-FAILURE-MODES.md). |
| Deep implementation | [Implemented Spec](IMPLEMENTED_SPEC.md), [Public API](06-PUBLIC-API-AND-TYPES.md), [Schema/Dependencies](07-DEPENDENCY-MAP.md), and [Testing](10-TESTING-ALIGNMENT.md). |
| Build or review an uplift | [Vision](01-VISION.md), [Gap Analysis](03-GAP-ANALYSIS.md), [TODO](TODO.md), [Open Questions](OPEN-QUESTIONS.md), then [_progress](_progress.md). |
