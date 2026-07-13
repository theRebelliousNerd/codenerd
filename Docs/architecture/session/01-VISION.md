# 01 — Vision: session execution

> Last verified: 2026-07-13  
> Package: `internal/session`

## 1. Product role

Session is the **Universal Execution Loop** for codeNERD: one clean Process path that can power interactive chat turns, CLI-spawned workers, campaign tasks, and specialist consultations.

Vision statement:

> Every agent specialization is a **JIT configuration + prompt atom set**, not a new Go class. Every side effect is **kernel-permitted**. Every parallel worker has **isolated conversation state**.

## 2. Target architecture

```
┌────────────── interactive surface ──────────────┐
│  CLI chat / Cobra / campaign orchestrator        │
└───────────────────────┬──────────────────────────┘
                        │ TaskRequest | Process(input)
┌───────────────────────▼──────────────────────────┐
│  TaskExecutor (JITExecutor)                      │
│    ├─ inline CloneForTask.ProcessWithIntent      │
│    └─ Spawner → SubAgent.Run                     │
└───────────────────────┬──────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────┐
│  Executor                                        │
│    observe → orient → JIT → LLM ↔ tools → respond│
└───┬─────────────┬──────────────┬─────────────────┘
    │             │              │
 kernel       tools.Global   VirtualStore gate
 (policy)     Ouroboros      (preflight/validate)
```

## 3. Behavioral targets

1. **Single loop** for all personas (coder, tester, reviewer, researchers, specialists).  
2. **Default deny** for tools when safety is on and permission cannot be proven.  
3. **Preset intents** for machine-delegated work (no perception thrash).  
4. **Dream mode** always isolated (subagent) so speculation cannot bleed.  
5. **Piggyback and native tool protocols** both first-class, including multi-turn where providers allow.  
6. **Durable session memory** for cross-session continuity (turns + compressed state).  
7. **Ouroboros missing tools** assert into policy so generation systems can respond.

## 4. Non-goals

- Not a second policy engine (do not reimplement Mangle in Go).  
- Not a UI layer (CLI owns presentation).  
- Not a storage engine (persister is an interface).  
- Not domain-specific coding heuristics (those live in prompt atoms / skills).

## 5. Success metrics (qualitative)

| Metric | Target |
|--------|--------|
| Domain shard Go LOC for personas | Near-zero new hardcoded shards |
| Safety bypass paths | Zero when EnableSafetyGate |
| Planning-only false completes on mutation intents | Mitigated via intent_requires_tool_call |
| Cross-task history contamination | None (CloneForTask / SubAgent) |
| Spawn capacity races | Pending reservation + recheck |

## 6. Evolution path (as encoded in comments)

1. Consumers already on `TaskExecutor` instead of raw ShardManager spawn.  
2. Residual ShardManager remains for profiles/system shards — not task execution.  
3. Future: complete Piggyback multi-iter loop; wire memory ops to cold storage; richer completion signaling.
