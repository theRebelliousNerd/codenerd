# `internal/session`

The universal JIT-driven execution loop for codeNERD.

## Runtime boundary

```text
Intent -> JIT prompt/config -> LLM tool proposal
  -> effective capability allowlist
  -> exact pending_action/3 + permitted/3
  -> VirtualStore validation/effect
  -> result loop -> articulation
```

The model is the creative center. Session owns the bounded turn lifecycle. Mangle
is the executive: only an exact `permitted(Action, Target, Payload)` authorizes a
tool call. Prompt text, registration, `safe_action/1`, and a nil/empty capability
config cannot authorize or expose tools.

## Components

- `Executor`: perception/intent, JIT compilation, native and Piggyback tool paths,
  exact safety gate, hollow-success recovery, articulation, and optional turn
  persistence.
- `Spawner`: bounded dynamic SubAgent registry. `Spawn` constructs/registers;
  higher-level helpers decide when execution starts.
- `SubAgent`: isolated task history/state with optional semantic compression.
- `JITExecutor`: TaskExecutor adapter used by Cortex and delegation/campaign paths.
- `SemanticCompressor`: bounded model summary for long-running worker state.

## Invariants

- Missing JIT/config may degrade the prompt but grants no ambient tools.
- Specialist config is path-contained, size-bounded, strictly decoded, and
  validated before spawn.
- Tool calls are bounded by call count, loop count, timeout, capability, exact
  constitutional permission, and VirtualStore validation.
- Native loops receive compact remaining-call/round telemetry. The base round
  limit may receive only bounded deterministic extensions when executed results
  are novel and successful; repeated trace cycles force convergence.
- Shared kernel use requires task/turn identity and cleanup; per-task history is
  cloned/isolated.

## Verify

```powershell
go test ./internal/session ./internal/jit/config
go test -race ./internal/session -run 'Capability|Safety|Spawn|Config'
```

Architecture, wiring, feature cards, and current gaps:
`Docs/architecture/session/README.md`.
