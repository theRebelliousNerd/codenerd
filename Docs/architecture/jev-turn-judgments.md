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
unsettled and acts on nothing.

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
| `wants_to_stop` | every turn | `stop_background_requested`; `side_turn_wants(/stop)` | Running background jobs are cancelled. A side-turn "stop" stops the running turn. |
| `approves_held_work` | while work is held | `approve_lane`, with... | A go-ahead that perception misread still approves the held work. |
| `adds_to_held_work` | while work is held | ...`approve_lane` needs it to read no | "Go, and also X" is not approval: it is a new request. |
| `answered_N` | one per open brief question | `brief_question_answered` (read by brief.mg) | An answered question stops holding the work, so yolo may start. |

`TestEveryTurnJudgmentHasAConsumingRule` fails when a question has no rule
reading it. `wants_answer` was asked until 2026-10-28, but nothing read it, so it
is no longer asked.

### Retrieval relevance: switch `retrieval_relevance`

The question is in `internal/turnjudge/relevance.go`, asked once per Tier 2
candidate: "Would reading this file help answer or carry out `request`?" The
state includes the file's head, up to `relevance_head_bytes`. The pool is
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

The question is in `internal/turnjudge/findings.go`: "Does `code` show the
problem that `finding` describes?" It is asked for each reviewer finding that
cites a readable file and line. The code sent is the `finding_context_lines`
lines on either side of the citation.

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

The question is in `internal/turnjudge/triage.go`: "Is `problem` a defect in
codeNERD's own code, rather than a failure of something around it?" It is asked
once for each hot problem in the problems log.

- Call: `triageProblems` (self_diagnosis.go), when the chain builds its task.
- Fact: `self_problem_triage(Signature, Category, Percent)`.
- Rule: `self_problem_around` (jev_evidence.mg), at or below no.
- What the user sees: the diagnosis task works the defects first. It lists the
  problems the kernel derives as "around" apart, so they are confirmed briefly
  rather than root-caused.
- Unsettled or failed: every problem is a defect, in the log's order. The last
  chain's judgments are retracted before Jev is asked again.

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
    `jev_review_checks` or `jev_diagnosis_triage`.
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
| `recent_turns` | 12 | turn judgments |
| `batch_size` | 20 | retrieval relevance, review checks |
| `relevance_pool_factor` | 6 | retrieval relevance |
| `relevance_head_bytes` | 3000 | retrieval relevance |
| `finding_context_lines` | 12 | review checks |

## Live probe

`go run ./cmd/tools/jev_probe -config .nerd/config.json -runs 3` scores every
turn judgment against the labeled cases in
`internal/turnjudge/testdata/cases.json`.

First tuning pass, 2026-10-28, `jev-1.14.0`, 46 cases × 3 runs:

- 294 of 303 judgments right, **0 wrong**, 9 unsettled (all in the middle band,
  where the rules act on nothing).
- 114k input tokens, 8.2 s.
- These counts include `wants_answer`, which is no longer asked.

That pass showed that `changes_files` reads yes for a bare "go" over held work
that changes files, which is fair. So approval asks `adds_to_held_work` instead
of reusing `changes_files`.

Unsettled cases worth watching:

- "go, and also add a test" on `approves_held_work` lands in the middle. That is
  the safe outcome, since it is a new request.
- "review my changes in internal/approval" on `names_target` scores about 0.65.
  It is read-only, so no rule reads it.

On codeNERD's own repository, 2026-10-28, the keyword ranking and the
Jev-judged Tier 2 compared like this:

| Request | Keyword Tier 2 | Jev-judged Tier 2 |
|---|---|---|
| "why does the approval broker open a second request for a duplicate call" | a skill ledger, audit docs, `sparse.go`… | `approval/broker.go`, `session/human_approval.go`, `mcp/remote_approval_test.go`, `approval/request.go`… (120 judged, 98 dropped, 756 ms) |
| "the footer does not wrap in the TUI" | 20 files, mostly agent and QA docs | `cmd/nerd/chat/view.go` |

That probe also exposed a keyword bug. `ExtractKeywords` took only code syntax,
so a plain-English request searched for nothing. It now also takes prose
identifiers and domain terms, at the lowest weight.

The workspace's `.nerd/config.json` has no typesafe section as of this writing,
so the current build has not been probed live.

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
| Triage, including the failed case | `TestTriageProblems_FactsAndKernelSplit`, `TestJevJourney_TriageListsFailuresAroundCodeNERDApart` |
