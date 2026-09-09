# codeNERD: The Evidence-First Context Compiler

> **Transcript of an external engineering research report.**
> Extracted verbatim from `codeNERD_Context_Architecture_Report1.pdf` on 2026-09-09.
> Layout artifacts (page furniture, dot leaders, hyphenation across line breaks) have
> been cleaned; wording is unchanged. Figures and charts are not reproducible in text
> and are noted where their captions appear.

| Field | Value |
|---|---|
| Title | codeNERD: The Evidence-First Context Compiler |
| Subject | Architecture, reproducible synthetic economics, and implementation guidance |
| Prepared for | Steve Moore, NextGen R&D |
| Date | 9 September 2026 |
| Repository snapshot | `7fd2762d` |
| Evidence boundary | Source review, provider documentation, and executed synthetic simulations. **No live LLM or software-task benchmark.** |

**How to read it:** Chapter 1 is the decision. Chapter 10 is the executed simulation
results — the only new empirical content. Chapters 11–13 are implementation and
promotion gates. Chapter 15 is the final decision and an honest statement of limits.

For how this report compares against what actually shipped in `internal/broker`,
see [`../README.md`](../README.md) and
[`../13-ROADMAP-AND-GATES.md`](../13-ROADMAP-AND-GATES.md).

---

The Evidence-First
Context Compiler

A practical architecture for sparse reasoning lanes,
cache-aware context, and lower verified-task cost.

ONE ACTIVE LEAD. REUSABLE EVIDENCE. SELECTIVE SPECIALISTS.

Prepared for Steve Moore
NextGen R&D

September 9, 2026
Repository snapshot: 7fd2762d

Evidence boundary
Source review, provider documentation, and executed synthetic simulations. No live LLM or software-task
benchmark. Implementation instructions and reproducible results included.

Contents
Read Chapter 1 for the decision, Chapter 10 for executed results, and Chapters 11–13 for implementation and
promotion gates.

- **01** — The recommended design
- **02** — Evidence, scope, and the current codeNERD seam
- **03** — The three-plane data model
- **04** — Prune early with task-specific observation codecs
- **05** — Compile the next obligation, not the latest sentence
- **06** — Cache-aware ordering and two-clock context epochs
- **07** — Reasoning-model and provider compatibility
- **08** — Sparse persistent lanes behind one conversation
- **09** — Price the next plan, not the next token
- **10** — Executed simulations: method and findings
- **11** — Implementation sequence and repository integration
- **12** — Operating instructions and reference policy
- **13** — Prove quality and economics before promotion
- **14** — Worked example: cancellation bug across multiple contexts
- **15** — Final decision and research limits
- **Appendix A** — Ordering and decision mathematics
- **Appendix B** — Reproduce the experiments
- **References** — Sources and provenance
How to read the evidence: [R] inspected repository source; [P] official provider documentation; [L] primary research; [S] executed synthetic
experiments. Citations link to the source register. All performance estimates are labeled by evidence class.

## 01 / The recommended design

Decision in one sentence
Build a single active lead agent backed by a versioned evidence graph and a cache-aware context compiler; add
persistent specialist lanes only when they resolve an independent information need more economically than the
lead. Do not begin with three agents answering every turn, a mandatory summarizer after every response, or
global reordering of live reasoning histories.

This recommendation combines Steve Moore's two ideas: everything crossing an LLM boundary becomes a typed
atom, and one user-visible conversation can be served by several differently organized reasoning contexts. The
efficient implementation is sparse. A lane is a reusable context, not an obligation to buy another inference.

The system should optimize total cost per independently verified completion, with success rate, security, and
deadline constraints. Cache-hit percentage, prompt length, number of agents, and tokens per response are
diagnostic measures, not the objective.

What to build first
The first release should implement a universal inference boundary, lossless native response storage, deterministic
observation projection, versioned result reuse, and a strong single-agent context compiler. This is the foundation
against which every multi-agent extension must compete. The next release can add a requirements/architecture
lane and an independent verification lane, activated by explicit evidence deficits or meaningful risk. A separate
presentation inference should remain optional: the lead can normally provide the final answer itself.

Keep three representations distinct. The event journal records what happened. The evidence graph records what
is currently supported, required, invalidated, or unresolved. The provider request is a temporary materialized view
compiled for the next inference. Preserve chronology in the first representation; use task structure in the second;
respect model protocol and cache constraints in the third.

The user-facing TUI should project accepted events and results from this system into one conversation. It is not
the source of truth for native model state, and it need not expose internal handoffs as a stream of unrelated
personalities.

The operating policy
Prefer, in order: reuse a still-valid result; perform a deterministic operation; continue the active lead with a
compact observation; consult one suitable specialist; use bounded parallel specialists for genuinely independent
questions; escalate the unresolved reasoning to a stronger model; compact or restart when the expected benefit
exceeds the transition cost. This is a preference ordering, not permission to skip required verification or override
safety constraints.

Place durable obligations and trusted contracts early. Put the immediate question, decisive evidence, and
unresolved verification obligations late. Keep useful, stable support between them. Material that is genuinely
irrelevant should be outside the request, not deliberately left in the middle to be ignored. Position is a secondary
optimization after evidence selection and protocol validity.

Run the semantic state update on every event, but run physical reorganization on a slower clock. Between valid
context boundaries, freeze already-exposed native messages and append only needed changes. At a boundary,
select a legal continuation mode and rebuild a better view. Never retain opaque reasoning that is incompatible
with the rebuilt prefix.

What the executed experiments support
This report includes 6,144 primary workload-policy runs, 8,960 additional sensitivity runs, 50,000 stochastic
restart scenarios, and 100,000 fixed-set ordering trials, plus exhaustive checking of 720 orderings. All were
executed as reproducible Python simulations for this report. [S1] They are accounting and scheduling
experiments, not LLM evaluations.

In the base layout model, projecting observations before exposure reduced modeled cost substantially. Periodic
epochs reduced cost a further 11.7% on average relative to projected append-only history, despite lowering
cache reuse. But at a 0.025 cached-read multiplier, the same periodic strategy was more expensive. Frequent
reordering was especially expensive under the base pricing assumptions.

In the routing model, evidence-result memoization was more consequential than agent splitting. Against a lean
single agent with the same memoization, sparse lanes produced mean paired changes ranging from a 0.37% cost
increase to an 8.06% reduction across four synthetic workloads. Always invoking all three lanes was much more
expensive. These results argue for conditional admission, not a universal multi-agent topology.

Do not interpret these numbers as predicted codeNERD savings. The simulations assume that the required
information survives projection and that the abstract information services produce equivalent useful results. They
do not measure reasoning quality, hallucinations, task success, or time to finish a real repository change. Sections
10 and 13 explain how to bridge that gap.

## 02 / Evidence, scope, and the current codeNERD seam

Evidence hierarchy
The report uses four evidence classes. Observed code means a source path was inspected at the pinned commit;
it does not mean its production behavior was executed. Documented provider behavior means the relevant
official documentation was checked on September 9, 2026; account and model support can differ. Synthetic
result means an executable model produced the reported number under stated assumptions. Design
recommendation means a proposed engineering choice, not an externally established performance fact.

No live model calls, real coding benchmark, provider integration suite, or full codeNERD test suite were run for
this report. No repository files were changed. The simulations ran locally and used no API keys. Their scripts, raw
CSVs, configuration manifests, and plotting code are supplied separately.

Repository baseline
The inspected default branch resolved to commit 7fd2762d1e95bf7d870c919e60b5343232751f18, the
September 9, 2026 merge of the MCP progressive-disclosure work. The recommendations are anchored to that
snapshot rather than to an unspecified moving main branch. [R1]

  Existing surface               What is present in the inspected source           Required change for this design

  Shared LLM interfaces          Several completion paths; Message carries         Add a lossless provider envelope and a
                                 role, text, tool calls, and tool results          common accounting/admission boundary

  UsageMetadata                  Input, output, thinking, and cached-content       Preserve provider-native usage and normalize
                                 counts                                            cache writes, reads, ordinary input, and nested
                                                                                   calls

  Tool loop                      Accumulated history is replayed; old payloads     Project before exposure; make later edits
                                 can be bounded while tool IDs/order are           explicit continuation transitions
                                 retained

  Prompt compiler                Atom selection, dependency resolution,            Compile the entire outbound envelope, not
                                 budget fitting, and final assembly                merely one prompt string

  CompilationContext             Versioned hash; activation fields are not fully   Introduce immutable selection snapshots and
                                 represented in identity/copy behavior             dependency-aware cache identity

  Subagent memory                Summary plus recent-turn reconstruction           Route memory transitions through
                                                                                   provider-valid epoch handling

  MCP results                    Structured shaping, elision metadata, and         Generalize this contract to every observation
                                 retained expansion handles                        source

The source surfaces in the table are documented by the pinned files. [R2][R3][R4][R5][R6][R7]

The shared Message representation and response-level thought-signature field are not sufficient to express every
provider's ordered native blocks by themselves. This is a representation limitation, not proof that every adapter

currently mishandles reasoning. Audit each adapter's actual replay path before concluding that a particular
provider is broken. [R2]

The existing tool-loop cap is useful as a defensive bound. However, blanking previously exposed result content is
materially different from selecting a smaller first exposure. The new architecture must distinguish these
operations instead of calling both “compression.” [R3]

The current subagent compression code constructs a summary followed by recent turns. That approach should
not be assumed safe for a continuation whose preserved reasoning is bound to earlier context. A transition
controller must decide whether to preserve, use provider-native compaction, or restart. [R5]

What the research establishes—and what it does not
Observation masking is a serious baseline. The Complexity Trap compared context-management methods in
software-engineering agents and found simple masking competitive with LLM summarization, with additional
benefits from a hybrid policy in the evaluated settings. The lesson here is to measure a low-overhead baseline
before paying another model to summarize every result. It is not evidence that arbitrary deletion is semantically
safe. [L1]

Lost in the Middle demonstrated positional sensitivity on retrieval and question-answering tasks. It motivates
testing beginning/end emphasis; it does not supply a universal attention curve for every current coding or
reasoning model. This report therefore does not simulate accuracy as a function of token position. [L2]

