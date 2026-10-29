# Docs/architecture — Index

> One line per directory directly under `Docs/architecture/`, derived from what is actually on disk. Each entry links to its corpus `README.md` (or directory) and summarizes the scope stated there.

- [articulation/](articulation/) — Piggyback envelope & prompt assembly: separates `surface_response` from typed `control_packet` (caps/filters) and assembles outbound prompts.
- [autopoiesis/](autopoiesis/) — self-creation: campaign/tool/agent need detection, Ouroboros runtime tool generation & hardening, learning and prompt-evolution (SPL).
- [browser/](browser/) — Browser Physics: Rod/Chrome session lifecycle, DOM/React reification to Mangle facts, CDP events, honeypot detection.
- [build/](build/) — Go toolchain env builder: filtered `[]string` env for `go build/test` with CGO `sqlite_headers`, `GOCACHE`, `GOOS/GOARCH` consistency.
- [campaign/](campaign/) — multi-phase campaign orchestration: goal decomposition, phase/task execution, context paging, checkpoints, adaptive replan, write-set locking, assaults.
- [cli/](cli/) — human control surface (`cmd/nerd`): Cobra commands, Bubble Tea interactive chat, workspace/cancellation/boot orchestration.
- [config/](config/) — workspace control plane: configuration loading, validation, and workspace selection.
- [context/](context/) — semantic compression & spreading activation: 9-component scoring/ranking for infinite-context sessions.
- [core/](core/) — executive engine: `RealKernel`, `VirtualStore`, Dreamer counterfactuals, constitution & schemas, `permitted/3` default-deny.
- [diff/](diff/) — text-diff utility wrapper.
- [embedding/](embedding/) — trustworthy semantic transduction: embedding engine, vector integration for prompt/store/perception/MCP/campaign/cli.
- [features/](features/) — leaf feature-toggle registry: flags (DifferentialEngine, FlightRecorder, …) without importing `config`.
- [init/](init/) — cold-start workspace init: `.nerd/` creation, project scan, `ProjectProfile`, Mangle profile facts, specialist KB seeding, agent registration.
- [jit/](jit/) — capability envelope for freshly configured agents (`internal/jit/config`) — prompt/session/system capability wiring.
- [logging/](logging/) — config-driven, categorized, file-based diagnostic logging.
- [mangle/](mangle/) — executable boundary: compiler/parse lock, deductive DB adapter, differential engine, validation/synthesis/proof/LSP/parse-safety.
- [mcp/](mcp/) — MCP client stack + JIT Tool Compiler: HTTP/stdio/SSE servers, tool discovery/analysis, hybrid Mangle+vector selection, VirtualStore wiring.
- [northstar/](northstar/) — Northstar Guardian: vision storage, LLM alignment checks, drift/observation persistence, vision predicates.
- [observability/](observability/) — process-level runtime observability leaf package.
- [perception/](perception/) — turning a request into grounded intent: perception transducer → `user_intent` facts.
- [persist/](persist/) — fact-snapshot persistence (`factsnap`: 1 Go file, 4 tests) for durable EDB snapshots.
- [projectdoc/](projectdoc/) — `nerd.md` project-instruction subsystem: strict frontmatter → kernel facts + advisory body → prompt, write-protection gate.
- [prompt/](prompt/) — Prompt JIT: 333 YAML / 888 atoms → Mangle skeleton + semantic flesh → dependency order → budget → assembled prompt + manifest.
- [regression/](regression/) — regression corpus: code-grounded regression harness.
- [retrieval/](retrieval/) — issue-driven sparse file discovery & tiered context assembly: keyword extraction, parallel scan, hit ranking.
- [session/](session/) — universal execution loop: JIT compile → model proposal → `AllowedTools` + exact `permitted/3` → VirtualStore preflight/execute → articulation.
- [shards/](shards/) — specialists around the logic-governed action pipeline: shard lifecycle, delegation, contracts.
- [sqlpragmas/](sqlpragmas/) — SQLite pragma helpers (~125 lines).
- [store/](store/) — multi-tier durable memory (SQLite + optional sqlite-vec): vector/graph/cold/session/world/atoms/traces/learnings/tools/corpora.
- [system/](system/) — motherboard: assembles kernel, stores, VirtualStore, JIT compiler, shards, executors, browser into one `Cortex`.
- [tactile/](tactile/) — motor boundary: bounded effect adapters behind constitutional permission.
- [testing/](testing/) — Context Test Harness: turn compression → fact storage → spreading-activation retrieval → checkpoint validation simulations.
- [tools/](tools/) — tool registry & journal: 25 Go files, tool I/O in `.nerd/tools.db`, Ouroboros wiring.
- [transparency/](transparency/) — operator transparency: Glass Box telemetry, tool events, shard-phase observation, safety explanations, derivation explainers.
- [types/](types/) — foundational shared contracts: fact/kernel/LLM/shard/session types to break import cycles.
- [usage/](usage/) — token/cost usage tracking (2 Go files).
- [ux/](ux/) — UX state & progressive guidance storage.
- [verification/](verification/) — verification gate: ~820-line verifier with 3 test files.
- [world/](world/) — world model: filesystem topology + AST/holographic/CodeDOM projection into Mangle EDB facts.

## Scaffolding (not a corpus)

- [_rebuild/](_rebuild/) — deep-rebuild scaffolding: `SUPERSTAR_CORPUS_STANDARD.md`, subagent instructions, legacy migration ledger.

## Related

- Full realized-corpus table: [INDEX.md](INDEX.md)
- Corpus authoring rules: [AGENTS.md](AGENTS.md) and [_rebuild/SUPERSTAR_CORPUS_STANDARD.md](_rebuild/SUPERSTAR_CORPUS_STANDARD.md)
- Machine ownership: [portfolio.toml](portfolio.toml) + each `corpus.toml`
