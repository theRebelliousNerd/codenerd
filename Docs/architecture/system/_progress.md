# system — Corpus Progress

## 2026-07-13 — Full rebuild (SUBAGENT_INSTRUCTIONS)

- **Mode:** DOCS ONLY under `Docs/architecture/system/`
- **Source researched:** `internal/system/` (5 non-test Go, 11 tests, crash dump `.mg` artifact)
- **Flagship:** `IMPLEMENTED_SPEC.md` — GetOrBootCortex, boot stages, Cortex surface, wiring
- **Doc set:** README, IMPLEMENTED_SPEC, 00–12, TODO, OPEN-QUESTIONS, _progress
- **Prior thin stubs:** superseded by new filenames (vision / current-state / public API / wiring / observability per rebuild contract)
- **No code changes** under `internal/`, `cmd/`, or elsewhere

### Research checklist

- [x] Listed package files  
- [x] Read factory.go, factory_adapters.go, agent_registry.go, holographic_code_scope.go, cortex_close.go  
- [x] Grepped exports / GetOrBootCortex reverse deps  
- [x] Mapped fact-flow position  
- [x] Noted honest gaps (maintenance cancel, TUI dual boot, VS os fallback)

### Verify (docs)

Paths cited under `internal/system/` and `cmd/nerd/` exist as of 2026-07-13.
