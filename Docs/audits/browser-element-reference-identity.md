---
doc-class: deep-dive
subsystem: browser-reference-identity
implementation-status: partial
evidence-status: source-only
last-verified: 2026-11-04
verified-against: e02b896fe8f94a907411a473f397daa37f0b4183
verification-method: static-source-inspection
supersedes: []
---

# Distinct browser controls can share one actionable ref

## Finding and decision

**Two unchanged, visible buttons with the same HTML name but different text can receive the same opaque browser ref. The later button overwrites the registry entry, so selecting the earlier button can dispatch a click to the later one.** This is not a hypothetical hash collision: `fingerprintKey` deliberately omits text and alternative selectors when `Name` is present, while the observation producer makes the same nonunique name selector primary for both elements. [Selector construction][selectors] [Registry key][key] [Batch registration][register]

The stronger witness starts with an exact semantic target for `Alpha`. `ResolveElementMatcher` correctly finds just the Alpha observation row, but returns its shared ref. The action path then looks that ref up again and receives Beta's fingerprint. With the stipulated ordinary DOM, Beta wins the resolver's score uniquely. The semantic ambiguity check succeeds locally while target correspondence is lost at the registry handoff. [Semantic matching][matcher-resolve] [Dispatch][dispatch] [Resolution][resolve] [Scoring][score]

**Required contract:** within a session and document generation, two distinct observed DOM elements must not share an actionable ref. Every successful ref lookup and action must preserve the element identity established by observation or unique semantic selection. If that identity cannot be established, fail before the browser effect and require fresh, sufficiently complete discovery.

This report derives a conditional wrong-target path from source. It does not report an executed browser fixture, measured failure frequency, unauthorized action, observed user harm or failure of every locator. No navigation, DOM mutation, race, score tie, duplicate HTML ID or model selection error is needed for the minimal witness. Browser availability, ordinary tool admission, successful observation, no affirmative honeypot block and successful Rod interaction remain explicit preconditions.

## Evidence boundary and distinct scope

Public main was inspected at [`e02b896fe8f94a907411a473f397daa37f0b4183`][pin] on November 4, 2026, and rechecked unchanged at 22:49 UTC. Repository links pin that revision. Inspection covered root and scoped guidance, browser architecture, tool registration and runtime binding, observation and registry code, semantic matching, action dispatch, relevant authored tests and the current audit inventory. Cached repository files were accepted only after their Git blob hashes matched the pinned tree. No repository program, test, build, live browser fixture, benchmark, provider inference or training was run. Evaluations below are proposed and unrun.

[Root guidance][root] places deterministic executive control in the kernel. [Architecture guidance][architecture-guidance] requires intended capabilities, implemented paths and remaining gaps to stay distinct. [Browser guidance][browser-guidance] specifically requires opaque session-scoped refs, generation invalidation, unique declarative matching and failure on stale or ambiguous refs. Correcting the correspondence between an observation and its effect belongs in the existing manager, registry and action path, not in a second reasoning engine or more persuasive prompts.

The current [browser epoch publication audit][prior-epoch] concerns a publication/retirement interleaving. The [research-browser ownership audit][prior-owner] concerns runtime ownership, live-kernel binding, read-sensor lifetime and evidence cleanup. The [URL userinfo audit][prior-redaction] concerns sanitization before evidence publication. Those are substantive and separate boundaries. This finding requires none of their failures: all ownership, redaction and epoch cleanup may work as intended while two current elements alias in one observation.

The browser parity ledger labels stable interactive references implemented but unverified against live Chrome. Treat that as a capability claim to narrow, not as evidence that element identity has been established. Its older neighboring gap statements are not substituted for inspection of current code. [Ledger][ledger]

