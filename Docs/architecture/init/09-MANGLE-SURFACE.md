# init — Mangle Surface (deep-dive)

> Last verified: 2026-08-15  
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
- `missing_tool_for(/project_init, /capability).` — one per detected project
  tool need (`determineRequiredTools`), capped at 8 and deduplicated

`missing_tool_for/2` is Declared in `internal/core/defaults/schemas_tools.mg`
and is the same predicate autopoiesis and campaign assert on a capability gap.
Init writes the *need*, never the tool: generation stays behind
`Orchestrator.ExecuteOuroborosLoop` with its full safety depth. Capability names
keep their underscores (`sanitizeToolCapability`, not `sanitizeForMangle`, which
strips them) so an operator reading a kernel query recognizes the tool.

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

Kernel evaluation faults dump `debug_program_ERROR.mg`, but that dump is written
under `.nerd/debug/` and is gitignored, so it never appears in the package tree
or the scanned source. `TestPackageTree_WhenScanned_ShouldContainNoMangleDebugDumps`
keeps it that way.

## Related

- [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md)
- [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md)
- Core Mangle corpus: `Docs/architecture/mangle/`, `Docs/architecture/core/`
