# Prompt JIT gap analysis

> Current evidence: [Current State](02-CURRENT-STATE.md). Target contract:
> [Vision](01-VISION.md). Authoritative implementation cards: [TODO](TODO.md).

## Method

Claims were traced from the live `internal/prompt` tree through production
construction and consumers, then checked with strict atom validation, ordered
runtime parity, package tests, and focused production-adapter race tests. A source
path is not proof until its named symbol and behavioral discriminator agree.

## Spec versus reality

| Capability | Reality | Verdict |
|---|---|---|
| Runtime atom compilation | Full collect -> select -> resolve -> fit -> assemble path in `internal/prompt/compiler.go#JITPromptCompiler.Compile` | `VERIFIED CURRENT` |
| Deterministic skeleton | Requires a kernel and Mangle-selected identity/protocol/safety/methodology in `internal/prompt/selector.go#AtomSelector.loadSkeletonAtoms` | `VERIFIED CURRENT` |
| Degradable flesh | Vector/flesh errors fall back or drop while skeleton survives | `VERIFIED CURRENT` |
| Exact cache identity | `CompilationContext.Hash` uses `compilation-context-v2`, length-prefixed fields, and canonical set encoding; retry/tool changes produce different keys | `VERIFIED CURRENT` (closed G1) |
| Request-scoped logic facts | Production `KernelAdapter.NewCompilationScope` clones the live `RealKernel`; deferred close discards selector facts on every exit | `VERIFIED CURRENT` (closed G2); third-party adapter residual |
| One atom schema | Validator, filesystem loader, embedded loader, and synchronizer use `ParsePromptAtomYAML`; 333 files produce the same ordered 888 IDs | `VERIFIED CURRENT` (closed G3) |
| Tool/config capability generation | Production provider is live; alternate `SimpleRegistry` catalog is test-only and drifted | `PARTIAL` (G5) |
| Decision receipt | In-memory manifest and logs exist, but no durable turn/permission/outcome correlation | `PARTIAL` (G4) |
| Safe prompt evolution | Evolved atom directories and manager exist; shadow/replay promotion evidence is absent | `PARTIAL` (G8) |
| Verification breadth | 235 listed tests; full prompt/sync and focused production scope/cache race gates pass; no fuzz or external-adapter conformance suite | `PARTIAL` (G7) |

## Closed truth gaps

### Closed G1: no-tool retry cache collision

`VERIFIED CURRENT` — `internal/prompt/context.go#CompilationContext.Hash` now hashes
every prompt-affecting scalar and set-like field under schema
`compilation-context-v2`. Values are length-prefixed; frameworks and available
tools are sorted and deduplicated without mutating caller state.
`internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_RetryContextBypassesPreRetryCache`
compiles an initial prompt, changes retry state and tools, and proves the retry
atom plus exact new tools appear without stale `read_file` capability text.

### Closed G2: production prompt facts shared across compiles

`VERIFIED CURRENT` — `internal/prompt/compiler.go#acquireCompilationKernel` asks a
`KernelScopeProvider` for a compilation-owned evaluator. Production
`internal/system/factory_adapters.go#KernelAdapter.NewCompilationScope` clones the
primary `RealKernel`; skeleton and flesh assert/query only that clone and deferred
scope close discards it on success, budget error, cancellation, and panic.

Evidence:

- `internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_CompilationScopesIsolateConcurrentPrompts`
- `internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_CompilationScopeDoesNotLeakOnBudgetError`
- `internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_CompilationScopeDoesNotLeakOnCancellation`
- `internal/prompt/compiler_scope_test.go#TestJITPromptCompiler_CompilationScopeClosesAfterSelectorPanic`
- focused `go test -race` over the four production scope/cache tests: PASS

`PARTIAL` residual — a third-party `KernelQuerier` that implements neither
`KernelScopeProvider` nor `KernelRetracter` still compiles through the compatibility
path without guaranteed isolation. Predicate-wide retraction is also unsafe for
concurrent third-party compiles, so adapter conformance or fail-closed behavior is
still a product decision.

### Closed G3: atom authoring/runtime contract drift

`VERIFIED CURRENT` — `internal/prompt/atom_schema.go#AtomDefinition` and
`ParsePromptAtomYAML` are the single versioned strict contract. Unknown fields,
unsupported schema versions, invalid sequence members, duplicate IDs, invalid
dependencies, and unknown world states fail closed. Legacy `agent_types`, metadata,
nested selectors, and semantic-string versions are observable migrations bounded
to 2027-01-01; built-ins may not use them.

