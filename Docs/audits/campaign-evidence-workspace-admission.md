# Campaign evidence reads need canonical workspace admission

## Status and scope

**Source-confirmed integration gap; proposed remedy, not an implemented or executed result.** Inspected CodeNERD main `6cbe3962fbdb24a1639b26e71f8096e4f4330048` on 2026-10-30, together with current guidance, the research inventory through `research-main-20261030-1722`, and active branch heads.

A campaign can automatically read a document named in its task brief and paste its bytes into the next task's input. That path checks the document's *lexical* workspace spelling and secret-name pattern, but does not apply the symlink-resolving workspace guard used by ordinary file tools. Consequently, an admitted task naming an in-workspace document alias can receive bytes from a readable file outside that workspace. No concurrent link replacement, generated tool, shell execution, completed predecessor, or model-issued `read_file` call is needed for the bounded case below. [Candidate discovery][brief-paths], [measurement][brief-measure], [selection][brief-policy], [rendering][render]

The established result is a source-derived read and task-input construction mismatch. This review did not demonstrate a provider request, disclosure of private information, an incorrect answer, mutation outside the workspace, or campaign completion. Any later provider delivery remains conditional on the ordinary session, client, context-budget and cancellation gates admitting that call.

## Existing contract and novelty

This is a targeted continuation of an existing containment obligation, not a claim that the repository never considered path safety:

- Earlier campaign QA explicitly asks that campaign file validation prevent reads and writes outside the campaign workspace, including symlinks. Earlier decomposer QA also flags unsafe artifact paths. Those are predecessor requirements, not proof that the specific current consumer is fixed. [Campaign QA §12.1][prior-qa], [decomposer QA][prior-decomposer]
- The current decomposer drops lexically unsafe declared artifact paths. Its sanitizer computes absolute/relative path spellings and rejects upward escape; it does not resolve links. This is a real protection against ordinary absolute/traversal inputs. [Artifact admission][artifact-admission], [sanitizer][artifact-sanitize]
- The CLI architecture records the analogous chat information-gathering repair: that read must use the same canonical containment as typed `read_file`. The campaign's automatic brief-document reader is a distinct consumer, not that repaired chat path. [Chat containment contract][chat-contract]
- Ordinary `read_file` resolves the requested path through the shared tools guard. That guard resolves symlinks and then checks the resolved path against the canonical root. [File-tool entry][read-file], [shared alias][guard-alias], [resolved containment][guard]

The named-document rule is intentional and useful: it avoids spending a tool round rediscovering a small document explicitly named by the task. The proposal preserves that behavior; it changes which bytes qualify as workspace evidence.

The latest research branch's three additions concern campaign learning receipts, generated-tool quality profiles, and recurse Go test dependency closure. Targeted source/document searches found no prior audit of this exact `task_brief_file → brief_document → task_evidence → renderEvidence` containment path. At inspection, `task_evidence.go` was byte-identical at all five active non-research heads inspected: orchestrator QA `51cfd426`, store lane `a53cfc30`, session lane `80af1863`, OAuth lane `aa7988de`, and cartographer QA `767a3291`. This is a bounded file comparison, not a claim that those branches are otherwise equivalent.

## Production route and exact trigger

### Harmless conditional fixture

Use only synthetic, non-secret fixture text:

1. The configured campaign workspace contains `docs/reference.md`, a stable filesystem symlink whose target is an existing readable regular Markdown file **outside** that workspace. The fixture's target contains a distinctive public marker, not credentials or private project material. Neither alias nor target matches an installed secret-path pattern.
2. Normal campaign decomposition/admission accepts a pending `/research` task with a brief such as “Summarize docs/reference.md.” Its `Shard` and `WriteSet` are empty. The alias is not declared as any task's artifact and is not the asked task's output; any report artifact is a different path.
3. The task belongs to the live campaign, is eligible, has an available concurrency slot, is not held or canceled, and the normal kernel/fact operations succeed. The installed evidence policy is the stock policy cited below.
4. The external target is nonempty and small enough for `campaign.upstream_inline_max_bytes`. It remains readable and unchanged through measurement/rendering. The configured task executor and its ordinary subsequent admission are available.

