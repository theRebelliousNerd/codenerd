---
doc-class: deep-dive
subsystem: session-critic-world-lsp
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-02
verified-against: 7cbda4865d3c0ace4fe24576a65da75471d4f7c4
verification-method: static-source-inspection
supersedes: []
---

# Preserve document identity across critic LSP diagnostics

## Decision and evidence boundary

Give the critic's Python and TypeScript language-server client a shared, platform-aware document identity contract. Construct valid file URIs from admitted native paths, correlate returned URIs by a defined equivalence policy, and preserve an explicit distinction between received diagnostics and unestablished coverage.

At the pinned revision, `internal/world/lsp.pathToURI` concatenates `"file://"` with a filesystem path. Initialization, document opening and diagnostic waiting use that helper. The receiver stores each publication under the server's unchanged URI string, while the waiter looks up the locally generated string. This creates two independent problems:

1. Native paths containing URI delimiters or characters requiring encoding, and ordinary Windows drive paths, are not serialized faithfully.
2. Even when the outgoing URI is valid, an equivalent URI spelling returned by the server need not match the map key used by the waiter.

The second problem can consume the remaining per-language diagnostic budget and prevent subsequent files in that language from being opened. The current caller then identifies the current and remaining files as not diagnosed; it does **not** silently report all such failures as clean. Earlier collected findings survive. Startup or initialization failures without cancellation have different behavior: they are logged at debug level and produce no critic diagnostic text. [URI helper][uri-helper] [Client boundary][client-boundary] [Diagnostic caller][diagnostic-caller]

This is a source-supported interoperability and evidence-correlation finding, not a reproduced incident, security bypass, false compiler/test pass, or measured loss of model quality. The affected production route is the critic's **Python and TypeScript-family stdio sessions**. Its Go grounding invokes `gopls check` as a command with native file arguments and does not use this URI path. [Language selection][language-selection] [Go command path][go-command]

All codeNERD implementation links below pin `theRebelliousNerd/codenerd` at **`7cbda4865d3c0ace4fe24576a65da75471d4f7c4`**. Source, guidance and existing test definitions were inspected. No application, build, test, model inference, benchmark, server process or failure injection was run. All witnesses and evaluations below are conditional or proposed. Only this standalone Markdown draft was written; repository files were not changed.

## Architectural intent and existing work

Root guidance calls for model creativity grounded in deterministic, revision-aware context and validation through production entry paths. Session guidance retains the split between capabilities, exact Mangle permission and execution. World guidance asks for one workspace identity before producing facts or keys. The architecture rules distinguish intended behavior, implementation and verified reachability. The proposed URI boundary is host-side representation and correlation; it grants no new capability and delegates no executive decision to an analyzer or model. [Root guidance][root-guidance] [Session guidance][session-guidance] [World guidance][world-guidance] [Architecture guidance][architecture-guidance]

The pinned audit queue, current session cancellation contract, the earlier local critic-cancellation draft, and adjacent advisory-repair, review-feedback-identity and compile-diagnostic-witness reports were screened. Their boundaries differ:

| Existing work | What is retained or distinguished here |
|---|---|
| Critic diagnostic owner stop | Current code already reconnects explicit cancellation and owner stop, closes transport on request termination, joins cleanup and checks admission between stages. This audit does not revive the older detached-cancellation baseline. |
| Advisory repair continuation | Concerns consuming tool results after the critic's advice has reached the coding model. URI identity determines whether analyzer evidence reaches the critic beforehand. The queue records the advisory repair as fixed. |
| Review feedback identity and current verdicts | Concerns canonical review/finding IDs and later human judgments. No feedback command or historical vote is needed for this witness. |
| Compile diagnostic witness admission | Concerns diagnostic categories becoming invalid analyzer command selectors. The present commands can be valid; the defect is document identity inside their protocol exchange. |

[Current cancellation contract][owner-contract] [Audit queue][audit-queue] [Advisory report][advisory-report] [Review feedback report][feedback-report] [Compile witness report][compile-report]

