# world — Gap Analysis

> Last verified: **2026-08-09** — truth-corrected for commit 1ad8238e: runtime canonical first-source precedence is fixed (embedded first, DB deduplicated) so stale database copies no longer shadow embedded atoms during prompt collection; boot database reconciliation and stale built-in removal remain OPEN and project-only atoms must be preserved. Pre-delegation world scan was fresh and exposed dirty state; cleanup was allowed because ownership baseline and shell scope enforcement were absent; world became stale only after unreported shell mutations because no incremental refresh ran. Coupled to prompt/03-GAP-ANALYSIS G9/G10, prompt/12-FAILURE-MODES FM17/FM18, session/03-GAP-ANALYSIS §7, session/09-SAFETY-AND-INVARIANTS I18–I21. New OPEN incident gaps added; do not mark closed until the seeded temp-corpus.db and both temp-repo negative exams exist and pass.

> Spec/vision vs `internal/world` reality.

## Matrix — vision / schema intent vs reality

| Area | Vision / schema intent | Reality | Gap? |
|------|------------------------|---------|------|
| Portable `file_topology` identity | Workspace-relative everywhere | Full scan uses `canonicalScanPath`; incremental often stores walk absolute paths | **Yes — High** |
| Deep holographic for all supported langs | Cartographer multi-lang | `MapFile` only deep-maps `.go`; multi-lang dataflow exists off main entry | **Yes — High** |
| `WorldPredicates` complete replace | Safe full replace of world EDB | List omits `entry_point`, `project_language`, CodeDOM preds, git, scope preds | **Yes — Medium** |
| `dependency_link` | Schema Decl + WorldPredicates | Sparse emission in scan path | **Yes — Medium** |
| Multi-language LSP | README extensibility | Only Mangle LSP manager | **Yes — Medium** |
| Dual ingestor (shard vs chat scan) | Single world truth | Two systems can both write facts | **Yes — Medium** |
| Incremental entry_point/project_language | Detected on full | Mostly full-scan path | **Yes — Low** |
| Holographic as JIT atoms | North-star JIT for LLM text | Go formatters build prompt strings | **Yes — Low** (design choice) |
| Full CFG dataflow | Sound analysis | Explicit scope-range heuristics | **No** — intentional |
| Constitutional gating of scan | Default deny actions | Scan is local read; not an “action” in policy sense | **No** |
| CodeDOM polyglot factory | Unified elements | DefaultParserFactory registers go/py/ts/rs/mg | **No** |
| Nano mtime invalidation | No same-second stale cache | Implemented FileCache + fingerprints | **No** |
| Deep fact cache in LocalStore | Reuse Cartographer results | `EnsureDeepFacts` depth=`deep` | **No** |
| Core import cycle avoidance | types + system bridge | `types` aliases + `HolographicCodeScope` | **No** |
| Canonical atom precedence at boot (2026-08-09) | Embedded corpus is authority for built-ins | Runtime FIXED in 1ad8238e (embedded collected first, DB deduplicated — no stale shadow at runtime); boot DB still OPEN — 878 stale corpus.db rows retained until DB reconciliation/removal; project-only atoms must be preserved | **Partial — boot DB Critical OPEN (task-integrity)** |
| Shell-effect world freshness (2026-08-09) | World reflects post-mutation reality before any success verdict | Pre-delegation scan was fresh and exposed dirty state; cleanup allowed because baseline/scope-check absent; run_command/bash classified non-write; world became stale only after unreported shell mutations because no incremental retraction/reassertion ran | **Yes — Critical (task-integrity)** |

## Priority backlog (gaps only)

### P0 — correctness

1. **Canonicalize paths in incremental scan** to match full scan (`canonicalScanPath` or shared helper).  
   Risk: duplicate `file_topology` rows for same file; session restore breaks.

2. **Document and/or extend `WorldPredicates`** so `ApplyIncrementalResult` Full mode cannot leave orphans or delete too little.

### P1 — depth parity

3. **Cartographer multi-lang `MapFile`** (or explicit secondary API wired from deep scan) for python/ts/js/rust `code_defines`/`code_calls`.

4. **Emit or retire `dependency_link`** — either implement import edges on scan or remove from replace-set / docs claims.

### P2 — architecture hygiene

5. **Clarify ownership**: chat incremental vs `WorldModelIngestorShard` (when each runs; who wins).

6. **LSP gopls adapter** behind `lsp.Manager` (already sketched in `lsp/README.md`).

7. **Optional:** move stable holographic sections toward prompt atoms if campaign prompts drift.

## Non-gaps (do not “fix” without redesign)

| Item | Why not a gap |
|------|----------------|
| Scope-range dataflow (not CFG) | Source comments state intentional simplicity for Mangle |
| Skip test files in fast AST | Reduces noise; topology still marks `/true` tests |
| Max AST bytes / 5MB dataflow skip | Performance guardrails |
| Soft-fail git when not a repo | Correct product behavior |
| Holographic package file cap 100 | Resource protection |

## Spec debt in older docs

Earlier auto stubs undercounted multi-lang CodeDOM and dataflow depth. This rebuild treats them as **implemented partials**, not zero.

## 2026-08-09 task-integrity incident — true-up (PARTIALLY FIXED / OPEN — truth-corrected for 1ad8238e)

> Records current reality, not aspirational completion. Runtime canonical
> first-source precedence is FIXED in 1ad8238e (embedded first, DB
> deduplicated); boot database reconciliation and stale built-in removal are
> still OPEN as is world/shell task-integrity. Coupled to
> prompt/03-GAP-ANALYSIS G9/G10, prompt/12-FAILURE-MODES FM17/FM18, and
> session/03-GAP-ANALYSIS §7 / session/09-SAFETY-AND-INVARIANTS I18–I21.

