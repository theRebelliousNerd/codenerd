# 11 — Observability: persist / factsnap

> Last verified against codebase: **2026-08-15**

## 1. Current state

| Signal | Present? |
|--------|----------|
| `internal/logging` categories | **Yes** — `CategoryStore` (debug + warn) |
| Metrics / counters | No |
| OpenTelemetry / tracing | No |
| Structured audit events | No |
| Debug dump hooks | **Yes** — the package *is* the dump format, and `nerd snapshot export` is the hook |
| Verbose test logs | Yes — `TestSizeComparison` logs byte sizes |
| Operator-visible output | **Yes** — `nerd snapshot list` reports codec, size, mtime, sidecar state |

## 2. What is logged

| Event | Category | Level | Fields |
|-------|----------|-------|--------|
| Write complete | `store` | debug | fact count, path, codec, bytes on disk, first 12 hex of sha256, duration |
| Read complete | `store` | debug | fact count, path, codec, bytes read, duration |
| Sidecar write failed | `store` | warn | sidecar path, error (write still succeeds — the snapshot is already durable) |
| Suffix/content codec disagreement | `store` | warn | path, suffix codec, sniffed codec |

Fact **contents** are never logged: snapshots carry source paths, symbol names
and campaign text. Only counts, sizes and digests appear.

`CategoryStore` is a placeholder. The natural home is a `CategoryPersist`; the
logger's category list is shared surface, so adding one is a separate change.

## 3. What is still invisible

- No counter of exports per workspace, so snapshot-directory growth is only
  visible via `nerd snapshot list`.
- No timing histogram: an export that gets slow shows up as one debug line, not
  a trend.
- Integrity failures surface as `ErrIntegrity` to the caller and are not
  separately counted.

## 4. Debug workflow

```bash
# What does this workspace's kernel actually hold?
nerd snapshot export debug-now
nerd snapshot list

# What is inside a snapshot someone sent you?
nerd snapshot import ./their-dump.sc.zst --show 20

# Render it as Datalog you can read and diff
nerd snapshot import debug-now --to-mangle /tmp/kernel.mg

# Codec sizes, informational
go test ./internal/persist/factsnap/ -v -run TestSizeComparison
```

Integrity can also be checked with standard tooling, because the sidecar is
sha256sum(1)-shaped:

```bash
cd .nerd/snapshots && sha256sum -c debug-now.sc.gz.sha256
```

## 5. Glass box / transparency

No integration with `internal/transparency` or the glass-box UI. Snapshots are
opaque binary files; operators inspect them through `nerd snapshot import`,
which prints a predicate histogram before any facts.

## 6. Observability gap score

**3 / 5** — write/read are traced with size, codec, digest and duration, and the
operator surface answers "what is on disk" and "what is in this file". Missing:
counters, trends, and a dedicated log category.
