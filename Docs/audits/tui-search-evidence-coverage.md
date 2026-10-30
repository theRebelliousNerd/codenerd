---
doc-class: deep-dive
subsystem: cli-search-evidence
implementation-status: partial
last-verified: 2026-10-30
verified-against: fef98230c plus uncommitted implementation
verification-method: static-source-and-test-definition-inspection; no builds or tests executed
supersedes: []
---

# TUI search coverage and answer evidence

## Decision

Give the TUI's existing local search helper a typed acquisition outcome, and preserve that outcome through both the slash-command display and the natural-language answering request. Keep the current membership boundary, literal matching semantics and kernel routing authority. A list of matching files is useful evidence; a count without those files is not a usable substitute. A capped or partly unreadable search must not be indistinguishable from a complete search.

The narrow gap survives the October 28 file-evidence repair. The shared TUI helper discards whether acquisition stopped at its cap or skipped unreadable member files. Its slash consumer prints ordinary results or an unqualified no-match message. Its residual natural-language consumer stores the paths only in `search_results` and gives articulation a count warning, while the new typed evidence channel receives no search result. This is not a claim that every search goes through that residual path or that the separate `search_code` tool is broken.

**Important alternative route:** VirtualStore explicitly maps `ActionSearchFiles` to `handleSearchCode`; the chat can observe that system execution and return its interpreted output instead. The natural-language loss described here is conditional on **no usable independent execution result reaching that answer path**. The 1.2-second result-observation wait is not evidence that every search falls through, nor an end-to-end execution bound. [Action alias][search-alias], [system-result handling][system-handler], [output interpretation][system-output]

**Evidence status:** static source and test-definition inspection only. No codeNERD execution, tests, model inference, benchmark, failure injection or implementation change was performed. The witnesses and evaluation below are proposed, not reproduced incidents.

## Related primary research

