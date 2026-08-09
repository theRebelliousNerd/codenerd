# init — Failure Modes

> Last verified: 2026-08-09

## Hard failures (abort Initialize)

| Mode | Cause | User symptom | Mitigation |
|------|-------|--------------|------------|
| Directory create fails | Permissions / full disk | Error return | Fix FS permissions; free space |
| Embedding engine init fails | Bad config / Ollama down / missing keys | Error after knowledge.db create | Fix `.nerd/config.json` embedding section; start Ollama; set GenAI key |
| Mangle overlay creation fails | Permissions / full disk | Error; existing overlay content remains untouched | Fix filesystem and retry |
| NewInitializer kernel fails | Core kernel construction error | Immediate error | Investigate `core.NewRealKernelWithWorkspace` |
| Context cancelled | Timeout / Ctrl+C | “Initialization cancelled” / ctx error | Raise timeout; re-run `--force` |

## Required failures (continue to collect, then `Success=false`)

| Mode | Cause | Effect |
|------|-------|--------|
| System shards or migration fail | Runtime/store issue | Failure recorded; later independent phases may still run |
| Scan or LoadFacts fails | FS/kernel issue | Failure recorded; profile may be incomplete |
| Profile/facts/prompt stores fail | I/O or DB | Failure recorded |
| Preferences/session/tools/registry/prompt sync fail | I/O or DB | Failure recorded |
| Structural validation fails, is invalid, or finds zero shard DBs | Corrupt/incomplete KB output | Failure recorded; `Success=false` |

## Soft failures (warnings, continue)

| Mode | Cause | Effect |
|------|-------|--------|
| Shared KB issues | Store errors | Warning; agents may miss inheritance |
| Agent KB create fails | Per-agent | Agent status failed; others continue |
| Strategic knowledge / LLM call fails | Provider, timeout, bad JSON | Warning plus aggregate provider/model failure counts |
| Core/campaign KB fails | Store | Warning |
| Tool generation | Stub always “empty” | Message, not failure |

## Detection failure modes

| Mode | Symptom | Mitigation |
|------|---------|------------|
| Language `unknown` | Generic SecurityAuditor/TestArchitect only | Add known config file; fix monorepo layout |
| Framework empty | Miss WebAPI/Frontend experts | Dep detection may still catch react/gin names |
| Lockfile unreadable | Miss transitive deps | Ensure lockfile present/parseable |
| Wrong primary language in monorepo | Max config-file count wins | Reorganize or force profile edit |

## Research / network

| Mode | Effect |
|------|--------|
| No Context7 key | Research thin; base atoms only |
| Context7 fetch short/empty | Fewer atoms; population score drops |
| SkipResearch true | Explicit basic population; recommendation to re-run force |

## Concurrency failure modes

| Mode | Risk | Notes |
|------|------|-------|
| Full ProgressChan | Drops updates | Non-blocking by design |
| Parallel agent errors | Isolated in result slice | One agent fail ≠ all fail |
| Shared embed engine | Concurrent DB writes separate files | Engine itself must be concurrency-safe (embedding package invariant) |

## Post-init operator recovery

```
# Re-run upgrade path
nerd init --force

# Refresh world facts only
nerd scan

# After migrations leave backups
nerd init --cleanup-backups

# Manual: inspect profile
# .nerd/profile.json , .nerd/profile.mg
```

## Dangerous misconceptions

1. **`Success: true` means semantic quality** — it means required artifacts completed; structural validation and enrichment metrics are separate.
2. **`IsInitialized` means agents ready** — only profile.json; agents may be skipped via SkipAgentCreate.
3. **scan == init** — scan does not rebuild agent knowledge.
4. **Tool definitions == runnable tools** — static JSON catalog; Ouroboros generation stubbed.
