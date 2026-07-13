# 12 — Failure Modes: persist / factsnap

> Last verified against codebase: **2026-07-13**

## 1. Catalog

| ID | Failure | Symptom | Mitigation (code or caller) |
|----|---------|---------|-----------------------------|
| F1 | Invalid fact arg for `ToAtom` | `factsnap: fact N (pred) to atom: …` | Fix upstream fact construction; validate before write |
| F2 | Parent directory not creatable | `factsnap: mkdir: …` | Ensure workspace writable; correct path |
| F3 | Disk full mid-write | gzip/zstd write or sync error; tmp removed | Free disk; retry; keep final path intact |
| F4 | Rename fails (Windows lock / AV) | `factsnap: rename …` | Close handles; exclude path from scanners; retry |
| F5 | Unknown codec enum | error; no final file | Use `CodecGzip`/`CodecZstd`/`CodecAuto` only |
| F6 | Wrong extension on read | JSON decode fails on binary data | Always use `CanonicalPath` or correct suffix |
| F7 | Corrupt gzip/zstd body | decode error from factstore loaders | Re-export from source of truth; do not partial-trust |
| F8 | Malformed legacy JSON | `legacy json decode` error | Fix dump or ignore |
| F9 | Missing file | `read` / `legacy read` OS error | Check path; handle `os.IsNotExist` at caller |
| F10 | Nil store (defensive) | `factsnap: nil store` | Should not occur from public loaders |
| F11 | Collect / GetFacts failure | `collect facts for pred/arity` | Upgrade mangle-go; report as library bug |
| F12 | Concurrent writers same path | torn final file or rename races | Caller mutex / single exporter |
| F13 | Type surprise after round-trip | bool/time not same Go type | Documented; normalize before compare/assert |
| F14 | NameType vs core string mismatch | equality fails vs `kernel.Query` | Use equalish / normalize; do not raw DeepEqual |
| F15 | Silent non-use | feature never ships | Wire a caller (see gap analysis) |
| F16 | Accidental deletion of package | lost good codec | Wiring audit before delete |
| F17 | mangle-go SimpleColumn format change | all snapshots unreadable | Pin module; migration tool; re-export |
| F18 | Path with no directory component | `MkdirAll(".")` / relative writes | Prefer absolute under workspace |

## 2. Failure flow (write)

```
error at any stage before rename
        │
        ▼
close resources (best effort)
        │
        ▼
deferred os.Remove(tmp) if cleanupTmp
        │
        ▼
return wrapped error
        │
        ▼
final path unchanged (if it existed)
```

## 3. Failure flow (read)

```
ReadFile fail ──────────────► error
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
2. Check for bool/time expectations.  
3. Diff predicates first, then arities, then args.

## 5. What will not fail (by design)

- Codec choice changing fact **meaning** (parity tests).  
- Leaving a successful write’s `.tmp` behind (cleared flag after rename).  
- Panicking on bad JSON (returns error).
