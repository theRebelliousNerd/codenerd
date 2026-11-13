## GAP-CORE-CATALOG-QUERY-PARSER-DEADLINE

Context-owned pattern queries must propagate cancellation while waiting for the
process-wide parser before acquiring kernel read ownership or evaluating a
catalog. They retain the same exact term, arity, readiness and external-query
semantics as ordinary Query. Cortex owner/replica/fan-out and cross-shard router
reads must forward that same owner context, retain routing/metrics/deduplication,
and join every fan-out worker before returning. Cancellation must return its cause, never degrade
into a predicate-only unfiltered query. Existing ordinary malformed-pattern
fallback stays compatible. Kernel mutex acquisition, fixpoint evaluation and
external callbacks remain synchronous; this is not a hard deadline for every
kernel operation. Exit: a real spawner catalog query stops while another reader
holds the parser; temporary facts and reservations are reclaimed, a healthy
follow-up succeeds, and ordinary/literal-type query controls remain green.

# core internals

Verified 2026-09-21 against commit `3463477` (`main`). All line references
are to `internal/core/kernel_facts.go`, `kernel_eval.go`, `kernel_query.go`,
`kernel_validation.go`, `kernel_provenance.go`, or `kernel_sysfacts.go`.

## Fact lifecycle (kernel_facts.go)

1. `Assert` (`kernel_facts.go:515-562`) validates each fact with `ValidateFact`
   (`kernel_facts.go:186-278`) before anything is stored; invalid facts are
   rejected at the door, not filtered later.
2. Numbers are scrubbed by `sanitizeFactForNumericPredicates`
   (`kernel_facts.go:1161-1186`): every numeric slot is int64, so NaN and
   infinities are removed before they can reach the pinned fork, which
   compares int64 only.
3. `canonFact` (`kernel_facts.go:155-167`) builds the canonical key and
   `addFactIfNewLockedErr` (`kernel_facts.go:442-486`) inserts only if the key
   is absent — asserting the same fact twice stores it once.
4. `Retract` (`kernel_facts.go:570-625`) removes by the same canonical keying.
   Facts live in a `newFactStore` map (`kernel_facts.go:89-113`) of `Fact`
   values (`kernel_facts.go:28-38`).

## Typed admission and the /name tolerance (kernel_fact_decl.go)

As built 2026-10-28.

- Every asserted fact is admitted against its Decl's bound alternatives by
  `predAdmission.check` (`kernel_fact_decl.go:275`). An alternative that
  accepts every argument exactly wins over one that needs the bare-string
  /name tolerance (`bareStringInNameSlot`, `kernel_fact_decl.go:329`), so
  `compile_context(/dim, "value")` is admitted by its `[/name, /string]`
  alternative without a defect report. When only the tolerance admits a fact,
  every tolerated argument is reported once per process (`noteLenientName`,
  `kernel_fact_decl.go:347`), not just the last one.
- The tolerance report is a producer defect: the value lands as a string no
  /name rule matches. `TestFactNameSlotProducersPassNames`
  (`internal/core/fact_name_slot_sweep_test.go`) type-checks the whole module
  and finds every `types.Fact` / `prompt.Fact` literal passing a string-typed
  value into a slot the production corpus declares /name only. It is a ratchet
  over `nameSlotStringBaseline` (per file, predicate and argument): a new site
  fails, and a fixed site must lower its count.
- Producers fixed from the 2026-10-28 dogfood log: `tool_capability`
  (`tool_registry.go` collectToolFacts, `internal/autopoiesis` via
  `types.Atom`), `atom_category` (`internal/prompt/selector_facts.go`),
  `file_topology` Language (`internal/shards/system/world_model.go`,
  `cmd/nerd/chat/helpers.go`), and `shard_executed` / `last_shard_execution` /
  `recent_shard_context` / `active_shard` ShardType (`shards.shardTypeName`,
  `internal/core/shards/manager_spawn.go:137`). `shard_profile` is emitted at
  its declared arity 3 with the profile kind (`/specialist` for a persistent
  shard; `shards.profileKind`, `internal/core/shards/manager.go:415`). The chat
  host asserts the declared `self_correction(Hypothesis, At)`
  (`cmd/nerd/chat/process.go:1229`) instead of the undeclared
  `self_correction_hypothesis/1`.

## Review contract boundaries

