---
doc-class: deep-dive
subsystem: campaign-recurse
implementation-status: proposed-refinement
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: source-inspection-only
supersedes: []
---

# Recurse sweep nodes need distinct source and stage identities

## Finding and boundary

Recurse represents physical source nodes and synthetic closing stages in one string namespace. Physical nodes can use raw relative directory names, while closing stages use the literals `wiring`, `review`, and `bench`. An otherwise valid Python workspace containing `review/__init__.py` therefore produces two nodes named `review`. The graph validator correctly rejects the duplicate before dispatching a node attempt. This is a fail-closed availability and general-workspace support defect, not an unsafe pass, silent skip, or permission bypass. [Stage constants][constants] [Python producer][python] [Closing stages][closing] [Validation][validation]

Make node identity deterministic, injective, and disjoint across physical components and synthetic stages. Keep human labels, CLI selectors, and filesystem paths separate. Preserve duplicate rejection and whole-graph validation rather than discarding a node or filtering away the collision.

Main revision `388205138ba6d47c1cff0c5e97c896dfa642a391` was resolved at 09:09 UTC on 2026-10-30. An independent source challenger corroborated the collision and validation ordering. No repository code, tests, applications, or model evaluations were run; fixtures and expected outcomes are source-derived and unrun.

## How a valid directory becomes an invalid graph

Workspace derivation canonicalizes the root, collects and scans source membership, collapses the physical graph, and then appends cross-cutting stages. The Python producer creates nodes from relative directories independently of whether their files contain imports. The collapse function constructs component IDs with `strings.Join(comp, "+")`, including singleton components, while retaining source paths separately. Consequently, a singleton directory `review` remains a physical node with ID `review`. [Derivation][derivation] [Python producer][python] [Component construction][components]

The closing-stage constructor then appends `wiring`, dependent on the physical top nodes; `review`, dependent on `wiring`; and `bench`, dependent on `review`. It does not establish a separate namespace for these nodes. The collision follows from combining legitimate outputs of the physical producer and stage constructor, not from malformed user-supplied graph data. [Closing-stage construction][closing]

`validateSweepGraph` rejects duplicate or empty IDs, unknown dependencies, and self-dependencies. `assertSweepGraph` validates before replacing graph facts. The CLI obtains whole-graph policy order before calling `FilterDAG`, so a subsystem selector cannot hide the duplicate. Preserve this whole-graph integrity boundary, including cycle checks. [Validation][validation] [Assertion][assertion] [Policy assertion path][policy-order] [Planning order][plan-order] [Soundness assertion][graph-sound]

The public journey describes planning, subsystem focus, and workspace generality. Credit the existing DAG generalization and cycle collapse; the refinement is identity representation, not replacement of that architecture or Mangle's graph-ordering authority. [Journey contract][journey]

## Primary conditional witness

Use a clean Git workspace with readable, tracked `review/__init__.py` and `pkg/__init__.py`. Both are valid empty Python files, with no imports. There is no `go.mod`, and no applicable ignore rule excludes these files. Assume healthy boot and kernel availability. No mutation, suspicious source content, or model-generated plan is needed: the directory name alone supplies the collision through normal workspace discovery. [Workspace membership][membership] [Python producer][python]

The physical graph contains `review` and `pkg`. The synthetic suffix adds another `review`. The CLI's `--plan` branch calls `writeRecursePlan`, which uses `RecurseSweepOrder`; whole-graph validation rejects the repeated identifier. Selecting `--subsystem pkg` still encounters the same error because filtering follows validation and policy ordering. A nested directory such as `src/review` is a negative control: its physical ID is not the literal `review`. [CLI plan route][cli-route] [Plan writer][cli-plan] [Whole-graph planning][plan-order]

The shared live loop also validates the whole graph before filtering. Thus CLI and TUI execution can encounter the same rejection, conditional on passing their preceding admission and setup checks. TUI code derives raw IDs into a boolean map and subsequently invokes `RunRecurseCycles`; its started-then-error presentation does not create a separate namespace solution. This audit does not claim a second independent UI defect. [Live ordering][live-order] [TUI preview and admission][tui-admission] [Shared loop call][tui-run] [Visible error handling][tui-error]