| Capability | Status at this pin | Evidence and limit |
|---|---|---|
| Progressive observation/action tools reach the manager bound at boot | Implemented | Both tools are registered; factory binds the manager and live kernel. [Registration][tools] [Factory][factory] [Tool handlers][tool-handlers] |
| Repeated registry keys reuse opaque refs | Implemented | A key lookup reuses the existing ref and replaces its stored fingerprint. [Registration][register] |
| Old-generation observation batches are rejected | Implemented | Registration checks the captured generation under its lock. [Generation admission][generation] |
| Navigation clears old refs | Implemented in the registry contract | `Clear` advances the generation and replaces both maps. This report does not audit every navigation caller. [Clear][clear] |
| Semantic discovery rejects truncation and multiple matching rows | Implemented | It checks completeness and exact match cardinality. [Matcher][matcher-resolve] |
| Distinct observed nodes always have distinct actionable refs | Incomplete | Same-name controls can share a key despite different text and paths. [Producer][selectors] [Key][key] |
| Unique semantic selection remains bound to the selected element through dispatch | Incomplete | Returning the aliased ref discards the row distinction. [Matcher][matcher-resolve] [Dispatch][dispatch] |
| Repair, latency improvement, task-quality improvement and incidence | Unimplemented or unverified here | The contracts and evaluations below are proposals, not measured results. |

## Production route and no-churn witness

### Reachability does not imply authorization

`research.RegisterAll` includes `BrowserObserveTool` and `BrowserActTool`. Their handlers require the bound runtime and call its manager's `Observe` and `ExecuteActions`. Factory boot constructs that manager, installs live-kernel query/retraction adapters and binds it into `BrowserRuntime`. The witness begins only after the effective catalog and host authorization admit the operation and a usable session exists. It does not claim that tool registration bypasses permissions. [Registration][tools] [Handlers][tool-handlers] [Factory][factory]

### A valid, ordinary page

Consider this static fragment, with normal visible layout and enabled controls:

```html
<div id="panel">
  <button type="button" name="mode">Alpha</button>
  <button type="button" name="mode">Beta</button>
</div>
```

For an eventual behavioral test, attach separate benign click counters to the two buttons. Keep both controls within the complete interactive observation window. They have no ID, test ID, aria-label, explicit role or row attributes. Their distinct text does not contain the other label. Do not navigate or change the DOM between observation and interaction. The container ID is unique and belongs to neither button.

HTML's `name` on a button describes its form-related name; it is not a unique DOM-node handle. The HTML standard explicitly supports name-based form-control access and gives `button` its own non-submitting type. Repeated names need not be repaired by modifying the site for a browser agent to distinguish these elements. [HTML button specification][html]

### Observation loses identity before any action

The producer computes a positional CSS path for each button. Because the two siblings have the same tag, their paths differ:

- Alpha: `#panel > button:nth-of-type(1)`.
- Beta: `#panel > button:nth-of-type(2)`.

But `selectorFor` first appends the name selector, so both have primary selector `button[name="mode"]`; the distinct paths are only alternatives. `record` captures the distinct text and label, but sets `InputType` only for `input` elements. These buttons therefore have empty `InputType` even though their separate `ControlType` is `button`. [Path and selector construction][selectors] [Raw record][record]

`registerRawElements` faithfully copies primary selector, alternatives, name, text and the remaining fields into `ElementFingerprint`. The loss happens in the registry key, not because observation never collected the distinction. [Raw-to-registry projection][raw-register]

For both records, `fingerprintKey` consists of the same sequence:

`["button", "name", "mode", "", "button[name=\"mode\"]"]`

The key joins these fields with a NUL separator. Its Name branch excludes the different text and positional alternatives. The comment that appending a selector disambiguates repeated names is not satisfied when the appended selector is itself the shared name selector. This is an equality collision in the chosen identity representation, not a probabilistic hash collision. [Key][key]

Let the allocated opaque ref be `R`; its exact counter value is immaterial. The batch loop then evolves as follows:

| Step | Returned observation copy | Registry state |
|---|---|---|
| Register Alpha | Alpha fingerprint receives R. | `elements[R]` contains Alpha. |
| Register Beta with the same key | Beta fingerprint also receives R. | `elements[R]` is overwritten with Beta. |
| Materialize interactive output | Alpha row has label Alpha and ref R; Beta row has label Beta and ref R. Full view preserves each row's own fingerprint text. | A later `Get(R)` returns Beta. |

The last row matters: materialization uses each `registered[i]` copy rather than rereading the final map entry. The output can truthfully display two distinct labels yet give callers no distinct handle for them. Registry locks and copy semantics are working as written; no concurrent access is involved. The returned data retains the interactive slice; policy-ref filtering does not deduplicate its rows. [Batch loop][register] [Lookup][get] [Interactive output][materialize] [Returned slice][returned-slice] [Public fingerprint][public-fingerprint]

