# 07 — Dependency Map: campaign

> Last verified: **2026-07-13**

## Upstream (campaign imports)

| Package | Why |
|---------|-----|
| `codenerd/internal/core` | Kernel interface, Fact, VirtualStore, LoadFacts/Query/Assert |
| `codenerd/internal/core/shards` | ShardManager monitoring surface |
| `codenerd/internal/perception` | LLMClient, Transducer |
| `codenerd/internal/session` | TaskExecutor / TaskRequest |
| `codenerd/internal/tactile` | Command execution (tests/build/assault stages) |
| `codenerd/internal/logging` | CategoryCampaign timers/logs |
| `codenerd/internal/types` | KernelTx, ExtractString, ShardInfo, SessionContext helpers |
| `codenerd/internal/northstar` | CampaignObserver alignment |
| `codenerd/internal/tools/research` | GroundingHelper, ThinkingHelper |
| `codenerd/internal/embedding` | DocumentIngestor config |
| `codenerd/internal/world` | Scanner for edge case detector |
| `github.com/google/uuid` | Campaign IDs |

Standard library: `context`, `sync`, `sync/atomic`, `encoding/json`, `os`, `path/filepath`, `time`, `crypto/sha256`, etc.

## Downstream (who imports campaign)

| Consumer | Path | Usage |
|----------|------|-------|
| CLI campaign commands | `cmd/nerd/cmd_campaign.go` | NewOrchestrator, Decompose, Run, assault builders |
| JIT prompt adapter | `cmd/nerd/campaign_jit_provider.go` | PromptProvider bridge |
| Campaign UI page | `cmd/nerd/ui/campaign_page.go` | Progress/types display |
| UI tests | `cmd/nerd/ui/pages_test.go` | fixtures |
| E2E | `tests/e2e/campaign_session_integration_test.go` | integration |
| Skills / internal docs | `.agents/skills/...`, `GEMINI.md` | human references |

Grep signal: `codenerd/internal/campaign` appears primarily under `cmd/nerd` and tests — not deep inside core.

## Policy / Mangle dependency (not a Go import)

Campaign **emits** and **queries** predicates defined/used in:

- `internal/core/defaults/campaign_rules.mg`
- Base policy campaign sections (`policy` corpus Section 19 per file header comments)
- `internal/core/defaults/build_topology.mg` (`phase_category` / architecture layers)

If those sources are missing from the kernel program, orchestrator queries return empty and execution stalls or fails closed via blocked/no-phase paths.

## Dependency direction rules

```
cmd/nerd ──► campaign ──► core / session / perception / tactile
                │
                └──X── articulation   (avoid cycle; JIT adapter lives in cmd)
```

Campaign must **not** import `cmd/*` or `articulation` directly. Prompt JIT uses interface + cmd-side adapter.

## Risk of cyclic growth

| Temptation | Correct alternative |
|------------|---------------------|
| Import articulation for prompts | `PromptProvider` interface |
| Import chat model for progress | channels + Progress DTO |
| Import world deeply in orchestrator | optional EdgeCaseDetector DI |
| Import tools/* freely | keep assault stages on tactile + TE intents |

## External side effects

| Side effect | Location |
|-------------|----------|
| Filesystem under `.nerd/campaigns/` | lifecycle, assault, journal |
| Process exec (go test, builds) | checkpoint, assault |
| LLM completions | decomposer, replan, pager compress, triage, fallbacks |
| Kernel fact store mutations | everywhere |
| VirtualStore route | rolling-wave refresh |

## Related corpora

- Core kernel: `Docs/architecture/core/`  
- Session/TE: `Docs/architecture/session/`  
- CLI wiring: `Docs/architecture/cli/`  
- Prompt JIT: `Docs/architecture/prompt/`