The revised agent-scaling study evaluated 260 configurations and found that architecture-task alignment matters:
decomposition can help, while sequential or tool-heavy work can suffer from coordination overhead. Its reported
performance range is not imported as a codeNERD forecast. [L3]

Mixture-of-Agents supports the feasibility of integrating multiple generated proposals. Self-MoA questions
whether mixing model families is inherently better than using multiple samples from one capable model.
RouteMoA motivates selective routing rather than buying every proposal first. Together, they support
experiments on useful diversity, not automatic three-way fan-out. [L4][L5][L6]

Scope boundary
The design covers hosted-model orchestration, prompt/context construction, tool-result handling, task memory,
and multi-session integration. It does not depend on editing model weights or merging private KV tensors.
Workflow-aware KV caching such as KVFlow is relevant to a future self-hosted inference backend, but it requires
serving-system control and is not an assumption of the portable design. [L7]

## 03 / The three-plane data model

Plane A: immutable events and artifacts
Capture each authorized LLM request, replayable native response item, tool invocation, result, file revision, user
correction, and acceptance decision as an event. Keep the raw artifact separately from its prompt representation.
Large logs, source files, and media should be referenced from a content-addressed artifact store rather than
copied into every graph node.

“Everything is an atom” should mean everything has identity, provenance, type, and lifecycle. It should not mean
serializing an enormous JSON envelope into every prompt. Most metadata belongs in the application, not in the
model's token stream.

Separate content identity from occurrence identity. Identical test output from two executions can share bytes,
while two execution receipts still record different commands, revisions, and times. A content hash is not evidence
that a side effect happened only once. Likewise, cached evidence is not authorization to repeat an operation.

Apply authorization and secret-handling policy before model exposure. Use encryption, access control, and
retention limits for retained artifacts and native continuation data. “Immutable” is an application history property,
not a claim that privacy deletion or legally required erasure can never occur. Deletion must invalidate associated
handles and dependent materializations.

Plane B: versioned evidence and obligations
The graph should represent requirements, constraints, observations, hypotheses, decisions, source spans, action
preconditions, verification receipts, and unresolved questions. Useful relationships include supports, contradicts,
depends_on, satisfies, supersedes, invalidated_by, and requires_exact_representation.

Support often has AND semantics. A test claim may depend jointly on source revision, command, environment,
and output. Represent that as a derivation with a dependency set or hyperedge, not as four independent votes. A
provenance graph can establish where a conclusion came from; it cannot by itself establish that an LLM's
interpretation is true.

A practical epistemic status distinguishes observed, hypothesized, derived, independently checked, contradicted,
and unknown. “Independently checked” must identify the check and its scope. Passing compilation is evidence of
syntactic/type compatibility, not a universal correctness certificate.

Use incremental invalidation. An exact source-span conclusion can depend on a file hash. A “no callers exist”
conclusion depends on the searched index, query scope, and its version, including the absence of matches. Broad
or negative queries often require wider invalidation than their returned rows suggest. Unknown dependency
coverage must be marked unknown; a time-to-live is not proof that evidence remains valid.

Plane C: immutable compiled views
A compiled view identifies selected atoms, their representations, layout, authority, native continuation binding,
output contract, budget, and observed/predicted cache boundary. Once sent, its native replay region is frozen.
New facts create new versions or appended deltas; they do not edit the historical bytes to pretend that the model
saw a different context.

Use a selection snapshot distinct from the atoms themselves. Activation, expected utility, and current trajectory
relevance are transient properties of the task. Updating them should not mutate content identity or accidentally
poison a compilation cache.

A suggested conceptual schema is below. It is an implementation contract, not a drop-in replacement for current
Go types.
Atom {
  ContentID, OccurrenceID, Kind
  Origin { source, original_role, trust_class, producing_event }
  Scope { tenant, project, task_revision, source_versions }
  Status { epistemic_state, validity, supersedes }
  Evidence { dependencies[], contradictions[], obligations[] }
  Views { handle, digest, exact_excerpt, full_payload }
  Native { provider, endpoint_version, replay_ref, binding }
}

CompiledView {
  ViewID, SelectionSnapshotID, TaskRevision
  SelectedRepresentations[], NativeReplayRegion
  ContinuationMode, ProviderProfileID
  InputBudget, OutputReserve, InFlightCostReservation
  RenderHash, PrefixDiagnostics, OmissionManifest
}

Representation rules
An instruction atom retains its original authority. A tool observation remains untrusted data even if it is important.
A derived summary must preserve uncertainty and cannot promote an external document's instructions into
application policy. A user correction creates a new authoritative constraint version rather than an equally
weighted competing opinion.

Every observation can offer a handle, structured digest, exact excerpt, and full payload where meaningful. The
compiler normally chooses one representation; it may add a short end-position restatement when useful. The
materialized manifest must record any deliberate duplication so token accounting and credit assignment do not
count it as independent evidence.

Native continuation atoms are special. Store opaque reasoning/signature objects losslessly, but do not parse them
as ordinary claims or mix them between incompatible sessions. Visible, intentionally requested work
products—hypotheses, test plans, patch rationale—can become evidence atoms. That is different from attempting
to expose or rewrite hidden chain of thought.

## 04 / Prune early with task-specific observation codecs

Order the optimization by avoidable expense
The best token is often one the system never requested. A search should narrow by repository scope, symbol,
relation, or fields before execution. A test query should retrieve the relevant failures and receipts rather than the
entire log. Programmatic aggregation should combine deterministic data before an LLM call when no reasoning is
needed.

After execution, retain the authorized raw artifact and project a useful first exposure. After exposure,
modifications belong to a continuation transition. Pruning generated output later does not refund generation cost;
reduce unnecessary narrative through stable output contracts at the producing call.

Start with deterministic codecs. An LLM summarizer is appropriate when the source is genuinely semantic and a
task-specific parser cannot produce a sufficient view economically. It should have its own budget, validation
policy, and fallback. Do not introduce an unmetered “small helper model” on every result path.

Initial codecs to implement
  Source                         Default first exposure                           Preserve or hydrate exactly

  Test/build output              Command, exit status, revision, failure count,   Full failure details, environment, omitted
                                 distinct failing assertions, relevant stack      output handle
                                 frames

  Source file                    Requested symbol or edit span, enclosing         Current bytes, line/range identity, file hash
                                 signature, immediate dependency contracts

  Search results                 Ranked distinct hits and relation/location       Exact matching spans and full query scope
                                 metadata

  Directory/repository scan      Aggregate structure and relevant changes         Full listing via scoped handle; absence claims
                                                                                  depend on scan version

  Shell command                  Exit code, decisive stderr/stdout, execution     Complete authorized artifact; never
                                 receipt                                          re-execute to expand it

  Specialist response            Claims, evidence IDs, revision, proposed         Underlying exact evidence and patch artifact
                                 action, uncertainty, verification status

  External document              Relevant passage plus source and trust label     Exact supporting passage; no instruction
                                                                                  promotion

The codec must distinguish “zero results,” “results omitted,” “source unavailable,” and “source could not be
parsed.” These states must not collapse into an empty string. An omission manifest should identify what was
withheld and the expansion operation needed to recover it.

Pin exactness at action boundaries
Before a source edit, hydrate exact current bytes and verify the precondition hash. Before claiming a test passed,
require the execution receipt for the applicable revision. Before invoking an unfamiliar tool, obtain the required
schema. Compact summaries can guide exploration, but exactness is mandatory where an operation's correctness
depends on bytes, signatures, or measured output.

Make hydration idempotent over retained artifacts. Expanding an earlier result must not repeat a destructive
command, send another message, create another issue, or execute a second patch. Retained bytes and live
re-observation are different operations and should have different names and permissions. The MCP handle
pattern in the inspected source already reflects this useful distinction. [R6]

Bound metadata overhead
Use compact model-facing identifiers, short source annotations, and stable section schemas. Keep full
dependency sets and verbose provenance in the external manifest. If a 50-token fact needs 300 tokens of
repeated JSON metadata to be shown, the representation is failing its purpose.

Measure useful evidence tokens, structural overhead, and recoveries after omission separately. A codec that
creates many follow-up fetches may be worse than a slightly larger initial excerpt. A reasonable initial controller
expands its first exposure when the same class of result repeatedly requires immediate hydration; it does not
blindly minimize every response.

Do not hardcode an 18% target compression ratio from this report's simulation. That fraction is an experimental
assumption, not a safe content policy. Let each codec preserve semantic units and mandatory fields, then observe
its achieved reduction.

## 05 / Compile the next obligation, not the latest sentence

Derive the live frontier
The compiler should start from the user's current requirements, unresolved acceptance criteria, active failure
hypotheses, and preconditions of the next admissible action. These define a live obligation frontier. Relevance to
the latest natural-language sentence is only one signal.

For “fix this cancellation hang without changing the public API,” the no-API-change constraint remains live even
during a long discussion of channels and goroutines. A rejected lock-contention hypothesis may remain useful in
compact form if it prevents repeating a failed investigation. Conversely, an unrelated successful build log should
not remain merely because it is recent.

The compiler should maintain two channels: the main supporting evidence and a bounded
contradiction/uncertainty channel. The latter preserves counterexamples, disputed interpretations, and missing
prerequisites. This avoids making trajectory pruning a feedback loop that hides everything inconsistent with the
first hypothesis.

Dependency-aware selection
Form candidate bundles around obligations and actions. A bundle contains the required evidence or an explicit
missing-evidence marker, plus a chosen representation for each dependency. Deduplicate shared support across
bundles. If an exact representation is required by an action precondition, the compiler may not substitute an
attractive but insufficient summary.

First satisfy hard contracts: authority, source validity, tool pairing, native continuation compatibility, and
mandatory requirements. Then rank optional bundles by estimated usefulness relative to marginal rendered cost.
Include logical matches, activation, query relevance, calibrated past utility, and uncertainty. Do not allow an
embedding score to override a stale revision or missing permission.

This is a constrained, multiple-representation selection problem, not an ordinary top-k search. An exact global
solver is unnecessary on the critical path. Begin with bounded greedy bundle admission using marginal union cost,
followed by a small number of legal replacement attempts. Bound candidate count, traversed edges, compilation
time, and repair iterations. If mandatory evidence cannot fit, split the action, hydrate selectively, use a supported
larger budget, or reject the proposed execution. Do not silently shave safety or edit preconditions to make the
numbers fit.

