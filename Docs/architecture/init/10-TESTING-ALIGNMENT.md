# init — Testing Alignment

> Last verified: 2026-08-09

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
| Strategic knowledge LLM path | No hermetic mock suite visible in package tests |
| Gemini grounding | Integration-dependent |
| Interactive selection I/O | Display tested; full stdin decision tree lightly covered |
| CLI `runInit` flags | Live in `cmd/nerd` tests, not `internal/init` |
| Validation against real migrated DBs | Partial via schema expectations |

## Recommended test additions (doc only)

1. Temp-dir `createDirectoryStructure` + `IsInitialized` after fake profile write (already partially present).
2. Fake `LLMClient` for `generateStrategicKnowledge` JSON parse success/fail.
3. Worker-pool cancellation when ctx cancelled mid-agent-create.
4. Golden files for `generateFactsFile` Mangle output.

## Alignment with principles

Tests emphasize **deterministic detectors and persistence**, matching the architectural principle that detection is rule-based. LLM enrichment remains the least tested — acceptable if treated as best-effort warnings, risky if product marketing claims guaranteed strategic quality.
