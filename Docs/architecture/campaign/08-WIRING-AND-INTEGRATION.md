# 08 — Wiring and Integration: campaign

> Last verified: **2026-07-13**

## Integration thesis

Campaign is a **library package** with no `init()` self-registration into the kernel. Callers must construct an `Orchestrator` with Cortex dependencies and load a kernel program that includes campaign rules.

## Cobra wiring

**File:** `cmd/nerd/cmd_campaign.go`

| Command | Handler role |
|---------|--------------|
| `nerd campaign start [goal]` | Boot deps → Decompose or assault → SetCampaign → Run |
| `nerd campaign status` | Load/list progress |
| `nerd campaign pause` | Signal pause on running orchestrator |
| `nerd campaign resume` | Load + Run |
| `nerd campaign list` | Enumerate `.nerd/campaigns/*.json` |

Parent registration: campaign command tree attached from `cmd/nerd` root (see CLI architecture).

Typical start path needs:

1. Workspace + API key + timeout (global flags)  
2. Cortex/kernel/LLM/executor/VirtualStore  
3. `session.TaskExecutor` (JIT)  
4. Optional: PromptAssembler via `CampaignJITProvider`  
5. Optional intelligence suite  

## JIT prompt adapter

**File:** `cmd/nerd/campaign_jit_provider.go`

```text
CampaignJITProvider.GetPrompt(role, campaignID)
  → articulation.PromptAssembler.AssembleSystemPrompt
       PromptContext{ ShardType from role, CampaignActive, CampaignPhase }
```

Wired with `orch.SetPromptProvider(provider)` so decomposer/replanner avoid static-only prompts.

## Chat / TUI

| Piece | Role |
|-------|------|
| Chat slash `/campaign …` | Long-horizon UX including assault entry (see CLI chat docs) |
| `cmd/nerd/ui/campaign_page.go` | Renders Progress-like state |
| ProgressChan / EventChan | Optional live updates when configured |

Fact-flow note: chat may still run normal OODA for non-campaign turns; campaign Run is a **nested long loop** holding the same Cortex.

## Session TaskExecutor

Migration path documented on `SetTaskExecutor`:

```text
orch := campaign.NewOrchestrator(cfg)
orch.SetTaskExecutor(session.NewJITExecutor(...))
```

`spawnTask` refuses nil TE. CheckpointRunner also requires TE for shard-based verification methods.

## Kernel program requirements

Ensure loaded Mangle sources include:

1. Campaign base policy (state machine / eligibility)  
2. `campaign_rules.mg` advanced rules  
3. Schemas/Decls for campaign predicates  
4. Build topology if phase categories enforced  

Without these, `Query("current_phase")` etc. yield empty results → no progress or block paths.

## Northstar

`SetNorthstarObserver(*northstar.CampaignObserver)`:

- Phase start checks can **block** transitions  
- Campaign end observation on success path  
- Risk gate may enable/disable northstar enforcement via toggles  

## Intelligence suite wiring pattern

```text
gatherer := NewIntelligenceGatherer(...)
board := NewShardAdvisoryBoard(consultProvider)
edge := NewEdgeCaseDetector(kernel, scanner)
pregen := NewToolPregenerator(...)

cfg.IntelligenceGatherer = gatherer
cfg.AdvisoryBoard = board
cfg.EdgeCaseDetector = edge
cfg.ToolPregenerator = pregen
// or post-New setters (also recompute risk gate state)
```

`wireIntelligenceComponents` in init also forwards into decomposer.

## Assault start pattern

```text
cfg := campaign.DefaultAssaultConfig()
// mutate Scope, Include, Stages...
c := campaign.NewAdversarialAssaultCampaign(workspace, cfg)
orch.SetCampaign(c)
orch.Run(ctx)
```

Artifacts appear under `.nerd/campaigns/<slug>/assault/` without decomposer.

## VirtualStore interaction

Rolling-wave issues a best-effort:

```text
virtualStore.RouteAction(ctx, next_action{ id, /refresh_scope, workspace })
```

Campaign does not generally own full OODA `next_action` derivation for ordinary tasks — TE/shards do.

## E2E wiring

`tests/e2e/campaign_session_integration_test.go` exercises campaign with session stack — prefer this over inventing new integration harnesses.

## Wiring audit checklist

Before declaring “campaign unused” or deleting a component:

1. Grep `NewOrchestrator`, `Decompose`, `NewAdversarialAssaultCampaign`  
2. Grep CLI handlers and chat commands  
3. Grep Mangle predicates asserted by `ToFacts`  
4. Check optional DI nil-guards (absence of wire ≠ dead code)  
5. Check journal/assault disk paths for operator tooling  

## Known partial wires

| Feature | Partial nature |
|---------|----------------|
| Intelligence gatherer | Implemented; may be nil in minimal CLI paths |
| Advisory hard-stop | Votes synthesized; hard abort not always |
| Static vs JIT prompts | Both supported; static default in package |
| ShardManager vs TE | TE required for spawn; SM for monitoring |

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).