Cycles in evidence relationships need not cause recursion without end. Track visited nodes and condense strongly
connected components where appropriate. An unresolved circular justification is not an independent proof. The
compiler's structural closure guarantee applies only to known, correctly recorded dependencies; graph
completeness remains an empirical and instrumentation problem.

One end-to-end budget
Allocate a budget for the fully serialized request, including tools, schemas, instructions, task context, retrieved
evidence, native replay, and current input. Separately reserve generation and any provider-specific reasoning
allowance required by the chosen model's context semantics. A zero remainder is a blocked plan, not a signal to
substitute a large default budget.

Use the provider's counting interface or a tested model-specific counter when available. Otherwise use a
conservative estimate with calibrated headroom and record the estimate error. Reconcile the predicted budget
with returned usage; do not claim exact token enforcement from a character-count heuristic.

The final encoder is the last accounting boundary. No project instructions, file context, tool definitions, or fallback
suffix may be appended afterward without a new count and manifest. Streaming output also goes through a
metered boundary; partial responses and cancellations must retain their native state and incurred cost.

The compile cycle
1. Record the event and update the versioned evidence graph.
2. Resolve the current task revision and live obligations.
3. Reject stale evidence and unauthorized effects.
4. Reuse sufficient, still-valid result atoms when possible.
5. Build bounded candidate evidence bundles.
6. Enumerate legal execution/context plans:
   continue, append focus, hydrate, consult, compact, restart.
7. Choose representations and fit the full outbound envelope.
8. Check native replay and cache-boundary compatibility.
9. Reserve budget; encode; validate; freeze; dispatch.
10. Record native output and actual usage; ingest explicit results.
11. Independently verify applicable claims and update obligations.

These are runtime responsibilities, not eleven extra LLM calls. Most are deterministic data operations. The normal
case should be one materialization and one inference, or zero inference when an existing result suffices.

Incremental implementation at large repository scale
Do not rescan a ten-million-line repository or clone an unbounded world graph for every compilation. Maintain
symbol, dependency, query-scope, and reverse-invalidation indices. The per-call kernel view should operate on a
bounded frontier and immutable index snapshots or overlays.

Cache structural subplans separately from task-specific materialization. A type contract or stable projection can
be reused under its dependency fingerprint; selection must still account for the current task. Changes should
invalidate affected bundles rather than changing a global workspace hash that makes every cached result useless.

Likewise, do not solve a graph-partitioning problem after every tool result. Observe co-use and invalidation
statistics, and reconsider lane topology only at meaningful task boundaries. Architectural sophistication must not
become an unmeasured CPU and latency tax.

## 06 / Cache-aware ordering and two-clock context epochs

Four different caches
A local compiler cache avoids repeating application computation. A result cache avoids repeating a valid inference
or deterministic operation. A provider prompt cache discounts processing of a matching prefix. A self-hosted KV
cache gives serving infrastructure more direct control. Their keys, validity rules, and economic effects differ.

The local CompilationContext hash is not a provider-cache key. Giving every atom a stable identifier does not
make it independently reusable at arbitrary positions in a hosted prompt. Current OpenAI documentation requires
the relevant rendered prefix and settings to match; its current supported models also differ in breakpoints,
minimum lengths, write accounting, and retention. [P1]

For the compiler, provider-cache cost is non-additive. Changing an early atom can destroy reuse of a long suffix.
Therefore an atom score cannot simply subtract a fixed “cached-token discount” from its own cost. Evaluate
complete legal candidate layouts, including their longest reusable prefix.

The recommended physical arrangement
The stable front contains application policy, durable task requirements, and useful reference contracts. A frozen
support region contains stable evidence selected for the current epoch. The active continuation contains the
provider-native history. A small attention tail emphasizes what changed, what matters next, and what remains
unverified.

Keep the most recent user request identifiable. Do not reframe it as an old assistant conclusion. Task data must
remain data even when positioned near the end. A provider adapter, not a generic string concatenator, decides
which roles and block types may carry each region.

The tail should contain the decisive fact or excerpt, not merely an opaque pointer that forces a tool call to
understand the next action. It should not duplicate an entire large document. Append a new short delta only when
something materially changes. Bound cumulative reminders and retire them through a legal epoch transition.

Change meaning frequently, layout selectively
The semantic clock updates activation, evidence validity, and obligations on every event. The layout clock
advances only when a transition is justified. This separates responsiveness from cache churn.

At each boundary compare a small candidate set: retain the current continuation; retain it with a compact focus
update; hydrate a missing exact excerpt; use a supported provider compaction/editing operation; or create a fresh
task-oriented handoff. A fresh handoff retires incompatible native reasoning. It is not a retroactive rewrite of the
same signed conversation.

Use hysteresis and a minimum residence interval to prevent near-equal scores from constantly swapping content.
Correctness can override residence: a changed user requirement or stale edit precondition must be handled
immediately. The response may be to append a correction and block effects, not necessarily to rewrite the entire
prefix.

Stable-first is more precise than important-first
Among already-selected, same-authority, freely reorderable blocks, both length and change probability matter for
cache reuse. Appendix A derives a simple adjacent-swap rule. It prioritizes a block using length times survival
odds, subject to task and protocol constraints. The rule is exact only in its stated fixed-set, independent-change
model.

The empirical ordering experiment found that placing volatile content first destroyed much more reuse than the
finer difference between two stable-first heuristics. Accordingly, the first engineering win is to remove
timestamps, counters, transient state, and fluctuating rosters from early stable material. Fine-tuning the order of
two stable references is a lower-priority optimization.

This rule does not justify adding irrelevant content to a prompt, moving safety constraints into an obscure region,
or reordering a native reasoning chain. First choose sufficient content. Then optimize layout within the
permissible degrees of freedom.

Cache boundaries and namespaces
A physical cache namespace must respect tenant, processing region, model compatibility, and authorization.
Logical lanes can share genuinely common prefixes within an authorized scope, but different users must not
receive each other's private material. Stable keys help grouping and accounting; they are not a mechanism for
merging incompatible model state.

Use an explicit cache boundary before a volatile suffix when the provider supports it and measurements justify it.
Do not pay to write a large suffix that is unlikely to be reused. Conversely, a growing native continuation may be
worth caching because it is repeatedly replayed during a tool sequence. The adapter must implement the
provider's actual boundary rules rather than assuming every atom boundary is eligible.

Cache lifetime is not task lifetime. A lane can stay dormant in application storage while its provider cache expires.
Do not issue empty keepalive inferences merely to preserve a hit without a measured economic justification. A
later cold start may be cheaper than artificial traffic.

## 07 / Reasoning-model and provider compatibility

Portable rule
Preserve replayable native items as returned, in their required order, along with tool-call/result relationships and
opaque signatures. Retain full raw responses for audit, but replay only the endpoint's documented replayable
items—not arbitrary top-level response metadata. Preserve unknown returned content in storage and require a
tested adapter behavior before forwarding it.

Separate three modes in the API abstraction: continue, provider-managed compaction/editing, and fresh
handoff. The view compiler may select among supported modes; it must not improvise an undocumented blend.

Provider findings checked September 9, 2026
  Provider surface               Documented constraint relevant here               Design consequence

  OpenAI Responses               Available opaque reasoning can be persisted;      Preserve complete replayable items or
  reasoning                      compatibility and effective reasoning-context     supported stored continuation; treat
                                 mode depend on the model family                   cross-family routing as a potential restart

  OpenAI standalone              The returned compacted window is the              Retain the returned window intact before
  compaction                     canonical next window and may contain more        appending new input
                                 than its encrypted compaction item

  Claude preserved thinking      Newer model behavior binds thinking to            Freeze prior native content; explicitly test
                                 compatible model history and an unchanged         mismatch behavior and transformations
                                 preceding prefix; enforcement can depend on
                                 account settings

  Claude caching                 Cache reads, writes, lifetime, and thinking       Keep provider-specific usage; do not assume
                                 settings have distinct consequences               all thinking-setting changes preserve the
                                                                                   prefix

  Gemini Interactions            Stateful continuation manages signatures;         Implement a separate Interactions profile
                                 stateless use must preserve returned thought      rather than reusing legacy Generate Content
                                 blocks                                            serialization

  Gemini Generate Content        Signatures can be attached to particular parts,   Preserve part-level binding and complete
                                 with function-call ordering requirements          relevant native content

The findings above are summaries of official documentation, not account-level integration tests. The report
deliberately avoids making an optional beta feature necessary for the base architecture. [P2][P3][P4][P5][P6][P7]

Reasoning continuity is a protected resource
A preserved chain may improve continuation, but a stale chain may also be inappropriate after a major task
revision. The scheduler should price a transition and enforce compatibility rather than assume that maximum
retention is always best.

Do not concatenate three specialists' opaque reasoning objects into the lead's context. Merge explicit findings
and exact evidence. The lead can learn from supplied work products without gaining access to the specialists'

hidden internal state. If two lanes need a shared reasoning continuation, use a documented compatible fork or a
new joint inference with explicit evidence; do not invent a generic KV merge.

Request short explicit work products such as claims, alternatives, checks, and uncertainty. Do not require raw
hidden chain of thought for the graph to function. A summary of reasoning is also not a guarantee that every
internally relevant fact has been captured.

Capability registry and contract tests
The provider profile should key on provider, endpoint/API version, model identity, relevant account features, and
adapter version. It should state replay rules, supported continuation modes, counting behavior, cache boundaries,
pricing normalization, and allowed model transitions. Keep access policy separate from cache compatibility.

Before enabling a profile, round-trip tool sequences, streamed signatures, empty reasoning blocks, multi-part
output, cancellation, and supported model transitions through a sandbox integration suite. A failed compatibility
check must select a safe supported path or fail clearly. Do not silently discard reasoning, tool results, or new user
constraints to keep the conversation moving.

Gemini's current Interactions caching is implicit; explicit cache objects remain a Generate Content option. This
distinction illustrates why API surface belongs in the profile. Explicit storage billing and duration must be included
when that option is used. [P8][P9]

## 08 / Sparse persistent lanes behind one conversation

