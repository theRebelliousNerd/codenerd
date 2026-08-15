# init — Testing Alignment

> Last verified: 2026-08-15

## Commands

```powershell
go test ./internal/init/...
go test ./internal/init/ -count=1
go test ./internal/init/ -run TestSanitizeForMangle -v
```

CLI-level init is heavier (embedding, optional network):

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./cmd/nerd/... -count=1
```

## Test file map

| File | Approx focus |
|------|----------------|
| `init_coverage_test.go` | Large matrix: sanitize, versions, tokens, hashes, project ID, IsInitialized, profile/prefs/session I/O, tools catalog, shared KB, backups, ETA, prefs hints, facts file, agent prefs, lockfile parsers, agent categorization |
| `init_test.go` | Broader init behaviors |
| `scanner_test.go` | Language/structure detection |
| `scanner_dependencies_test.go` | Lockfile parsing |
| `agents_knowledge_helpers_test.go` | Atom hash set, research parse, base atoms, doc hash |
| `interactive_display_test.go` | Display + DefaultInteractiveConfig |
| `typeu_coverage_test.go` | Type U parse/validate + directory structure |
| `initializer_truth_test.go` | Overlay preservation, timeout, corrupt config, LLM metrics/race, success truth, phase numbering |
| `preferences_preservation_test.go` | Merge-preserving `savePreferences`, corrupt-file refusal, hint override, force-reinit end to end |
| `agents_curation_test.go` | Type U merge and collision replacement, scripted interactive selection, terminal gate, auto-accept, EOF degradation |
| `strategic_knowledge_parsing_test.go` | Hermetic fake LLM: fenced/bare/brace-bearing JSON, unparseable fallback, relevance arrays, `extractJSON` table |
| `profile_detection_test.go` | Framework ranking and determinism, monorepo manifest discovery, corpus ingestion and reconcile survival, content hash, `missing_tool_for` facts, package-tree hygiene |

## What is well covered

- Pure functions with no LLM/network.
- Persistence round-trips for profile, prefs, session, tools JSON.
- Lockfile parsers (empty, notable deps, invalid JSON).
- ETA tracker phase accounting.
- Type U validation edge cases.
- Backup find/cleanup dry-run and delete.
- Force-init `.gitignore`/Mangle overlay preservation and concurrent LLM outcome accounting.
- Workspace-rooted kernel dependencies and required-failure success semantics.

## Gaps

| Area | Gap |
|------|-----|
| Full `Initialize` happy path | Hard to unit-test without embedding engine + temp workspace scaffolding; limited E2E |
| Parallel agent creation races | Not stress-tested |
| Strategic knowledge LLM path | Parsing is now hermetically covered; the prompt-construction and grounding branches are not |
| Gemini grounding | Integration-dependent |
| Interactive selection I/O | y/n/EOF/auto-accept covered via `InteractiveIO`; the `c` customize toggle loop is still only display-tested |
| CLI `runInit` flags | `cmd/nerd/cmd_init_scan_test.go` pins the help text and the curation flags; end-to-end `--define-agent` through a real `Initialize` is not exercised |
| Validation against real migrated DBs | Partial via schema expectations |

## Recommended test additions (doc only)

1. Worker-pool cancellation when ctx cancelled mid-agent-create.
2. Golden files for `generateFactsFile` Mangle output.
3. The `c` customize toggle loop driven through `InteractiveIO`.
4. End-to-end `nerd init --define-agent` producing a shard knowledge DB.

## Alignment with principles

Tests emphasize **deterministic detectors and persistence**, matching the architectural principle that detection is rule-based. LLM enrichment remains the least tested — acceptable if treated as best-effort warnings, risky if product marketing claims guaranteed strategic quality.