These are **preconditions**, not a prediction that a model will produce this plan for every wording. A plan rejected before scheduling is a negative control. A plan that declares the alias as its own output exercises a different rule and is not this witness.

### Source trace

| Boundary | Current source behavior |
|---|---|
| Real TUI campaign entry | `startCampaign` creates the orchestrator, decomposes the goal and installs the resulting campaign. The owned background runner calls `orch.Run(ctx)`. [TUI entry][tui-entry], [runner][tui-run] |
| Planner representation | `buildCampaign` preserves the task description and explicit shard fields. It only infers a missing write set from artifacts/description for mutating task kinds. [Task construction][task-build], [write-set inference][write-inference] |
| Scheduler admission | The phase scheduler obtains eligible tasks, observes concurrency, acquires any required write-set lease and launches `runSingleTask`; that calls `executeTaskWithRollback`. These existing admissions remain required. [Schedule][schedule], [worker][worker] |
| No hidden write-path rescue | `/research` is not a mutating task type. With an empty write set, `resolveTaskWriteSet` returns nil before consulting description/artifacts; the lease helper permits an empty nonmutating set. The task-effect validator does not require a file target for research. This is appropriate write coordination, but not read admission. [Kinds][kinds], [write-set resolution][write-set], [lease][lease], [effect contract][effects] |
| Actual task handler | With empty `Shard`, type routing selects `executeResearchTask`. It builds the input before calling `spawnTask`. [Routing][routing], [research handler][research] |
| Candidate extraction | `briefPaths` extracts `docs/reference.md`, applies `types.CanonicalPath`, rejects lexical outside/absolute paths, and accepts an existing result of `os.Stat`. `CanonicalPath` normalizes identity strings, not filesystem authorization. [Discovery][brief-paths], [identity helper][canonical] |
| Evidence measurement | `evidenceFacts` stats the named path, requires a regular file, and applies `tools.IsSecretPath` to the alias. It emits `task_brief_file(Task, Path, Ext, Bytes)`. `IsSecretPath` is a name-pattern matcher, not a link resolver. [Measurement][brief-measure], [secret matcher][secrets] |
| Executive selection | Stock `brief_document` admits nonempty document-class named files that no task declares; `task_evidence(..., /inline)` requires that the path is not the task's output and is within the inline size limit. [Rules][brief-policy] |
| Bytes enter the task input | For a named document not in the artifact map, `renderEvidence` joins the workspace and alias, then calls raw `os.ReadFile`. The inline branch emits those bytes. `taskContextSection` returns the rendered evidence and `buildTaskInput` appends it to the task input. [Reader][render], [section][section], [input][input] |
| Downstream handoff | `spawnTask` puts that input in `session.TaskRequest.Task` and invokes the observed executor through `RunUntilVerified`. The research handler does not replace it with a guarded tool read first. [Request][request], [observed dispatch][dispatch] |

The critical difference is not whether the *spelling* begins inside the workspace. Both the alias and its parent can look local while filesystem resolution reaches elsewhere. The current raw read has no containment decision between selection and opening. The Go documentation distinguishes ordinary path-based opens from rooted access and warns that merely prefixing a root does not confine symlink traversal. [Go filesystem semantics][go-os]

If later session execution proceeds, the isolated task path preserves `req.TaskText()`, and the subagent passes its task to `ProcessWithIntent`. The session ultimately uses that input in its generation path. This establishes a genuine LLM-facing handoff, not an unused diagnostic field; it does not establish that a particular live request was sent. [Isolated input][isolated], [subagent entry][subagent], [generation][generation]

## Safeguards and limits that must remain explicit

