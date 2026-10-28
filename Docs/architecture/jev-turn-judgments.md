# Jev judgments

Status: as built, 2026-10-28.

Code:

- `internal/typesafe`: the client, including usage billing (`usage.go`).
- `internal/turnjudge`: every question and every fact projection.
- `cmd/nerd/chat/jev_turn.go` and `cmd/nerd/chat/self_diagnosis.go`: chat wiring.
- `internal/system/factory_task_retrieval.go`: task retrieval wiring.
- `internal/core/defaults/policy/jev_evidence.mg` and `side_turns.mg`: the rules.
- `cmd/tools/jev_probe`: live scoring.

## What it is

Jev is TypeSafe's System One model. It answers typed questions about a state
with calibrated probabilities. codeNERD asks it yes/no questions (Nouls) in four
places. Each answer becomes a kernel fact, and a rule reads that fact. Go
never acts on a probability itself. The one exception is the retrieval drop,
described below.

Jev enhances perception and the other lanes. It never replaces them:

- Perception still reads the turn.
- The LLM still writes every reply.
- Every rule makes the system more careful or more attentive.

A missing judgment is never a "no". This holds whether Jev is unconfigured,
switched off, failed, timed out, or answered something unreadable: there is
then no fact, and the lane behaves as it would without Jev.

The thresholds are `judgment_yes_percent` (default 70) and `judgment_no_percent`
(default 30). They reach the kernel as `config_param(/jev_yes_percent, _)` and
`config_param(/jev_no_percent, _)`. A judgment at or above yes reads as yes
(`jev_yes`), and one at or below no reads as no (`jev_no`). Between the two it is
unsettled (`jev_unsettled`, jev_evidence.mg). Every judgment has all three rows:

| Reading | Outcome |
|---|---|
| yes | the rule that reads it acts |
| no | the rule that reads it acts (a no is the evidence for `jev_disputes_mutation`, `review_finding_refuted`, `self_problem_around`, the retrieval drop) |
| unsettled | no rule acts; perception's own reading routes the turn, the chat log line names the unsettled judgments, a retrieved file keeps its keyword rank, a problem stays a defect |

`confidence` is never read: a Noul has none, and the thresholds are on the
probability (`TestJevJourney_UnsettledJudgmentsAreNamedAndActOnNothing`).

## How a request is built

The guidance is TypeSafe's (docs.typesafe.ai, the `typesafe-ai` skill); these
are the choices it led to.

- Request shape follows `POST /v1/systemone`: `model`, `state`, `questions`
  (all Nouls today; Choice and Score are supported by `internal/typesafe` and
  limit-checked, Box and images are not used). 429 and 529 retry, any other
  non-200 is an `APIError`, and a failed call is never a judgment.
- Judgments are Nouls because each is one yes/no condition. There is no
  Choice, so there is no "nothing fits" companion to ask; a decision never goes
  to Jev ("should we ...") because the rules in `jev_evidence.mg` hold the
  truth table.
- State is named fields a person would look at: `message`, `recent_turns`
  (whole turns), `brief` (goal, constraints, decisions, open questions),
  `held_work` (`action`, `target`, `handled_by`, `requested_as`), `running_work`.
  An empty field is left out. Facts the code knows are stated: when older turns
  are dropped for `recent_turns`, `earlier_turns_omitted` says how many; a
  head cut for context says `the first N of M bytes`.
- Questions about one item put the item in the question, as fields, not a
  string template (`open_question`, `file`, `problem`, `finding`).
- Questions that share state go in one request, and those that matter only
  sometimes are asked with them: all turn judgments are one call, and
  `approves_held_work`, `adds_to_held_work` and `wants_to_stop` ride in it only
  while there is held or running work to refer to.
- Wording lives in one module per use: `questions.go` (turn), `relevance.go`,
  `findings.go`, `triage.go`.

## Every question, its switch and its consumer

### Turn judgments: switch `turn_judgments`

The questions are in `internal/turnjudge/questions.go`. They are asked in one
request per turn, beside perception, and land as `jev_judgment(Name, Percent)`.
A side turn (a message typed while work runs) asks the same set. Its answers land
as `side_turn_judgment(Name, Percent)`.