This is a bounded nonduplication review, not a claim that every branch, issue or historical discussion was searched. Search results were used to locate paths; implementation claims were checked against the explicit pin rather than the search index's different revision. Old architecture prose about a generic external-LSP world manager is not needed: the current manager says that unused external-server half was removed, while the session critic directly constructs clients. [Manager scope][manager-scope]

## The production path

The shared factory constructs the session executor, resolves the session policy and supplies its workspace. `ExecutorConfigFrom` enables post-edit critic review and forwards the configured LSP budget. `ProcessWithIntent` attaches that budget to its task context. The post-edit scheduler obtains its rounds from Mangle; Go and other source writes owe `/critic`, whose driver calls `verifyAndUpliftWithCritic`. This is not a helper-only observation. [Factory][factory] [Configuration adapter][config-adapter] [Budget attachment][budget-attachment] [Round policy][round-policy] [Round driver][round-driver]

Admission still matters. The critic returns early when disabled, its result is nil, ownership is stopped, no reviewable written files can be loaded, or no critic client is available. After Go diagnostics, it invokes `lspDiagnostics` over the turn's written paths. Python selects `pyright-langserver --stdio`; `.ts`, `.tsx`, `.js` and `.jsx` select `typescript-language-server --stdio` with the corresponding language IDs. Only selected languages start. [Critic entry and grounding][critic-entry] [Language selection][language-selection]

For each selected language, `serverDiagnostics` creates a new client and request context, initializes the workspace, then processes files **sequentially**:

1. Resolve a relative written path against the workspace and read its bytes.
2. Send `didOpen` with that native path and the read text.
3. Wait for diagnostics for the same native path.
4. Format accepted error/warning diagnostics using the caller's workspace-relative file label and one-based line number.
5. Only then advance to the next file.

A wait error appends an incomplete-file notice for `files[i:]` and breaks this language's loop. Results from preceding files stay in `lines`. The outer dispatcher can proceed to the next selected language if ownership remains live. The budget is per language session, not per file and not a whole-turn bound; its default is 90 seconds. [Diagnostic caller][diagnostic-caller] [Budget definition][budget-definition]

Returned text is recorded in `ExecutionResult.StaticDiagnostics` and appended to the critic's grounding. The critic prompt is then built from changed-source views and this grounding before the model call. No claim that a model actually receives or acts on it follows without the remaining admission conditions, successful prompt assembly and a live call. The critic's opinion remains advisory and is not a compiler or test verdict. [Grounding to prompt][grounding-prompt]

## Where identity is lost

### Native paths are not URI strings

The package helper does no absolute-path conversion, component-aware escaping or platform conversion. Initialization calls `filepath.Abs` first, but making a native path absolute does not serialize it as a URI. The production caller ordinarily resolves document paths against its workspace, but the helper still receives native filesystem syntax. `DidOpen` and `WaitForDiagnostics` reuse the same helper, which gives local consistency only when the server preserves exactly that string. JSON escaping does not supply URI escaping. [URI helper][uri-helper] [Initialization and opening][client-open]

The following are static examples of the helper's output, **not executed server observations**:

| Native path | Concatenation result | Representation problem |
|---|---|---|
| `/ws/app.py` | `file:///ws/app.py` | Positive control: valid ordinary absolute POSIX spelling. |
| `/ws/a b.py` | `file:///ws/a b.py` | The filename's space remains unencoded. |
| `/ws/a#b.py` | `file:///ws/a#b.py` | A filename character is emitted as a fragment delimiter. |
| `/ws/a?b.py` | `file:///ws/a?b.py` | A filename character is emitted as a query delimiter. |
| `/ws/%61pp.py` | `file:///ws/%61pp.py` | The literal percent sequence is emitted as an escape; it no longer denotes the literal-percent filename. |
| `C:\work\app.py` | `file://C:\work\app.py` | Native backslashes and drive syntax are not translated into a file-URI path. |

The special-character examples concern filesystems that permit those names; they are not a demand that Windows accept POSIX filename syntax. UNC and extended Windows namespace paths also require a deliberate policy rather than adding a fixed number of slashes. Relative helper input is another API edge, but a relative path reaching the helper is not necessary for this production finding.