1. **The ordinary file tools are not shown to have this same stable-link escape.** The cited shared guard would reject the fixture's external target. This document does not claim that the campaign disables or weakens that guard; it reaches a separate raw reader.
2. **Lexical traversal is already rejected in the named-document path.** An outside absolute filename or `../` path is not the stable-link fixture. Artifact sanitization is also a meaningful control, even though direct brief-document discovery does not pass through it.
3. **Name-based secret filtering works for directly matching names.** The existing named-document test installs a `private.md` pattern and checks that its marker is absent, along with own-output and unfinished-artifact controls. No tests were run in this review. [Existing controls][tests]
4. **Evidence selection can exclude the candidate.** Empty files, non-document extensions, nonexistent/unreadable files, an own-output match, a declared-but-unfinished artifact, policy errors, or different installed policy can prevent the stated inline route. Over-limit documents use a digest/handle path instead; the primary witness stays below the limit.
5. **Kernel ownership and error handling still matter.** The task must be present in the live campaign; measurement/query failures stop input construction. `evidenceMu` serializes measurement and selection within an orchestrator. That mutex does not authorize a filesystem target or freeze external filesystem changes. [Measurement lifecycle][measure-lifecycle], [section][section]
6. **This is not a mutation-permission or final-verdict finding.** Write guards, Dreamer/permission checks on later tools, deliverable checks, verification and campaign acceptance remain intact. They cannot retroactively make an already-read external byte sequence into workspace evidence, but they may still block or fail the task.
7. **Structural indexing has a concrete competing safeguard.** Its source-file symlink branch resolves through `tools.ResolveWorkspacePath`, rejects a failed/outside resolution, checks the resolved target's secret classification and membership, and uses the resolved target for indexing. The witness uses brief-named Markdown specifically; this is not a structural CodeDOM escape. Broader automatic readers must be traced separately, not assumed defective by analogy. [Index control][index-control]
8. **The stable case does not depend on TOCTOU.** A separate future race test is necessary for any proposed check-then-open repair. Platform symlink support, filesystem permissions, installed policy and actual serving behavior remain environment-dependent.

## Proposed contract

> Every byte-bearing automatic campaign evidence read must be admitted against the campaign's canonical workspace and the current read/secret policy before it is represented as model-visible workspace evidence. The admitted file identity and the bytes rendered must belong to the same read operation or explicitly validated snapshot.

Apply this as an extension of the existing file-read contract, not an ad-hoc prompt instruction asking the model to ignore unwanted bytes.

### Minimal parity repair

- Route candidate paths through a shared read-admission helper that takes **the orchestrator's explicit workspace**, not a process-wide environment fallback.
- Resolve aliases, check root containment, and apply the existing secret classification to the relevant requested and resolved identities. Keep lexical identity for facts/display separate from authorization identity.
- Use the admitted path for measurement and reading; do not validate one path and then reopen the original alias.
- Cover both the named-document and declared-artifact branches, plus citation extraction from those artifacts. Otherwise a partial fix only changes which producer reaches the same raw read.
- Preserve stock Mangle relevance/form decisions. Host code supplies trustworthy path/read measurements; the policy still decides whether admitted evidence is inline, a digest, or a handle.
- Return a typed unavailable/denied evidence outcome without copying forbidden content into an error, log, handle or prompt. If an explicitly required input is denied, report the unresolved input rather than claiming it was absent or silently substituting another file.

This fixes the static alias witness but must not be marketed as a complete defense against hostile concurrent path replacement.

### Stronger read-identity option

Prefer a rooted read capability or equivalent handle-based opening where the platform supports it. Open against an owned canonical workspace root; derive size/type from the opened file and read the same file handle. Keep the capability's lifetime explicit and release it during orchestrator shutdown.

Go's `os.Root` is a relevant implementation option, not an automatic drop-in policy: it constrains symlink escape while allowing in-root relative links, but its documented contract rejects absolute symlink targets and has platform limitations. It does not itself exclude bind mounts, special files or application-secret names. Decide compatibility for legitimate in-root absolute aliases explicitly instead of silently broadening scope or breaking them. [Go Root contract][go-root]