| Judgment | Asked | Rule that reads it | What the user sees |
|---|---|---|---|
| `changes_files` | every turn | `jev_disputes_mutation`, `needs_clarification` (jev_evidence.mg); `side_turn_wants(/later)` (side_turns.mg) | A mutation Jev reads as no change asks first. A change request with no target asks first. A side-turn change request is queued. |
| `names_target` | every turn | `jev_disputes_mutation`, `needs_clarification` | A mutation, or a change request, that names no code asks first. |
| `hypothetical` | every turn | `jev_disputes_mutation`; `jev_restriction(/hypothetical)` | A "what if" read as a mutation asks first. The turn runs under the hypothetical restriction. |
| `rules_out_changes` | every turn | `jev_restriction(/read_only)` | "Just look, don't touch" restricts the turn to read-only (constitution.mg 7D). |
| `about_codenerd` | every turn | `self_diagnosis_lane` | Read-only background work about codeNERD's own failures becomes the self-diagnosis chain. |
| `wants_to_stop` | while work runs (`running_work`: the running turn, background jobs) | `stop_background_requested`; `side_turn_wants(/stop)` | Running background jobs are cancelled. A side-turn "stop" stops the running turn. |
| `approves_held_work` | while work is held | `approve_lane`, with... | A go-ahead that perception misread still approves the held work. |
| `adds_to_held_work` | while work is held | ...`approve_lane` needs it to read no | "Go, and also X" is not approval: it is a new request. |
| `answered_N` | one per open brief question | `brief_question_answered` (read by brief.mg) | An answered question stops holding the work, so yolo may start. |

`TestEveryTurnJudgmentHasAConsumingRule` fails when a question has no rule
reading it. `wants_answer` was asked until 2026-10-28, but nothing read it, so it
is no longer asked.

### Retrieval relevance: switch `retrieval_relevance`

The question is in `internal/turnjudge/relevance.go`, asked once per Tier 2
candidate: "Would reading this file help with `request`?" The state is
the request; each question has the file's path and head, up to
`relevance_head_bytes`, with a note when the head is cut. The keywords
retrieval matched are not sent: they are what the keyword rank already used, and
a shared word pulls the answer toward yes. The pool is
`relevance_pool_factor` times Tier 2's size, sent as parallel requests of
`batch_size` files each.

- `turnjudge.RelevanceFor` resolves the use for both retrieval passes:
  - the chat's seed (`process_seed.go`);
  - every task the session executor runs, including `nerd fix`, delegations and
    subagents (`taskRetrieverConfig`, `internal/system/factory_task_retrieval.go`).
- Fact: `context_relevance(Issue, File, Percent, Kept)`. It is keyed by issue and
  retracted with the issue.
- Rule: `retrieval_brief_file` (jev_evidence.mg). A Tier 2 file that Jev judges
  relevant goes into the model's retrieval brief whatever its keyword score.
  The keyword floor (`retrieval.brief_min_relevance`) measures word overlap;
  Jev has read the file.
- The drop is decided in Go. Inside the tier build (`judgeTier2`), a file at or
  below `judgment_no_percent` is left out of Tier 2, and Tier 2 keeps Jev's order.
  This is the one Jev decision Go makes, because it happens before any fact
  exists. `context_relevance(..., /false)` records it.
- A file the user mentioned is never judged.
- A failed judgment leaves the keyword ranking and the floor in charge. A partial
  judgment never reorders anything.

### Review checks: switch `review_checks`

The question is in `internal/turnjudge/findings.go`: "Does `code` contain the
problem that `finding` describes?" It is asked for each reviewer finding that
cites a readable file and line, with `cited_line` stated outright and `batch_size`
findings to a request. The code sent is the `finding_context_lines` lines on
either side of the citation, numbered as in the file.

- Calls: `checkReviewFindings`, for a foreground review (process.go) and a
  background review (background.go).
- Fact: `review_finding_check(Review, File, Line, Percent)`.
- Rules, at or below no:
  - `review_finding_refuted`;
  - `review_suspect(Review, "jev_refuted_finding")`, which feeds
    `reviewer_needs_validation`;
  - an `injectable_context` line telling the model to verify the finding.