Names remain names throughout the data-flow judgments: scanner variable and
function identities flow through `is_guarded`, `unsafe_deref`,
`is_error_checked`, `unchecked_error`, `active_hypothesis`, and
`prioritized_hypothesis` in `defaults/schemas_reviewer.mg`. System shard
registry, heartbeat, startup, and unhandled-case identities share `/name`.
`current_intent/1` uses the same name identity as `user_intent/5`.

Memory values, delegation descriptions, tool lifecycle labels, compilation
error details, and escalation subjects are strings. Rules serialize a name
with `fn:name:to_string` when crossing into those content slots. Learned
context renders both the predicate and its arguments as text and joins the
current intent's verb. Agenda priority is numeric; campaign priority retains
its separate named ordering. Intelligence safety actions and coder rejection
patterns retain their producers' string contracts.

Activation identities admit exactly strings and names through separate whole
tuple alternatives in `defaults/schemas_memory.mg`:
`bound [/string, /number] bound [/name, /number]`. Assert-time admission in
`kernel_fact_decl.go` accepts a fact only when one alternative accepts every
argument. The parsed `fn:Union(/string, /name)` bound passes strict rule analysis
on the pinned engine but its runtime type handle rejects both primitive
identities. Static bounds analysis and interpreter derivation therefore cannot
replace tests that assert through the production kernel. Each alternative keeps
the score numeric; neither admits a numeric identity or a textual score.
File, prompt and trace
references remain strings; tools retain the name identity supplied by
`tool_capabilities/2` and `tool_language/2`. Both goal-driven and
learned-preference boosts preserve that identity. Serializing the tool in an
activation rule changes the relation's key and prevents name-bound queries
from finding the boost. `policy/activation.mg` and `policy/knowledge.mg` render
identities with `fn:string:concat` only when deriving the string-valued
`context_atom/1` and `context_priority/2` projections. Cold-storage preference
keys and values remain strings through `VirtualStore.HydrateLearnings`.
`VirtualStore.QueryActivations` and campaign phase compression emit explicit
`MangleString` references so absolute paths retain string identity even though
activation also admits names. Hydration asserts these typed facts directly.
`TestHydratedLanguagePreferencePreservesToolActivationIdentity` exercises
storage, hydration, activation and context inclusion through the real kernel;
`TestActivationAdmitsNamesAndStringsAndRendersContext` covers both identity
variants and rejects numeric identities. Their execution remains pending.
`TestCompressPhasePreservesStringActivationIdentity` covers the campaign
decay producer through the real kernel; its execution is also pending.

Prompt conflict ties use `atom_ordinal/2`, measured once across all candidates
by `internal/prompt/selector_facts.go` and supplied by `selector.go` before
selection queries. Mangle compares these integer ordinals to choose the
lexically first candidate when scores tie. The ordinal is owned by the prompt
shard in `internal/shards/registration.go` and cleared with the other compile
inputs in `internal/prompt/compiler.go`. Candidate input order does not change
the result; skeleton and flesh share one ordinal space.

`VirtualStore.parseBuildDiagnostics` emits the declared five-field diagnostic
with an integer line, severity name, and separate error-code and message
strings. Workflow clarification emits `campaign_awaiting_clarification/1` and
keeps its question in the action result. Summary observations and perception
ambiguity retain full string content, including a leading slash. Async router
timeouts use integer seconds. Ouroboros iteration history uses the same
integer percentage scale as current stability.

`next_action/1` is the executive relation. The three- or four-field Fact values
passed to VirtualStore routing are Go action envelopes, parsed directly by
`virtual_store_routing.go`; they are not assertions of that relation. Context
policy embeds are libraries: `internal/context/working_set.go` loads the core
schemas and config rules with both working-set and rendering policies.
Review tooling must follow these assembly and API boundaries.

Behavioral coverage is authored in `internal/core/mangle_review_contracts_test.go`,
`internal/prompt/selector_conflict_contract_test.go`,
`internal/shards/system/review_boundary_contracts_test.go`, and
`internal/autopoiesis/stability_score_test.go`. The existing per-clause strict
bounds ratchet tracks remaining disagreements; a review without confirmed
error findings does not imply that the entire corpus passes strict analysis.
These changes still require execution of the owning packages and sharded boot
checks before their behavioral acceptance is established.

## Candidate rule pre-check (kernel_policy.go)

