# init — Mangle Surface (deep-dive)

> Last verified: 2026-07-13  
> Package: `internal/init` — generates facts/templates; does not own core policy corpus.

## What init writes

| Path | Kind | Content |
|------|------|---------|
| `.nerd/profile.mg` | Generated facts | Project identity predicates |
| `.nerd/mangle/extensions.mg` | Template | Commented Decl examples for user schema |
| `.nerd/mangle/policy_overrides.mg` | Template | Commented `permitted` override example |
| Kernel in-process | Transient facts | Scan `ToFacts()` via `LoadFacts`; optional `doc_ingestion` asserts |

## profile.mg predicates (generated)

From `generateFactsFile` in `profile.go`:

- `project_profile(ID, Name, Description).` — escaped strings
- `project_language(/lang).` — sanitized name constant
- `project_framework(/fw).` — if known
- `project_architecture(/arch).` — if known
- `build_system(/name).` — if known
- `architectural_pattern(/p).` — per pattern
- `entry_point("path").` — escaped paths

**Decl responsibility:** core schemas must Decl these predicates before boot loads the file. Init does not emit Decl lines into `profile.mg`.

## Doc ingestion facts

`assertDocFact` pushes:

```
doc_ingestion(path, status, hash, unix_ts)
```

Statuses: `/discovered`, `/analyzing`, `/extracting`, `/stored`, `/synthesized`, `/skipped`, `/failed`.

These are best-effort debug/campaign tracking on the **init-time** kernel instance (not necessarily persisted as `.mg` files).

## Templates vs constitutional safety

`policy_overrides.mg` intentionally ships **commented** examples only. Init must not auto-enable broad permissions. Session boot loads user overlays according to core/config rules (outside this package).

## Non-product artifact

`internal/init/debug_program_ERROR.mg` is a kernel crash dump residue if present — not part of the init Mangle product surface.

## Related

- [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md)
- [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md)
- Core Mangle corpus: `Docs/architecture/mangle/`, `Docs/architecture/core/`