- What the user sees: a "Findings Jev could not see in the code they cite" list
  beside the review. Nothing is hidden or dropped on Jev's word.

### Self-diagnosis triage: switch `diagnosis_triage`

The question is in `internal/turnjudge/triage.go`: "Was `problem` caused by a
bug in codeNERD's own code?" It is asked once for each hot problem in the
problems log, `batch_size` problems to a request, each with its level, category
and whole message. The repeat count is not sent: it does not say what caused the
problem.

- Call: `triageProblems` (self_diagnosis.go), when the chain builds its task.
- Fact: `self_problem_triage(Signature, Category, Percent)`.
- Rule: `self_problem_around` (jev_evidence.mg), at or below no.
- What the user sees: the diagnosis task works the defects first. It lists the
  problems the kernel derives as "around" apart, so they are confirmed briefly
  rather than root-caused.
- Unsettled or failed: every problem is a defect, in the log's order. The last
  chain's judgments are retracted before Jev is asked again.

## Learning and autopoiesis

The questions are in `internal/learnjudge`, one file per use. The rules are in
`policy/jev_learning.mg` (kernel decisions) or a truth table in the consuming
package. Each use has its own switch; a failed, timed-out or switched-off call
is never a "no": each use below names its fallback, and the fallback is logged.

### Correction judgment: switch `correction_judgments`

Being built (2026-10-28).

### Task verdicts: switch `task_verdicts`

Being built (2026-10-28).

### Problem types: switch `problem_types`

Being built (2026-10-28).

### Tool checks: switch `tool_checks`

Ruling: the decision is a Go truth table in `internal/autopoiesis/tool_jev.go`,
not kernel rules. The Ouroboros loop runs on its own transient engine, and its
outcome is the `LoopResult` its callers already read.

The questions are in `internal/learnjudge/toolcheck.go`. The loop builds its Jev
from the operator's typesafe section (`OuroborosLoop.initJev`); tests install
one with `SetJev`.

- **Duplicate check, before generating.** It is one request
  (`CheckDuplicate`):
  - a Choice over the registered tools: "Which of these existing tools comes
    closest to doing what `request` needs?" Each option is a tool name, and its
    criteria are the tool's description;
  - the companion Noul: "Does one of `existing_tools` already do what `request`
    needs?"
  - The request is the purpose, input and output. The proposed name is left
    out, so that a name shared with an existing tool does not pull the choice.
  - A tool being regenerated under its own name is not a candidate.
  - Over 255 tools, only the newest 255 are compared, and the log says how many
    were left out.
- **Code check, after the code passes the safety check.** It is one request
  (`CheckCode`):
  - "Does `code` do what `request` asks the tool to do?"
  - "Does `code` also do something that `request` does not ask for?"
  - The code goes whole up to 60 KB. Past that, the head and tail go, with
    `code_omitted_bytes`.

| Check | Answer | Outcome |
|---|---|---|
| duplicate | companion at or above yes | the chosen tool is reused, nothing is generated. `LoopResult.ReusedTool` names it, `ToolHandle` is it, and no generation learning is recorded. `GenerateToolFromCode` (the `generate_tool` action) refuses, naming the tool to call instead. |
| duplicate | at or below no, or between | generate |
| code | does-asked at or below no, or does-unrequested at or above yes | regenerate, with Jev's reason as a `request_mismatch` violation fed to `RegenerateWithFeedback`. This shares the safety retry budget. Past the budget, the tool is rejected. `GenerateToolFromCode` refuses, with the reason. |
| code | otherwise (yes, or unsettled) | the code goes on to the Thunderdome |
| either | Jev unconfigured, switched off, failed or timed out | as before: generate and accept. The log names why (`jev: no duplicate check ...`, `jev: ... failed, generating as before`). |

**Probe, 2026-10-28, `jev-1.14.0`.** The payloads are in
`jev-probes/2026-10-28-learning/tool_checks/`, and the summaries are in
`tool_checks-duplicate.txt` and `tool_checks-code.txt`. The probe used a catalog
of 8 tools, because this workspace has generated none.

- **Duplicate check: 9 of 9 right.**
  - The 4 duplicates scored 0.80–0.97, and the top choice was correct at 1.00.
  - The 5 non-duplicates scored 0.02–0.07. These included hard near-misses:
    - JSON→YAML against `yaml_to_json`;
    - relative-link checking against an http link checker;
    - per-package line counts against a glob line counter.
