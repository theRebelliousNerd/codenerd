# init — Mangle Surface (deep-dive)

> Last verified: 2026-10-27  
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

profile.mg records measurements only. It used to carry
`missing_tool_for(/project_init, /capability)` rows for a fixed per-language
list; `autopoiesis.mg` and `delegation.mg` derive a pending tool-generation
delegation from any such row, so every interactive boot started Ouroboros
codegen. Project tools are now chosen in phase 7e and registered through
`available_tools.json` instead.

## Orientation roster facts (asserted into the orientation engine)

- `profile_signal(/language|/framework|/dependency|/tests, Key)` from the profile.
- `vision_agent(Name, Why)`, `vision_agent_description`, `vision_agent_topic`,
  `vision_agent_question`, `vision_agent_fast_question`, `vision_core_support`
  from the validated domain-expert proposal (`domain_experts.go`).
- Rules in `internal/orient/vision_agents.mg` merge them into
  `orient_agent*`; `orient_param` supplies the roster bounds;
  `ecosystem_agents.mg` keeps the core pair on `core_agent_supported` and
  bounds imports with `imported_kept`.

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
