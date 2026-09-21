# articulation wiring — and what is NOT built

What is connected, what exists but nothing calls, and what the design
assumes that the code does not do. What the package is: README.md. How it
works: INTERNALS.md.

## Wired and reachable

| Symbol | Production caller |
|---|---|
| `ApplyConstitutionalOverride` (`emitter.go:854-902`) | `internal/session/executor.go:1740` — filters blocked mangle atoms after parsing |
| `NewEmitter` + `Emit` (`emitter.go:729-738`, `741-773`) | `cmd/nerd/cmd_instruction.go:78,96,370` — one-shot instruction path writes the envelope to stdout |
| `SetJITBudgets` + `EnableJIT` (`prompt_assembler.go:1255-1281`, `1216-1225`) | `cmd/nerd/chat/campaign.go:65-66`, `cmd/nerd/chat/campaign_assault.go:56-57`, `cmd/nerd/chat/campaign_recurse.go:162-163`, `internal/shards/registration.go:493-495`, `internal/system/factory.go:1636-1638` |
| `SetJITCompiler` (`prompt_assembler.go:1229-1238`) | `internal/shards/registration.go:493`, `internal/system/factory.go:1636` |
| `NewPromptAssemblerAdapter` (`prompt_assembler_adapter.go:22-24`) | `internal/shards/system/perception.go:182,257`, `internal/system/factory.go:1640,1850` |
| `mapToPromptContext` (`prompt_assembler_adapter.go:51-137`) | `prompt_assembler.go:381`, inside `AssembleSystemPrompt` |
| `PiggybackEnvelopeSchema` | `internal/perception/client_schema.go:23` |
| `NewPromptAssemblerWithJIT` (`prompt_assembler.go:81`) | `cmd/nerd/chat/campaign.go:60`, `cmd/nerd/chat/campaign_assault.go:51`, `cmd/nerd/chat/campaign_recurse.go:157`, `cmd/nerd/cmd_campaign.go:259` |
| `ProcessLLMResponse` (`emitter.go:1072`) | `internal/session/executor.go:1517,1570`, `internal/shards/system/planner.go:320` |
| `ProcessLLMResponseAllowPlain` (`emitter.go:1078`) | `internal/session/executor.go:2315`, `internal/session/piggyback_promotion.go:33`, `internal/shards/requirements_interrogator.go:138`, `internal/shards/system/legislator.go:89`, `internal/shards/system/mangle_repair.go:351`, `cmd/nerd/chat/delegation.go:475`, `cmd/nerd/chat/helpers.go:466` |
| `HasSelfCorrection` (`emitter.go:972-979`), `HasMemoryOperations` (`emitter.go:982-988`), `GetMemoryOperationsByType` (`emitter.go:991-1001`) | `internal/session/executor.go:2331,2347,2353` |
| `NewStreamParser` (`stream_parser.go:38`) | `cmd/nerd/chat/helpers_articulation.go:301` |

## Exists but nothing calls it

Each item below was checked with a whole-repo grep whose result set was
not truncated; the only hits are the definition, package tests, and the deadcode baseline.

- `Emitter.EmitSurface` (`emitter.go:776-781`): no callers anywhere.
- `Emitter.ParseAndProcess` (`emitter.go:784-794`): no callers anywhere.
- `Emitter.CreateEnvelope` (`emitter.go:797-813`) and
  `Emitter.MarshalEnvelope` (`emitter.go:816-838`): called only from
  `internal/articulation/emitter_helpers_test.go:13,21`. The production
  one-shot path builds `PiggybackEnvelope` literally
  (`cmd/nerd/cmd_instruction.go:96`) instead of using them.
- `AppendReasoningDirective` (`emitter.go:936-943`) with
  `ReasoningTraceDirective` (`emitter.go:912`) and
  `ShardReasoningDirective` (`emitter.go:930`): called only from
  `internal/articulation/emitter_extra_test.go:74-82`, and listed in
  `scripts/testdata/deadcode-baseline.txt:144`. No shard prompt is built
  through them in production.
- `GetKernelContext` (`kernel_context.go:20-31`): no production callers;
  listed in `scripts/testdata/deadcode-baseline.txt:147`. Its body delegates
  to `BuildContextSection` (`kernel_context.go:30`), which likewise has no
  production callers.
- `AssembleQuickPrompt` (`prompt_assembler.go:1160`): called only from
  `internal/articulation/prompt_assembler_test.go:386`; listed in
  `scripts/testdata/deadcode-baseline.txt:148`.
- `GetStats` / `ResetStats` (`emitter.go:690-706`): read only from
  `internal/articulation/emitter_test.go:119,124`; no production use.
- `MustExtractSurface` (`emitter.go:1136`): no callers at all; listed in
  `scripts/testdata/deadcode-baseline.txt:146`.
- `ExtractSurfaceOnly` (`emitter.go:951`): called only from
  `internal/articulation/emitter_helpers_test.go:56-63`; listed in
  `scripts/testdata/deadcode-baseline.txt:145`.

## Assumed by the design, not done by the code

- Strict validation is available but unwired: `ResponseProcessor` carries
  `RequireValidJSON` (`emitter.go:126-135`), yet the package's own LLM
  entries go through `processLLMResponse`, which hardcodes it to false
  (`emitter.go:1090`). Nothing on this path sets it to true.
- `ApplyConstitutionalOverride` mutates the envelope in place and returns
  an audit record (`ConstitutionalOverride`, `emitter.go:845-850`), so
  callers must treat the pointer as mutated. It runs only on the
  session-executor path (`executor.go:1740`): it has no other production
  caller, so display paths built on `ExtractSurfaceOnly` /
  `MustExtractSurface` never pass through it.
- `GetStats` / `ResetStats` (`emitter.go:690-706`) expose the in-memory
  `ProcessorStats` counters (`emitter.go:138-144`); nothing persists or
  exports them.
- Three places must agree on the envelope shape — the schema text from
  `GetPiggybackSchema` (`schema.go:235-240`), the allowed-keys list
  (`schemaAllowedKeys`, `emitter.go:458-499`), and the `UnmarshalJSON`
  methods (`protocol_types.go:49-78,100-138,166-205`) — and keeping them
  in agreement is manual.

Verified: 2026-09-20 · branch `main`, working tree clean · commit `456e5217`.
