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
