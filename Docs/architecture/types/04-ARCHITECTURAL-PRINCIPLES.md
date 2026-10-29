# 04 — Architectural Principles: `internal/types`

> Last verified: **2026-07-13**  
> Binding principles for contributors editing this package. Package-specific — not generic “write clean code.”

## P1 — Types package stays cycle-free

`internal/types` must not import `core`, `session`, `shards`, `perception`, `campaign`, `store`, or CLI packages.

If a new type needs those packages to compile, the type is in the wrong place **or** you need another interface/marker.

## P2 — Facts are the only logical currency

Cross-package logical state is `types.Fact` (or a type alias). Do not invent parallel fact structs with different field names.

## P3 — ToAtom never poisons the EDB silently

Unsupported or nil args **error**. No `fmt.Sprintf("%v")` of pointers into the store. Errors must identify predicate and arg index when possible.

## P4 — Name constants are explicit or heuristically safe

Prefer `MangleAtom("/x")` for intentional atoms. Plain strings starting with `/` are names only if they pass the conservative heuristic (not file paths).

## P5 — Interfaces over concrete implementers

Consumers depend on `types.Kernel`, `types.LLMClient`, `types.ShardAgent`, `types.VirtualStore`, not concrete core types — except at composition roots (boot).

## P6 — Optional capabilities via type assertion

Do not grow `LLMClient` with every provider feature. Add a small optional interface + document the assertion pattern.

## P7 — Blackboard is structured, not free-form only

`SessionContext` carries structured sections (intent, campaign, git, TDD, tools, constitutional). Free-form fields exist for compression, but new cross-shard data should prefer typed fields or facts.

## P8 — Transactions for multi-op EDB mutations

Buffered retract/assert with single `Commit` when multiple kernel mutations must be atomic. Implementers **must** support `KernelTransactor`; `NewKernelTx` panics otherwise.

## P9 — Extract helpers for all Fact.Args reads

New code reading fact arguments uses `Extract*` / `Arg*`, not bare type assertions that panic on mismatch.

## P10 — Permissions are labels; policy is Mangle

`ShardPermission` values are capability tags for config. Constitutional enforcement remains `permitted(...)` / VirtualStore — do not implement deny logic in this package.

## P11 — Keep the package small

Foundational structures only. Domain-specific DTOs (campaign task graphs, browser DOM nodes, embedding records) stay in their packages unless they create a real cycle that only a shared interface can break.

## P12 — Aliases are OK; forks are not

`type Fact = types.Fact` in `core`/`world` is fine. Re-defining a different Fact struct is not.
