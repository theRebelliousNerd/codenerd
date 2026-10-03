# world — Safety and Invariants

> Last verified: **2026-07-13**

## Safety posture

World is primarily a **read-only perception** package:

| Action class | Behavior |
|--------------|----------|
| File read / walk | Core path |
| Hash compute | Content read |
| Git log | Subprocess read |
| Write | Only caches under `.nerd/cache/` and LocalStore world tables |
| Code mutation | **Not** performed by world |

Constitutional `permitted(...)` is **not** implemented here. Side-effect tools that *use* world facts must still gate via kernel policy elsewhere.

## Invariants

### I1 — Topology independence

Parse failure must not prevent `file_topology` emission when the file is readable/hashable.

### I2 — Portable path preference

When `filepath.Rel(root, path)` succeeds without `..`, store relative slash path. (Enforced in full scan; **must** hold for new emitters.)

### I3 — Nano-resolution invalidation

Cache hits require `ModTime.UnixNano()` **and** `Size` equality. Second-granularity is forbidden for new cache keys.

### I4 — Bounded concurrency

Walk workers acquire semaphore tokens **before** goroutine spawn (`fs.go` critical fix).

### I5 — Enhancement soft-fail

Cartographer continues with symbol facts if dataflow fails. Git scan returns nil facts (not hard error) when not a repo.

### I6 — Test-file AST skip on fast path

Non-test filter for symbol extraction; topology still marks `/true` for tests.

### I7 — Size gates

| Gate | Limit |
|------|------:|
| Fast AST skip | `MaxASTFileBytes` (default 2MB) |
| Dataflow skip | 5MB |
| Holographic package files | 100 |
| Prioritized callers | 10 |
| Caller body lines | 50 |

### I8 — Replace-set honesty

Full apply via `ApplyIncrementalResult` only removes predicates in `WorldPredicates`. Emitters outside that set require explicit retract strategy.

### I9 — Scope diagnostic dedupe

FileScope diagnostic facts keyed by `fact.String()` to avoid duplicate flood.

### I10 — Import cycle

Do not add `world → session/cli` imports. Use `types` and system bridges for core.

## Concurrency safety

| Structure | Guard |
|-----------|-------|
| `FileCache.Entries` | `sync.RWMutex` |
| `FileScope` state | `mu` + `diagMu` + `cbMu` |
| `lsp.Manager` | `sync.RWMutex` |
| `TestDependencyBuilder` | `sync.RWMutex` |
| Scan aggregation | channels (no result mutex) |
| Parser pool | `sync.Pool` (borrow/return discipline) |

## Content / encoding safety

`detectEncoding` surfaces BOM, mixed line endings, invalid UTF-8 as diagnostic facts rather than silent corrupt parse assumptions.

## Security notes

- Git invoked with fixed args (`log`, depth, pretty format); root as `cmd.Dir` only.
- Tree-sitter parses untrusted workspace content — treat as untrusted input; caps reduce resource exhaustion.
- Do not expand ignore allowlist for `.nerd` / `.git` without explicit product decision.

## Mangle Decl

World does not ship package-local Decl files. Safety of predicates depends on:

1. `schemas_world.mg` Decl lines
2. Matching arity/types in Go emitters
3. Policy rules that consume them

Mismatch → silent non-unification or engine errors (see failure modes).

## Accepted incomplete-scan safety contract

**PROPOSED UPLIFT — admitted delta work:** the same GAP-WORLD-L04-01 preservation contract extends through admitted incremental workers, not only the initial census and full fallback. Propagate real admitted-file read/hash failures and cancellation after census before canonical-row retirement, database update/deletion, manifest publication or a successful result. Stage cache/fact updates and join all admitted work before returning an error. `internal/world/incremental_scan.go:341` currently discards a hash error and `internal/world/incremental_scan.go:486` joins workers without a final cancellation/error barrier before publication. Require seeded nonempty-cache, real post-census disappearance, cancellation-during-worker and joined-work negative controls, retaining successful refresh/deletion/migration and syntax-parser best effort. Root's round3-world gate verifies the existing census/full-fallback regressions, not this uncovered worker boundary.

**PROPOSED UPLIFT — GAP-WORLD-L04-01:** an incomplete workspace census must not be interpreted as evidence of deletion. Cancellation, membership errors, filesystem walk errors, and admitted file-metadata errors should reach the caller before canonical-row retirement, deletion derivation, or cache publication. Preserve the previous world/cache generation on those failures. A successful census may still remove genuinely deleted files and perform the existing canonical migration.

The current seam is `internal/world/incremental_scan.go#Scanner.ScanWorkspaceIncremental` (`internal/world/incremental_scan.go:69`). Acceptance requires seeded persisted facts and cache bytes, deterministic failed/canceled enumeration, and unchanged-generation assertions, alongside the real-deletion and membership regressions. A helper-only green does not establish that the production scanner observes this contract.

**PROPOSED UPLIFT — fallback coverage:** this obligation also applies when the manifest is empty, absent or invalid but persisted world rows already exist. The first successful census must not authorize destructive migration before the subsequent full-scan fallback has completed its own required enumeration. `internal/world/incremental_scan.go#Scanner.ScanWorkspaceIncremental` delegates that branch to `internal/world/fs.go#Scanner.ScanWorkspaceCtx` and `#Scanner.ScanDirectory`; their failed/canceled publication paths must preserve the prior database and prior manifest bytes or absence. Join admitted full-scan workers before returning. A nonempty-cache test alone does not establish this guarantee; include successful first-run publication and canonical migration controls.
