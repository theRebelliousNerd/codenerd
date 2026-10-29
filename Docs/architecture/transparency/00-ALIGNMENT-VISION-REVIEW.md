# transparency — Alignment & Vision Review

> Last verified: 2026-07-13  
> Scores are evidence-based against current `internal/transparency/` + known consumers.

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully realized and wired |
| 4 | Implemented; minor gaps or optional paths incomplete |
| 3 | Core present; important wiring or depth partial |
| 2 | Scaffold / partial; not production-complete for the claim |
| 1 | Documented intent only |
| 0 | Contradicts north star or absent where claimed |

## Dimensions

### 1. LLM creative / Logic executive split — **4 / 5**

**Evidence**

- Package does not choose actions; it formats traces (`Explainer` → `mangle.DerivationTrace`), reports safety blocks, and streams subsystem events.  
- Decision narrative looks for `next_action` roots (`explainer.go` `ExplainDecision`).  
- QuickExplain knows `permitted`, `user_intent`, `clarification_needed` as *descriptions*, not as policy.

**Gap:** Heuristic string classification for errors/safety can mis-attribute causes that only the kernel truly knows. Provenance-backed `/explain` is more accurate for some rules (`cmd/nerd/chat/cmd_explain.go` notes this vs `/why`).

### 2. Constitutional safety visibility — **4 / 5**

**Evidence**

- `SafetyReporter` + `ExplainSafetyAction` produce remediation pointing at `/shadow`, `/why`, `/query permitted`.  
- Error category `ErrorCategorySafety` for permission/constitutional language.  
- doc.go lists “Safety gate explanations” as a first-class feature.

**Gap:** Automatic feed from VirtualStore / policy deny path into `ReportSafetyViolation` is not universally wired; much classification is pattern-based on action/target strings. `TransparencyManager.ReportSafetyViolation` is gated by `Enabled && SafetyExplanations`.

### 3. Non-intrusive execution path — **5 / 5**

**Evidence**

- Event buses drop on full channel (`default:` branches in `Emit` / `EmitImmediate` / `ToolEventBus.Emit`).  
- Disabled bus returns immediately.  
- TransparencyManager methods no-op when master disabled or feature flags off.  
- Package does not import `cmd/nerd` or assert kernel facts.

### 4. Opt-in depth vs always-on tools — **5 / 5**

**Evidence**

- `ToolEvent` docs: “ALWAYS displayed regardless of debug mode.”  
- `DefaultTransparencyConfig()` sets `Enabled: false` while feature subflags default true (ready when master on).  
- Glass Box bus enabled at boot for producers; TUI `initGlassBox` controls subscription/display.  

Aligns with “magic visible” without flooding beginners if display is gated—while tools stay honest.

### 5. JIT / prompt transparency — **2 / 5**

**Evidence**

- Category `CategoryJIT` and config `JITExplain` exist.  
- Status table prints JIT Explain flag.

**Gap:** This package does not implement JIT atom selection explainers; JIT subsystem + chat must emit events / render. Flag is largely **config surface without package-local enforcement**.

### 6. Observability under concurrency — **4 / 5**

**Evidence**

- Sequence IDs (`atomic.Uint64`), batch window 50ms / limit 20, verbose immediate path.  
- Subscriber buffer 512 for multi-shard storms.  
- ShardObserver / TransparencyManager use `sync.RWMutex`.

**Gap:** Drop-on-full is correct for non-blocking but means **silent loss** under extreme load with no drop counter (Stats has `TotalEmitted` but not drops).

### 7. Test / maintainability — **4 / 5**

**Evidence**

- 9 test files including large `transparency_comprehensive_test.go` covering classifier, observer, safety, manager.  
- Event bus filter, flush, unsubscribe, clear-turn covered.

**Gap:** Little package-local integration testing against real VirtualStore emit path (covered more in `cmd/nerd/chat` / core tests).

### 8. Wiring completeness vs inventory — **3 / 5**

**Evidence of strong wiring**

- Boot creates `TransparencyManager`, `GlassBoxEventBus`, `ToolEventBus` (`session_boot.go`, `session_shared_boot.go`).  
- VirtualStore emits tool + routing events.  
- ShardManager emits Glass Box shard events; stores TransparencyManager as `any`.  
- System router emits ToolEvents.  
- Chat `/transparency`, `/glassbox`, `/why` + explainer.

**Evidence of partials**

- `SetTransparencyManager` stores `any` and does not call `StartShard`/`UpdateShardPhase` in manager spawn path observed.  
- Live shard UX is **Glass Box events**, not `ShardObserver` phase state machine.  
- Config flags `StreamReasoning`, `OperationSummaries`, `JITExplain` under-wired in-package.

### 9. North-star communication (“why not just what”) — **4 / 5**

**Evidence**

- Explainer rule-name glossary (permission_gate, commit_barrier, …).  
- Safety FormatViolation sections “Why was this blocked?” / “How to proceed”.  
- ClassifiedError remediation lists.

**Gap:** Rule glossary is a fixed map—unknown rules fall back to `rule 'name'`. No dynamic loading of rule docs.

## Summary table

| Dimension | Score |
|-----------|------:|
| Logic executive / LLM creative | 4 |
| Constitutional safety visibility | 4 |
| Non-intrusive path | 5 |
| Opt-in vs always-on tools | 5 |
| JIT explain depth | 2 |
| Concurrent observability | 4 |
| Tests | 4 |
| End-to-end wiring | 3 |
| Why-not-what narratives | 4 |
| **Mean (approx.)** | **~3.9** |

## Verdict

**Living, production-relevant package**—not a stub. Strongest on bus design and non-intrusive emission; weakest on full consumption of every config flag and on coupling `TransparencyManager`/`ShardObserver` into the real shard lifecycle (Glass Box partially supersedes that path).

Alignment with codeNERD north star: **good**. Primary improvement is **wiring honesty** (use or demote dormant flags/APIs) rather than rewriting the core design.
