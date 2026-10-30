---
doc-class: deep-dive
subsystem: working-context-admission
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: 486db57ce490302c65d7f484e7f6070ee0e70862
verification-method: static-source-inspection
---

# Tool output is not an archival receipt

## Decision and evidence status

Keep the full-request input guard and durable observation archive, but stop recognizing an already-archived result by its text prefix. The final fitter currently treats any tool result beginning `Observation archived:` as ineligible for archival. That string can also occur at the beginning of a genuine tool result. Under the conditions below, a request that could fit by replacing that result with its real recall handle is instead refused.

This is a narrow representation/continuation defect, not an authorization bypass, observed incident, universal failure, or measured efficiency gain. Inspection only: no tests, runtime execution, provider inference, profiling, or implementation were performed.

Source freeze: main `486db57ce490302c65d7f484e7f6070ee0e70862`; research branch `research-main-20261030-0211` at `d613a1c38be5ec3b79e7e134a224d5910dbc3ac5`, read on 2026-10-30 around 03:09–03:11 UTC.

## Production trace

1. A declared workspace creates the working loop, archive and recall capability, and records the serving client's input cap. [Working-loop installation](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/working_context.go#L231-L297).
2. A successful `run_command` returns stdout directly, appending stderr only when present. It does not reserve or escape the archival prefix. This is a concrete producer when that command is actually granted and passes its normal gates; the finding does not require granting shell access to a model that lacks it. [Command output](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/tools/shell/execute.go#L327-L358). The built-in `cat` fallback likewise copies file bytes into its result rather than adding line numbers. [Fallback](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/tools/shell/builtins.go#L312-L334). Ordinary `read_file`, by contrast, adds numbered lines, so simply reading a file beginning with the string is not an equivalent witness through that tool.
3. Session execution records the returned output at the working-observation boundary; successful batches place `out` in `ToolResult.Content`. [Recording](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/executor_tools.go#L1095-L1111), [batch projection](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/executor_tools.go#L1007-L1017).
4. `prepareWorkingRequest` first assembles the ledger, measures the whole request, asks the policy to compact, and assembles again. Only then does the final guard repeatedly select the largest unarchived result and replace it with a durable recall pointer. If selection returns size zero, it reports input-budget exhaustion. [Fitter](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/working_context.go#L591-L650).
5. `largestToolResult` excludes a result solely when `strings.HasPrefix(content, archivedResultPrefix)` holds. The constant is `Observation archived:`. It does not check whether the host actually replaced this result or whether its content names a valid archive handle. [Prefix and predicate](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/working_context.go#L970-L1004).
6. Production native/Piggyback requests call this fitter before dispatching to the provider. [Dispatch](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/working_context.go#L1007-L1032).

The trust boundary is source-bearing result bytes becoming host-owned representation state. A result may legitimately quote an archival message, contain a fixture, or print a log with this prefix. It need not be malicious. Its leading text does not prove that its body has been replaced by a host-generated reference.

## Conditional witness and existing mitigations

Use an authorized successful tool returning a body beginning exactly `Observation archived:` followed by sufficient text to exceed the serving slot's input budget. The result has been durably recorded and is the newest result; it is neither stale nor superseded. The task, instructions, catalog and actual host-generated recall pointer fit within the budget.

The policy protects recent rounds: normal shed candidates must be `working_aged`. Stale or superseded results can nevertheless be dropped, so the witness explicitly excludes those cases. Live recalled results have further protection. [Age and drop rules](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/context/working_set.mg#L313-L369).

After policy fitting, the final guard still sees an oversized request. If this is the only remaining substantive tool body, `largestToolResult` skips it and returns size zero. The request is refused rather than sent with a genuine pointer. With other eligible bodies it can first archive those and then refuse, unnecessarily displacing other evidence. These are source deductions, not executed outcomes.

An existing test already supplies the relevant guard shape: a 200,000-token session budget, 20,000-token serving cap, and a recent result of `4*30000` bytes that must become a pointer. It does not use the colliding prefix. Reuse that shape for a regression rather than inventing a huge resource-exhaustion demonstration. [Existing slot-cap fixture](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/slot_input_cap_test.go#L60-L106).

Important limits:

- A request already fitting the window is unaffected.
- Policy eviction before the guard can replace the body with a real pointer and avoid the defect.
- Output formatters that add an earlier header or line number prevent this exact prefix collision.
- The final guard remains fail-closed: this finding is an avoidable refusal, not acceptance of an oversized request.
- Durable storage and recall already exist; the issue is not missing archival infrastructure.
- A leading space or different capitalization does not match the exact predicate.

## Proposed contract and alternatives

Archival state must be host-issued and tied to the exact result instance or projection, not reconstructed from its displayed bytes.

Two bounded alternatives deserve comparison:

1. Have request assembly return the messages plus a host-owned set of result positions already rendered as pointers. Seed it from actual policy eviction, then update it after each guard replacement. This is local to a single assembled request and avoids introducing mutable archival flags into unrelated provider types.
2. Introduce a typed result projection containing the source identity, `full` versus `archive-reference` representation, and the rendered payload. Preserve the disposition through flat/native-block conversions and use only the payload at the provider boundary.

Do not key a new map merely by unvalidated tool-call ID: existing history can contain repeated IDs, and the ledger deliberately has rules for that case. Decide whether state is per occurrence or per observation, and keep assembly, fitting and native-block rebuilding consistent. Recompute request-local state on every assembly; avoid an unbounded lifetime map. A broad result-type migration must justify its complexity against the smaller assembly-owned set.

The displayed prefix and truncation notice can remain unchanged for users and models. Do not escape, delete, or rewrite genuine source content just to prevent collision. Do not weaken policy retention or alter allowed tools. The state must not become authorable through a tool's returned JSON or text.

## Proposed validation and complete costs

Unrun validation:

- Equal-size ordinary and prefix-leading raw bodies: both fit by real archival under the same tight cap, and both remain byte-identical when enough space exists.
- Real policy-generated and guard-generated pointers: never repeatedly archive them; recall resolves the correct retained source.
- Recent, aged, stale, superseded and live-recalled observations: preserve existing policy decisions.
- Several results, duplicate call IDs, multiple results in one message, and native block-built messages: representation state follows the exact output actually sent.
- Missing durable observation/handle, task-or-catalog-only overflow: existing honest refusal remains.
- Producer-to-provider integration with an already-authorized raw-output tool: verify receipt creation, actual request bytes and provider dispatch/no-dispatch, not just the helper's returned index.

Measure end-to-end verified completion, avoidable budget refusals, retries/redelegations, recalls, final input/output/cached-token accounting, and latency. Also measure fitter allocations and host bookkeeping overhead. Holding a small request-local set is not free; a typed cross-layer migration has compatibility and maintenance costs. Fixing the refusal may cause an additional valid model request and therefore increase immediate billed tokens while avoiding an unresolved task. No universal token saving is implied. Compare identical provider/model, task snapshots, allowed tools, policy, caps and acceptance criteria; a smaller prompt achieved by discarding evidence is not an improvement.

## Prior-work boundary

The main and research audit inventories were inspected. The nearest read studies cover context-ledger revision collection and ordered compaction costs; working-observation supersession; generated-tool output fidelity; native provider context admission; request-visible tool contracts; research-observation identity; and tool-effect receipts. None of those inspected texts identifies this prefix-as-state predicate. This is a bounded novelty claim, not proof that no prior discussion exists.

In particular, [context-ledger-prefix-costs](https://github.com/theRebelliousNerd/codenerd/blob/d613a1c38be5ec3b79e7e134a224d5910dbc3ac5/Docs/audits/context-ledger-prefix-costs.md) explicitly preserves the final guard while optimizing different upstream work. [Working-observation supersession](https://github.com/theRebelliousNerd/codenerd/blob/d613a1c38be5ec3b79e7e134a224d5910dbc3ac5/Docs/audits/working-observation-supersession-contract.md) concerns which source evidence a later read covers, not whether arbitrary result text is an archival receipt.

Exit criterion: changing the first characters of a genuine tool body to the display marker cannot change its archival eligibility, while genuine host-generated pointers, policy decisions, source bytes and final request limits retain their existing semantics.
