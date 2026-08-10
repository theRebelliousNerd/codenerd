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

### FM4 — Unscoped third-party kernel adapter

| | |
|--|--|
| **Symptom** | Wrong atoms for subsequent or concurrent compiles outside the production adapter |
| **Cause** | External `KernelQuerier` implements neither `KernelScopeProvider` nor safe lifecycle cleanup |
| **Mitigation** | `VERIFIED CURRENT` for production: cloned `RealKernel` compilation scopes plus mixed-context race tests. Require equivalent scope support or fail closed for third-party adapters. |

### FM5 — Cache stale prompt

| | |
|--|--|
| **Symptom** | Tool nudge / available tools change ignored |
| **Cause** | A new prompt-affecting context field is added without versioning/hash coverage |
| **Mitigation** | `VERIFIED CURRENT`: `compilation-context-v2` hashes all live fields and canonicalizes sets; keep field-completeness and real retry regressions mandatory |

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

### FM15 — Compilation-scope regression

| | |
|--|--|
| **Symptom** | Language, intent, shard, candidate, or vector state from another turn influences selection |
| **Cause** | A production adapter stops cloning/sandboxing, or selection accidentally uses the live kernel instead of the compilation scope |
| **Mitigation** | `VERIFIED CURRENT`: `acquireCompilationKernel` plus production `KernelAdapter.NewCompilationScope`; focused success/error/cancel/concurrent/panic regressions |

### FM16 — Atom contract regression

| | |
|--|--|
| **Symptom** | Validator/filesystem/embedded routes disagree, or a built-in atom requires migration |
| **Cause** | A new route bypasses `ParsePromptAtomYAML`, or schema/vocabulary changes without parity update |
| **Mitigation** | `VERIFIED CURRENT`: shared strict parser, canonical built-ins, bounded migration warnings, and golden ordered 888-ID parity |

### FM17 — Canonical stale atom precedence (stale corpus.db built-ins shadow canonical)

| | |
|--|--|
| **Symptom** | Runtime: FIXED in 1ad8238e — embedded canonical is now collected first and corpus.db copies are deduplicated during prompt collection, so stale database copies no longer shadow embedded atoms; boot DB still OPEN — corpus.db still retains 878 stale built-in rows diverging from the 888-ID canonical corpus until reconciled/removed |
| **Cause** | Validator, filesystem, and embedded loaders check count/order/digest (888 IDs) but boot synchronizer does not yet enforce embedded authority with DB reconciliation/removal, so stale corpus.db built-in rows persist on disk even though runtime collection already deduplicates |
| **Mitigation** | Runtime FIXED (1ad8238e): treat embedded corpus as authority for any built-in atom ID during collection (first-source). Boot still OPEN: synchronizer must reconcile/replace and remove stale corpus.db built-in rows to match embedded content and never drop project-only (non-built-in) atoms; preserve count/order/digest parity after DB reconciliation |
| **Acceptance exam** | Negative acceptance exam seeds a temporary corpus.db with 878 stale built-in records diverging from the current embedded 888-ID corpus plus one project-only atom present only in corpus.db, boots the synchronizer, and asserts (a) stale built-ins are reconciled to the canonical embedded content (or removed, with runtime falling back to embedded) and (b) the project-only atom survives reconciliation |

### FM18 — Shell-effect task integrity bypass (run_command/bash mutate without detection)

| | |
|--|--|
| **Symptom** | run_command/bash shell effects mutate the workspace (e.g., git checkout of dirty tracked work, rm -rf of untracked directory) while being classified as non-write tools; pre-delegation world scan was fresh and exposed dirty state — cleanup was allowed because ownership baseline and shell scope enforcement were absent; world became stale only after unreported shell mutations because no incremental refresh ran |
| **Cause** | No immutable pre-task ownership baseline (tracked vs untracked, dirty vs clean, ownership by task); shell effects not detected, attributed, or scope-checked before success verdict; accepted mutations not wired to incremental world retraction/reassertion so world becomes stale only after unreported mutations |
| **Mitigation** | Capture immutable pre-task ownership baseline at task start; detect, attribute, and scope-check every run_command/bash shell effect and fail closed before any success verdict; never revert pre-existing dirty tracked work nor delete pre-existing untracked paths; wire accepted mutations to incremental world retraction and reassertion so kernel world reflects post-shell reality and does not go stale |
| **Acceptance exam** | Two negative acceptance exams in temporary repos: (a) git checkout of dirty tracked work — task must not revert the pre-existing dirty tracked file to HEAD; (b) recursive deletion of an untracked directory (e.g., rm -rf of an untracked folder) — task must not delete the pre-existing untracked directory. Both must reproduce the violation fixture and pass only when scope-checked shell detection and baseline preservation are implemented without losing either artifact |
## Severity summary

| ID | Severity | Degrade-safe? |
|----|----------|---------------|
| FM1–FM3 | High | No (skeleton) |
| FM4–FM5 | High | Production guarded; external/new-field regressions remain possible |
| FM6 | Medium | Yes |
| FM7 | Medium | Partial |
| FM8 | Low–Med | Ordered somehow |
| FM9–FM10 | High for tools | Prompt may still work |
| FM11–FM14 | Low–Med | Usually yes |
| FM15–FM16 | High | Guarded by focused production and ordered-parity gates |
| FM17 | Critical — runtime FIXED (1ad8238e), boot DB reconciliation/removal OPEN | Runtime degrade-safe (embedded first, DB deduplicated); boot DB still retains stale rows until reconciled |
| FM18 | Critical | No — shell effects bypass task-integrity/world-policy until detection wired (pre-delegation scan was fresh; staleness only post-mutation) |

## Incident triage order

1. Log line `JIT[...]` stats string.
2. Manifest dropped reasons / DebugMode.
3. Confirm production compilation scope creation/close; inspect external adapter capability.
4. Confirm embedded count at boot.
5. Check Hash-related state changes.
6. Check ConfigAtom for intent.
7. Mangle policy Decl/query.
8. [2026-08-09 incident] Verify runtime precedence fixed in 1ad8238e (embedded first, DB deduplicated); verify boot DB reconciliation still OPEN — confirm DB stale built-ins reconciled/removed and project-only atoms preserved — check synchronizer DB path.
9. [2026-08-09 incident] Verify task-integrity: pre-delegation scan was fresh, so confirm immutable pre-task baseline was absent; verify run_command/bash shell-effect detection/attribution/scope-check before any success verdict, no revert of pre-existing dirty tracked work, no delete of pre-existing untracked paths, and incremental world retraction/reassertion so world stale only after unreported mutations.

## 2026-08-09 task-integrity incident — true-up note (current reality — truth-corrected)

This document records current reality as of 2026-08-09, truth-corrected for
commit 1ad8238e. FM17 runtime canonical first-source precedence is fixed
(embedded collected first, DB deduplicated — stale database copies no longer
shadow embedded atoms during prompt collection); FM17 boot database
reconciliation and stale built-in removal remain OPEN and project-only atoms
must be preserved. FM18 pre-delegation world scan was fresh and exposed dirty
state; cleanup was allowed because ownership baseline and shell scope
enforcement were absent; world became stale only after unreported shell
mutations because no incremental refresh ran. Required contracts and negative
acceptance exams are pinned above; verification of those exams is tracked in
the gap analyses (prompt 03-GAP-ANALYSIS G9/G10, session 03-GAP-ANALYSIS,
world 03-GAP-ANALYSIS). Do not mark FM17/FM18 closed until the seeded temp-DB
and temp-repo exams exist and pass without losing the dirty tracked file or the
untracked directory.