An `EvalSymlinks` check followed by an independent open remains susceptible to replacement between those operations; Go's primary explanation discusses that distinction and rooted opening. The evidence contract should say which guarantee is provided. [Go traversal-resistant APIs][go-root-blog]

If external documents are a desired feature, give them an explicit, auditable read grant/source-ingestion contract and distinct provenance. A local-looking symlink should not silently become that grant. Do not infer read permission from task relevance, a planned output artifact, or a positive model answer.

### Lifecycle and data compatibility

No campaign snapshot migration is inherently required for minimal read containment. A cached path/classification must not outlive the root, policy or file identity it admitted. On denial or source replacement, retire or replace the relevant measured evidence instead of leaving an earlier admission current. Existing completed-task outputs and recall handles should remain usable when they were legitimately admitted; the remedy must not delete user files or rewrite old reports merely because a new read is refused.

## Cost and tradeoffs

| Cost dimension | Expected change and measurement |
|---|---|
| Model calls and tokens | No new classifier/model call is needed. Successful in-root briefs should retain the same useful evidence and call count. A denied required input may require a user clarification; count that explicitly rather than hiding it as a retry. |
| Filesystem latency | Canonical resolution adds path-component work; rooted reads may use additional system calls. Measure cold/warm p50/p95 input-construction latency by candidate count, path depth and link depth. Go documents that deep rooted paths can cost more. [Primary discussion][go-root-blog] |
| Memory | Avoid reading a file twice merely to revalidate it. Retaining immutable selected bytes costs their total size; retaining bounded file handles trades memory for descriptor pressure. Preserve existing digest/handle context limits independently of read authorization. |
| Handles and cancellation | A reusable root uses an OS resource on common platforms. Define ownership, close-on-error/shutdown behavior and maximum simultaneously open evidence files. Check cancellation between files without claiming it cancels every filesystem syscall. |
| Engineering and maintenance | Reusing the existing guard is smaller than designing a new authorization system. A stronger same-handle contract needs platform fixtures and policy/identity integration. Avoid a second divergent path matcher. |
| Compatibility and quality | Legitimate in-root symlinks, symlinked workspace roots and explicit outside-document workflows are positive controls. A blanket ban on all symlinks is simpler but loses supported navigation and should not be mistaken for equivalence. |
| Diagnostics and persistence | Record admitted/denied/missing/change-detected counts and safe source identity, not content. Additional persistent audit receipts are optional and should have a stated owner/retention cost; this fix need not add another per-read database write. |

No latency, cost, quality or attack-frequency measurements were made. The expected benefit is consistent read scope and truthful evidence provenance, not a claimed benchmark gain.

## Fair evaluation plan — not executed

Start with deterministic synthetic fixtures, the real evidence rules and normal task construction. Capture the input given to a fake observed executor; no live model is needed to establish the boundary.

| Case | Required result |
|---|---|
| Stable `docs/reference.md` link to a small external public marker; accepted eligible research task; empty write set; alias not any artifact/output | Current source predicts marker in constructed input. Corrected route must not read/render/retain its bytes; it must report a bounded denied/unavailable input. |
| Same relative link resolving to a regular allowed file inside the workspace | Retain evidence and intended inline form; do not “fix” the negative case by losing all links. |
| Direct allowed in-root document, different process CWD, symlinked workspace root | Same bytes and canonical provenance as ordinary file read; explicit workspace authority wins. |
| Direct outside absolute path and `../` path | Preserve existing refusal. Keep these distinct from the new stable-link case. |
| Direct secret-pattern match; harmless in-root alias to a synthetic pattern-protected fixture | No marker enters facts carrying content, prompt, error or recall store. Keep the policy's explicit empty-pattern configuration as a separate control. |
| Named own output, unfinished declared artifact, empty/missing/non-document file | Preserve current evidence-selection semantics; do not weaken those exclusions to make the positive case pass. |
| Completed upstream declared artifact through an external alias | Exercise the second raw-reader producer under its own completion/dependency preconditions. Passing only the direct-brief fixture is insufficient coverage. |
| Below/at/above inline limit | Containment must precede all inline/digest/handle forms; large size must not turn denial into a retained external transcript. |
| Alias changed between candidate discovery, admission and opening | Use a deterministic seam. Assert either bytes from the specifically admitted open file or a change/denial result, never bytes from the replacement external target. Report the actual platform guarantee. |
| Permission failure, broken/looping link, kernel failure, cancellation, unavailable executor | No invented evidence or completion; no leaked handles; preserve the ordinary failure reason. |
| Normal TUI campaign start through scheduling and fake serving client | Verify the allowed path reaches the production consumer, and a denied marker does not reach any captured model request. Test input construction and final transport separately so a later unrelated failure cannot hide an earlier unauthorized read. |

