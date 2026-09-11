# TODO

Ordered by dependency. Items marked **[gate]** must be answered with data before
the work they gate begins.

## Now

- ~~**[gate] Calls-per-epoch histogram**~~ — **built.** `internal/broker/epoch.go`
  fingerprints each request's cacheable head (tool definitions then system
  prompt, in wire order) onto the receipt, segments receipts into epochs per
  (scope, provider, model), and reports the distribution with a per-provider
  break-even derived from published cache economics rather than hard-coded.
  Read it with `nerd meter epochs`. **The gate is now a matter of running
  sessions and looking**, not of building anything.

  **Half of Q1 turns out to be answerable from the code, and it sharpens the
  other half.** An epoch in this architecture is not a session, it is one
  turn's tool loop. The system prompt handed to a provider is
  `compileResult.Prompt` plus `withFileContext` for the current target, so it
  changes from turn to turn by construction — that is what JIT context
  management *means*, and no cache strategy will talk it out of it. What holds
  still is the system prompt inside one turn's native loop, where
  `runToolLoop` passes the same string into every round while only the message
  history grows, and messages are deliberately outside the fingerprint.

  So the ceiling on epoch length is the tool-loop round count, capped by
  `budget.iterationLimit` — and the Piggyback structured-output path runs a
  single iteration by design, so epochs are 1 there whatever else is true.
  That makes the aggregate median ambiguous in a way that decides Phase 4 the
  wrong way round: a p50 of 1 means either turns that used no tools, where
  there is no loop to lengthen, or a tool loop that is not reusing its prefix,
  which is exactly what Phase 4 fixes. `nerd meter epochs` now splits by call
  shape for that reason. **Read the by-shape table, not the headline.**

  It also names the Phase 4 design input. If the prefix is going to move every
  turn anyway, the lever is not a better rebuild controller but request
  *ordering*: a stable skeleton ahead of the volatile JIT selection ahead of
  the file context, so the cacheable head is the part that does not move. That
  is a change to how the prompt is assembled, not to how it is cached, and it
  should be settled before any controller is built on top of it.

  **And caching is already switched on for one client and off for the other,
  by omission.** `EnableSystemCaching()` — which wraps the system prompt in a
  `cache_control: {"type": "ephemeral"}` block — is called in exactly one place
  in the repository: the Anthropic CLASSIFICATION client in
  `client_factory.go`. The main Anthropic client, the one carrying the large
  JIT-compiled system prompt this whole phase is about, never calls it.

  Nobody wrote that decision down, which is why it is here rather than in a
  commit. Reading the code, the default is off for the agent and on for the
  labeller, and the two sites are four hundred lines apart.

  The arithmetic is already in this repository. `cacheEconomicsByProvider` puts
  Anthropic at write 1.25x, read 0.10x, TTL 5 minutes, and `BreakEvenCalls()`
  derives `(1.25 - 0.10) / (1 - 0.10) = 1.28` calls. So caching pays from the
  SECOND call onward within the window, and costs 25% of the system prompt's
  input tokens on a turn that makes only one.

  That maps onto the by-shape split above rather than onto a global on/off:

    native tool loop   the same system prompt goes into every round while only
                       the messages grow, so a turn with N rounds pays
                       1.25 + 0.1(N-1) instead of N. Four rounds: 1.55 against
                       4.00, a 61% reduction on the system prompt's input cost.
    Piggyback          one iteration by design. Epoch length is 1 whatever else
                       is true, so caching is a flat 25% loss there.

  The honest conclusion is that this is not a global setting and should not be
  made one. It is a per-call-shape decision, and the shape is known at the call
  site and not at client construction, which is where `EnableSystemCaching` is
  set today. **What Gate A needs from the epoch histogram is not "should we
  cache" but the round-count distribution of the native tool loop**, and a way
  to set the flag per request rather than per client.

  A TTL note that the by-shape table above already makes and that applies
  doubly here: Anthropic holds an entry ~5 minutes, and a single high-reasoning
  call can run 2-5 of them. An epoch whose rounds are slow enough can lose the
  entry between calls and pay the write premium for nothing, which is what the
  EXPIRED column in `nerd meter epochs` is for.
