# system — Failure Modes

> Last verified: **2026-07-13**

## FM1 — Stale Cortex after config switch (Bug #15 class)

| | |
|--|--|
| **Symptom** | Commands use wrong provider/model/workspace after mid-process change |
| **Cause** | Historical unkeyed singleton; residual risk if provider/model not re-read into cache key |
| **Mitigation** | Keyed cache by workspace+provider+apiKey+model; `ResetCortexForWorkspace` |
| **Residual** | Reset not wired from auth/config UI; TUI bypasses cache entirely |

## FM2 — Failed boot poison (prevented)

| | |
|--|--|
| **Symptom** | (Would be) permanent failure for a key after one transient error |
| **Cause** | Caching nil/error entries |
| **Mitigation** | GetOrBootCortex does not insert on error |
| **Tests** | Missing explicit regression test |

## FM3 — Kernel Evaluate failure

| | |
|--|--|
| **Symptom** | Boot returns error; no Cortex |
| **Cause** | Invalid Mangle program / domain registration failure |
| **Mitigation** | Hard fail; operator fixes policy/schema; crash dumps may write `debug_program_ERROR.mg` |
| **Soft?** | No |

## FM4 — JIT / embedded corpus failure

| | |
|--|--|
| **Symptom** | Boot error: failed to load embedded corpus / init JIT |
| **Cause** | Binary missing embedded assets; compiler option misconfig |
| **Mitigation** | Hard fail — incomplete Cortex without JIT is not product-complete |

## FM5 — System shards fail to start

| | |
|--|--|
| **Symptom** | `failed to start system shards` |
| **Cause** | Shard factory panic, missing deps, resource limits |
| **Mitigation** | Hard fail; tests disable shards via BootConfig |

## FM6 — Missing LLM credentials

| | |
|--|--|
| **Symptom** | Boot OK; any Complete* fails with “no LLM client configured” |
| **Cause** | No config/env/apiKey |
| **Mitigation** | `missingLLMClient`; non-LLM commands still work |
| **Soft?** | Yes for boot |

## FM7 — Embedding unavailable

| | |
|--|--|
| **Symptom** | Warnings; semantic search / vector atom selection degraded |
| **Cause** | Health check fail; NewEngine error |
| **Mitigation** | Continue with nil engine; AtomLoader without vectors |

## FM8 — LocalDB open failure

| | |
|--|--|
| **Symptom** | No knowledge.db; no tracing wrapper; maintenance skipped; world facts fall back to scan.mg |
| **Cause** | Permissions, disk, sqlite driver |
| **Mitigation** | Soft continue; holographic uses memCache path |

## FM9 — MCP bridge failures

| | |
|--|--|
| **Symptom** | Tools for MCP servers unavailable |
| **Cause** | NewMCPIntegrationBridge error; ConnectAll error async |
| **Mitigation** | Warn logs; boot continues |

## FM10 — Maintenance vs Close race

| | |
|--|--|
| **Symptom** | Potential panic in `runMaintenance` if LocalDB nilled by Close |
| **Cause** | Cancel func discarded; no nil guard in runMaintenance |
| **Mitigation** | **Needed** — store cancel; guard LocalDB |

## FM11 — Double Cortex in CLI+TUI process

| | |
|--|--|
| **Symptom** | High memory; two system shard sets; conflicting file locks |
| **Cause** | GetOrBootCortex cache + separate BootCortexWithConfig TUI boot |
| **Mitigation** | Unify entry points |

## FM12 — Reset without Close

| | |
|--|--|
| **Symptom** | SQLite locks / leaked goroutines after ResetGlobalCortex |
| **Cause** | Reset only deletes map entries |
| **Mitigation** | Document; prefer Close then Reset; tests must Close |

## FM13 — session file I/O policy bypass

| | |
|--|--|
| **Symptom** | Session path writes files without VirtualStore policy path |
| **Cause** | `sessionVirtualStoreAdapter` os fallback |
| **Mitigation** | Route through VS when non-nil |

## FM14 — Hybrid prompt store partial failure

| | |
|--|--|
| **Symptom** | Some hybrid atoms missing from corpus |
| **Cause** | Per-atom StoreAtom errors logged and skipped |
| **Mitigation** | Soft; count returned may be < input |

## FM15 — Image LLM / worker LLM mis-route

| | |
|--|--|
| **Symptom** | Image gen hits Ollama or shards hit main unexpectedly |
| **Cause** | Config paths for NewWorkerClient / NewImageClient |
| **Mitigation** | Explicit separate clients in initPerceptionLayer; image never uses worker |

## Quick triage table

| Operator observation | Start at |
|----------------------|----------|
| “boot failed cortex kernel” | FM3, policy .mg |
| “failed to init JIT” | FM4 |
| “system shards” | FM5 / Disable list |
| Commands work but chat “forgets” model | FM1 / FM11 |
| Windows test TempDir | FM12 / Close missing |
| Deep facts missing | FM8 + holographic LocalDB path |
