# articulation — Architecture Corpus (`internal/articulation`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/articulation/`  
> Role in fact-flow: **LLM output → structured surface + control** (and **kernel/session → system prompts**)

## Scope

This corpus documents codeNERD’s **articulation layer**: the dual-channel Piggyback Protocol (surface text vs control packet), robust LLM response parsing (including stream and salvage paths), JSON Schema for structured output, constitutional surface/atom overrides, and the **PromptAssembler** bridge that turns kernel + session + JIT atoms into shard system prompts.

It is **not** the perception transducer (`Docs/architecture/perception/`), not the JIT atom compiler itself (`Docs/architecture/prompt/`), and not the chat TUI that consumes articulation results (`Docs/architecture/cli/`).

### Fact-flow placement

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools → LLM
  → articulation (parse Piggyback / assemble prompts) → TUI / stdout / kernel assert
```

Articulation is the **corpus callosum** between free-form model text and deterministic kernel state:

| Direction | Mechanism |
|-----------|-----------|
| Outbound (prompt) | `PromptAssembler` + optional JIT (`internal/prompt`) |
| Inbound (response) | `ResponseProcessor` / `ProcessLLMResponse*` / `StreamParser` |
| Safety edge | `applyCaps`, mangle syntax filters, `ApplyConstitutionalOverride` |

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, parse state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and constructors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, chat, session, shards, perception |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Caps, injection defense, concurrency |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, fuzz, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | `CategoryArticulation`, timers, stats |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Backlog and rebuild log |

## Verify commands

```powershell
# Unit + boundary tests for this package
go test ./internal/articulation/...

# Verbose (parse path debugging)
go test ./internal/articulation/... -v -count=1

# Fuzz entrypoint (ResponseProcessor)
go test ./internal/articulation/ -fuzz=FuzzResponseProcessor_Process -fuzztime=10s

# Downstream consumers that depend on Piggyback parsing
go test ./internal/session/ -count=1
go test ./cmd/nerd/chat/ -count=1
```

Package self-doc (may lag this corpus): `internal/articulation/README.md`.

## Quality bar

Modeled on `Docs/architecture/cli/`: real paths, control-flow diagrams, wiring evidence, honest gaps — **not** thin inventory stubs. Every cited path exists under `internal/articulation/` or a verified importer.

## Related corpora

- `Docs/architecture/prompt/` — JIT compiler, atoms, budgets  
- `Docs/architecture/perception/` — NL → intent (mirror of articulation)  
- `Docs/architecture/session/` — clean executor Piggyback++ tool loop  
- `Docs/architecture/cli/` — chat helpers that stream-parse and process envelopes  
- `Docs/architecture/core/` — kernel assert path for `mangle_updates`
