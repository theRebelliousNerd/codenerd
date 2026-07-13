# OPEN QUESTIONS — campaign

> Last verified: **2026-07-13**  
> Real open questions from code reading; not rhetorical filler.

## Q1 — Who is the single owner of “active campaign” process state?

Disk holds many campaigns; orchestrator holds one. Is there a process-global registry (CLI only) or can multiple orchestrators run concurrently against one workspace safely (write-set + JSON collisions)?

## Q2 — Should advisory blocking concerns hard-fail decompose?

Code logs synthesis and concerns but planning often continues. Product choice: hard gate, user prompt, or risk-score only?

## Q3 — Exact policy Section 19 surface vs `campaign_rules.mg` split

Header claims base SM in policy Section 19. Maintainers need a single index of which file owns `eligible_task` / `current_phase` to avoid dual definitions.

## Q4 — TaskExecutor mandatory forever?

Constructor allows ShardManager without TE for construction, but `spawnTask` requires TE. Should validation require TE always and drop SM-only construction?

## Q5 — Confidence scale consistency

`ToFacts` scales confidence 0–1 → 0–100 integers for metadata. Do all Mangle rules assume 0–100? Any path still asserting 0–1?

## Q6 — Assault multi-language future

Discover is Go-centric (`discoverGoTargets`). First-class scopes for polyglot monorepos?

## Q7 — Nested campaign resource isolation

`CampaignRefInheritance` fields exist; how thoroughly is FS/memory/tool scope enforced at runtime vs documented intent?

## Q8 — Heartbeat save vs user Pause race

Autosave and pause both lock `o.mu` — fine. Should paused campaigns suppress autosave noise/journal growth?

## Q9 — Static prompt retirement date

No formal trigger for deleting large `prompts.go` after JIT atoms land. What is the acceptance criterion?

## Q10 — Relation to multistep chat decomposer

`cmd/nerd/chat/multistep_decomposer.go` exists outside this package. Is it a parallel path or should it call `campaign.Decomposer` exclusively?

---

Resolve by code change + doc update; close questions here when answered with evidence paths.