Start with one active lead, not a compulsory ensemble
Give the lead enough reasoning capability to integrate ambiguous evidence, direct the tools, and recognize when
its context is insufficient. Add two logical lanes: requirements/architecture and independent verification. Their
contexts persist in application storage, but they run only on admission. Do not schedule idle inferences to keep a
provider cache warm; account for cache expiry when a genuine request arrives.

The implementation lane and the user-facing lead can initially be the same agent. This avoids a mandatory
synthesis call between every coding action and the user. When hard technical work is already resolved and
checked, a cheaper presenter may be useful for a particular product experience. When findings conflict, however,
integration is hard reasoning and should not be assigned to a weak model merely because it is called a
summarizer.

A lane's durable identity should describe its workspace, authority scope, specialty, compatible model family,
reference snapshot, and context epoch. An invocation ID describes one job. Conflating those IDs would turn
every new subtask into a cold session even when an existing context remains suitable.

Admit a lane for an information deficit
The architecture lane is appropriate when the proposed patch crosses an interface boundary, a user constraint
conflicts with an implementation plan, or the lead cannot establish a required invariant. The verification lane is
appropriate when a patch needs independent counterexample search, tests may bypass the production path, or a
consequential claim lacks a test receipt. A local syntactic repair generally should not trigger both.

Consultation should specify the question to resolve, not ask for an unrestricted opinion. Give the specialist a
versioned evidence slice and an output contract. Cap its context, generated work product, tool calls, and total
spend. The cap is a guardrail; it is not permission to truncate away required evidence without disclosure.

If the questions are independent, two consultations may run concurrently. If one needs the other's conclusion,
schedule the dependency instead of manufacturing parallelism. The primary research on agent scaling supports
task-dependent orchestration rather than a fixed number of agents. Mixture-of-Agents and Self-MoA also caution
against treating model diversity itself as proof of better results. [L3][L4][L5]

Integrate evidence packets, not complete exploratory transcripts
Use a packet with the following logical fields. Most bookkeeping stays outside the rendered prompt.
EvidencePacket
  question_id; task_revision; source_snapshot
  claims[]: content, epistemic_status, supporting_atom_ids
  contradictions[]: claim_ids, distinguishing_evidence
  artifacts[]: exact_patch_or_result_reference, content_hash
  verification[]: command, environment, revision, outcome
  unresolved[]: missing_evidence, proposed_discriminating_check
  proposed_action: optional; never self-authorizing
  expansion_refs[]; native_continuation_ref

The evidence joiner validates the packet's scope and dependencies. Compatible claims can be combined.
Conflicting claims remain visible as a conflict requiring resolution. Repeated claims that descend from the same

source do not become independent evidence through repetition. Three agents reading the same wrong
annotation should not outvote a directly reproduced failure.

A packet may suggest an effect, but only the authorized execution boundary may perform it. Start with one
writer. Parallel patch exploration should use isolated worktrees or equivalent snapshots, followed by an explicit
merge and verification step. A proposed patch must declare its read set and expected pre-edit hashes;
mismatches trigger revalidation rather than a blind apply.

Result reuse comes before another model call
Before dispatch, check whether a still-valid packet already answers the same obligation under compatible
constraints. Match meaning and scope conservatively; a loose embedding match is not enough to reuse a
correctness claim. Exact question templates, normalized parameters, and dependency versions are useful initial
keys.

Do not key every reusable result only on the entire repository commit. That is safe but excessively invalidates
unrelated knowledge. Track the smallest sound dependency set. Conversely, do not track only positive source
hits: a claim that no caller exists depends on the queried index scope, and a passing test can depend on
configuration, environment, fixtures, and transitive code. Negative observations require a validity certificate for
the search or execution scope.

A valid result may satisfy a repeated machine information demand without any inference. It does not
automatically satisfy a new user request for a fresh check. If the user asks to rerun a test, record and perform that
new occurrence; identical output bytes do not make the execution interchangeable.

Use a small verified casebook for in-context adaptation
A specialist can retain a compact selection of validated examples, conventions, and previous failure patterns.
Promote these into a versioned casebook at an epoch boundary, rather than appending every generated opinion
as a lesson. The model receives the selected casebook as context; the provider cache only reduces repeated
processing of the same compatible rendering. It does not update model weights or establish that a lesson is true.

Task execution evidence should remain separate from general lessons. A historical test pass is not a current test
pass. A useful debugging pattern is guidance, not proof that the next bug has the same cause.

One global admission controller
Every subagent may request a consultation, but it should ask the same global scheduler. It should not create a
private recursive committee with an independent budget. Set a global concurrency cap, reservation ledger, depth
limit, and cancellation tree. Reuse an existing suitable lane when possible.

The interface shown to Steve remains one conversation projected from canonical events: request received,
investigation authorized, evidence accepted, edit applied, verification completed, answer emitted. If a new user
instruction arrives, increment the task revision and invalidate affected plans. A late specialist result can be
archived or partially reused, but must not silently answer a superseded request.

## 09 / Price the next plan, not the next token

Normalize actual cost before optimizing it
Maintain the provider's raw usage record and a separately normalized ledger. Distinguish ordinary input, cached
reads, cache writes by lifetime, visible output, billed reasoning, explicit cache storage, tool fees, and
context-management calls. Reasoning that is already included in billed output must not be charged twice. Failed
calls, retries, canceled work, and subagent usage belong in the same task account.

The relevant objective is expected remaining cost to a verified completion, subject to quality and policy
constraints. In compact form:
remaining_cost(plan) = input + cache_writes + cache_reads
                     + generation + tools + storage
                     + context_management + expected_rework

Elapsed time is another constraint, not identical to spend. Parallel requests can lower latency while increasing
total cost. Record deadline risk and the critical path separately; use an explicit dollar-equivalent latency penalty
only if the product owner chooses one.

Current provider pricing differs by model, endpoint, and cache lifetime. The experiments therefore use normalized
multipliers, not a purported universal dollar rate. A base read multiplier of 0.10 and write multiplier of 1.25 are
documented examples, while a 0.025 read multiplier is also relevant to current Claude offerings. These are dated
examples; production should obtain rates from a versioned configuration matched to observed usage. [P1][P5]

Compare a small set of legal actions
For each boundary, consider valid result reuse, deterministic work, continued lead inference, one specialist,
bounded parallel specialists, model escalation, native compaction, and a fresh handoff. Reject actions that violate
authority, evidence requirements, model compatibility, or the task's remaining budget. Price the remaining actions
as complete plans.

Initially, use conservative rules rather than a learned policy claiming to know success probabilities. Trigger a
consultation on a real missing obligation or repeated failure. Estimate its value from comparable completed tasks
only when there is enough data, and condition on the evidence already available. A second independent test may
have value that a fourth repeated explanation does not.

Routing work to a warm but unsuitable specialist is a false economy. Likewise, a cheaper model is only cheaper
per completed task if its additional rework and failure rate do not erase the rate difference. Keep a capable lead as
the default until a paired evaluation supports a lower tier for a defined task class.

The rebase inequality
Consider a warmed, changeable region of B tokens that can be replaced by S tokens. Let H be the number of
remaining calls using that region, r the cached-read multiplier, w the write multiplier, q the probability of reuse on
each subsequent call, and K the extra transition overhead in ordinary-input-token equivalents. The common
unchanged prefix and unchanged output cost cancel in this simplified comparison.
m = q*r + (1-q)*w
keep = r*B + (H-1)*m*B
rebase = w*S + (H-1)*m*S + K
rebase only when rebase < keep

The first old-region read is assumed warm. A cold current request needs a different first term. K must include
extra summarization, additional generation needed to reconstruct a plan, and any otherwise omitted transition
costs. When a new layout changes subsequent output length or task success, those effects must also be
measured rather than hidden inside an unexplained constant.

For q=1, a strict break-even condition is:
(H-1)*r*(B-S) > w*S + K - r*B

Cheap cached reads can delay break-even substantially. Low cache survival can make retaining a large region
expensive again. A shorter remaining horizon makes a one-time rebuild less attractive. These competing effects
explain why neither always-compacting nor never-compacting is a universal rule.

Use a forecast of future demand, not the future realized trace. Require an uncertainty margin and a minimum
residence interval unless correctness forces a transition. The report's restart experiment tests a 15% forecast
margin; that is an experimental policy, not an empirically tuned production default.

How much quality improvement would extra agents need?
Let C be mean cost per attempted task and p be verified success probability under a fixed attempt policy. The
ratio C/p is the large-sample cost per verified success when unsuccessful attempts remain in the numerator. A
proposed configuration with cost C1 and success p1 improves this ratio over C0,p0 only if:
p1/p0 > C1/C0

This is a screening calculation, not a model of arbitrary retries. If the baseline already succeeds on 90% of tasks,
even reaching 100% allows only about an 11.1% cost increase under this metric. A threefold cost increase cannot
be justified on that workload by success-rate improvement alone. It might still be justified by a different
requirement, such as reducing severe defects, but that requirement must be explicit.

Do not optimize a ratio while hiding lower success, discarded hard tasks, or degraded tail latency. Report the
numerator, denominator, workload coverage, and verification criteria separately.

## 10 / Executed simulations: method and findings

What was actually run
The attached Python suite performs synthetic trace replay, exact block-prefix accounting, version-aware result
memoization, a stochastic restart decision model, and fixed-set ordering experiments. It makes no external
inference calls and does not run codeNERD, SWE-bench, or a language model. Its purpose is to expose economic
failure modes and choose what to test in the implementation.

  Experiment                     Executed scale                                  Question it answers

  A: context layouts             256 paired traces x 4 policies = 1,024 runs     How do projection, periodic rebuilding, and
                                                                                 prefix disruption affect modeled spend?

  B: information-demand          4 workloads x 256 traces x 5 policies = 5,120   Does splitting contexts beat a lean single
  routing                        runs                                            agent with equivalent result reuse?

  A/B sensitivity                8,960 additional workload-policy runs           How do cache price, TTL, projection ratio,
                                                                                 demand overlap, and horizon change the
                                                                                 result?

  C: restart policy              50,000 randomized scenarios                     How does a forecast-based guard compare
                                                                                 with always/never rebuilding?

  D: fixed-set ordering          100,000 change trials; all 720 permutations     Which legal ordering maximizes expected
                                 checked                                         surviving prefix length in a simple model?