The protocol sends `DocumentUri` as a string containing a valid URI and explicitly warns against assuming the peer uses the same drive-letter case or colon encoding. RFC 3986 distinguishes component delimiters from data and treats escaped unreserved characters as equivalent to their literal form. RFC 8089 defines the file scheme's absolute-path and authority structure, with filesystem-dependent mapping considerations. These are the interoperability contracts behind the finding, not guarantees that every server repairs invalid input. [LSP URI definition][lsp-uri] [RFC 3986][rfc3986] [RFC 8089][rfc8089]

### Incoming and outgoing strings have different owners

`handleDiagnostics` stores the converted diagnostic array as `diags[payload.URI]`. It neither parses the URI nor resolves it to an opened-document identity. `WaitForDiagnostics` computes `pathToURI(path)` and checks that exact key every 20 milliseconds. The server can have published useful diagnostics while the waiter remains unable to find them. A successful equality check establishes only equality with that key; it does not independently establish a document revision or final analysis status. [Diagnostic receive and wait][client-diagnostics]

A sufficient conditional witness uses no special filesystem name:

1. A normally admitted writing turn reaches the critic with readable `/ws/app.py` and another Python file in its write set, and the configured server initializes successfully.
2. For `app.py`, the client sends `file:///ws/app.py` and waits under that same string.
3. The server publishes the diagnostic set for this document using the equivalent valid spelling `file:///ws/%61pp.py`, with no publication under the exact outgoing spelling before termination.
4. The receiver inserts that other key. The waiter never finds its requested key.
5. If the owner remains live and the transport otherwise permits normal progress, the request budget expires. The current and later Python files are named as not diagnosed; the later file was never opened by this loop. Earlier files, if any, retain their findings.

This is a protocol-permitted alias schedule derived from the code and specification, not a claim that an installed Pyright or TypeScript version emits this particular escape. An ordinary exact-echo server is an essential passing control. A server may instead reject malformed input during initialization, preserve it literally, normalize it, or publish nothing; the source does not select one universal outcome. [URI equivalence][rfc3986] [Exact-key wait][client-diagnostics] [Caller disposition][diagnostic-caller]

Fixing only outbound serialization is therefore incomplete. It makes the emitted URI well formed but does not require another implementation to return the same string. Conversely, accepting more incoming aliases cannot repair an outbound URI that already names the wrong resource. Both sides must resolve to one deliberately chosen document identity.

## Timeout and failure behavior that must remain visible

The observed branches deserve separate treatment:

| Condition | Current source behavior |
|---|---|
| No matching written files or empty workspace | No language diagnostic request. |
| Server cannot start while request context is live | Debug log and no diagnostic text; missing optional binaries are intentionally nonfatal. The branch also covers other startup failures. |
| Initialization fails while request context is live | Debug log and no diagnostic text. An invalid root URI can take this route if the server rejects it. |
| File cannot be read | Notice for that file; continue to later files. |
| `didOpen` write fails | Notice with the returned error for this and remaining files; stop that language loop. |
| No publication matches the wait key | Wait until context termination or explicit client close; notice for current and remaining files. |
| Matching empty diagnostic set | Map entry exists, so wait succeeds with no finding lines. |
| Ownership stops | Existing checks suppress later admission and the owned request/transport is canceled and cleaned up. |

[Caller branches][diagnostic-caller] [Wait and empty-set handling][client-diagnostics] [Owner machinery][owner-machinery]

There is a related terminal-error limitation. On a framing/read error, `readLoop` releases pending RPC calls and returns, but does not close `doneCh` or install a diagnostic-wait terminal error. If the server disconnects after initialization while the caller is waiting for an absent map key, that wait can remain until the request timer/owner stop causes closure. Also, the caller discards the wait error's detail and selects `canceled` only when the context reports `context.Canceled`; otherwise it emits the generic time-budget wording. These are source-level residual error-attribution issues, not a claim of a permanent hang. [Read-loop termination][client-readloop] [Wait error conversion][diagnostic-caller]