“Before node dispatch” must not be confused with “before all side effects.” Live execution acquires ownership, may begin a mission, clears stop state, reads the journal, settles in-flight work, prepares Git state, opens the journal, and resumes history before reaching passes. Depending on state, it may reconcile work or switch/create a branch before detecting this graph problem. The clean, fresh witness assumes no in-flight attempt; it establishes rejection before any new node attempt, not a side-effect-free live command. [Live setup sequence][setup]

Renaming the directory avoids this input but is not a general repair. Legitimate physical nodes and closing stages must remain representable without excluding ordinary source-directory names.

## Identity also governs scope and history

The DAG contract documents IDs as stable references for waves, findings, and resume, while `Paths` separately describes source scope. Empty paths are reserved for whole-tree cross-cutting nodes. Attempt construction uses node identity in titles and goals but derives scope from `Paths`; its write set copies those paths. A repaired encoded ID must never be interpreted as a filesystem path or accidentally convert a physical node into a whole-tree task. [DAG representation][dag] [Attempt scope][attempt-scope] [Write set][write-set]

Identity persists beyond ordering. Resume tracks completed work by historical node string, restores policy attempts and node identity, and reasserts completed visits. Journal visit records use `node.ID`; gate keys also incorporate the node identifier. Changing the producer without a migration contract could therefore transfer or lose completion evidence. In particular, an old synthetic `review` visit must never count as a visit to a physical `review` component. [Resume handling][resume] [Visit reassertion][reassert] [Visit recording][visit-record] [Gate identity][gate-key]

Delimiter joining creates another instance of the same problem. A component containing directories `a` and `b` and a singleton directory named `a+b` both receive ID `a+b`. An additional unrun JS/TS fixture can use mutual imports between `a/u.ts` and `b/u.ts`, with independent `a+b/u.ts`. This is an encoding counterexample, not an observed incident or security-injection claim. Prefixing only synthetic stages leaves this ambiguity unresolved. [Component construction][components] [JS/TS discovery][js-discovery]

The duplicate validator is a successful protective control. An inspected test exercises duplicate, unknown, self-referential, and empty-node rejection; malformed graphs must remain rejected after the producer is repaired. Normal-name workspace and SCC fixtures are useful context, not proof against these collisions. No test was run. [Validation tests][validation-tests] [Workspace tests][workspace-tests]

## Proposed identity contract

Represent physical components and synthetic stages as distinct kinds. Serialize them using a versioned, deterministic, injective encoding of kind and payload: sorted canonical member paths for a physical component, and an enumerated stage identifier for a closing stage. Use unambiguous framing or escaping rather than raw delimiter joining. Prefixes alone are insufficient if arbitrary physical strings can inhabit the prefixed namespace.

Keep display labels readable and separate from internal IDs. Keep filesystem `Paths` unchanged and authoritative for scope. CLI selectors should resolve separately: a bare source path can remain an alias when uniquely resolvable, while explicit typed stage selectors distinguish the closing `review` from a source directory of that name. Do not silently use first-wins maps, discard colliding nodes, or require repository renames.

Go should observe membership and produce faithful identities; Mangle should retain ordering, readiness, and decision ownership. The policy consumes string IDs, allowing stronger encoding without making the logic engine a path parser. Continue full-graph validation before filtering. [Policy schema][policy-schema] [Policy decisions][policy-decisions] [Filter representation][filter]

Fact construction currently sorts and ranks IDs for a lexicographic tie-break. New serialization can change ordering among otherwise-ready independent nodes. Explicitly decide whether to retain a separate canonical path/display tie-break key, rather than silently treating encoding order as policy. Preserve dependency order either way. [Rank construction][rank]

Because IDs drive resume, version new journal identity records. Map legacy strings only when historical membership and node kind establish a unique interpretation. Preserve old journal bytes and record the mapping decision. Ambiguous history needs explicit reconciliation or a conservative new pass under a visible policy; it must not silently donate visits to another node or erase history. A changed SCC membership is an identity change, even if a familiar display label survives.

A hash-based identifier is an alternative, but it introduces collision handling and reduced readability; it still requires kind separation and canonical input. The essential property is unambiguous identity, not a particular printable syntax.

An optional earlier preflight can catch known collisions before branch preparation and journal setup. It is not a substitute for validation under live ownership and freshness: graph changes between preflight and execution must invalidate stale results or trigger revalidation. Moving filtering ahead of validation is not an acceptable availability shortcut.

## Costs and tradeoffs