- **The companion is needed.** On the reverse-direction case, the Choice alone
  picked `yaml_to_json` at 0.98, while the companion said 0.05.
- **Code check: 5 of 5 right where it decides.**
  - A correct TODO scanner scored asked 0.81 and extra 0.16.
  - A scanner that rewrites the files scored extra 0.97, and one that posts
    results scored extra 0.98.
  - A stub scored asked 0.01, and the wrong conversion direction scored 0.03.
  - The stub's extra score was 0.34, which is unsettled and acts on nothing.

**Dogfood, 2026-10-28.** The runs used `nerd tool generate` in a scratch Go
module, with this repository's config.

- **Run 1 generated a JSON syntax checker in 4m47s.** In an earlier
  workspace, the code check read a TODO lister as asked 0.94 and extra 0.25,
  and accepted it.
- **Run 2, a reworded request with `tool_checks` on**: "validate a .json config
  and tell me where its first syntax mistake is". The duplicate check scored
  0.92, chose the checker at 1.00, and reused it. The run took 3s, and nothing
  was generated.
- **Run 3, the same request with `tool_checks: false`.** It spent 153s
  generating a second JSON checker, which then failed its own generated test.
- **The probe exposed a bug, now fixed.** `RuntimeRegistry.Restore` gave every
  tool the description "Restored from disk", because generated source carries
  no `// Description:` line. In the first workspace, the duplicate check worked
  only because the tool's name spelled out its purpose. `WriteTool` now heads
  the source with the description (`withDescriptionHeader`, toolgen.go), so a
  restart keeps what each tool does.

### Autopoiesis checks: switch `autopoiesis_checks`

As built 2026-10-28. Questions: `internal/learnjudge/autopoiesis.go`. Operation
`jev_autopoiesis_checks`. They replace the keyword quick checks
(`QuickAnalyze`, `ShouldTriggerCampaign`, `ShouldCreatePersistentAgent`,
now deleted) and the language-reading pattern rules of the tool quality
evaluator.

**A chat request** (`autopoiesis.Orchestrator.SuggestForRequest`, called from
section 1.8 of `cmd/nerd/chat/process.go` on every turn that is not a direct
answer). One request, state `request` and `target`, four Nouls:

| Judgment | Question |
|---|---|
| `several_deliverables` | Does `request` ask for several separate pieces of work, each with its own result? |
| `ordered_stages` | Does part of the work in `request` have to be finished before another part of it can start? |
| `many_files` | Will carrying out `request` take changes to many files? |
| `continues_after_request` | Does `request` ask for work that keeps going after this request has been answered? |

- Facts: `autopoiesis_judgment(Name, Percent)`; the keyword heuristic's match
  as `autopoiesis_heuristic(/campaign | /persistent)`.
- Rules (`policy/jev_autopoiesis.mg`): a campaign is suggested when two of
  several deliverables, staged work and many files read yes;
  `persistent_agent_suggested(/recurring, /jev)` when the work continues after
  the request. A Jev judgment, whatever it says, outranks the heuristic.
- What the user sees: a derived campaign runs the requirements clarifier in
  place of the turn (as the keyword match did before); a persistent agent is a
  warning naming `/define-agent`. Each names its evidence, `jev` or
  `heuristic`.
- No answer (unconfigured, switched off, failed, or past
  `turn_judgment_timeout`): the keyword heuristic decides through the same
  rules, and the log says so.

**A generated tool's output** (`QualityEvaluator.outputIssues`). Four Nouls
over the tool, its input and its output (bounded to 24,000 bytes, the
omission stated): `reports_error`, `reports_partial_results`,
`reports_more_pages`, `reports_rate_limit`. At or above yes is the issue;
below it is none, even where the old pattern matched. The exact
empty-output rule always runs. No answer: the patterns decide.

Live probe, 2026-10-28, `jev-latest`
(`go test -tags jevlive ./internal/learnjudge/ -run LiveAutopoiesis`; payloads in
`Docs/architecture/jev-probes/2026-10-28-learning/autopoiesis_checks/`):

