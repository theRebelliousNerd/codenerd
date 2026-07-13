# Articulation feature cards

> Sole authoritative `NERD_FEATURE` surface for this corpus.

## P0: Make processor accounting and truncation safe

<!-- NERD_FEATURE
id: articulation-race-utf8-boundary-v1
owner: articulation
status: verified
kind: truth-gap
depends_on: []
affects: [articulation, cli, observability]
-->

**Value.** Shared response processors report honest counts under parallel shard
traffic, and truncation never emits malformed UTF-8 to the terminal.

**Evidence and observed gap.** `internal/articulation/emitter.go#ResponseProcessor`
mutated `ProcessorStats` without synchronization; surface/reasoning byte slices
could split a multi-byte rune. Statistics now use a mutex and `applyCaps` uses a
UTF-8-valid byte boundary.

**Desired behavior.** Counters are race-free snapshots and byte ceilings preserve
valid text while retaining hard resource bounds.

**Non-goals.** Do not make mutable processor configuration safe to change during
active parsing, redefine byte ceilings as rune ceilings, or remove truncation.

**Affected contracts.** Response processing, CLI rendering, metrics snapshots,
reasoning/surface caps.

**Positive acceptance.** `TestResponseProcessor_ConcurrentStats` reaches the exact
parallel count under `-race`; `TestResponseProcessor_Process_SurfaceTruncationPreservesUTF8`
proves valid UTF-8 and the truncation marker.

**Negative acceptance.** ASCII cap behavior is unchanged; `ResetStats` is locked;
oversized data remains bounded.

**Rollback.** Revert synchronization/helpers/tests together only if callers gain
an enforced single-owner processor contract and equivalent UTF-8-safe rendering.

## P0: Unify exact control application

<!-- NERD_FEATURE
id: articulation-exact-control-gate-v1
owner: articulation
status: proposed
kind: truth-gap
depends_on: []
affects: [articulation, session, core, shards, memory]
-->

**Value.** A control packet behaves identically in chat, clean session, shards,
campaigns, and future transports; malformed or unauthorized effects fail closed.

**Evidence and observed gap.** `internal/articulation/emitter.go#ResponseProcessor.applyCaps`
provides syntactic defense, while actual Mangle assertion, tools, feedback, memory,
and specialist handling live in several consumers. Package filtering cannot prove
`Decl`, arity, provenance, path containment, or `permitted/3`.

**Desired behavior.** Convert parsed controls into typed effect envelopes with
schema version, correlation, provenance, exact target/payload, validation result,
and idempotency key. Route every consumer through the same declaration check and
constitutional decision before applying anything.

**Non-goals.** Articulation does not become the policy engine; syntax acceptance
is not permission; unknown fields/operations do not degrade to execution.

**Affected contracts.** ControlPacket, session executor, kernel facts, memory,
knowledge dispatch, tool resolution, chat paths.

**Positive acceptance.** Cross-system tests feed one packet to every entrypoint
and prove identical allow/deny/application receipts; undeclared/wrong-arity facts,
unknown tools, traversal targets, duplicates, and stale IDs fail closed.

**Negative acceptance.** Surface fallback cannot create an effect; partial failure
does not reapply completed idempotent operations; denied controls never reach an
executor.

**Rollback.** Keep existing consumer adapters behind the typed gate until parity
is proven; rollback selects the previous adapter but retains default deny.

## P1: Generate one versioned Piggyback contract

<!-- NERD_FEATURE
id: articulation-protocol-generation-v1
owner: articulation
status: proposed
kind: leverage
depends_on: [articulation-exact-control-gate-v1]
affects: [articulation, perception, prompt, session, testing]
-->

**Value.** A field added once cannot silently disappear between Go decoding,
provider JSON schema, prompt instructions, streaming, and consumer application.

**Evidence and observed gap.** `protocol_types.go`, `schema.go`, prompt suffixes,
perception provider schema builders, and consumer code encode related protocol
knowledge manually. Tests cover many slices but no generated parity authority.

**Desired behavior.** Define a versioned protocol model that generates or checks
Go types/tags, strict/tolerant schemas, provider variants, prompt atom fragments,
stream field vocabulary, migrations, and conformance fixtures.

**Non-goals.** Do not remove bounded tolerant recovery, force all providers to one
transport API, or let generated schemas authorize controls.

**Affected contracts.** Envelope schema, parser, provider structured output, JIT
atoms, stream parser, session consumers.

**Positive acceptance.** CI proves required/optional keys, enums, bounds, and
migration fixtures agree across every route; a deliberate field drift fails
before build/release.

**Negative acceptance.** Unknown future versions fail or surface without control;
deprecated aliases have an explicit expiry; generators are deterministic.

