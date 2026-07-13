# system — Alignment / Vision Review

> Last verified: **2026-07-13**  
> Against: codeNERD north star (LLM creative center; Mangle kernel executive; constitutional `permitted`; JIT-first prompts)

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully aligned in code and call sites |
| 4 | Strongly aligned; minor gaps or dual paths |
| 3 | Partial; important wires present but incomplete |
| 2 | Weak; concept present, easy to misuse |
| 1 | Misaligned or absent |

## Dimension scores

| Dimension | Score | Evidence |
|-----------|------:|----------|
| **Inversion of control** | **5** | Boot builds kernel first among executive pieces; LLM clients are injected *into* perception/shards/session, not the other way around. `initKernel` registers policy domain with `permitted` ownership. |
| **Constitutional safety surface** | **4** | Policy kernel shard owns `permitted` / `blocked` / `constitution` / `dangerous_action` (`factory.go` `shardConfigs`). System shards can be disabled via `DisableSystemShards` for tests — intentional escape hatch, not production default. |
| **JIT-first LLM behavior** | **5** | Boot always constructs `prompt.JITPromptCompiler`, wires `articulation.PromptAssembler`, registers project + agent corpora, and builds `session.NewJITExecutor`. Hybrid PROMPT atoms from kernel boot are ingested into corpus DB. |
| **Single wiring authority** | **4** | Package comment: “Motherboard”. Almost all production CLI uses `GetOrBootCortex`. **Exception:** interactive chat uses `BootCortexWithConfig` directly (`cmd/nerd/chat/session_shared_boot.go`), bypassing the keyed cache. |
| **Deterministic assembly** | **4** | Fixed stage order in `BootCortexWithConfig`. Soft-fails for non-critical subsystems (embedding, MCP, taxonomy). Hard-fails on kernel evaluate, JIT compiler, system shard start. |
| **Lifecycle / resource safety** | **3** | `Cortex.Close` stops shards, closes JIT/DB/learning, closes perception layer, evicts cache. `GetOrBootCortex` starts maintenance but **discards** the returned cancel func — maintenance lives until process exit or Close does not stop the ticker (Close does not cancel maintenance). |
| **Cache correctness** | **5** | Keyed by SHA-256 of `workspace \0 provider \0 apiKey \0 model` (Bug #15). Failed boots not cached. Double-checked locking. Close evicts by key. |
| **Adapter honesty** | **3** | Necessary boundary adapters exist (`KernelAdapter`, session/MCP adapters). `sessionVirtualStoreAdapter.ReadFile/WriteFile` use **os fallback**, not VirtualStore `HandleAction` — documented in source as interim. |
| **Observability of boot** | **4** | Category-tagged logs (Perception, Context, Tools, Embedding, Session, Store, World). TUI adds `[boot]` step printing outside this package. |
| **Testability / DI** | **5** | `BootConfig` overrides: `UserConfigOverride`, `LLMClientOverride`, `KernelOverride`, `DisableSystemShards`. Boot tests exercise full and no-LLM paths. |
| **Wiring-before-deletion culture** | **5** | Package *is* the wiring layer. Dead-looking adapters still used by session/MCP/JIT paths. |

### Weighted overall: **~4.3 / 5**

The motherboard strongly realizes the north star. Remaining friction is dual boot entry points (cache vs direct), incomplete lifecycle cancellation for maintenance, and a few adapter shortcuts.

## North-star checklist (package-local)

| North-star rule | system’s job | Status |
|-----------------|--------------|--------|
| LLM = creative center | Wire main / shard / image LLM clients; never make LLM own policy | Done |
| Kernel = executive | Boot `CortexKernel` + domains; attach VirtualStore to kernel | Done |
| `permitted(...)` default deny | Policy domain registered; actual rules live in `core/defaults/policy` | Owned elsewhere; wired here |
| JIT prompt atoms | AtomLoader, corpus materialize, agent DBs, PromptAssembler | Done |
| Audit wiring before delete | This package *is* the audit surface for “how does X get a Kernel?” | Done |

## Anti-alignments (do not claim)

- system does **not** implement constitutional rules itself.  
- system does **not** replace perception or articulation.  
- `GetOrBootCortex` is **not** used by the interactive TUI today.
