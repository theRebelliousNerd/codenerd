# Idea corpus: code quality (lane: make the LLM write the best code it can)

Repo: `/home/user/codenerd` at `d61dbc6`. Every `path:line` below was read at that commit.
Jev facts are the briefing's (TypeSafe AI, released 2026-09-15; not independently verified here).
"Today" means the idea can ship with no Jev; where Jev is named, an LLM call can stand in until
Jev is available (slower, costlier, and uncalibrated).

Guiding constraint, from `README.md` Part III and `internal/verification/verifier.go:193-205`:
a model (LLM or Jev) may **classify, withhold, or oblige more work**. It may never grant a
permission and never mint `/done`. Each Jev-backed idea below is built to fit that
asymmetry. Thresholds go in `.mg` as `config_param`, never as Go literals
(`internal/core/defaults/executive_literals_test.go:207`).

---

## Top 7 by bang-for-buck

The ranking is by expected quality gain per unit of (token spend + effort), using evidence
from this repo's own ladder and dogfood runs. Q-22, the defect-replay bench, should land
before or with the first of these, because it is how any of them gets measured.

| # | id | title | expected quality effect | token cost | effort | needs Jev? | confidence |
|---|---|---|---|---|---|---|---|
| 1 | Q-10 | Survivor triage round: stop discarding the pin gate's surviving mutants on the green path | Catches the class of defect behind R1-13. There, the two surviving condition mutants sat on exactly the two lines where the review later found the regressions, and today nobody sees them | 0 when no survivors. + one targeted round only when a survivor is charged | S | n (Jev makes the guard/decision split cheap and calibrated) | high |
| 2 | Q-14 | Admission-against-interest audit of the final response | Kills "/done over work its own text says is unfinished" (N41: R5-3, R5-4, R3-1) | ≈0 with Jev (one Noul over the response). Today: one small LLM call | S | y (LLM fallback) | high |
| 3 | Q-01 | Requirement ledger: the verdict learns the scope of the request | `/done` requires per-requirement evidence. Closes the gap the external audit calls architectural ("no gate can see the scope of what was asked") | + small (one extraction, then N Noul questions over one state) | M | y (LLM fallback) | medium-high |
| 4 | Q-02 | Reproduce-first contract for `/fix` (a harness-frozen reproducer through `internal/evidence`) | Tests pin the brief's behaviour, not a helper (R1-10, R1-11), and it stops fixes to defects that do not exist (R1-19). Agentless-style reproduction tests are reported to raise SWE-bench resolves | 0/+ (moves the test the gates already demand to before the fix) | M | n | medium-high |
| 5 | Q-11 | Hunk-scoped, request-aware critic, with risk pre-screen and finding verification | Fewer off-target findings (R1-11: 3 of 4 were in untouched code), and it reviews against what was asked | − (diff hunks instead of up to 6 whole files × 24 KB; low-risk hunks skipped) | S-M | partial | medium |
| 6 | Q-17 | Recurse reflexion memory: a reverted attempt tells the next one why | Retries stop repeating the same failure. Today the loop gives up after two identical failures | + small (a bounded note per retry) | S | n | medium |
| 7 | Q-12 | Counterexample critic: findings as executable probes, run old vs new | The advisory critic becomes forcing without the risk of charging a hallucinated finding: a probe that fails on new code and passes on the preimage is a regression witness | 0/+ (probes replace prose) | M | n | medium |

Crossover with the token lane: Q-11, Q-20 and Q-04 save tokens. Q-08 spends them, and only
behind a gate.

---

## Where quality is lost today

Each item is a failure mode visible in code or in this repo's own records. The run IDs
(R1-x …) are from `Docs/journeys/05-elite-harness-ladder.md` and
`.claude/skills/codenerd-dogfood/references/component-ledger.md`.

1. **"Done" cannot see the scope of the request.** `turn_verified` reads only mechanical gates.
   It never reads the request (`internal/core/defaults/policy/coder_safety.mg:296-298`). The only
   arm that speaks for requested behaviour is `turn_acceptance`, and that needs a
   caller-supplied contract (`coder_safety.mg:298`, `internal/evidence/change.go:38-44`). The
   consequences are recorded runs:
   - R5-3 and R5-4 closed `/done` after 11/189 and 85/189 deletions (ladder:526-527).
   - R3-1 closed `/done` against a brief that said "zero problems" (ladder:534).
   - R2-2 closed `/done` with a `Decl` only, though the brief required a consumer (ladder:522).
   - R6-D1 reported "Campaign completed successfully" over output failing 17 deterministic
     checks (ladder:537).

   The external audit names the gap and leaves it open:
   `Docs/journeys/07-external-audit-2026-09-19.md:203-231`.
2. **Campaign requirements are linked to tasks by word overlap, and nothing reads the link.**
   `linkRequirementsToTasks` counts shared words longer than 3 characters and links a task at ≥2
   (`internal/campaign/decomposer.go:901-928`). `requirement_coverage` is declared
   (`internal/core/defaults/schemas_campaign.mg:294-296`), but no rule in the policy corpus
   consumes it. So an uncovered requirement blocks nothing.
3. **Tests pin a helper or a branch, not the behaviour asked for.**
   - R1-10: five tests pass with the fix reverted at its call sites (ladder:516).
   - R1-11: the reported direction is pinned by nothing (ladder:517).
   - The pin gate (N22) is a floor, not the reviewer (ledger:5024-5028).
4. **The strongest mechanical signal for hidden defects is thrown away on the green path.**
   Condition mutants that survive are recorded in `result.PinAdvisory`
   (`internal/session/pin_gate.go:580-592`). They reach the model only inside the pinning repair
   prompt (`pin_gate.go:782`), and that prompt runs only when pinning **failed**
   (`pin_gate.go:773`). The critic runs at round 3 and pinning at round 5
   (`internal/core/defaults/policy/turn_rounds.mg:33,35`), so the critic can never see them. In
   R1-13 every declaration pinned, 12 conditions survived, and two of them were "the branches the
   review's two defects live in" (ledger:5074-5078). The run closed `/done`.
5. **The critic is slow, off-target and request-blind.**
   - It is handed up to 6 whole files × 24 KB (`internal/session/critic.go:317,322,369-396`), plus
     removals and diagnostics.
   - It is never handed the request, the diff hunks, the tests or the callers
     (`critic.go:73-164`, `internal/session/build_verify.go:744`). `turnDiffSection` already
     exists for the repair rounds (`internal/session/turn_diff.go:79`).
   - It runs on the planner slot (`build_verify.go:907-914`) and takes 2-5 minutes, including
     2 min 50 s for "NO FINDINGS" (ledger:4784).
   - It reported 3 of 4 findings in code the change did not touch (ledger:5020-5023).
   - It missed the defects the human review found in R1-6 (ledger:4828), R1-8 and R1-13.
   - Its system prompt is a Go string constant (`build_verify.go:895-898`), not an atom.
6. **Regressions outside the tests are found only by the human reviewer.** The reviewer probes
   "the inputs next to the ones the tests use" (ladder:206-212, criterion 7). Examples:
   - R1-6 accepts brace-dropping edits below a JS regex holding a quote or a Rust lifetime, and
     HEAD refused them (ladder:512).
   - R1-13 made two element names unreachable that HEAD reached (ladder:519).

   No gate compares old and new behaviour on inputs the tests do not assert.
7. **Fixes are attempted without proving the symptom first.** R1-19 produced "a competent change
   to a defect that does not exist" (ladder:101-110). An acceptance contract *requires* a
   reproducer that fails before the edit (`internal/evidence/change.go:254-256`), but only a
   human can author one.
8. **Context is thin exactly where edits go wrong.**
   - Holographic callers are matched by substring: `strings.Contains(caller, sym)`
     (`internal/world/holographic.go:1151-1153`).
   - Callers are rendered as names only (`holographic.go:520-576`). The caller-body renderer
     `FormatWithPriorities` (`internal/world/holographic_impact.go:684`) has no production caller.
   - "Tests: yes/no" is a filename check (`holographic.go:1166-1178`), and `TestCoverage` is never
     set.
   - The LSP client can only initialize, open a file and wait for diagnostics
     (`internal/world/lsp/client.go:320-367`). There are no references and no call hierarchy.
   - R1-15 failed because "the one thing that explains the failure -- the rename in its own diff
     -- was never in front of it" (ladder:79-83).