[Liu and colleagues’ long-context study](https://arxiv.org/abs/2307.03172) examines how model access to relevant information can vary with its position in context. It motivates evaluating the actual answering request, not inferring answer quality from an internal fact alone; it does not prove a failure in this CodeNERD path.

## Scope and source freeze

Main first resolved to `32b8417bb9b0ec70a83b06226c67c906d0b30be0`. It was independently re-resolved and all relevant source files were refetched at [`6cb5f88e00eaf939d10dee46c16c68baf4b53d57`][main] at approximately 05:12 UTC on October 30. The compare between those revisions contains one continuation/persistence commit and changes five other files; the helper, callers, routing and evidence code examined here are unchanged. The claims below cite the newer pin.

The [root guidance][guidance] places executive decisions in Mangle, makes tools serve evidence acquisition and reduced cognitive work, and requires bounded model requests without arbitrary task-ending call limits. The [architecture-document standard][standard] requires current behavior, proposed contract and checkable exits to remain distinct. Scoped [chat guidance][chat-guidance] was inspected; it adds no alternative search contract.

This is a search-specific residual to the existing TUI work, not a new claim that its repairs are absent:

| Existing account | Boundary preserved here |
|---|---|
| [TUI evidence turn ownership][tui-audit] | Its as-built section records repaired result baselines, bounded file observations, actual file-body delivery and the gather-evidence lane. This note concerns the still-separate search helper and its two consumers. The earlier audit's late-result ownership and multipage file-reading residuals are not refiled. |
| [Workspace search navigation fidelity][navigation-audit] | Its producers are the modular `search_code` tool and the VirtualStore action using the shared observation codec. The TUI `searchInFiles` helper does not call that codec. This note does not repeat lexical-to-semantic projection or symbol-address findings. |
| [Undeclared assertion inventory][undeclared] | It already names `search_results`. The assertion's absence from the declared surface is not newly discovered here. The concrete user/model delivery and search-coverage consequences are the subject. Merely adding a declaration is not the proposed repair. |

The relevant prior documents and targeted repository searches were inspected; this is not an exhaustive claim to have read every historical audit or research branch.

## What runs today

### The direct slash route

The command dispatcher calls `handleCmdSearch` directly for `/search`. The handler joins its argument tokens into a literal pattern, calls `searchInFiles(m.workspace, pattern, 20)` synchronously, and adds its result to conversation history. It does not delegate a model-driven search. [Dispatcher][dispatch], [handler][slash]

`searchInFiles` refreshes the workspace membership view, walks the supplied root, and asks `mem.Admit` before reading a file. Each eligible readable file contributes one path if `strings.Contains(string(data), pattern)` succeeds. Thus its count is **matching files**, not occurrences or matching lines. Dot directories can be members; default-excluded dependencies remain excluded. These are useful existing semantics to preserve. [Membership setup][membership], [helper][helper], [membership fixture][membership-test]

### The residual natural language route

The normal policy maps `/search` to `/search_files`. Its `next_action` derivation is guarded by absence of `executive_processed_intent(/current_intent)`. The later part of `processInput` queries those actions and passes them to `executeInfoGatheringActions`. These are real entry points, but their presence does not prove that every natural-language search reaches the helper. [Mapping][mapping], [action derivation][next-action], [caller][residual-caller]

For a `search_files` action, the helper uses `intent.Target` as its literal pattern and cap 10. On success it appends:

- a `search_results(target, joinedPaths)` fact to `executionResults`;
- a warning saying how many matches were found.

It does **not** append a search observation to the separately returned `evidence` slice. If search returns an error, this branch adds no diagnostic or partial result. [Residual search branch][residual-search]

The caller loads `executionResults` into the kernel, then constructs `ConversationContext` with the separate `evidence` slice. Articulation renders that typed evidence, history and warnings. It does not receive the local `executionResults` collection. The inspected default injection bridge consumes `injectable_context`, not arbitrary EDB contents, and the current undeclared-assertion inventory explicitly includes `search_results`. A custom policy or independent prior context could supply evidence; loading these facts is not the shipped delivery guarantee. [Caller and context construction][delivery], [actual rendering][rendering], [default bridge][prompt-bridge], [inventory][undeclared]

## Finding S1 acquisition loses its coverage

The helper's return shape is only `([]string, error)`. Two decisions destroy information needed by either consumer:

1. When the count reaches `maxHits`, a later eligible file returns `filepath.SkipDir`; the function does not record that some domain was skipped or that the result saturated.
2. When `os.ReadFile` fails for an admitted file, the walker returns nil and continues. The result records neither the failed file nor the resulting uncertainty. [`searchInFiles`, lines 331 to 360][helper]

Do not conflate these read failures with every filesystem error. A walk callback error or membership-admission error is propagated. The slash consumer shows `Search failed` for a propagated error. However, any paths accumulated before that error are discarded from the display, and the residual natural-language branch emits neither the error nor those paths. [Helper][helper], [slash handler][slash], [residual branch][residual-search]

The slash consumer prints `No matches found` for an empty successful slice, or `Found N matches` and the paths otherwise. There is no partial/budget/failed-member banner. It cannot distinguish an exhaustive 20 matching files from the first 20 of 21, because the helper erased that distinction. The wording does not literally say “these are all matches”; the contract problem is that the interface supplies no evidence by which the user or later model can distinguish complete from incomplete. [`handleCmdSearch`, lines 390 to 413][slash]

### Unexecuted source witnesses

**Cap witness:** a workspace has 21 ordinary admitted readable files containing the literal needle. The slash route returns no more than 20 paths without disclosing omitted acquisition. The natural-language helper's analogous boundary is 10 versus 11. An exact-cap result must not automatically be labeled known-truncated either: without an additional observation, exactly N and more-than-N remain distinguishable only as “at the limit; completeness unknown.”

**False-absence witness:** membership and directory traversal succeed, but the only matching admitted file fails at `os.ReadFile`. The helper returns no paths and no error for that failure; the slash handler takes its ordinary no-match branch. This requires the error to occur at the file-read branch, not earlier in membership/traversal.

**Mixed-partial witness:** several paths are found before another member cannot be read. The displayed positive matches remain true, but the result gives no reason to distrust an inference that the remaining workspace contains no further matches.

These witnesses concern acquired lexical evidence, not permission widening, semantic reference proof or a demonstrated wrong edit.

## Finding S2 residual search paths do not reach the answering request

For a residual search, the paths are present in a local Go value but only the count enters warnings. The newly repaired evidence route does not pick them up. `gatherWorkspaceEvidence` selects perception's `ScopeFile`, else a path-like `Target`; it does not accept search results. With a plain search term and no file scope, it returns a typed unavailable observation saying that no file was named. That is better than pretending to have read a file, but it is not a delivery mechanism for already discovered matching paths. [Residual search][residual-search], [target selection and fallback][gather], [prompt delivery][delivery]

### Exact conditions and exclusions

The natural-language consequence requires all of the following:

1. The turn survives classification and earlier exits and reaches the residual information-gathering section.
2. Its queried actions actually include `/search_files`; the executive-processed guard has not removed that derivation.
3. The local search obtains relevant paths that the answer would need.
4. No successful independent system-result branch returns an answer with the same evidence, and no other supplied context already gives those paths.
5. Articulation is then called with only the local count warning and whatever separate evidence observations exist. [Residual caller][residual-caller], [delivery][delivery], [system handler][system-handler]

A proposed deterministic route witness uses a valid `/query` `/search` intent with a concrete non-path search term, a suggested search tool, candidate confidence below the configured delegation threshold, and no clarification or earlier lane. The evidence signal prevents direct-answer routing; the failed delegation gate permits `/gather_evidence`. The helper can still derive `/search_files` when the executive has not marked the intent processed. The fixture must control the independent system executor rather than assume it does not run. This is a reachable contract shape to test, **not a measurement of how often perception generates it in production**. [Delegation gate][delegation-gate], [evidence lane][evidence-lane], [action guard][next-action]

The existing tests establish the intended low-confidence and tools-only gather-lane definitions. They were read, not run here. They do not establish the proposed `/search` composition witness, which needs its own actual-request capture. [Gather tests][gather-tests]

### Alternative routes that can already work

- A confident search can take the delegated route. `processInput` returns a background-start message for policy-selected read-only background work, rather than executing this residual articulation path. This is not a missing delegation capability. [Delegated/background branch][delegated]
- `handleSystemDelegations` inspects execution results, waits when appropriate, and can return a formatted/interpreted system result before the residual articulation call. It also exposes routing blockers. Such an independent successful search can rescue the user journey even while the local helper's paths are not delivered. [System handling][system-handler], [output interpretation][system-output]
- `handleSystemDelegations` immediately returns no result when the kernel or shard manager is absent; an empty result with no delegation also falls through. Its 1.2-second observation wait does not itself prove a pending action completed or never ran. The existing TUI ownership audit owns the broader late-work/retry issue. [System handling][system-handler]
- The modular `search_code` tool and VirtualStore search build `observation.Search` and call the shared codec. Their results and expansion behavior are a different path; the source gap here must not be described as “all search loses evidence.” [Modular producer][modular-search], [VirtualStore producer][virtual-search]
- Direct `/search` does display its returned paths. Its defect is missing coverage, not the natural-language path's missing-path delivery. [Slash rendering][slash]

## Proposed outcome contract

This section is proposed design, not a claim that these fields or predicates exist.

The common outcome should identify the literal query, workspace/domain and applicable exclusions, matching **file** identities, acquisition completion state, configured limit, known skipped/read-failed members, any terminal traversal error, and whether continuation or targeted reacquisition is available. Positive matches remain useful even when coverage is partial. Empty and complete may support “none found in the searched domain”; empty and partial may not.

The host should acquire and report bounded observations. Mangle should continue to determine routing, whether additional evidence is required and which available continuation is admitted. Do not introduce a second Go planner or broaden permissions to make every search finish.

For the smallest correction:

1. Preserve the current literal matching behavior and membership checks.
2. Make cap/failed-member/traversal dispositions observable at the helper return.
3. Render returned paths plus coverage into both the slash response and the actual answer request. Do not rely on adding a declaration for `search_results` without a consumer.
4. Keep all-body file reads distinct from search evidence: finding a path does not mean the answer model has seen that file's body.
5. If further investigation is needed, provide a supported, bounded continuation or a truthful unresolved outcome. Never substitute an arbitrary fixed call count for completion.
6. Define cancellation and acquisition-byte boundaries before advertising a resource guarantee. A displayed result cap is not an input-byte budget.

Reusing the shared observation types/codec is an option, not an instruction to call a tool outside its normal authority. Its regex-oriented modular acquisition and the TUI's literal matching are not interchangeable without an explicit semantic adapter. A narrow typed result for the existing helper may be cheaper and preserve behavior, while a shared backend reduces parallel implementations but adds integration and ownership work. Neither option justifies reopening the existing workspace containment or restriction gates.

## Costs and alternatives

The current helper calls `os.ReadFile` for each admitted file it searches. Its matching-file cap does not bound the size of a single file, or how many nonmatching files must be read. It accepts no context and performs no cancellation checks. The slash handler performs the call inline before returning its Bubble Tea command result. These are source-established resource exposures, not measured memory or latency incidents. [Helper][helper], [slash handler][slash]

Report separate quantities rather than combining unlike units into a speculative score:

| Dimension | What to measure |
|---|---|
| Acquisition | Directory entries visited; membership-refresh cost; files attempted/read/failed; bytes read; repeated reads; time spent after cancellation. |
| Memory and CPU | Whole-file buffers; byte-to-string conversion and substring work; retained path bytes; outcome serialization; peak live memory and allocation rate. Do not claim an allocation profile from source alone. |
| UI and completion | Slash-command input blocking; time to first useful paths; time to grounded final answer; cancellation settlement; human recovery time. |
| Model/provider | Actual added prompt bytes/tokens; input, output, cached-input and reasoning usage as reported; clarification, retry and corrective turns; specialist and synthesis calls. Do not double-count token subcategories. |
| Evidence quality | Correct located-file recall within the declared domain; false absence; false completeness; path-delivery loss; unsupported claims about unseen file bodies; unnecessary abstention. |
| Storage and lifecycle | Any retained observations, handles, eviction/expiry costs and replay/reacquisition work, but only if the chosen design introduces retention. |
| Safety | Membership/containment equivalence, no additional grants, no hidden execution or mutation, and no use of a stale search as current edit authorization. |

Compare three controlled alternatives:

- **Typed helper plus direct delivery:** minimal surface change, no model call inherently required; adds bounded result metadata and prompt paths. Retains duplicate search implementation unless separately consolidated.
- **Shared acquisition/observation backend:** can align coverage, cancellation and expansion behavior; requires preserving literal semantics, admission, source identity and consumer-specific output limits.
- **Route every evidence search through a working executor:** richer continuation, but can add model calls, tool-catalog tokens, task setup and waiting cost for trivial searches. It must be compared under the same authority and evidence quality, not credited merely for hiding the residual branch.

A helper that reads a file successfully but does not deliver its useful result can cause repeat scans or human repair; those are conditional downstream costs to measure. No failure rate, speedup, token saving or net monetary benefit has been established.

## Evaluation before claiming completion

All work in this section is proposed and unexecuted.

### Deterministic source and request witnesses

| Case | Required observation |
|---|---|
| 0, 19, 20 and 21 slash matching files | Correct file count and paths; declared coverage distinguishes complete from at-limit/partial; no invented exact omitted count. |
| 0, 9, 10 and 11 residual matching files | Same contract at cap 10, including actual answering-request capture. |
| Only matching member fails at read | No unqualified absence; failed/unknown coverage is visible without exposing protected contents. |
| Positive matches then a read error | Preserve true matches and disclose incomplete coverage. |
| Positive matches then walk/admission failure | Typed partial plus error, or an explicit chosen failure contract; no silent disappearance represented as a successful complete search. |
| Unique matched filename sentinel | Sentinel/path and coverage reach the real articulation request, not merely an intermediate Go result or kernel assertion. |
| Tools-only, non-path search intent | Gather fallback does not erase or contradict a successful search observation by treating “no named file” as “no search evidence.” |
| Low-confidence residual versus confident delegation | Prove the intended route conditions; keep successful delegated behavior and do not broaden its confidence gate. |
| Independent system search succeeds, fails, is absent or delayed | Attribute which outcome supplied the answer; no false assertion that the residual helper owns every search; no duplicate work introduced as a delivery repair. |
| Large nonmatching file and many small files | Measure acquisition bytes/memory/cancellation separately from displayed result count. |
| Dot directory, default-excluded dependency and denied/outside source | Preserve existing membership and containment semantics. |
| Literal punctuation and repeated occurrences in one file | Preserve substring behavior and matching-file counts; do not silently turn the query into a regex or occurrence count. |
| Follow-up asks which file to inspect | The retained conversation or supported handle carries the returned paths and their coverage, not just the original count. |

The existing membership fixture and file-evidence prompt sentinel are useful controls but not proof of search delivery. Extend the actual normal caller composition, not only a renderer test with hand-built evidence. [Membership fixture][membership-test], [file-evidence request fixture][file-test]

For model evaluation, use paired held-out user requests on the same repository, provider/model/version, budgets, permissions and starting context. Include slash search followed by explanation, natural-language literal lookup, capped workspaces, admitted read errors and follow-on inspection. Use ordinary production routing with world/CodeDOM context enabled; a fixture with deliberately unavailable system rescue establishes the seam but cannot estimate production prevalence. Randomize baseline/treatment order and include warm/cold caches. Independently score path correctness, stated coverage, supported answers, false absence/completion and human repairs. Report failures and uncertainty alongside latency and resource distributions.

### Proposed completion gates

| Research gate | Checkable exit |
|---|---|
| TUI SEARCH COVERAGE | The real slash and residual callers distinguish complete, saturated/partial and failed acquisition; positive evidence is preserved and empty partial results cannot claim complete absence. |
| TUI SEARCH DELIVERY | A unique discovered-path sentinel and its coverage are present in the actual answer request under the controlled residual trigger. |
| TUI SEARCH PARITY | Literal matching, file-count units, membership, restrictions and successful alternative routes remain unchanged or have an explicitly accepted new contract. |
| TUI SEARCH RESOURCE ACCOUNTING | Measured acquisition and cancellation bounds are separate from result/prompt bounds; any newly introduced limits yield honest partial outcomes. |
| TUI SEARCH USER VALUE | Paired normal-entry journeys preserve or improve evidence correctness with full reported cost and no increase in prohibited effects. |

These are proposed research gates, not existing gap IDs or closed architecture rows. Before implementation, add the selected contract and checkable exits to the owning CLI capability spec, then cite implemented wiring and executed evidence. The useful first milestone is narrow: **the TUI reports which files it found and what it did not establish, and its answer model receives that same evidence.**

## Sources

[main]: https://github.com/theRebelliousNerd/codenerd/commit/6cb5f88e00eaf939d10dee46c16c68baf4b53d57
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/agents.md#north-star
[standard]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/Docs/journeys/09-architecture-doc-standard.md#the-citation-rules-unchanged-from-2026-09-20-because-they-were-measured
[chat-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/agents.md#L1-L5
[tui-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/Docs/audits/tui-evidence-turn-ownership.md
[navigation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/Docs/audits/workspace-search-navigation-fidelity.md
[undeclared]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/core/defaults/testdata/undeclared_asserts.txt#L1-L54
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/commands.go#L159-L160
[slash]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/commands_handlers_analysis.go#L381-L419
[membership]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/helpers_files.go#L20-L34
[helper]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/helpers_files.go#L331-L360
[membership-test]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/helpers_test.go#L64-L99
[mapping]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/core/defaults/policy/delegation.mg#L137-L143
[next-action]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/core/defaults/policy/delegation.mg#L567-L577
[residual-caller]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/process.go#L888-L954
[residual-search]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/process_helpers.go#L122-L133
[delivery]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/process.go#L928-L994
[rendering]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/helpers_articulation.go#L251-L275
[prompt-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/core/defaults/policy/prompt_context.mg#L27-L83
[gather]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/turn_evidence.go#L111-L154
[delegation-gate]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/core/defaults/policy/delegation.mg#L351-L356
[evidence-lane]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/core/defaults/policy/routing_arbitration.mg#L289-L310
[gather-tests]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/turn_evidence_test.go#L139-L218
[delegated]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/process.go#L520-L574
[system-handler]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/process_dream_delegation.go#L159-L196
[system-output]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/process_dream_delegation.go#L199-L249
[search-alias]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/core/virtual_store_routing.go#L445-L446
[modular-search]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/tools/core/search.go#L696-L744
[virtual-search]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/core/virtual_store_file_actions.go#L445-L543
[file-test]: https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/turn_evidence_test.go#L235-L268

## As built (2026-10-30)

The research text above describes its pinned revision. Current-source review at
`fef98230c` confirmed both findings before these edits. Commits: none; this
implementation is uncommitted. Status is **partial** because behavioral and
fail-before execution remain pending, not because either delivery repair is
intentionally unwired.

| Finding or boundary | Ruling and current implementation |
|---|---|
| S1: lost cap/read-failure coverage and discarded partial positives | **Real; repaired in source.** `cmd/nerd/chat/search_evidence.go:20` defines the typed outcome; acquisition at `:41` retains paths, observed failure count, first diagnostic sample, terminal error, attempted files and bytes read. The cap stops at another admitted file and reports unknown remaining coverage. `renderSearchEvidence` (`:189`) serves both consumers; an incomplete empty search cannot establish absence. |
| S2: residual paths never reach the answering request | **Real; repaired in source.** `process_helpers.go:123` appends the outcome to typed evidence, `turn_evidence.go:135` preserves it without reinterpreting the literal as a file request, and `helpers_articulation.go:253` renders it in the actual request via `process.go:999`. Explicit file scope still obtains a separate body observation. |
| Undeclared `search_results` assertion | **Real; removed.** Delivery uses the existing evidence channel, so no declaration or dormant consumer is added. Its entry is removed from `internal/core/defaults/testdata/undeclared_asserts.txt`; no baseline grows. |
| Prior file-evidence and gather-lane repairs | **Already present, preserved.** `turn_evidence.go:58` still reads typed whole/prefix/unavailable bodies; `internal/core/defaults/policy/routing_arbitration.mg:299` still derives the gather lane and `policy/delegation.mg:356` retains the configured delegation gate. No Mangle files or prompt atoms change. |
| Independent successful routes | **Exclusion confirmed.** `process_dream_delegation.go:159` still observes system results and may return before residual articulation; `internal/core/virtual_store_routing.go:445` still aliases search to the shared-codec producer. The modular producer (`internal/tools/core/search.go:717`) remains separate. This repair claims no change in route prevalence or delayed-result ownership. |
| Resource exposures | **Real; partly reduced.** Streaming literal acquisition (`search_evidence.go:137`, `:151`) replaces whole-file buffering, checks cancellation and records bytes acquired. The cap remains a matching-file limit. Slash acquisition, membership refresh and individual filesystem calls remain synchronous; total scan bytes, latency, allocation profiles and cancellation settlement are not qualified. |

Membership and literal file-count semantics remain the domain contract. One
preservation assumption required strengthening: the old helper admitted a
symlink's path and then followed its target in `os.ReadFile`; membership alone
was not a canonical containment check. `searchMember` (`search_evidence.go:103`)
now uses the existing `tools.ResolveWorkspacePath`, reports denied acquisition,
and refuses nonregular targets before opening them. No permission is widened.

Authored regression definitions are in `cmd/nerd/chat/search_evidence_test.go`:
`TestSlashSearch_CoverageBoundaries` (0/19/20/21 and retained-history follow-up),
`TestProcessInput_SearchDeliversPathsAndCoverage` (0/9/10/11 through real policy,
the default workspace scanner, residual helper and answering-request capture with no independent system
rescue), `TestSearchConsumers_AdmittedReadFailure`,
`TestSlashSearch_PartialTraversalPreservesPositivePaths`,
`TestSearchInFiles_AtLimitDoesNotInventOmittedMatches`, unavailable-domain,
streamed-literal/cancellation, outside-symlink, explicit-scope and confident-route
controls. The existing membership fixture in `helpers_test.go:65` uses the new
outcome. Permission-failure fixtures explicitly skip on hosts that can read
mode-000 files/directories.

**Fail-before evidence is source-derived only:** the prior slash response has no
coverage field, and the prior residual request has no discovered filename
sentinel; the new dispatcher/request assertions target those omissions through
unchanged production entry points. Neither a pre-fix failure nor a post-fix pass
has been executed. No build, test, model evaluation, performance measurement or
corpus regeneration ran. The selected contract and open validation gate are in
`Docs/architecture/cli/03-GAP-ANALYSIS.md`; source wiring and limitations are in
`Docs/architecture/cli/IMPLEMENTED_SPEC.md`. Root-run tests, production user-value
evaluation and measured resource qualification remain outstanding. No continuation
handle or automatic retry is advertised.