The main random seed is 20260909. Experiment-specific seed offsets and all parameters are recorded in the
scripts and manifests. Bootstrap intervals below resample paired synthetic traces 1,500 times. They quantify
variation under the chosen generator, not uncertainty about actual LLM performance. A narrow interval cannot
compensate for an unrealistic assumption.

Shared accounting assumptions
One cost unit is the assumed price of one million ordinary input tokens. Ordinary input has multiplier 1, the base
cached read is 0.10, the base cache write is 1.25, and the base generation multiplier is 5. Generation prices are
illustrative, not tied to a named model. All policies in each primary comparison use the same model tier.

The prefix cache hashes block identities, lengths, and all preceding blocks. It supports all eligible block-end
boundaries, unlimited capacity, a 1,024-token minimum, and a base 1,800-second TTL refreshed by use. This is an
idealized reuse upper bound: actual provider breakpoint limits, routing misses, concurrency warm-up, eviction,
multimodal counting, and signed-history restrictions require adapter-specific measurement. Native protocol
safety is an assumption of the legal transition, not something this cache simulator proves.

The suite checks token conservation, early-prefix invalidation, TTL expiry, memo invalidation after a dependency
revision, algebraic break-even, and equal memo eligibility for the lean and split baselines. Outputs and
management charges are included. CPU inference latency and task success are not simulated.

A. Early projection is the first economic lever
Each trace contains 48 decisions, an 18,432-token common base, and variable tool observations. Raw
observations are lognormally distributed around a 5,000-token median and clipped to 900–18,000 tokens.
Projected observations retain 18% with a 200-token minimum. Every policy pays for 1,200 generation tokens per
decision; 400 visible answer tokens enter later replay. All policies obey a 120,000-token input ceiling.

Raw append keeps observations until pressure forces a checkpoint. Projected append reduces them before
exposure. The epoch policy adds a 200-token attention update and rebuilds every 12 decisions from a checkpoint
and a small recent exchange window. Every rebuild pays an additional 3,000 ordinary-input-token equivalents.
The every-step ordering policy is a deliberately volatile random-order stress test over a bounded recent
region—not an implemented semantic relevance algorithm. Its changes are modeled as fresh legal snapshots, not
edits to protected native thinking.

  Policy                                                   Mean total cost               Cached fraction               Mean rebuilds

  Raw append + pressure bound                              1.0481                        89.3%                         2.87

  Projected append                                         0.7023                        96.0%                         0.00

  Twelve-decision epochs                                   0.6203                        91.1%                         3.00

  Every-step volatile reorder                              1.1343                        63.6%                         47.00

Figure 1. Synthetic layout accounting. Lower total modeled cost is better; generation is held equal, and transition charges are included. This
is not a measured coding-success result. [S1]

The ratio of mean projected-append cost to mean raw-append cost is about 0.670, a 33.0% reduction in this
generator. Against projected append, the epoch policy saves a mean paired 11.66% with a 95% bootstrap interval
of 11.55–11.78%. Its mean cache fraction is lower: about 91.1% versus 96.0%. More cache hits were not the
cheaper plan.

The volatile every-step policy costs about 61.5% more than projected append by ratio of means. That finding
supports avoiding unnecessary layout churn under discounted reads; it does not prove that all adaptive reordering
is harmful, because the simulator gives reordering no reasoning-quality benefit.

Figure 2. Cache-price sensitivity with 64 paired traces per policy and rate. At a 0.025 read multiplier, the fixed epoch policy loses to
projected append. Neither layout is universally optimal. [S1]

At a 0.025 read multiplier, the periodic policy costs approximately 0.510 units versus 0.497 for projected append,
about 2.6% more. The same policy becomes attractive as cached reads become more expensive. A separate
sweep varies projection fractions from 0.10 to 0.60 and horizons from 12 to 96 decisions. Its raw results are
included; the production policy should use those dimensions as inputs, not copy the arbitrary twelve-decision
schedule.

B. Give the single-agent baseline the same advantages
Experiment B models 60 information-demand events over three domains. The lean agent receives only the
required domain references, never the whole corpus by default. Where memoization is enabled, it has the same
valid-result reuse eligibility as the sparse lanes. Both can reuse compatible prefixes. Useful abstract answers are
assumed equivalent across policies; no intelligence advantage is assigned to multiple agents.

Each domain has 18,000 reference tokens, plus a shared 6,144-token base. An information component costs 700
generation tokens, and a decision costs 300. A one-domain sparse consultation does not require a separate
presenter; multi-domain work pays an integration call with compact packets. A fixed fan-out policy calls all three
services and the integrator on every event, yielding 240 calls per trace.

The four workloads vary repetition, dependency change, cache-expiring gaps, and whether an event needs two
domains. The name “multi-domain churn” means more changing, multi-domain demands; it does not simulate truly
indivisible cross-domain reasoning. New source versions invalidate results. A memo hit means an abstract
information demand is satisfied without an inference, not that a real coding task completes without execution or
verification.

  Workload                                Lean                 Lean + reuse            Sparse + reuse              Always three

  Local / stable                          0.6759               0.2933                  0.2909                      1.5346

  Mixed reuse                             0.9819               0.5293                  0.5094                      1.6514

  Multi-domain churn                      1.8736               1.5186                  1.3946                      2.2960

  Long idle gaps                          1.2348               0.6690                  0.6709                      1.8917

The largest gains generally come from avoiding repeat inference through valid-result reuse. In the mixed-reuse
workload, lean-agent cost falls from approximately 0.982 to 0.529 units when memoization is enabled. Splitting
that already-memoized baseline into sparse lanes lowers the ratio-of-means cost only to approximately 0.509.
The architecture should capture the first gain before paying to engineer the second.

Figure 3. Mean paired cost reduction of sparse memoized lanes versus the lean memoized agent. Error bars are 95% bootstrap intervals
over 256 synthetic traces per workload. Negative values mean splitting costs more. [S1]

  Workload                                Mean paired cost reduction                     95% synthetic interval

  Local / stable                          0.66%                                          0.30% to 1.03%

  Mixed reuse                             3.49%                                          2.92% to 4.01%

  Multi-domain churn                      8.06%                                          7.65% to 8.43%

  Long idle gaps                          -0.37%                                         -0.68% to -0.06%

The long-gap workload is especially instructive. Independent specialist caches can cool between uses; context
separation is not a free shared-memory mechanism. Even where splitting saves input cost, it may require more

calls: in mixed reuse, the memoized lean baseline averages 31.7 calls and sparse memoized lanes 45.8. Extra
real-world latency, integration errors, or output could erase a small accounting advantage. Fixed three-way
fan-out averages 240 calls.

Figure 4. Routing sensitivity at equal model prices. Curves vary cache TTL while changing the fraction of demands requiring two domains.
Values below one favor sparse lanes in this simplified information-service workload. [S1]

The full routing sweep also includes a specialist price scale of 0.35 as a what-if. It does not establish that a
cheaper model has equal capability. Do not use those rows as evidence that smaller specialist models maintain
coding quality. The primary result and figure above use equal prices.

C. Restart only when a plausible horizon pays for it
For a 30,000-token warmed region replaced by 12,000 tokens, write multiplier 1.25, and subsequent reuse
probability one, the first strictly profitable remaining-call horizon is:

  Read multiplier                            K=0                             K = 5,000                      K = 20,000

  0.025                                      33                              44                             78

  0.1                                        8                               11                             19

  0.25                                       3                               4                              8

  0.5                                        2                               2                              4

Figure 5. Analytical restart thresholds checked by the simulator. Transition overhead is expressed in ordinary-input-token equivalents. The
calculation excludes a quality benefit or penalty unless represented in the overhead. [S1]

The stochastic experiment samples 50,000 scenarios: old region sizes of 24,000–72,000 tokens, replacement
fractions of 0.25–0.75, three cache-read rates, three reuse probabilities, and three transition overheads.
Remaining calls are geometrically distributed around latent means of 3, 8, or 20, capped at 60. The deployable
guard sees a noisy forecast of the latent mean—not the realized future horizon. It rebases only when predicted
cost is at least 15% below predicted keep cost.

  Policy                                                          Mean cost                      Mean regret versus oracle

  Never rebase                                                    0.19011                        0.07318

  Always rebase                                                   0.13102                        0.01410

  Forecast guard                                                  0.12718                        0.01025

  Perfect-future oracle (bound only)                              0.11693                        0.00000

The guarded policy beats both always and never rebasing in average cost under this selected distribution. The
oracle uses the realized future horizon and is an unattainable comparison bound, not an implemented scheduler.
Changing the horizon or restart-cost distributions could change the ranking. Real reasoning loss after a handoff
may be much larger or smaller than the chosen overhead. Record it in live evaluation rather than treating this
model's constant as truth.

D. Stable early does not always mean smallest change probability first
A separate experiment holds the selected content fixed. Six freely reorderable blocks total 25,600 tokens and
have independent between-request change probabilities. There is no position-dependent quality model. Only the
surviving identical prefix is priced. All 720 permutations are checked, followed by 100,000 shared random change
trials.

  Ordering                                            Expected cached tokens     Empirical cached tokens

  Volatile-first stress                               3,495.5                    3,524.4

  Change probability only                             23,404.9                   23,397.9

  Length-adjusted stability                           23,559.5                   23,552.5

For adjacent blocks i and j, placing i first is better in this model when:
length_i * (1-change_i) / change_i
    >= length_j * (1-change_j) / change_j

A never-changing block goes first. This length-adjusted stability rule achieves the exact optimum for the six-block
model. Sorting only by change probability is already close: the improvement is about 155 expected cached tokens
out of 25,600. Moving highly volatile state out of the front creates the much larger improvement.

Apply this rule only inside regions that are free to reorder and comparable in semantic placement priority. User
authority, native protocol, dependency order, and evidence sufficiency dominate it. The experiment does not
license moving urgent constraints into an obscure location merely because a large document is stable. Appendix A
provides the derivation and the assumptions that limit it.

What the simulations deliberately do not claim
They do not show that an 18% projection retains all necessary information, that twelve decisions is the best
epoch size, that multiple models are more correct, that in-context learning improves solve rates, or that provider
reasoning survives an arbitrary rewrite. They do not model real attention, human behavior, code execution,
repository search quality, or adversarial prompt injection. They do not claim statistically established
cost-per-success improvements because success was not measured.

