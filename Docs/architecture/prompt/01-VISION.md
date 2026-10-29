# Prompt JIT vision

> Target state, not a claim of current behavior. Last reconciled: 2026-07-13.

## Vision statement

Each codeNERD model call receives a deterministic, context-true, budget-bounded
instruction program whose selection can be explained, replayed, and related to
the executive decision without letting prompt prose authorize effects.

## Strategic goals

### Goal G1: Correctness before cache speed

Two contexts that can produce different prompt text, selected atoms, budget fit,
or runtime capability descriptions must never share a cache entry. Cache identity
will be defined from an immutable compilation input rather than an informal subset
of struct fields. A cache hit will return an immutable result or a safe copy, and
TTL/size behavior will be explicit. Success closes the no-tool-nudge collision and
the broader omissions documented as gaps G1 and G6.

### Goal G2: Compile-scoped logic, even under concurrency

Every compilation will have an isolated fact lifetime. Candidate metadata,
context, vector hits, selected facts, and derived results will be associated with
a compile ID or evaluated in a sandbox/transaction that disappears on success,
failure, cancellation, and panic. Concurrent turns will never see one another's
language, intent, shard, or world state. This preserves Mangle as executive while
closing gap G2 instead of bypassing logic with a second Go-only selector.

### Goal G3: One atom contract from authoring to runtime

YAML authoring, embedded loading, SQLite synchronization, offline validation,
selector vocabularies, and documentation will consume one versioned schema. An
unknown field will either be rejected everywhere or admitted through an explicit
migration; aliases such as `agent_types` will never be silently ignored by one
loader and rejected by another. Success means the checked-in atom tree passes its
own strict validator and the validator's atom count agrees with runtime loading.
This closes gap G3.

### Goal G4: Prompts become explainable executive inputs

For each model call, a redacted decision receipt will connect compilation context,
atom version/hash and source, Mangle selection reason, dependency and conflict
decisions, render mode, budget drops, runtime capability descriptions, and the
later permission/outcome correlation ID. Operators will be able to ask why an atom
appeared, why another disappeared, and what changed between turns. Receipts will
support diagnosis without storing secrets or complete user payloads. This closes
gap G4 and upgrades the current in-memory manifest from a compiler snapshot into
an operational contract.

### Goal G5: Safe evolution through shadow evidence

Prompt changes will first run in shadow against the same immutable context, while
only the production compilation reaches the model. Diffed receipts will reveal
selection, budget, and capability changes before rollout. Later, a bounded replay
lab may compare alternatives against redacted contexts and outcome rubrics, but it
will not replay tools or auto-promote safety/identity atoms. This addresses gaps G7
and G8 without turning prompt optimization into uncontrolled self-modification.

## Guiding principles

1. **Typed truth before prose.** Context and permissions are facts/contracts;
   prompt text teaches the model but never becomes executive authority.
2. **Deterministic skeleton, optional flesh.** Identity, protocol, safety, and
   methodology remain logic-selected. Semantic retrieval may enrich context but
   must degrade safely.
3. **Isolation is part of correctness.** A clean final fact store is insufficient
   if concurrent compiles could observe one another while running.
4. **Receipts over folklore.** Selection and budget claims require machine-readable
   evidence linked to one turn, not a log line without correlation.
5. **Evolution is staged.** Validate -> shadow -> compare -> explicitly promote ->
   observe -> rollback. Safety atoms never self-promote from a single outcome.

## Non-goals

- The prompt package will not decide user intent; perception owns that input.
- It will not execute tools, approve actions, or replace downstream `permitted/3`.
- It will not store fuzzy natural-language banks as Mangle facts; embeddings may
  retrieve candidates, after which logic reasons over typed metadata.
- It will not maximize prompt size or atom count. A smaller, evidenced program may
  be superior.
- It will not preserve obsolete ConfigAtom registries merely for compatibility;
  an unused catalog must have a migration owner or be removed deliberately.

## Success metrics

| Metric | Current evidence | Target | Measurement |
|---|---|---|---|
| Prompt-affecting cache collisions | `VERIFIED CURRENT` — `internal/prompt/context.go#CompilationContext.Hash` uses the versioned `compilation-context-v2` identity, length-prefixes values, hashes every prompt-affecting field, and canonicalizes set-like slices; the production retry regression proves a distinct result | Keep zero in field-completeness/property tests | Change every prompt-affecting field independently; assert hash/result changes or an explicit non-affecting classification |
| Compile fact isolation | `VERIFIED CURRENT` — production `KernelAdapter.NewCompilationScope` clones the live `RealKernel`; concurrent mixed contexts and error/cancel paths leave no live prompt facts | Keep zero cross-turn visibility during 100+ concurrent mixed-context compiles | Race/integration test using `internal/system/factory_adapters.go#KernelAdapter` plus a real kernel/compiler |
| Atom contract parity | `VERIFIED CURRENT` — strict validator, filesystem runtime, and embedded runtime share `ParsePromptAtomYAML`; all report the same ordered 888 IDs over 333 YAML files with no migrations or warnings | Keep validator/runtime ordered parity green | Strict validator + embedded/filesystem digest comparison in CI |
| Receipt completeness | In-memory manifest covers selected/dropped atoms but not durable turn/permission/outcome correlation | 100% model calls emit one redacted receipt with a stable correlation ID | Session integration test and retained receipt schema validation |
| Prompt package tests | 235 listed tests; full prompt/sync and focused production scope/cache race gates pass; no fuzz entry point | Unit, integration, race, fuzz, and shadow-diff gates for high-risk contracts | `go test`, `go test -race`, fuzz seed corpus, production-adapter integration profile |
| Rollback time | No measured selector-policy rollback receipt | One configuration/atom promotion can be reverted and verified in under 5 minutes | Campaign drill with before/after receipts and no tool replay |

## Relationship to codeNERD's north star

Prompt JIT is the transduction layer that lets a creative model receive rich,
specific guidance without making the model its own executive. The system is most
valuable when Mangle makes a deterministic decision, prompt compilation explains
that decision in the model's language, and the later effect remains behind a
default-deny gate. The north star is therefore not “better prompting” in isolation;
it is a verifiable chain from formal truth to creative work and back to formal,
observable outcomes.

## Uplift roadmap

### Phase 1: Truth repair

- Close G1 with a field-classified immutable cache key and the real no-tool-retry
  regression.
- Close G2 with production-adapter cleanup and compile-scoped isolation.
- Close G3 by making strict validation and runtime loading share the atom schema.

### Phase 2: Contract consolidation

- Resolve G5 by selecting one live ConfigAtom authority or generating capability
  descriptors from the registered tool surface.
- Resolve G6 by implementing tested TTL semantics or removing the unused option.
- Add fuzz and production-adapter integration gates from G7.

### Phase 3: Explainability

- Close G4 with a versioned, redacted prompt decision receipt.
- Add inspector diffs, retention configuration, and correlation with permission
  and execution outcome while keeping content redaction on by default.

### Phase 4: Shadow evolution

- Run candidate selector and budget policies in shadow.
- Only after stable shadow evidence, prototype the deferred replay lab for G8 with
  no tool execution and manual promotion of constitutional content.

## North-star alignment matrix

| Goal | Supporting gaps | Roadmap phase | Priority |
|---|---|---|---|
| G1 Correctness before cache speed | G1, G6 | 1-2 | Critical |
| G2 Compile-scoped logic | G2, G7 | 1-2 | Critical |
| G3 One atom contract | G3 | 1 | Critical |
| G4 Explainable executive inputs | G4 | 3 | Important |
| G5 Safe shadow evolution | G7, G8 | 4 | Strategic |