- `HotLoadRule` and `HotLoadLearnedRule` refuse a negated atom with a
  wildcard argument (`!active_shard(_, T)`) before the sandbox compile.
  `checkUnsafeNegation` (`kernel_policy.go:29`) reads the parsed clauses: only
  an `ast.Variable` named `_` in a `NegAtom` counts. The regex it replaced took
  the underscore of a name constant (`!executive_processed_intent(/current_intent)`)
  for a wildcard. A rule that does not parse is left to the sandbox compiler.

## Rebuild and eval order (kernel_eval.go)

1. `rebuildProgram` (`kernel_eval.go:61-156`) concatenates schemas, policy,
   and learned rules and stages the program through `writeProgramLocked`
   (`kernel_eval.go:38-56`).
2. `evaluate` (`kernel_eval.go:171-306`) runs the fixpoint over the staged
   program; a failed rebuild is dumped to `debug_program_ERROR.mg` by
   `writeFailedProgramDump` (`kernel_eval.go:543-553`), which is the first
   artifact to open on a boot failure.
3. `Clear` / `Reset` / `Clone` (`kernel_eval.go:392-493`) are lifecycle
   helpers on the kernel, not steps in the production path.

## Query semantics (kernel_query.go, kernel_facts.go)

- `Query` (`kernel_query.go:24-142`) is the basic ask; `QueryWithBindings`
  (`kernel_query.go:145-246`) carries bindings in, and `QueryCallback`
  (`kernel_query.go:249-362`) streams results instead of collecting them.
- Bulk and derived reads bypass the single-question path: `QueryAll`
  (`kernel_facts.go:791-851`) and `GetDerivedFacts`
  (`kernel_facts.go:1083-1134`).

## Learned rules are validated at load, not at eval (kernel_validation.go)

- `validateLearnedRulesContent` (`kernel_validation.go:197-361`) rejects bad
  learned content before it joins the program.
- `checkInfiniteLoopRisk` (`kernel_validation.go:365-478`) screens for
  recursion the fixpoint would not terminate on.
- `healLearnedRules` (`kernel_validation.go:557-688`) repairs what is
  repairable instead of rejecting everything; the startup outcome is
  retrievable via `GetStartupValidationResult`
  (`kernel_validation.go:730-740`).

## Provenance is opt-in (kernel_provenance.go)

- Nothing is recorded unless provenance is enabled: `EnableProvenance`
  (`kernel_provenance.go:29-36`) flips it, `SetProvenanceContext`
  (`kernel_provenance.go:23-27`) scopes it.
- `RecordDerivation` (`kernel_provenance.go:42-60`) captures rule firings;
  `Explain` (`kernel_provenance.go:72-112`) returns the derivation tree.
  With provenance off, `Explain` has nothing to show.

### Proofs are recorded per goal, on demand (kernel_proof_slice.go)

Evaluation never records derivations, whether or not provenance is enabled. A
recorded fixpoint over the whole world shard cost 7.5-8.2 s and about 880 MiB of
heap per write; every deliberation enables provenance, so each kernel write in
one paid that. `Explain` now asks `proofSliceLocked`
(`internal/core/kernel_proof_slice.go:72`) for the goal's slice:
`recordProofSliceLocked` (`:98`) walks the predicates the goal depends on
(`upstream`, `:210`), takes exactly those predicates' EDB rows
(`edbAtomsOfLocked`, `:177`) and records one evaluation of that part of the
program. The recording is kept until the next write or program change, so a
proof never outlives the facts it was drawn from. Enabling provenance
mid-session explains the current evaluation at once; there is no "armed but not
yet recorded" state. `TestProofSlice_MatchesTheFullRecordedPass`
(`internal/core/kernel_proof_slice_test.go:137`) holds every goal's slice equal
to a fully recorded pass over the production corpus.

### LoadFacts prepares batches on every core (kernel_facts_prepare.go)

`prepareFactsLocked` (`internal/core/kernel_facts_prepare.go:35`) interns, keys
and admits facts against their Decl in parallel; `insertPreparedLoadLocked`
(`:73`) inserts in input order, so the same fact wins and the same facts are
refused as in a serial load.

### World rule shapes (coder_safety.mg, schemas_shards.mg)

