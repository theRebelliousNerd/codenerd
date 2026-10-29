# 12 — Failure Modes: Northstar

> Last verified against codebase: 2026-07-13  
> Package: `internal/northstar`

## FM1 — Vision defined in wizard but Guardian has none

| | |
|--|--|
| **Symptom** | `/alignment` always skipped; campaigns never vision-check; Warn on init |
| **Cause** | Wizard writes `.nerd/northstar.json`; Guardian reads SQLite only |
| **Mitigation today** | Manually ensure `Store.SaveVision` / `UpdateVision` from same content |
| **Proper fix** | Single write path (see gap analysis P0) |

## FM2 — Kernel never receives northstar facts

| | |
|--|--|
| **Symptom** | Policy/injectable context as if `!northstar_defined()` |
| **Cause** | `SetParentKernel` not called (primary `session_boot.go` path) or Initialize before kernel set |
| **Mitigation** | Use shared-boot pattern; call `SetParentKernel` then `Initialize` or `UpdateVision` to refresh |

## FM3 — LLM returns free-form prose

| | |
|--|--|
| **Symptom** | Score stuck ~0.7, Result warning, explanation “Unable to parse…” |
| **Cause** | Parser requires `SCORE:` / `RESULT:` lines |
| **Mitigation** | Defaults soft; does not hard-block. Re-run or improve prompt adherence |

## FM4 — LLM transport error

| | |
|--|--|
| **Symptom** | Warning, score 0.7, explanation includes error |
| **Cause** | `CompleteWithSystem` fails |
| **Mitigation** | Non-fatal; check still persisted |

## FM5 — Campaign blocked at start

| | |
|--|--|
| **Symptom** | Error: goal does not align with vision |
| **Cause** | Alignment result `blocked` after LLM/threshold |
| **Mitigation** | Revise campaign goal or vision; or risk toggle disable northstar (config) |

## FM6 — Protected campaign without observer

| | |
|--|--|
| **Symptom** | Risk gate blocks: northstar observer not configured |
| **Cause** | Campaign package risk_scoring, not Guardian itself |
| **Mitigation** | Wire `SetNorthstarObserver` before start |

## FM7 — Store open / schema failure

| | |
|--|--|
| **Symptom** | Boot skips Northstar handler; alignment cmd returns error |
| **Cause** | Directory permissions, sqlite driver/CGO, disk full |
| **Mitigation** | `NewStore` returns error; callers log and continue without guardian |

## FM8 — Dual concurrent Guardians on same DB

| | |
|--|--|
| **Symptom** | Interleaved state counters; possible SQLITE_BUSY under load |
| **Cause** | Boot guardian + ephemeral `/alignment` guardian both open DB |
| **Mitigation** | SQLite + mutex per process; still not a single in-process singleton |
| **Note** | ProfileHot pragmas help; not multi-writer perfect |

## FM9 — High-impact false positives/negatives

| | |
|--|--|
| **Symptom** | Too many or too few auto-checks |
| **Cause** | Glob/prefix semantics of `matchesHighImpactPath` |
| **Mitigation** | Adjust `GuardianConfig.HighImpactPaths`; tests cover nested `*.mg` style cases |

## FM10 — Stale overall_alignment after manual DB edit

| | |
|--|--|
| **Symptom** | State looks wrong |
| **Cause** | Bypassed Store APIs |
| **Mitigation** | Always use Store methods; EWMA only updates on `RecordAlignmentCheck` |

## FM11 — Mitigation text lost in Mangle

| | |
|--|--|
| **Symptom** | Policy cannot distinguish mitigations |
| **Cause** | `ToFacts` always emits `/mitigation` |
| **Mitigation** | Use description fields elsewhere or fix encoding |

## FM12 — Nil store on Guardian methods

| | |
|--|--|
| **Symptom** | Panic or error on Initialize/Check paths |
| **Cause** | `NewGuardian(nil, …)` allowed |
| **Mitigation** | Tests allow construct; do not call methods without store in production |

## Recovery summary

| Mode | Auto-recover? |
|------|----------------|
| Soft alignment parse/LLM fail | Yes (continues work) |
| Hard campaign block | No (must change goal/vision) |
| Missing vision | Yes for availability; no for enforcement |
| Missing kernel wire | Silent functional loss |
| Dual store | Silent semantic split |