### Direct refs and unique semantic targets converge on Beta

A caller choosing Alpha's row passes its ref R. `ExecuteActions` obtains a portable matcher using `MatcherForRef`, which reads the registry entry now containing Beta. `InteractRef` resolves that same ref. [Dispatch][dispatch] [Matcher lookup][matcher-for-ref]

The semantic route is even more instructive. Submit an `interact` operation with no ref and this target:

```json
{"tag_name":"button","name":"mode","text":"Alpha"}
```

`ResolveElementMatcher` makes a fresh full interactive observation, rejects truncated discovery and matches all supplied semantic fields conjunctively. Only the Alpha row matches the exact text, so the function correctly sees one match and returns that row's R. The fresh observation recreates the same registry alias. `ExecuteActions` then calls `MatcherForRef(R)` and `InteractRef(R)` rather than carrying the selected row's identity to the effect. [Semantic discovery and cardinality][matcher-resolve] [Exact field matching][matcher-fields] [Dispatch][dispatch]

`resolveElementRef` gets Beta's current-generation fingerprint. It queries the shared primary selector before trying alternative selectors. The two candidates score as follows:

| Contribution against the stored Beta fingerprint | Alpha | Beta |
|---|---:|---:|
| Matching tag starts the score | 1 | 1 |
| Matching strong `name="mode"` | 60 | 60 |
| Stored text Beta occurs in candidate text | 0 | 20 |
| Total | **61** | **81** |

Empty expected ID, test ID, aria-label, role and input type add no checks. Text mismatch is not rejection, but this witness does not rely on a tie or a weak winner: Beta is the unique higher-scoring match. Resolution immediately returns it from the primary selector, before reaching Beta's distinct alternative path. A tie-rejection patch alone would leave this witness intact. [Selector order and early return][resolve] [Scoring][score]

`InteractRef` checks the selected element's visibility and honeypot verdict before calling Rod's click. With an ordinary allowed Beta control and a successful click, the batch marks the operation successful because no error occurred. That is a successful browser effect on the wrong element relative to the original Alpha selection, not proof that the intended action happened. [Interaction][interact] [Honeypot gate][guard] [Success accounting][success]

This conclusion is conditional on the specified normal browser behavior. It is not an assertion that a particular site or real user action has been reproduced. The independent click counters in the proposed evaluation are the required effect oracle; the tool's success flag alone is insufficient.

## What does and does not protect this boundary

- **Generation fencing protects document turnover.** Both controls are in the same current generation, so the existing guard correctly accepts the batch. Increasing the generation only on navigation does not separate two current nodes.
- **Repeated observation does not recover the lost distinction.** The unchanged producer emits the same colliding keys again. Asking the model to observe more carefully does not make R unambiguous.
- **Semantic uniqueness is valuable but insufficient by itself.** The matcher finds exactly the intended row. The defect occurs after that check, when a many-to-one handle is substituted for the row identity.
- **Text scoring cannot repair the registry overwrite.** It optimizes against Beta's stored fingerprint and therefore reinforces the substitution in this fixture. Merely adding text to the key would address this exact pair but leave legitimate same-text controls unresolved and make ordinary text changes affect identity.
- **Alternative selectors exist but do not establish identity.** They are excluded from this key and are not reached after the primary selector finds a scored candidate. Blindly preferring positional paths would require its own policy for insertion, reorder and replacement.
- **Kernel admission and honeypot checks answer different questions.** An admitted, visible, non-trap Beta control is not thereby the Alpha control selected by the user or planner. Preserve these checks; do not present the finding as a permission bypass.
- **Unique IDs or distinct names avoid this particular alias.** They are useful negative controls, not assumptions every ordinary page satisfies.

## Proposed identity and failure contracts

These are requirements for a future implementation, not code changes made by this audit.