Do not remove the cancellation repair while changing identity. `diagnosticRequestContext` retains the independent positive budget and metadata but watches ordinary explicit cancellation and the independent owner channel, including after a soft caller deadline. The disposer cancels and joins its watcher. `closeDiagnosticClient` attaches `Client.Close` to request termination, performs bounded polite shutdown while the context is live and joins a started close callback. The critic, dispatcher and per-file loop check stop before further work. Existing fixtures cover these contracts, including blocked initialization writes. They were read, not rerun. [Owner machinery][owner-machinery] [Current owner contract][owner-contract] [Owner tests][owner-tests] [Owner controls][owner-controls]

The 90-second request budget is not a proven wall-clock bound on all file reads, process cleanup or supported descendants. The production transport closes pipes, kills the direct process and waits; this audit does not infer universal process-tree containment from that code. [Process transport][process-transport]

## Freshness is a separate supporting limitation

The current client starts with a new diagnostic map for each language review, so there is no established cross-turn diagnostic-cache reuse here. `DidOpen` assigns version 1, which is reasonable for a freshly opened document in a new session. It does not clear a preexisting entry or register an open generation; the publication decoder ignores the optional `version`, and initialization does not advertise version support. These are concrete limits, not a claim that version 1 itself violates LSP. [Client construction][client-construction] [Opening][client-open] [Publication decoder][client-diagnostics]

Within one session, a project-aware server can publish for a later file before this loop opens it, or send an early empty batch followed by a later nonempty batch. If the first matching entry is visible when `WaitForDiagnostics` checks, it returns that entry. A subsequent publication replaces the map value, but the caller does not revisit its previously formatted output. This conditional timing schedule can therefore yield a stale or incomplete observation even when URI correlation works. It is not needed for the primary identity witness.

LSP push diagnostics replace the previous set, and an empty set clears previous diagnostics. The publication version is optional. The standard notification does not contain a universal final-analysis marker, so neither “first publication” nor an arbitrary quiet interval proves complete analysis of a project. Ignoring optional versions is not by itself a protocol violation; promising stronger freshness or completion than the client can establish would be an evidence-contract error. [Publish diagnostics semantics][lsp-publish]

The critic also reads review source before diagnostics, while `serverDiagnostics` later reads each opened file independently. The inspected path carries no shared source digest from these reads into `Diagnostic`. A concurrent edit can separate the review bytes and diagnostic bytes. This is a conditional snapshot limitation supported by these reads, not proof of a concurrent incident or a reason to make optional diagnostics a mandatory gate. [Critic source read][critic-entry] [Per-file source read][diagnostic-caller] [Diagnostic shape][client-construction]

## Proposed document identity and evidence contract

### One admitted source identity with separate representations

Before protocol work, construct a typed document record from the existing workspace and authorized written-file population. Keep distinct fields for native read path, stable internal identity, display label, outbound URI, source digest/open generation and session identity. The names are design notation, not shipped APIs. A received URI is evidence about an already registered document; it is never permission to open another path.

Choose the existing workspace identity as the base for relative inputs. Do not resolve document input against arbitrary process CWD. Produce a valid URI by composing its components, with one deliberate native-path conversion. Treat source characters as data, not already-encoded URI text. If a supported path cannot be represented faithfully, emit an explicit unsupported/unavailable result rather than silently changing it.

For incoming diagnostics, parse the URI, validate its allowed scheme and authority, and map only supported equivalents to the registered identity. Keep the original wire value for troubleshooting. Do not resolve arbitrary returned URIs through filesystem reads or network lookups. Unknown, malformed or ambiguous identities must not satisfy a document wait or become a clean result. Optional workspace-wide observations may be retained separately, but cannot discharge an opened-document obligation merely because their basenames match.

### Platform and encoding policy must be explicit

The implementation proposal needs positive and negative controls, not a broad string-cleaning function:

