# 07 — External audit, 2026-09-19: findings, verification and routing

Two external audits of `40c6f61a` reached Steve on 2026-09-19 (his downloads, `AUDIT.md` and
`AUDIT (1).md`, with `probe-output.txt`): a seven-finding capability and wiring audit (F1-F7) and
an additive "wave 2" of sixteen (N01-N16). Both are source-anchored; wave 1 reproduced three
findings and wave 2 eight mechanisms with isolated Go probes. This file is the tracked record:
what each finding claims, whether it was checked against the code here, where it is routed, and
what landed. The audits themselves stay where Steve put them.

Their acceptance criterion, adopted as ours: evidence cannot be borrowed, dropped, used after
becoming stale, or detached from the mutations it is supposed to describe.

## Status

- last updated: 2026-09-19 14:56
- landed: F7 (`247a2402`), F6 (`ac5c9b03`), F3 (`8dd5ef48`), F5 (`a6d4e572`), F1 (`662699eb`),
  N03 (`90100128`), N07 (`0b0a4d49`), N02 (`8c906461`), N01 (`b41565cf`), F2 with C3 (`6df972de`),
  F4 (`ad91108d` rollback with C4, `be3fa92e` isolation); L2 (`00482316`); C1 (`32f4d9c4`), C2
  (`8b007804`); N04 by codeNERD (`3d9ba680`, ladder R1-5, assisted); L4 (`57f00dbc`); the N01
  follow-on (`0915bfab`: a turn whose writes owe a test run is sent back to run one).
- next, hand-built: L5 (below).
- next, as codeNERD ladder briefs (one or two files, symptom and evidence in hand): N10, N09, N17,
  N18 with briefs ready (scratchpad `brief_n10_*`, `brief_n09_*`, `brief_n17_*`; N18 landed
  `24e9cc56`, R1-7);
  N05, N06, N14 need an observation first (N14 is confirmed by reading only).

## Found here while working the audit

- **L1 the VirtualStore's `run_tests` and `build` handlers run a free command string.**
  `handleRunTests`/`handleBuildProject` (`core/virtual_store_actions.go`) take `command` from the
  action payload (or a command-looking target) and run it through `cmd.exe /C` or `bash -c`. The
  model-facing `run_tests`/`run_build` tools are typed and refuse a `command`; this path is reached
  by kernel-routed actions. Whether any producer lets model text into that payload is unaudited --
  safety-gate territory, hand-built.
- **L2 a test ID inside the lock manager's containment check.** `acquire` skipped the
  out-of-workspace refusal for a task named `"t1"` so a test asserting that an escaping path is
  granted could pass; any campaign task given that ID took a lease outside the workspace.
  Landed `00482316` (the exemption deleted, the test corrected, the refusal pinned for every ID).
- **L3 declared-set leases compare exact keys.** A task that declared the directory
  `internal/foo` and one that declared `internal/foo/x.go` are not serialized at plan time: the
  lock manager keys a lease by the path as declared. F4's write-time guard checks ancestors, so
  the write itself is caught; the plan-time acquisition still admits both tasks at once, and one
  of them then has its write refused. **Landed `aae24670`** by codeNERD (ladder R1-11, assisted): the
  declared lease and the write-time check both refuse a path another task holds, a directory above
  it, or a path below it; the direction the report was about was pinned by none of its tests, so
  that test was added by hand.
- **N09, N10 reproduced (13:05-14:06) for codeNERD briefs.** N10: an `edit_lines` change inside a
  Go raw string holding a Mangle program is refused as unbalancing delimiters, though the Go file
  is as valid after it -- the same guard meets every edit to a test's embedded program. N09:
  `get_element "Close"` returns `A.Close` silently and `B.Close` answers to no name the tool
  accepts.