They do show, within these stated models, that early projection, valid-result reuse, cache price, cache survival,
coordination overhead, and horizon can reverse seemingly obvious choices. Those mechanics are sufficient to
reject fixed fan-out and unconditional reordering as defaults, and to motivate the controlled implementation and
evaluation that follow.

## 11 / Implementation sequence and repository integration

Phase 0: make the boundary observable and lossless
Begin at internal/types/interfaces.go, the provider adapters, the session executor, and all direct completion
callers. Add a typed inference request/response boundary with ordered native replayable blocks, raw response
retention, normalized usage, and a provenance manifest. Preserve existing adapter behavior while the new path
runs in shadow mode.

Do not force every provider into the least expressive text/tool structure. Store provider-native replay objects
separately from the normalized semantic view. The normalized view is for selection, routing, and audit. The native
object is for legal continuation. A stable abstraction can reference both without attempting to reinterpret an
opaque signature.

The following is an interface sketch, not drop-in code or a claim that these names already exist in codeNERD:
type InferenceBroker interface {
    Compile(ctx context.Context, req ViewRequest) (CompiledView, error)
    Execute(ctx context.Context, view CompiledView) (RecordedResponse, error)
}

type ProviderProfile interface {
    ValidateContinuation(view CompiledView) error
    Render(view CompiledView) (NativeRequest, error)
    Count(req NativeRequest) (TokenEstimate, error)
    NormalizeUsage(raw NativeUsage) (UsageLedgerEntry, error)
}

Inventory every live call path, including perception, critic, summarization, streaming, tools, subagents, and
headless commands. In production, network-capable model clients should only be reachable through the broker.
A diagnostic test can install a sentinel client and assert that each path produces a request manifest. No
downstream helper may append unbudgeted text after final rendering.

Exit gate: a representative call through every path emits a complete manifest and reconciled usage; native golden
fixtures survive round-trip serialization; the change does not alter selected prompt content yet. The report's code
inspection motivates this work but does not substitute for those tests. [R2][R3]

Phase 1: artifact retention, codecs, and sound result reuse
Generalize the MCP digest-and-handle pattern into a shared artifact service for files, searches, test results, shell
output, and subagent work products. Preserve original authorized bytes, scope, content hash, occurrence ID, and
the projection recipe. Avoid duplicating a second generic handle system if the current one can be extended
cleanly. [R6]

Each codec should produce structured omissions and an exact expansion route. Tests must cover enormous single
fields, multi-byte text, empty legitimate results, errors near the tail, and bounded rendering without losing status.
A model should be able to distinguish “zero matches” from “matches omitted” from “source unavailable.”

Add result certificates with positive dependencies, negative query/index dependencies, environment versions,
and task-constraint scope. Invalidate them on relevant writes or state changes. Start with exact normalized
demand keys; add semantic reuse only after a separate validation strategy exists.

Exit gate: deterministic codec contracts and invalidation tests pass; the lean baseline gains artifact recovery and
memoization; no result is labeled current after a relevant source change. Valid-result reuse counts are visible
separately from provider cache hits.

Phase 2: build the lean context compiler
Adapt internal/prompt to compile the whole request envelope, not only its system prompt. Extend existing
selection and dependency machinery with typed evidence representations, obligation coverage, and an explicit
native-continuation region. Route project instructions, file context, tool schemas, observations, and task-state
updates through the same final budget ledger.

Maintain strict input/output reserve accounting using the selected model's actual limits and caller intent. Count
the fully serialized request where supported, or use a conservative calibrated estimate with a stated margin. If
mandatory content exceeds the budget, select a supported compaction/handoff or fail with an actionable error.
Do not silently truncate away a user constraint or a required protocol object.

Keep selection and rendering deterministic for identical snapshots. Hash every output-affecting source, transform
version, and context signal used by the compiler. If ActivatedFacts becomes live, add its immutable snapshot
identity to cache dependencies and deep-copy or otherwise freeze its contents. The existing context hash does
not currently include that map. [R4][R7]

Exit gate: the optimized single-agent baseline is correct, reproducible, and instrumented. Its evidence coverage,
expansion rate, request length, observed cache reads, and completion costs form the real comparison baseline.

Phase 3: introduce legal context epochs
Replace direct in-place history blanking and generic memory-summary rewrites on the new path with explicit
continuation transitions. Preserve the old path behind a rollback flag until parity is proven. Add candidate plans
for continue, bounded attention append, hydration, native compaction, and fresh handoff. [R3][R5]

Initially, rebase only at obvious safe boundaries: a completed tool exchange, a phase change with no outstanding
effect, or a supported native checkpoint. Even at a completed exchange, preserved reasoning may bind to older
history; safe means adapter-validated, not simply “the tool returned.” Price the rebuild, including all extra calls and
reasoning effort.

Test cross-turn user revisions and cancellations. If a constraint changes, correctness can force a transition even
when the cache would prefer continuing. Save enough event and evidence state to recover from a crash between
compilation and dispatch. Persist exact manifests so a replay can explain the decision without needing a live
provider cache.

Exit gate: native compatibility tests pass, protected items are never silently rewritten, and every transition is
auditable with its expected and observed cost.

Phase 4: add two optional persistent lanes
Extend the subagent lifecycle with stable lane IDs, per-lane context epochs, evidence contracts, and
version-aware admission. Keep the spawner's global usage and configuration propagation, and ensure lane calls
use the same broker. The existing isolated subagent and planner-client hooks provide a useful starting point.
[R5][R8]

Initially admit a specialist through explicit conditions: missing architectural obligation, independent verification
need, consequential disagreement, or repeated ineffective actions. Allow zero or one consultation for most
decisions, and two only for independent questions. Do not require an integrator when the lead can consume one
validated packet directly.

Use the same logical mechanism for subagent consultations without allowing private recursive budgets. Record
what question a consultation resolved and whether the lead actually used the returned evidence. A warm lane
that consistently contributes nothing should be retired or merged.

Exit gate: lanes outperform the already-optimized lean baseline on a predeclared workload and quality criterion,
not merely on token counts or prompt-cache percentage.

Phase 5: learn a scheduler only from trustworthy outcomes
After the accounting and verification foundation is stable, estimate admission value from completed tasks and
controlled ablations. Use task-level holdouts. Include failed and canceled consultations. Compare stronger versus
cheaper tiers on matched task classes, and measure integration failure independently from specialist accuracy.

Do not learn routing credit solely from the final model's opinion that an atom was useful. Ground outcomes in
patch acceptance, tests that exercise the production path, requirement checks, and avoidance of measured
rework. Preserve causal ambiguity in the metrics: a correlated improvement after a consultation is not
automatically caused by it.

Exit gate: a learned policy has an interpretable fallback, calibrated uncertainty on its supported domain, a global
spend cap, and an evaluation showing improvement over the rule-based scheduler.

## 12 / Operating instructions and reference policy

A deployable starting policy
The configuration below is a proposed interface for implementation, not an existing codeNERD configuration
schema. Token caps are initial engineering settings to tune with real traces; they are not measurements from the
synthetic experiment. Resolve hard context limits, pricing, and replay rules from the provider profile.
context_compiler:
  mode: shadow_then_guarded
  native_replay: lossless
  projection: before_first_exposure
  raw_artifacts: retained_with_access_policy
  final_request_budget: required
  mandatory_overflow: compact_handoff_or_error
  evidence_reuse: dependency_validated
  candidate_limit: 2000
  expansion_edge_limit: 16000
  attention_tail_target_tokens: 256
  default_rebase_policy: economic_at_legal_boundary
  forecast_saving_margin: 0.15
  minimum_epoch_events: 8 # hysteresis, not a forced rebuild

orchestration:
  default_active_leads: 1
  optional_specialist_lanes: [architecture, verification]
  global_max_concurrent_inferences: 3
  max_parallel_consultations: 2
  max_consultation_depth: 1
  mandatory_presenter_call: false
  specialist_output_target_tokens: 700
  effect_policy: single_authorized_writer
  task_budget_source: caller_required
  model_downgrade: only_after_task_class_validation

A target is not an unconditional truncation limit. If a specialist cannot report its necessary evidence within 700
tokens, it should return a compact packet with exact artifact references or explicitly request a larger admitted
budget. If the candidate/edge limit prevents obligation coverage, return a coverage deficit and expand under a
new budget; do not pretend the selected set is complete.

Reserve enough task budget for verification and user-facing completion before starting exploration. This reserve
should depend on the requested task and model, not a universal output percentage. The admission controller
must consider already-authorized in-flight spend and provider-specific retry behavior.

Use unique event IDs and effect idempotency keys. Cancellation propagates to admitted children, and late results
cannot mutate the active task without a version check. An expansion fetch must read stored authorized bytes; it
must not create a second external side effect. A failed counting, compatibility, or authority check must not default
to unrestricted dispatch.

The request lifecycle
on event:
  append immutable authorized event; atomize useful semantic content
  update dependency versions, obligations, conflicts, and task revision
  identify unresolved next-action requirements

  if a still-valid result satisfies this demand:
      reuse it; record the certificate and avoid inference
  otherwise:
      enumerate supported lead / specialist / transition plans
      reject unsafe, stale, unbudgeted, or protocol-invalid plans
      choose a conservative plan with an explicit evidence contract
      compile required bundles and optional supporting representations
      render native request; count the entire envelope
      validate snapshot, protocol, authority, and budget again
      freeze manifest; dispatch through the broker
      retain native response and raw usage
      parse explicit work products; validate before promoting claims

before effect:
  verify task revision and expected source hashes atomically
  execute once under authority and idempotency controls
  attach receipt; invalidate affected evidence and reusable results

before user-facing completion:
  check acceptance obligations and current verification receipts
  report unresolved or unverified claims honestly

The final snapshot check must occur sufficiently close to dispatch or effect execution to prevent a
time-of-check/time-of-use gap. For code writes, compare expected file hashes under the workspace's
transaction/locking mechanism. If the source changed, recompile the affected view or retry from a new snapshot;
never silently apply a stale patch.

