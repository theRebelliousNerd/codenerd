# transparency — Gap Analysis

> Last verified: 2026-07-13  
> Compare: package `doc.go` + config flags + vision vs actual code + wiring

## 1. Matrix: claimed vs real

| Claim / surface | Reality | Severity |
|-----------------|---------|----------|
| Opt-in transparency master switch | **Real** — default off; `/transparency` + Enable APIs | — |
| Non-intrusive emit | **Real** — drop on full, disabled no-op | — |
| Lazy expensive work | **Mostly real** — Explainer only when called; buses cheap | Low |
| Shard execution phases visible | **Partial** — `ShardObserver` solid; live path prefers Glass Box `CategoryShard` from manager, not Manager.StartShard | **Medium** |
| Safety gate explanations | **Partial** — formatter/reporter real; automatic capture from all deny sites incomplete | **Medium** |
| JIT explain mode | **Config + category only** — no package-local JIT atom explainer | **Medium** |
| Proof trees / derivation explain | **Real formatter**; trace production is mangle/core/chat | Low |
| Operation summaries | **Formatter only**; few automatic producers | Low |
| Error categorization | **Real** heuristic classifier | Low (accuracy) |
| Stream reasoning flag | **Status display**; not enforced in this package | **Medium** |
| Glass Box categories filter | **Real** on bus; chat must call SetCategories from config | Low |
| Glass Box verbose | **Real** (immediate path) | — |
| Tool events always on | **Real** | — |
| Manager on ShardManager | **Stored as `any`**, not type-used for phases | **Medium** |
| Drop observability | **Missing** counters | Low |
| Headless JSON event stream | **Missing** | Low (future) |

## 2. Spec vs reality detail

### 2.1 doc.go feature list

| Feature in doc.go | Package implements | Wired end-to-end |
|-------------------|--------------------|------------------|
| Shard execution phases | Yes (`ShardObserver`) | Partial |
| Safety gate explanations | Yes | Partial |
| JIT explain mode | Category + config only | No (elsewhere/partial) |
| Proof trees | Explainer over traces | Yes via `/why` + traces |
| Operation summaries | Format helper | Sparse producers |
| Error categorization | Yes | Via FormatError when used |

### 2.2 Config flags vs Manager behavior

| Flag | Read by TransparencyManager | Effect |
|------|----------------------------|--------|
| `Enabled` | Yes | Master |
| `ShardPhases` | Yes | Gates Start/Update/End shard + Enable cascade |
| `SafetyExplanations` | Yes | Gates ReportSafetyViolation |
| `VerboseErrors` | Yes | FormatError verbosity |
| `StreamReasoning` | Display only in GetStatus | No branch in Manager |
| `JITExplain` | Display only | No branch |
| `OperationSummaries` | Display only | No branch |
| Glass Box fields | No (chat owns) | Correct separation if chat honors them |

### 2.3 Dual shard visibility systems

| System | State | Feed |
|--------|-------|------|
| `ShardObserver` | Full API + tests | Needs `StartShard`/`UpdateShardPhase` callers |
| Glass Box `CategoryShard` | Live in TUI | `ShardManager.emitShardEvent` |

**Gap:** operators using `/transparency` active operations list may see **empty** while Glass Box shows spawns—because they are not the same feed.

### 2.4 Safety feed

| Source of deny | Feeds SafetyReporter? |
|----------------|----------------------|
| Kernel `permitted` failure | Only if some adapter calls Report* |
| VirtualStore action failure | Emits Tool/Routing events; not necessarily SafetyViolation |
| ExplainSafetyAction | Offline heuristic only |

## 3. Prioritized backlog (from gaps)

### P0 — correctness / honesty

1. **Either wire or document demotion of `ShardObserver` vs Glass Box.** Prefer one canonical live stream; keep Observer for structured status if fed.  
2. **Type `SetTransparencyManager`** or stop storing dead `any` if unused.

### P1 — product completeness

3. Auto-report safety violations when constitutional gate denies (pass rule + action + target).  
4. Honor or remove status-only flags (`JITExplain`, `StreamReasoning`, `OperationSummaries`).  
5. Align config comment category list with `CategoryRouting` + `AllCategories()`.

### P2 — operability

6. Drop counters on buses (`Stats`).  
7. Optional structured JSON sink for campaigns/CI.  
8. Expand rule glossary in Explainer from policy metadata.

## 4. Non-gaps (do not “fix” these)

| Observation | Why not a bug |
|-------------|----------------|
| Explainer not owned by Manager | Stateless; created per command is fine |
| Manager doesn’t own Glass Box | Avoids forcing Glass Box through opt-in master; tools/debug stay independent |
| Drop on full channel | Protects executive latency |
| Heuristic ClassifyError | Explicit design for UX when typed errors aren’t available |
| No `.mg` in package | Observability, not policy |
| Verbose default-off for StreamReasoning | Prevents flood |

## 5. Risk if gaps ignored

- **False confidence:** status table claims features that never fire.  
- **Split-brain UX:** Glass Box vs `/transparency` active ops disagree.  
- **Safety story incomplete:** users see “failed” tool lines without structured “why blocked” narratives.  
- **Maintenance drag:** unused Manager field on ShardManager confuses auditors (wiring skill will flag it).

## 6. Recommended acceptance tests for gap closure

1. Spawn shard with Manager enabled → `GetActiveExecutions` non-empty during run.  
2. Force `permitted` deny → `GetRecentViolations` non-empty without manual Report call.  
3. Set `JITExplain=true` → observable JIT explanation path or flag removed from status.  
4. Bus under artificial full channel → Stats.Drops increments (once implemented).