Compare current behavior, minimal shared-guard parity, and the stronger rooted/same-handle design on identical plans, policies, files and budgets. Report false denials and retained authorized evidence alongside blocked external bytes. Use Windows and Unix results separately; unsupported link creation is an explicit coverage limitation, not a passing containment result. Only after separate authorization would a bounded live campaign study be appropriate.

The acceptance exit is narrow: automatic campaign document evidence and equivalent explicit file reads agree on workspace scope, while authorized evidence remains available and downstream task/verification behavior is unchanged. No tests, application execution, inference, implementation or repository writes were performed for this review.

## Source references

All CodeNERD links below are pinned to the inspected main revision.

[brief-paths]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/task_preload.go#L50-L79
[brief-measure]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/task_evidence.go#L190-L242
[brief-policy]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/campaign_evidence.mg#L183-L211
[render]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/task_evidence.go#L560-L608
[prior-qa]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/.quality_assurance/journal_entry.md#L262-L264
[prior-decomposer]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/.quality_assurance/campaign/2026-03-01_10-00-EST_decomposer_negative_testing.md#L92-L95
[artifact-admission]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/decomposer_planning.go#L539-L553
[artifact-sanitize]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/normalization.go#L123-L163
[chat-contract]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/architecture/cli/03-GAP-ANALYSIS.md#L117-L123
[read-file]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/tools/core/file_ops.go#L207-L220
[guard-alias]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/tools/core/workspace_guard.go#L9-L32
[guard]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/tools/workspace_guard.go#L153-L255
[tui-entry]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/campaign.go#L29-L84
[tui-run]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/campaign.go#L90-L150
[task-build]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/decomposer_planning.go#L460-L475
[write-inference]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/decomposer_planning.go#L556-L570
[schedule]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_tasks.go#L250-L293
[worker]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_tasks.go#L416-L460
[kinds]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/task_mutation_types.go#L3-L21
[write-set]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_tasks.go#L569-L595
[lease]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_tasks.go#L480-L500
[effects]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/task_effect_contract.go#L8-L37
[routing]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_task_handlers.go#L223-L259
[research]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_task_handlers.go#L402-L436
[canonical]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/types/path_identity.go#L25-L54
[secrets]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/tools/secret_paths.go#L26-L80
[section]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/task_evidence.go#L426-L479
[input]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_task_results.go#L164-L194
[request]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_task_handlers.go#L30-L69
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_task_handlers.go#L89-L119
[isolated]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/session/task_executor.go#L522-L547
[subagent]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/session/subagent.go#L347-L365
[generation]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/session/executor.go#L1941-L2003
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/task_evidence_test.go#L252-L299
[measure-lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/task_evidence.go#L315-L400
[index-control]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/world/structure_refresh.go#L255-L286
[go-os]: https://pkg.go.dev/os@go1.27.3#DirFS
[go-root]: https://pkg.go.dev/os@go1.27.3#Root
[go-root-blog]: https://go.dev/blog/osroot

