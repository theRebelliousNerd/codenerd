# 07 - Context Management & Memory

## 1. Token Budget & Counting

### Token Budget Configuration

The default total budget is **200,000 tokens** (`types.go:47`), split across four categories:

| Category | Default | Percentage | Purpose |
|----------|---------|------------|---------|
| `CoreReserve` | 10,000 | 5% | Constitutional facts, schemas |
| `AtomReserve` | 60,000 | 30% | High-activation context atoms |
| `HistoryReserve` | 30,000 | 15% | Compressed history + recent turns |
| `WorkingReserve` | 100,000 | 50% | Current turn processing |

Budget can be overridden via `NewConfigWithBudget(totalBudget)` (`types.go:135-147`), which recalculates reserves as percentages of the new total. Negative or zero values silently default to 200k.

### TokenCounter Implementation

`TokenCounter` (`tokens.go:19-29`) uses a **~4 chars/token heuristic** calibrated for Claude's tokenizer:

- `CountString()` (`tokens.go:32-39`): Uses `utf8.RuneCountInString()` divided by `charsPerToken` (4.0). Returns `int(float64(runeCount) / 4.0)`.
- `CountFact()` (`tokens.go:42-67`): Adds 4 base tokens for predicate overhead, then per-argument estimates: `/name` atoms get `1 + len/4`, strings get full count + 2 for quotes, numbers get 2, bools get 1, unknown types get 3.
- `CountFacts()`, `CountScoredFacts()`, `CountTurn()`, `CountTurns()`: Aggregate helpers.

### Hard Enforcement Mode

`TokenBudget` (`tokens.go:147-162`) defaults to `hardEnforcement: true` (`tokens.go:169`).

- `Allocate(category, tokens)` (`tokens.go:186-230`): Per-category budget check. Returns `false` on overage (logs via `ContextDebug`). Unknown categories are rejected with a warning.
- `AllocateWithError()` (`tokens.go:234-240`): Wraps `Allocate()` and returns `ErrContextWindowExceeded` (`tokens.go:144`) on failure.
- `CheckTotalBudget()` (`tokens.go:244-253`): Aggregate check against `TotalBudget`. Logs at ERROR level on violation.
- `MustFitWithinBudget()` (`tokens.go:257-266`): Pre-flight check before allocation.
- `ShouldCompress()` (`tokens.go:300-308`): Triggers when `Utilization() >= CompressionThreshold`.

## 2. Context Compression

### ProcessTurn Flow (`compressor.go:637-787`)

`ProcessTurn()` is the central pipeline, protected by `c.mu.Lock()`:

1. **Extract atoms** from `ControlPacket` + pre-extracted atoms (`compressor.go:648-663`).
2. **Commit to kernel** via `AssertBatch()`, with per-atom fallback on batch failure (`compressor.go:665-682`).
3. **Mark new facts** for recency scoring; refresh campaign/issue activation contexts (`compressor.go:684-688`).
4. **Process memory operations** (promote_to_long_term, forget, store_vector) (`compressor.go:690-697`).
5. **Create CompressedTurn** - surface text is discarded, only `IntentAtom`, `FocusAtoms`, `ResultAtoms`, and `MangleUpdates` are kept (`compressor.go:699-723`).
6. **Add to sliding window** (`compressor.go:726`).
7. **Recalculate budget** and check utilization (`compressor.go:736-741`).
8. **Trigger compression** if `shouldCompress()` returns true (`compressor.go:743-752`).
9. **Prune old turns** from sliding window (`compressor.go:754-759`).
10. **Persist state** to SQLite (best-effort) + log top 50 hot facts for activation analytics (`compressor.go:770-784`).

### Compression Trigger Threshold

Compression fires at **60% utilization** (`types.go:57`):
```
CompressionThreshold: 0.60
```
The `shouldCompress()` method (`compressor.go:811-815`) delegates to `budget.ShouldCompress()` which compares `Utilization() >= CompressionThreshold`.

### How User Input Is Compressed

Surface text is never stored. The `compress()` function (`compressor.go:867-955`):

1. Determines turns to compress (everything outside the `RecentTurnWindow` of 5 turns).
2. Collects **key atoms** with a hard limit of **64** (`compressor.go:879`): `c.collectKeyAtoms(turnsToCompress, 64)`.
3. Generates LLM-based summary via `generateSummary()` (`compressor.go:884`).
4. Falls back to `generateSimpleSummary()` on LLM failure (`compressor.go:887-888`).
5. Enforces `TargetCompressionRatio` (100:1) - if summary exceeds token budget, tries atom serialization; if that also exceeds, trims the summary (`compressor.go:902-921`).

