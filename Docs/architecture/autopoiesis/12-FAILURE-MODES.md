# 12 — Failure Modes: Autopoiesis

> Last verified against codebase: **2026-07-13**

Concrete failures observed or implied by code, with mitigations.

## FM-01 — Safety policy missing / empty

**Symptom:** tools that should fail safety pass; init warn about `go_safety.mg`.  
**Cause:** `core.GetDefaultContent("go_safety.mg")` fails; `goSafetyPolicy = ""`.  
**Mitigation:** treat empty policy as fail-closed (gap); verify embed in CI; watch init logs.

## FM-02 — Safety rejection exhaustion

**Symptom:** `LoopResult.Success=false`, stage safety, error lists violations.  
**Cause:** LLM keeps emitting forbidden imports/calls.  
**Mitigation:** regenerate with feedback up to MaxRetries; inject learnings; templates with safer scaffolds.

## FM-03 — Thunderdome kill loop

**Symptom:** panic_maker_kill, tools rejected after MaxPanicRetries.  
**Cause:** brittle code (nil, OOM, race).  
**Mitigation:** feedback from `FormatBattleResultForFeedback`; raise code quality; adjust attack severity only carefully.

## FM-04 — Compile / tidy failure

**Symptom:** commit fails; CompileResult.Errors filled.  
**Cause:** invalid Go, missing modules, network for proxy, wrong GOOS/GOARCH.  
**Mitigation:** validateCode earlier; workspace replace directive; Yaegi fallback only for stdlib tools; operator ensures Go toolchain.

## FM-05 — Simulation / halt by Mangle

**Symptom:** “halted by Mangle policy” or simulation failure.  
**Cause:** max iters, stagnation, degradation, illegal transition.  
**Mitigation:** inspect local engine facts; reduce thrash; fix schemas_state load warnings (open-loop mode if schema missing).

## FM-06 — Ouroboros engine init panic

**Symptom:** process panic during `NewOuroborosLoop`.  
**Cause:** `mangle.NewEngine` error treated as fatal.  
**Mitigation:** ensure mangle defaults healthy; catch at Orchestrator construction in higher layers if needed.

## FM-07 — Kernel not attached

**Symptom:** no `tool_registered` facts; delegations always 0.  
**Cause:** `SetKernel` never called or nil.  
**Mitigation:** factory always sets bridge; tests use mocks; logs “No kernel attached”.

## FM-08 — Dual path under-hardened tool

**Symptom:** tool on disk without Thunderdome/safety depth of Ouroboros.  
**Cause:** chat `GenerateTool` or `ExecuteAction` light path.  
**Mitigation:** route through `ExecuteOuroborosLoop`; audit process.go.

## FM-09 — Registry / kernel desync

**Symptom:** binary exists, kernel query empty (or reverse).  
**Cause:** restore without SetKernel; assert failures; orphan binaries without source.  
**Mitigation:** restore skips orphans; `syncExistingToolsToKernel` on SetKernel; batch assert errors logged.

## FM-10 — Relative binary path

**Symptom:** execute error “must be absolute”.  
**Cause:** bad CompileResult path or manual registry edit.  
**Mitigation:** compiler writes absolute under CompiledDir; refuse relative.

## FM-11 — Tool subprocess hang

**Symptom:** long wait until context timeout.  
**Cause:** infinite loop in tool; missing timeout on caller.  
**Mitigation:** `ExecuteTimeout`, CommandContext; Thunderdome timeouts for arena.

## FM-12 — Learning store corruption / race

**Symptom:** lost learnings, flaky tests.  
**Cause:** concurrent RecordLearning without locks (guards exist).  
**Mitigation:** existing race tests; always use LearningStore APIs.

## FM-13 — Session tool spam

**Symptom:** many tools/session, disk growth.  
**Cause:** low confidence threshold, disabled caps.  
**Mitigation:** MaxToolsPerSession=3 default; MinToolConfidence=0.75.

## FM-14 — Prompt evolution atom pollution

**Symptom:** degraded prompts after auto-promote.  
**Cause:** low-quality judge, AutoPromote true.  
**Mitigation:** confidence threshold, interval, max atoms; disable auto-promote in sensitive envs.

## FM-15 — Agent specs without runtime

**Symptom:** agents created on disk but never fire.  
**Cause:** package writes specs; scheduling elsewhere incomplete.  
**Mitigation:** document ownership; wire to user-agent / shard systems intentionally.

## FM-16 — Panic during loop body

**Symptom:** StagePanic, stats.Panics++, recovered result.  
**Cause:** unexpected nil deref in stages.  
**Mitigation:** deferred recover; ouroboros_panic_test; fix root cause.

## Recovery order of operations

1. Read Autopoiesis logs for stage + error.  
2. Inspect `SafetyReport` / Thunderdome battle if present.  
3. Check `.nerd/tools` source + `.compiled`.  
4. Query parent kernel predicates.  
5. Retry via Ouroboros with refreshed learnings after fixing templates/policy.