- ~~**[gate] Atom co-use analysis**~~ — **built.** `internal/prompt/couse.go`
  records which atoms are selected together per compilation, settled against the
  turn's outcome, and reports lift, Jaccard, clusters and category alignment.
  Read it with `nerd meter atoms`. Same status: the gate needs data, not code.
- **`ActivatedFacts` — do not populate it yet.** The earlier note here claimed
  this was "a one-line change with no trap behind it". That was wrong, and the
  two real blockers are now recorded on the field itself:

  1. **There is no relation between facts and atoms.** Every selector dimension
     on `PromptAtom` is a context dimension — mode, phase, verb, shard,
     language, framework, model, provider, world state. None is fact-shaped.
     "Boost atoms related to hot facts" needs a definition of *related* that
     does not exist in the corpus schema. Populating the map hands the selector
     data it has no rule to act on.
  2. **It is in `Hash()`, and `Hash()` is the prompt cache key.** Activation
     scores move every turn, so live scores give every compilation a unique key
     and switch the prompt cache off — every turn becomes a full compile.
     `TestActivatedFactsWouldDefeatThePromptCache` pins this, and demonstrates
     the shape of the fix (quantize to coarse buckets, or carry the set of hot
     facts rather than exact scores).

  `Compressor.GetActivationScores()` is likewise called by nothing. The producer
  and the consumer both exist and neither is connected, because the thing
  between them was never designed.
- ~~Reconciliation alarm~~ — **done.** `internal/broker/reconcile.go` accumulates
  per-model estimate-vs-billed drift and warns once per model past 20 samples
  and 10% mean absolute error. Mean absolute error is the headline rather than
  net bias, so an estimator wrong by 30% on every call is not scored as perfect
  because its errors happened to balance.
- ~~Runtime sentinel-client test~~ — **done.**
  `internal/perception/broker_sentinel_test.go` builds a client through the real
  exported constructor for all ten providers and both CLI engines and asserts
  each is metered, then drives a refusal end to end to prove a receipt is
  emitted — no API key, no network, no fixture server.

## Next — Phase 1, observation codecs — **complete**

All three codecs are built, wired into every live producer, and redeemable
through a registered read-only verb. What follows records each one and, where
it matters more than the byte count, the boundary it had to hold.

- ~~Tail-aware test-output codec~~ — **withdrawn.** The claim behind it was
  wrong: `ClampText` was already head+tail. The genuine narrower defect
  (byte-offset cuts landing mid-line) is fixed on this branch. A structural
  test-output codec is still worth building, but as structure, not as rescue.
- ~~Code-search codec: symbols and dependency edges, not matching lines.~~ —
  **built.** `internal/observation` composes `internal/retain` with a
  code-search projection: each match resolves to the innermost element
  containing it, and only dependency-bearing edges are kept (`references`,
  `imports`). Wired into the two live producers — the `search_code` tool and
  the VirtualStore action — plus a `search_expand` verb to redeem a handle,
  registered through the tool registry, the effect table, `safe_action` and
  `coreTools`, because a handle the model cannot redeem is a promise it is
  structurally unable to keep.

  **Hydration cannot re-run, structurally rather than by discipline.**
  `CodeSearch` holds a `*retain.Store` and nothing else; the source reader is
  an argument to `Encode`, so `Hydrate` has no reader to reach for. A
  reflection test pins the field list.

  Measured on this repo, codec vs raw grep output: `logging.Tools` 2374 vs
  9246 bytes, `ActionResult` 3653 vs 9985, `WorkspaceRoot` 4428 vs 8687,
  `ClampText` 1653 vs 2957, and the honest worst case `retain.` at 2094 vs
  2019 — a wash on a search with one hit per symbol, while still carrying more
  structure. The first rendering was *larger* than grep on exactly that shape,
  which is why `TestResultText_ShouldNameEachSymbolOnce` exists.