### Rolling Summary Mechanism

`RollingSummary` (`types.go:316-333`) accumulates `HistorySegment` entries. Each segment records:
- Start/end turn numbers, summary text, key atoms, token metrics, compression ratio.

After each compression, `rebuildRollingSummaryText()` (`compressor.go:1024-1042`) concatenates all segment summaries into a single text block prefixed with total turns and overall ratio.

### State Persistence and Rehydration

`LoadState()` (`compressor.go:1294-1338`): Restores `sessionID`, `turnNumber`, `rollingSummary`, `recentTurns`, and hot facts. Hot facts are re-asserted into the kernel (deduplication via `String()` key). **No validation of the incoming `CompressedState` structure** - nil fields or corrupted data flow directly into the compressor.

## 3. Spreading Activation

### The 9 Scoring Components (`activation.go:417-431`)

Each fact is scored by summing 9 independent components:

| # | Component | Function | Max | Description |
|---|-----------|----------|-----|-------------|
| 1 | `base` | `computeBaseScore()` (:454-466) | 100 | Predicate priority (corpus > config > default 50) |
| 2 | `recency` | `computeRecencyScore()` (:470-494) | 50 | Step decay: <1m=50, <5m=30, <30m=10, else 0 |
| 3 | `relevance` | `computeRelevanceScore()` (:497-617) | unbounded | Intent target match +40, focused paths +30, focused symbols +20, verb-predicate boosts (up to +50 per verb) |
| 4 | `dependency` | `computeDependencyScore()` (:620-661) | 40 (capped) | Forward deps inherit 30% of priority; reverse deps add 5pts each; symbol graph spreading; capped at 40.0 |
| 5 | `campaign` | `computeCampaignScore()` (:663-727) | 60 (capped) | Campaign ID +25, phase +30, task +35, files +20, symbols +15, campaign predicates up to +50; capped at 60.0 |
| 6 | `session` | `computeSessionScore()` (:729-739) | 15 | Flat +15 bonus if fact was added this session |
| 7 | `issue` | `computeIssueScore()` (:741-800+) | unbounded | Issue ID +30, keyword weight*50, mentioned files +40, tiered files (Tier1=+50 to Tier4=+10), error types +35, expected tests +45 |
| 8 | `feedback` | `computeFeedbackScore()` | varies | Learned predicate usefulness from historical LLM feedback via `ContextFeedbackStore` |
| 9 | `backReference` | `computeBackReferenceScore()` | varies | Boost for facts from referenced turns in follow-up questions; uses `BackReferenceActivationContext` |

### Threshold Filtering (`activation.go:334-353`)

`FilterByThreshold()` removes facts below the activation threshold of **105.0** (`types.go:63`).

Design rationale: `Base(50) + Recency(50) = 100`. Threshold of 105 requires **at least some relevance boost** beyond just being recent, preventing irrelevant-but-recent facts from polluting context.

`SelectWithinBudget()` (`activation.go:358-379`) **always applies threshold filtering first** (defensive design), then greedily fills the token budget in score-descending order.

### Score Computation Details

- `ScoreFacts()` (`activation.go:241-303`): Entry point. Rebuilds symbol graph, scores all facts, sorts descending. Extracts intent verb (arg[2]) for verb-specific feedback lookup.
- No mutex protection on `ActivationEngine` - relies on callers (the `Compressor`) to hold `c.mu`.
- The `factTimestamps` map grows unboundedly - no cleanup mechanism for old entries.

## 4. Memory Tiers & SQLite

### The 4-Tier Architecture

| Tier | Table | Description |
|------|-------|-------------|
| **RAM** | In-memory `FactStore` (kernel) | Working facts for current session |
| **Vector** | `vectors` (`local_core.go:138-147`) | Semantic search via SQLite + optional sqlite-vec extension |
| **Graph** | `knowledge_graph` (`local_core.go:150-164`) | Entity-relation-entity triples with weights |
| **Cold** | `cold_storage` (created separately) | Persistent facts with access tracking (last_accessed, access_count) + archival tier |