Canonical encoding can be linear in total member-path bytes after existing SCC and sorting work. Longer IDs increase fact, journal, and UI bytes. Compatibility affects selectors, tie-breaking, interfaces, tests, and history migration. Measure these costs; this audit claims no speedup or token saving.

Encoding itself needs neither another model call nor additional filesystem I/O. A separate preflight may duplicate walking or language discovery, including Go listing where applicable, unless reusable results are bound to a trustworthy workspace version. Conservative handling of ambiguous history can revisit gates and require additional model turns in normal operation. Measure that recovery cost separately from identity construction, and do not preserve unsafe evidence reuse merely to minimize retries.

## Unrun evaluation plan

Compare current and proposed implementations with identical workspaces, tools, and budgets. Exercise actual discovery through kernel ordering and CLI planning, then TUI/shared-loop execution with a scripted executor and no inference. Helper-only tests would not establish full-path behavior.

Cover each reserved name and all three together; an unrelated `pkg` focus; nested names as negative controls; the SCC delimiter fixture; and legal Unicode, spaces, and platform-specific path cases. Include ordinary Go, Python, and JS/TS workspaces, shuffled discovery order, cycles, and deliberately malformed duplicate graphs that must still be rejected.

Verify that every physical component appears exactly once, synthetic stages retain closing order and intended edges, and independent-node tie-breaking follows the chosen contract. Check unchanged `Paths`, scope, and write sets. Exercise ignored or unreadable sources and existing dirty-workspace, lock, and cancellation guards without weakening them.

Resume cases should demonstrate unique legacy mapping or an explicit block/reconciliation outcome, never an incorrect inherited visit. Change the graph between preflight and live execution to test freshness. Record errors, identities, edges, scopes, visits, latency, allocations, serialized bytes, source-scan counts, model-call counts, and retries. These are proposed observations, not measured results.

## Scope and prior work

The targeted screen covered campaign, recurse, gate, and self-improvement audits, journey 10, and research heads `493e65a2892d7c5262de95999b8c2e256f6a3c4f`, `136e02f0a38897a40931747b1e56a6f0c14feb19`, `77069960b69aba14a67dbd682d6f65435e8ada50`, and `07bdcb50873498d22fce498f599073df00ac5750`. The workspace, plan, cycle, CLI-recurse, and chat-recurse files matched across those research heads; the screen found no exact collision treatment.

At active heads `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and `aa7988dec4c62116656ea722ff73b687a8509908`, the producer matched and consumers retained validation-before-filtering, although broader files differed. This is bounded source comparison, not an exhaustive novelty claim or whole-tree equivalence.

The issue is distinct from checkpoint build-profile fidelity, prompt-trial exposure, mission error-free witnesses, and recoverable deletion. Its contribution is a concrete producer identity counterexample and a migration-aware refinement of the existing recurse graph contract.

## Pinned references

All implementation references use the verified main revision above. Line ranges support static reasoning only.

[constants]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_workspace.go#L48-L52
[derivation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_workspace.go#L63-L91
[python]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_workspace.go#L416-L435
[components]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_workspace.go#L190-L222
[closing]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_workspace.go#L228-L248
[membership]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_workspace.go#L251-L309
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_plan.go#L85-L105
[assertion]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_plan.go#L136-L146
[plan-order]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_plan.go#L42-L64
[policy-order]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_plan.go#L273-L279
[graph-sound]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_plan.go#L321-L327
[rank]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_plan.go#L113-L126
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/journeys/10-forever-loop.md#L181-L209
[cli-route]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_campaign_recurse.go#L161-L164
[cli-plan]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_campaign_recurse.go#L295-L303
[live-order]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L457-L478
[tui-admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_recurse.go#L137-L180
[tui-run]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_recurse.go#L205-L211
[tui-error]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_recurse.go#L340-L360
[setup]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L329-L392
[dag]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_dag.go#L16-L32
[attempt-scope]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_attempt.go#L56-L73
[write-set]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_attempt.go#L122-L130
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1530-L1570
[reassert]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L480-L491
[visit-record]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L831-L871
[gate-key]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L296
[validation-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_sweep_test.go#L227-L245
[workspace-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_workspace_test.go
[js-discovery]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_workspace.go#L505-L540
[policy-schema]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/recurse.mg#L107-L110
[policy-decisions]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/recurse.mg#L401-L479
[filter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_dag.go#L37-L78
