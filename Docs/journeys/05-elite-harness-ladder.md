# 05 — The elite-harness ladder

Steve's goal (2026-09-19): codeNERD dogfoods itself until the codebase is error free, and climbs
a ladder of milestones that would let it be called an elite coding harness: 1, 2, 5, 10 and 50
file edits from a single prompt on a real problem or optimisation quest; a rewrite of every doc in
`Docs/architecture` that codeNERD does itself, to a standard its reviewer is impressed by; and
recursive, incremental improvement of the whole codebase.

This file is the program of record: the rungs, what counts as passing one, the error-free gates,
the protocol for a run, and the status of each rung with the run that moved it.

## Status

- last updated: 2026-09-19 20:25
- **R0 gates passed** on `10223378`: three consecutive uncached runs (06:21-06:36), each G1 build,
  G2 `go vet -tags sqlite_vec ./...` and G3 `go test -count=1 ./...` green, 89 of 89 packages, 0
  cached, 4.8-4.9 min. The earlier attempts ran `go test ./...` with the test cache, so a "green"
  run re-reported old passes for every package whose tests touch no files -- a timing flake there
  could never show; they do not count. R0's other clause (a run's verdict matches its evidence) is
  reviewed on every codeNERD run: true for R1-3, R1-4 and R1-4b (both failed and said so; R1-4b's
  message did not say why, fixed in `27a4f0db`).
- current rung: R1 -- streak 0: R1-3 landed; R1-4 was refused by the removed-tests guard, R1-4b by
  its own coverage round, R1-4c by the broker (a replayed think counted by its ciphertext), R1-4d
  ended `/unverified` -- two harness clocks cut a model that thinks for minutes (the repair
  episode's, then `nerd fix`'s 25-minute default). Steve, on seeing it: "there should not be
  timeouts like that... some agentic runs are like hours long." No run-level clock is left
  (`c9f8212d`, `4316415f`, `02e4c8dd`, `f91c39b6`, `856ff1fe`; program of record
  `06-unattended-hardening.md`). R1-4e (brief v2 on `06947d43`, no clock left) ended
  `/unverified` truthfully: its forced coverage test exposed an order-dependent preload, the model
  moved to the kernel's one-fragment load with the old loop as fallback, and the review found a
  broken sibling still fails an unrelated file through that fallback -- a design defect in the fix,
  not a harness blocker. Next: the audit's tool-level briefs for the streak -- N04, N10 and N09 are
  reproduced with briefs ready -- then this brief again. R1-5 (N04: a failed `apply_edits` left the
  half-written file behind) landed **assisted** (`3d9ba680`): fix and tests codeNERD's, verdict true,
  one doubled blank line its coverage round left removed by hand -- the session formats a turn's Go
  before the forcing rounds run, never after (N18) -- so the streak does not move. R1-6 (N10: a
  valid edit inside a Go raw string refused for its delimiters) ended `/done` and did not land: the
  fix scans the file's prefix for the lexer state at the edit, and a quote it cannot read -- a
  JavaScript regex holding one, a Rust lifetime -- leaves it "inside a string" for the rest of the
  file, so an edit below that drops a closing brace goes through where HEAD refuses it. The critic
  found nothing. R1-7 (N18: a forcing round's Go was never gofmt'd) **landed** (`24e9cc56`): the
  turn formats what it wrote again, last, before the closure re-measures the gates; a whole-turn
  test, failing at HEAD; no hand edit. **Streak 1.** R1-8 (N09: a shared method name resolved
  silently) ended `/unverified` -- its alias parser's defensive branches left uncovered -- and the
  review found a regression besides: a package-level function beside a same-named method is
  unreachable by its only listed name. Its critic was cut at 3 minutes (H1). **Streak 0.** R1-9
  (N17) was stopped by the working policy (`read_only_stall`, 24 rounds, no edit) -- the harness's
  fault: the working focus froze on the last file read before the commit regime closed reading, so
  eleven recalls of the files it had to edit rendered another file's context each time (N21, fixed
  `51e86c27`). The critic's own clocks are gone too (`8958ffb0`). R1-10 (N17 again) wrote 22 s after
  reading closed and landed **assisted** (`2c373714`): the fix is codeNERD's, but its five tests
  exercise the new helper and all pass with the fix reverted at its call sites -- the scenario
  test the brief asked for was added by hand -- and it spans three files. The critic ran 4 min
  51 s. Found: N22 (the forcing gate does not check that a turn's tests fail without its change)
  and N23. Streak 0. R1-11 (L3, the overlapping write-set leases) landed **assisted**
  (`aae24670`): the fix is right on all 15 overlap shapes the review probed and HEAD is wrong on 10,
  but the campaign suite passes with the declared lease's ancestor check taken back out -- the
  direction the brief reported is pinned by none of its five tests, so that test was added by
  hand. The critic ran 5 min 19 s. Streak 0. Two runs in a row whose tests pinned a helper or a
  branch rather than the behaviour the brief named is what N22 was built for, and it landed
  (`53eb3551`): a /fix, /create or /implement turn that wrote Go now owes `/pinned` -- every function
  it changed, taken out on its own, must make a test it wrote fail -- with a forcing round before
  the verdict. R1-12 (N09 again) **failed** at the test gate, where it should: its naming change
  orphaned a package-level function and the package's own test caught it, the same class of
  regression R1-8 produced from this brief. The repair loop then never wrote: three attempts, 18
  model calls, 746.7k input tokens, 26 `recall_context` calls and no edit -- the round closes the
  read tools and its prompt carried the test runner's output but not the failing test (N24, fixed
  by hand). Streak 0. The gate now also asks what a change decides (`7f02f4d2`, N22b): every condition
  on a changed line held at a constant, the survivors recorded and handed to the model rather than
  charged -- five of eight survivors on a hand-written change were guards nothing could pin.
  R1-13 (N09 a third time) compiled and passed first time -- no repair round -- and its fix is the
  one the brief asked for, but the review found it takes two names away: a Go function beside a
  same-named method, and every Python or JavaScript method, become unfetchable. R1-14 was handed
  those two symptoms as its brief and fixed both, and **the pinning gate refused its first two
  attempts** until it wrote the round-trip test the review would have demanded. Neither landed:
  `go test ./...` afterwards failed in `internal/observation`, whose codesearch test pins how a hit
  inside a method is named -- **the test gate ran the packages the turn wrote and nothing else**
  (N25, fixed by hand `c4c097fc`: the packages that import them run too). The work is saved and the
  brief is run again with the gate's scope fixed. R1-15 ran it from scratch with that gate in place
  and **failed at the same assertion as R1-12**: the failing test's source was in every repair
  prompt (N24 held) and the model still hunted a broken extractor, because the one thing that
  explains the failure -- the rename in its own diff -- was never in front of it (**N26**, fixed by
  hand `ca7b31bf`: the build and test rounds carry the diff of the turn's writes). Streak 0.
  R1-16 (N09 a fourth time) got the fix almost whole and its own coverage test found a real bug in
  the helper it had just written (`goReceiverBaseType("func (a *pkg.A[T, U]) Close() error")` = `"U]"`,
  want `"A"`) -- and then **every remaining round told it "The tests pass"** above that FAIL trace,
  asked for more tests, and forbade the production fix the failure needed, so it weakened its own
  assertion to escape (**N27**, fixed by hand `2e48c308`: a round's prompt is chosen from what its
  recheck reported, and two of the seven rounds were already carrying an ad-hoc `testsBroke` boolean
  doing this by hand). R1-17 fixed the search projection the new importer gate had caught and
  **landed**; with R1-13 and R1-14 its work shipped as `e216f275`, the whole N09 fix written by
  codeNERD across three briefs with no hand edit -- three prompts, so no rung moves. R1-18 (doc 08's
  D2, the delimiter guard refusing valid edits) wrote **the better change and was refused for it**:
  it deleted the span-balance heuristic and leaned on the whole-file Go parser check already called
  on the next line, which meant deleting the two tests whose only subject was the deleted function,
  and the removed-tests gate cannot tell a dead test from a hidden one. Until that was fixed no turn
  could delete dead code through codeNERD at all. Hand-fixed: a removed test is released only when
  the same turn deleted a function that test names. The run also exposed the gate reporting
  `tests ok` over a four-minute timeout on six importer packages (`8f88be65`). Streak 0 -- and R1-18
  is the first failure that was entirely the harness's. R1-19 ran the same brief with that gate
  fixed and produced a competent change to **a defect that does not exist**: D2 claimed the
  delimiter guard refused valid edits and asserted their output "parses and compiles", which was
  never measured. Net delimiter balance is conserved, so refusing when the span's net changes is
  refusing exactly the edits that unbalance the file -- the guard was correct by construction, and
  every cited refusal would have broken the file. The run's change is kept on its merits (an
  already-unbalanced file can now be repaired; `}{` is caught by a stack where a net count passes
  it; all fifteen extensions keep their check) but it is **not a landing**: criterion 1 is that the
  run fixed the named symptom. The rule that comes out of it -- **a finding does not enter a brief
  until its falsifying check has been run**; for a refusal, apply the edit and see whether the
  result is valid.