- Requests: 30 of 31 labeled judgments right, 0 wrong, 1 unsettled ("explain
  how the JIT compiler selects atoms and how the kernel ranks them":
  several deliverables 0.34).
- Tool output: 16 of 16 right. "found 3 lines: 12: ERROR failed to connect"
  reads as a result (no error), where the old pattern flagged a failure.
- Read off the probe, not tuned to it: "implement a complete feature for
  billing" is one deliverable (0.21) built in stages (0.88) across many files
  (0.89). That is why staged work across many files is a campaign without
  several deliverables. "Rename Executor to Runner across the whole codebase"
  is many files (0.84) but not staged (0.36), so it is one task.
- "From now on, always use tabs in Go files" reads as continuing (0.86), so it
  suggests a persistent agent, as the keyword heuristic did.

### Atom checks: switch `atom_checks`

Being built (2026-10-28).

## Latency and cost

- The turn judgments run beside perception. Routing waits for them, so they are
  bounded by `turn_judgment_timeout` (default 5s), not `request_timeout`. A turn
  waits at most that long past perception. A call still outstanding at the
  deadline is no evidence. The log line names the call time and how long routing
  waited (`jev: ... (routing waited 0s)`).
- Retrieval relevance runs inside the retrieval pass's own budget
  (`retrieval.DefaultSeedTimeout`).
- Review checks and triage are bounded by `request_timeout` (default 30s,
  retries included).
- Every answered call is billed to the session's usage tracker
  (`typesafe.Client.Ask`, then `usage.RecordFromContext`):
  - provider `typesafe`;
  - the model that answered;
  - operation `jev_turn_judgments`, `jev_retrieval_relevance`,
    `jev_review_checks`, `jev_diagnosis_triage`, or a learning use's own
    (`learnjudge.Op*`, such as `jev_autopoiesis_checks`).
- `jevContext` carries the tracker to calls made off the turn (side turns,
  background reviews).
- Jev models are not in the price table, so their tokens count as
  `unpriced_tokens`, never as free.

## Config (`.nerd/config.json`)

```json
"typesafe": {
  "model": "<a model alias or versioned ID from GET /v1/models>",
  "base_url": "<the API origin>",
  "api_key_env": "TYPESAFE_API_KEY"
}
```

An absent section means Jev is off. A section that cannot resolve logs why and
leaves Jev off. No model or endpoint is defaulted.

| Key | Default | Read by |
|---|---|---|
| `request_timeout` | 30s | every call (client) |
| `turn_judgment_timeout` | 5s | turn and side-turn judgments |
| `judgment_yes_percent` / `judgment_no_percent` | 70 / 30 | every rule above; the retrieval drop |
| `turn_judgments`, `retrieval_relevance`, `review_checks`, `diagnosis_triage` | on | the use of that name |
| `correction_judgments`, `task_verdicts`, `problem_types`, `tool_checks`, `autopoiesis_checks`, `atom_checks` | on | the learning and autopoiesis use of that name |
| `recent_turns` | 12 | turn judgments |
| `batch_size` | 20 | retrieval relevance, review checks, triage |
| `relevance_pool_factor` | 6 | retrieval relevance |
| `relevance_head_bytes` | 3000 | retrieval relevance |
| `finding_context_lines` | 12 | review checks |

## Live probe

`go run ./cmd/tools/jev_probe -config .nerd/config.json -runs 2` scores every
turn judgment against the labeled cases in
`internal/turnjudge/testdata/cases.json`. `-uses internal/turnjudge/testdata/uses_cases.json`
adds retrieval relevance, review checks and triage; `-variants FILE` asks
several wordings of one judgment as separate questions over the same state;
`-record DIR` keeps every exact request and reply (no key: it lives in a
header the recorder never reads). Payloads from the runs below are in
`Docs/architecture/jev-probes/2026-10-28-jev-latest/`.

2026-10-28, model alias `jev-latest` (the versioned `jev-1.14.0` is accepted but
`GET /v1/models` lists only `jev-latest` and `jev-preview`; pin a versioned ID in
`typesafe.model` once thresholds are tuned), thresholds 70/30:

