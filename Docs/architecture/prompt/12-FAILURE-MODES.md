# prompt — Failure Modes

> Last verified: **2026-07-13**

## Catalog

### FM1 — Skeleton kernel missing

| | |
|--|--|
| **Symptom** | Compile error: CRITICAL Mangle kernel not configured / skeleton failed |
| **Cause** | `WithKernel` not applied |
| **Mitigation** | Always wire kernel adapter in boot; unit tests use mock KernelQuerier |

### FM2 — Empty skeleton corpus

| | |
|--|--|
| **Symptom** | CRITICAL: no skeleton atoms found |
| **Cause** | Embedded corpus not loaded; all skeleton filtered by structured-output? (unlikely) |
| **Mitigation** | `WithEmbeddedCorpus(LoadEmbeddedCorpus())`; verify embed build |

### FM3 — Mangle selection returns empty skeleton

| | |
|--|--|
| **Symptom** | Zero skeleton atoms; category warnings; unsafe thin prompts |
| **Cause** | Facts/schema mismatch; blocked_by_context too aggressive; Decl missing |
| **Mitigation** | Check `jit_compiler.mg`; debug query blocked/mandatory; assert fact order |

### FM4 — Stale compile_context

| | |
|--|--|
| **Symptom** | Wrong atoms for subsequent compiles |
| **Cause** | Kernel without Retract; assert without cleanup |
| **Mitigation** | KernelRetracter path; e2e `jit_kernel_context_cleanup` |

### FM5 — Cache stale prompt

| | |
|--|--|
| **Symptom** | Tool nudge / available tools change ignored |
| **Cause** | `Hash()` omits dimensions that changed |
| **Mitigation** | Expand Hash; or ClearCache on those transitions |

### FM6 — Vector timeout / no embeddings

| | |
|--|--|
| **Symptom** | Flesh thin; domain atoms missing |
| **Cause** | Timeout; empty project DB embeddings; no SemanticQuery |
| **Mitigation** | Degrade is OK; SyncEmbeddedToSQLite; raise timeout carefully |

### FM7 — Budget saturation

| | |
|--|--|
| **Symptom** | Warnings: mandatory skipped; low-priority categories empty |
| **Cause** | TokenBudget too small vs skeleton; huge mangle encyclopedias selected |
| **Mitigation** | Raise budget; mangle mandatory caps; polymorphic content; tighter selectors |

### FM8 — Dependency cycle

| | |
|--|--|
| **Symptom** | Warn: cycle broken at atom X; odd order |
| **Cause** | Mutual DependsOn in YAML |
| **Mitigation** | DetectCycles offline; fix atom graph |

### FM9 — ConfigFactory no atom

| | |
|--|--|
| **Symptom** | Warn failed generate; tools unset |
| **Cause** | Unknown intent verb |
| **Mitigation** | Register ConfigAtom; consult→general fallback; GenerateFallback |

### FM10 — Tool name mismatch

| | |
|--|--|
| **Symptom** | Agent thinks tool allowed; executor rejects / VirtualStore miss |
| **Cause** | ConfigAtom string ≠ registered tool |
| **Mitigation** | Single source of tool names; integration tests |

### FM11 — Project/agent DB load failure

| | |
|--|--|
| **Symptom** | Warn load failed; fewer candidates |
| **Cause** | Schema mismatch, locked DB, path wrong |
| **Mitigation** | EnsureSchema; non-fatal continue with embedded |

### FM12 — Knowledge/learning bridge hang

| | |
|--|--|
| **Symptom** | Slow compiles |
| **Cause** | Embed search without timeout honor |
| **Mitigation** | KnowledgeSearchTimeout; non-fatal collect paths |

### FM13 — Concurrent DB mutation during compile

| | |
|--|--|
| **Symptom** | Flaky missing atoms |
| **Cause** | Register/unregister mid-collect without snapshot discipline |
| **Mitigation** | Existing RLock snapshot pattern; avoid long holds |

### FM14 — Baseline vs JIT divergence

| | |
|--|--|
| **Symptom** | Different behavior when JIT off |
| **Cause** | Baseline only mandatory matching context |
| **Mitigation** | Prefer JIT path in production; document baseline limits |

## Severity summary

| ID | Severity | Degrade-safe? |
|----|----------|---------------|
| FM1–FM3 | High | No (skeleton) |
| FM4–FM5 | High | Silent wrong |
| FM6 | Medium | Yes |
| FM7 | Medium | Partial |
| FM8 | Low–Med | Ordered somehow |
| FM9–FM10 | High for tools | Prompt may still work |
| FM11–FM14 | Low–Med | Usually yes |

## Incident triage order

1. Log line `JIT[...]` stats string.  
2. Manifest dropped reasons / DebugMode.  
3. Confirm kernel + retract.  
4. Confirm embedded count at boot.  
5. Check Hash-related state changes.  
6. Check ConfigAtom for intent.  
7. Mangle policy Decl/query.
