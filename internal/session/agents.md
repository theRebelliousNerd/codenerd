# Session contributor guidance

- Preserve the control split: the model proposes; the effective capability
  envelope narrows availability; exact Mangle `permitted(Action,Target,Payload)`
  authorizes; VirtualStore validates and executes.
- Nil or empty runtime config/AllowedTools means no tools. Tool registration,
  Ouroboros generation, prompt text, and `safe_action/1` never grant capability or
  permission.
- Canonicalize and bound the exact payload before asserting `pending_action/5`.
  Reject empty names, oversize payloads, wrong arity, mismatched targets/payloads,
  stale facts, and missing kernel/gates.
- Treat `nerd.md` write protection as fail closed: recognized write tools need a
  target and a live kernel authority before any executor, VirtualStore, or
  registry path may mutate.
- Specialist YAML must remain path-contained, size-bounded, strictly decoded, and
  validated before registration or spawn. Failed config leaves no agent behind.
- Keep native and Piggyback tool paths on one capability, permission, timeout,
  cancellation, accounting, post-edit build/test/critic, idempotency, and
  result-bound contract. Piggyback has no native repair round.
- Preserve the deadline conclusion window and tool-use/result pairing; never
  replay a side effect while transitioning from exploration to forced final.
  Every terminal path must still reach the shared post-edit proof gate.
- Adaptive tool-budget extensions must remain deterministic and bounded by the
  hard tool-call/time ceilings. For write intents, novel reads are orientation,
  not progress: only a durable write or focused post-write verification earns
  an extension. Repeated traces and read-only stalls deny extension. Keep the
  live calls/rounds nudge short and attach it to a paired tool result.
- An open (progress-driven, no count ceiling) loop is governed by
  `internal/context/working_set.mg`. The loop reports `working_progress`
  (intent, rounds, writes, rounds since the last write and verification) at
  every boundary; the policy answers with `working_stop` (unresolved),
  `working_finalize` (exploration over: the pending batch runs, then the
  forced-final path and the post-edit gate), `working_nudge` (steering text
  on the round's last tool result) or `working_regime(/commit)` (a change
  task that ignored the implement nudge for a span, or that wrote and then
  neither wrote nor verified for a nudge span: read tools leave the offered
  catalog, a read asked for anyway is answered with the regime and not run,
  `recall_context` stays; a verification lifts it, a write does not). A task
  that wrote and drifted is finalized only a commit span after its reading
  closed. Change the spans in the .mg, not in Go.
- A write-oriented turn on the native tool path is planned before it runs
  (`work_steps.go`): one short model call lists the edit sites as
  `STEP <file> :: <change>` lines; with two or more, the executive runs each
  step as its own pass of the loop (own anchor naming the step and the
  earlier steps' outcomes, own focus, own policy spans), gives a step that
  made no edit one more pass with reading closed, runs the post-edit gate
  once after the last step, appends a step ledger to the response, and
  fails the turn (`ErrStepsIncomplete`) when a file no step edited remains;
  a step on a file an earlier step edited is reported, not failed (planners
  split imports out). One step or no plan is the single pass. Keep the plan a list of edit sites, not an
  approach; the model plans, the harness sequences.
- Post-edit repair rounds (build, tests) go through the working request
  path. The first round is open; a round that read without editing is
  followed by one more under the commit regime with the compiler or test
  output again. Two rounds at most; the regime is restored afterwards.
- A working request carries the current call/result pair whole. A result is
  archived to a `recall_context` pointer (which states the body's size) only
  when the request cannot otherwise fit the configured input window, largest
  result first. The tool catalog is charged to the window, never to the
  observation section; that section's ceiling is the transcript bound it
  replaces (`maxToolLoopHistoryBytes`). Recall returns a whole body unless the
  caller pages. No fixed character thresholds on this path. The transcript
  keeps the last `working_transcript_rounds` (policy fact) native rounds so
  the model sees its own recent turns; those observations are excluded from
  the section, so nothing is sent twice. A `read_file` observation records
  its line span; a later read of the same file at the same revision that
  covers an earlier read's span replaces it (`working_span`), as a repeated
  body does (`working_digest`).
- New LLM-facing behavior is a prompt atom first. `AvailableTools` describes the
  effective envelope; it is not authority.
- Pass bounded task text into JIT retrieval even when delegation supplies only
  an intent verb. Distinct tasks must have distinct retrieval/cache identities;
  an expert's name is not a substitute for its actual task.
- Completion is revision-bound. Preserve `artifact_changed`, `checks_passed`,
  and `behavior_verified` as distinct stages. Missing acceptance or stale checks
  mean unverified. Final repair edits invalidate earlier witnesses.
- Test-output detection must distinguish runner summaries from other tool
  counters. Browser operation failures, including deliberate stale-ref probes,
  are not pytest results; mixed responses containing actual runner signatures
  still require execution evidence.
- Caller contracts enter through `evidence.WithContract`; never create or weaken
  acceptance from model prose. Keep headless `fix --acceptance` and session
  execution on the same transaction and persisted report path.
- Maintain explicit ownership for executor history, Spawner reservations, SubAgent
  state, coherent config snapshots, shared kernel use, persistence, and teardown.
  Add race tests for changes.
- Run session/JIT config tests, focused `-race`, relevant integration tests, and
  reconcile `Docs/architecture/session/` when contracts or wiring change.

- Resolve the tool envelope BEFORE prompt compilation: `resolveAvailableTools`
  populates `CompilationContext.AvailableTools` from the precompiled config when
  present, else from `ConfigFactory.ResolveAllowedTools` for the turn verb
  (including the `/general` fallback). Fail closed on resolution error (empty
  catalog, capability atoms omitted); never add tools to make a prompt compile.
  `compileConfig` must grant exactly what resolution promised for the same
  intent. The no-tool retry path clones the catalog into its retry context so
  the nudge sees the real allowed set.