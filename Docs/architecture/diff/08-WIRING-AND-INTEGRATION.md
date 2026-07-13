# 08 — Wiring and Integration: `internal/diff`

> Last verified against codebase: 2026-07-13

## 1. Registration model

**There is no registration.** Unlike shards, tools, or Cobra commands, `internal/diff` is
a plain Go library. Nothing calls `RegisterDiff` into the kernel.

Integration = **import + call**.

## 2. Boot / Cortex

Boot paths (`cmd/nerd/chat/session_boot.go`, session executor, etc.) do **not** construct
a global diff engine as part of Cortex assembly. Diff engines appear when the UI needs them.

## 3. Primary wiring: DiffApprovalView

Source: `cmd/nerd/ui/diffview.go`

| Step | What happens |
|------|----------------|
| Import | `"codenerd/internal/diff"` |
| Type aliases | `DiffLine`, `DiffHunk`, `FileDiff`, `DiffLineType` |
| Const aliases | `DiffLineContext/Added/Removed/Header` |
| Construction | `NewDiffApprovalView` sets `diffEngine: diff.NewEngine()` |
| File diffs | `CreateDiffFromStrings` → `diff.ComputeDiff` (**DefaultEngine**) |
| Word diffs | `diffEngine.ComputeWordLevelDiff` on remove+add pairs |
| Binary UI | `if diff.IsBinary` → “Binary file - diff not shown” |
| Hunk headers | Uses `OldStart/OldCount/NewStart/NewCount` |

### Dual-engine quirk

```
CreateDiffFromStrings  ──► DefaultEngine.cache   (process-wide)
DiffApprovalView.word  ──► private Engine.cache  (per view)
```

Not a correctness bug, but cache locality differs: priming via `CreateDiffFromStrings`
does not warm the view’s private engine.

## 4. Mutation approval flow (integration narrative)

```
Agent / tool produces (path, before, after)
        │
        ▼
CreateDiffFromStrings / ComputeDiff
        │
        ▼
PendingMutation{ Diff: *FileDiff, Warnings, Reason, ... }
        │
        ▼
DiffApprovalView.AddMutation
        │
        ▼
User navigates hunks, toggles word-diff / whitespace (UI-side filter)
        │
        ├─ y / a  → approved mutations returned to caller
        └─ n / q  → rejected / closed
        │
        ▼
Caller applies writes only if higher-level policy allows
```

`internal/diff` stops at structure. Approval flags and apply live in UI / session / tools.

## 5. Kernel / VirtualStore / Mangle

| Surface | Wired? |
|---------|--------|
| `user_intent` facts | No |
| `next_action` derivation | No |
| VirtualStore routes | No |
| System/domain shards | No |
| Prompt atoms | No |
| `permitted(...)` | No |

If a future design asserts `file_diff_summary(...)` into the kernel, transduction should
happen **outside** this package (e.g. session layer maps `FileDiff` → Mangle atoms) so
the library stays pure.

## 6. CLI Cobra commands

No dedicated `nerd diff` command lives in this package. Any CLI-facing preview would call
`ComputeDiff` from `cmd/nerd` (none found on 2026-07-13 beyond UI).

## 7. Wiring-before-deletion checklist

If someone proposes deleting `internal/diff` as “unused”:

1. Grep `codenerd/internal/diff` — `diffview.go` will fail to compile.  
2. UI type aliases and `CreateDiffFromStrings` are hard dependents.  
3. Prefer fixing consumer wiring over deletion.  
4. AUDIT.md currently lists `internal/diff` as **clean** — consistent with a focused package.

## 8. Integration test entry points

```powershell
go test ./internal/diff/...
go test ./cmd/nerd/ui/ -run 'Diff|Word'
```
