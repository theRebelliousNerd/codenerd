# 12 — Failure Modes: persist / factsnap

> Last verified against codebase: **2026-08-15**

## 1. Catalog

| ID | Failure | Symptom | Mitigation (code or caller) |
|----|---------|---------|-----------------------------|
| F1 | Invalid fact arg for `ToAtom` | `factsnap: fact N (pred) to atom: …` | Fix upstream fact construction; validate before write |
| F2 | Parent directory not creatable | `factsnap: mkdir: …` | Ensure workspace writable; correct path |
| F3 | Disk full mid-write | gzip/zstd write or sync error; tmp removed | Free disk; retry; keep final path intact |
| F4 | Rename fails (Windows lock / AV) | `factsnap: rename …` | Close handles; exclude path from scanners; retry |
| F5 | Unknown codec enum | error; no final file | Use `CodecGzip`/`CodecZstd`/`CodecAuto` only |
| F6 | Wrong extension on read | **Recovered** — magic-byte sniff decodes it and logs a warn | Still write canonical suffixes: `snapshot.Resolve` matches on name + extension |
| F6b | Snapshot renamed so `nerd snapshot import <name>` misses it | `snapshot: no snapshot matching …` | Pass the full path, or restore the canonical name |
| F7 | Corrupt gzip/zstd body | decode error from factstore loaders | Re-export from source of truth; do not partial-trust |
| F7b | Truncated snapshot that still decodes | `factsnap: integrity check failed` (`ErrIntegrity`) | The sidecar catches it; re-export. Without a sidecar this silently yields a short fact set |
| F7c | Sidecar present but snapshot legitimately replaced out of band | every read fails `ErrIntegrity` | Delete the stale `.sha256`, or rewrite through `factsnap` so both files move together |
| F8 | Malformed legacy JSON | `legacy json decode` error | Fix dump or ignore |
| F9 | Missing file | `read` / `legacy read` OS error | Check path; handle `os.IsNotExist` at caller |
| F10 | Nil store (defensive) | `factsnap: nil store` | Should not occur from public loaders |
| F11 | Collect / GetFacts failure | `collect facts for pred/arity` | Upgrade mangle-go; report as library bug |
| F12 | Concurrent writers same path, one process | Last writer wins cleanly | `lockPath` serialises; temp files are unique per call |
| F12b | Concurrent writers same path, two processes | data from A with digest from B → permanent `ErrIntegrity` | Use distinct names (the CLI default is timestamped) |
| F13 | Type surprise after round-trip | bool/time not same Go type | Documented; normalize before compare/assert |
| F14 | NameType vs core string mismatch | equality fails vs `kernel.Query` | Use equalish / normalize; do not raw DeepEqual |
| F15 | Silent non-use | **Closed** — `nerd snapshot` is a real caller | Grow callers, not codec surface |
| F16 | Accidental deletion of package | lost good codec | Wiring audit before delete |
| F17 | mangle-go SimpleColumn format change | all snapshots unreadable | Pin module; migration tool; re-export |
| F18 | Path with no directory component | `MkdirAll(".")` / relative writes | Prefer absolute under workspace |
| F19 | Snapshot name from a script contains a separator or `..` | `snapshot: name … must not contain a path separator` | Sanitise upstream; the rejection is the containment boundary |
| F20 | Export selects predicates that match nothing | `no facts to export …` — no file written | Check spelling, or drop `--predicate`; an empty snapshot would later read as "the kernel knew nothing" |

## 2. Failure flow (write)

```
error at any stage before rename
        │
        ▼
close resources (best effort)
        │
        ▼
deferred close + os.Remove(temp) unless the rename committed
        │
        ▼
return wrapped error
        │
        ▼
final path unchanged (if it existed), no temp residue
```

Sidecar failure is the one non-fatal write error: the snapshot is already
durable, so the write succeeds with a warn line and the file simply reads back
unverified.

## 3. Failure flow (read)

```
ReadFile fail ──────────────► error
        │
        ▼
sidecar mismatch ───────────► ErrIntegrity (nothing decoded)
        │
        ▼
decode fail ────────────────► error (no partial facts returned on SC path)
        │
        ▼
collectFacts fail ──────────► error
        │
        ▼
success → []Fact
```

Note: legacy JSON path returns the full unmarshaled slice or error; no streaming partials.

## 4. Operational playbooks

### Export fails mid-campaign

1. Capture error string (codec, path).  
2. Confirm disk free space and path permissions.  
3. Retry once; if rename fails on Windows, check antivirus locks.  
4. Fall back to JSON only for emergency debug (`json.Marshal` at caller) — do not change factsnap default.

### Import fails after upgrade

1. Run `go test ./internal/persist/...` on new binary.  
2. If unit tests pass but old files fail, format changed — re-export from DB.  
3. Try `LegacyJSON` only if file is actually JSON.

### Facts “don’t match” after round-trip

1. Sort and normalize (`int`/`int64`, `/name` strings as MangleAtom).
2. Check for bool/time expectations, and for whole-valued floats returning as
   `int64` (OPEN-QUESTIONS Q8).
3. Diff predicates first, then arities, then args.
4. `nerd snapshot import <ref> --to-mangle /tmp/a.mg` on both snapshots gives two
   sorted Datalog files that `diff` can compare directly.

### Snapshot suspected corrupt

```bash
cd .nerd/snapshots && sha256sum -c name.sc.gz.sha256
```

The sidecar is sha256sum(1)-shaped precisely so this works without `nerd`.

## 5. What will not fail (by design)

- Codec choice changing fact **meaning** (parity tests).
- Leaving a successful write’s temp file behind.
- Panicking on bad JSON (returns error).
- Returning a truncated fact set from a file whose sidecar disagrees.
- Writing a snapshot outside `.nerd/snapshots/` from a name (paths require the
  explicit `--out` flag).
