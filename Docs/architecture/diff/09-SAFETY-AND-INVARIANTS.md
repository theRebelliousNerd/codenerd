# 09 — Safety and Invariants: `internal/diff`

> Last verified against codebase: 2026-07-13

## 1. Scope of “safety”

This package does not implement constitutional `permitted(...)`. Safety here means:

- **Resource safety** (time, memory) under adversarial/pathological text.  
- **Data integrity** of returned structures.  
- **Concurrency safety** of shared engines.  
- **Honest signaling** (`IsBinary`) so UI does not render garbage.

Effectful safety (write file, run shell) is **out of band**.

## 2. Invariants

### I1 — Non-nil FileDiff on success path

`ComputeDiff` constructs a `FileDiff` and returns a pointer; it does not return `nil` for
ordinary string inputs. Callers may still nil-check defensively.

### I2 — Binary implies empty hunks

If `IsBinary == true`, `Hunks` is empty (short-circuit before Myers).

### I3 — Empty content flags are content-based

- `IsNew ⇔ oldContent == ""`  
- `IsDelete ⇔ newContent == ""`  
Independent of path strings. Both may be true simultaneously.

### I4 — Context clamp

Any path into `groupIntoHunks` via `convertToHunks` clamps context to `[0, 1000]`.

### I5 — Timeout configured at construction

Every `NewEngine` sets `dmp.DiffTimeout = 5s`. Zero would disable (code comments note this);
production constructor does **not** set zero.

### I6 — Paths are labels

Empty, huge, or weird paths must not panic the engine. Content is the algorithm input.

### I7 — Hunk counts match line types

For each hunk:

```
OldCount = count(LineRemoved ∪ LineContext)
NewCount = count(LineAdded ∪ LineContext)
```

Enforced by `computeHunkCounts` and tested.

### I8 — Cache key ignores paths

Same content pair → same cached hunks regardless of path labels; paths rewritten on hit.

## 3. Threat / abuse model (library)

| Threat | Mitigation | Residual |
|--------|------------|----------|
| Huge binary as string | NUL short-circuit | Non-NUL binary (UTF-16, etc.) still diffs as text |
| Minified giant line | 5s DiffTimeout | Result may be partial/empty per library semantics if timed out |
| Extreme context int | clamp | Public API doesn’t expose context today |
| Cache memory exhaustion | None | Unbounded `sync.Map` |
| Hash collision wrong result | None | Theoretical |
| Shared hunk mutation | None | Shallow clone |
| Concurrent ClearCache | New map swap | Subtle races documented as TEST_GAP |

## 4. Concurrency rules for callers

| Pattern | Guidance |
|---------|----------|
| Many goroutines `ComputeDiff` on one `Engine` | Supported (`sync.Map`); tested |
| Mutate returned `FileDiff.Hunks` | **Unsafe** w.r.t. cache sharing — treat as read-only or deep-copy first |
| `ClearCache` during heavy compute | Prefer quiet periods; not formally barriered |
| Share `DefaultEngine` across unrelated features | Accept shared cache pollution; use `NewEngine` for isolation |

## 5. Mangle / Decl

**N/A.** No predicates, no Decl requirements, no stratification concerns.

## 6. Constitutional boundary

```
diff package          →  describe Δ(text)
policy / VirtualStore →  permit & apply Δ
```

Never collapse these layers inside `internal/diff`.

## 7. Binary detection design note

NUL-only detection matches common Unix tooling heuristics and is cheap (`IndexByte`).
It will **not** flag all binaries (e.g. pure ASCII payloads that are still non-source).
That is an intentional simplicity trade-off, not a claim of perfect binary classification.