- Preserve literal percent signs on output; decode input components at most once. Do not double-decode `%252F` into a separator or collapse an encoded separator into an authorized neighboring path.
- Preserve POSIX path case and literal backslashes. Translate native Windows separators only under Windows-path semantics, and normalize a Windows drive identity without lowercasing every path component.
- Define whether supported local-authority forms are equivalent. Never strip an arbitrary host and treat the remainder as local. Supporting an approved UNC workspace requires preserving its server/share identity; it does not authorize unrelated remote shares.
- Handle Unicode names without blindly applying global normalization or case folding. For undecodable native bytes or unsupported extended/device namespaces, return an explicit limitation instead of lossy substitution.
- Decide lexical alias, symlink and hard-link treatment with the workspace contract. Two names referring to one inode need not be interchangeable source labels, and symlink resolution can change containment and race behavior. Do not add unrestricted `EvalSymlinks` as an incidental URI fix.
- Keep exact permissions, reserved-directory rules, write protections and allowed file population unchanged. Correlation must not broaden authority or turn a server's claimed URI into a capability.

These are conservative design requirements for this adapter, not claims of an existing exploit. RFC 3986 warns against repeated decoding and distinguishes safe unreserved normalization from reserved-character changes. RFC 8089's security and filesystem sections discuss nonlocal authorities, case/encoding aliasing and system-specific names; its appendix examples are not a universal mandate to accept every nonstandard form. [RFC 3986 encoding and comparison][rfc3986] [RFC 8089 filesystem and security considerations][rfc8089]

### Attach diagnostics to an observation with known limits

Register a new open generation before sending `didOpen`, and do not let pre-open map entries automatically satisfy it. Preserve publication replacement, including a legitimate empty set. If version interpretation is implemented, advertise it and compare versions against the registered document; retain an explicit unversioned state when the peer omits them. Do not fabricate a version or pretend a local generation proves which source bytes a remote unversioned publication analyzed.

Where stronger final evidence is needed, choose a documented server capability/profile or an appropriate diagnostic request mode and qualify what it guarantees. A fixed sleep, longer timeout or debounce is not a substitute for that contract. A narrow first repair may fix URI identity while honestly labeling push results as observations rather than complete analysis. It should not wait the entire budget after every valid result solely to manufacture confidence.

For this critic path, prefer reusing one captured source snapshot for review rendering and document opening, or check its existing revision identity before combining the two. A changed snapshot should make the association visibly outdated/unverified, not automatically cause another model call or replay any edit. Decide how optional refreshed analysis is admitted under the same owner and budget.

Expose request state independently of diagnostic count: matched publication, unversioned observation, timeout, canceled, malformed/unmatched identity, transport failure, read failure and optional server unavailability. These are proposed semantic distinctions, not existing schema literals. Close a diagnostic wait on terminal transport failure with its actual reason. Keep partial findings and all-unchecked-file accounting. A successful empty publication is meaningful, but silence, failure and unsupported identity are not equivalent evidence.

Go owns codec, protocol transport and typed observations. Mangle retains authority over task obligations and completion. No model or embedding request is intrinsically required to translate a path, compare identities or classify a terminal transport event.

## Costs and alternatives

Let `D` be the configured language-session budget and `r` the remaining budget when the first unmatched document wait begins. Under responsive transport/cleanup and no earlier owner stop, a key mismatch can spend approximately `r` waiting. It also prevents later files of that language from opening. It does **not** create a fresh `D` per skipped file. Python and TypeScript sessions have separate budgets, so each selected affected language can incur its own remaining wait. This is budget exposure, not a measured delay or hard bound.

The proposed codec and identity lookup add work proportional to path/URI length and bookkeeping for registered documents. Generation/version tracking adds state and synchronization. Source digests may require additional reads unless existing snapshot evidence can be reused; filesystem canonicalization is more expensive and has different risks than lexical URI normalization. Do not hide these costs behind a claim that map lookup is constant time.

Correct correlation may reduce idle waiting and recover findings, but those findings can increase critic input tokens and trigger useful or mistaken advisory work. More complete analysis can consume more host CPU and take longer than returning an early empty batch. No token, cost, quality or success-rate gain was measured here.