The two checked-in legacy files are canonical now. `KnownWorldStates` derives from
the typed live dimensions and includes `reflection_hits` and
`no_tool_call_retry`. `cmd/tools/validate_prompt_atoms#TestCheckedInCorpusOrderedParity`
pins validator/filesystem/embedded order, count 888, and ordered-ID digest.

Receipt: `go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms
-fail-on-warn` -> 333 files, 888 atoms, zero issues.

## Open gaps

### Gap G4: manifest is not an end-to-end decision receipt

`internal/prompt/manifest.go#PromptManifest` records context hash, token usage,
selected atoms, and dropped atoms. It does not identify the session/turn, exact
source version, permission decision, model response, tool effect, redaction policy,
or retention. `GetLastResult` is one process-local pointer.

Blocked goal: G4. Severity: important operability.

### Gap G5: alternate ConfigAtom catalog is dormant and drifted

Production uses `internal/prompt/config_factory.go#NewDefaultConfigAtomProvider`.
`internal/prompt/config_defaults.go#RegisterDefaultConfigAtoms` is referenced only
by tests and uses a different intent/tool vocabulary. Policy-set names now share a
canonical registry, but the tool capability catalogs are not generated from the
live registry.

Blocked goal: G3 contract consolidation. Severity: important maintenance.

### Gap G6: cache TTL and cached-result ownership are underspecified

`CompilerConfig.CacheTTLSeconds` defaults to 300, but entries carry no timestamp
and lookup enforces no TTL. Size eviction and explicit clear are live. Cache hits
also return a shared `*CompilationResult`; callers are expected to treat it as
immutable, but the type does not enforce that contract.

Blocked goal: G1 hardening. Severity: important freshness and ownership honesty.

### Gap G7: verification residuals remain

There is no `Fuzz*` target. Full prompt/sync and focused production adapter tests
pass under `-race`; however, external adapters still lack a conformance suite
proving scope-or-fail-closed behavior.

Blocked goals: operational hardening. Severity: important verification.

### Gap G8: evolved atoms lack shadow/replay evidence

`EvolvedAtomManager` can load promoted and pending files, but there is no
turn-correlated shadow comparison, outcome rubric, promotion receipt, or safe
counterfactual replay. Automated optimization without those gates could confound
model, context, prompt, and tool-policy changes.

Blocked goal: G5. Severity: strategic; moonshot remains deferred.

## North-star alignment map

| Gap | State | Required evidence |
|---|---|---|
| G1 retry/cache identity | Closed | Keep v2 field and retry regressions green; version future field changes |
| G2 production isolation | Closed with external-adapter residual | Require scope-capable adapters or fail closed; add adapter conformance tests |
| G3 atom contract drift | Closed | Keep strict CLI and ordered 888-ID parity green; remove migrations on schedule |
| G4 decision receipt | Open | Versioned redacted schema; one receipt per model call; permission/outcome correlation test |
| G5 dormant config catalog | Open | One authority or generated tool parity; live registry conformance test |
| G6 TTL/ownership | Open | Tested TTL semantics or removed setting; immutable/copy-on-read result contract |
| G7 verification residuals | Open | Fix test mock race; add fuzz and external-adapter conformance gates |
| G8 no shadow/replay | Open | Shadow-only diff receipts, manual promotion, rollback drill, no side-effect replay |

## Priority order

### Important now

1. G7 pin external-adapter isolation policy and add conformance/fuzz gates.
2. G5 generate or remove the dormant capability catalog.
3. G6 make TTL and cached-result ownership honest.
4. G4 persist a redacted end-to-end receipt.

### Strategic after receipts

5. G8 shadow selection and only then the deferred replay lab.

## Recommendations

| Horizon | Recommendation | Rollback boundary |
|---|---|---|
| Immediate | Add scope-provider conformance tests and pin fail-closed behavior for unscoped adapters | Retain compatibility wrapper during migration |
| Immediate | Require `KernelScopeProvider` in production-facing constructors or explicitly reject unscoped adapters | Compatibility flag or adapter wrapper during migration |
| Short term | Generate or delete alternate ConfigAtom tool catalog; test live registry parity | Revert generated manifest to last known-good version |
| Short term | Implement or remove cache TTL; make cached results immutable/copy-on-read | Clear process cache and restore prior ownership contract |
| Short term | Add durable redacted receipt and inspector diff | Disable persistence while preserving in-memory manifest |
| Strategic | Shadow compiler and bounded replay lab | Shadow off switch; never replay tools; atom promotion remains manual |