9. **Workspace conventions are a generic template, not mined.**
   - The "project/go/conventions" atom is hardcoded prose ("Use channels for communication",
     "prefer 'i' over 'index'") (`internal/init/profile.go:343-376`).
   - `UserPreferences.TestStyle` and `ErrorHandling` are set only by CLI hints
     (`internal/init/initializer.go:197-200`, `internal/init/profile.go:150-175`).
   - The mandatory Go failure-mode atom still leads with "Loop variable capture (pre-Go 1.22)"
     in a Go 1.26 module (`internal/prompt/atoms/language/go_ai_failure_modes.yaml:8,26`, `go.mod:3`).
10. **Repair loops can burn tokens without a change of hypothesis.**
    - R1-12: 3 attempts, 18 model calls, 746.7k input tokens, 26 `recall_context` calls, no edit
      (ledger:5037,5047).
    - The policy's only responses are to close reading or to give up
      (`internal/core/defaults/policy/repair_episode.mg:42-55`). Nothing re-plans the cause.
11. **Judges report uncalibrated confidence, and defaults are invented.**
    - The prompt-evolution judge defaults confidence to 0.85/0.80 when the model omits it
      (`internal/autopoiesis/prompt_evolution/judge.go:305-313`).
    - The north-star guardian defaults its score to 0.7 (`internal/northstar/guardian.go:371,550`).
    - The delegation judge's self-reported confidence is compared to
      `delegation_judge_reject_confidence` (`internal/core/defaults/policy/delegation.mg:369-372`).
      That judge sees the task and the shard's prose, not the diff or the gate evidence
      (`internal/verification/verifier.go:397-405`).
12. **The recurse loop forgets why an attempt was reverted, and its metrics can be gamed.**
    - On revert, the diff is discarded and only a failure signature is kept
      (`internal/campaign/recurse_cycle.go:747-765`).
    - The next attempt's task has no memory of it (`internal/campaign/recurse_attempt.go:96-113`,
      `recurse_cycle.go:38-57`).
    - Improvement is judged by a regex count of test functions and by raw coverage
      (`internal/gates/metrics.go:15-28,51-54`, `internal/core/defaults/policy/recurse.mg:247-251`).
      A test that executes code but asserts nothing raises both.
13. **Model routing is by verb, not by difficulty.** `intent_requires_reasoning_model` keys on the
    verb or action only (`delegation.mg:297-302`). A one-line `/fix` and a cross-package race fix
    are served identically.
14. **Only Go turns are pinned.** `/pinned` is owed only for `turn_write_class(Turn, /go)`
    (`coder_safety.mg:203`). Python, TypeScript and Rust changes are never checked for "a test
    notices this change gone".

---

## Ideas

Ideas are ordered along the path request → plan → context → generation → verification →
learning → infrastructure. The top-7 ranking is the table above.

### Q-01 — Requirement ledger: the verdict learns the scope of the request

- **Pitch:** turn the brief into typed requirement facts, and make `/done` require evidence for
  each one.
