# 09 — Safety and Invariants: persist / factsnap

> Last verified against codebase: **2026-08-15**

## 1. Safety posture

factsnap is a **serializer**. It does not participate in constitutional safety (`permitted(...)`). That is intentional: safety attaches when facts are **asserted** into the executive plane, not when bytes are written.

| Concern | Handled by factsnap? | Handled by caller? |
|---------|----------------------|--------------------|
| Default-deny actions | No | Kernel policy |
| Path traversal / workspace root | `factsnap`: no (`MkdirAll` any path). `snapshot.SanitizeName`: yes — names are operator input and are rejected unless they are a plain filename | Callers that build their own paths |
| Untrusted snapshot content | No | Validate before Assert; `nerd snapshot import` never asserts without `--assert` |
| Partial file on crash | Yes (unique temp + fsync + rename + dir fsync) | — |
| Silent truncation / bit rot | Yes (`.sha256` sidecar verified on read) | — |
| Concurrent writers to one path | Yes, in-process (`lockPath`) | Cross-process callers still serialise themselves |
| Sensitive fact leakage | No | Caller controls which facts export |

## 2. Binding invariants

### I1 — Atomic publish

Either the final path contains a complete snapshot after a write returns nil, or
it is unchanged and no orphaned temp remains. The temp file is uniquely named
per call, so a second writer cannot interleave into it; the destination
directory is fsynced after the rename. Tests: `TestWriteCodec_UnknownCodec`,
`TestWrite_WhenWritersRaceOnOnePath_ShouldLeaveOneReadableSnapshot`.

### I1b — Data and digest agree

The sidecar is computed from the exact bytes that were written (the encoder tees
into the digest), and same-path writers are serialised, so a published snapshot
never carries another writer's digest. `NoSidecar` deletes a stale sidecar
rather than leaving one that would fail every later read. Tests:
`TestWrite_WhenDefaultOptions_ShouldEmitSha256Sidecar`,
`TestWrite_WhenNoSidecarRequested_ShouldRemoveStaleSidecar`.

### I2 — Codec parity

For any fact slice, gzip and zstd writes decode to the same equalish fact set. Test: `TestCodecParity`.

### I3 — Detection contract

`Read` chooses a codec from the container's magic bytes when they are
recognisable, and from the path suffix otherwise. Content wins on disagreement
and the disagreement is logged. Writers must still use canonical suffixes:
`snapshot.Resolve` finds files by name + suffix, not by scanning contents.
Tests: `TestRead_WhenSuffixStripped_ShouldSniffGzipMagic`,
`TestRead_WhenSuffixContradictsContent_ShouldTrustContent`.

### I3b — Integrity is fail-closed when claimed

If a sidecar exists, `Read` refuses the file on mismatch (`ErrIntegrity`) rather
than returning a plausible short fact set. If no sidecar exists, the read
proceeds — verification is opt-in by the writer, and absence is not evidence of
corruption. Test: `TestRead_WhenSnapshotTruncated_ShouldFailIntegrityCheck`.

### I4 — Deterministic encoding

`SimpleColumn{Deterministic: true}` on every write. Do not flip without corpus + test updates.

### I5 — Name constant symmetry

Name-typed arguments read back as `types.MangleAtom` so `ToAtom` re-encodes them
as names. Test: `TestNameConstant_WhenRoundTripped_ShouldStayMangleAtom` (two
hops, including a path-shaped name that the string heuristic would reject).

### I5b — Encoding is stable from the second hop

Bools become `/true` / `/false` and whole floats become integers on the first
hop; from hop two onward, repeated export/import is a fixed point. Tests:
`TestBool_WhenRoundTripped_ShouldBecomeNameConstantAndThenStayStable`,
`TestFloat_WhenRoundTrippedTwice_ShouldBeStableAfterFirstHop`.

### I5c — Snapshot names cannot escape the snapshot directory

`snapshot.SanitizeName` rejects separators, dot-leading names, `..` and exotic
characters before any path is built. Tests:
`TestSanitizeName_WhenNameEscapesDirectory_ShouldReject`,
`TestExport_WhenNameEscapesDirectory_ShouldNotWriteOutsideSnapshots`.

### I6 — No import of core from the libraries

Keeps both packages acyclic; the CLI holds the `internal/core` dependency.
Conversion drift is managed by tests (including
`internal/persist/snapshot/kernel_roundtrip_test.go`, which is `package
snapshot_test` precisely so the production packages stay core-free).

### I7 — Error wrapping

Public failures return non-nil `error` with `factsnap:` prefix; no panics on bad paths / bad JSON / missing files.

## 3. Concurrency invariants

- Single-writer per path: **enforced in-process** by `lockPath`; the loser's
  bytes are simply superseded, never blended.
- Cross-process writers: not enforced. Two `nerd` processes exporting the same
  name can still leave process A's data with process B's digest; use distinct
  names (the default is timestamped) if that is a real scenario.
- Readers of a fully published file: OK.
- Reading while rename is in progress: the reader sees either the old or the new
  file, never a partial one.

## 4. Filesystem permissions

| Action | Mode |
|--------|------|
| `MkdirAll` | `0o755` |
| `os.CreateTemp` | `0o600` (Go default) |
| Final file | `0o644` — explicit `chmod` before rename |

The explicit chmod exists because `os.CreateTemp` creates `0600` files: without
it, a snapshot's mode depended on which function wrote it. Multi-user shared
workspaces holding sensitive facts may want tighter modes at a higher layer.

## 5. Mangle Decl / stratification

**N/A.** No `.mg` sources under `internal/persist/`.

## 6. Security notes for future importers

1. **Do not** auto-load snapshots from untrusted paths on boot. `nerd snapshot`
   deliberately has no boot hook.
2. Prefer explicit operator / campaign action to rehydrate. `--assert` loads
   into a kernel that dies with the process; `--to-mangle` produces a file a
   human must move into `.nerd/mangle/` themselves.
3. `Read` slurps the whole file into memory — size-bound it if snapshots can be
   attacker-supplied.
4. A valid sidecar proves the file was not corrupted, **not** that its contents
   are trustworthy: an attacker who writes the snapshot also writes the digest.
5. Treat the JSON legacy path as equally untrusted.

## 7. Comparison to core atom conversion

| Invariant | core `baseTermToValue` | factsnap `baseTermToValue` |
|-----------|------------------------|----------------------------|
| NameType | `string` | `MangleAtom` |
| Logging on unknown type | `logging.Kernel(...)` | returns `c.Symbol` / sprintf silently |

factsnap is stricter about **round-trip identity**, looser about **observability
of unknown AST types**. The divergence is deliberate and documented at
`factsnap.baseTermToValue`; see OPEN-QUESTIONS Q5 for what unification would
actually cost.
