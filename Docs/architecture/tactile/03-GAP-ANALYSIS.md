# tactile — Gap Analysis

> Last verified: **2026-07-13**

## Method

Compare vision ([01-VISION.md](01-VISION.md)) and north star to living code. Distinguish **real gaps** from **intentional non-goals**.

## Spec vs reality matrix

| Vision item | Reality | Gap? |
|-------------|---------|------|
| All shell via Executor | Yes when callers use it | **Call-site discipline** |
| Policy-before-motor | VirtualStore path yes; bare executor constructors free | **Medium** — process/convention gap |
| Fact completeness when logger wired | Strong event→fact mapping | **Low** — wiring not always on |
| Sandbox spectrum | Direct/Docker/NS/Firejail/Job/cgroup | **Partial** — default Composite only none+docker |
| Platform realism | Solid per-OS code | **Low–Medium** — GetPlatformExecutor asymmetry Windows vs Darwin |
| Structured results | ExecutionResult rich | **None** |
| File motor + facts | FileEditor complete | **Low** — FileOpPatch emits no facts |
| python/swebench layers | Implemented | **Medium** — external adoption thin |
| Output analyzers multi-lang | Go-centric | **Low** intentional for now |
| No local .mg | Correct | **n/a** — decls global |

## Prioritized gaps

### P0 — correctness / safety process

| ID | Gap | Why it matters | Suggested direction |
|----|-----|----------------|---------------------|
| G-P0-1 | Callers can construct `NewDirectExecutor()` and run shell **outside** VirtualStore permission | Bypasses constitutional default-deny | Document contract; prefer factory through VS; audit callers |
| G-P0-2 | Chat boot uses Direct, not always audited Composite (`initModernExecutor` parallel path) | Execution facts may not hit kernel on primary UX path | Unify boot on modern audited executor |

### P1 — capability honesty

| ID | Gap | Why | Direction |
|----|-----|-----|-----------|
| G-P1-1 | Composite never auto-registers namespace/firejail | Requesting those modes falls through to **default direct** silently | Register when available or fail closed in Validate at composite level |
| G-P1-2 | Silent fallback to defaultExecutor when mode missing from map | Sandbox request may execute unsandboxed | Fail hard or emit `execution_blocked` |
| G-P1-3 | Windows `GetPlatformExecutor` ignores Docker availability for return type | Platform “best” less useful on Windows | Align with Darwin Composite behavior |
| G-P1-4 | RetryExecutor delay not real sleep | Retries may spin | Use `time.After` + ctx |

### P2 — integration depth

| ID | Gap | Why | Direction |
|----|-----|-----|-----------|
| G-P2-1 | Not all audit predicates Decl’d/consumed in policy | Facts without rules are telemetry only | Audit Decl vs ToFacts catalog |
| G-P2-2 | docker stats not used for ResourceUsage | Docker path has SupportsResourceUsage false | Optional stats parse |
| G-P2-3 | PersistentDocker idle timeout config unused for eviction | Config field vs behavior drift | Implement idle reaper or drop field |
| G-P2-4 | SWE-bench harness not first-class CLI command surface | Benchmark path harder to operate | Optional CLI later |

### P3 — polish

| ID | Gap |
|----|-----|
| G-P3-1 | OutputAnalyzer Go-only |
| G-P3-2 | FileOpPatch no facts |
| G-P3-3 | Combined stdout/stderr not true interleave (concatenate) |
| G-P3-4 | Package README structure mentions `executor.go` legacy SafeExecutor — file not present in tree |

## Non-gaps (do not “fix”)

| Item | Why not a gap |
|------|----------------|
| No permission engine inside tactile | North star: executive is kernel/VS |
| No prompt atoms | Not LLM-facing |
| No Vectryx coupling | Correct isolation |
| Success=true on non-zero exit | Documented intentional semantics |
| Env allowlist vs full environment | Security feature |

## Gap summary scorecard

| Area | Status |
|------|--------|
| Core execute path | Strong |
| Sandbox default composition | Weak-moderate |
| Fact generation | Strong |
| Fact consumption | Moderate |
| Boot wiring | Moderate |
| Platform depth | Strong (code), uneven (selection) |
| Benchmark stack | Strong library / weak productization |
