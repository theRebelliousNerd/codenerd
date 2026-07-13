# init — Failure Modes

> Last verified: 2026-07-13

## Hard failures (abort Initialize)

| Mode | Cause | User symptom | Mitigation |
|------|-------|--------------|------------|
| Directory create fails | Permissions / full disk | Error return | Fix FS permissions; free space |
| Embedding engine init fails | Bad config / Ollama down / missing keys | Error after knowledge.db create | Fix `.nerd/config.json` embedding section; start Ollama; set GenAI key |
| NewInitializer kernel fails | Core kernel construction error | Immediate error | Investigate `core.NewRealKernel` |
| Context cancelled | Timeout / Ctrl+C | “Initialization cancelled” / ctx error | Raise timeout; re-run `--force` |

## Soft failures (warnings, continue)

| Mode | Cause | Effect |
|------|-------|--------|
| System shards fail to start | Shard manager issue | Warning; detection continues |
| Migration check fails | Corrupt agent DBs | Warning |
| Scan fails | FS / scanner error | Warning; profile may lack file counts |
| LoadFacts fails | Kernel fact error | Warning |
| Profile/prefs/session write fails | I/O | Warning; may leave uninitialized marker missing if profile fails |
| Shared KB issues | Store errors | Warning; agents may miss inheritance |
| Agent KB create fails | Per-agent | Agent status failed; others continue |
| Strategic knowledge fails | No LLM / bad JSON | Warning |
| Core/campaign KB fails | Store | Warning |
| Tool generation | Stub always “empty” | Message, not failure |
| Prompt sync fails | YAML/DB | Warning |
| Validation invalid DBs | Missing tables/low atoms | Printed issues; Success may still true |

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
| Context7 fetch short/empty | Fewer atoms; quality score drops |
| SkipResearch true | Explicit basic quality; recommendation to re-run force |

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

1. **`Success: true` means perfect KBs** — validation may still report invalid/low-atom DBs.
2. **`IsInitialized` means agents ready** — only profile.json; agents may be skipped via SkipAgentCreate.
3. **scan == init** — scan does not rebuild agent knowledge.
4. **Tool definitions == runnable tools** — static JSON catalog; Ouroboros generation stubbed.