- **C1/C2 together: a plan's targets are corrected by what the campaign has learned, not guessed
  once.** Studied 2026-09-19 13:58. The rolling-wave refinement (`Replanner.RefineNextPhase`,
  `replan.go`) is shown each completed task's description and status only -- not its result
  (the research findings the task result cache and the durable artifacts hold) -- and each
  upcoming task's description and type, not its write set. Its JSON parser accepts a `write_set`
  the prompt never asks for. So a target phase 0's research proved wrong (R1-2: phase 2 aimed at
  `internal/cli/check-mangle.go`; the command is `cmd/nerd/cmd_mangle_check.go`, which phase 0
  found) cannot be corrected, and `reconcileTaskTypeWithWriteSet` then turns the missing modify
  target into a create at the wrong path (C2). Shape of the fix: the refinement's context carries
  the completed tasks' results and the upcoming tasks' write sets, with each write-set path
  marked present or absent on disk; a `/file_modify` whose targets are all absent is named to the
  refinement as a plan error to correct, never retyped; the instruction to correct targets from
  the research lives in the replanner's prompt atom (JIT), not in Go prose. **Landed**: C1
  `32f4d9c4` as shaped; C2 `8b007804` -- no retype, the attempt told its target is absent, and
  the modify contract satisfied by a pre-existing file the attempt changed (building it found the
  micro-checkpoint refusing the real fix too: "none of planned write_set paths exist" -- it now
  checks the attempt's writes with the declared paths).
- **L4 the micro-checkpoint builds the tree a second time, worse.** After every mutating campaign
  task, `runTaskMicroCheckpoint` (`micro_checkpoint.go`, March 2026) runs `go build ./...` through
  the tactile executor with a 20-second limit and the executor's environment allowlist, which has
  no `CGO_CFLAGS`. The turn it follows has already built `./...` under the session's build gate
  (`verifyBuild`: the project's build environment, its own bound), and since F2 a task completes
  only on a `/done` turn, which owes that gate green. So the checkpoint repeats a gate with a
  20-second clock -- a cold build here takes longer, the timeout reads as transient, and a good
  change is rolled back -- and, with tasks side by side, builds a tree holding a sibling's
  half-finished edits. Measured 14:55: `go build ./...` without `CGO_CFLAGS` succeeds here in 9.0 s
  warm, so the missing environment was not itself fatal; the clock and the duplication were.
  **Landed `57f00dbc`**: the checkpoint's build deleted (every mutating task that writes Go ends
  in a session turn whose `/done` owes that gate); it keeps its file-existence check.
- **L5 a tool-creation task completes without its tool.** `executeToolCreateTask`
  (`orchestrator_task_handlers.go`) asserts `missing_tool_for` and polls for `tool_registered` for a
  hardcoded 30 minutes; at the deadline it returns `{"status": "pending", ...}` with a nil error,
  so the task is marked completed and the phase moves on with no tool. Open, hand-built (the
  campaign's completion gate). Its second half: the kernel's `missing_tool_for` becomes work only
  through `autopoiesis.Orchestrator.ProcessKernelDelegations`, which the interactive boot paths
  poll every 2 s and `cmd_instruction.go` calls once; its own comment says campaigns "do not poll
  at all -- they call ProcessKernelDelegations once at the point in the turn where a delegation
  could exist", and `executeToolCreateTask` never calls it. So a headless campaign's tool-creation
  task always waits the full 30 minutes for a registration nothing produces. The campaign
  orchestrator has no handle on autopoiesis, so the fix is wiring through its config and every
  campaign boot path: an R2 candidate.
- **N20 the MCP verbs are offered with no MCP server configured.** Every persona's catalogue
  carries `mcp_map`, `mcp_probe`, `mcp_call`, `mcp_expand` and `mcp_context` "regardless of how
  many servers are connected" (`prompt/config_factory.go`), a deliberate fixed cost; with zero
  servers they are schemas on every turn that can only fail, and R1-6 sent `run_tests` through
  `mcp_call` ("no MCP servers are configured for this workspace"). The catalogue is not derived
  (S21); open.
- **N21 the working focus froze under the commit regime.** The focus -- whose context each
  working request renders -- moved only when a call named a path; a recall names an observation,
  so once reading closed the focus stayed on the last file read. R1-9 recalled the files it had to
  edit eleven times while every request rendered `working_meter.go`, and stalled. Landed
  `51e86c27`: a recall moves the focus to the recalled record's file.
- **N22 the forcing gate does not check that a turn's tests fail without its change.** Coverage
  counts executed lines; a unit test of a new helper executes every line while the behaviour the
  helper exists for stays unpinned. R1-10's five tests passed with its fix reverted at the call
  sites. The reviewer's fail-before check is mechanical: the executor holds each written file's
  preimage, and `go test -overlay` can run the turn's new tests against the preimages without
  touching the tree. **Landed `53eb3551`** (hand-built: the completion gate). The pinning gate
  (`internal/session/pin_gate.go`) takes the turn's changes out one at a time -- a changed function
  put back as it was, an added one removed, the file's imports reconciled -- overlays each on the
  workspace and runs the tests the turn wrote: they fail or stop compiling, and the change is
  pinned; they all pass, and it is not. Functions are compared by token stream, so a comment or a
  re-layout is not a change. What a turn owes is policy (`turn_owes_gate(Turn, /pinned)` for /fix,
  /create and /implement turns that wrote Go, from a new `turn_verb` fact); a refactor or an
  optimisation keeps behaviour and owes nothing. A forcing round after coverage asks for the tests
  that pin what is unpinned, and the verdict names `/change_not_pinned` otherwise.
- **N22b the gate is function-shaped, and a change can be unpinned inside one.** R1-11's fix put
  both directions of the overlap check in one function, so taking the function out fails the tests
  either way and the gate is satisfied -- while the direction the report was about was pinned by
  nothing (found by hand). Measured the same day with a prototype that mutates the lines the turn
  changed and runs the turn's own tests: 16 mutants over 29 changed lines, 3 survived, and one of
  them was exactly that gap (`if key == root` forced true, skipping the ancestor walk). Two
  cautions from the same run: one mutant left the tests hanging for ten minutes, so a mutant needs
  a bound derived from the baseline run; and `sort.Strings(held)` survived while changing nothing
  observable -- the equivalent-mutant problem. **Landed `7f02f4d2`, recorded rather than charged.**
  Every condition on a line the turn changed is held true and false and the turn's tests are run
  against it; the survivors reach the log, the turn's record and the pinning round's prompt, and
  the verdict still reads the declarations only. The reason is a measurement: on N24's own commit
  26 units gave 8 survivors and five were guards whose forcing changes nothing observable, so a
  charge would sometimes be unanswerable. The tests are run once unmutated first, which bounds
  every mutant at twice that run plus a minute; a loop condition is only held false.
- **N23 the test repair round keeps a stale build failure.** Its recheck sets `result.BuildCheck`
  on a failed build and never on a passing one, so a later attempt that fixes the build leaves the
  failure recorded until the closure re-measures; and the round logs "giving the model one repair
  round" while it runs up to the repair budget's attempts. Found by R1-10's critic in code the
  change did not touch. Checked 2026-09-19: the stale record is latent, not observable -- nothing
  reads `result.BuildCheck` between the round and the closure, which re-measures every gate, and
  the critic's "turn signals" line passes `true, true` by construction. What is left is the log
  line, which says one round and means up to the repair budget's. Not worth a run; fix it when
  that file is open for another reason.
- **N24 a repair round could not see the test it broke.** The round closes the read tools after
  its first round that writes nothing, and its prompt carried the runner's output only, so a model
  that had not already read the failing test had no way to. R1-12 spent three attempts, 18 model
  calls and 746.7k input tokens on 26 `recall_context` calls without an edit, and the turn died at
  the test gate with the failure it started with. Fixed by hand: the prompt carries each failing
  test's source as it is on disk, found in the package the runner named or beside the turn's own
  writes -- the answer the removed-tests round already gives ("a paste, not a reconstruction from
  memory").
- **N25 the test gate ran the turn's own packages and nothing else.** A change to a package
  others import was verified by its own tests alone. R1-13 and R1-14 renamed Go methods in the
  CodeDOM reader; `internal/tools/codedom` was green, the pinning gate satisfied, both turns
  `/done` -- and `internal/observation`'s codesearch test, which pins how a hit inside a method is
  named, failed on the next full suite. Fixed by hand `c4c097fc`: the packages that import the turn's
  run in the same gate, one hop, found through code and tests, with the same baseline attribution
  so a package that was already red is not charged. They run where the verdict is decided -- the
  gate and the closure -- not inside every repair round's recheck.
- **N26 a repair round was never shown what the turn changed.** The prompt carried the failure,
  and since N24 the failing tests, but not the turn's own edits, so the model re-diagnosed from
  scratch every attempt: R1-12 and R1-15 each spent three attempts asking why an element was "not
  extracted" while the rename that did it was one line of their own diff. Fixed by hand `ca7b31bf`:
  the build and test repair prompts carry the diff of the turn's writes, preimage on the left, a
  created file marked as created.
- **N27 a forcing round stated its own subject over a red test run.** `coverageRepairPrompt`
  opens with the constant *"The tests pass, but no test executes these lines of code you changed:"*
  and the block under it is whatever the round's recheck last returned -- which on a red recheck is
  a FAIL trace. R1-16 (2026-09-19) got all three of its remaining attempts that way, was told to
  write more tests and forbidden the production fix the failure needed, and weakened its own
  assertion to escape. The seam: `repairSpec.promptFor` took only the failing output, so it could
  not see what the recheck already knew; `vet` and `pinning` had each smuggled it back through an
  ad-hoc `testsBroke` local. Fixed by hand `2e48c308`: `recheck` reports a `repairFailure`
  {Output, TestsBroke} and `repairSpec.prompt` chooses, with a shared default so no round can fall
  through to a false preamble. Both ad-hoc locals deleted.
- **N28 an importer check with no verdict was reported as a pass, under a clock shorter than the
  suite it gated.** R1-18 printed `build ok | tests ok | 0 uncovered | 0 findings` over a gate that
  timed out: `gateTests` propagated only `VerifyFailed` from `verifyImporters`, so an indeterminate
  or canceled run fell through to the pass measured on the turn's *own* packages. The clock that
  produced it was `testVerifyTimeout = 4 * time.Minute`, hardcoded -- `internal/session`'s own tests
  take 259 s, so any turn touching a package it imports could never have its importers verified.
  Both introduced with the importer gate itself (`c4c097fc`). Fixed by hand `8f88be65`: the merge is
  a named decision (`mergeImporterVerdict`), and both verification budgets are zero, with a
  non-positive budget meaning unbounded rather than already-expired.
- **N29 the removed-tests gate could not tell a dead test from a hidden one.** A turn that deletes
  dead code must delete its tests, and the gate refused every such turn -- R1-18 wrote the better
  change (the span-balance heuristic replaced by the whole-file Go parser check already called on
  the next line) and was failed for deleting the two tests whose only subject was the function it
  deleted. Until this, no turn could delete dead code through codeNERD at all. Fixed by hand: a
  removed test is released only when the same turn deleted a function that test names, matched on
  the whole identifier (`tokenText` writes an identifier as `IDENT <name>`), so a near-name does
  not release anything.
- **N30 the CodeDOM fact layer was dark in every agent run.** `code_element` answered 1,175 queries
  with no row in R1-16 while `code_defines` answered 950 of 1,216 from the same kernel. Four breaks,
  any one sufficient: the `user_intent` a run asserts carries the brief *prose* in the target
  position, so `file_topology(Target, ...)` cannot match; `codedom_edit.mg` keys on
  `/current_intent` while the run asserts `/task_intent_1`; the session executor never consults
  `next_action` at all, so `next_action(/open_file)` is unreachable from this path; and the
  handlers emit scope facts only under an already-open scope. Cost beyond the empty query: all 197
  lines of `policy/codedom_edit.mg` key on `code_element`, so edit safety, breaking-change risk,
  the API-handler warnings and the CodeDOM activation boosts derived nothing. Fixed by hand
  `e78e7241` on Steve's call ("fix the codedom dude! add capability"): the run parses the file its
  focus names into the layer, replaced per file by content digest. Full write-up in
  [08-codedom-journeys-2026-09-19.md](08-codedom-journeys-2026-09-19.md).
- **N41 two turns closed /done over work their own text said was unfinished, and the mechanism
  is NOT yet established.** R5 run 3: 11 of 189 forwarding stubs removed, verdict `/done`,
  "Evidence: artifact_changed. Verified by evidence: the build and the tests were both measured
  green after the final edit" -- one paragraph after "I cannot execute the remaining 177
  deletions ... Requesting re-invocation." R5 run 4: 85 of 189 removed, `/done`, "Final deletion
  batch -- the remaining 48 files."

  **The mechanism is now established, and it is a deliberate decision rather than a defect.**
  On the full kernel a doc-only turn derives `turn_owes_gate` 0 rows, `has_unmet_gate` 0, and
  `turn_done` 1: it owes nothing, so every gate `turn_verified`'s write branch negates holds
  vacuously and the turn is done for having written one markdown file. Three named tests pin
  exactly that -- `turn_write_class_test.go` "a document owes no gate" (wantDone: true),
  `test_run_gate_test.go` "aDocumentIsDoneOnExecution" ("nothing compiles or runs a document"),
  and `TestVerifyCompletedToolTurn_ADocumentEditOwesNoTestRun`. A change making `/doc` owe
  `/test_run` and blocking doc-only verification was written, proved fail-before/pass-after on
  `NewRealKernel`, and reverted when the suite showed it contradicting all three.

  So the gap is not a missing gate. It is that completion is measured by gates, gates measure
  the repository, and no gate can see the scope of what was asked. The reasoning behind
  done-on-execution is sound in its own terms -- holding a turn hostage to a gate a document can
  never satisfy would be worse -- and the consequence is that a docs task which did 6% of the
  work reports success. Closing it means deriving an acceptance contract from the brief, which
  is the `turn_acceptance` path that already exists and is opt-in, not another entry in
  `turn_owes_gate`. That is an architectural decision and the architect's to make.

  A methodological note worth more than the finding: the same question answered the OPPOSITE way
  on a six-file policy harness, where a doc turn appeared to owe `/build` and `/test` and
  `turn_unmet_gate` would not exclude a passing gate. Loading a subset of the corpus changes
  which rules exist. A claim about the kernel is only worth what `NewRealKernel` says.


- **N42 the commit regime withdraws the tools a removal sweep needs to finish.** R5 run 3:
  75 tool calls -- 29 `recall_context`, 15 `read_file`, 11 `grep`, 11 `delete_file` -- with the
  session log recording the regime engaging "after 35 executed tool call(s)". One grep per
  deletion, so 35 calls bought 11 files, and when the read tools closed the remaining 177 became
  unfindable. The regime is right about what it was built for; a removal sweep legitimately
  alternates find and delete and looks identical from outside.

  Telling the model its own tool surface in the brief -- one search can return the whole set now
  that a cap announces itself, and a round may carry many calls -- took the same task from 11
  files to 85 and from 3 tool calls to 88 deletes. So the regime was not the binding constraint;
  the search pattern was. Recorded rather than changed.

  A batch `delete_file` was considered and NOT built: `checkSafetyWithGate` extracts ONE target
  and asserts one `pending_action` and one `file_recoverable`, so a call carrying 189 paths would
  be gated on whichever path the extractor picked and the other 188 would ride in behind it.

- **N43 the class was matched structurally, and three real documents were destroyed.** R5 run 4
  deleted 88 files: 85 were genuine forwarding stubs from a sealed list, and three were not --
  `cli/02-CURRENT-STATE-CLI.md` (7,053 bytes), `cli/03-GAP-ANALYSIS-CLI.md` (2,962) and
  `prompt/06-PUBLIC-API-AND-TYPES.md` (5,729), all of them real inventories and gap matrices.

  The brief defined the class by content -- "the test is whether anything is left once you take
  the redirect away" -- and the match was made on shape. Elsewhere in the corpus a
  package-suffixed name (`02-CURRENT-STATE-WORLD.md`) IS a stub left by the 2026-07-13 rebuild;
  `cli/` never got that rebuild, so there the suffixed files are the live documents and there is
  no unsuffixed sibling. `prompt/` has two files claiming slot `06`, which is the same shape a
  superseded pair makes. Every structural signal pointed the right way and the content did not.

  183 `read_file` calls preceded 88 deletions, so files were being read; reading did not prevent
  it. The honest reading is that a filename rule, once it has explained 85 cases, is not
  abandoned for the 86th. A content test is the only thing that separates them, and it is cheap:
  a redirect stub is under 2 KB and has nothing left when its links are removed.

- **N39 a gate certified inert code, and the four corpus gates had three more holes.**
  Fixed `64e7ab7a`, `a6a4afc3`, `77790de3`, `52b7c5f1`, all hand-built because these gates score
  the dogfood ladder. (1) All four counted `internal/testing/` as production Go -- a harness
  imported by other packages' tests cannot be named `_test.go` -- which put 7 predicates on the
  undeclared list that only `context_harness` asserts, and in the other direction let
  `simulator.go`'s keyword list `{"test_failure", "assertion", "test_result"}` silence a genuinely
  starved `pytest_failure /assertion`. (2) The undeclared scan's `Predicate:\s*"..."` had no
  leading boundary, so it matched the tail of `FactPredicate:`, a `types.ShardLearning` row label
  that `core/dream_router.go` says "has no Decl anywhere and must not get one" -- 933 `Predicate:`
  literals against 3 `FactPredicate:` ones. (3) The guidance promised that declaring without a
  consumer "moves it to the starved-predicate list", which cannot happen: starved requires a rule
  body to READ the predicate, so a declared-and-unread one leaves the undeclared list and arrives
  nowhere. That promise is what made R2-2's Decl-only change look complete.
  (4) The worst: **`TestStarvedPredicateBudget` treated any Go string literal as a producer**, so
  `mkPred("target_is_large", ...)` satisfied it. The kernel keeps a registration only when
  `decl.IsExternal()` (`kernel_eval.go`), so against a plain Decl the handler never runs -- and
  the gate still announced "4 predicate(s) on the starved list now have a producer". Had R2-3
  regenerated the baseline as its brief asked, four predicates would have left the list having
  gained nothing. Now a predicate needs a live production route: a rule head, a real assert, or
  an external() Decl. `goProduces` has all four branches pinned, plus a guard that fails if the
  corpus stops declaring anything external, which would silently turn the rule into "every
  registration is inert".

- **N40 the JIT compiled one prompt for a 67-minute turn, and the knowledge it withheld was the
  knowledge the turn needed.** Measured on R2-3
  (`20260920_054430..._051356_...`), exactly one compile for the whole run:

      Kernel query: target_need(/go, Need) -> 0 results (0ms)
      CompilationContext{mode=/active, shard=/coder, lang=/go, intent=/fix,
                         budget=94000, world_states=[]}

  ~70 tool calls, 22 file reads and 2 writes ran against that single prompt. No steps were planned,
  so `stepSystemPrompt`'s per-step re-compile (`e434033b`) never ran, and the needs decided before
  the turn knew anything were never re-derived as it learned exactly which files it was touching.
  `policy/jit_needs.mg` holds one rule, `target_need(/mangle, /authoring_mangle)`, and
  `targetNeeds` asks with the language alone -- so a turn whose work is the Go side of the Mangle
  FFI is, by construction, a `/go` turn with no Mangle knowledge in the window.

  What it cost is exact. `atoms/mangle/engine_truths_pinned.yaml` already carried "an `external()`
  premise whose input argument is a variable bound by an earlier atom PANICS", which is precisely
  the wall R2-3's design was heading for, and it is gated `world_states: ["authoring_mangle"]`.
  The harness had the fact and decided it was not needed.

  Half fixed. The atom was missing the other half of the contract -- that a `mkPred` registration
  reaches the engine only behind an `external()` Decl, and that the two truths compose, so a
  predicate whose rules read it with a bound variable cannot be served by an external at all --
  and that is now written down, verified against `kernel_eval.go` and against the live gate.
  **Delivery is deliberately not fixed here.** `target_need` is keyed on language, and the honest
  trigger is not a language: the candidates are a path-keyed need (which needs a positive atom to
  bind the path, so it means an explicit boundary-file relation in policy) or re-deriving needs
  when the working focus lands, which is the real JIT promise and a larger design decision than a
  measurement licenses. Recorded as H11 was: the number is established, the fix is not guessed.

- **N10 CLOSED (2026-09-20) by R1-19's whole-file delimiter check, and the mechanism is now
  known.** The audit reported that an `edit_lines` change inside a Go raw string holding a Mangle
  program was refused for delimiter balance though the file is as valid after it. The cause was
  that the old guard lexed only the SPAN -- `netDelimiters(strings.Join(oldLines, "\n"))` -- so a
  span sitting inside a raw string was read as ordinary code and the quoted program's braces were
  counted as structural. Nothing in that call could know the span was quoted; the file was never
  consulted. Verified both directions on the same probe: a raw string containing `policy {` /
  `allow` / `}`, with line 4 replaced by `policy {` + `  nested {` --
  the pre-R1-19 guard refuses it ("braces: replaced text had net +1, new content has net +2") and
  the current one accepts it, leaving valid Go. R1-19 was run against a brief whose premise was
  wrong (D2, retracted), and closed a real defect the brief never mentioned.
- **N37 the pinning gate reported a change unpinned without checking whether anything pinned it.**
  It ran only the tests the TURN wrote, so a turn that correctly wrote none -- because a repo-wide
  invariant test already covered its change -- was told "nothing would notice if a change were
  lost". R1-20 (2026-09-20) deleted three asserts of an undeclared predicate and updated the
  undeclared-assert baseline; `TestUndeclaredAssertBudget`, three packages away, fails the moment
  the production file goes back. Cost: 3 repair attempts, 18 model calls, 940,761 input tokens
  writing a test that should not exist, and a correct change ending `/unverified`. Fixed by hand:
  the gate puts the turn's production changes back and runs the suite with `-failfast` before
  asking the model for a test. The claim is now a measurement.
- **N38 a provider must be wired on three construction paths, not one.** The CodeDOM fact layer
  (N30) was wired onto the session executor, measured dark, then onto the Spawner, measured dark
  again: `nerd fix` runs on `j.executor.CloneForTask()` (`task_executor.go:325`), which copies an
  explicit list that did not include it. Worth an audit of what else `CloneForTask` and
  `Spawner.Spawn` fail to carry.
- **N36 a test that encodes a defect as a contract makes it unfixable through codeNERD.**
  R2-1 (2026-09-19) found the right seam for N31 and was failed by
  `TestSettersForwardAndAreInertWithoutSupport`, whose comment argues *for* the design being
  fixed. The removed-tests gate will not let such a test go, and the test-repair prompt's standing
  posture is "the test states the required behaviour"; its escape hatch ("if -- and only if -- you
  can show the test itself asserts something incorrect, say so explicitly and explain why") was
  never taken in three attempts and a million input tokens. Open: measure how often the failing
  test is the wrong one, and whether the escape hatch is reachable in practice.
- **N31 grounding capability detection is defeated by the broker passthrough.** `NewGroundingHelper`
  decides whether a client can ground by type assertion, and the broker's `core` implements every
  method of `GroundingController` as a silent forward that no-ops when the underlying client lacks
  it. So the assertion succeeds for every provider: on a Meta config the log prints "Gemini
  grounding: Google Search enabled", the call forwards into the Muse Spark client, falls off the
  `ok` check, and nothing is enabled -- while that model's own native search is never reached. A
  wrapper that forwards every optional interface makes `client.(Capability)` meaningless. Open.
- **N32 the parity check is right, the alarm is mis-scoped, and underneath it a registered tool
  cannot be executed.** Corrected twice; this is what the code says. `VerifyKernelToolParity`
  builds its "registry" set from `synth.ListRuntimeTools()` -- Ouroboros-generated binaries only,
  3 of them -- and compares it against every `tool_registered` fact, 9. The six `go_*` come from
  `core.ToolRegistry.collectToolFacts`, so they are reported `unknown_in_kernel` on every boot by
  a comparison that was never going to match. That much is mis-scoped.
  **But the concern the check states is real**, in its own words: "a `tool_registered` fact with no
  binary behind it makes the kernel plan around a capability that will fail at call time."
  `ToolRegistry.ExecuteRegisteredTool` runs `exec.CommandContext(ctx, tool.Command, args...)`,
  passing `tool.Command` as the **executable name** -- and the init scanner writes command *lines*
  (`internal/init/tools.go`): `go_lint` is `"golangci-lint run"`, `go_fmt` is `"go fmt ./..."`. So
  a registered project tool that is reached would look for a binary whose name contains a space.
  Three parts, and they want separating before any of them is briefed: the parity check compares
  like with like; a command line is split from its executable; and the model gets a typed lint tool
  (G4/G5). Open.
- **N33 `shard_status/3` is asserted with no Decl.** "the fact is stored but no rule can read it,
  and Query will not return it." Open.
- **N34 token counting drifts 14.7% for `muse-spark-1.3-contributor`.** Predicted 1,235,137 against
  1,343,721 billed over 20 calls. The broker's own warning says a persistent gap means `measure()`
  no longer matches what the client sends -- so every window-budget decision is made on a wrong
  tokenizer. Open.
- **N35 `project_forbidden_path` costs 0.9 s per query.** 14 queries, 12.7 s, all answering, on the
  write path. Open.
- **N19 the working request drops history before the first kept round.** `prepareWorkingRequest`
  (`working_context.go`) sends the loop's anchor and history from the earliest kept assistant
  tool-call round onward; a user message with no tool round before it is not sent. Every production
  caller reaches a forcing round after the turn's own tool calls, so nothing observed is lost today;
  found building the N01 follow-on's test, which first passed an empty history. Latent.
- **N17 the commit regime's sentence is sent twice.** In a repair round under the commit regime,
  each re-sent demand ends with "Reading is closed for this task..." twice: `repair_loop.go`
  appends it to the prompt, and the round's re-send (`build_verify.go`) appends it to that prompt
  again. Observed in R1-5's coverage round. Brief ready.
- **N18 a forcing round's writes are never formatted.** The session gofmt's the Go a turn wrote
  once, after build repair and before the test, coverage, vet and critic rounds
  (`formatWrittenGoFiles`, `executor_tools.go`); what those rounds write stays as written and the
  turn reports `checks_passed`. R1-5's coverage round left a doubled blank line (its landing is
  assisted for it); `orchestrator_tasks.go` carried the same at HEAD. Open, a codeNERD brief.

## Routing rule


Hand-built: completion-gate, verdict, rollback and scheduling-authority logic (the model does not
widen the rule that constrains it), and anything that stops codeNERD from running. Everything else
that is one or two files with the evidence in hand is a ladder brief for codeNERD -- these are
real defects in its own tools, found by someone else, which is the best rung-1 material the ladder
has had. Larger items wait for their rung.

## Findings

"Verified" means the claimed mechanism was found in the code at HEAD on 2026-09-19 (a file:line
read, or a run where one is named). It is not a reproduction unless the row says so.

| ID | Audit priority | Finding | Verified here | Route | Status |
|---|---|---|---|---|---|
| F1 | P1 | Transient turn facts keyed by verb, not execution: `turn_evidence(Verb,…)`, `turn_gate(Verb,…)`, unkeyed `hollow_success` on the kernel `CloneForTask` shares; cleanup can sweep another executor's facts | reproduced on `a6d4e572`: B closed `/done` on A's test gate; B's cleanup took A's verdict; an early-return turn swept A's `turn_evidence` | hand | **landed `662699eb`** (every verdict relation keyed by a minted turn atom; cleanup retracts only what the turn asserted; created files are the executor's record, not `created_source`/`test_file_for` in the shared kernel; the legacy claim predicates deleted) |
| F2 | P1 | Campaign `spawnTask` takes the string route and drops the typed outcome, so `/unverified` with a nil error completes the task (the ladder's C3/C4) | known (C3/C4) | hand, before R2+ campaigns | **landed `6df972de`**, with C3 (the campaign reads the observed return and fails any outcome but `/done` by its missing evidence; the retry carries the last failed attempt's error; a prose-only executor is refused at construction) |
| F3 | P1 | Gate order build→tests→coverage→vet→removed-tests→**critic**→close: the critic edits after the last guard, and the final closure refreshes tests only (`gateTests(…, false)`, second return discarded) | seven regressions fail on `a9534981` | hand | **landed `8dd5ef48`** (the closure re-measures build, tests with coverage, vet and the test inventory at the final revision; the critic runs before the forcing rounds; a broken uplift is undone) |
| F4 | P1 | Declared write sets lock and snapshot; actual writes outside them are neither isolated nor rolled back (the ladder's C4) | known (C4); reproduced on `6df972de`: a retry found the failed attempt's import in a file outside the set | hand, before R2+ campaigns | **landed**: rollback `ad91108d` (the observed return carries each write's preimage and postimage; a failed attempt's writes are undone compare-and-swap, never over a later writer's); isolation `be3fa92e` (a write takes the path's lease at the moment it happens, without waiting; a path another task holds, or a directory above it, is refused by name) |
| F5 | P1 | `go vet` findings in files the turn did not write become `VerifyPassed`; a cause in `state.go` (a mutex added) diagnosed in `use.go` is discarded | the audit's case passed vet on `8dd5ef48` | hand | **landed `a6d4e572`** (findings are the difference from the same packages vetted before the turn, by file and message; no baseline charges every finding; a failure naming nothing is indeterminate). Reverse dependencies are not vetted, as they are not tested -- with N05 |
| F6 | P1 | A deleted test is excused when any test anywhere shares its name | `workspaceTestNames`; fails on `247a2402` as the audit said | hand | **landed `ac5c9b03`** (a test moves only within its package directory, among files the default build includes; a copy behind `//go:build ignore` is a removal) |
| F7 | P2 | Rollback deletes a pre-existing empty file (`exists = pre != ""`) | reproduced on `f91c39b6` | hand | **landed `247a2402`** (also: unreadable ≠ absent in the snapshot, the current-state read and the test-baseline overlay; an undo that fails stays written) |
| N01 | P1 | Build/test gates run only when a `.go` file was written; policy demands them for every write, so non-Go work has no affirmative completion path | `build_verify.go:222,297,602`; reproduced on `4b68a04f`: a `.md`-only turn derived no `turn_done` | hand (completion contract) | **landed `b41565cf`** (what a write owes is policy, by extension: Go owes the executor's gates, documentation nothing mechanical, anything else a passing test run after the last write -- `/test_run`, from N07's receipts) |
| N02 | P1 | `getEligibleTasks` falls back to an in-memory scan whenever the kernel's answer is empty -- including a valid "wait" | `orchestrator_phases.go:94-98`; the fallback checks dependencies only, so it schedules tasks `eligible_task` withholds for write-set conflicts, ordering or backoff. Reproduced on `0b0a4d49`: a pending task whose write target an in-progress task holds was scheduled | hand (scheduling authority) | **landed `8c906461`** (the kernel is the only scheduler; a kernel holding none of the phase's tasks is reloaded and asked again. Removing the scan exposed two defects it hid, both fixed: a priority-inversion deadlock in `has_earlier_task`, and a campaign clock nothing moved, so a retry stayed in backoff for the run -- the orchestrator now feeds `current_time` at most once a second) |
| N03 | P1 | Three failed phase checkpoints call `completePhase`: `/completed`, progress, a success observation | `orchestrator_tasks.go:144-152`; the old test pinned `/completed` | hand (completion) | **landed `90100128`** (the phase closes `/unverified`: hard dependents blocked, the campaign blocked on it by name, the observer told failure, a resume re-arms the checkpoint) |

| N04 | P1 | `apply_edits` rollback visits `succeeded` only; a target whose write failed part-way is left corrupt | `apply_edits.go:332-379` | codeNERD brief | **landed `3d9ba680`** by codeNERD (ladder R1-5, assisted: one blank line by hand, N18) |
| N05 | P1 | Impacted-test selection follows two dependency edges, not a fixed point | `test_dependency.go` ("simplified - could query kernel for full transitivity") | codeNERD brief | open |
| N06 | P2 | Coverage-gap report counts production callers as test coverage | not yet checked | codeNERD brief | open |
| N07 | P1 | `run_impacted_tests` is a test execution by name alone; a dry run or an empty selection mints executed-test evidence | reproduced on `90100128`: a dry run counted `SuccessfulTestTools = 1` | hand (evidence) | **landed `0b0a4d49`** (a test run is a receipt the process-starting code records -- typed runner, `runGoTests`, shell test commands, the VirtualStore handler; the counter is `TestRunCalls`) |
| N08 | P1 | The registered CodeDOM reader is per-line regex: misses generic funcs and `async def`, invents declarations inside raw strings | not yet checked | R2 (route readers to the parser) | open |
| N09 | P2 | `get_element` returns the first same-named element; `A.Close` vs `B.Close` silently resolves to one | `elements.go` first match | codeNERD brief | open: R1-8's fix (not landed) orphaned a function beside a same-named method |
| N10 | P2 | The edit delimiter guard counts only the replaced fragment, so a valid edit inside a multiline literal is refused | `lines.go:171-179`; R1-4d's log has one such refusal (08:34:57) to classify | codeNERD brief | open: R1-6's fix (not landed) turned the guard off below a JS regex or Rust lifetime |
| N11 | P1 | A failed reread supersedes the successful observation of the same revision | not yet checked | R2 (working policy + store) | open |
| N12 | P1 | Aggregate observations are stamped with the focused file's revision | not yet checked | R2 | open |
| N13 | P1 | The working-context archive scope is random per executor (`rand.Text()`), so a resumed executor cannot reopen it | `working_context.go:125-126` | R2 | open |
| N14 | P2 | Co-use statistics settle every nil-error turn as success, `/unverified` included | `executor.go:913-917` | codeNERD brief | open |
| N15 | P2 | The impacted-test provider is process-global, last workspace wins | `run_impacted_tests.go:63-98` | R2 | open |
| N16 | P2 | A contained symlink stops snapshot certification (fail-closed, a capability limit) | `change.go:173-174` | decision (Steve) | open |
| N17 | P3 | Under the commit regime a repair round's re-sent demand carries the regime sentence twice | R1-5's coverage round (llm_io 14:43:28); `repair_loop.go` and `build_verify.go` each append it | codeNERD brief | **landed `2c373714`** by codeNERD (ladder R1-10, assisted: the scenario test by hand) |
| N18 | P2 | Go written by the test, coverage, vet and critic rounds is never gofmt'd; the turn still reports `checks_passed` | R1-5 (one `gofmt: formatted` line, 14:38, before the coverage round's insert at 14:44) | codeNERD brief | **landed `24e9cc56`** by codeNERD (ladder R1-7) |
