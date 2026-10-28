---
doc-class: deep-dive
subsystem: research-context
implementation-status: proposed
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Research observation identity and admissible search evidence

## Decision

Give each acquired research observation its own source identity, acquisition outcome and immutable projection before the session adopts it into working context. Preserve the browser-backed tools, the feature branch's runtime ownership repair, the existing working-context archive and Mangle's control of admission, freshness and retention.

The distinct gap is not that research is unwired or that tools cannot return text. It is that **a successful tool result can carry a stronger evidence meaning than the host established, and can then be dated against a different source**. The DuckDuckGo adapter labels any parseable page without recognized cards as no results. Fetch paging navigates again without binding the continuation to the first document. Redirect identity and link base are not carried. Finally, session working context assigns URL/query results to its current workspace focus and that file's revision.

A better model prompt alone cannot reconstruct the missing source identity, distinguish a challenge page already collapsed to “No results,” or know whether two offsets came from the same document. The proposed repair belongs at acquisition and context adoption, with JIT guidance consuming truthful typed results.

**Evidence status:** static source diagnosis and proposed design, not a reproduced user incident or measured defect rate. No repository files, implementation, tests, codeNERD execution, provider inference, training or benchmarks were changed or run. Main and the current feature head were read through GitHub; local outputs are research material only. Source-derived witnesses below are conditional traces, not executed fixtures.

## Source freeze and relation to existing work