1. **Separate node identity from descriptive similarity.** A ref identifies an observed element, not an equivalence class of elements sharing name, text or a preferred selector. Repeated records of the same node may reuse a ref; distinct nodes may not. Include session/document scope and define frame scope explicitly if observations expand to multiple frames.
2. **Make registry admission collision-aware.** A proposed key reuse must establish that it denotes the same observed node. If the batch cannot prove that, return a bounded identity error rather than overwriting the entry. Validate/stage the batch before publishing actionable refs so a late collision cannot leave an apparently successful partial snapshot or silently repoint an older handle.
3. **Carry the selection through dispatch.** A unique semantic match must yield an identity-bearing binding or a ref whose lookup is guaranteed to preserve that binding. Before effect, verify the target against that binding. Do not discard the original selected identity and then treat a second successful lookup as sufficient.
4. **Define stale and replacement behavior separately.** A disconnected/replaced node should either fail and require rediscovery or follow an explicitly specified, uniquely justified semantic rebinding policy. Ordinary DOM churn is not permission to turn an old ref into whichever current control scores highest. Rebinding must not silently broaden authorization.
5. **Preserve opacity, boundedness and executive controls.** Keep selectors and internal handles out of model-facing observations and portable fixtures. Preserve generation rejection, truncated-discovery rejection, redaction, visibility and kernel checks, cancellation and action limits. Return a useful refusal category without leaking private page values.

A private node handle, a manager-owned identity table or a verified snapshot binding could support these contracts. This report does not pick a browser mechanism without lifecycle and compatibility work. Adding a unique counter to every observation row is also not enough by itself: it must remain attached to the correct node during resolution, and duplicate appearances of one node across slices need defined reuse behavior.

Two primary designs clarify the distinction. The dated July 2, 2026 W3C WebDriver working draft defines node references through node-keyed maps and session/context scope, with explicit stale-element handling. That is a useful identity model, not a claim that CodeNERD implements or must switch to WebDriver. [WebDriver elements][webdriver] Playwright's locator documentation makes single-target operations reject multiple matches. That is a useful cardinality contract, but the Alpha matcher already meets cardinality here; identity must also survive the next handoff. [Locator strictness][playwright]

## Full costs and tradeoffs

For N observed records, the existing registry performs one key lookup and copied fingerprint write per record. A separate identity index and batch collision validation can remain linear in record count plus total identity/fingerprint bytes, using expected constant-time map access apart from key hashing, but require additional keys, retained metadata and staged updates. Measure bytes and allocation counts rather than assuming the extra maps are free. [Registry][register]

If identity comes from browser/backend handles, acquiring, checking and releasing them adds protocol work and ownership obligations across navigation, target closure and cancellation. Retaining strong handles may retain browser objects; weak or expiring handles need a defined failure path. Batching may reduce round trips but cannot replace evidence of distinct identity. No specific overhead or compatible implementation is established here.

Current resolution queries selectors and performs per-candidate property/attribute/text reads. Any new verification must be evaluated alongside that existing work. The final tag fallback caps its candidates at 200, while primary selector queries do not use that particular cap; a repair must not claim a whole-resolution work bound based only on the fallback. Bound scans and fail if completeness needed for uniqueness cannot be established. [Resolution][resolve] [Scoring][score]

Fail-closed collisions or stale bindings can require another observation, increasing latency, protocol calls and model-facing token use. Repeatedly observing the unchanged colliding representation would only create a refusal loop, so containment needs a diagnosable unsupported-identity result and a repair path, not indefinite retries. Stable identity should reduce avoidable rediscovery, but that performance benefit is unmeasured.

A more specific key may reduce aliasing while fragmenting one node's identity when text, position or mutable attributes change. A more permissive key improves apparent stability while increasing substitution risk. The quality objective is correct target correspondence under an explicit replacement policy, not maximum ref reuse or maximum click success. No additional model inference or training is needed to demonstrate the current loss; whether a repair improves end-to-end task success requires separate evaluation.

## Fair, unrun evaluation plan

Read-only inspection found registry tests for reuse of an identical fingerprint, clearing, stale-generation batch rejection and returned-copy isolation. The matcher tests cover validation, sensitivity and selector-free portable conversion. The live policy observation test covers measured facts and ranking, but does not provide the two same-name/different-text identity witness. These are authored tests, not passing results from this audit. [Registry tests][registry-tests] [Matcher tests][matcher-tests] [Policy test][policy-test]

A fair comparison should preserve the same pinned baseline, browser version, page fixture, observation mode, admission policy, action, timeouts and output budgets. Change only the candidate identity repair. First establish the original behavior; do not assume the source-derived path is already a measured baseline.