**Rollback.** Preserve the prior generated artifacts and a bounded version adapter;
never restore unversioned silent acceptance.

## P1: Bound and specify streaming articulation

<!-- NERD_FEATURE
id: articulation-stream-state-machine-v1
owner: articulation
status: proposed
kind: truth-gap
depends_on: [articulation-protocol-generation-v1]
affects: [articulation, cli, perception]
-->

**Value.** The TUI can stream surface text from hostile or malformed responses
without unbounded buffering, decoy-field exposure, races, or ambiguous completion.

**Evidence and observed gap.** `internal/articulation/stream_parser.go#StreamParser.ProcessChunk`
stores the full response in a `strings.Builder`, finds the first textual marker,
and exposes no explicit complete/error/truncated status. Its test file records a
concurrency and decoy gap.

**Desired behavior.** Implement a bounded incremental JSON state machine derived
from the protocol vocabulary. Specify single-owner or synchronized use, maximum
prefix/surface/control buffering, escape/unicode behavior, duplicate/decoy rules,
completion, cancellation, and final reconciliation with full parsing.

**Non-goals.** Do not stream control fields, accept single-quoted pseudo-JSON as
trusted structure, or retain unlimited raw output for debugging.

**Affected contracts.** Streaming TUI, provider chunks, StreamParser API, final
ResponseProcessor reconciliation.

**Positive acceptance.** One-byte chunks, split escapes/runes, decoys, duplicate
surface keys, abrupt EOF, cancellation, and oversized streams remain bounded and
produce deterministic status; race/fuzz tests pass.

**Negative acceptance.** Bytes inside another JSON string cannot trigger surface
streaming; no control content reaches the display; final strict parse disagreement
is observable.

**Rollback.** Retain the current parser behind a bounded compatibility wrapper for
one release; disable live streaming rather than expose control on failure.

## P2: Persist an articulation-to-effect receipt

<!-- NERD_FEATURE
id: articulation-effect-receipt-v1
owner: articulation
status: proposed
kind: north-star
depends_on: [articulation-exact-control-gate-v1, articulation-protocol-generation-v1]
affects: [articulation, session, observability, transparency]
-->

**Value.** Operators can explain what the model said, what the parser accepted,
what policy denied, what actually happened, and what the user saw without storing
the whole response.

**Evidence and observed gap.** `ArticulationResult` holds parse method, confidence,
warnings, surface, control, and raw response in memory; `ProcessorStats` aggregates
counts. Neither is a durable turn-correlated effect record.

**Desired behavior.** Emit a redacted receipt with protocol/parser versions,
response digest, parse/recovery tier, cap/drop reasons, typed control IDs, exact
gate decisions, idempotency/outcome links, surface digest, and retention class.

**Non-goals.** Do not persist raw model output, hidden reasoning, secrets, full
tool payloads, or make the receipt part of permission derivation.

**Affected contracts.** ArticulationResult, executive decision receipts, session
correlation, transparency storage, incident response.

**Positive acceptance.** Success, tolerant fallback, truncation, denial, partial
application, retry, and receipt-store failure each produce a schema-valid and
redaction-safe diagnosis linked to the turn.

**Negative acceptance.** Receipt failure cannot grant/apply control; cache or retry
does not merge distinct turns; high-cardinality content stays out of metrics.

**Rollback.** Disable persistence while retaining structured logs and in-memory
results; receipt loss remains signaled.

## P3: Shadow protocol and parser laboratory

<!-- NERD_FEATURE
id: articulation-shadow-protocol-lab-v1
owner: articulation
status: deferred
kind: moonshot
depends_on: [articulation-effect-receipt-v1]
affects: [articulation, testing, autopoiesis]
-->

**Value.** Candidate protocol/parser changes can be measured against real failure
shapes and attacks without affecting a user or applying model controls.

**Evidence and observed gap.** Package fuzz/boundary fixtures are strong but there
is no receipt-diff shadow runner or promotion contract.

**Desired behavior.** Replay immutable redacted responses through current and
candidate parsers, compare structural receipts, enforce resource/safety gates,
and require human-approved promotion with rollback.

**Non-goals.** Never execute controls, replay secrets, auto-promote from a model
judge, or treat parser agreement as semantic permission.

**Affected contracts.** Test corpus, protocol versions, receipt schema, promotion
governance.

**Positive acceptance.** Fixed inputs produce reproducible diffs and resource
measurements; candidates have no kernel/tool/store handles; promotions cite
hostile negative cases.

**Negative acceptance.** Candidate crashes/timeouts are contained; raw corpus data
is redacted before persistence; production output remains the sole live result.

**Rollback.** Delete lab artifacts and disable shadow dispatch; production parsing
is unchanged.
