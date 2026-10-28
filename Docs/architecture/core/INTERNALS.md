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
