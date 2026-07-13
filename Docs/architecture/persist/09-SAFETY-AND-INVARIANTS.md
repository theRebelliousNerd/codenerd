# 09 — Safety and Invariants: persist / factsnap

> Last verified against codebase: **2026-07-13**

## 1. Safety posture

factsnap is a **serializer**. It does not participate in constitutional safety (`permitted(...)`). That is intentional: safety attaches when facts are **asserted** into the executive plane, not when bytes are written.

| Concern | Handled by factsnap? | Handled by caller? |
|---------|----------------------|--------------------|
| Default-deny actions | No | Kernel policy |
| Path traversal / workspace root | Partial (`MkdirAll` any path) | Caller should constrain roots under `.nerd/` |
| Untrusted snapshot content | No | Validate before Assert |
| Partial file on crash | Yes (tmp + rename) | — |
| Sensitive fact leakage | No | Caller controls which facts export |

## 2. Binding invariants

### I1 — Atomic publish

Either the final path contains a complete snapshot after `WriteCodec` returns nil, or it is unchanged and no orphaned tmp remains after failure (best-effort remove). Tests: `TestWriteCodec_UnknownCodec`.

### I2 — Codec parity

For any fact slice, gzip and zstd writes decode to the same equalish fact set. Test: `TestCodecParity`.

### I3 — Extension contract

`Read` codec choice is a pure function of path suffix. Violating extension naming is a caller bug.

### I4 — Deterministic encoding

`SimpleColumn{Deterministic: true}` on every write. Do not flip without corpus + test updates.

### I5 — Name constant symmetry

Name-typed arguments read back as `types.MangleAtom` so `ToAtom` re-encodes them as names. Changing this without multi-hop tests is a regression.

### I6 — No import of core

Keeps package acyclic. Conversion drift is managed by tests, not by importing kernel.

### I7 — Error wrapping

Public failures return non-nil `error` with `factsnap:` prefix; no panics on bad paths / bad JSON / missing files.

## 3. Concurrency invariants

- Single-writer per path: **not enforced** — caller serializes.  
- Readers of a fully published file: OK.  
- Reading while rename in progress: OS-dependent; treat as rare race for future multi-process use.

## 4. Filesystem permissions

| Action | Mode |
|--------|------|
| `MkdirAll` | `0o755` |
| `os.Create` temp | process umask defaults |
| Final file | same as create |

No explicit `chmod` after write. Multi-user shared workspaces may need tighter modes at a higher layer.

## 5. Mangle Decl / stratification

**N/A.** No `.mg` sources under `internal/persist/`.

## 6. Security notes for future importers

1. **Do not** auto-load snapshots from untrusted paths on boot.  
2. Prefer explicit operator / campaign action to rehydrate.  
3. Size-bound large `Read`s if snapshots can be attacker-supplied.  
4. Treat JSON legacy path as equally untrusted.

## 7. Comparison to core atom conversion

| Invariant | core `baseTermToValue` | factsnap `baseTermToValue` |
|-----------|------------------------|----------------------------|
| NameType | `string` | `MangleAtom` |
| Logging on unknown type | `logging.Kernel(...)` | returns `c.Symbol` / sprintf silently |

factsnap is stricter about **round-trip identity**, looser about **observability of unknown AST types**.