| Alternative | Benefit and remaining limit |
|---|---|
| Escape only spaces | Small change; misses delimiters, percent sequences, Windows syntax and incoming aliases. |
| Correct outbound URIs only | Necessary serialization repair; still assumes the server returns the same spelling. |
| Lowercase or decode all strings | Can create false matches between distinct files or change reserved syntax; unacceptable without a platform-specific identity policy. |
| Require exact echo from every server | Simple but weaker interoperability than LSP's stated expectations; would need explicit supported-server restrictions and visible rejection. |
| Increase the request timeout | More time for real analysis, but an unequal map key does not become equal through waiting. |
| Replace push collection with pull diagnostics | Potentially clearer request provenance where supported; changes capability negotiation, server compatibility, result handling and costs. Not a universal drop-in repair. |
| Disable diagnostics | Removes wait cost and useful grounding together; not equivalent to conserving identity. |

Evaluate total completed-task cost, including analyzer startup/indexing, CPU, memory, I/O, transport/polling, cleanup, prompt bytes/tokens, critic and advisory calls, rechecks, retries and harmful or useful resulting edits. Report cold and warm cache conditions separately. A faster result that omitted a finding or mislabeled failure as clean is not an efficiency win.

## Existing tests and prospective evaluation

The inspected test definitions provide useful positive controls but do not establish this contract:

- `TestPathToURI` includes a `contains` field but asserts only nonempty output. It does not check valid URI structure, round-trip identity or platform behavior. [Helper test][uri-tests]
- The session fake server copies `didOpen`'s URI unchanged into its publication. Tests cover Python grounding, optional absence, every written Python file, severity filtering and incomplete-file notices; exact echo cannot detect an equivalent-spelling mismatch. [Session protocol fixture][server-tests]
- The client test exercises missing binaries and in-flight initialization disconnection. That RPC waiter is different from the diagnostic-map waiter after initialization. [Client tests][client-tests]
- Current owner-stop fixtures cover cancellation, subsequent-stage suppression and blocked protocol writes. They are regression obligations for any identity change, not missing mechanisms to reinvent. [Owner tests][owner-tests] [Owner controls][owner-controls]

All cases below are proposed and **unrun**. First use deterministic protocol fixtures through the real client and session caller; use handshakes rather than timing guesses. Expected URI examples should be independently specified, not generated by the same helper under test.

| Evaluation case | Acceptance observation |
|---|---|
| Plain absolute POSIX path with exact echo | Same findings and file coverage, without unnecessary extra wait or request. |
| Spaces, literal `%`, `#`, `?`, non-ASCII names on supporting filesystems | Valid outgoing URI identifies the exact bytes read; no unintended query/fragment or double encoding. |
| Equivalent valid incoming spelling | Correlates to the registered document; later files open; no false incomplete notice. |
| Distinct case-sensitive files, encoded separators and double escapes | Remain distinct or explicitly rejected; never match by over-normalization. |
| Windows drive paths and approved UNC workspace | Native-platform cases preserve drive/share identity; unrelated authorities and device forms do not acquire access. |
| Relative paths with process CWD outside the workspace | Resolve against the declared workspace and retain existing membership/authority behavior. |
| Unknown scheme, malformed URI, unrelated absolute path or basename collision | Does not satisfy another document's wait, trigger new reads or become evidence of clean source. |
| Several files with an unmatched first or middle publication | Earlier findings retained; affected coverage named accurately; repaired path reaches later files without a restored file cap. |
| Empty publication and replacement publication | Empty replaces previous findings; any completeness claim follows the chosen observation contract, not count alone. |
| Publication before open, older/current/omitted versions, late update | Generation/version policy enforced; unversioned uncertainty remains explicit; no fabricated finality. |
| Source mutation between review capture and document opening | No stale diagnostics represented as current evidence for different source bytes. |
| EOF or framing failure after initialization | Diagnostic wait terminates with transport reason, not only timer expiry or a fabricated clean result. |
| Initialization rejection, missing binary, read failure and write failure | Existing optional behavior and partial-file accounting remain explicit; distinguish failure classes. |
| Pre-cancel, cancel during wait/write, owner stop after soft deadline | Existing budget/owner semantics and cleanup join retained; no later language or critic call admitted after stop. |
| Fresh consecutive reviews | No cross-session diagnostic association, leaked handles or stale map reuse. |