Three world-model rules were reordered to start from their smallest relation:
`doc_exists_for` goes through `dir_has_doc`
(`internal/core/defaults/schemas_shards.mg:411`), `dependent_count` reuses
`intelligence_dependent_count`, and `intelligence_file_depends` starts from
`file_topology`. Their results are pinned against the earlier shapes by the
`TestWorldRuleShapes_*` tests (`internal/core/world_rule_shapes_test.go:86`,
`:93`).

### Not built: world facts on relevance

The world shard holds every world fact in RAM (about 1.06 GB live for a
600k-fact workspace, each EDB fact held roughly four times: Go fact, dedupe
index, remembered atom, indexed store). The north star asks for evidence that a
turn does not reason over to stay recoverable in the store and enter the kernel
on relevance. Every world predicate is read by at least one rule today
(`dependency_link` by 17 rule heads), so this needs those rules restated over a
demand-loaded relation, not a cache. It is the largest open memory cost.


## System facts feed the outside world in (kernel_sysfacts.go)

- `UpdateSystemFacts` (`kernel_sysfacts.go:24-107`, locked variant
  `UpdateSystemFactsLocked`, `kernel_sysfacts.go:110-190`) refreshes
  environment-derived facts, including git state parsed by `parseGitStatus`
  (`kernel_sysfacts.go:193-223`).

## How it is verified

- `go test ./internal/core/...` exercises the kernel and the store.
- Startup self-check: `GetStartupValidationResult`
  (`internal/core/kernel_validation.go:730-740`) surfaces what load-time
  validation decided.
- Boot failure forensics: `debug_program_ERROR.mg` from
  `writeFailedProgramDump` (`internal/core/kernel_eval.go:543-553`).


## Development constitution adoption

The dev-only reload boundary must publish programs, evaluated stores and the
Cortex derivation map as one accepted generation. Capture the corpus twice and
reject if the assembled bytes differ; the identity names the captured bytes,
not an editor transaction. All shards consume that immutable capture. A writer
that changes files after capture requires another watcher event.

Stage a clone of each kernel while holding its mutex, preserving appended
schemas, policy, learned rules, limits and current EDB. Reject specialized
kernels whose constitution prefix was replaced. Compile, validate every retained
fact and evaluate every candidate before publishing anything. Cortex operations
hold an adoption read barrier from route selection through use; reload holds its
write side, then the routing mutex and all kernel mutexes in domain order.
Direct kernel writes serialize on those same kernel mutexes. This avoids late
program conflicts and preserves legitimate writes without rollback.

Build routing from the staged programs and the installed ownership/consumer
manifests. Replace each staged shard's shared EDB from the catch-all's canonical
inventory before evaluation, so newly consuming shards can derive immediately.
Consumption follows the rebuilt map, including local joins against owned
predicates and their explicit consumer copies. A rule reading only shared facts
has a uniform result served by the catch-all; registering an otherwise empty
shard does not require a shared replica there. The hydration regression uses
world-owned `file_topology` and the shipped `context_priority` join through
`file_exists` to require an actual world consumer, and checks the result through
the routed Cortex query. A separate regression preserves catch-all routing for
uniform results. A stale replica outside the current consumers is discarded on
reload from the canonical inventory.
Publication performs no fallible work. Rejections retain the prior programs,
stores and map. Temporary candidate stores and a quiescent pause are deliberate
costs of this opt-in development feature; latency and memory remain unmeasured.

## Campaign check receipt authority

`coder_safety.mg` judges each raw check exit against the host-supplied exit set for
that exact turn and sequence, defaulting to zero when omitted. These facts and
conclusions are protected host witnesses. Their shard home is the same world
shard as `turn_check_run` and `turn_gate`; model updates cannot widen the pass set.

### Bound queries and memory readings (query_bound.go, limits.go)

`BoundQuery` (`internal/core/query_bound.go:17`) spells the pattern for a
predicate's rows whose string arguments at given positions equal given values;
the kernel answers it from the store's argument index. Callers that want one
file's or one symbol's rows of a world-sized relation use it instead of
copying the relation.

`RuntimeMemory` (`internal/core/limits.go:120`) reads heap-object and
runtime-mapped bytes from `runtime/metrics`, which does not stop the world; the
limits enforcer and the chat footer use it in place of `runtime.ReadMemStats`.

`Fact.String` (`internal/types/types.go:141`) writes one buffer; its text is
unchanged (`internal/types/fact_string_equiv_test.go`).