- **Mechanism:**
  1. After perception and before the tool loop, one extraction pass turns the brief into
     `turn_requirement(Turn, ReqID, Kind, Text)`. `Kind` ∈
     {`/behavior`, `/count`, `/artifact`, `/prohibition`}.
     - For campaigns, reuse `extractRequirementsSmart` (`internal/campaign/decomposer_requirements.go:19`).
     - For `nerd fix`/chat, reuse the perception call so no extra round-trip is needed.
  2. Evidence per requirement, strongest first:
     - (a) A mechanical witness. A test the turn wrote that fails with the change taken out
       (reuse the pin gate's overlay, `pin_gate.go:533`) and that the model links to `ReqID` in
       its control packet.
     - (b) A deterministic check for `/count` and `/artifact` kinds (file exists, grader output,
       N of M). Jev is weak at counting, so it never answers these.
     - (c) One Jev pass for `/behavior` kinds. The state is the requirement list plus the turn's
       diff hunks from `renderTurnDiff`. It asks one Noul per requirement: "Do these hunks
       implement requirement R?" All questions ride one state, so cost is flat in N.
  3. The driver asserts `turn_requirement_met(Turn, ReqID, Source, Pct)`.
  4. Policy (new rules beside `coder_safety.mg:296-333`):

  ```mangle
  turn_requirement_unmet(T, R) :- turn_requirement(T, R, _, _), !turn_requirement_witnessed(T, R).
  turn_requirement_witnessed(T, R) :- turn_requirement_met(T, R, /test, _).
  turn_requirement_witnessed(T, R) :- turn_requirement_met(T, R, /check, _).
  turn_requirement_witnessed(T, R) :- turn_requirement_met(T, R, /judged, P),
      config_param(/quality_requirement_met_min_pct, Min), P >= Min.
  turn_missing_evidence(T, /requirement_unaddressed) :- turn_unverified(T), turn_requirement_unmet(T, _).
  ```

  5. Add `!turn_has_unmet_requirement(Turn)` to the write arm of `turn_verified`. A turn with no
     extracted requirement is unaffected.
- **Plugs in at:**
  - `internal/core/defaults/policy/coder_safety.mg:297` (the write arm) and `:323-333` (missing
    evidence).
  - `internal/session/executor.go:2519` (`assertTurnEvidence`, which asserts the facts).
  - `internal/session/executor.go:926` (`ProcessWithIntent`, the extraction point).
  - `internal/core/mangle_updates.go:272`: add the new predicates to the model-blocked list.
- **Quality effect + basis:** closes N41's class. The audit's own conclusion is that the gap
  needs "an acceptance contract from the brief"
  (`07-external-audit-2026-09-19.md:219-225`). R2-2, R3-1, R5-3, R5-4 and R6-D1 would each have
  ended `/unverified`, naming the unmet requirement, instead of `/done`.
- **Token cost:** + small. One extraction (or zero extra if folded into perception) plus one Jev
  pass. At the quoted $0.042/M input, a 20k-token state costs ≈$0.00084 (arithmetic, not a
  measurement). A later token saving: a turn stops when every requirement is witnessed, instead
  of when the model feels done.
- **Measure it by:**
  - The Q-22 bench: the rate of `/done` over briefs whose reviewer found the scope incomplete.
  - Reviewer-agreement rate of `turn_requirement_met` on landed runs.
- **Risks / failure modes:**
  - The extraction invents requirements, which over-obliges. Mitigate by letting the user edit
    the list interactively, and by giving headless runs the Q-03 treatment.
  - Jev false positives on `/behavior`: it is literal and degrades with irrelevant state. Keep
    the state to hunks only.
  - A requirement that no test can pin (docs). Allow `/judged` only for `/doc` write class.
- **Effort:** M. **Needs Jev?** y (LLM stand-in today). **Confidence:** medium-high.

### Q-02 — Reproduce-first contract for `/fix` (harness-frozen reproducer)

- **Pitch:** for a behaviour fix, the first phase writes a failing reproducer. The harness
  freezes it into an `evidence.Contract` before any source edit, and the fix must turn it green.
- **Mechanism:**
  1. **Phase A (tests only).** Write tools are limited to `_test.go` / test paths by a
     phase-scoped allowlist fact. The model writes a test named for the symptom. The harness runs
     it at HEAD.
     - It must **fail**, with a compile failure excluded (`testBuildFailed`,
       `build_verify.go:432`).
     - With Jev, one Noul decides whether the failure output exhibits the symptom in the brief.
       Without Jev, the model states the expected failing assertion and the harness matches the
       test's reported failure line.
     - If it cannot be made to fail, the turn ends `/unverified` with
       `/symptom_not_reproduced`. That is the R1-19 lesson made mechanical: "a finding does not
       enter a brief until its falsifying check has been run" (ladder:108-110).
  2. **Freeze.** The harness builds an `evidence.Contract` with `Reproducer: true`. `Authority`
     is `harness:reproducer`, never the model's final text. `evidence.Begin` then hash-protects
     the test file (`internal/evidence/change.go:198-268`) and re-verifies that it fails.
  3. **Phase B (fix).** Normal tool loop. Test files stay writable, but a change to the protected
     reproducer is reported (`Verify` already does this, `change.go:283-288`).
  4. **Verify.** `tx.Verify` gives `turn_reproducer_verified(Turn, ContractID)`. This is a new,
     weaker fact than `turn_acceptance`, because the author is the model, not the caller.
     Policy: `/fix` turns owe it once the reproducer phase ran.
- **Plugs in at:**
  - `cmd/nerd/cmd_direct_actions.go:279-313`, where the contract is threaded today.
  - `internal/evidence/change.go:102-131` (`Validate`) and `:198` (`Begin`).
  - `internal/session/executor.go:2527-2529`, where `turn_acceptance` is asserted.
  - `coder_safety.mg:197-203`: `behavior_change_intent(/fix)` owes `/reproduced`.
  - `turn_rounds.mg:31-38` gets no new round. This is a *pre*-edit phase.
- **Quality effect + basis:**
  - A reproducer written before the helper exists has to exercise the public behaviour. That
    addresses R1-10 and R1-11, where tests pinned the helper and the brief's scenario was added
    by hand.
  - It prevents R1-19.
  - Externally: Agentless selects patches with generated reproduction tests. ReProAgent reports
    that better reproduction tests raise Agentless's SWE-bench Verified resolves from 188 to 201.
    That is the paper's number, not reproduced here.
- **Token cost:** 0/+. The coverage and pin rounds already force a test afterwards. This moves it
  first, and a failing reproducer should shorten diagnosis.
- **Measure it by:**
  - Q-22: re-run R1-10, R1-11 and R1-19 and check whether the reproducer is the brief's scenario.
  - Landing rate on symptom-only briefs.
  - Count of "tests pass with the fix reverted" findings by the reviewer.
- **Risks / failure modes:**
  - Flaky or load-dependent symptoms (R1-1) cannot be reproduced. Downgrade to the current flow
    with an explicit `/symptom_not_reproducible` reason.
  - The model writes a reproducer that fails for the wrong reason. The Jev symptom-match, or the
    failing-line match, mitigates this.
  - Non-Go workspaces need the `/test_run` command runner (`internal/session/test_run_round.go`).
- **Effort:** M. **Needs Jev?** n (improved by y). **Confidence:** medium-high.

### Q-03 — Clarify-or-assume: requirement determinability before code

- **Pitch:** ask when a requirement cannot be pinned down. In headless mode, write the
  assumption down as a fact the verdict must report, instead of guessing silently.
- **Mechanism:**
  1. For each `turn_requirement` (Q-01), ask Jev one Noul: "Is the expected behaviour for R
     determined by the brief plus this repository context?" The state is the brief plus a
     holographic summary.
  2. Asserted as `requirement_determinable(Turn, R, Pct)`.
  3. Policy:
     - Interactive and not yolo:
       `next_action(/interrogative_mode) :- requirement_underdetermined(_)`. This reuses the
       clarification machinery (`internal/core/defaults/policy/clarification.mg:15-18`) and emits
       one question per turn, the lowest-probability requirement first.
     - Headless or yolo: the model must state an assumption. It becomes
       `turn_assumption(Turn, R, Text)`. The final report lists it, and the critic (Q-11) is told
       which assumption each hunk rests on.
- **Plugs in at:**
  - `clarification.mg:5-8`. Today's only confidence gate is `focus_resolution`'s path score
    < 85, and `ambiguity_flag` is built from the perception LLM's self-reported confidence
    (`internal/shards/system/perception.go:662-675`).
  - The report goes through `turn_summary.go`.
- **Quality effect + basis:**
  - Today's clarification is about *which file/verb*, never *what behaviour*.
  - TiCoder (test-driven intent clarification) reports better correctness judgments with
    lightweight user feedback.
  - Verbalized LLM confidence is systematically overconfident (Xiong et al.). That is the signal
    `ambiguity_flag` rests on today.
- **Token cost:** ≈0 (one Jev pass). It saves whole wrong-direction turns.
- **Measure it by:** the fraction of reverted or not-landed runs whose root cause was a
  misread requirement (ledger tags), and the clarification rate per 100 turns (it must stay low).
- **Risks:** nagging. Mitigate with a `config_param` cap on questions per turn and with
  determinability thresholds tuned on Q-22. Jev degrades with irrelevant state, so keep the state
  small.
- **Effort:** S-M. **Needs Jev?** y. **Confidence:** medium.

### Q-04 — Difficulty-routed compute

- **Pitch:** a calibrated difficulty score decides model slot, thinking budget, and whether the
  expensive quality mechanisms (Q-02, Q-08, Q-12) are owed.
- **Mechanism:**
  1. One Jev Score with 5 described levels, from "single local edit, behaviour obvious" to
     "cross-package, concurrency/protocol semantics, unclear cause". The state is the brief plus
     the holographic summary of named targets (importer count, callers, has-tests).
  2. Asserted as `task_difficulty(Intent, LevelPct, ConfPct)`.
  3. Policy adds `intent_requires_reasoning_model(Verb) :- current_task_difficulty_at_least(4)`.
  4. `quality_mechanism_owed(/best_of_n)` holds only at level 5, or after a non-converging repair
     (Q-09).
  5. Thinking budget becomes a derived `config_param` override per turn.
- **Plugs in at:**
  - `internal/core/defaults/policy/delegation.mg:297-302`.
  - `internal/session/executor.go:440` (`llmForVerb` already asks the kernel per turn).
  - `internal/types/ctxkeys.go:155` (the sampling override path exists).
- **Quality effect + basis:**
  - Hard tasks get the strong model. In "Thinking Longer, Not Larger", serial test-time compute
    improves SWE agents.
  - The ledger shows the costliest failures on multi-seam briefs (R1-13: 42 min, 114 calls).
- **Token cost:** 0/− overall. Hard tasks go up, easy ones stay on the worker (shared with the
  token lane).
- **Measure it by:** landing rate vs spend by difficulty bucket, and the calibration of the
  predicted level against actual repair-round count.
- **Risks:**
  - Jev's difficulty judgment is unvalidated on this repo's briefs. Calibrate on the ledger
    first: it holds the minutes, calls and outcome of every run.
  - A misroute is costly only in one direction, so bias to up-route.
- **Effort:** S-M. **Needs Jev?** y. **Confidence:** medium.

### Q-05 — Plan coverage and ordering check (campaigns and work steps)

- **Pitch:** replace keyword requirement→task linking with a calibrated assignment. An uncovered
  requirement then triggers a replan.
- **Mechanism:**
  1. For each requirement, one Jev Choice over `{task_1 … task_n, none}`. That fits Jev's ≤255
     options. The state is the requirement plus the task descriptions and write sets.
  2. Assert `requirement_coverage(ReqID, TaskID)` only when p ≥ threshold, and
     `requirement_uncovered(ReqID)` when `none` wins.
  3. Add the missing consumer: `plan_issue(/requirement_uncovered, R) :- requirement_uncovered(R)`.
     This feeds the existing refinement loop (`internal/campaign/decomposer.go:470-476`, which
     refines on `validatePlan` issues).
  4. Deterministic ordering check: a task whose write set reads a path that a *later* task
     creates is an issue. Also handle C1, "targets fixed before research" (ladder:134-138).
  5. Work steps (`internal/session/work_steps.go:36-44`): one Noul per extracted requirement asks
     whether some STEP line delivers it, before steps run.
- **Plugs in at:**
  - `internal/campaign/decomposer.go:478-486` and `:901-948`.
  - `internal/core/defaults/schemas_campaign.mg:294-296`.
  - `internal/session/work_steps.go`, the step planner.
- **Quality effect + basis:**
  - R6-D1's 5-task plan silently dropped required slots (ladder:537).
  - Today a requirement is "covered" if two words longer than 3 characters match
    (`decomposer.go:906-925`).
- **Token cost:** − (Jev replaces nothing costly, but prevents campaign phases wasted on plans
  that could never satisfy the goal).
- **Measure it by:** requirements linked and actually delivered at campaign end, and the replan
  rate.
- **Risks:** the Choice over many near-duplicate tasks may split probability mass. Use top-2 sum
  plus confidence.
- **Effort:** S-M. **Needs Jev?** y (an LLM classification works today). **Confidence:** medium-high.

### Q-06 — Change-impact context pack (symbol-exact, tests, dossier)

- **Pitch:** for the edit target, hand the model the call sites of the symbols it is about to
  change, the tests that exercise them, and the file's recent failure history, before it asks.
- **Mechanism:**
  1. **Callers of changed/target symbols with call-site snippets** (±3 lines), resolved by symbol
     identity, not substring.
     - For Go: `gopls references file:line:col` / `gopls call_hierarchy` via the CLI already used
       for `gopls check` (`internal/session/lsp_diagnostics.go:52-92`).
     - For other languages: add `textDocument/references` and `callHierarchy/incomingCalls` to the
       LSP client (`internal/world/lsp/client.go:267-345` has `call` and `Initialize`).
  2. **Tests that exercise the target.** Derive from the coverage profile the test gate already
     produces (`internal/session/coverage_profile.go`). Also add a background per-test coverage
     map built during `nerd scan` (`test_exercises(Test, File, Func)` facts), then render test
     names and their assertion lines.
  3. **File dossier:** the last N repair-episode `InitialFailure` digests touching this file
     (`RepairRecord`, `internal/session/repair_loop.go:64-72`), recurse reverts on the node, and
     open critic findings.
  4. Rank items with Aider-style personalized PageRank over the symbol graph the world model
     already has (`code_defines`/`code_calls`). Budget-fit it through the working ledger.
- **Plugs in at:**
  - `internal/world/holographic.go:348-599` (`PromptSection`).
  - Fix the substring call-graph match at `:1151-1153`.
  - Replace the filename test check at `:1166-1178`.
  - Wire the unused `FormatWithPriorities` caller-body path
    (`internal/world/holographic_impact.go:684`), or delete it.
- **Quality effect + basis:**
  - R1-15 failed because the explanatory diff was not in view. R1-12 failed because the failing
    test was not (ladder:63-66).
  - Aider reports higher edit accuracy with ranked repo maps.
  - SWE-agent's ablations show context and navigation design materially changes resolution.
- **Token cost:** + small per request, − in reads. R1-13 spent 64 `read_file` calls on a 3-file
  change (ledger:5063). Crossover with the token lane.
- **Measure it by:** `read_file`/`recall_context` calls before first edit, and the Q-22 landing
  rate.
- **Risks:** stale snippets after edits. Render on each focus change, as the outline already does
  (`holographic.go:578-589`). gopls latency needs to be cached per content digest.
- **Effort:** M. **Needs Jev?** n. **Confidence:** medium.

### Q-07 — Workspace convention fingerprint → per-package house-style atoms

- **Pitch:** mine how *this* codebase writes code and tests, and serve that instead of a generic
  template.
- **Mechanism:**
  1. A deterministic `go/ast` (and tree-sitter) pass per package at `nerd init`/`scan`. It
     measures shares, not opinions:
     - error construction (`fmt.Errorf("…%w")` vs sentinel vs `errors.New`);
     - error-message casing;
     - test style (table-driven share, `t.Run` share, helper usage, `t.Helper()`, golden files,
       testify vs stdlib);
     - logging API/category usage (`logging.Get(logging.CategoryX)`);
     - doc-comment conventions;
     - receiver naming;
     - file-size norms (the repo has a modularity gate, `internal/session/modularity.go`).
  2. Emit `package_convention(Pkg, Dimension, Value, SharePct)` facts.
  3. Render a ≤150-token atom per package, `project/conventions/<pkg>`, which the JIT selects when
     the target is in that package.
  4. Replace the hardcoded `project/go/conventions` atom.
  5. The critic (Q-11) gets a single "convention deviations in the hunks" line, computed by the
     same analyzer on the diff. This is deterministic and needs no LLM.
- **Plugs in at:**
  - `internal/init/profile.go:305-376` (`buildProjectAtoms`).
  - `internal/init/initializer.go:197-200` (the fields exist, never mined).
  - JIT selection in `internal/prompt/selector.go`.
- **Quality effect + basis:** code that "fits the codebase first time". R1-7's nit and the doubled
  blank lines (N18) are style drift the gates catch late. Generic atoms can contradict the repo
  (item 9 in the section above). Item 12 of the owner's program asks for domain experts built
  holistically (ladder:560).
- **Token cost:** −/0. A small targeted atom can let the JIT drop generic encyclopedic ones
  (`go_ai_failure_modes.yaml` is mandatory, priority 95). Crossover with the token lane.
- **Measure it by:** vet/critic style findings per turn, reviewer "nit" counts, and share of
  new tests matching the package's test style.
- **Risks:** mining legacy mistakes as conventions. Use a share threshold and recency weighting
  (git blame age). Keep it advisory in the prompt, not a gate.
- **Effort:** M. **Needs Jev?** n. **Confidence:** medium.

### Q-08 — Gated best-of-N with execution-first ranking

- **Pitch:** when a task is hard or the repair is stuck, generate K candidates and let the real
  gates choose. A cheap calibrated ranker breaks ties.
- **Mechanism:**
  1. Owed only when `quality_mechanism_owed(/best_of_n)` holds (Q-04 level 5, or
     `repair_not_converging`).
  2. Generate K candidate edits with diverse sampling (`types.SamplingFromContext`,
     `internal/types/ctxkeys.go:155`) and, better, diverse *hypotheses* (Q-09).
  3. Evaluate each candidate without touching disk. For Go, use `go test -overlay`, the mechanism
     the pin gate already uses (`internal/session/pin_gate.go:626-672`). Otherwise, run
     sequentially with `snapshotTurnFiles`/`restore` (`build_verify.go:797-800`).
  4. Rank lexicographically by:
     - build/tests green;
     - the Q-02 reproducer passes;
     - pin gate passes;
     - fewest surviving mutants (Q-10);
     - CodeT-style dual agreement (candidates that pass the same set of model-generated tests
       form consensus groups);
     - a Jev Score rubric over the diff (minimality, fits conventions, addresses every
       requirement).
  5. Keep one candidate, and journal the rest as `.nerd/attempts` patches (`buildable_tree.go:51-53`).
- **Plugs in at:**
  - `internal/session/repair_loop.go` (episode driver).
  - `repair_episode.mg:42-55`: add `repair_move(E, /fan_out)`.
  - `internal/session/executor_tools.go:447` (`verifyCompletedToolTurn`) for the full-turn
    variant.
- **Quality effect + basis:**
  - CodeMonkeys: parallel trajectories plus test-voting selection, 57.4% on SWE-bench Verified;
    selection over an ensemble of top submissions scored 66.2%.
  - R2E-Gym: execution-based and execution-free verifiers each saturate around 42-43% and reach
    51% combined.
  - OpenHands: 60.6% to 66.4% with 5 attempts and a critic.
- **Token cost:** ++ (K×). It pays for itself only behind the gate. Execution does most of the
  ranking, and Jev's output is free.
- **Measure it by:** landing rate on Q-22's hard subset against K× spend, and the fraction of
  wins where the chosen candidate was not candidate #1.
- **Risks:**
  - Nothing distinguishes the candidates when all pass weak tests (R2E-Gym's "low
    distinguishability"). Q-10 and Q-12 supply the distinguishing signal.
  - Overlay-only evaluation is Go-only.
- **Effort:** L. **Needs Jev?** partial. **Confidence:** medium.

### Q-09 — Repair hypothesis restart instead of "close reading and grind"

- **Pitch:** when a repair episode repeats a failure, restore the pre-episode state and restart
  from a different stated cause, instead of giving up or re-reading.
- **Mechanism:**
  1. On `repair_not_converging(E)` (`repair_episode.mg:42-47`), ask the model for 3 alternative
     causes of the failure, one line each, each naming the evidence it would change. This is one
     cheap call with no tools.
  2. Jev Choice picks the most probable given the failure output plus the turn diff, with
     calibrated probabilities. Today, the model's own ranking stands in.
  3. Restore the episode's starting snapshot, then run one fresh attempt whose prompt names the
     rejected hypothesis and the chosen one.
  4. Record `repair_hypothesis(E, H, Outcome)` so a hypothesis is never retried in the episode.
- **Plugs in at:**
  - `internal/core/defaults/policy/repair_episode.mg:49-55`: new `repair_move(E, /restart)`
    before `/give_up`.
  - `internal/session/repair_loop.go` (the episode driver) and `build_verify.go:532-650`
    (`repairRound`).
- **Quality effect + basis:**
  - R1-12 (746.7k tokens, no edit) and R1-15 (the model "hunted a broken extractor") are
    wrong-hypothesis loops (ladder:63-66, 79-83).
  - Reflexion: verbal self-reflection on failure raised HumanEval pass@1 to 91%.
- **Token cost:** − net. It cuts runaway loops, and the restart is bounded by the same
  `session_repair_max_attempts`.
- **Measure it by:** input tokens per repair episode, and the convergence rate of episodes that
  hit `repair_not_converging`.
- **Risks:** all three hypotheses may be wrong. The episode still ends at its cap, which is no
  worse than today.
- **Effort:** S-M. **Needs Jev?** n (improved by y). **Confidence:** medium.

### Q-10 — Survivor triage round: surviving mutants become obligations

- **Pitch:** the pin gate's surviving condition mutants are the best defect locator in the
  system. Classify each one and oblige a test for the ones that are real decisions.
- **Mechanism:**
  1. Add `round_order(/survivors, 6)` and shift `/vet`, `/removed_tests` and `/test_run` by one.
     `turn_round_owed(T, /survivors) :- turn_round_ran(T, /pinned), turn_pin_survivor(T, _)`.
     `verifyPinning` asserts `turn_pin_survivor(Turn, UnitLabel)` for each survivor
     (`pin_gate.go:585-592`).
  2. Classify each survivor into {`/observable_decision`, `/guard_no_observable_effect`,
     `/equivalent`}. With Jev, use one Choice per survivor over a shared state: the hunk, the
     enclosing function, and the forced condition. Today, one small LLM call lists every survivor
     with the hunk.
  3. Assert `pin_survivor_class(T, U, Class, Pct)`.
  4. Policy charges only confident observable decisions:

     ```mangle
     turn_survivor_charged(T, U) :- pin_survivor_class(T, U, /observable_decision, P),
         config_param(/quality_survivor_charge_min_pct, Min), P >= Min.
     turn_missing_evidence(T, /decision_not_pinned) :- turn_unverified(T), turn_survivor_charged(T, _).
     ```

  5. The round's repair prompt is `advisorySection` (`pin_gate.go:731-739`), narrowed to the
     charged survivors. The recheck re-runs only those mutants, with the overlay already built.
  6. Uncharged survivors go into the final report as "decisions no test distinguishes".
- **Plugs in at:**
  - `internal/core/defaults/policy/turn_rounds.mg:31-53`.
  - `internal/session/pin_gate.go:580-592,773,782`.
  - The `rounds` map in `internal/session/executor_tools.go:467-497`.
- **Quality effect + basis:**
  - R1-13: 12 survivors, 2 on the exact lines of both regressions the review found
    (ledger:5074-5078). The turn closed `/done` and the survivors reached no one.
  - The code's own reason for not charging is that "five of eight surviving conditions were
    guards whose forcing changes nothing observable" (`pin_gate.go:581-586`). A classifier is
    exactly the missing piece.
  - Meta's ACH turns undetected mutants into LLM test-generation prompts and uses an LLM
    equivalent-mutant detector: precision 0.79 and recall 0.47, rising to 0.95/0.96 with
    pre-processing.
- **Token cost:** 0 when there are no survivors. Otherwise ≈0 for classification with Jev, plus
  one targeted repair round only for charged survivors.
- **Measure it by:**
  - Q-22: do the survivors charged on R1-8, R1-13 and R1-6 (Go parts) point at the
    reviewer-found defects?
  - Charged-survivor precision: the fraction whose forced test fails on some plausible input.
- **Risks:**
  - Over-charging guards stalls turns. The threshold is policy, and the class is withheld when
    confidence is low.
  - The test written to kill a mutant can be vacuous. The recheck proves the mutant dies.
- **Effort:** S. **Needs Jev?** n (y makes it cheap and calibrated). **Confidence:** high.

### Q-11 — Hunk-scoped, request-aware critic with risk pre-screen and finding verification

- **Pitch:** review what changed, against what was asked, with the callers in view. Spend the
  expensive reviewer only on risky hunks, and check each finding before paying an uplift round.
- **Mechanism:**
  1. **Input.** Replace whole files (`critic.go:369-396`) with:
     - the turn's hunks, each with its enclosing function (`renderTurnDiff`, `turn_diff.go:89`);
     - the brief and requirement list (Q-01);
     - the callers and call sites of changed exported symbols (Q-06);
     - the names of tests touching them;
     - diagnostics and survivors (Q-10).
  2. **Pre-screen.** One Jev Score per hunk on a 5-level risk rubric (logic/contract/concurrency/
     security vs mechanical). Only hunks at or above the level set by `config_param` go to the
     planner-slot critic. Low-risk hunks get a cheap-model pass or none.
  3. **Output.** Keep the `FINDING file:line severity: claim` protocol (`critic.go:59`), plus a
     required `REQ:<id>` or `REGRESSION` tag.
  4. **Verify findings before uplift.** One Jev Noul per finding: "Is this claim supported by the
     hunk shown?" Drop findings on lines outside the turn's hunks, which is deterministic. This is
     what went wrong in R1-11.
  5. Move `criticSystemPrompt` (`build_verify.go:895`) and `buildCriticPrompt`'s instructions
     into atoms. Creed II: behaviour lives in atoms, not Go strings.
- **Plugs in at:**
  - `internal/session/build_verify.go:675-803` (`verifyAndUpliftWithCritic`).
  - `internal/session/critic.go:73-164,284-295`.
  - `internal/prompt/atoms/reviewer/`.
  - `ctx = broker.WithPurpose(ctx, broker.PurposeCritic)`, which is declared
    (`internal/broker/types.go:109`) and never used, so critic spend becomes visible in the ledger.
- **Quality effect + basis:**
  - Evidence item 5 above. Self-preference bias studies show LLM judges favour outputs of their
    own family, and the critic falls back to the writing model when no planner slot is set
    (`build_verify.go:907-914`). Diff-scoping plus request-awareness targets the defects that
    matter.
  - R2E-Gym notes that execution-free verifiers "rely on stylistic features". Grounding in hunks
    and callers is the counter.
- **Token cost:** −. Hunks instead of 6 × 24 KB files, low-risk hunks skipped, and fewer uplift
  rounds on invalid findings.
- **Measure it by:** critic wall time and input tokens per turn, the share of findings in
  untouched code (target 0), and Q-22 defect recall.
- **Risks:**
  - Hunk-only review misses cross-function context. The callers section compensates.
  - The pre-screen misses a subtle hunk. Keep a floor: always review hunks that change a
    condition or a return.
- **Effort:** S-M. **Needs Jev?** partial. **Confidence:** medium.

### Q-12 — Counterexample critic: findings as executable probes, old vs new

- **Pitch:** ask the critic for inputs, not opinions. Run each proposed probe on the preimage and
  on the new code. The outcome matrix decides, not the critic.
- **Mechanism:**
  1. After the critic's prose pass, ask for at most M probes: a `_test.go` function (or a
     workspace-native test) exercising an input "next to" the tests' inputs. Human reviewers do
     this (ladder:206-212): other languages the check lists, a function beside a same-named
     method, a directory and a path under it.
  2. Run each probe twice, with `-overlay` mapping the changed files to their preimages
     (`PreWriteContents`) and without. The pin gate already builds such overlays (`pin_gate.go:626`).

     | old | new | meaning |
     |---|---|---|
     | pass | fail | **regression witness**: `turn_regression_witness(T, Probe)` owes a repair round, and the probe is added to the tests |
     | fail | pass | improvement, kept as a test |
     | fail | fail | pre-existing, reported |
     | pass | pass | finding refuted, dropped |

  3. A regression witness is an executed test, not an opinion. Charging it breaks no creed
     (`build_verify.go:665-671` forbids failing a turn on a *model's* word).
- **Plugs in at:**
  - A new `/probe` round after `/critic` in `turn_rounds.mg`.
  - `internal/session/pin_gate.go`: reuse the overlay writer and `runPinUnit`.
  - `coder_safety.mg:323-333`: add `/regression_witnessed`.
- **Quality effect + basis:**
  - R1-6: new accepts, HEAD refuses. R1-13 and R1-8: HEAD reached an element, new does not.
    These are exactly "old pass, new fail" shapes (ladder:512,514,519).
  - The CodeT principle: execution agreement beats judge opinion.
  - R2E-Gym's hybrid: execution-free signal proposes, execution disposes.
- **Token cost:** 0/+. Probes replace some prose, and execution is free of tokens.
- **Measure it by:** Q-22 regression-witness recall on R1-6, R1-8, R1-13 and R1-14 diffs, and the
  false-witness rate on landed diffs (R1-3, R1-7, R1-17).
- **Risks:**
  - A probe that asserts HEAD's buggy behaviour flags the intended change as a "regression".
    Cross-check with the requirement ledger: Jev Noul "Is this behaviour change required by R?"
    Only unrequired changes are charged.
  - Old-vs-new probes are hard when the API signature changed. Skip or adapt.
- **Effort:** M. **Needs Jev?** n. **Confidence:** medium.

### Q-13 — Behavioural trace diff of changed functions under the existing suite

- **Pitch:** get differential testing with no LLM. Log the inputs and outputs of changed exported
  functions while the existing tests run, under old and new code, and diff them.
- **Mechanism:**
  1. For each changed exported Go function, generate an overlay wrapper that records
     `(argsHash, resultHash)` to a temp file, by renaming the original through the overlay. Run
     the package's tests plus importer tests (`importer_packages.go`) once with the preimage
     overlay and once with the current code.
  2. Any `argsHash` seen in both runs with different `resultHash` is a behavioural delta. Deltas
     whose test still passed are the interesting ones: behaviour changed and no assertion noticed.
  3. Assert `turn_behavior_delta(T, Func, N)`.
  4. The model must mark each delta intended, linked to a `ReqID` (Q-01), or fix it. Jev Noul
     "Is delta D required by requirement R?" can pre-sort the deltas.
- **Plugs in at:**
  - `internal/session/pin_gate.go` (overlay machinery).
  - `internal/session/importer_packages.go`.
  - A new round after `/pinned` in `turn_rounds.mg`.
- **Quality effect + basis:** it catches "the change takes something away" regressions (R1-13,
  R1-14's `internal/observation` contract) whenever *some* existing test drives the input, even
  one that does not assert the output. It is fully mechanical: "Verify outcomes, not process".
- **Token cost:** 0 (compute only). One classification call when deltas exist.
- **Measure it by:** deltas surfaced on Q-22's regressing diffs vs landed diffs.
- **Risks:**
  - Non-deterministic outputs (time, maps, pointers). Hash canonicalized JSON or skip
    non-comparable types.
  - Wrapper generation for methods, generics or unexported types is fiddly. Start with top-level
    exported functions whose results are comparable values.
- **Effort:** M-L. **Needs Jev?** n. **Confidence:** medium-low (engineering risk), high value
  if it works.

### Q-14 — Admission-against-interest and claims audit of the final response

- **Pitch:** a model's claim of success is not evidence. A model's *admission* of incompleteness
  is. Read the final response for admissions and for claims the evidence contradicts.
- **Mechanism:**
  1. One Jev pass over the turn's final `surface_response`. The state is the response plus the
     `turn_missing_evidence`/gate summary.
  2. Questions:
     - Noul: "Does the response state that part of the requested work was not done, not
       verified, or needs another invocation?"
     - Noul per extracted claim sentence (≤10): "Is claim C contradicted by the evidence block?"
  3. Assert `turn_self_reported_incomplete(T, Pct)` and `turn_claim_contradicted(T, N)`.
  4. Policy: `turn_missing_evidence(T, /self_reported_incomplete)` holds above threshold.
     Contradicted claims are appended to the surfaced answer as corrections. They are never
     silently rewritten.
- **Plugs in at:**
  - `internal/core/defaults/policy/coder_safety.mg:262` (`turn_done`) and `:323-333`.
  - `internal/session/executor.go:2519-2560` (`assertTurnEvidence`, which already inspects the
    response for test-runner output via `responsePresentsTestRunnerOutput`).
  - `internal/session/turn_summary.go`.
- **Why Jev and not a string match:** a phrase scan over `.Response` is a "textmatch" that the
  executive-literal budget counts and only lets shrink (`executive_literals_test.go:224-226`).
  Classification belongs to a model, and the kernel decides.
- **Quality effect + basis:**
  - N41: R5-3 closed `/done` "one paragraph after 'I cannot execute the remaining 177 deletions
    ... Requesting re-invocation'" (`07-external-audit-2026-09-19.md:203-209`).
  - R3-1: `/done` "though its own report named what it had not verified" (ladder:534).
  - R3-2: "its closing report told the user to delete the two files it had already deleted"
    (ladder:535).
- **Token cost:** ≈0 (a Jev pass is sub-second, and output is free). Today: one small LLM call.
- **Measure it by:** the rate of `/done` turns whose response contains an admission (a hand-audit
  sample from logs), and the reviewer-found "report lies" count.
- **Risks:**
  - The response is model-authored, so it is adversarial state by definition. The asymmetry makes
    that safe: it can only withhold `/done`.
  - False positives on hedged but complete answers. Tune the threshold on labelled ledger runs.
- **Effort:** S. **Needs Jev?** y (LLM fallback). **Confidence:** high.

### Q-15 — Pinning and mutation beyond Go, and beyond "remove the function"

- **Pitch:** pin Python, TypeScript and Rust changes too, and add the classic operator mutants on
  changed lines in Go.
- **Mechanism:**
  1. **Polyglot.** For `turn_write_class(T, /other)` with a known test runner (the `/test_run`
     gate already resolves one, `internal/session/test_run_round.go`), revert each changed
     function (tree-sitter spans from CodeDOM, `internal/world/ast_treesitter.go`) in a temp copy
     of the file, with the workspace restored after, and run the turn's tests.
  2. **Operators (Go).** Relational boundary (`<`↔`<=`), negation, `return nil`/zero value, and
     off-by-one on changed lines only. This is bounded by change size, like `conditionUnits`
     (`internal/session/condition_units.go:12-24`).
  3. Survivors flow into Q-10's triage. Jev also filters equivalent mutants (ACH's detector
     role).
- **Plugs in at:**
  - `internal/core/defaults/policy/coder_safety.mg:203`: add
    `turn_owes_gate(Turn, /pinned) :- turn_verb(Turn, V), behavior_change_intent(V), turn_write_class(Turn, /other), workspace_test_runner(_)`.
  - `internal/session/pin_gate.go:190` (`pinUnits`) and `condition_units.go:27`.
- **Quality effect + basis:** evidence item 14 above. Python and TypeScript turns get build and
  test signals only. Mutation adequacy is a stronger test-quality signal than coverage (ACH).
- **Token cost:** 0 in compute terms (no LLM), plus repair rounds when unpinned.
- **Measure it by:** the unpinned rate on non-Go turns, and reviewer "tests pass with fix
  reverted" findings on polyglot runs.
- **Risks:**
  - Slow suites. Measure only the turn's tests (`turnTests`, `pin_gate.go:401`), with worker
    caps (`pin_gate.go:612`).
  - A file revert on disk is less safe than an overlay. Use a temp copy plus restore with the
    existing snapshot helpers.
- **Effort:** M-L. **Needs Jev?** n. **Confidence:** medium.

### Q-16 — Domain-shape obligations: input classes and properties

- **Pitch:** before coding, list the input equivalence classes a change must handle. For
  parser, serializer and guard-shaped changes, the kernel owes property tests.
- **Mechanism:**
  1. **Shape table.** In the extraction pass (Q-01), the model lists
     `input_class(Turn, ClassID, Text)`. Examples: "each language the check lists", "method vs
     same-named function", "empty/unicode", "directory vs path under it".
  2. One Jev Noul per class asks: "Does some test in the turn's test hunks exercise class C?"
     An unexercised class goes into `turn_missing_evidence(T, /input_class_untested)`.
  3. **Shape-owed properties.** One Jev Choice per changed function over
     {parser, serializer/formatter, normalizer, validator/guard, other}. Then:
     - `turn_owes_gate(T, /property)` for parser+serializer pairs (round-trip), normalizers
       (idempotence), and guards (the metamorphic "balance-preserving edit is never refused").
     - Go's native `FuzzXxx` gives the runner, and `gates/metrics.go:51` already counts `Fuzz`
       functions.
- **Plugs in at:**
  - `coder_safety.mg:178-203` (new owed gates).
  - `turn_rounds.mg` (a `/property` round).
  - `internal/session/test_verify.go` (runner).
- **Quality effect + basis:**
  - The review enumerated 15 shapes for R1-11 and 7 for R1-13 (ladder:517,519). R1-6's defect
    was an unlisted language class (ledger:4810-4820).
  - The pinning gate "forced the round-trip test the review would have demanded" in R1-14
    (ladder:74-75).
  - LLM-generated property-based tests find edge cases (arXiv 2510.25297).
- **Token cost:** + small (a class list and one property test), − reviewer rework.
- **Measure it by:** Q-22 recall on shape-class defects (R1-6, R1-13), and the property-test
  failure rate at first run.
- **Risks:**
  - Class lists become ritual. Charge only classes Jev marks as unexercised with high
    confidence.
  - Property tests can be vacuous. Pin them with Q-10.
- **Effort:** M. **Needs Jev?** y (for classification; LLM fallback). **Confidence:** medium.

### Q-17 — Recurse reflexion memory: a reverted attempt tells the next one why

- **Pitch:** carry the reverted diff's summary and the ratchet's reasons into the next attempt at
  the same finding or node.
- **Mechanism:**
  1. At revert (`recurse_cycle.go:747-765`), before `git.revert`, save:
     - a bounded patch (like `saveAttemptPatch`, `internal/session/buildable_tree.go:73`);
     - the derived reasons: `recurse_gate_worse(Cycle, Gate)`, `recurse_metric_regressed(Cycle)`,
       or target still open with its gate output tail.
  2. Add `PriorAttempts []PriorAttempt{Summary, WhyReverted, EvidenceTail}` to `RecurseAttempt`
     (`recurse_cycle.go:38-57`).
  3. Render the prior attempts in `recurseAttemptTask` and `recurseImproveTask`
     (`internal/campaign/recurse_attempt.go:96-147`) as "Tried: … Reverted because: … Do not
     repeat it."
  4. Per node, aggregate recurring reasons into a `recurse_node_lesson(Node, Text)` fact, and
     render it as a JIT-selected atom for that node's files.
  5. `finding_stalled` (`recurse.mg`) stays as is, but "two identical failures" now involves two
     informed attempts instead of two blind ones.
- **Plugs in at:**
  - `internal/campaign/recurse_cycle.go:652-659,747-765`.
  - `internal/campaign/recurse_attempt.go:96-147`.
  - `internal/campaign/recurse_journal.go:33-52`: add a `why` field.
- **Quality effect + basis:**
  - Reflexion's episodic memory of failure feedback: HumanEval pass@1 91%, and a 22% gain on
    AlfWorld.
  - Today a retry in the next pass starts from nothing and is stopped after two identical
    failures instead of learning.
- **Token cost:** + small (bounded note), − repeated failed cycles.
- **Measure it by:** the kept rate of second attempts at a finding, and cycles-to-keep per
  finding, from the journal (`nerd campaign recurse status`).
- **Risks:** anchoring on the previous approach. Phrase it as "rejected approach", and cap the
  summary.
- **Effort:** S. **Needs Jev?** n. **Confidence:** medium.

### Q-18 — Anti-Goodhart recurse metrics: mutants killed, not tests counted

- **Pitch:** `/stabilize`, `/harden` and `/extend` are kept only if newly added tests kill
  mutants that survived before.
- **Mechanism:**
  1. Add `MetricKilled` to `internal/gates/metrics.go`. It samples K pin-gate-style mutants over
     the node's functions (deterministic seed per pass, same mutant set before and after), runs
     the node's tests against each via overlay, and counts kills.
  2. `recurse_angle_metric(/harden, /killed, /up)` and `(/stabilize, /killed, /up)` replace or
     accompany `/tests` and `/coverage` (`recurse.mg:247-251`).
  3. Keep the `/tests` count as a guard (no test removed), not as a success signal.
- **Plugs in at:**
  - `internal/gates/metrics.go:15-28,51-54`.
  - `internal/core/defaults/policy/recurse.mg:241-285`.
  - `internal/campaign/recurse_cycle.go` (`r.metrics`).
- **Quality effect + basis:** a regex-counted `func Test…(` rises for an empty test, and coverage
  rises for a test that asserts nothing. Mutation score is the adequacy measure that cannot be
  raised that way (ACH).
- **Token cost:** 0 (compute).
- **Measure it by:** the share of kept improvement commits whose added tests kill at least one
  mutant, from a retrospective on `nerd/recurse` history.
- **Risks:** compute cost on large nodes. Sample K and cache per node content digest.
- **Effort:** M. **Needs Jev?** n. **Confidence:** medium-high.

### Q-19 — Observed-failure atoms: distil repair episodes into pre-emptive checklists

- **Pitch:** the harness sees every compiler and test failure its model makes. Turn recurring
  classes into short "before you finish" atoms, and retire generic ones that never fire.
- **Mechanism:**
  1. Classify each `RepairRecord.InitialFailure` (`internal/session/repair_loop.go:64-72`)
     deterministically by compiler or test-runner error code/prefix.
  2. Key the classes by (language, package, provider/model). The provenance is already on
     `TurnRecord` (`internal/session/executor_learning.go:18-57`).
  3. Assert `failure_class_seen(Lang, Pkg, Model, Class, Count)`.
  4. Policy promotes a class at or above a `config_param` count to an atom
     (`project/lessons/<class>`), selected by the JIT for matching turns. It retires after M
     turns without recurrence.
  5. The atom text is a template per class, with at most one line from the most recent instance.
     This replaces the hand-picked three classes in `buildRepairPrompt`
     (`internal/session/build_verify.go:489-499`) with measured ones.
- **Plugs in at:**
  - `internal/session/executor_learning.go`.
  - `internal/core/defaults/policy/coder_learning.mg:15-67` (it already has
    `coder_error_pattern` → `coder_promote_to_long_term(/error_avoidance, …)`).
  - `internal/prompt/evolved_atoms.go`.
- **Quality effect + basis:**
  - First-time-right rate. Prompt evolution has never promoted an atom (`evolved=0`,
    `Docs/journeys/03-knowledge-delivery-and-window.md:298`), and its judge runs on invented
    default confidences (evidence item 11).
  - Deterministic promotion from counted failures is the evidence-first alternative.
- **Token cost:** + tiny (a few lines per turn), − repair rounds.
- **Measure it by:** the repair-episode rate per class before and after promotion, per model.
- **Risks:** atom bloat. Cap it through the JIT budget and retirement. Per-model keys avoid
  teaching one model another's mistakes.
- **Effort:** M. **Needs Jev?** n. **Confidence:** medium.

### Q-20 — Calibrated judges: replace self-reported confidence with measured probability

- **Pitch:** every place where a model judge's confidence feeds a threshold gets a calibrated
  probability, and the judge sees evidence, not prose.
- **Mechanism:**
  1. **Delegation judge** (`internal/verification/verifier.go:387-411`):
     - The state is the task, the attempt's `observation.Return.Changed` diff, the turn verdict
       and the gate summary (`internal/observation/subagent.go:97-111`), not `ret.Output` prose.
     - One Noul: "Does this change accomplish the task?"
     - Assert `judge_verdict(Root, A, /fail, Pct)` with `Pct` = 100 − P(yes)
       (`verifier.go:272`). The policy is unchanged (`delegation.mg:369-372`).
  2. **Prompt-evolution judge** (`internal/autopoiesis/prompt_evolution/judge.go:57-102`): a
     Choice over the error categories, with no invented default confidences
     (`judge.go:305-313`).
  3. **North-star guardian** (`internal/northstar/guardian.go:329,550`): a Score over the
     existing Passed/Warning/Failed/Blocked levels, with no 0.7 default.
- **Plugs in at:** the lines named above, plus a `broker` purpose tag for Jev spend.
- **Quality effect + basis:**
  - Verbalized LLM confidence is systematically overconfident (Xiong et al., "Overconfidence is
    Key").
  - Thresholded policies over miscalibrated numbers make arbitrary decisions.
  - Calibration also makes the learning loop trustworthy, because prompt evolution learns from
    these verdicts.
- **Token cost:** −. Jev replaces LLM judge calls (shared with the token lane).
- **Measure it by:** a reliability diagram of judge probability against later ground truth
  (accepted delegations later reverted; evolved atoms later demoted).
- **Risks:** Jev's calibration claim is self-reported. Keep the Q-21 calibration ledger, and fall
  back to "judge unavailable → fail closed" as today (`verifier.go:262-270`).
- **Effort:** S-M. **Needs Jev?** y. **Confidence:** medium.

### Q-21 — Jev as a Mangle oracle: demand facts in, probability facts out (infrastructure)

- **Pitch:** policy rules can ask semantic questions without making the fixpoint call the
  network.
- **Why not `external()`:**
  - External predicates are called during evaluation. `ShouldQuery` always returns true
    (`internal/core/external_predicates.go:39-43`).
  - A head with an external premise is "volatile", re-derived every time
    (`internal/core/kernel_eval_cone.go:45`).
  - Evaluation holds the kernel lock (`internal/core/kernel_eval.go:305-320`).
  - R2-3 found that marking Decls `external()` with a bound argument panics in
    `topdown.go:99` (ladder:523).

  A 70-500 ms network call inside a volatile rule under the kernel lock is the wrong shape.
- **Mechanism (the demand/answer pattern the executor already uses for `turn_next_round`,
  `internal/session/executor_tools.go:556-587`):**

  ```mangle
  Decl oracle_ask(QID, Kind, StateRef, Schema) bound [/string, /name, /string, /string].
  Decl oracle_answer(QID, Option, Pct) bound [/string, /string, /number].
  Decl oracle_confidence(QID, Pct) bound [/string, /number].
  oracle_pending(Q) :- oracle_ask(Q, _, _, _), !oracle_answered(Q).
  ```

  1. Rules derive `oracle_ask`. A Go driver runs *outside* evaluation. It groups pending
     questions by `StateRef` so that many questions ride one state, one pass, and output is free.
  2. The driver builds the state from harness-controlled text only, and caps it at 64k tokens.
  3. It calls Jev (or an LLM adapter today), then asserts `oracle_answer` and
     `oracle_confidence` as integer percents. This is the convention `focus_resolution` uses
     (`internal/perception/transducer.go:318-321`).
  4. Results are cached in SQLite by `sha256(state ‖ schema ‖ model version)`, and invalidated by
     content digest.
- **Safety invariants:**
  - Add `oracle_*` to the model-blocked predicates (`internal/core/mangle_updates.go:272`).
  - A corpus test fails if any rule deriving `permitted/3` depends, transitively, on `oracle_*`.
  - Oracle facts can only feed `turn_missing_evidence`, `*_owed`, routing and ranking. That is
    withhold or oblige, never grant.
- **Calibration ledger:** every answer is logged with its Kind. When ground truth arrives
  (reviewer verdicts, later reverts, mutant kills), a `nerd meter oracle` report builds
  per-Kind reliability and suggests thresholds. The thresholds stay `config_param`, set by a
  human.
- **Plugs in at:**
  - `internal/core/defaults/schemas_*.mg` (Decls).
  - `internal/session/executor_tools.go:556` (driver pattern).
  - `internal/broker/types.go:104-118` (add `PurposeOracle` so spend is metered).
- **Quality effect:** enabling. Q-01, Q-03, Q-04, Q-05, Q-10, Q-11, Q-14, Q-16 and Q-20 all
  become rules over `oracle_answer`.
- **Token cost:** 0 by itself.
- **Measure it by:** oracle latency p95, cache hit rate, and per-Kind calibration error.
- **Risks:**
  - Adversarial or irrelevant state moves answers. Build state from harness text, and delimit
    model-authored spans.
  - Vendor lock-in. The adapter interface keeps an LLM fallback.
- **Effort:** M. **Needs Jev?** y (LLM adapter works today). **Confidence:** high on design.

### Q-22 — Defect-replay bench: measure the harness on diffs whose defects are already known

- **Pitch:** the ladder already paid for ground truth. Replay the gates, critic, triage and probes
  on saved diffs from runs where the reviewer found a defect, and on diffs that landed clean.
- **Mechanism:**
  1. Build fixtures of (HEAD commit, brief, codeNERD's diff, reviewer's failing probe/shape list).
     Sources:
     - R1-6 (`r1_6_diff.patch`, kept in the scratchpad per ledger:4800);
     - R1-8, R1-10, R1-11, R1-13, R1-14;
     - R2-2, R3-1, R5-3 (scope);
     - clean landings R1-3 (`df4a3063`), R1-7 (`24e9cc56`), R1-17 (`e216f275`).
  2. Run only the post-edit machinery on each fixture. No coding model is needed, which makes
     the bench cheap and reproducible.
  3. Score each mechanism on:
     - recall of the reviewer's defects (by file:line or failing probe);
     - false positives on clean landings;
     - tokens and wall time.
  4. Wire it to `nerd regression run` style reporting (`internal/regression/battery.go`).
- **Plugs in at:** `internal/regression/`, `Docs/journeys/05-elite-harness-ladder.md` (a new
  "bench" table), and `.nerd/attempts/` patches (`internal/session/buildable_tree.go:73-90`).
- **Quality effect + basis:** it turns "which idea helps" into a measurement before committing to
  a design. Creed IV: "verify outcomes, not process". Anthropic's agent-eval guidance recommends
  exactly this kind of targeted eval before scaling.
- **Token cost:** 0 in production.
- **Measure it by:** the bench itself, tracked per commit.
- **Risks:**
  - Small N (tens of fixtures) overfits. Keep adding every reviewer-found defect as it happens.
  - Some diffs were not preserved and need re-creation from the ledger's descriptions.
- **Effort:** S-M. **Needs Jev?** n. **Confidence:** high.

---

## Wild cards

Bolder, lower-confidence ideas.

- **W-1 (wild): System-One pre-flight on every write.** Before a write tool executes, one Jev
  Noul over (the brief's requirement list, the proposed hunk) asks: "Does this hunk move toward
  requirement R?" Low probability returns a one-line nudge *instead of* executing, and the model
  may insist. At 70-500 ms and free output, it is cheap enough per edit. It could catch
  over-building (P4: "found the cause… then over-built", ladder:532) at the moment it happens. The
  risk is that nudges become noise and Jev is literal about partial hunks. It is safe only as a
  nudge, never a refusal: refusals are `permitted/3`'s job alone.
- **W-2 (wild): Train a codeNERD critic on its own ledger.** The recurse journal (kept/reverted
  commits with gate deltas) and the ladder are labelled trajectories. OpenHands trained a 32B
  critic with a TD objective on test outcomes and used it to rerank 5 attempts (60.6% to 66.4%).
  If TypeSafe allows task-specific fine-tuning of Jev, a "will the ratchet keep this?" Noul
  trained on this repository's history is the most on-distribution verifier possible. Whether
  that is available is unknown.
- **W-3 (wild): Two-model disagreement as a review trigger.** Run the worker model's patch
  alongside the planner's on level-4/5 tasks (Q-04). If both pass the gates, Jev scores semantic
  agreement between the two diffs. Disagreement on a hunk sends it to the counterexample critic
  (Q-12) with both versions as the old/new pair. This is CodeT's consensus idea across models
  instead of samples. It costs up to 2× on hard tasks only.
- **W-4 (wild): The requirement ledger as a proof tree.** Make `nerd why turn_done` show, for
  each requirement, the witness that discharged it (a test ID with its fail-before hash, a
  deterministic check, or an oracle answer with its probability and state hash). Every "done"
  becomes a citation, the README's north-star bullet "Every claim is a citation", at requirement
  granularity. Low technical risk. The wild part is making the reviewer's landing criterion 7
  auditable from `nerd why` alone.
- **W-5 (wild): Mutant-guided test generation as a recurse angle.** Add a fifth recurse angle,
  `/sharpen`. Sample surviving mutants on a node (Q-18) and hand the top-K to the model the way
  Meta's ACH does ("write a test that fails on this mutant"). Keep it only if kills rise and no
  gate is worse. This is a forever-loop that improves test adequacy without churning code, which
  is the cheapest safe work the loop could do on a green node.

---

## Sources

- Agentless (reproduction tests + regression tests for patch selection): https://github.com/openautocoder/agentless
- ReProAgent (reproduction tests raise Agentless resolves 188→201 on SWE-bench Verified, as reported): https://arxiv.org/pdf/2607.09123
- "Can Old Tests Do New Tricks for Resolving SWE Issues?": https://arxiv.org/abs/2510.18270
- SWE-RL (Agentless Mini, reproduction-test selection): https://arxiv.org/pdf/2502.18449
- SWE-agent, Agent-Computer Interfaces (linting guardrails, bounded views, ablations): https://arxiv.org/abs/2405.15793
- CodeT, code generation with generated tests (dual execution agreement): https://arxiv.org/abs/2207.10397
- CodeMonkeys, scaling test-time compute for SWE (57.4%; ensemble selection 66.2%): https://arxiv.org/abs/2501.14723
- R2E-Gym, hybrid execution-based / execution-free verifiers (42-43% each → 51%): https://arxiv.org/abs/2504.07164
- OpenHands inference-time scaling with a trained critic (60.6% → 66.4%): https://www.openhands.dev/blog/sota-on-swe-bench-verified-with-inference-time-scaling-and-critic-model
- "Thinking Longer, Not Larger" (serial test-time compute for SWE agents): https://arxiv.org/pdf/2503.23803
- Meta ACH, mutation-guided LLM test generation (equivalent-mutant detector precision/recall): https://arxiv.org/abs/2501.12862
- Reflexion (verbal reinforcement; HumanEval 91%): https://arxiv.org/abs/2303.11366
- TiCoder, test-driven intent clarification: https://arxiv.org/abs/2208.05950 and user study https://arxiv.org/abs/2404.10100
- Aider repository map (tree-sitter + PageRank): https://aider.chat/docs/repomap.html and https://aider.chat/2023/10/22/repomap.html
- Confidence elicitation / verbalized overconfidence: https://arxiv.org/abs/2306.13063 and https://arxiv.org/pdf/2405.02917
- Self-preference bias in LLM judges: https://arxiv.org/pdf/2410.21819 and https://arxiv.org/abs/2604.06996
- LLM-generated property-based tests and edge cases: https://arxiv.org/pdf/2510.25297
- Anthropic, Building effective agents (evaluator-optimizer): https://www.anthropic.com/engineering/building-effective-agents
- Anthropic, Writing effective tools for agents: https://www.anthropic.com/engineering/writing-tools-for-agents
- Anthropic, Demystifying evals for AI agents: https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents
- Jev (TypeSafe AI): facts as given in the task briefing (released 2026-09-15; Choice/Score/Noul; 64k; $0.042/M input; self-reported accuracy and calibration, not independently reproduced). See README.md for the Jev sources; the claims there are TypeSafe's own.
