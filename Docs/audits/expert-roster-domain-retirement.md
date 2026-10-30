---
doc-class: deep-dive
subsystem: expert-roster-refresh
implementation-status: proposed-refinement
last-verified: 2026-10-30
verified-against: 486db57ce490302c65d7f484e7f6070ee0e70862
verification-method: static-source-inspection
---

# Expert roster domain retirement

## Decision and evidence boundary

Make roster refresh reconcile the evidence it owns, rather than only adding its latest observations. The current implementation can successfully synchronize an agent's edited prompt atoms, measure that its former domain tag is absent, and nevertheless leave the old domain active in the running kernel. Future expert selection then reasons correctly over an obsolete premise.

This is a conditional source-level finding at public main `486db57ce490302c65d7f484e7f6070ee0e70862`, not an executed incident or a measured quality or efficiency regression. Inspection was read-only; no tests, application execution, model inference, training or benchmarks were run. The proposed repair preserves Mangle as the executive: Go measures and publishes source support; policy still decides which expert a task needs and what role it receives.

## Production producer and consumer chain

Boot's roster measurement combines several legitimate sources. Registry domains are asserted when the field is present; registered knowledge databases contribute their current language/framework tags. An explicit empty registry list is different from an absent field: absence permits one-time domain derivation and persistence. [Roster measurement](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/shard_expert_facts.go#L118-L148), [registry contract](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/agent_registry.go#L84-L91).

The compiler's `ShardTechnologies` reads distinct non-exclusion `lang` and `framework` tags from `atom_context_tags`. It reads the database, not a separately retained technology-name cache. [Tag reader](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/compiler_experts.go#L167-L208).