| Set | Cases x runs | Right | Wrong | Unsettled |
|---|---|---|---|---|
| Tuning set (`cases.json`, 51 cases, the set the first wording was fitted to) | 51 x 2 | 180 of 186 | 2 | 4 |
| Holdout (`holdout_cases.json`, 31 new cases written before they were run) | 31 x 2 | 106 of 108 | 0 | 2 |

Misses, all on the safe side:

- "do it but skip the docs part", "go, and also add a test for it" and "yes, and
  update the changelog too" read 0.25 to 0.65 on `approves_held_work`. The
  question approves starting the work as it is, so these are not approvals; the
  labels in the case files predate that wording. `adds_to_held_work` reads them
  yes in every run.
- "review my changes in internal/approval" reads 0.56 to 0.58 on `names_target`.
  It is read-only, so no rule reads it.

Wording comparison (`-variants`, `variants-names_target.json`): four wordings of
`names_target` over the same state. A rewording to "Is it clear which ... in
`message`?" was worst (3 unsettled of 14); "Does the user's request say which
file, function or feature they want to work on?" (goal anchored) and the first
wording read 13 and 13 of 14 right, none wrong; all four were 8 of 8 on the holdout.
The goal-anchored wording is the one in use.

The other uses, same day (`uses_cases.json`, real repo files and inline source):

- Relevance: 14 of 15 right across three requests, one unsettled; the files that share
  a word with the request (`retry` in the embedding client, `approval` in a Mangle
  grant path) read 0.06 to 0.21. `internal/typesafe/usage.go` for "how is an unpriced
  model billed" read 0.31: a defensible miss, billing is decided in `usage/pricing.go`.
- Review checks: 7 of 7 right with the cited line stated (the same finding on
  guarded or parameterised code reads 0.02 to 0.05; a finding whose line is in
  the wrong function reads 0.05).
- Triage: 3 of 3 defects (0.80 to 0.84) and 5 of 5 surroundings (0.04 to 0.10);
  a parse error in a policy file (0.55), a deadline exceeded (0.17) and a slow
  evaluation (0.26) are the hard cases and are not scored.

Not yet probed: long Tier 2 heads, many-turn conversations near the 64k
context limit, and any model other than `jev-latest`.

## Tests

| Use | Test |
|---|---|
| Client: failure is an error; usage is billed by operation | `internal/typesafe/client_test.go`, `usage_test.go` |
| Config: switches, tuning, `turn_judgment_timeout` | `internal/config/typesafe_test.go` |
| Turn judgments: request shape, fact projection, failure is no evidence; every question has a rule | `internal/turnjudge/judge_test.go`, `consumers_test.go` |
| Turn judgments in chat: go-ahead, disputed mutation, unnamed change, answered question, stop, failed call, restriction, self-diagnosis lane | `cmd/nerd/chat/journey_jev_test.go`, `jev_uses_test.go` |
| Slow Jev bounded; usage billed; each switch off asks nothing | `cmd/nerd/chat/jev_uses_test.go` |
| Retrieval: reorder and drop, failure keeps the ranking, issue-keyed facts | `internal/retrieval/relevance_test.go` |
| Retrieval: a relevant file passes the floor; unsettled and failed do not; facts leave with the pass | `TestTaskRetriever_JevRelevantFileReachesTheBriefPastTheFloor` |
| Retrieval wiring for task passes | `internal/system/factory_task_retrieval_test.go` |
| Review checks | `TestCheckFindings_JudgesCitedCodeAndSkipsUncited`, `TestJevJourney_RefutedReviewFindingIsFlagged` |
| Triage, including the failed case, batching, and the count left out | `TestTriageProblems_FactsAndKernelSplit`, `TestTriageProblems_BatchesAndLeavesOutTheCount`, `TestJevJourney_TriageListsFailuresAroundCodeNERDApart` |
| Request shape: held and running work as fields, stop asked only while work runs, omitted turns counted, one condition per question | `internal/turnjudge/judge_test.go` |
| Relevance carries no matched keywords and says when a head is cut; findings state the cited line; batches keep order and fail whole | `internal/turnjudge/uses_test.go` |
| Unsettled judgments are named and act on nothing; running work as a person reads it | `cmd/nerd/chat/jev_calibration_test.go` |