The inspected refs were main at [356f65e](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871), feature `feat/conversation-brief` at [c2d39e5](https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c), and the audit inventory on `codenerd-research-docs` at [9928930](https://github.com/theRebelliousNerd/codenerd/commit/9928930b91ab7723c291b0a372f47cfcb24f1d78). The feature's research reader differs materially from main: it leases one runtime, joins its DOM watcher, checks observer completeness and reports cleanup failure. Those improvements are credited, not re-reported as missing. The relevant `working_context.go`, `working_set.go`, `working_set.mg` and `working_revision.go` contents were individually compared and are byte-identical across main and feature. The acquisition findings were verified separately in both versions rather than inferred from a potentially bounded compare-file list.

The [root guidance](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md) makes relevant, recoverable, revision-aware evidence a harness responsibility and reserves executive decisions for Mangle. [Prompt guidance](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/agents.md#L47-L58) distinguishes stored/retrieved knowledge from what a consumer actually receives. This proposal applies those existing principles to external research observations.

It does not duplicate:

- **Research browser ownership and evidence retirement:** runtime leases, cleanup, producer joins and transient facts. That audit already proposes retaining a compact receipt; this report specifies what the retained observation must mean and how the working ledger must adopt it.
- **TUI evidence turn ownership:** residual workspace reads, turn correlation and first-articulation delivery. A host reading a local file and a browser reading a remote document need different source identities.
- **Terminal knowledge synthesis grounding:** carrying parent answer context through specialist consultation into the final request. That consumer should inherit correct research observations, but cannot repair an acquisition already mislabeled upstream.
- **Retrieval freshness coverage and context value:** workspace keyword/semantic retrieval caches and tier outcomes. This is not another scanner-cache invalidation proposal.
- **Context ledger prefix costs:** reducing repeated revision collection and policy materialization. Efficiently hashing the wrong file does not establish a remote document's revision.

The [research-read spec](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/tools/13-RESEARCH-READS-THROUGH-BROWSER.md) already warns that live DuckDuckGo was not verified and may return a challenge. That warning is not a newly discovered possibility. The new actionable diagnosis is the exact promotion from an unrecognized but successfully fetched page to a successful negative search result, and its downstream treatment.

## The actual research to answer path

The following is the normal delegated investigation route when routing selects an executor and the effective catalog and authorization admit the research tool. It is not a claim that every TUI question uses this route.

| Boundary | Current behavior and evidence |
|---|---|
| TUI task handoff | [delegation.go 35–74](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation.go#L35-L74) calls the task executor and, where available, preserves a structured observed return. |
| Task execution | [task_executor.go 352–409](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L352-L409) uses the kernel's isolation decision; the inline route clones the executor and calls `ProcessWithIntent`. The clone owns its task working archives. |
| Tool registration | [virtual_store_tools.go 45–113](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/virtual_store_tools.go#L45-L113) hydrates modular/global registries; research [RegisterAll](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/register.go) installs browser-backed search/fetch, with grounded provider search separately conditional. |
| Authority | [constitution.mg 243–264](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/constitution.mg#L243-L264) includes research classifications, but exact request permission, restrictions, effective catalog and confirmation still apply. Registration is not universal availability. |
| Acquisition | Feature [browser_read.go 119–274](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/browser_read.go#L119-L274) navigates and checks HTTP status, load, quietness and sensor completeness, then returns HTML/text; adapters project that document. |
| Tool success | [registry.go 412–455](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/registry.go#L412-L455) records success from nil tool error. Session [executor_tools.go 943–1051](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L943-L1051) retains output, records the observation and returns a non-error result after successful execution. |
| Working adoption | [working_context.go 368–431](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L368-L431) assigns entity/revision/kind, stores the body and allocates a recall handle. |
| Subsequent answer requests | [working_context.go 577–642](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L577-L642) carries whole results or explicit archive handles into subsequent provider requests. Ledger policy controls stale/superseded retention. |

There is also a provider-native `grounded_web_search` tool. Its [structured output](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/grounded_web_search.go#L54-L121) preserves answer text, citations, retrieved pages and usage. Do not attribute the DuckDuckGo parser defect to this provider path. Its query-only result nevertheless encounters the same generic session adoption code if executed in the working loop. Provider citations, fetched page bodies and local source observations should remain distinct evidence kinds.

## Finding one remote evidence inherits a workspace revision

The session recorder extracts a candidate entity only from `path`, `file_path`, `file`, `target` or `working_dir`. Research tool schemas use `url`, `query` or `topic`. When no file entity or CodeDOM ref is found, `observed = loop.focus`; `loop.set.Revision(observed)` supplies the stored revision. [Recorder](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L368-L422).

This is more than an imperfect display label:

- [Revision collection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_revision.go#L55-L110) resolves entities as workspace paths and hashes files or parsed elements.
- [Recall](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_set.go#L107-L120) reports a current revision and stale flag using that entity.
- [Ledger rules](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_set.mg#L186-L203) derive staleness from unequal revisions or unavailable/absent sentinels. [Compaction and hotness](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_set.mg#L257-L300) then use those judgments for live/hot status and eviction. Results are archived, not irrecoverably deleted.

**Source-derived witness A:** a task reads `client.go`, then fetches external API documentation. The remote observation receives `client.go` and its local digest. Editing `client.go` can make the documentation observation stale even though the remote snapshot is unchanged. Conversely, unchanged local bytes do not prove the external resource is still current. This does not assert an observed bad final answer; it establishes a false dependency in host freshness accounting.

**Witness B:** when the intent target is not a resolvable workspace file, `normalizeWorkingEntity` uses `.`; initialization makes that the focus. A directory is not a readable file digest, so the normal revision path can yield unavailable. A research-only result can therefore be immediately unresolvable/stale for a workspace reason, even though its browser acquisition succeeded. [Initialization and normalization](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L215-L237), [normalization](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L301-L321), [digest](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_revision.go#L175-L187).

Important limits:

1. Distinct URL/query arguments are **not automatically collapsed**: the recorder hashes tool name plus marshaled input into `Kind`, and the observation ID also includes body and other data. The defect is source ownership/freshness, not loss of every query identity.
2. These archives are task/session scoped; this is not evidence that all research becomes globally persistent knowledge.
3. No browser revalidation is performed by checking a local digest. A remote snapshot can remain valid historical evidence while its current applicability is unknown.
4. File-focused relevance and source identity can both be useful. Documentation may be relevant to `client.go` without having `client.go` as its source or freshness dependency.

## Finding two an unrecognized search page becomes negative evidence

The DuckDuckGo parser walks expected result-card markup. If no recognized cards yield both title and URL, it returns an empty result list with nil error. The caller then returns “No results found for” the query, also with nil error. It does not require a recognized empty-results state, validate a matching query response or expose an unrecognized-layout outcome. [Feature parser and acquisition](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/web_search.go#L124-L179), [negative-result projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/web_search.go#L62-L90).

A static HTTP-200 document containing a challenge, a consent page or changed card markup can satisfy transport and stability checks while producing no recognized cards. The feature's improved observer completeness correctly establishes that a page was observed to be quiet; it does not establish that the quiet page was a search answer. The adapter discards the page body at this boundary, leaving later reasoning no opportunity to distinguish those cases.

The output is subsequently a successful registry execution and a non-error tool result. That is not itself a false claim that the whole investigation succeeded, but the model receives an unsupported negative-search formulation rather than the actual uncertainty. The learning/metrics path also records transport/parser return success, which should not be interpreted as research evidence utility.

The 1 MB search guard is already marked when it trips. Preserve that honesty. Even an honestly partial recognized result page supports “these hits were observed,” not an exhaustive absence claim. An explicit `max_results` similarly scopes what was requested; it should not become web-wide completeness.

**Required distinction:** acquired page; recognized search response; returned hit set; confirmed provider empty state; challenged/authentication-required; unrecognized layout; transport failure; and partial projection. Unknown must not be silently converted to empty. A recognized provider-empty page still supports only a scoped search outcome at its acquisition time, not “the fact does not exist.”

## Finding three the fetched document loses its retrieval identity and link base

The browser page record returns `URL: rawURL`. The DOM extraction returns HTML and text only, not the current document URL or effective base URI. The status subscriber filters for document responses but does not retain request/frame/loader identity in `documentStatus`. [Feature status acquisition](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/browser_read.go#L98-L105), [listener](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/browser_read.go#L161-L187), [return](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/browser_read.go#L260-L274), [DOM extraction](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/browser_read.go#L454-L474).

For a redirect, the requested URL remains a useful locator, but it is not a complete description of which document supplied the returned bytes. This report does not claim a measured cross-frame status failure; the source establishes that the retained record cannot demonstrate navigation-specific status provenance.

The Markdown converter accepts a `baseURL` parameter, but passes it recursively without using it to resolve anchors. At [web_fetch.go 272–277](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/web_fetch.go#L272-L277), it emits the raw `href`. Therefore the correct diagnosis is **unresolved relative references**, not “links are resolved against the wrong requested URL.”

A document requested through an old URL, redirected into a versioned documentation directory, with `href="migration.html"`, can return useful content plus a relative link whose proper base is not in the result contract. A model may infer the right target, but the harness has not preserved it deterministically. Absolute links already present remain useful. DuckDuckGo's redirect-wrapper decoding is a separate existing repair and must remain intact.

The relevant standard is narrow: relative references require an established base, and a redirected retrieval uses the final retrieval URI absent a higher-precedence base. The browser's effective `baseURI` also accounts for the document's base-URL algorithm. [RFC 3986 section 5.1](https://www.rfc-editor.org/rfc/rfc3986#section-5.1), [WHATWG DOM baseURI](https://dom.spec.whatwg.org/#dom-node-baseuri). These sources support the identity/link contract, not an empirical claim about codeNERD's failure frequency.

## Finding four a paging offset is not bound to the original document

Every `executeWebFetch` invocation calls `readBrowserPage` before applying `offset` and `max_length`. The reader opens and closes a fresh tab. The continuation hint names an offset only; there is no snapshot handle or expected content digest in the fetch schema. [Fetch and projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/web_fetch.go#L56-L117), [paging contract](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/web_fetch.go#L133-L157).

Each individual projection is rune-aware and reports the remaining size. That is good local behavior. It does not compose into a same-document paging guarantee: inserting text before the previous offset between navigations can duplicate content when the hint is followed; deleting text before that offset can skip material. Neither sequence establishes coherent coverage of one snapshot. This is a conditional changing-source witness, not a claim that static documentation always changes between calls.

The existing working archive does not automatically solve this tool-level case. The tool returns the selected window, so the session archives that returned window; it does not receive the omitted document remainder. By contrast, an unpaged full `web_fetch` result may already be archived whole by working context and paged through `recall_context`. Preserve that working path rather than inventing a second universal cache.

The adapter needs to distinguish two requests: “continue this acquired snapshot” and “refresh this URL, then show a region.” A refresh may be desirable, but must not masquerade as a continuation of earlier evidence.

## Proposed observation contract

Use one typed research-observation envelope carried through the existing tool, archive and request paths. The fields below are design requirements, not new API names asserted to exist.

| Concern | Required meaning |
|---|---|
| Ownership | Parent task/turn/tool-call identity and authorized runtime context; no cross-owner archive adoption. |
| Source | Research kind; requested locator/query; resolved document locator where observed; provider/adapter; effective base URI for extracted links. |
| Acquisition | Timestamp, navigation identity where available, measured transport status and semantic response classification; explicit unknown rather than invented success. |
| Snapshot | Digest of the sanitized representation actually retained, representation/converter version and immutable body handle. A digest identifies observed bytes, not eternal remote freshness. |
| Coverage | Requested domain, observed domain, projection span/units, guards, extraction deficits and continuation availability. |
| Applicability | Historical/currently revalidated/unknown under a declared freshness policy; never borrowed from a local file's digest. |
| Context relation | Optional task-relevance links to local files, separate from source identity and invalidation dependencies. |
| Cleanup | Existing feature cleanup disposition, separate from content coverage and semantic validity. |

Go should measure navigation identity, extract and sanitize content, calculate projection metadata, and persist owned snapshots. Mangle should derive admissibility for a specific answer claim, revalidation need, continuation, retention and completion limits. New model-facing explanations should be JIT atoms selected for actual capabilities. Do not introduce an ad hoc second planner or let source text assign its own trusted outcome.

### Adoption and freshness

Extend the existing observation/working-record seam to accept a typed source descriptor instead of inferring all sources from file-shaped arguments. Preserve file/element revision behavior and their efficient shared collection. URL/query/topic observations must no longer inherit workspace focus; local relevance can still influence selection through a separate relation.

A saved remote snapshot is immutable historical evidence. Treat “the retained bytes are intact,” “the remote source was revalidated,” and “this fact is sufficiently current for this task” as separate judgments. Unavailable revalidation should narrow claims, not erase known historical content or mark it current. Do not turn every working-context round into a network request. Derive revalidation only when the answer's currency requirement, policy, elapsed observation age or contradictory new evidence requires it.

Exact query/provider/configuration identity is important for reuse; search output should not be treated as a stable document keyed by query alone. Retain its acquisition snapshot, query scope and result-page classification.

### Search outcomes and bounded recovery

Recognize valid result and empty-result states using an adapter-specific, versioned measurement contract. Known challenge/consent/login states should produce a typed unavailable outcome; unfamiliar states remain unknown. Avoid a huge brittle phrase denylist pretending to prove completeness. A recognized container plus query/response identity and explicit empty state is a positive witness; absence of selectors is not.

The executive may select a permitted alternate provider, retry a transient failure, ask for necessary user interaction, or issue a partial answer. Do not automatically click through challenges, sign in, change browser profiles or widen network/tool authority. Failed semantic acquisition should not consume unbounded retries or become a misleading success in research-quality metrics.

### Snapshot continuation and links

Retain the sanitized full representation once under existing bounded storage policy, then page by observation handle and declared units. Include the snapshot identity in every window. A later refresh creates a new observation and may relate it to the earlier one; it does not splice into the old offset sequence.

If full retention exceeds permitted storage/process bounds, expose a partial snapshot with a truthful nonrecoverable remainder, or a continuation mechanism that validates the same source version. Do not advertise a cursor that silently re-fetches another generation. Keep the current browser requirement; this proposal does not restore an HTTP bypass.

Capture current document URL and effective base URI with the rendered snapshot. Resolve permitted relative links deterministically or carry the explicit base alongside them. Treat resolved destinations as source data, not authorization to navigate. Strip or redact credentials and sensitive URL data under the same evidence policy before persistence or model delivery.

## Cost and capability tradeoffs

This work should improve the value of research per accepted answer, not maximize the number of successful tool calls.

**Likely overheads:** source descriptors and typed statuses add small metadata; snapshot hashing costs work proportional to retained representation size; snapshot retention consumes storage; stronger unknown-state handling can cause more abstentions or authorized alternate searches. Revalidation has network/provider latency and sometimes monetary cost. The current byte guards occur after rendered content acquisition, so snapshot work must not claim to have created a hard bound on Chrome's memory or all pre-extraction allocation.

**Likely avoidable costs:** an explicit continuation can reuse a retained snapshot rather than reopening and re-rendering a page for every offset. Correct remote identity can avoid pointless stale notices/evictions after unrelated file edits and false fresh assumptions when local bytes are unchanged. Typed unknown-search outcomes can avoid downstream reasoning founded on false absence. These are hypotheses, not measured savings.

For a document of retained size B read in k windows, a snapshot design targets one acquisition plus O(B) conversion/hash/storage. Paging proportional to total returned bytes is only a target with suitable indexed or chunked rune access; it is not a property established for the current archive. Offset-based string or SQLite substring access may repeatedly scan prefixes, so measure bytes/code points scanned and storage reads as well as returned bytes. Today's tool-level continuation performs k acquisitions/conversions. Actual cost depends on browser caching, dynamic content, storage representation and whether the existing unpaged-plus-recall route was already used. Compare against that stronger baseline too, not only the weakest case.

Measure full-task cost: provider input/output and cache-read tokens, search charges where applicable, browser navigations, bytes retained/read, conversion/hash CPU, peak browser/host memory, ledger compactions, source revalidations, repeat user clarifications, latency to supported answer, and abandoned/failed runs. Publish accepted-answer rate and unsupported-claim rate alongside cost; an apparent reduction bought by incomplete answers is not an improvement.

## Existing tests and the missing composition

The feature's browser runtime tests cover lifecycle cancellation, runtime binding, inherited configuration, observer incompleteness, transient retirement and cleanup failure. Those are substantial existing controls and should stay. They establish a different contract from semantic search validity and snapshot identity. [Runtime test definitions](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/browser_runtime_test.go).

The fetch paging test requests multiple windows from one unchanged local fixture and checks offset hints. The parser-empty test supplies HTML with no result cards and expects an empty slice; it does not establish a provider-confirmed empty-result state. The registered-search regression checks complete DuckDuckGo redirect destinations, which is useful but separate from a fetched document's final URL/base. [Paging test](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/research_coverage_test.go#L1361-L1392), [empty-parser test](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/research_coverage_test.go#L1667-L1677), [registered redirect test](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/web_search_redirect_test.go#L51-L108).

Working-context tests already cover retaining results, moving focus and restoring the original file observation on recall. Extend that existing fixture style to a real research result after a local read, instead of testing a second hand-built ledger with corrected metadata. The inspected tests are source evidence only; none were run for this report. [Working-context tests](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context_test.go#L260-L425).

## Proposed evaluation

No checks in this section were executed. Start with deterministic source/transport fixtures, then request-capture integration, then authorized end-to-end TUI tasks using the ordinary catalog, CodeDOM/world-model context and policy gates.

| Fixture | Required observation |
|---|---|
| Read local file A, acquire web result, modify A | Remote snapshot retains its own identity; no false remote staleness solely from A's digest. |
| Same local file, remote page changes | Historical snapshot remains intact; current applicability is unknown until required revalidation, not certified by A. |
| Research-only task whose initial focus is a directory | No workspace-directory digest is substituted for the remote source. |
| Two distinct queries with similar or identical result prose | Distinct acquisition/query provenance preserved; no false claim that all observations previously collapsed. |
| Valid populated DDG fixture | Existing title/URL/snippet extraction and redirect query/fragment preservation remain intact. |
| Explicit empty-results fixture | Scoped provider-empty outcome with acquisition identity. |
| HTTP-200 challenge, consent, login, changed markup | Unknown/unavailable semantic outcome, never successful empty search evidence. |
| Search body guard and explicit max-results | Positive hits preserved with accurate scope/partialness; no exhaustive claim. |
| Redirect into versioned docs with relative links and a base element | Requested URL, resolved document identity and effective link base retained; emitted links resolve correctly. |
| Embedded frame or second navigation during read | Status and content identity correlate or outcome is inconclusive; no unqualified metadata from another document. |
| Page content changes between offset calls | Continuation uses same snapshot or explicitly reports generation mismatch/new snapshot. |
| Snapshot expires or is pruned | Recall reports unavailable; no silent refresh pretending to be the old snapshot. |
| Oversize source and bounded storage | Guard/retention deficit remains visible, no unreachable “more” cursor. |
| Tool result crosses ledger compaction and recall | Body, source, semantic state and coverage survive into the actual next provider request. |
| Provider-native grounded search | Provider citations/results/usage retained as their own evidence type; no DDG-specific classification imposed. |
| TUI asks for code plus external API comparison | Local code and external documentation stay separate but related; final answer can name missing/currentness evidence without conflating them. |

The request-capture test should inspect what the answer model actually receives after acquisition, recording, compaction and recall. A tool unit test that sees the right string is insufficient; a correct stored envelope that the request renderer omits is also insufficient.

Matched experimental treatments should include current implementation; typed source/semantic outcomes without snapshot continuation; and full typed outcomes plus snapshot continuation. Keep question set, initial source snapshot, authorized tools, model/provider, budget and output acceptance criteria fixed. Add the existing unpaged fetch plus recall path as a baseline. Stratify by no-result/challenge, dynamic pages, redirects, local-file edits and stable static pages. Track false empty-result declarations, false freshness/staleness, snapshot continuity, usable citation targets, supported final claims and total cost.

Exit criteria should be explicit: no false negative-search promotion on the adversarial fixtures; no remote observation assigned a workspace revision; every continuation bound to one representation or explicitly changed; correct source/coverage survives the final consuming request; no weakening of existing runtime ownership, authorization, redaction or bounded-context behavior. Broader factual correctness remains a separate empirical goal.

## Spec first work order

1. Refine tools and context/session specs with the research-observation identity and semantic-outcome contract, citing the current seams rather than declaring them implemented.
2. Add composed acquisition-to-request fixtures that expose wrong-source adoption and false empty outcomes before implementation.
3. Introduce typed source adoption and truthful search classification while preserving the existing archive and runtime owner.
4. Bind fetch continuation to snapshots and preserve resolved URL/base identity.
5. Update JIT evidence guidance and task completion policy to consume these measured outcomes; do not teach unsupported retrieval or silently widen authority.
6. Measure the matched treatments before adding caching or revalidation heuristics.

The completion condition is a grounded investigation whose final consumer can distinguish what was requested, what was actually acquired, which exact representation was delivered, and what remains unknown. Browser cleanup success, a nil tool error, a recall handle and a local file hash are individually useful facts; none substitutes for that source contract.