Additional supporting tables:
- `world_files` (`local_core.go:169-181`): Per-file metadata cache (path, lang, size, modtime, hash, fingerprint)
- `world_facts` (`local_core.go:183-196`): Cached world model facts (fast/deep depth) with composite PK
- `activation_log` (`local_core.go:201-209`): Spreading activation score history
- `session_history` (`local_core.go:213-226`): Conversation turns with UNIQUE(session_id, turn_number)
- `compressed_states` (`local_core.go:230-241`): Serialized compression state per session/turn
- `task_verifications` (`local_core.go:244-264`): Learning from retry loops
- `reasoning_traces` (`local_core.go:267-294`): Shard LLM interaction traces with embeddings
- `review_findings` (`local_core.go:297-300`): Persistent review history

### SQLite Configuration (`local_core.go:72-89`)

```go
db.SetMaxOpenConns(1)          // Single writer - serialized access
db.SetMaxIdleConns(1)
PRAGMA busy_timeout = 5000     // 5s wait on lock contention
PRAGMA journal_mode = WAL      // Write-Ahead Logging for crash recovery
PRAGMA synchronous = NORMAL    // 5-10x write speedup vs FULL (safe with WAL)
```

`NewLocalStore()` (`local_core.go:58-132`): Creates directory, opens DB, initializes schema, detects sqlite-vec extension, initializes TraceStore, backfills content hashes.

### Parameterized Queries vs fmt.Sprintf

The schema initialization uses string-literal DDL (safe - no user input). Data access methods use `db.Exec` and `db.Query` with parameterized queries (e.g., `StoreCompressedState` uses `?` placeholders). The `StoreFact` API serializes args as JSON before insertion, providing a natural injection barrier.

## 5. CHAOS FAILURE PREDICTIONS

### P1: 100MB Input Hits Token Counter - **HIGH**

**Vector:** Pass a 100MB string into `TokenCounter.CountString()` (`tokens.go:32-39`).

`utf8.RuneCountInString()` is O(n) and must scan the entire string. For 100MB UTF-8, this consumes ~100MB working memory and produces an `int` result of ~25,000,000 tokens. This value propagates into `ProcessTurn()` where `originalTokens` (`compressor.go:700`) overflows the budget, triggering compression on every turn. The real danger: `CountString` is called on `turn.SurfaceResponse` and `turn.UserInput` **before** any budget check, so the full 100MB is already in memory. No input size validation exists at the `ProcessTurn` entry point (`compressor.go:637`).

### P2: Corrupted Compressed State Crashes LoadState - **CRITICAL**

**Vector:** Inject malformed JSON into `compressed_states.state_json` in SQLite, then trigger session rehydration.

`LoadState()` (`compressor.go:1294-1338`) performs **zero validation** on the incoming `*CompressedState`. A nil `RollingSummary.Segments` slice is safe (Go nil slices iterate fine), but a `HotFacts` entry with a corrupted `Fact` (e.g., nil `Args` slice) will panic when `Assert()` calls `f.String()` inside the kernel (`compressor.go:1315`). The deduplication loop at `compressor.go:1310-1311` calls `f.String()` on every existing fact - if the kernel contains a fact with nil fields, this panics. No `recover()` wrapper protects LoadState.

### P3: SQLite WAL Grows Unbounded - **MEDIUM**

**Vector:** Sustained high-throughput writes without reader checkpointing.

WAL files grow when there are long-running readers blocking checkpoint advancement. With `MaxOpenConns=1` (`local_core.go:77`), the single connection alternates between reading and writing. However, if the `activation_log` table receives 50 inserts per turn (`compressor.go:776-783`) across thousands of turns without `MaintenanceCleanup()`, the activation_log table itself grows unbounded. The WAL stays manageable due to auto-checkpointing, but the **table data** bloats indefinitely. No automatic garbage collection runs unless `MaintenanceCleanup()` is explicitly called with `CleanActivationLogDays`.

### P4: Activation Score Overflow with Adversarial Facts - **HIGH**

**Vector:** Inject thousands of facts with predicates matching all boost categories simultaneously.

The `computeIssueScore()` (`activation.go:741-800`) iterates over `issueContext.Keywords` map with `weight * 50.0` per keyword match. An attacker who controls issue keywords (e.g., via a crafted GitHub issue body) can set 100 keywords that all appear in a single fact string. Score: `30 + (100 * 50) + 40 + 50 + 35 + 45 = 5200`. Unlike `computeDependencyScore` (capped at 40) and `computeCampaignScore` (capped at 60), `computeIssueScore` and `computeRelevanceScore` have **no cap**. The `Total()` function (`activation.go:429-431`) sums all 9 components with no ceiling. At `float64` precision this won't overflow, but it can produce scores so large they dominate the sorted list, suppressing all legitimate context.