Make the manifest diagnostic, not expensive context
Keep the detailed manifest outside the prompt. Render only the metadata that changes the model's
interpretation: source provenance, current version, trust boundaries, explicit omission, and relevant uncertainty.
Internal UUIDs, full dependency lists, billing fields, and complete ranking decompositions usually belong in logs
rather than in every model request.

Expose a TUI diagnostic command that answers: which atoms were selected, which obligation they support,
which representation was rendered, what was omitted, which native continuation was used, where the common
prefix ended, and what the request actually cost. This should be derived from the recorded manifest, not
reconstructed from changing current state.

Security and failure behavior
Do not elevate retrieved text or another agent's prose into user or system authority. Keep instructions,
observations, and proposals distinct even when all are atoms. Quarantine malformed output, untrusted
executable snippets, and cross-scope references. Native opaque blocks also need access controls and retention
policy; opacity is not an exemption from sensitive-data handling.

Cross-session reuse must respect workspace, tenant, branch, user permission, and task scope. A shared cache
namespace is not authorization to disclose a prior session. Redacted or deleted artifacts must invalidate derived
views and expansion handles according to policy. Do not retain raw payloads indefinitely merely because
recovery is convenient.

## 13 / Prove quality and economics before promotion

Establish the right baselines
Compare five configurations: the current codeNERD path; a lean single lead with early projection and valid-result
reuse; that same lead with context epochs; sparse persistent lanes on the same compiler; and fixed fan-out as an
explicit control. Add cheaper-model variants only after the equal-tier comparisons are understood.

Hold the task set, repository snapshots, provider/model identity, reasoning configuration, tools, permissions,
budget policy, and verification criteria fixed. Record actual provider versions where exposed. Separate cold-cache,
warm-cache, and realistic idle-gap conditions. Avoid running all baseline trials before all treatment trials when
provider load or product changes may bias the comparison.

Run each task in an isolated equivalent workspace and allow the agent's trajectory to diverge. Replaying a fixed
tool sequence cannot establish performance of a policy that changes which tool the agent chooses next. Synthetic
replay remains useful for billing and invariants, but it is not a substitute for a real closed-loop evaluation.

Task classes that can falsify the design
Include short local repairs where consultation overhead should lose; multi-file refactors with strict API
compatibility; long debugging sessions with repeated false leads; tests that accidentally mock away the
production path; late user constraint changes; stale file reads; missing negative evidence; and long idle gaps.
Include codebases large enough to expose retrieval and indexing costs, but do not use repository line count as a
substitute for task difficulty.

Create hidden acceptance checks that the agent cannot satisfy merely by changing the visible test. Record
whether source inspection, actual execution, and requirement verification agree. For tasks without a decisive
automatic test, use blinded human review against an explicit rubric and label that outcome separately.

Metrics and statistical treatment
The primary economic metric is total spend divided by independently verified successful tasks. Include
unsuccessful attempts, retries, all specialists, context management, tool fees, and cache storage. Also report
success probability, median and tail latency, severity-weighted defects, repeated failed approaches, evidence
hydration rate, stale-result rejection, and admission utilization.

Use paired task-level analysis. Multiple seeds on one task are not independent task samples. Bootstrap at the task
level, retaining all seeds and variants within each sampled task. Report paired cost differences, success
discordances, and uncertainty. Do not hide a lower completion rate inside a favorable conditional-on-success
cost.

Start with a diagnostic pilot to find instrumentation and codec errors. Choose the main sample size using the
pilot's paired variance and success discordance. A small pilot cannot establish a tight non-inferiority margin. Do
not label a two-percentage-point quality bound proven merely because a few dozen tasks show no obvious
regression. Keep tuning tasks separate from the final evaluation, and predeclare any sequential stopping or
multiple-comparison correction.

Suggested promotion contract
As an initial product gate, require a credible task-level cost reduction of at least 10% for a complexity-increasing
feature, with a predeclared success non-inferiority margin no worse than two absolute percentage points, no
unacceptable increase in severe defects, and compliance with the user-facing deadline objective. These
thresholds are recommendations for decision-making, not experimentally established tolerances for Steve's
workload.

A feature that shows only a 3% uncertain accounting improvement but introduces many more calls may not justify
its maintenance burden. Conversely, a verification lane can be valuable without lowering average spend when the
explicit goal is reducing severe mistakes. Evaluate that as a quality investment rather than mislabeling it a
token-saving feature.

Do not promote if the confidence interval is too wide to support the chosen gate. Collect more evidence or keep
the feature opt-in. A non-result is useful: it may tell us the lean single-agent compiler already captures most of the
available benefit.

Required invariant and fault-injection tests
  Test family                    Failure to inject                                  Required response

  Native replay                  Missing signed part, reordered tool result,        Reject or choose a documented transition; no
                                 incompatible model switch                          silent state loss

  Final budget                   Huge scalar, expanded schema, long project         Stay inside the actual envelope or fail
                                 instruction, exhausted output reserve              explicitly

  Exactness                      Source changes after a model read but before       Refuse stale edit and rebuild affected
                                 patch application                                  evidence

  Negative evidence              Add a caller after a cached “no callers” result    Invalidate the search-scope certificate

  Provenance                     Three packets repeat the same unsupported          Preserve shared ancestry; no fabricated
                                 claim                                              independent support

  Authority                      Tool output contains new instructions or a         Keep it as untrusted data/proposal
                                 specialist requests an unauthorized effect

  Recovery                       Expand an elided result produced by a              Return retained bytes; never rerun the effect
                                 mutating tool

  Cancellation                   User revises task while two specialists are        Cancel or mark stale; apply nothing under the
                                 working                                            old revision

  Cache accounting               Cold start, TTL expiry, partial prefix, provider   Reconcile cost without counting reasoning
                                 write/read fields                                  twice

  Codec fidelity                 Crucial diagnostic is in the middle or tail of a   Retain or expose a recoverable exact slice and
                                 large payload                                      omission status

These tests should be implemented with deterministic fixtures first and then with a small, budgeted provider
sandbox suite. No live provider protocol test was executed for this report. The attached simulator's tests cover its
economic model only.

## 14 / Worked example: cancellation bug across multiple contexts

Request and first compilation
Steve asks: “Fix the parser hang, preserve the public API, and make sure normal EOF still works.” The journal
records the exact request. The graph derives three obligations: eliminate the hang, preserve the interface, and
verify EOF behavior. None is discarded because a later stack trace resembles the immediate failure more closely.

The lead receives durable constraints early and the reproduction question late, with exact current source between
them. It requests a targeted test. The returned projection includes the failing assertion, distinguishing stack
frames, command, revision, and a handle to the retained full log.

Selective consultation
The graph records that a channel receive ignores cancellation on source version H7. It retains a compact rejection
of the earlier lock-contention hypothesis, not the entire unsuccessful investigation.

The architecture lane stays dormant if the lead can repair the wait within the interface contract. If the lead
proposes a new public parameter, the scheduler admits a consultation to resolve that conflict. The specialist
receives the contract and proposal, not every debugging turn. Its finding must identify supporting evidence and
unresolved assumptions.

Edit and verification
The authorized writer checks H7 under the workspace lock before applying the patch. A mismatch invalidates the
plan. A successful edit produces H8 and invalidates affected old test receipts.

The verification lane now receives a concrete patch and obligations. It checks cancellation timing and normal EOF,
including whether the tests exercise the actual parser path. A pass must cite a real command receipt on H8. If
EOF fails, the exact contradiction returns to the lead. A separate summarizer is not required to relay one usable
evidence packet.

Cache, continuation, and the user-visible result
The task contract and stable interfaces remain in compatible prefixes. Exposed native exchanges are not resorted
mid-reasoning. A compact attention update emphasizes the current failure. A supported boundary permits a
rebuilt evidence region only when correctness or expected future value justifies the transition.

The architecture lane may never run again. Its application state survives provider-cache expiry. Verification
evidence is reusable only while its source and environment dependencies remain valid. A new user requirement
increments the task revision so a late specialist result cannot complete the wrong task.

The TUI presents one answer: what changed, what was actually verified, and what remains uncertain. Recorded
manifests explain which consultations were bought, which evidence was used, and what the requests cost. The
user does not need to reconcile three independent conversations to know whether the fix is done.

## 15 / Final decision and research limits

The most efficient approach in my judgment
Implement evidence reuse and early observation projection inside a single, lossless inference boundary first.
Build the lean single-agent compiler before adding sparse specialist lanes. Preserve native reasoning within
supported continuations, emphasize a small task-specific tail, and reorganize at economically justified legal
boundaries. Let a global scheduler choose when a specialist or stronger model is needed.

Do not make a particular provider's experimental multi-agent feature, a fixed three-model ensemble, or a custom
serving engine prerequisite. The portable foundation is explicit artifacts, dependency-valid evidence, typed
authority, native replay, final budgeting, and measured scheduling. Advanced serving-side KV reuse is a separate
optimization requiring control of the inference engine; research such as KVFlow is relevant there, not an API-level
guarantee unlocked by atom IDs. [L7]

The main conceptual change is that memory becomes a derived task state, the transcript becomes evidence, and
the prompt becomes an execution view. Multiple contexts are useful when they preserve distinct, reusable
information and independently resolve meaningful obligations. They are not inherently efficient merely because
they are specialized or cached.

What remains an empirical question
The effect of projection and ordering on real model decisions remains unmeasured here. The cost of restarting
hidden reasoning is not known. The probability of losing a decisive diagnostic, the benefit of a verified casebook,
and the accuracy of cheaper specialists all require real task trials. Provider-native state and cache behavior must
be tested against the exact model, endpoint, and account profile.

The simulations provide economic counterexamples and reproducible decision boundaries. They do not prove
that this architecture is globally optimal. The recommendation is a staged engineering choice: capture
improvements with low coordination overhead, expose all costs, preserve correctness, and make every additional
layer demonstrate incremental value.

Immediate implementation order
The next changes should be native-envelope/usage instrumentation; source codecs and retained artifacts;
dependency-valid result reuse; unified whole-request compilation; legal context epochs; and finally optional
specialist admission. This order makes each step observable and gives the multi-agent idea a strong baseline to
beat.

The first goal is not more reasoning threads. It is fewer unnecessary inferences and better information in the
inferences that remain.

## Appendix A / Ordering and decision mathematics