- ~~File-read codec: exact source around the edit plus a precondition hash.~~ —
  **built.** A read made in order to edit needs the exact region the edit
  concerns and enough context around it to be sound, where "enough" is the
  enclosing code element rather than an arbitrary window: a function cut in half
  reads as a whole one. The rest of the file is carried as an outline of what is
  in it and where.

  Measured on this repo: `internal/session/executor.go` is 106933 bytes; read
  whole through the codec it costs 21281, and reading twenty lines of it costs
  5695.

  The elision is the smaller half. A read also mints a PRECONDITION, and the
  edit that follows can be refused when the lines it rests on have changed
  since. A stale read is worse than no read — the model has been given every
  reason to believe it understands a file it no longer does, and will edit
  confidently against a version that is gone.

  The same read/write boundary holds and for the same reason. Checking a
  precondition is inherently a comparison against the live file, so the live
  bytes are an argument the edit verb passes in — the very buffer it is about to
  modify. A store that opened the file itself would compare two instants and
  prove nothing about the third one the edit lands on.
- ~~Subagent-return codec: findings, evidence refs, changed artifacts,
  verification status, remaining uncertainty — not the transcript.~~ —
  **built.** `internal/observation/subagent.go` projects a delegation's return
  into what the parent decides on, and retains the transcript under a handle.
  Wired into the three places a subagent's output reaches another model's
  reasoning: the `delegate` VirtualStore action (`handleDelegate`, output and
  the `delegation_result` fact), the campaign orchestrator's `CONTEXT FROM
  TASK` injection (`completeTask` → `projectTaskReturn` → `storeTaskResult`),
  and the chat blackboard's cross-shard handoff (`priorShardContext`, which
  replaces `truncateForTask(RawOutput, 500)`). The three human-facing surfaces
  — `formatDelegatedResponse`, `formatInterpretedResult`, the `nerd spawn`
  result — still get the prose, deliberately: the interpretation call has no
  tool catalog, so a handle in that prompt would be unredeemable.

  **Two of the five come only from structure, and the codec refuses to read
  them out of prose.** The parent ACTS on "what changed" and "what was
  verified", so a wrong answer is worse than none. `ExecutionResult` already
  computes `WrittenPaths`, `BuildCheck`, `TestCheck`, `UntestedPaths` and
  `CriticFindings` on every turn and `SubAgent.execute` discarded all of it at
  the boundary — the same reader/writer/no-wire shape as everything else on
  this branch. `ObservedTaskExecutor` / `ObservedTaskDelegator` carry it
  through to `handleDelegate`. The first version did fall back to
  `testoutput.Parse` over the whole return, and a reviewer writing "the error
  from Flush is discarded" scored three test failures on a shard that never ran
  a test; `TestProjectReturn_ShouldNotReadAVerificationVerdictOutOfProse`
  exists because of it, and a producer that genuinely knows its output is a
  test log calls `observation.ReportedTests` itself.

  **Hydration cannot re-delegate, structurally rather than by discipline.**
  `Subagents` holds a `*retain.Store` and nothing else, and here that matters
  more than next door: re-running a search answers from a moved world, while
  re-running a subagent writes files and spends tokens. The redemption verb is
  `subagent_expand` — a read-only verb of its own rather than an argument on
  `delegate`, because a depth or budget cap that denies further delegation must
  not also take away the transcript of the delegation that already happened.
  Registered through the tool registry, the effect table, `safe_action`,
  `modular_tool_allowed` and `coreTools`.

  Measured over the 67 real agent outputs in `.quality_assurance/`: 69892 bytes
  projected against 2313106 raw, 3.0% in total; the largest is 892 against
  93643. The honest worst case is the opposite shape — a return with nothing to
  elide. A five-byte "Done." costs 72 bytes and a 504-byte return costs 571, a
  fixed 67-byte header either way; nothing is elided below `minRetainBytes` and
  no handle is minted, so the loss is bounded rather than compounded.
  `TestProjectReturn_AtTheElideThreshold_ShouldCrossOverInTheRightDirection`
  pins the crossover in both directions.

  A third of those 67 returns carry no severity marker and no `file:line`
  citation anywhere — long structured prose — and projected to a status line
  and a handle until the projection grew a section outline, which is the same
  move the file-read codec makes for the part of a file it does not print.
- ~~Generalize the MCP elision/handle mechanism rather than building a second
  one~~ — **done.** Retention now lives in `internal/retain` and MCP composes
  it with its JSON projection. The seam is retention versus projection:
  retaining bytes under a content-addressed id, with TTL / byte / entry
  ceilings and an eviction hook, is not MCP-specific, while shaping a JSON
  payload through a pointer and a view is. Each codec below brings its own
  projection and reuses the retention.
- **Hydration must read the retained artifact, never re-run the tool.** Re-running
  gives a different answer from the one the reasoning was built on.

## Adjacent — what the prompt is built from

Not broker work, but found while answering "are we spending tokens on the right
things", and the same defect shape as everything else on this branch: a reader,
a writer, and no wire between them.

- ~~The interactive turn compiled with no language.~~ — **fixed.** 326 of the
  corpus's 918 atom entries declare a language (113 Mangle, 72 Go, 27 Python, 24
  Rust, 22 Java, 21 TypeScript, and every TDD, debugging and refactoring
  methodology file). `matchSelector` fails closed, so an empty language selects
  none of them rather than all of them — which is the correct rule, and is what
  stops Go advice leaking into a Python session.

  `CompilationContext.Language` was set in exactly one place in the repository:
  the `nerd init` scan. `WithLanguage` has twenty-six callers, all tests. The
  interactive path had one other source — a language inferred from the intent
  target's file extension — so turns naming a file were fine and turns that did
  not ("add tests for the compressor", "refactor the orchestrator") had no
  language at all.

  The workspace already knew: the world scan asserts `project_language`, and
  `nerd init` writes it into `.nerd/profile.mg`, which chat loads at boot.
  Nothing downstream had ever read it back.

  Precedence is now explicit rather than emergent: the file the turn is about
  beats the project's dominant language. That ordering had already inverted
  once — the target inference was guarded on "language not already set", which
  was the same condition as "always" for as long as nothing set one earlier, so
  filling the project language silently switched the more precise source off.

- ~~The framework dimension contributed in neither direction.~~ — **fixed.**
  Frameworks are the mirror image and the asymmetry is deliberate: the check is
  skipped entirely when the context names none, so all 42 framework-gated atoms
  stay eligible in every session, django and react included. They compete rather
  than being included outright, so the cost is not a fixed number of wasted
  tokens; the real loss is that a project built on bubbletea and cobra had
  nothing favouring the bubbletea and cobra atoms. `project_framework` was
  already written by `nerd init`, and `internal/init`'s own comment says the
  fact exists "to build the /framework JIT" selector.

- **`build_layer` is dark, and not for want of a wire.** 18 atoms gate on it and
  `CompilationContext.BuildLayer` is populated from nothing but `ExtraContext`,
  which no caller fills. Unlike language and framework there is no fact to read:
  the six layers (`/scaffold`, `/domain_core`, `/data_layer`, `/service`,
  `/transport`, `/integration`) would have to be *derived* from what the turn is
  touching. That is a classifier nobody has written, not a connection nobody
  made, and it should be costed as a feature.

- **The two selectors disagree about language, and both are documented as
  correct.** Worth knowing before anyone tunes atom selection, because it means
  the primary and fallback paths do not select the same set.

  `jit_compiler.mg` splits dimensions in two. The REGIME dimensions — `/shard`,
  `/mode`, `/phase`, `/layer`, the three wizard steps, `/provider`, `/model` —
  are fail-closed: "the honest answer for a compile that never set the dimension
  is *not that one*". Everything else is situational and permissive, and its
  comment names language explicitly: *"no language in context should not
  suppress an atom that happens to mention Go"*.

  `matchSelector` in Go, which is the fallback path when Mangle is unavailable,
  is fail-closed for every dimension including language.

  So with no language in context the kernel admits all 326 language-gated atoms
  — Go, Python, Rust, Java and TypeScript advice at once, competing for the same
  budget — and the fallback admits none of them. That is the same
  contradictory-identity failure `internal/session/executor.go` documents for
  shards, where a custom agent with no shard type "was handed 25+ contradictory
  built-in identities and answered as whichever it latched onto".

  **It is not one dimension, it is the default stance.** The two selectors
  disagree about everything situational, and agree in exactly one place:

  | dimension | entries | Mangle | Go `matchSelector` |
  |---|---|---|---|
  | `languages` | 326 | permissive | fail-closed |
  | `intent_verbs` | 195 | permissive | fail-closed |
  | `world_states` | 21 | permissive | fail-closed |
  | `frameworks` | 42 | permissive | permissive |
  | the 9 regime dimensions | — | fail-closed | fail-closed |

  Go is fail-closed by default with one hand-made exception, frameworks, whose
  block is skipped entirely when the context names none. Mangle is permissive by
  default with a declared list of exceptions, `regime_dimension`. Two opposite
  defaults that happen to meet on the regime list and on the one case somebody
  special-cased by hand.

  **The fix above resolves the language row by making the disagreement
  unreachable rather than by picking a winner**: a scanned workspace now always
  supplies a language, so neither semantics applies. That is the right shape —
  choosing a winner means either changing kernel policy or making the fallback
  permissive, and both are prompt-quality decisions that want an eval.

  The other two rows are open in principle and are not live gaps, which is
  worth saying precisely because the language row was. `buildCompilationContext`
  sets `IntentVerb` from the intent on the executor path, `toCompilationContext`
  sets it from `UserIntent` on the articulation path, and an empty verb is
  defaulted to `/general` in three separate places — so `intent_verbs` is
  normally supplied. World states are computed per turn from kernel facts and
  are legitimately absent when nothing is wrong, which is the case the
  permissive default was written for.

  So the thing to fix was language, and it is fixed. What remains is that the
  two defaults are still opposite, and any dimension anyone leaves empty in
  future inherits the disagreement rather than a decision.

  A kernel outage therefore does not merely degrade selection, it inverts it for
  542 of the corpus's 918 entries. Whatever is decided about the defaults, that
  is worth a line in the fallback's own doc, because "the fallback selects
  differently" is a much smaller claim than what actually happens.

- **The tag namespaces are fine.** Atoms emit `atom_tag(ID, /lang, /go)` while
  the context writes `current_context(/lang, /go)` through an explicit long/short
  mapping (`add("language", "lang", ...)`, `add("build_layer", "layer", ...)`).
  Checked because a mismatch there would have made both fixes above useless on
  the Mangle path; it is correct. The one real duplication is `/shard` versus
  `/shard_type`, which the selector emits both of and the policy reads both of,
  already carrying a comment saying so.

## Then — Phase 2, the typed graph

- Declare `supports`, `contradicts`, `supersedes`, `depends_on`, `satisfies`,
  `invalidates`.
- Obligation-driven selection rules extending `context_compilation.mg`.
- Result reuse keyed on dependency-version validity — this skips whole
  inferences, which is worth more than every prefix optimization combined.
- `independent_support` as a derived predicate: count distinct evidence roots,
  not distinct agents.
- **Carry the activation caps forward.** The current caps and the 105 threshold
  are an injection defence; a rewrite could quietly lose it.

## Then — Phase 3, provider fidelity

- ~~`types.Message` must carry ordered native content blocks, signatures, ids and
  continuation references losslessly, across all seven adapters.~~ — **built,
  and the loop that feeds them no longer flattens.**

  `types.ContentBlock` is the ordered content — text, thinking with its
  signature, tool_use with its id, tool_result with the id it answers — and it
  lives in an UNEXPORTED field on `types.Message`. That is the whole of the
  design decision. `Text`, `ToolCalls` and `ToolResults` stay as a flat
  projection filled once by the constructors, so the several dozen callers that
  read them keep working, and because the block list cannot be set by a struct
  literal the two views cannot be given contradictory values. `Content()` is
  the single read path: it returns the blocks when a constructor built them and
  otherwise lifts the flat fields in the fixed order tool_result → text →
  tool_use, which is exactly what every adapter emitted by hand before, so no
  legacy turn changes shape on any wire.

  Adapter fidelity is not uniform and the code now says where it stops.
  Anthropic is total (thinking and redacted_thinking in their own wire shapes,
  signatures verbatim, ids paired). The OpenAI Responses surface used by Meta
  is total in order and reasoning; replayed encrypted_content now rides on the
  message and the per-turn side cache is the fallback for legacy turns rather
  than the only source. Gemini carries order and per-part thought signatures
  but has **no tool ids on the wire at all** — a functionResponse pairs by tool
  NAME and the "call_N" ids are minted from position. Every Chat Completions
  surface (OpenAI, xAI, xAI-OAuth, OpenRouter, ZAI, Ollama, DashScope,
  Moonshot) keeps ids and pairing and **cannot** keep either interleaving or
  reasoning: an assistant turn has one content string and there is no
  request-side field for a signature. Gemini's Piggyback path and the two CLI
  engines carry no typed blocks at all, by construction.

  **That paragraph is now a table the tests hold to.**
  `internal/perception/provider_fidelity.go` declares `BlockFidelity`, keyed by
  request FORMAT rather than by vendor — OpenAI ships two surfaces with
  different fidelity, which is the distinction a vendor-keyed table loses. Three
  booleans, all about the REQUEST direction, because reading a response is the
  easy half and the expensive mistakes are all on the way back in, where a turn
  the model never produced is handed to it as its own history: `KeepsOrder`,
  `ReplaysReasoning`, `CarriesToolIDs`.

  Three comments already pointed at this table before it existed — in
  `internal/types`, in `client_tool_helpers.go` and in `xaioauth` — which is the
  same reader-with-no-writer shape as everything else on the branch. Prose is
  where a limit goes to become a claim.

  `provider_fidelity_test.go` runs the four real mappers over one interleaved
  turn and holds each to its declaration in BOTH directions. A mapper that
  quietly GAINS a capability fails as loudly as one that loses it: a gain means
  either the table is wrong and callers are declining to pay for reasoning they
  could replay, or the mapper is now sending a field the endpoint rejects. Two
  invariants ride alongside — no surface may drop a tool result, and a surface
  that cannot replay reasoning must DROP it rather than fold it into the prose,
  where it would corrupt every structured-output parse downstream.

  `xaioauth` is probed from an external test package, which is the only seam
  that reaches it: the mapper is unexported and lives in a package
  `internal/perception` imports, so `xaioauth_test` importing
  `internal/perception` is the one direction that does not cycle. It is a
  hand-copied Chat Completions clone, and hand-copied clones drift.

  `ReplaysReasoning` is the axis with a bill on it. Where it is false, reasoning
  is generated fresh every turn of a tool loop and thrown away every turn — the
  provider bills for it and has no request-side field to take it back. That is a
  Phase 4 input: a break-even that ignores it will over-estimate what a Chat
  Completions surface is worth caching.

  **The last flattening call site is converted.**
  `internal/session/executor_tools.go` built each assistant turn as
  `types.Message{Role: "assistant", Text: ..., ToolCalls: ...}` — the literal
  that drops the order and the signature — at three sites, all now
  `types.AssistantMessageFrom`. Two things had to move with it, and neither was
  the one this line predicted.

  **`LLMToolResponse` is the one settable-both-ways type in the design**, and
  three post-parse edits were writing only the flat half. Piggyback promotion
  is the sharp one: it reads a control envelope out of the prose and turns it
  into tool calls, writing `Text` and `ToolCalls` and leaving the blocks
  holding the original envelope with no tool_use. `AssistantMessageFrom`
  prefers blocks, so a block-carrying response would have gone into history as
  an assistant turn claiming it called nothing, followed by a user turn
  answering calls that are not in the transcript. The two `ToolCalls = nil`
  sites were the same half-change in reverse. All three now go through
  `Rewrite` / `ClearToolCalls`, which move both views and keep thinking blocks
  in front, since their signatures cannot be regenerated.

  **The transcript bound had to become block-aware.** `boundToolLoopHistory`
  shrinks old tool results by assigning `ToolResults`, which on a block-built
  message changes only the projection: `Content()` still returns the full
  payload and the bound silently stops holding. The tool-RESULT turns are still
  flat literals, so this was latent — and it stopped being safe to leave latent
  the moment assistant turns became block-built, because the next person to
  convert the user turns closes the loop. `Message.WithToolResults` rewrites
  both views and both bounding paths use it.
- Provider profile: supported continuation modes, compaction, reminder
  placement, **and cache economics** (write penalty ÷ read discount), so Phase 4's
  break-even is derived per provider rather than hard-coded.

  **The cache-economics half of this is done**, and the roadmap did not say so.
  `internal/broker/epoch.go` carries `CacheEconomics` per provider — write
  multiplier, read multiplier — and `BreakEvenCalls()` derives the threshold as
  `(write−read)/(1−read)`, which is a call count independent of prefix size and
  `+Inf` where reads are not discounted. An unknown provider gets `noCaching`
  rather than a plausible-looking guess. So Phase 4's break-even is already
  derived per provider, which is exactly what this line asked for.

  It also carries a **TTL** per provider, which this line did not ask for and
  which turned out to matter more than expected: an epoch whose wall-clock span
  outruns the provider's cache window was evicted between calls, so it looks
  long enough to pay and cannot. Anthropic and OpenAI hold an entry ~5 minutes,
  Gemini and DeepSeek an hour — and a single high-reasoning call can run 2-5
  minutes, which eats most of the shorter window. `nerd meter epochs` scores
  those epochs separately in its EXPIRED column. Counting calls without
  measuring time is how a cache strategy gets approved on paper and loses money
  in production.

  Still missing from the profile: continuation modes, compaction, reminder
  placement. Those are Phase 4 inputs but not break-even inputs.

- **The assembled order is measured, and it is inverted for two of three
  sections.** This line already names the lever — "a stable skeleton ahead of
  the volatile JIT selection ahead of the file context" — so here is what the
  code does today, so Phase 4 starts from fact rather than from the sentence.

  `internal/session/executor.go` builds the system prompt as:

      compileResult.Prompt            JIT-selected atoms, varies every turn
    + projectDoc.PromptSection()      stable for the life of the project
    + fileContext.PromptSection(t)    per-file, the most volatile part

  Both helpers append (`systemPrompt + "\n\n" + section`), so the ordering is
  volatile, stable, most-volatile. A prefix cache matches a prefix: everything
  after the first differing byte is uncacheable, so the project doc — identical
  on every turn in a project, and the one section that could anchor a prefix —
  is stranded behind the JIT selection and can never be part of one. The file
  context being last is already right.

  **Deliberately not reordered here.** Where an instruction sits in a prompt
  changes how strongly a model follows it, and moving the project's own
  instructions ahead of the JIT selection is a change to behaviour, not a
  change to encoding. It wants an eval, not a commit. The measurement is the
  contribution; the decision is Phase 4's.

  The same applies inside `compileResult.Prompt`: the assembler's category
  order decides how much of the prompt is stable-prefix, and identity, protocol
  and safety are the categories that hold still across turns. Whether they lead
  today has not been measured and should be, in the same pass.

## Later — gated

- Phase 4 economic rebasing. Preconditions: Gate A passed, Phase 3 landed,
  latency a term in the objective, hysteresis, correctness interlock, manual
  override.

  The preconditions have moved and the list should say so. Gate A now needs
  sessions run rather than code written; the break-even half of Phase 3 landed
  earlier; and the ordered native content blocks have now landed too, with six
  of seven adapter families converted and the fidelity limits of the seventh
  written down rather than discovered later.

  So what is left blocking Phase 4 is no longer a missing representation. It is
  two things, one small and one not:

  1. **One flattening point still in the loop.** `executor_tools.go` rebuilds
     each assistant turn from `Text` and `ToolCalls`, discarding the ordered
     blocks before any adapter sees them. The adapters are lossless and the
     loop feeding them is not, so none of Phase 3's fidelity currently reaches
     a provider. `types.AssistantMessageFrom` is the replacement. It is not a
     three-line swap: the final-completion site deliberately snapshots the
     tool calls before the response clears them, and the history eviction
     blanks tool results through the flat fields — which changes only the
     projection on a block-built message, so eviction has to move with it or
     silently stop bounding the transcript.

  2. **The design question Q1 surfaced, unchanged: the prefix moves every turn
     by construction**, because the system prompt is the JIT compilation plus
     the current target's file context. A rebuild controller built on a head
     that never holds still is optimising the wrong layer. Request ORDERING —
     a stable skeleton ahead of the volatile selection ahead of the file
     context — has to be settled first, and the measurement of what the code
     does today is recorded under Phase 3 above. That is a change to how the
     prompt is assembled rather than to how it is cached, and it wants an eval
     because where an instruction sits changes how strongly it is followed.

  On "latency a term in the objective": worth being precise, because it is easy
  to read as an efficiency goal and it is not one. Latency is recorded on every
  receipt and priced into no decision. The single place wall-clock enters is
  cache TTL, above, which is a token question wearing a clock.
- Phase 5 lanes. Preconditions: Gate A passed, Phase 2 landed, single-writer or
  worktree isolation for concurrent mutation, one global admission controller.

## Housekeeping

- ~~Raise broker coverage from 76.5%~~ — **done: 94.1%.** The gaps that mattered
  were not the arithmetic. `passthrough.go` — the capability-preservation
  surface the eight-shape design exists to protect — was at 0%, and every
  forwarder is now checked in both directions, because the whole design rests on
  a forwarder producing exactly what the unwrapped client would have. The eight
  shapes were tested for what they *advertise* and never for whether the
  advertised method works; a shape delegating to the wrong core method would
  pass the capability matrix and fail at runtime looking like a provider
  problem. `Configure` is now exercised for real rather than through a replica
  of its logic, including the cap-change path that must carry accumulated spend
  across a ledger rebuild.
- ~~Speculative broker surface~~ — **removed.** `Meter.Calibrator`,
  `Meter.Receipts`, `Meter.Reconciler`, `Meter.Drift`, `Meter.PrimaryModel` and
  `ReceiptFunc` had no production caller: the readout reads the workspace log,
  because a `nerd meter` invocation is a different process from the agent that
  spent the tokens, so the in-process getters had nothing to serve. The repo's
  own `scripts/deadcode-budget.sh` is what surfaced them.
- ~~Configure a compression purpose budget~~ — **moot; see Q6.** The compressor
  makes no LLM calls: `Compressor.generateSummary` is its only call site and is
  dead, replaced by kernel-driven observation masking. Compression costs zero
  tokens today, so `PurposeCompression` is on the exemption list with that
  reason and the test fails if it starts being tagged again without the
  exemption being removed.
- ~~Surface `Meter.Receipts()` through a `nerd` subcommand~~ — **done.**
  `nerd meter`, `nerd meter epochs`, `nerd meter atoms`, each with `--json`.
  Both measurement streams persist to rotating JSONL logs under `.nerd/meter/`
  via `internal/jsonl`, because every question they answer spans processes and
  the readout is itself a different process from the agent that spent the
  tokens.