### P5: Context Compression LLM Call Fails Repeatedly - **MEDIUM**

**Vector:** Network partition or LLM API outage during long session.

When `generateSummary()` fails (`compressor.go:885-889`), the system falls back to `generateSimpleSummary()`. This fallback produces a significantly lower-quality summary but doesn't fail. However, the `TargetCompressionRatio` enforcement (`compressor.go:902-921`) may then trim this already-poor summary further. Repeated failures degrade context quality progressively - each compression cycle loses more information. The system has **no retry logic** and **no circuit breaker** on the LLM call. Additionally, the error is logged as Warn, not Error, making it easy to miss in monitoring.

### P6: Feedback Store Poisoning Suppresses Important Context - **HIGH**

**Vector:** Manipulate `ContextFeedbackStore` to assign negative/zero feedback scores to critical predicates like `diagnostic`, `test_state`, or `security_violation`.

The `computeFeedbackScore()` (`activation.go:443`) adds a learned usefulness score. If an attacker can write to the feedback store (e.g., by controlling LLM responses that generate feedback), they can systematically downrank safety-critical predicates. Since the feedback score is additive in `Total()`, a sufficiently negative feedback score can push `security_violation` facts (base=100) below the activation threshold of 105.0, causing the system to **silently drop security findings from context**.

### P7: Concurrent ProcessTurn Calls - **MEDIUM**

**Vector:** Multiple goroutines calling `ProcessTurn()` simultaneously (e.g., parallel shard outputs).

`ProcessTurn()` holds `c.mu.Lock()` (`compressor.go:641-642`), so concurrent calls are serialized at the Compressor level. However, `ActivationEngine` has **no mutex** of its own. While the Compressor's lock protects access during normal flow, `ScoreFacts()` (`activation.go:241`) directly mutates `ae.state.ActiveIntent` and rebuilds `ae.symbolGraph` without synchronization. If any code path calls `ScoreFacts()` outside the Compressor's lock (e.g., `SelectWithinBudget` called independently), concurrent map writes to `symbolGraph` (`activation.go:315`) will panic with a Go runtime fatal: concurrent map writes.

### P8: factTimestamps Map Memory Leak - **HIGH**

**Vector:** Long-running session (hours/days) with continuous fact churn.

`RecordFactTimestamp()` (`activation.go:399-403`) appends to `factTimestamps` and `sessionFacts` maps with no eviction policy. Each `factKey()` generates a string representation of the fact. Over a 24-hour session with aggressive world model updates (file_topology, symbol_graph facts changing on every save), these maps can accumulate hundreds of thousands of entries. The `ClearState()` method exists but is only called on `Reset()` (`compressor.go:1352`), not during normal operation. Memory grows monotonically throughout the session.

### P9: Integer Overflow in Token Budget Arithmetic - **MEDIUM**

**Vector:** Set `TotalBudget` to `math.MaxInt` and then allocate.

`NewConfigWithBudget()` (`types.go:135-147`) computes reserves as `totalBudget * 50 / 100`. For `totalBudget = math.MaxInt64`, this overflows to a negative number. Subsequently, `Allocate()` comparisons like `tb.used.working+tokens > tb.config.WorkingReserve` (`tokens.go:218`) compare a positive number against a negative reserve, always returning false - **all allocations succeed regardless of actual usage**. The budget becomes effectively infinite, defeating the entire context window enforcement system.

---

### Summary Table

| # | Prediction | Severity | Primary File:Line |
|---|-----------|----------|-------------------|
| P1 | 100MB input overwhelms token counter | HIGH | `tokens.go:32-39`, `compressor.go:700` |
| P2 | Corrupted CompressedState panics on LoadState | CRITICAL | `compressor.go:1294-1338` |
| P3 | SQLite activation_log bloats unbounded | MEDIUM | `compressor.go:776-783`, `local_core.go:201-209` |
| P4 | Uncapped issue/relevance scores suppress context | HIGH | `activation.go:741-800`, `activation.go:429-431` |
| P5 | Repeated LLM compression failures degrade context | MEDIUM | `compressor.go:884-889` |
| P6 | Feedback poisoning drops security facts | HIGH | `activation.go:443`, `types.go:107-109` |
| P7 | ActivationEngine concurrent map write panic | MEDIUM | `activation.go:315`, `activation.go:241` |
| P8 | factTimestamps map leaks memory indefinitely | HIGH | `activation.go:399-403` |
| P9 | Integer overflow disables budget enforcement | MEDIUM | `types.go:142-145`, `tokens.go:218` |
