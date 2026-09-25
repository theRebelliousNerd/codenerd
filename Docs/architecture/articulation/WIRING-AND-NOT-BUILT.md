# articulation wiring — and what is NOT built

What is connected, what exists but nothing calls, and what the design
assumes that the code does not do. What the package is: README.md. How it
works: INTERNALS.md.

Re-verified 2026-09-25 (lane A build-out); first written 2026-09-20 against
`456e5217`. Line numbers below were re-read on 2026-09-25.

## Wired and reachable

| Symbol | Production caller |
|---|---|
| `ApplyConstitutionalOverride` (`emitter.go:850`) | `internal/session/executor.go:1754` — after `core.FilterMangleUpdates` (`:1747`) blocked atoms, marks the surface and drops them |
| `NewEmitter` + `Emit` (`emitter.go:725`, `:737`) | `cmd/nerd/cmd_instruction.go:78,96,370` — one-shot instruction path writes the envelope to stdout |
| `SetJITBudgets` + `EnableJIT` | `cmd/nerd/chat/campaign.go`, `cmd/nerd/chat/campaign_assault.go`, `cmd/nerd/chat/campaign_recurse.go`, `internal/shards/registration.go`, `internal/system/factory.go` |
| `SetJITCompiler` | `internal/shards/registration.go`, `internal/system/factory.go` |
| `NewPromptAssemblerAdapter` | `internal/shards/system/perception.go`, `internal/system/factory.go` |
| `mapToPromptContext` | inside `AssembleSystemPrompt` |
| `PiggybackEnvelopeSchema` | `internal/perception/client_schema.go:23` |
| `NewPromptAssemblerWithJIT` | `cmd/nerd/chat/campaign.go`, `cmd/nerd/chat/campaign_assault.go`, `cmd/nerd/chat/campaign_recurse.go`, `cmd/nerd/cmd_campaign.go` |
| `ProcessLLMResponse` (`emitter.go:1027`) | `internal/session/executor.go`, `internal/shards/system/planner.go` |
| `ProcessLLMResponseAllowPlain` (`emitter.go:1033`) | `internal/session/executor.go`, `internal/session/piggyback_promotion.go`, `internal/shards/requirements_interrogator.go`, `internal/shards/system/legislator.go`, `internal/shards/system/mangle_repair.go`, `cmd/nerd/chat/delegation.go`, `cmd/nerd/chat/helpers.go` |
| `HasSelfCorrection`, `HasMemoryOperations`, `GetMemoryOperationsByType` | `internal/session/executor.go` |
| `NewStreamParser` | `cmd/nerd/chat/helpers_articulation.go` |

**Every path that applies `mangle_updates` to a kernel filters them first.**
The session executor (`executor.go:1747`), the session planner
(`internal/shards/system/planner.go:1081`) and the perception firewall
(`internal/shards/system/perception.go:291`) all go through
`core.FilterMangleUpdates`; the executor additionally marks the surface via
`ApplyConstitutionalOverride`. The display helpers that skip it
(`ExtractSurfaceOnly`, `MustExtractSurface`) have no production caller and
apply nothing to a kernel.

**The envelope's three descriptions are held together by a test.** The JSON
Schema the model is held to (`PiggybackEnvelopeSchema`, `schema.go`), the
structs responses decode into (`protocol_types.go`), and the key set strict
decoding accepts (`schemaAllowedKeys`, `emitter.go:454`) agree at every level
today; `TestEnvelopeShape_SchemaStructAndStrictKeysAgree`
(`envelope_shape_agreement_test.go`) fails, naming the path, when one drifts
(checked by dropping `priority` from `schemaAllowedKeys`).

## Exists but nothing calls it

Each was re-checked with a whole-repo grep on 2026-09-25. No wiring gap was
found for any of them: nothing in production needs what they do and does it
another, worse way. Deleting them is a maintainer call; they are listed in
`scripts/testdata/deadcode-baseline.txt` where noted.

- `Emitter.EmitSurface` (`emitter.go:772`): no callers. No CLI flag asks for
  surface-only output.
- `Emitter.ParseAndProcess` (`emitter.go:780`): no callers.
- `Emitter.CreateEnvelope` (`emitter.go:793`) and `Emitter.MarshalEnvelope`
  (`emitter.go:812`): tests only. `cmd_instruction.go` builds the envelope
  literally and `Emit` marshals it itself.
- `GetKernelContext` / `PromptAssembler.BuildContextSection`
  (`kernel_context.go:20`, `:35`): no production callers (baseline).
- `AssembleQuickPrompt` (`prompt_assembler.go:1160`): tests only (baseline).
- `GetStats` / `ResetStats` (`emitter.go:686`, `:697`): tests only.
- `MustExtractSurface` (`emitter.go:1091`), `ExtractSurfaceOnly`
  (`emitter.go:906`): no production callers (baseline).
- `AppendReasoningDirective` and its two directive constants, listed here on
  2026-09-20, are gone: deleted in `a5e12f0` ("the model's reasoning trace is
  kept whole; the dead directive is gone"). The deadcode baseline still names
  `AppendReasoningDirective` and needs a refresh.

## Assumed by the design, not done by the code

- Strict validation is available but off: `ResponseProcessor.RequireValidJSON`
  (`emitter.go:128`) is false on every production path
  (`processLLMResponse`, `emitter.go:1045`). Declined: a response that does not
  parse falls back to plain text with a nil control packet, so nothing from
  it reaches a kernel; strict mode would turn a model's plain answer into an
  error without a safety gain.
- `ApplyConstitutionalOverride` mutates the envelope in place and returns an
  audit record (`ConstitutionalOverride`), so callers must treat the pointer
  as mutated.
- `GetStats` / `ResetStats` expose in-memory `ProcessorStats` counters; nothing
  persists or exports them. Declined: no consumer wants them.
- `Emit`'s error is ignored at both call sites in `cmd/nerd/cmd_instruction.go`
  (`:96`, `:370`). `json.Marshal` fails only on a non-finite
  `IntentClassification.Confidence`; if it did, `nerd run` would print nothing.
  Open: noted, not changed in this pass.

## Closed in this pass (2026-09-25)

| Item (2026-09-20 wording) | Verdict | Evidence |
|---|---|---|
| Display paths never pass through `ApplyConstitutionalOverride` | stale as a risk: those helpers have no production caller; every kernel-applying path filters | `FilterMangleUpdates` call sites above |
| Three places must agree on the envelope shape, manually | built: an agreement test | `envelope_shape_agreement_test.go` |
| `AppendReasoningDirective` unused | stale: deleted in `a5e12f0` | `grep` finds no definition |
