# mangle — Go engine, grammar, and learner loop

The Go package this repository keeps under the name `internal/mangle`.
It is the deterministic half of the neuro-symbolic loop: it parses Mangle source,
evaluates rules over an in-memory fact store, validates candidate rules, and
repairs model-generated atoms. The LLM proposes; this package disposes.

Verified 2026-09-21 against commit `0f7253b`.

## Layout

| Path | What it is |
|---|---|
| `internal/mangle/engine.go` (1,410) | `Engine`: fact store, evaluator with gas limit, `Query`, persistence hooks |
| `internal/mangle/lsp.go` (1,019) | `LSPServer`: regex-based `.mg` indexer; completion, hover, definition, diagnostics |
| `internal/mangle/schema_validator.go` (598) | `SchemaValidator`: Decl-gated validation for hand-written and learned rules |
| `internal/mangle/grammar.go` (794) | `AtomValidator` + `RepairLoop`: atom shape/type checks and model-prompt repair |
| `internal/mangle/proof_tree.go` (502) | `ProofTreeTracer`: derivation traces with ASCII/JSON rendering |
| `internal/mangle/parse_lock.go` (71) | Serializes parser entry; the upstream parser is not goroutine-safe |
| `internal/mangle/simd.go` (29) | Build-tagged SIMD probe; scalar fallback is the real path |
| `internal/mangle/synth/` (5 source files) | `CompileSpec`: pattern IR → Mangle text, with sanity + semantic gates |
| `internal/mangle/feedback/` (6 source files) | `FeedbackLoop`: LLM rule generation with validation and repair |
| `internal/mangle/transpiler/` (1 source file) | `sanitizer.go`: model-output scrubber for pasted Mangle |

Sizes: Go: 7 files / ~4,400 lines. AI-plumbing Go helpers (3 sub-packages / 12 source files).
The `.mg` corpus the engine loads lives outside the package, under
`internal/core/defaults/`; the runtime overlay lives at `.nerd/mangle/learned.mg`.

## The one-paragraph model

`Engine` holds base facts plus derived facts and answers `Query` calls against
both (`engine.go:869-1016`). Nothing evaluates until something inserts facts or
calls `Evaluate` (`engine.go:550-558`) — there is no background fixpoint.
`SchemaValidator` decides whether a rule may join the program: hand-written
rules need declared predicates of matching arity; learned rules additionally
may not define protected control-plane predicates (`schema_validator.go:164-208`).
When the model emits a rule, `feedback/` generates, normalizes, validates, and
repairs it (`feedback/loop.go:GenerateAndValidate`); when the model emits a
fact-shaped atom, `grammar.go` type-checks it and, on failure, builds the
repair prompt it sends back (`grammar.go:769-793`). The LSP server never talks
to the engine — it indexes `.mg` text with regexes (`lsp.go:162-221`).

## Who uses it

Imported by `internal/core`, `internal/world/lsp`, `internal/autopoiesis`,
`internal/browser`, `internal/context`, and `cmd/nerd`. The kernel
(`internal/core/kernel_policy.go`) is the primary consumer: rule validation,
sandbox compilation, learned-rule persistence, and LLM-driven rule generation
all run through it. The editor path (`internal/world/lsp/manager.go:117-128`)
builds an engine and an `LSPServer` and indexes the workspace once at startup.
`nerd why` (`cmd/nerd/cmd_query.go:308-310`) is the only production caller of
the proof-tree tracer.

## Guarantees and non-guarantees

- Every numeric Mangle slot is `int64`; one `float64` fact aborts the kernel
  fixpoint. Every predicate needs a `Decl` before use, and no predicate may be
  declared twice at the same arity — a duplicate takes the kernel down at boot.
  Both invariants are enforced by the fork under `engine.go`, not by convention.
- The engine itself does no I/O: no sockets, no subprocesses, no filesystem.
  Persistence is an injected interface (`engine.go:145-149`); the only model
  call in the package is the injected `LLMClient.Complete`
  (`feedback/loop.go:199`).
- `Query` requires the queried predicate to have a `Decl` and errors otherwise
  (`engine.go:884-888`); it merges stored rows with fresh derivations, dedups,
  sorts, and gives up after a 5s default timeout (`engine.go:904-917,961-999`).
- The parser Fork accepts tab-indented continuation blocks; the sanitizer
  exists because model output routinely violates this and other atom rules.

## Where the details live

- `INTERNALS.md` — engine lifecycle, validation gates, learner loop, LSP indexing.
- `WIRING-AND-NOT-BUILT.md` — what is wired and reachable, what exists but
  nothing calls, and what the design assumes that the code does not do.