There is a production refresh path after boot. Successful creation of a new wizard agent calls `Cortex.ActivateAgent`. Activation invokes `SyncAll`, which synchronizes all discovered definitions, then registers the named agent, synchronizes the registry and refreshes the roster. Thus activating B can also synchronize a previously curated edit to A. This does not require claiming that the wizard itself edits existing A definitions; it deliberately preserves them. [Wizard handoff](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/cmd/nerd/chat/agent_wizard.go#L154-L168), [activation](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/agent_activation.go#L32-L69).

Synchronization checks each YAML's content hash. A changed, successfully parsed nonempty atom set is passed to `ReplaceAtoms`; replacement transactionally deletes old context tags and prompt atoms before inserting the new set. The finding therefore does not depend on an old tag remaining in SQLite. [Synchronization](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/sync/synchronizer.go#L117-L165), [replacement](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/prompt/loader.go#L307-L324).

The discontinuity is the publication operation. `refreshShardRoster` measures the new roster and calls `kernel.AssertBatch(roster.facts)`. Its subsequent work records previously undecided domains; it does not reconcile old measured tuples against the new set. An absent current tuple is not a retraction of a previously asserted tuple. [Refresh implementation](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/shard_expert_facts.go#L205-L249).

The consumer accepts domain evidence from any source. `expert_needed` joins task technology, `shard_agent_domain` and a ready roster agent. Import evidence can also provide the confidence for consultation; spawning and critique retain their separate policy conditions. The session helper invokes this selector and supplies the derived names to compilation. [Expert policy](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/core/defaults/policy/shard_experts.mg#L97-L168), [session selector](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/session/expert_context.go#L19-L43).

## A source-isolated witness

Use a ready agent with a neutral name such as `AgentA`, an explicitly present `domains: []`, no independently supporting catalog/name domain, and a nonempty prompt atom set carrying a Bubble Tea framework tag. Boot measures and asserts `(AgentA, bubbletea, knowledge_base)`.

During that session, the user curates A's YAML to remove the Bubble Tea tag while retaining valid nonempty atoms. Successful creation and activation of B invokes the normal all-agent synchronizer. Grant successful A replacement, successful database reads and successful roster publication. A's current SQLite tags now lack Bubble Tea, and the new roster measurement supplies no replacement support for that old tuple. The old kernel tuple nevertheless remains because refresh only asserts.

A subsequent task targeting a scanned file with a matching Bubble Tea import can still derive A as needed from that retained premise. Compare a fresh kernel boot over the same final files and explicit empty registry domains: without another source supporting the domain, the stale premise should not be reproduced. This running-versus-fresh contrast isolates roster retirement, rather than prompt-cache invalidation or the usefulness of particular expert prose.

Do not broaden the claim to automatic removal of deleted agents. Registry synchronization is an upsert mechanism, and deletion/readiness semantics need their own contract. Do not use nil registry domains in the primary witness: boot may intentionally record the former domain there, leaving legitimate registry support after a YAML edit. Likewise a domain implied by the agent's name or another source must survive withdrawal of only its KB support.

## Other consumers and existing limits

Knowledge refresh reads the same kernel domain relation, then passes each agent's domains into `KnowledgeSubjects` and publishes wanted subjects. Retired domain evidence can therefore influence acquisition planning as well as prompt routing. Actual provider work remains subject to acquisition availability, refresh rules, budget and source outcomes; this trace does not establish unnecessary network requests or their cost. [Domain reader](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/knowledge_refresh.go#L222-L240), [subject construction](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/system/knowledge_refresh.go#L83-L128).

Current JIT source generations can correctly expose A's changed content; that is not equivalent to retiring its selection premise. Relevance and budget can omit the resulting candidates. A consultation may fail, provide useful general advice despite the outdated classification, or never be requested at the applicable confidence. Generic reasoning, tools and later verification may still produce correct work. Nothing here establishes a capability escalation or permission bypass.

## Proposed source ownership contract

Retain the last successfully published roster measurement under a workspace/session owner and reconcile its source-qualified facts against a new complete measurement. Remove only supports owned by that measurement. Preserve independently asserted facts and policy-derived `/agent_name` domains. A broad `Retract("shard_agent_domain")` is not an acceptable substitute for ownership: it can erase legitimate registry/catalog support and concurrent producers.

Two possible designs are an atomic owner-scoped snapshot replacement or generation-qualified source observations with one active generation. Either should make a successful refresh visible as one coherent transition to selection and background knowledge refresh. Exact old/new set differencing is a smaller option if the kernel provides the needed atomicity and ownership semantics; do not assume that separate retract and assert calls are atomic.

Define incomplete-source behavior explicitly. An unreadable database is not a measured empty domain set. Preserve its last known support as stale/unavailable, or deny claims based on it under policy, rather than silently treating an error as authoritative removal. A successful read returning no tags is a different outcome. Registry, catalog and KB observations should have separate provenance so one failed source does not require freezing every healthy source.

Serialize or fence concurrent refreshes so an older measurement cannot overwrite a newer publication. Include the measured source generation when feasible: a YAML/database change between measurement and publication must not install an obsolete snapshot as current. Kernel clones used by in-flight compilations may legitimately retain their prior snapshot; establish which subsequent task boundary must observe the new one. Avoid mutating live task facts or replacing the selector's existing temporary-fact ownership machinery.

The change is factual reconciliation, not an extra Go expert-selection heuristic. Mangle should continue deriving needs, confidence, roles and review obligations from the active source observations. Advisory text still cannot widen capabilities or waive verification.

## Proposed evaluation and full costs

No evaluation below has been run. Start with a deterministic integration witness through real boot, successful nonempty YAML replacement, B activation, roster refresh and the real selector. Capture both current database tags and kernel supports, not only a returned expert list. The expected contrast is that withdrawing A's only Bubble Tea support removes A from a later Bubble Tea selection while retaining A's ready status and unrelated domains.

Controls should cover unchanged refresh; newly added domains; explicit registry support retaining a domain after KB withdrawal; name-derived and catalog support; nil versus empty registry lists; multiple agents sharing a domain; malformed/empty YAML; failed sync and failed DB reads; unavailable compiler; repeated activation; cold reboot; and concurrent refreshes whose measurements complete out of order. Verify that independent assertions survive, existing selection facts are not retracted, and source-generation changes invalidate stale publications. Also inspect the resulting wanted-subject set without assuming every wanted subject becomes a provider call.

Compare assertion-only behavior, exact owned-set reconciliation and generation-scoped snapshots under the same roster, source edits, kernel and task sequence. Measure false-retained/false-removed domain supports, expert selection correctness, publication consistency, subject-planning changes, SQL work, fixpoint work, CPU, latency, retained snapshot memory and concurrent-refresh contention. Reconciliation adds set storage and comparison, while generation-qualified facts require bounded retirement of old generations. Neither implementation is free.

At the complete-task level, separately assess expert relevance, verified success, unnecessary consultation, repair work, model/tool/provider tokens and cost, and total latency. Include synchronization and embedding work already incurred by activation, failed attempts and acquisition work that actually passes policy. Fewer experts or fewer calls are not automatically better; preserving genuine domain support and task quality is part of acceptance. No runtime saving or answer-quality gain is claimed.

## Relationship to existing work

The specialist-roster workspace audit owns the mismatch between process CWD and the root used to advertise specialists. This finding instead assumes the correct workspace and successful source refresh, then examines retirement of already-published domain support. Expert-selection temporary-fact ownership concerns concurrent task facts, not long-lived roster observations. Expert lending, planned-step card scope and final-repair continuity concern delivery after expert selection; none repairs an obsolete premise used to choose the expert.

The earlier prompt-budget audit already identifies monotonically growing registry entries without pruning. That known registry issue is not claimed here: A remains present and ready throughout this witness. [Earlier registry finding](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/2026-09-09-system-hardening/02-PROMPT-BUDGET.md).

The bounded audit review found no separate owner for this exact refresh-retirement seam. This is not an exhaustive novelty claim. Its acceptance condition is narrow: after successful withdrawal of one source's domain support, future selection must use the reconciled support set, while independent sources, incomplete-read distinctions, concurrent publication and existing executive and safety boundaries remain intact.
