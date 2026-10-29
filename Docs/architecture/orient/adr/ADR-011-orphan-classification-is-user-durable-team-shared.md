---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# ADR-011: Orphan Classification Is the User's, Durable, and Team-Shared

## Context

When an agent encounters code that is not accounted for by any specification (orphan code), conventional AI tools make one of two catastrophic mistakes:
1. They hallucinate justifications, assuming all existing code is intentional production architecture.
2. They aggressively delete or refactor unfamiliar code, destroying work-in-progress prototypes or undocumented features.

In codeNERD today:
- `kernelClarification` (`cmd/nerd/chat/process_dream_delegation.go:26-62`) and `clarification.mg:42-88` are wired exclusively for ambiguous user intent strings, not for code classification.
- No schema or persistence mechanism exists to record whether un-specced code is a feature, an experiment, or trash.
- Any decision made by a user in chat is lost when the session terminates, forcing repeated questions on subsequent runs.

## Decision

1. **Operator Authority via Clarification Seam**:
   The operator is the sole authority on orphan code classification. When an orphan code unit is derived (excluding tests, entry points, and excluded tree treatments), the kernel routes the question through the existing `kernelClarification` dialogue.
2. **Controlled Cognitive Budget**:
   Questions are grouped by package directory and strictly capped per session via `config_param(/orient_orphan_question_cap, 5)`. The model provides a pre-selected best guess based on code structure and commit history.
3. **Five Canonical Classifications**:
   The operator chooses from five explicit semantic categories:
   - `/feature`: Legitimate feature missing its spec $\rightarrow$ schedules specification creation.
   - `/experiment_develop`: Active prototype $\rightarrow$ tags unit as an active experiment.
   - `/experiment_park`: Suspended prototype $\rightarrow$ parks code outside production readiness gates.
   - `/failed_experiment`: Abandoned work $\rightarrow$ schedules safe deprecation.
   - `/trash`: Obsolete cruft $\rightarrow$ schedules immediate CodeDOM cleanup transaction.
4. **Durable, Team-Shared Persistence**:
   Operator answers are written to `.nerd/orientation/answers.json`, a file committed to git version control.
5. **Boot Re-Assertion**:
   Go sensors load `answers.json` on boot, asserting durable `code_classified(Unit, Answer)` facts into the kernel. Durable facts permanently override model guesses and prevent re-asking questions for previously resolved units.

## Consequences

- **Positive**: Prevents accidental destruction of experimental or un-specced features; eliminates repetitive questioning across sessions and team members; ensures alignment between human intent and automated cleanup.
- **Negative**: Adds git-tracked configuration files to the repository; requires developer attention during onboarding.
- **Risks**: Developers might skip questions. Mitigated by intelligent pre-selected defaults and the ability to defer questions without blocking immediate commands.

## Witness

**Witness:** `test:TestOrphanClassification_DurableAndTeamShared`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Existing clarification UI seam | `kernelClarification` in `cmd/nerd/chat/process_dream_delegation.go:26-62`. |
| Declarative clarification rules | `clarification_question` and `clarification_option` in `internal/core/defaults/policy/clarification.mg:42-88`. |
| Durable orphan classification target test | Planned test `TestOrphanClassification_DurableAndTeamShared` in `internal/orient/orphan_test.go`. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The architectural decision is accepted, but the orphan clarification generator and `.nerd/orientation/answers.json` loader have not yet landed in production source code as of commit `bb7bafac`. Status flips to `implemented` once `TestOrphanClassification_DurableAndTeamShared` passes.
