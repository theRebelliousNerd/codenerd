---
doc-class: governance
subsystem: browser
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: 6597099c
supersedes: []
---

# ADR-002: Partition browser_act Constitutional Permissions and Default-Dismiss Dialog Policy

## Context

In codeNERD's baseline policy (`internal/core/defaults/policy/constitution.mg:161-162`), `browser_act` is declared as a blanket `safe_action`:

```mangle
safe_action(/browser_observe).
safe_action(/browser_act).
```

When `browser_act` was confined to basic navigation, synthetic clicks, and text typing, this blanket permission was acceptable because the blast radius was limited to standard web navigation.

However, BrowserNERD 1.2.0 introduces two capabilities with significantly higher blast radius:
1. **File Uploads (`upload`)**: Setting file paths on `<input type="file">` elements via CDP can attach sensitive host files (such as source code, keys, or credentials) to remote servers if not strictly contained.
2. **JavaScript Dialog Confirmations (`dialog`)**: Web applications frequently guard irreversible operations behind `confirm()` or `prompt()` modals (for example, "Permanently delete repository?"). If `browser_act` can unilaterally confirm these modals under a blanket safe action, destructive changes could occur without operator consent.

Additionally, when an unhandled JavaScript dialog appears, Chrome freezes the entire JavaScript execution thread in that tab until the dialog is answered. If the agent does not immediately resolve the dialog, subsequent CDP commands hang until timeout.

## Decision

1. **Partition `browser_act` Constitutional Governance**:
   - The blanket `safe_action(/browser_act)` fact is removed from `constitution.mg`.
   - Routine non-destructive browser operations (`navigate`, `click`, `type`, `fill`, `key`, `history`, `hover`, and `sleep`) are granted safe action status conditionally based on their operation payload.
   - High-impact operations, specifically `upload` and armed dialog accepts, are classified as potentially dangerous and require distinct, explicit constitutional permission derivations (`permitted(Op, Reason, Context)`).
2. **Default-Deny Dialog Resolution**:
   - The CDP dialog listener (`Page.javascriptDialogOpening`) must answer dialogs immediately upon opening to prevent renderer freezing.
   - The default policy is strict dismissal (`accept: false`), answering with default-deny semantics.
   - A dialog is accepted (`accept: true`) if and only if an explicit policy rule or armed confirmation fact is active in the kernel for that specific session and dialog message.
3. **Universal Honeypot Gate Enforcement**:
   - The security check `guardElement` (`internal/browser/progressive_action.go:324`) must execute for all locator branches (`text`, `label`, `role`, and coordinate matches), preventing the model from circumventing honeypot derivations by substituting alternative selector types.

## Consequences

### Positive
- Closes a critical security loophole where the model could attach arbitrary files or approve destructive web modals without constitutional oversight.
- Eliminates renderer freezes caused by unhandled JavaScript dialogs.
- Guarantees consistent honeypot and trap detection regardless of the element resolution strategy.

### Negative
- Workflows that genuinely require file uploads or modal confirmations must have explicit constitutional rules configured in `constitution.mg`.
- Batches containing upload steps must be checked against constitutional rules prior to execution.

## Witness

**Witness:** Test `TestBrowserAct_UploadRequiresConstitutionalPermission` in `internal/core/defaults/policy/constitution_test.go` and rule partitioning in `internal/core/defaults/policy/constitution.mg`.