| Case | Required oracle |
|---|---|
| Static Alpha/Beta page, direct Alpha ref | Full observation has both rows; distinct physical nodes require distinct refs. Clicking the Alpha row changes only Alpha's counter, or refuses before either changes. |
| Same page, semantic Alpha target | Exact selection remains bound through dispatch. Beta's counter must stay zero; tool success alone is not a pass. |
| Reverse DOM order | Selection must follow the requested element, not whichever same-key record was registered last. |
| Distinct IDs, distinct names and one-button controls | Ordinary valid interactions continue to work. Do not win by refusing every click. |
| Repeated observation of the same unchanged node | Defined stable reuse holds without merging it with siblings. |
| Same text/name controls, repeated aria-labels, repeated test IDs and radio groups | Distinct node identity survives descriptive equality; genuinely ambiguous semantic requests refuse rather than choosing first/last. |
| One node included in more than one observation slice | Intended reuse is distinguished from two-node aliasing, with no policy/evidence identity split. |
| Reorder, text change, node removal and replacement | Enforce the documented stale/rebinding policy; never silently act on a replacement solely because it inherits a selector. |
| Navigation, truncated discovery, cancellation and identity-cap exhaustion | Existing refusals remain; no partial successful publication, leaked handle or post-cancellation effect. |
| Hidden/disabled/trap controls and redacted portable output | Identity repair preserves the existing independent guards and privacy boundaries. |

Use benign local effects and an independent DOM/event counter oracle. Include enough unrelated controls to exercise the normal bounded observation path, then separate limit tests from the two-element witness. Keep security settings, credentials and authorization unchanged. Record actual target identity, wrong-target count, correct completion count, refusal reason, protocol calls, retained handles/memory, observation tokens and latency distributions. Report discovery/identity failures separately from browser availability, timing and policy refusals. Model-level task studies, if later authorized, should hold the model and prompt fixed and report both extra rediscovery cost and target accuracy.

The acceptance criterion is not that Alpha has a higher score. It is that choosing Alpha cannot become a successful Beta effect through an aliased ref, while ordinary identity-preserving actions remain usable.

## References

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/e02b896fe8f94a907411a473f397daa37f0b4183
[root]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/agents.md
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/Docs/architecture/AGENTS.md
[browser-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/agents.md#L36-L42
[prior-epoch]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/Docs/audits/browser-epoch-publication-retirement.md
[prior-owner]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/Docs/audits/research-browser-ownership-and-evidence-retirement.md
[prior-redaction]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/Docs/audits/browser-evidence-url-userinfo.md
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/Docs/architecture/browser/15-PARITY-LEDGER-AND-GAPS.md#L29-L35
[tools]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/tools/research/register.go#L19-L34
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/system/factory.go#L2050-L2070
[tool-handlers]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/tools/research/browser_progressive.go#L39-L110
[selectors]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_observe.go#L817-L844
[record]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_observe.go#L846-L892
[raw-register]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_observe.go#L748-L759
[key]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/element_registry.go#L137-L158
[register]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/element_registry.go#L50-L92
[generation]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/element_registry.go#L61-L74
[get]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/element_registry.go#L95-L104
[clear]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/element_registry.go#L124-L135
[materialize]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_observe.go#L570-L588
[returned-slice]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_observe.go#L449-L478
[public-fingerprint]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_observe.go#L762-L772
[matcher-resolve]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/declarative_matcher.go#L90-L119
[matcher-fields]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/declarative_matcher.go#L122-L145
[matcher-for-ref]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/declarative_matcher.go#L64-L88
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_action.go#L121-L141
[resolve]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_action.go#L573-L630
[score]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_action.go#L633-L676
[interact]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_action.go#L300-L338
[guard]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/honeypot_gate.go#L68-L113
[success]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_action.go#L268-L295
[registry-tests]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/element_registry_test.go#L5-L55
[matcher-tests]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/declarative_matcher_test.go#L9-L43
[policy-test]: https://github.com/theRebelliousNerd/codenerd/blob/e02b896fe8f94a907411a473f397daa37f0b4183/internal/browser/progressive_observe_policy_live_test.go#L31-L169
[html]: https://html.spec.whatwg.org/multipage/form-elements.html#the-button-element
[webdriver]: https://www.w3.org/TR/2026/WD-webdriver2-20260702/#elements
[playwright]: https://playwright.dev/docs/locators#strictness