### World reality on 2026-08-09 — PARTIALLY FIXED (runtime), OPEN (boot DB + world freshness)

- Runtime: FIXED in 1ad8238e — embedded atoms are now collected first and
  corpus.db copies are deduplicated during prompt collection, so stale database
  copies no longer shadow embedded atoms at runtime even though corpus.db still
  retains 878 stale built-in rows until boot reconciliation removes them.
  Parity checked 888-ID count/order/digest; runtime precedence now correct.
  Remaining OPEN (boot): synchronizer does not yet reconcile/replace and remove
  stale corpus.db built-in rows to match the embedded 888-ID canonical corpus on
  boot; project-only atoms must still be preserved on disk.
- The pre-delegation world scan was fresh and exposed dirty state; cleanup was
  allowed because ownership baseline and shell scope enforcement were absent.
  The task therefore incorrectly cleaned pre-existing tracked and untracked work
  (`git checkout` reverting dirty tracked files, `rm -rf` deleting an untracked
  directory). The world became stale only after the unreported shell mutations
  because no incremental refresh ran — not before delegation.
- `run_command` and `bash` can mutate the workspace while being classified as
  non-write tools. Those shell effects are not surfaced to the world as
  write-intent facts.
- Before 1ad8238e the one-shot world scan ran before delegation but did not
  refresh after shell effects; after a mutation the kernel world still reflected
  the pre-delegation snapshot until the next full scan, so later policy/world
  queries decided on false facts. After the incident’s characterization, runtime
  prompt shadowing is fixed but incremental world refresh after shell effects is
  still missing.

### Required world contracts (PARTIALLY FIXED for runtime, OPEN for boot DB + shell/world)

1. **Canonical source precedence (runtime FIXED in 1ad8238e) plus boot DB reconciliation/removal while preserving project-only atoms (OPEN)** — runtime: embedded collected first and DB deduplicated so stale copies no longer shadow embedded at prompt collection. Boot still OPEN: on every boot the synchronizer must reconcile stale corpus.db built-in rows (878 stale records diverging from the 888-ID canonical corpus) to the embedded content, remove stale built-in duplicates, and never drop project-only (corpus-only) atoms. World consumers must not see stale built-ins on disk and must preserve project atoms.
2. **Immutable pre-task ownership baseline** — snapshot (tracked vs untracked)
   × (dirty vs clean) × (owned by this task vs pre-existing) at task start
   and freeze it. All later revert/delete decisions must consult this
   baseline. Pre-delegation scan itself was fresh; enforcement was missing.
3. **Shell effects must be detected, attributed, scope-checked, and fail
   closed before success** — every run_command/bash invocation is a potential
   world mutation. The session/world boundary must detect filesystem effects,
   attribute them, scope-check against the task’s permitted set and the
   baseline, and deny success before any success verdict on violation.
4. **Pre-existing dirty or untracked paths must never be reverted or
   deleted** — a dirty tracked file or untracked directory that was
   pre-existing in the baseline is inviolate; `git checkout -- <file>` and
   `rm -rf <untracked-dir>` against such paths are violations even though the
   pre-delegation scan already knew they were dirty/untracked.
5. **Accepted mutations must trigger incremental world retraction and
   reassertion** — when a shell effect is scope-checked and accepted, the
   world must incrementally retract and reassert the affected facts
   (incremental world update) before any later policy/world query or success
   verdict, so the kernel reflects post-shell reality and is stale only if
   this refresh is skipped after a mutation.

### Negative acceptance exams (all OPEN)

- **Canonical precedence exam:** seed a temporary corpus.db with 878 stale
  built-in copies diverging from the embedded 888-ID corpus plus one
  project-only atom, boot the synchronizer/world load, and assert (a) stale
  built-ins now match the canonical embedded content and (b) the project-only
  atom survives. Must fail today.
- **Git checkout of dirty tracked work:** in a temporary repo, commit a file,
  mutate it to dirty, run the task/shell path that previously issued
  `git checkout -- <file>`, and assert the dirty content survives. Must fail
  today and pass only with baseline + shell-effect fail-closed.
- **Recursive deletion of an untracked directory:** in a temporary repo,
  create an untracked directory tree, run the task/shell path that previously
  issued `rm -rf <untracked-dir>` via run_command/bash, and assert the
  directory survives intact and the world still reflects it. Must fail today.

## Exit criteria for “gap closed”

| Gap | Done when |
|-----|-----------|
| Path identity | Property test: full vs incremental same Path arg for same file |
| Replace-set | Every emitted predicate either listed or explicitly “ephemeral/scope-only” |
| Multi-lang deep | `EnsureDeepFacts` accepts non-Go or dedicated path with tests |
| dependency_link | Grep shows real emitters + tests **or** removed from WorldPredicates |
| Canonical precedence (2026-08-09) | Seeded temp-corpus.db exam passes: 878 stale built-ins reconciled to 888-ID embedded canonical and project-only atom preserved |
| Pre-task ownership baseline (2026-08-09) | Immutable baseline snapshot at task start with tests for tracked/dirty and untracked classification |
| Shell-effect fail-closed (2026-08-09) | Every run_command/bash mutation detected/attributed/scope-checked with fail-closed before success; negative temp-repo exams for dirty checkout and untracked rm -rf no longer lose data |
| No revert/delete of pre-existing dirty/untracked (2026-08-09) | Temp-repo git-checkout and rm -rf exams assert preservation byte-for-byte |
| Incremental world after shell (2026-08-09) | Accepted shell mutation triggers incremental retraction/reassertion and subsequent world query reflects post-shell filesystem state |
