# JIT capability envelope — gap analysis

> Gaps compare [02-CURRENT-STATE.md](02-CURRENT-STATE.md) with
> [01-VISION.md](01-VISION.md). A missing consumer is not presented as an
> implemented feature.

## Spec versus reality matrix

| ID | Capability | Current reality | Target | Verdict |
|---|---|---|---|---|
| J1 | Uniform validation | specialist YAML validates with hostile regressions; generated/fallback paths do not uniformly require it | every boundary validates | **PARTIAL** |
| J2 | Explicit degraded mode | nil/empty execution is fail-closed, but baseline and zero-value fallbacks are implicit | named, bounded, fail-closed degradation | **PARTIAL** |
| J3 | Executable policy identity | default providers resolve stable set IDs to validated canonical global-corpus members; no set version or per-agent application | explicit set identity/version and pinned global/selective semantics | **PARTIAL** |
| J4 | Honest field consumption | loop/safety/model/workspace/persona fields lack or compete with consumers | one owner and negative test per field | **PARTIAL** |
| J5 | Capability provenance | config pointer reaches executor without a stable producer/version receipt | immutable turn snapshot and reason | **NO** |
| J6 | Complete diagnostics | compiler stats and logs exist but do not correlate effective grants to effects | redacted capability receipt | **PARTIAL** |
| J7 | Safe comparative evolution | no no-effect config comparison | receipt-driven shadow comparison after J1–J6 | **NO / DEFERRED** |

## Built but not previously specified clearly

- Specialist names reject path separators and `..`, and YAML is capped at one
  megabyte in `internal/session/spawner.go#loadSpecialistConfig`.
- The normal tool catalog silently drops allowlist names missing from the modular
  registry in `internal/session/executor.go#buildToolDefinitions`.
- The executor keeps constitutional, Dreamer, timeout, and postcondition gates
  after the JIT capability check in
  `internal/session/executor_tools.go#executeToolCall`.
- Compiler and spawner have different fallback layers; this is behavior, not one
  documented recovery state machine.

## Specified or implied but not built

- `Safety.RequirePolicyEnforcement` does not enforce policy loading or deny a
  config in the session hot path.
- `Policies` resolves against the embedded global inventory but does not carry a
  stable set ID/version or apply a selectively loaded corpus per agent.
- `ToolLoop` does not set the executor's active iteration/call/error behavior.
- `Model` and `Workspace.RootPath` do not select a per-agent model or sandbox.
- No effective-capability receipt proves which producer, fallback, grants, policy
  set, and limits reached a turn.

## Partially implemented contracts

| Gap | Proven slice | Missing seam |
|---|---|---|
| J1 validation | `Validate`, table tests, and mandatory specialist-YAML validation | mandatory calls after factory/fallback creation |
| J2 degradation | baseline prompt, reduced-budget retry, deny-all empty config | typed reason and explicit narrow grants if degraded tools are ever desired |
| J3 policy | canonical resolution, boot-inventory parity, and shared factory sets | set identity/version and explicit per-agent versus global meaning |
| J4 field use | allowlist and intent metadata have consumers | loop/safety/model/workspace/persona ownership and parity |
| J5 provenance | compiler manifest and last-result stats | immutable config ID carried into session/effect telemetry |
| J6 diagnosis | CategoryJIT/Session logs | correlated, redacted, retention-bounded receipt |

## North-star alignment map

| Gap | Lane | Goal blocked | Consequence |
|---|---|---|---|
| J1 | wiring, testing | J1 — One valid boundary value | generated/fallback configs can cross without the same validation proof as specialist YAML |
| J2 | safety, recovery | J1 and J4 | empty config denies tools but operators still see only fallback prose rather than a typed reason |
| J3 | Mangle, permission | J2 and J3 | canonical membership is proven, but it does not yet change per-agent executive rules or identify a versioned set |
| J4 | JIT, state, testing | J2 | schema promises controls whose owners are elsewhere or absent |
| J5 | state, observability | J4 | no immutable answer to “which config governed this turn?” |
| J6 | observability, recovery | J4 | degradation and capability drift require log reconstruction |
| J7 | uplift | enabled by J1–J6 | comparison would be unsafe and unmeasurable today |

## Priority assessment

### Critical

- **J1 / Goal J1:** extend the verified specialist validation contract to
  generated and fallback configs.
- **J2 / Goals J1 and J4:** preserve verified deny-all nil/empty semantics and
  define a typed degradation/reason contract.
- **J3 / Goal J3:** preserve the verified canonical registry and define
  set/version identity plus global-versus-selective enforcement semantics.

### Important

- **J4 / Goal J2:** wire or remove each decorative field and pin loop precedence.
- **J5–J6 / Goal J4:** carry a versioned, redacted receipt from producer through
  executor.

### Nice to have

- **J7:** no-effect shadow comparison only after the operational contracts pass.
- Per-agent model/workspace selection only when a real product need justifies the
  containment and provider-routing complexity.

## Recommendations

1. **Verified slice:** keep validation after specialist YAML with negative tests
   for blank identity and missing policies (J1).
2. **Verified slice plus residual:** keep unlisted modular and Ouroboros tools
   deny-all; separately define explicit degradation and reason semantics (J2).
3. **Verified slice plus residual:** core-inventory and provider-parity tests prove
   every default resolves; next carry set identity/version and pin whether session
   selectively loads policies or records membership in the global corpus (J3).
4. **Short term, medium:** generate a schema-to-consumer matrix in tests and choose
   one precedence rule for `ToolLoop` versus `ExecutorConfig` (J4).
5. **Strategic, medium:** add a redacted capability receipt and recovery reason
   codes (J5–J6).
6. **Strategic, large:** evaluate candidate envelopes in a no-effect shadow with
   bounded stored results (J7).

Authoritative implementation contracts and acceptance controls are the four
cards in [TODO.md](TODO.md).
