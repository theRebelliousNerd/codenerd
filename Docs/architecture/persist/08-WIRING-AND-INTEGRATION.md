# 08 — Wiring and Integration: persist

> Last verified against codebase: **2026-08-15**  
> Status: **wired** — first production caller landed (`nerd snapshot`, kernel debug export)

## 1. Registration hooks

| Hook type | Present? |
|-----------|----------|
| `init()` registration | Yes — `cmd/nerd/cmd_snapshot.go` `init()` builds the flag set and attaches the three subcommands to `snapshotCmd` |
| Shard registration | No |
| VirtualStore route | No |
| CLI `AddCommand` | Yes — `snapshotCmd` (registered on `rootCmd` in `cmd/nerd/main.go`) |
| Config key | No |
| Boot step in `system` / `session_boot` | No — deliberately; see §5 |
| Mangle Decl / policy | No — snapshots carry facts already declared by the constitution |

`factsnap` and `snapshot` remain **import-and-call** libraries with no
self-registration; the CLI is the thing that registers.

## 2. Current call graph

```
cmd/nerd/cmd_snapshot.go
  ├── runSnapshotExport ── core.NewRealKernelWithWorkspace  (local kernel, no LLM)
  │                     ├── kernel.GetBaseFacts()      (default: EDB only)
  │                     ├── kernel.Query(pred)          (--predicate, repeatable)
  │                     ├── kernel.QueryAll()           (--derived)
  │                     └── snapshot.Export ──► factsnap.WritePath ──► .nerd/snapshots/<name>.sc.gz
  ├── runSnapshotImport ── snapshot.Import ──► snapshot.Resolve + factsnap.Read
  │                     ├── snapshot.Summarize          (predicate histogram)
  │                     ├── writeFactsAsMangle          (--to-mangle: reviewable Datalog)
  │                     └── kernel.LoadFacts            (--assert: in-process only)
  └── runSnapshotList  ── snapshot.List ──► Entry rows (name, codec, size, mtime, sidecar)
```

Chosen first caller: **kernel debug export** (OPEN-QUESTIONS Q1). It is the
lowest-risk site because it reads a kernel that the command itself booted, it
needs no API key, no network and no shards, and nothing it writes is read back
automatically.

## 3. Fact-flow position (as built)

```
workspace .mg + embedded constitution
        │
        ▼
core.NewRealKernelWithWorkspace ──► EDB (GetBaseFacts)
        │
        ▼
snapshot.Export ──► factsnap.WritePath ──► .nerd/snapshots/name.sc.gz
                                          + name.sc.gz.sha256
```

Import path (restore):

```
operator  →  snapshot.Import  →  factsnap.Read (verifies sidecar)  →  []types.Fact
                    │
                    ├── default: summarise only
                    ├── --to-mangle: render Datalog for review, then the operator
                    │                decides whether it belongs in .nerd/mangle/
                    └── --assert:  LoadFacts into a kernel that lives and dies
                                   with the process
```

There is deliberately **no** path from a snapshot file to workspace boot state.
A snapshot is untrusted the moment it leaves the process that wrote it, so
adoption requires an operator to move rendered Datalog into `.nerd/mangle/`
themselves. That is OPEN-QUESTIONS Q3 resolved as option **C**.

## 4. Canonical workspace paths

| Path | Written by | Notes |
|------|-----------|-------|
| `<workspace>/.nerd/snapshots/` | `snapshot.Dir(root)` | Created on first export |
| `<workspace>/.nerd/snapshots/<name>.sc.gz` | gzip export (default) | `<name>` is sanitized: letters, digits, `-`, `_`, `.` |
| `<workspace>/.nerd/snapshots/<name>.sc.zst` | `--codec zstd` | |
| `<name>.sc.gz.sha256` | every write unless `Options.NoSidecar` | sha256sum(1) format |
| anywhere | `nerd snapshot export --out PATH` | escape hatch for attaching a snapshot to a bug report |

Default export name is `kernel-YYYYMMDD-HHMMSS` (`snapshot.DefaultName`), which
sorts chronologically as text.

## 5. Wiring anti-patterns

| Anti-pattern | Why |
|--------------|-----|
| Delete package for “unused” | It has a production caller now |
| Import core from factsnap | Cycle risk — the CLI holds the core dependency, not the library |
| Assert snapshot at boot | Bypasses operator review; that is why no boot step exists |
| Export derived facts by default | Re-importing conclusions turns them into premises; `--derived` is opt-in |
| Write without canonical extension | Breaks bare-name resolution in `snapshot.Resolve` |
| Hand-build `.nerd/snapshots/...` paths at a call site | Use `snapshot.Dir` / `Export`; naming rules are one place |
| Use factsnap for non-fact blobs | Wrong abstraction |

## 6. How to wire the next caller

1. Convert domain objects → `[]types.Fact` (many already have `ToFacts()`).
2. Call `snapshot.Export(root, name, facts, codec)` — do not call `factsnap`
   directly unless the file belongs outside the workspace.
3. On load: `snapshot.Import` → inspect → assert only through kernel APIs the
   caller already owns.
4. Add an integration test at the caller package, not only factsnap unit tests
   (see `internal/persist/snapshot/kernel_roundtrip_test.go` for the shape).

Remaining candidates, still unwired: `internal/campaign` assault artifacts and
`internal/world` scan freezes. Both now have a paved path.

## 7. Related “persist” words that are **not** this package

Grep hits for `persist` / `snapshot` in browser, campaign, autopoiesis refer to
**other** persistence mechanisms. Do not conflate:

- `SessionManager.persistSessions` (browser metadata JSON)
- Campaign assault JSONL results
- Autopoiesis “persistent agent” (logical agent lifecycle)
- `nerd direct` root snapshots (`snapshotDirectRoot`, a filesystem listing)

Only `internal/persist/**` is this corpus’s subject.