Expected surviving prefix for independent block changes
Let the selected, freely reorderable blocks have lengths l_i and change probabilities p_i between adjacent
requests. Assume the previous request's compatible prefixes are available, every relevant block boundary is
eligible, and the cache stops at the first changed block. The expected surviving cached length of ordering pi is:
E[cached | pi] = sum over k of:
  l_(pi[k]) * product over j<=k of (1 - p_(pi[j]))

For adjacent i,j, their suffix beyond the pair has the same survival product in either order. Ignoring the common
surviving prefix, i then j contributes:
(1-p_i)*l_i + (1-p_i)*(1-p_j)*l_j

Swapping them changes the pair contribution. Ordering i before j is preferable exactly when:
p_j*(1-p_i)*l_i >= p_i*(1-p_j)*l_j

For positive probabilities, sort by l_i*(1-p_i)/p_i descending. Zero-change blocks can go first. The
adjacent-interchange argument yields the optimum for this model, which the six-block exhaustive test confirms.

This is not a general proof for prompt ordering. Correlated source changes, restricted cache boundaries, multiple
stored historical prefixes, semantic dependencies, position-dependent quality, and opaque native bindings change
the optimization problem. In practice, apply a stability heuristic only within legal regions, then use measured cache
accounting to evaluate it.

A quality-constrained context selection objective
A useful engineering abstraction is a constrained representation-selection problem. Choose at most one primary
representation per semantic unit, except deliberate short restatements. Require obligation coverage and
dependency closure, respect exactness and trust constraints, and fit the entire outbound envelope. Score optional
bundles using expected task relevance, evidence freshness, diversity of useful support, and a penalty for
redundancy.

Cache cost is a property of the resulting ordered request, not an independent discount attached to each chosen
atom. Therefore, enumerate a small number of legal layouts and continuations, then price their actual longest
compatible prefixes. Avoid an expensive global permutation search on every event.

A practical two-stage solver first constructs a feasible required set, then greedily admits optional bundles by
marginal usefulness per incremental rendered token. A second pass compares legal orderings and native transition
modes. The heuristic is intentionally incomplete: if required coverage is infeasible, declare the deficit rather than
pretending a low-score mandatory item was optional.

## Appendix B / Reproduce the experiments

Files and commands
The companion ZIP includes the simulator, sensitivity suite, plotting script, CSV results, manifests, reference
policy, and this report's editable manuscript. It does not include font files or any secret repository data. Python
3.11 or later is sufficient for the scripts; the results in this report were generated in the execution environment
recorded by environment.json.
python -m pip install numpy pandas matplotlib
python simulate.py --suite layout --seeds 256
python simulate.py --suite routes --seeds 256
python simulate.py --suite rebase
python sensitivity.py
python plot_results.py

Commands write to the results and figures directories beside the scripts unless an explicit output directory is
supplied to the main simulator. The main script runs its internal invariant tests before the requested experiment.
Seeded numerical results should agree to ordinary floating-point tolerance in a compatible environment;
bootstrap intervals use NumPy's seeded generator.

The primary output files are layout_raw.csv, layout_summary.csv, routes_raw.csv, routes_summary.csv,
rebase_cases.csv, rebase_summary.csv, rebase_break_even.csv, and ordering.csv. Sensitivity outputs
cover cache prices, projection/horizon combinations, and lane-price/TTL/multi-domain combinations. Manifests
record seeds and run configuration. CSVs are plain data, not claims of actual provider billing.

Interpretation discipline
Keep a clear distinction between synthetic cost, provider-measured usage, and verified task outcomes. Do not
upload the synthetic result table to a performance dashboard as though it were real coding telemetry. When real
traces become available, rerun accounting on them and conduct separate closed-loop task trials. A fixed trace can
validate billing, but it cannot tell us what an agent would have done after receiving different context.

The report's figures are generated from the supplied CSVs. The random-order policy is a volatility stress test; the
ordering experiment is a mathematical cache model; neither is a simulation of model attention. The routing
experiment models abstract information demand, not autonomous software development. These restrictions
should remain attached to any exported chart.

References / Sources and provenance
Official provider documentation
[P1] OpenAI. Prompt caching. Accessed September 9, 2026. Prefix identity, current model-dependent read/write accounting,
retention, and compatibility. https://developers.openai.com/api/docs/guides/prompt-caching

[P2] OpenAI. Reasoning models. Accessed September 9, 2026. Opaque reasoning persistence, reasoning-context modes, and
model compatibility. https://developers.openai.com/api/docs/guides/reasoning

[P3] OpenAI. Compaction. Accessed September 9, 2026. Inline and standalone compaction; canonical returned windows.
https://developers.openai.com/api/docs/guides/compaction

[P4] Anthropic. Preserved thinking. Accessed September 9, 2026. Prefix/model binding and preservation constraints;
model/account-dependent enforcement. https://platform.claude.com/docs/en/build-with-claude/preserved-thinking

[P5] Anthropic. Prompt caching. Accessed September 9, 2026. Explicit/automatic cache boundaries, write/read pricing,
lifetime, and availability behavior. https://platform.claude.com/docs/en/build-with-claude/prompt-caching

[P6] Google. Thought signatures — Interactions API. Accessed September 9, 2026. Stateful and stateless signature handling.
https://ai.google.dev/gemini-api/docs/thought-signatures

[P7] Google. Thought signatures — Generate Content API. Accessed September 9, 2026. Part-level binding and function-call
sequence requirements. https://ai.google.dev/gemini-api/docs/generate-content/thought-signatures

[P8] Google. Context caching — Interactions API. Accessed September 9, 2026. Implicit caching and endpoint-specific
behavior. https://ai.google.dev/gemini-api/docs/caching

[P9] Google. Context caching — Generate Content API. Accessed September 9, 2026. Implicit and explicit context caching.
https://ai.google.dev/gemini-api/docs/generate-content/caching

Primary research
[L1] Lindenbauer et al. The Complexity Trap: Simple Observation Masking Is as Efficient as LLM Summarization for Agent
Context Management. arXiv:2508.21433v3, 2025. Evidence about observation masking and summarization on evaluated
agent workloads, not a codeNERD benchmark. https://arxiv.org/abs/2508.21433v3

[L2] Liu et al. Lost in the Middle: How Language Models Use Long Contexts. Transactions of the Association for
Computational Linguistics, 12:157–173, 2024. Position sensitivity under the paper's evaluated conditions.
https://aclanthology.org/2024.tacl-1.9/

[L3] Towards a Science of Scaling Agent Systems. arXiv:2512.08296v3, revised April 8, 2026. Task-dependent effects across
agent configurations; the revised version is the basis used here. https://arxiv.org/abs/2512.08296v3

[L4] Mixture-of-Agents Enhances Large Language Model Capabilities. arXiv:2406.04692, 2024. Multi-model output
aggregation; benchmark results do not establish coding-cost savings here. https://arxiv.org/abs/2406.04692

[L5] Rethinking Mixture-of-Agents: Is Mixing Different Large Language Models Beneficial? arXiv:2502.00674, 2025.
Self-MoA comparisons and the limitations of diversity as a proxy for quality. https://arxiv.org/abs/2502.00674

[L6] RouteMoA: Dynamic Routing without Pre-Inference Boosts Efficient Mixture-of-Agents. arXiv:2601.18130, 2026.
Selective routing as an alternative to paying for every candidate. https://arxiv.org/abs/2601.18130

[L7] KVFlow: Efficient Prefix Caching for Accelerating LLM-Based Multi-Agent Workflows. arXiv:2507.07400, 2025.
Serving-level workflow-aware cache management, not a portable hosted-API facility. https://arxiv.org/abs/2507.07400

codeNERD source snapshot
All source observations use commit 7fd2762d1e95bf7d870c919e60b5343232751f18 of theRebelliousNerd/codenerd,
checked September 9, 2026. A source read demonstrates code present at that snapshot, not successful execution or
production wiring of every path.

[R1] Snapshot and merge.
https://github.com/theRebelliousNerd/codenerd/commit/7fd2762d1e95bf7d870c919e60b5343232751f18

[R2] Shared LLM interfaces and usage representation. internal/types/interfaces.go. https://github.com/theRebellio
usNerd/codenerd/blob/7fd2762d1e95bf7d870c919e60b5343232751f18/internal/types/interfaces.go

[R3] Session tool-loop construction and history bounding. internal/session/executor_tools.go. https://github.com/
theRebelliousNerd/codenerd/blob/7fd2762d1e95bf7d870c919e60b5343232751f18/internal/session/executor_tools.go

[R4] JIT compilation, selection, assembly, and budget enforcement. internal/prompt/compiler.go. https://github.com/
theRebelliousNerd/codenerd/blob/7fd2762d1e95bf7d870c919e60b5343232751f18/internal/prompt/compiler.go

[R5] Subagent context and memory lifecycle. internal/session/subagent.go. https://github.com/theRebelliousNerd/c
odenerd/blob/7fd2762d1e95bf7d870c919e60b5343232751f18/internal/session/subagent.go

[R6] MCP result shaping, expansion handles, and retained payloads. internal/mcp/controlplane_invoke.go and
internal/mcp/handles.go. https://github.com/theRebelliousNerd/codenerd/blob/7fd2762d1e95bf7d870c919e60b534
3232751f18/internal/mcp/controlplane_invoke.go ; https://github.com/theRebelliousNerd/codenerd/blob/7fd2762d1e95bf
7d870c919e60b5343232751f18/internal/mcp/handles.go

[R7] Compilation context, clone, and cache hash. internal/prompt/context.go. https://github.com/theRebelliousNerd/
codenerd/blob/7fd2762d1e95bf7d870c919e60b5343232751f18/internal/prompt/context.go

[R8] Spawner configuration, planner client, and usage propagation. internal/session/spawner.go. https://github.com/
theRebelliousNerd/codenerd/blob/7fd2762d1e95bf7d870c919e60b5343232751f18/internal/session/spawner.go

Simulation provenance
[S1] Synthetic experiments executed for this report. simulate.py, sensitivity.py, the generated CSVs, and their JSON
manifests in the companion package. Primary seed 20260909; additional seed choices appear in the scripts/manifests. No
LLM inference or live coding benchmark was performed. The report's numerical tables and figures derive from these files.