- landed since R1-2: the forcing gate (`fd3c1d99`: changed code no test executes, and `go vet`
  findings in the turn's own files, are verdict evidence with a repair round first -- R1-2 had
  passed as done over both); two more load flakes (`10223378`: the watcher debounce test, and the
  boot test now names whatever still holds the workspace after `Close` instead of failing only in
  TempDir's cleanup); the removed-tests guard's repair round (`279b73fe`: a turn that deleted
  tests is handed each one's source to put back and the tests are rerun, where it used to fail on
  the spot -- R1-4's blocker); a forcing round that gives up with the suite red is undone to its
  last green state, the vet round keeps the tests green, and the final check names what failed
  (`27a4f0db` -- R1-4b's blocker); a replayed encrypted think is measured by the reasoning tokens
  the provider counted for it, not its ciphertext's length (`d54424d0` -- R1-4c's blocker)
- open: the external audit of 2026-09-19 -- 23 findings, verified against the code and routed in
  `07-external-audit-2026-09-19.md` (F7 `247a2402` and F6 `ac5c9b03` landed; F2 and F4 are C3/C4
  below); the harness defects below (C1-C4, V1); G6 needs a toolchain download
  (asked); codeNERD cannot run the gates it will be asked to clear -- no typed tool runs `go vet`,
  staticcheck or golangci-lint for the model (the forcing gate runs vet itself, the model cannot),
  and `nerd fix --acceptance` takes failing tests, not a gate. G4/G5 items are therefore not rung-1
  probes (criterion 5), and R5 (clear a class of findings) and R7 (measure the gates, pick the
  next item) need that tool first.

### Harness defects to fix before R2

C1-C5 from the R1-2 campaign comparison (`campaign_1284b6bb`); V1 found while fixing C5:

- **C1 targets fixed before research.** Every write set was decided when the plan was made, before
  phase 0's research ran. Phase 2's task targets `internal/cli/check-mangle.go`, a path that does
  not exist; the command lives in `cmd/nerd/cmd_mangle_check.go`, which phase 0 found. Nothing
  carries research results back into the tasks that follow it. **Landed `32f4d9c4`**: the refinement
  between phases is shown each completed task's result and each upcoming task's ID and write set,
  every path marked `exists` or `ABSENT`; the instruction to retarget by ID is the replanning atom's.
- **C2 a missing modify target becomes a create.** `reconcileTaskTypeWithWriteSet`
  (`internal/campaign/decomposer_planning.go`) retypes a `/file_modify` whose write set does not
  exist to `/file_create` (planned `/file_modify`, executed `/file_create`), so a wrong guess turns
  into a new file at the wrong path instead of a plan error. **Landed `8b007804`**: no retype; the
  attempt is told its planned target is absent, and a pre-existing file its attempt changed
  satisfies it (the validator reads C4's attempt record); creating the guess satisfies nothing, and
  the refusal says so to the retry.
- **C3 a retry drops its reason.** The retry re-spawned the same create task without the failure
  that stopped it, and the create-only fallback refused because the file now existed.
- **C4 rollback covers the declared write set, not the attempt's writes.** It removed the created
  file and kept the coder's edit to `cmd_mangle_check.go` -- outside the set -- which imported it,
  so the tree stopped building. `TestRollback_KeepsFilesOutsideTheWriteSet` pins today's
  behaviour; a fix has to tell the attempt's own writes from anyone else's. The campaign cannot
  today: it spawns through `TaskExecutor.Execute`, which returns the shard's prose, while the
  typed return (`observation.Return`, on `session.TaskResult.Observed`) already carries the
  attempt's write set (`Changed`), its own error (`Failure`) and the kernel's verdict (`Outcome`).
  C3 and C4 share that root: the typed result (S1) stops one seam short of the campaign.
- **C5 a gated coder turn has a way around its gate.** When the coder shard fails a `/file_create`
  task for any reason but a cancel -- including the test obligation ("hollow success blocked: turn
  created Go source ... without a test file") -- `executeFileTaskFallback`
  (`internal/campaign/orchestrator_task_handlers.go`) asks the model to "Generate the following
  file" and writes the answer through the VirtualStore: permission and write validation apply, the
  turn's obligations (tests, coverage, vet, verdict) do not. In R1-2 it was refused only because the
  blocked turn had already written the file; C2's retype is what made it reachable. The completion
  gate's reach -- hand-built, not a codeNERD brief. Fixing it found the bypass latent: the fallback
  asserted a `pending_action` payload (`{"content_bytes":N}`) the kernel's permission check, which
  matches the exact routed payload, never matches, so every fallback write was refused -- the
  document deliverables it exists for (F-DOC-1) included -- after the model had generated them.
  Correcting the payload alone would have opened the bypass; the fix lands both: documents only,
  the canonical payload, and every refusal leading with the coder's reason.
- **V1 the VirtualStore does not write what it is given.** `handleWriteFile` passes every
  `write_file` through `extractCodeBlockForFile`, which trims the content and, for a Go file,
  drops everything before the first `package ` -- a `//go:build` line or a licence header. The
  store's own `FileWriteValidator` then reports "content hash mismatch" for any content that ends
  in a newline (measured: the campaign's degraded-generation placeholder). The coder's tool loop
  writes through `internal/tools/core` and is unaffected; the store's route serves the campaign
  fallback, the system router, `cmd_instruction.go`, `pending_action.go` and the interactive gate.
  A multi-file fix (the handler and whatever its callers rely on it to strip): an R2 candidate for
  codeNERD. **Worse than first measured (2026-09-19 15:25, probed on `727e8ddd`):** the helper looks
  for any bare fence first, whatever the file type, so a Go file whose package comment shows a
  fenced example is written as the example's inside alone (`//\tx := 1\n//` -- the package clause
  and every declaration gone), and a Markdown document with an untagged fence is written as that
  block's inside. The first R2 brief (scratchpad `brief_v1_*`).

## What counts as a landing

A landing is one prompt to codeNERD (`nerd fix "<brief>"`, or one `nerd chat` request) on a real
problem in this repository, whose result is kept. All of these hold:

1. **The brief is a symptom, not a diagnosis.** It quotes what was observed (an error, a log line,
   a failing check, a measured cost) and what should hold instead. It does not name the cause or,
   where the symptom allows, the file.
2. **codeNERD alone edits the tree.** No hand edit to the diff before it is kept. A trivial
   post-edit (formatting, a comment) is recorded as such and caps the landing at "assisted".
3. **The change is proven.** Tests that fail before and pass after (the reviewer checks the
   "fail before" by reverting the fix), and `go build ./...`, `go vet ./...` and `go test ./...`
   green after. Since `53eb3551` the harness checks "fail before" itself, one changed function at
   a time (N22); the reviewer still reverts the change as a whole, and looks for the behaviour a
   function-shaped check cannot see (N22b).
4. **The verdict is truthful.** codeNERD's own result says done only when its evidence shows it:
   no "done" over a test that never ran, no success line over a failed gate.
5. **The failure is reproducible with codeNERD's own tools** (a command it may run), or the brief
   carries an acceptance contract. A failure only the reviewer can reproduce (R1-1 needed 32 busy
   loops) cannot be told red from green inside the run, so it is not a rung-1 probe.
6. **The file count is the fix's, not the noise's.** Files edited to fix the problem, tests
   included, count; generated files and formatting churn do not.
7. **The review finds no defect the change introduces** (added 2026-09-19 07:20, before R1-4's
   rerun). The reviewer measures the change beyond its own tests -- planted errors, the inputs next
   to the ones the tests use -- and a behaviour the change makes worse than before fails the
   landing even when every test and gate is green. Nits (style, a missed reuse) are recorded and
   do not. R1-3 met it; R1-4's fix would not have: a malformed Decl planted in `task_stage.mg` was
   reported in that file alone before (measured, beside the three rejections the brief is about)
   and in 84 files after, none of them naming it.

Every run, landed or not, is recorded in the dogfood ledger
(`.claude/skills/codenerd-dogfood/references/component-ledger.md`) with the brief, minutes, tool
calls, tokens, files, what landed and what it missed. A run that fails for a reason in the
harness (a tool that refuses a correct edit, context that never reached the model, a verdict that
lies) is followed by fixing that reason, test first, and the same brief is run again: the blocker
is the finding.

## The rungs

| Rung | Pass when | Why it is on the ladder |
|---|---|---|
| R0 Stable ground | the gates G1-G3 are green on `main` for three suite runs in a row, and a run's verdict matches its evidence | nothing above it can be measured on sand |
| R1 One file | three consecutive landings, each fixing a real problem whose fix is one file (plus its test) | the brief shape works when the cause is local |
| R2 Two files | three consecutive landings where the fix spans two files (a producer and its consumer, a rule and its Go assertor) | the agent follows a cause across a seam |
| R3 Five files | two landings of five-file fixes or features | planned multi-step work, context served per step |
| R4 Ten files | two landings of ten-file changes (an API migration, a cross-cutting fix) carried out through CodeDOM multi-file edits, not hand-edited file by file | a change with a blast radius is executed by a tool | **1 of 2 landed 2026-09-20** (`231cfa7c`, 11 files, assisted) |
| R5 Fifty files | one landing of a fifty-file change from a single prompt: an optimisation quest or a sweep that clears a whole class of gate findings | long-horizon work without drift | **PASSED 2026-09-20** (`540e15a5`, 189 files, 189/189 correct) |
| R6 The architecture docs | codeNERD rewrites every document in `Docs/architecture` from the code (never from the old docs), every claim citing a path and symbol that exist, links and symbols verified by a checker, and the reviewer, reading a sample of each package's docs against the code, finds nothing false and nothing important missing | the harness can hold a whole codebase in view and write the truth about it |
| R7 Recursion | `nerd campaign recurse` (or its successor) runs unattended for N cycles: measures the gates, picks the next failing item, fixes it, proves it, and each cycle leaves the gates strictly better and nothing worse | it improves itself |

Rungs are climbed in order; a later rung's run may happen earlier as a probe, but it does not
count until the rungs below it are passed.

## Choosing rung material (2026-09-20)

A rung needs work that is real, determinate, and multi-file by nature. The first
vehicle tried for R2 was `undeclared_asserts.txt` -- Go asserting a predicate no
`.mg` declares. It is the wrong material above R1, and the reason is worth keeping.

Every entry on that list resolves to a judgment, not an edit: declare it, delete the
assert, or wire a consumer. Five candidates were examined before briefing and all
five were design decisions, three of them gate defects rather than code defects:

| candidate | what it actually was |
|---|---|
| `turn_references_symbol` | asserted only by `internal/testing/context_harness`; a test harness writing into a production baseline |
| `success_pattern`, `failure_pattern`, `correction_pattern` | `FactPredicate:` on `types.ShardLearning` -- a LearningStore row label that `core/dream_router.go` says "has no Decl anywhere and must not get one" |
| `atom_depends` | the Mangle route for a capability Go already implements in `prompt/resolver.go`; deleting it moves away from the north star, wiring it is a redesign |
| `atom_exclusive` | plumbed through six files and enforced by nothing -- and declared by 0 of 351 atoms, so there is no symptom to brief |

Briefing any of them would have produced R2-2's outcome or worse: the `success_pattern`
brief would have instructed codeNERD to declare three predicates the design forbids
declaring. The list is a fine R1 vehicle and a bad R2+ one.

`starved_predicates.txt` is the opposite shape and is the vehicle from here. A starved
predicate already has its consumer -- a rule reads it -- so "does anything want this?"
is settled and only the producer is in question, which the rule body specifies exactly.
That makes the fix determinate and naturally two-sided: a Go assertor and the rule that
reads it, which is what R2 is defined as.

Three gate fixes landed first, because the scoreboard was wrong in three ways
(`64e7ab7a`, `a6a4afc3`, `77790de3`): test support counted as production Go in all four
corpus gates, the undeclared scan read `FactPredicate:` as a kernel assert, and the
guidance promised that declaring without a consumer "moves it to the starved-predicate
list" -- which cannot happen, and is what made R2-2's Decl-only change look complete.
82 entries -> 72, and one genuinely starved atom (`pytest_failure /assertion`) surfaced
that a coincidental string in a test simulator's keyword list had been silencing.

## Picking a starved predicate: 29 of 61 lead anywhere (2026-09-20)

Choosing R2 targets one at a time stopped working. Seven candidates were verified before
briefing and seven dissolved -- not because the predicate was wrongly listed, but because
what it feeds is itself read by nothing. `file_line_count` is the clean example: it is
genuinely starved, its consumer `long_file_warning` is genuinely derived, and
`long_file_warning` appears in exactly one place in the repository, the rule that derives
it. Fixing the producer would build a chain to nowhere.

So the question was measured instead (`scripts/starved_reachability.py` -- `scripts/` is
gitignored here, so the script is in the tree but untracked and this section is its record):
walk forward from
each starved predicate through rule body -> rule head, and ask whether anything reachable is
queried by production Go.

| | |
|---|---|
| starved predicates | 61 |
| reach a predicate Go queries | **29** |
| reach nothing Go reads | 32 |

Only the 29 are rung material; the 32 are a different question (whether the subsystem should
exist at all), and answering it by wiring a producer would be the worst kind of progress.

Two cautions from building it. The first version reported 0 of 61, which was the script and
not the corpus -- Python's `str.partition` returns `(before, sep, after)`, so the rule body
was bound to the separator and every rule looked bodiless. A measurement that says
"everything is dead" deserves the same suspicion as one that says "everything is fine".
The second: reachability says a chain ENDS somewhere Go reads, not that fixing one link
makes that endpoint fire -- every other link has to hold too.

What it surfaced, and the R2 target that came out of it: `turn_age_category`.
`internal/context/compressor_metrics.go:722` and `:732` query `should_mask_observation` and
`should_preserve_reasoning` on every compression. Both derive from `turn_age_category`
(`policy/context_compilation.mg:135-145`), under a header saying these rules "replace
LLM-based summarization with kernel-derived masking decisions", and
`schemas_context.mg:54-58` says the fact is "asserted by Go from the compressor's turn
tracking". Nothing asserts it. So no turn is ever masked, and the invariant the second query
exists to enforce -- "we mask observations, never reasoning" -- is checked against an empty
set every time. A live consumer, a documented producer that does not exist, and a plain Go
assert as the fix: the shape R2 is defined as.

Also visible in the 29: `coverage_goal` and `coverage_metric` reach `block_commit`, and
`element_action` reaches `blocked_action`. Those are the commit gate and the permission
surface, which stay hand-built.

## R6 scoped (2026-09-20)

Measured before briefing, so the rung has a target rather than an adjective:

| measure | value |
|---|---|
| real Go packages under `internal/` + `cmd/` | 91 |
| directories under `Docs/architecture` | 41 (40 real packages + `_rebuild`) |
| markdown files | 929, ~4.8 MB |
| docs citing at least one repo path | 379 |
| **docs citing no path at all** | **550 (59%)** |
| path citations | 3,576 |
| dead citations | 141 (3.9%), 20 distinct paths |
| line numbers past end of file | 0 |

It refutes "the citations are wrong": 96.1% of cited paths exist and no cited line
number is past its file's end. The largest single error is stale rather than invented --
`cmd/nerd/chat/session_boot.go` is cited 65 times and was deleted by `5bcd12f8`
("delete the dead legacy boot path"), so the docs describe a boot path the repo removed
on purpose. One citation is `internal/foo.go`.

### 189 of the 929 files are forwarding stubs

The dominant finding, and it is a standing-rule violation rather than a quality
judgement. The 2026-07-13 rebuild renamed the package-suffixed files
(`02-CURRENT-STATE-WORLD.md` -> `02-CURRENT-STATE.md`,
`03-GAP-ANALYSIS-CONTEXT.md` -> `03-GAP-ANALYSIS.md`) and dropped five slots
(`01-DOMAIN-MODEL`, `04-INVARIANTS-AND-GATES`, `05-CROSS-SYSTEM-WIRING`,
`06-TESTING-STRATEGY`, `08-FAILURE-MODES`) -- and left every original file in place as
a redirect. 189 files across 29 packages, 20% of the corpus by file count and 0.6% by
bytes (29 KB of 4.6 MB), whose entire content is a pointer somewhere else:

    # Redirect
    This filename is a **legacy stub**. Use:
    - [01-VISION.md](01-VISION.md)
    ...
    Rebuilt 2026-07-13.

That is the pattern the architect's 2026-09-01 ruling names item by item: no
"forwarding stub, exec wrapper, re-export, alias, compat path, deprecated-but-kept
entry, or 'old path still works' note", because it "lets two truths coexist so the next
reader cannot tell which one is live". The heading is not even consistent across them --
"# Moved", "# Redirect", "# SUPERSEDED", "# Superseded", "**legacy stub**" -- because
each was written by a separate per-package agent, which is the vertical generation the
north star warns about, visible in the shims themselves.

They are also entirely orphaned: **no markdown link anywhere in the repository resolves
to any of them**, inside `Docs/architecture` or outside it. (A first count said 20 live
docs linked to `01-DOMAIN-MODEL`; that was a grep for the string, which the stubs' own
bodies contain -- corrected by resolving links to files.) So the whole set can be
deleted with nothing to repoint, which is what makes it the R5 vehicle: 189 files from
one prompt, a determinate membership test, and the only real risk being over-reach onto
a genuine doc that happens to use the word "superseded". That risk is what the rung
tests, and the verification catches it: the corpus must lose exactly the files whose
entire body is a redirect, and `git diff --stat` must show no deletion of anything
larger than a notice.

### The rest of the corpus is unfalsifiable, not wrong

Excluding the stubs, quality splits sharply by template slot -- share of that slot's
files citing any repo path:

| slot | files | cite code | avg size |
|---|---|---|---|
| `IMPLEMENTED_SPEC.md` | 40 | 98% | 21.3 KB |
| `08-WIRING-AND-INTEGRATION.md` | 39 | 100% | 5.5 KB |
| `TODO.md` | 39 | 87% | 7.1 KB |
| `07-DEPENDENCY-MAP.md` | 38 | 84% | 3.6 KB |
| `02-CURRENT-STATE.md` | 39 | 82% | 6.5 KB |
| `06-PUBLIC-API-AND-TYPES.md` | 39 | 33% | 6.5 KB |
| **`05-INTERNAL-ARCHITECTURE.md`** | 39 | **28%** | 6.3 KB |
| **`09-SAFETY-AND-INVARIANTS.md`** | 39 | **28%** | 5.5 KB |
| `04-ARCHITECTURAL-PRINCIPLES.md` | 39 | 15% | 3.6 KB |
| `12-FAILURE-MODES.md` | 39 | 13% | 5.7 KB |
| `01-VISION.md` | 39 | 10% | 4.2 KB |

A VISION doc that cites no code is doing its job. A doc named INTERNAL-ARCHITECTURE
that cites no code in 72% of packages is 6 KB of prose about code it never points at,
and SAFETY-AND-INVARIANTS at the same rate is worse, because an invariant nobody can
locate cannot be checked. Those two slots, plus 12-FAILURE-MODES (a failure mode that
names no site is not a failure mode), are where the rewrite earns its keep.

Coverage is the other half: 41 of 91 packages have any doc at all.

This is why the corpus reads as substantial and cites as nothing, and why the standing
instruction never to use `Docs/architecture` as evidence is correct without being a
statement that its sentences are false.

R6's verification therefore has a deterministic half that needs no reviewer: every path
a doc cites resolves, every line number is within its file, no two files claim the same
numbered slot, and no package that exists is undocumented. The judgment half -- is the
claim true, is anything important missing -- stays with the reviewer, reading a sample
per package against the code. The checker is `doc_citation_check.py`; it belongs in the
tree before the rung is attempted, so the rung is scored rather than admired.

## The error-free gates

"Error free" means every gate at zero on `main`, each measured by a command anyone can run:

| Gate | Command | Baseline 2026-09-19 |
|---|---|---|
| G1 build | `go build ./...` | 0 errors |
| G2 vet | `go vet -tags sqlite_vec ./...` (every package) | 0 |
| G3 tests | `go test -count=1 ./...`, three consecutive green runs (no flakes; uncached, or a run re-reports old passes) | 88 of 89 on `f26140e1`: `TestRunToolLoop_ReservesTimeForFinalVerdict` fails 1 in ~6 runs alone ("compile working context: context deadline exceeded") |
| G4 staticcheck | `staticcheck -tags sqlite_vec ./...` | 103: 95 unused (U1000; 24 of them in `cmd/nerd/cmd_campaign.go`), 3 SA4000, 2 SA5011, 1 SA4023, 1 SA9003 |
| G5 golangci-lint | `golangci-lint run --build-tags sqlite_vec --max-issues-per-linter=0 --max-same-issues=0 ./...` (default linters; without the two caps it prints 169 and hides the rest) | 3,286: errcheck 1,851 (1,176 in tests), staticcheck 1,319 (1,190 are QF1012 `WriteString(fmt.Sprintf(...))`), unused 95, ineffassign 16, govet 5 (`reflect.Ptr`) |
| G6 vulnerabilities | `govulncheck -tags sqlite_vec ./...` | 11 reachable: 8 in the standard library (go1.26.4, fixed in 1.26.6), 2 in `google.golang.org/grpc` v1.81.1, 1 in `golang.org/x/text` v0.37.0 |
| G7 policy corpus | `nerd check-mangle internal/core/defaults/*.mg internal/core/defaults/policy/*.mg internal/core/defaults/schema/*.mg` | 3 of 135 files: each uses a predicate declared in a sibling file (`reviewer.mg`, `policy/task_stage.mg`, `policy/schemas_perception_latency.mg`) that the checker does not preload, though the kernel loads them together -- the checker disagrees with the kernel. 0 wildcard negations in the corpus |
| G8 atom corpus | Mangle examples in `internal/prompt/atoms` that fail to load unmarked (`TestAtomCorpus_MangleExamplesTheEngineRejectsDoNotGrow`) | 344 at `f26140e1` (888 on 2026-09-18, 654 after `86e461f2`) |
| G9 the corpus guards | `go test ./internal/prompt/ -run TestEmbeddedCorpus` | green |
| G10 formatting | `go test ./internal/build/ -run TestRepository_EveryGoFileIsGofmtClean` (every tracked Go file, go/format) | 28 of 2,542 on `df85abe6`; 0 after `08419f76` |

A gate that is not zero is a backlog of real problems: it is where R1-R5 briefs come from.
Sized to the rungs, the backlog on `f26140e1` offers: single-file items (the final-verdict flake,
the checker's preload, SA2001's empty critical section, the SA5011 test that dereferences after
`t.Error`); a five-file class (govet's `reflect.Ptr`, SA1012's nil contexts); a ten-file class
(ST1005's 20 capitalised error strings); and the fifty-file quest (QF1012's 1,190 sites, a change a
tool should carry out). The 95 unused functions are not a deletion list: each is first audited
for the wiring it was meant to have (repo contract).

## Which entry point for which rung

| Entry point | What it is | Used for |
|---|---|---|
| `nerd fix "<brief>"` | one turn: the executor's tool loop, with planned steps compiled per step | R1, and as the baseline an R2 campaign is compared against |
| `nerd campaign start "<goal>" --type remediation\|feature\|migration` | a short campaign: the goal decomposed into phases and tasks, each verified before the next | R2-R5, R6 (one campaign per package's docs) |
| `nerd campaign recurse --waves N --subsystem S --angles A` | waves of campaigns over the subsystem DAG, each wave led by what failed last | R7 |

Why both: a single fix turn is the shortest loop to measure, but a change that spans files is what
a campaign exists for (decompose, execute, verify per phase). Where a brief could go either way,
it is run both ways and the ledger records which landed and at what cost.

## Protocol for one run

1. Pick a real problem from a gate's backlog or from the study's seams, sized to the rung.
2. Write the brief: the symptom, the evidence, what should hold. Save it as a file.
3. Rebuild `nerd.exe` from `main` (the prompt atoms are embedded: a corpus fix only reaches the
   agent after a rebuild). Nothing else edits the tree while the run is in flight.
4. Run the entry point for the rung and log it (minutes, tool calls, model calls and tokens from
   `.nerd/logs`; for a campaign, `nerd campaign journal` and `status`). While it runs, follow
   every WARN and ERROR in every log category, not the one file the run's headline lives in:
   `python scripts/nerd_logwatch.py [poll_seconds]` (`scripts/` is gitignored; the tool is
   recorded here). It follows each `*.log` touched in the last 15 minutes whatever process
   wrote it, starts a pre-existing file at its end and a new one at byte 0, and prints a message
   shape the 1st, 10th, 100th and 1000th time so a retry storm cannot bury a new failure. A run's
   summary line is not evidence of its health: on 2026-09-21 a run reported as "planning" was on
   the retry after two 2-minute LLM timeouts, and the timeouts were only in the api log.
   After it, `python scripts/nerd_cache_report.py [YYYY-MM-DD|all]` reads the broker's receipts
   (`.nerd/meter/receipts.jsonl`): input, cached and output tokens by purpose, the cached share
   of input on tool-loop follow-up rounds split by whether the cacheable prefix changed, and
   input per round for the largest sessions. Baseline 2026-09-21, 44.3M input in 974 calls:
   49% cached; the prefix changed on 767 of 854 follow-up rounds (47% cached there, 83% where it
   held), because the per-round working section sat in the system prompt ahead of every message.
   A campaign whose goal has a deterministic check carries it: `--accept "<argv>"` (exit 0 is the
   only pass; a failure appends a remediation phase briefed with the command's output; three
   failed rounds block the campaign `/acceptance_failed`). For R6 it is
   `--accept "python scripts/r6_structcheck.py <pkg>"`. A campaign with no witness can still
   report success over failing checks (P8, 2026-09-21), so its "completed" is not a landing.
5. Review the diff against the landing criteria; keep or revert; run the suite; commit with
   "via nerd fix" and the brief's name.
6. Ledger entry. If the harness blocked it: fix the blocker test-first and rerun the brief.

## Runs

(Newest last.)

| date | rung | brief | entry | outcome | minutes | tool calls | files | commit |
|---|---|---|---|---|---|---|---|---|
| 2026-09-19 02:26 | R1 | R1-1 final-verdict flake | nerd fix | not landed: `working_stop(/read_only_stall)` after 24 rounds, nothing written. Harness blocker: the working window dropped every file but the last one read (fixed by hand, `7856287c`) | 6.1 | 27 | 0 | -- |
| 2026-09-19 02:50 | R1 | R1-1 again, fixed window | nerd fix | not landed, reverted: `/done` over a change that still fails 40 of 40 under load; the runtime edit broke tool-use/result pairing; its test edit (an assertion drop) was refused and never redone. The failure is load-dependent and codeNERD's tools cannot reproduce it -- a poor rung-1 probe; fixed by hand for R0 | 15.6 | 46 | 2 | reverted |
| 2026-09-19 03:09 | R1 | R1-2 check-mangle in kernel context | nerd fix | not landed, reverted: the corpus passes (135/135) and real errors still fail, but `go vet` unreachable code, an unused function, every corpus error misattributed to every file, and no test. Its `run_build` replaced the running nerd.exe (became R1-3) | 25.3 | 108 | 2 | reverted |
| 2026-09-19 03:38 | R1 | R1-2, same brief | campaign `--type remediation` | not landed: timed out at 60 min in phase 3 of 4 with the tree not building. Plan targets guessed before research, modify retyped to create, a retry without its reason, a rollback scoped to declared targets | 60.1 | -- | 1 + a doc | reverted |
| 2026-09-19 04:41 | R1 | R1-3 run_build leaves artifacts | nerd fix | **landed**: fix + a test that fails before and passes after; found and fixed its own regression from a full-suite run | 23.9 | 52 | 2 | `df4a3063` |
| 2026-09-19 06:37 | R1 | R1-2 again (R1-4), forcing-gate binary | nerd fix | not landed, reverted: every gate held -- build, tests, the new coverage round (converged in two attempts), vet -- then the removed-tests guard refused the turn: a whole-file `write_file` of the test file dropped three existing tests that still pass against the new code. The guard was the one gate with no round (fixed by hand, `279b73fe`). The fix: corpus 135/135 and rule errors attributed right, but one malformed Decl cascades into 84 misattributed errors | 14.2 | 63 | 2 | reverted |
| 2026-09-19 07:22 | R1 | R1-2 again (R1-4b), removed-tests round in | nerd fix | not landed, reverted: build and tests green, then the coverage round's own test failed three times (it counted a marker four schema files also contain) and the round left it in place; the final check failed the turn without naming why. Fixed by hand: a red give-up is undone, the final check names what failed. The fix (whole corpus in one program) blames all 135 files for one sibling's error -- criterion 7; brief v2 states that property | 20.6 | 49 | 2 | reverted |
| 2026-09-19 08:02 | R1 | check-mangle v2 (R1-4c) | nerd fix | not landed, reverted: after a 22,853-token think the broker counted the replayed encrypted reasoning by its ciphertext length and refused the next request as window_exceeded (193,735 counted; the day's think-then-request pairs cost a fraction of their estimate). Fixed by hand: a redacted think is measured by the reasoning tokens the provider counted. The unfinished fix (the disk file with the embedded corpus) met v2's property | 12.8 | 40 | 1 + a repro | reverted |
| 2026-09-19 08:31 | R1 | check-mangle v2 (R1-4d) | nerd fix | not landed, reverted: build, tests and vet green, six changed blocks executed by no test; the coverage round's second model call ran 368 s and the repair episode's 6.2-minute clock cut it with nothing returned, then `nerd fix`'s 25-minute default cut the critic. `/unverified`, and honestly so. Fixed by hand: no run-level clock anywhere (repair episode, CLI, `llm_timeouts`, campaigns). The fix (each policy file's Decls preloaded one by one) met v2's property but silently drops a Decl with a trailing comment (latent: `chaos.mg:40`, `:84`) and had no test | 25.1 | 77 | 1 | reverted |
| 2026-09-19 13:51 | R1 | check-mangle v2 (R1-4e) | nerd fix | not landed, reverted: build, tests and vet green; the coverage round's own test failed -- the per-file preload dropped `reviewer.mg` (it uses a predicate a later file declares), so a brief file still failed. The model moved to a one-fragment load with the per-file loop as fallback; the round gave up with five defensive error branches uncovered, `/unverified` and saying so. Review: a sibling with a planted error still fails `intent_routing_rules.mg` through the fallback; 55 s and 482 warning lines on a clean corpus (HEAD 12.2 s). No harness blocker; the coverage prompt's ban on removing unreachable lines it named is recorded | 19.5 | 30 | 2 | reverted |
| 2026-09-19 14:28 | R1 | apply_edits partial write (R1-5, N04) | nerd fix | **landed, assisted**: the failed file is put back and confirmed, or named; five fault-injection tests, all failing at HEAD; build, vet, tests green; review probes (first file, last of three, the restore failing, a write that failed after writing everything) pass. One doubled blank line removed by hand -- the coverage round's writes are never gofmt'd (N18) | 17.0 | 29 | 2 | 3d9ba680 |
| 2026-09-19 14:55 | R1 | edit_lines delimiters inside a raw string (R1-6, N10) | nerd fix | not landed, reverted: `/done`, build, tests and the suite green; the brief's two edits pass and an edit in a block comment now does. Review: the prefix scan does not know a language's literals -- below a JS regex holding a quote or a Rust lifetime, an edit that drops a closing brace is accepted (HEAD refuses it). The critic found nothing | 18.0 | 38 | 2 | reverted |
| 2026-09-19 15:21 | R1 | a forcing round's Go never formatted (R1-7, N18) | nerd fix | **landed**: a second format pass at the end of the turn, before the closure re-measures the gates; a whole-turn test through ProcessWithIntent, failing at HEAD; build, vet, suite green. Nits: the early pass stays, its comment's "once" is stale | 19.7 | 57 | 2 | 24e9cc56 |
| 2026-09-19 15:50 | R1 | get_element shared method names (R1-8, N09) | nerd fix | not landed, reverted: `/unverified` (12 changed blocks no test executes; the coverage round gave up) and true. Review: `A.Close`/`B.Close` reach their elements and a bare name over two receivers is refused; but a function `Close` beside a method `A.Close` is reached by no name (HEAD reached it), and Python/JS methods sharing a name are reached by none. The critic was cut at 3 minutes | 27.0 | 56 | 4 | reverted |
| 2026-09-19 16:19 | R1 | the commit regime's sentence sent twice (R1-9, N17) | nerd fix | failed: `working_stop(/read_only_stall)`, 24 rounds, nothing written. It located the fix and read the harnesses; reading closed with its last read on `working_meter.go`, and eleven recalls of `build_verify.go`, `repair_loop.go` and neighbours each rendered `working_meter.go`'s context. Harness blocker N21, fixed `51e86c27` | 4.5 | 35 | 0 | -- |
| 2026-09-19 16:34 | R1 | the commit regime's sentence sent twice, again (R1-10, N17) | nerd fix | **landed, assisted**: an idempotent append at both call sites; `/done`, suite green; its five tests pass with the fix reverted at the call sites, so the brief's scenario test was added by hand (fails with them reverted). Three files: not an R1 fix | 24.8 | 78 | 4 | 2c373714 |
| 2026-09-19 17:07 | R1 | overlapping write-set leases, a directory and a path under it (R1-11, L3) | nerd fix | **landed, assisted**: the `ancestors` parameter deleted and a descendant check added, so both the declared lease and the write-time check refuse an overlap either way; `/done`, suite green, right on all 15 probed shapes. Its five tests pin only the direction it found uncovered -- the suite passes with the reported direction's check removed -- so that test was added by hand | 19.2 | 33 | 2 | aae24670 |
| 2026-09-19 17:49 | R1 | get_element shared method names, again (R1-12, N09) | nerd fix | **failed**: the naming change orphaned a package-level function (`ForbidsPath was not extracted`), caught by the package's own test; the repair loop then never wrote -- 3 attempts, 18 model calls, 746.7k input tokens, 26 recall_context calls, no edit (N24, fixed by hand). Reverted | 11.7 | 55 | 0 | -- |
| 2026-09-19 18:27 | R1 | get_element shared method names, third time (R1-13, N09) | nerd fix | **not landed**: the qualified names the brief asked for, first time with no repair round; the review found a Go function beside a same-named method and every Python/JS method unfetchable. The pinning gate's first production run: 21 declarations pinned, 12 decisions recorded advisory -- two of them the branches the defects live in | 42.3 | 127 | 3 | -- |
| 2026-09-19 19:16 | R1 | the two names R1-13 took away (R1-14) | nerd fix | **not landed**: both fixed and 15 probe shapes hold, but `go test ./...` fails in internal/observation -- a contract the gate never ran (N25). The pinning round refused two attempts and forced the round-trip test | 18.3 | ~60 | 3 | -- |
| -- | R1/R2 | R1-15 through R1-19, R2-1, R2-2 | -- | rows not written at the time; per-run detail is in the dogfood ledger. Recorded here rather than reconstructed, because guessing a run's minutes and tool calls to fill a table is how a measurement becomes a story | -- | -- | -- | -- |
| 2026-09-20 00:35 | R2 | `turn_references_symbol` undeclared (R2-2) | nerd fix | **not landed**, reverted: `/done`, every gate green, 2 files -- and the change was a `Decl` and nothing else, against a brief that required "dropped or declared AND given a consumer". The baseline's own guidance invited it by promising that declaring alone "moves it to the starved-predicate list", which cannot happen (fixed `a6a4afc3`). Its target has since left the list anyway under `64e7ab7a`: the predicate was asserted only by `internal/testing` | 13 | -- | 2 | reverted |
| 2026-09-20 01:44 | R2 | target size/complexity, four starved predicates (R2-3) | nerd fix | **not landed**, reverted: `/unverified` and honestly so -- no tests, baseline not regenerated. The code is good (a new `virtual_store_target.go`, named thresholds, the complex bar matched to `intent_routing_rules.mg`'s existing `N > 50`) and **inert**: it registers four external handlers against plain Decls, and `kernel_eval.go` keeps a callback only when `decl.IsExternal()`. It cannot be completed as designed either -- marking the Decls `external()` sends a bound `Target` into `arg.(ast.Constant)` (topdown.go:99) and panics. The gate said "4 predicate(s) now have a producer"; fixed `52b7c5f1` | 67.2 | ~70 | 2 | reverted |
| 2026-09-20 12:05 | R5 | 189 forwarding stubs in Docs/architecture (R5-1) | nerd fix | **failed**: refused on the first file -- `delete_file` hits `requires_permission(/delete_file)` and cannot self-authorize. Worked around it by emptying files to zero lines, which is worse than the stub; said so honestly. Three verbatim examples in the brief produced a literal three-step plan. Blocker fixed `93504014` (architect chose: recoverable deletes self-authorize) | 10.0 | 3 | 0 | -- |
| 2026-09-20 12:09 | R5 | same brief, delete gate open (R5-2) | nerd fix | **failed**: `working_stop(/repeated_cycle)` after 3 calls. grep returned exactly its cap of 100 for a class of 189 and said nothing, so the model -- which had already raised max_results -- re-ran the identical search and was stopped for it. Its pattern was correct. Blocker fixed `697b260e` | 4.0 | 3 | 0 | -- |
| 2026-09-20 12:24 | R5 | same brief, cap announced (R5-3) | nerd fix | **not landed**, reverted: 11 of 189 deleted, **11 of 11 correct**. 75 calls, one grep per delete; the commit regime closed the read tools after 35 and the remaining 177 became unfindable. Closed `/done` while asking to be re-invoked (N41) | 15.5 | 75 | 11 | reverted |
| 2026-09-20 12:42 | R5 | brief names the tool surface (R5-4) | nerd fix | **not landed**, reverted: 88 deleted, 85 correct and **3 real documents destroyed** -- `cli/` never got the 2026-07-13 rebuild, so its suffixed filenames are the live docs, and the class was matched by filename rather than content (N43). 183 reads preceded 88 deletes. Closed `/done` over 85 of 189 | 24.0 | ~290 | 88 | reverted |
| 2026-09-20 13:20 | R5 | brief states the CONTENT test (R5-5) | nerd fix | **LANDED** `540e15a5`: **189 of 189 correct, 0 over-deleted, 0 links left dangling** (14 existed at HEAD, unchanged). The three real documents R5-4 destroyed were all spared. One collateral edit and a right one: a test constant repointed off a deleted path. `/done` on checks_passed. The fix was the brief, not the harness -- name the content test (under 2 KB AND nothing left once heading and links go), say to apply it every time *including when the name matches one already removed*, and demand a report of candidates REJECTED | 34.0 | ~290 | 190 | `540e15a5` |
| 2026-09-20 15:40 | R4 | perception re-exports five articulation types (R4-1) | nerd fix | **LANDED, assisted** `231cfa7c`: 11 files across 3 packages, every call site repointed, shim gone, build and suite green. `/unverified` and rightly -- for a type alias no test CAN fail with the change taken out. Its own tests say it better: "an alias IS the canonical type ... the invariant is therefore a source invariant", checked at the source WITH a vacuity guard. Assisted by MY error: proving fail-before I `git checkout`'d transducer.go and destroyed its edit to that one file; reconstructed by hand | 75.0 | ~200 | 11 | `231cfa7c` |
| 2026-09-20 16:20 | R6 pilot | rewrite Docs/architecture/diff from the code (R6-1) | nerd fix | **LANDED** `04d33667`: 19 documents about 2 Go files became 3; 106 KB -> 9.6 KB. **57 citations, 0 dead, 0 line numbers past EOF; 26 of 26 symbol attributions correct**; dated and pinned to the commit it ran against. Four plausible-but-checkable claims verified by hand, all true, incl. a real dead counter (`Stats.Collisions` has no production writer) and a behavioural gap traced across two packages (a binary edit renders as nothing). Third file named WIRING-AND-NOT-BUILT.md -- the standard's hardest rule, adopted by name | 19.5 | ~250 | 21 | `04d33667` |
| 2026-09-21 01:33 | R6 pilot | nebulous brief, internal/context, the dream-plus-truth standard (R6-P1) | campaign `--type feature --docs` | **killed by hand at 66 min**: 9 of 24 tasks, 3 documents. C1/C3/C5 held at campaign scale. Measured: 166 raw filesystem calls against 14 get_elements; 5.5M input tokens for 45 KB; kernel evaluation 4,033 s summed against 2,065 s of LLM calls; refinement doubled a finished phase. Harness blockers fixed by hand: `7954a5d9` (cone evaluation), `5b439060` (structure index, five tools, `working_search_open`), `b2e38be9` (task-ID match, buildable tree) | 66 | 194 | 3 | not committed |
| 2026-09-21 04:03 | R1 | refinement doubled a phase (P4), symptom-only | nerd fix | **not landed**, reverted: found the cause from the symptoms (canonicalised the ID slash), then over-built, broke an existing dedupe test, and its third repair attempt left the repository not compiling. 6 structural queries, 0 grep. Hand fix `b2e38be9` | 17.1 | 20 | 2 | `b2e38be9` (hand) |
| 2026-09-21 04:36 | R6 pilot | the same nebulous brief on the rebuilt binary (R6-P2) | campaign `--type feature --docs` | **corpus landed `ea90cc63`, campaign exited blocked**: 30 of 30 tasks, 0 failed, 13 documents, ~190 KB; 132 structural queries, 0 grep; 0 bare citations in 02-CURRENT-STATE (151 before). Exit `blocked: /phase_unverified`: the headless `/manual_review` checkpoint never gets a well-formed `checkpoint_verdict/4` (P9). Six deterministic defects left, committed as found and briefed next (three files without front-matter, two superseded files kept, an ADR with no witness); a model-judged verify task passed over them (P8) | 95 | 416 | 14 | `ea90cc63` |
| 2026-09-21 06:12 | R3 | context corpus fails its structural grader, 6 problems (R3-1), symptom-only: the grader's output | nerd fix | **partial landing**: 7 files, 31 lines, every edit correct -- front-matter on the three files that lacked it, a witness that resolves (`TestEveryProtocolMemoryOpIsHandled`), the wrong pinned commit corrected where it touched. 6 problems -> 2. Missed: the two superseded files were never deleted (no `delete_file` call was made) and 12 references to them stand; six files keep the old pin. Verdict `/done` over a brief that said "zero problems", though its own report named what it had not verified | 15.3 | 62 | 7 | this commit |
| 2026-09-21 06:29 | R3 | the residue of R3-1: two superseded files and the references to them (R3-2), symptom-only | nerd fix | **landed, assisted**: both files deleted through `delete_file`, their surviving claims moved into the files whose question they answer, 9 of 12 references repointed, grader 6 -> 0 across the two runs. Assisted because 6 mentions stayed: 3 were in a file the brief never listed (my reference list was cut by `head -12`), and one sentence in IMPLEMENTED_SPEC said two documents did not exist that now do; fixed by hand, 4 edits. Its closing report told the user to delete the two files it had already deleted. One unscoped `run_tests` cost six of the 33 minutes (P10) | 32.8 | -- | 6 (+4 by hand) | this commit |
| 2026-09-21 07:03 | R2 | recall fails on every hydrate of a long-goal run (P1), symptom-only | nerd fix | **landed, assisted**, `/unverified` and rightly: it named the whole cause (`RecallSimilar` says semantic and calls the keyword path; `VectorRecallSemantic` already exists) and landed half of it -- the keyword query bounded to 32 distinct words, three test files, suite green -- then said in its own report that it had not made the reroute or the long-query regression test. 5 structural queries before raw search opened. The coverage round ran out of attempts on its own tests. Reroute, context threading through four call sites and the regression test (fails with the production error without its bound) by hand | 29.4 | -- | 4 (+2 by hand) | this commit |
| 2026-09-21 07:44 | R6 | nebulous brief, internal/diff (R6-D1) | campaign `--type feature --docs` | **not landed, reverted**: "Campaign completed successfully" in 22 min over output failing 17 deterministic checks -- a 5-task plan, invented filenames and doc-class values, 9 required slots skipped. The standard's "scale to the package" read as licence to drop what it calls non-optional. Output kept at `.nerd/dogfood/r6/pilot_diff_output/` | 22 | -- | 5 | not committed |
| 2026-09-21 08:09 | R6 | the same brief plus one sentence: "every slot the standard lists is required, whatever the package's size" (R6-D2) | campaign `--type feature --docs` | **corpus landed, campaign stopped by hand at 25 of 26 tasks**: the planner went from 5 tasks to 26; 16 documents under the right names, two capability specs, two ADRs; grader 16 -> 4 (two pre-rewrite files unlabelled, two ADRs without a witness: the same residue `context` left). Five phase checkpoints approved through the repaired verdict path (`78f8de1d`). The last task, a `/test_run`, failed three times on two repository gates I had broken that night (`75c90a12`), which no docs task could turn green: the campaign retried an identical failure instead of reporting one outside its write scope (P11) | 117 | -- | 16 | this commit |
| 2026-09-21 10:16 | R3 | internal/diff corpus, 4 structural problems, symptom-only: the grader's output, generated by `scripts/r6_loop.sh` (R3-3) | nerd fix x2 | **LANDED, unassisted**: round 1 took 4 -> 1 in 13.7 min (front-matter on the two pre-rewrite files, a witness on each ADR); the grader then refused ADR-001's witness `symbol:Engine.ComputeDiff` as unresolvable -- a fabricated symbol a model-judged review would have passed -- and round 2 replaced it with `symbol:ComputeDiff` plus `file:internal/diff/diff.go` in 3.5 min. 4 files, 0 hand edits, grader 0. The first 4 minutes of round 1 were two 2-minute LLM timeouts in step planning, which I had reported to Steve as a healthy planning call until he asked whether I was watching the logs directory | 17.2 | -- | 4 | this commit |