Next exercise the normal configured session/post-edit scheduler with an authorized Python or TypeScript edit and a scripted critic. Verify the exact diagnostic text and coverage state in the critic request, not merely a helper return. Retain the normal world/CodeDOM and permission setup; the deterministic provider double avoids live-model expense while proving wiring. Then, in a separately authorized integration pass, record exact installed Pyright/TypeScript server versions, OS/filesystem, project configuration and wire traces. Run genuine clean/error cases and the filename matrix. A fixture proves the adapter contract; it does not prove every real server's analysis scheduling.

Compare baseline and candidate with the same files, analyzer versions, diagnostic delays, source revisions, request budget, tool/permission envelope and cache conditions. Do not make the candidate look faster by shortening its timeout, hiding later files or skipping post-edit obligations. Measure association success, false matches, uncovered files, observed/stale/unversioned batches, error classification, time to each publication, time to return, server/process lifetime, CPU, memory, bytes and cleanup. For end-to-end trials separately measure critic/advisory tokens, calls, valid findings, false advice, harmful edits, task outcome and total cost.

**Checkable exit:** every supported written file is represented by a faithful outbound URI; supported equivalent returned URIs resolve only to its admitted document identity; failure and incomplete coverage stay distinguishable from empty findings; optional evidence does not claim unavailable freshness or finality; owner stop, existing budgets, all-written-file coverage and exact authority remain intact. Update world/session specifications and implementation-status claims only with appropriately scoped evidence from checks that actually ran.

[root-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/agents.md
[world-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/agents.md
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/Docs/architecture/AGENTS.md
[uri-helper]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/manager.go#L98-L103
[manager-scope]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/manager.go#L12-L17
[client-boundary]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/client.go#L320-L419
[client-open]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/client.go#L320-L353
[client-diagnostics]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/client.go#L363-L419
[client-readloop]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/client.go#L193-L240
[client-construction]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/client.go#L38-L94
[process-transport]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/client.go#L100-L160
[diagnostic-caller]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/lsp_diagnostics.go#L275-L349
[language-selection]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/lsp_diagnostics.go#L226-L273
[go-command]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/lsp_diagnostics.go#L88-L153
[critic-entry]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/build_verify.go#L917-L990
[grounding-prompt]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/build_verify.go#L981-L1036
[owner-machinery]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/build_verify.go#L1241-L1313
[owner-contract]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/Docs/architecture/session/03-GAP-ANALYSIS.md#L164-L193
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/system/factory.go#L2564-L2592
[config-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/session_config.go#L23-L49
[budget-attachment]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/executor.go#L1305-L1322
[budget-definition]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/config/session.go#L77-L101
[round-policy]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/core/defaults/policy/turn_rounds.mg#L40-L61
[round-driver]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/executor_tools.go#L458-L497
[audit-queue]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/Docs/audits/ASTRA_QUEUE.md
[advisory-report]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/Docs/audits/advisory-repair-continuation.md
[feedback-report]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/Docs/audits/review-feedback-identity-and-current-verdicts.md
[compile-report]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/Docs/audits/compile-diagnostic-witness-admission.md
[uri-tests]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/manager_test.go#L56-L80
[server-tests]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/lsp_servers_test.go#L22-L220
[client-tests]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/world/lsp/client_test.go#L122-L161
[owner-tests]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/diagnostic_owner_test.go#L33-L185
[owner-controls]: https://github.com/theRebelliousNerd/codenerd/blob/7cbda4865d3c0ace4fe24576a65da75471d4f7c4/internal/session/diagnostic_owner_controls_test.go#L17-L100
[lsp-uri]: https://raw.githubusercontent.com/microsoft/language-server-protocol/gh-pages/_includes/types/uri.md
[lsp-publish]: https://raw.githubusercontent.com/microsoft/language-server-protocol/gh-pages/_specifications/lsp/3.17/language/publishDiagnostics.md
[rfc3986]: https://www.rfc-editor.org/rfc/rfc3986.html
[rfc8089]: https://www.rfc-editor.org/rfc/rfc8089.html
