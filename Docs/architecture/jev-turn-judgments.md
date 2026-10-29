# Jev judgments

Status: as built, 2026-10-28.

Code:

- `internal/typesafe`: the client, including usage billing (`usage.go`).
- `internal/turnjudge`: every question and every fact projection.
- `cmd/nerd/chat/jev_turn.go` and `cmd/nerd/chat/self_diagnosis.go`: chat wiring.
- `internal/system/factory_task_retrieval.go`: task retrieval wiring.
- `internal/core/defaults/policy/jev_evidence.mg`, `conversation_acts.mg` and `side_turns.mg`: the rules.
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
- Judgments are Nouls because each is one yes/no condition, except the
  conversation act, a Choice (below): routing takes one lane, so it needs the
  act a turn mainly is. A decision never goes to Jev ("should we ...") because
  the rules in `jev_evidence.mg` and `conversation_acts.mg` hold the truth
  table.
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
| `about_codenerd` | every turn | `self_diagnosis_lane`; `wants_direct_answer` (self_diagnosis.mg) | Read-only background work about codeNERD's own failures becomes the self-diagnosis chain. A /self_diagnose question it reads as no is answered directly. |
| `conversation_act` (Choice) | every turn | `jev_chosen` (jev_evidence.mg), then `conversation_act_route` (conversation_acts.mg) | A reply, a remark, a question about codeNERD, pasted material, a stop or small talk that perception put on a task verb is answered in the conversation, or asks first when read as a mutation. See below. |
| `wants_to_stop` | while work runs (`running_work`: the running turn, background jobs) | `stop_background_requested`; `side_turn_wants(/stop)` | Running background jobs are cancelled. A side-turn "stop" stops the running turn. |
| `approves_held_work` | while work is held | `approve_lane`, with... | A go-ahead that perception misread still approves the held work. |
| `adds_to_held_work` | while work is held | ...`approve_lane` needs it to read no | "Go, and also X" is not approval: it is a new request. |
| `answered_N` | one per open brief question | `brief_question_answered` (read by brief.mg) | An answered question stops holding the work, so yolo may start. |

`TestEveryTurnJudgmentHasAConsumingRule` fails when a question has no rule
reading it. `wants_answer` was asked until 2026-10-28, but nothing read it, so it
is no longer asked.
`asks_assistant_state` was asked until 2026-10-28 (late); the conversation act
`question_about_codenerd` replaced it, for every verb, not only
`/self_diagnose`.

### Conversation act: switch `turn_judgments`

Status: as built, 2026-10-28. Code: `turnjudge.conversationActQuestion`,
`policy/conversation_acts.mg`, the Choice reading in `policy/jev_evidence.mg`.

The task vocabulary (43 verbs in `perception.DefaultTaxonomyData`, 18 mirrored
as `verb_def` rows) has no word for a turn that is not a task. A Choice over the
43 verbs plus a "does any fit" Noul read 50 of the user's 70 real messages as a
weak fit: "yes", "done", "save", "that's bad", a pasted log, "what mode are you
in?". Perception forced them onto the nearest verb, and the kernel routed them as
work. Ruling: a conversation-act layer beside the task verb, not more verbs: an
act is what the user does in the conversation, a verb is what work they ask for,
and one turn has both. The task verb stays perception's (clear task requests
already read 0.96 to 1.00, and a second verb reader would need a disagreement
table with no ground truth).

One Choice, `conversation_act`, asked every turn with the other judgments, over
`request_for_work`, `reply_to_codenerd`, `question_about_codenerd`,
`feedback_on_codenerd`, `correction_of_codenerd`, `shared_material`,
`stop_request`, `small_talk`. No companion "does any fit" Noul: the options leave
nothing uncovered, and the companion never read no on 78 real messages (lowest
0.45). The whole distribution reaches the kernel as `jev_choice(Question,
Option, Percent)`.

The kernel reads it in two steps:

1. `jev_choice_top` is a merge lattice (`fundep` + `merge`, keyed by question,
   value `fn:pair(Option, Percent)`): a higher option replaces the held one in
   the fixpoint, and two options at the same percent are both kept, so a tie is
   visible (`jev_choice_tied`). `jev_chosen(Question, Option)`: the top option,
   alone, at or above yes. Otherwise `jev_choice_unsettled`, named in the chat
   log beside the unsettled Nouls. The lattice is codeNERD's first; it needed a
   kernel fix (below).
2. `conversation_act_route(Act, Disposition)`, with no work held (held work is
   the go-ahead Nouls' to read):

| Act | Perception reads | Disposition |
|---|---|---|
| `request_for_work` | anything | none: perception routes |
| question, feedback, material, stop, small talk | not a mutation, no file or tools needed | `/answer_in_conversation` (`wants_direct_answer`) |
| `question_about_codenerd` | not a mutation, no file named (tools may be) | `/answer_in_conversation`, from the operating state |
| question, feedback, material, stop, small talk | a mutation | `/ask_first` (`jev_disputes_mutation`, the clarify lane) |
| any of those | `about_codenerd` yes | none: the self-diagnosis chain may take it |
| reply, correction | a target | none: the task it carries is perception's |
| reply, correction | no target, no file or tools | `/answer_in_conversation` |
| any | unsettled, tied, or an option no row maps | none (`conversation_act_unmapped` is logged as an error) |

The act only ever answers in the conversation or asks first; it never starts
work. `decideRoute` logs each `conversation_act_route` row. A side turn asks the act with
the rest but reads only the Nouls (`side_turn.go` projects `jev_judgment`), so
its act is not used yet.

Kernel fix found on the way: cone evaluation ran only the cone's own rules, so
a predicate the engine runs top-down (a `deferred()` premise, a lattice's merge
predicate) had no rules there. Every lattice row was kept as incomparable, on
the cone path only (full evaluation was right). `coneIndex.topDown` now carries
those rules with the cone, and a merge predicate is upstream of its lattice
(`TestConeEvaluationMatchesFullEvaluation_LatticeAndDeferred` fails without the
fix). The capability manifest's `deferred_and_merge_evaluation` moved from
unverified to production; generated rules still may not declare either. Two
engine rules a new lattice must keep: the merge is a total preorder (one that
answers both ways keeps a row that depends on arrival order), and anything the
merge reads is named in the lattice rule's body too. mangle-go v0.7 orders
strata by rule bodies only, never by a merge predicate, so a merge reading a
derived predicate can run before it exists and keep every tied row.
`jev_choice_higher` reads no predicate.

Vocabulary fix beside it: "add a test" and "write a test" are `/test` synonyms
now. A verb Choice split "add a test for Max" 0.57 `/test` to 0.42 `/create`
and read "write table-driven tests" as `/write` (0.62); with them, 1.00 and 0.90
`/test`, and "add a Max function" stays `/create` (1.00).

Live probe, Jev 1.14, thresholds 70/30, production request (every turn
question in one call, `jev_probe -cases`):

| Set | Cases x runs | Right | Wrong | Unsettled |
|---|---|---|---|---|
| Holdout (`act_cases.json`, 20 written before they were run) | 20 x 2 | 37 | 0 | 3 |
| The user's real messages with their 12 prior turns (kept out of the repo) | 78 x 1 | 57 | 3 | 18 |
| The tuning set (`cases.json`, the other judgments, after the change) | 51 x 1 | 90 of 93 | 1 | 2 |

The three wrong real readings route the same as their labels: two northstar
wizard answers read as feedback, a complaint read as a correction. The
unsettled ones are mostly wizard answers and bare paths, and route as before.
Payloads: `Docs/architecture/jev-probes/2026-10-28-conversation-act/`.

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

## Prompt atoms

Code: `internal/atomjudge` (both questions), `internal/prompt/importance.go`,
`internal/prompt/relevance.go`, `cmd/tools/prompt_builder -score-importance`,
rules in `jit_compiler.mg` and `policy/jit_selection.mg`. Spec:
`Docs/architecture/prompt/IMPLEMENTED_SPEC.md` sections 5.2 and 6.3.

### Atom importance: build time, no switch

A Score per embedded atom: "If the coding agent ignored `guidance`, what would
most likely happen?" The five levels are written situations, from "changes only
style or wording" to "breaks the build, loses or corrupts data, or violates
safety or permissions". The state is one line saying what a guidance block is;
each atom is its own question, carrying its id, category, scope and full text.

- Asked by `prompt_builder -score-importance`, never at runtime. The answers
  live in the embedded ledger `internal/prompt/atom_importance.json`, with the
  content hash judged, the probabilities and the Jev model ID.
- Fact: `atom_importance(Atom, Level)`, only for an atom with a current score.
- Rules: `conflict_loser` and `candidate_atom` (`policy/jit_selection.mg`). Go
  orderings use it as a tie-break. It replaced the hand-set `priority`, which
  is no longer read by anything.
- Mandatory atoms are scored but stay rule-selected. An unscored atom, or one
  edited since it was scored, has no importance; it is never given a stand-in
  level.
- Billed as `jev_atom_importance`.

### Atom relevance: switch `atom_relevance`

A Noul per flesh atom the vector search nominates: "Does `guidance` apply to the
work in `request`?" The state is `request{text, intent, target, language,
frameworks, agent}`; each atom is its own question.

- The pool is the vector channel's slots plus the next most similar eligible
  atoms, up to `atom_relevance_pool`. Mandatory atoms are never asked.
- Asked once per turn: the answers are cached by request and atom content, so
  the many compiles of one turn ask once. Bounded by `atom_relevance_timeout`.
- Facts: `atom_relevance(Atom, Percent)` and
  `atom_relevance_threshold(/yes|/no, Percent)`, per compile and ephemeral.
- Rules (`jit_compiler.mg`, the flesh admission table):
  - at or above yes: admitted from anywhere in the pool (`/judged_applies`);
  - unsettled: admitted only from a vector slot;
  - at or below no: refused, even from a vector slot (`flesh_refused`);
  - not judged (off, failed, late or incomplete): the vector slots, as before.
- Cache-aware: the turn's probabilities never order the prompt. A task
  (agent, verb, target) carries the flesh its previous judged compile admitted;
  carried atoms stay through an unsettled reading, leave only when Jev reads
  them as not applying, and assemble in their previous order ahead of new flesh
  (`flesh_admitted(_, /carried)`, assembler `tierCarried`). New flesh follows by
  importance, then ID, and is shed first.
- The log names the pool and timing, then the kernel's admitted and refused rows
  (`logRelevanceAdmission`).
- Billed as `jev_atom_relevance`.

### Atom relevance: dogfood A/B, 2026-10-28

The runs were `nerd chat` turns against codeNERD's own source, comparing
`atom_relevance` on and off. Costs come from the session usage tracker on the
built-in price table: uncached input, cache writes, cache reads and output. Jev
adds about $0.0013 per judged compile (30k tokens at $0.042/Mtok).

| Task (agent, provider) | Runs on / off | Prompt tokens on / off | $ per session on / off | Cache-read share on / off | Outcome |
|---|---|---|---|---|---|
| explain JIT flesh selection (explain, Muse) | 2 / 2 | 9.7k, 9.8k / 7.5k, 7.5k | 0.0066, 0.0060 / 0.0045, 0.0082 | 25%, 59% / 52%, 80% | answered in all runs, same substance |
| explain blocked_by_missing_tool (explain, Muse) | 2 / 2 | ?, 8.3k / 8.0k, 30.0k | 0.0066, 0.0052 / 0.0050, 0.0058 | 48%, 32% / 69%, 73% | answered in all runs |
| review broker.go for races (reviewer, Opus) | 1 / 1 | 30.5k / 28.3k | 2.08 / 1.18 | 10% / 0% | both died on a provider 400 |
| TUI footer wrap (reviewer, Opus) | 2 / 2 | 19.7k / 17.5k | 0.699, 0.695 / 0.643, 0.644 | 0% / 10% | all died on a provider 400 |
| 3-turn broker task, run ABBA (explain, researcher, reviewer) | 2 / 2 | per turn 7.4-9.2k, 19.2-20.6k / 8.1k, 8.9k, 17.9k | 0.712, 1.267 / 1.112, 0.620 | 44%, 39% / 50%, 49% | turns 1-2 answered in all runs; turn 3 died on a provider 400 in all runs |

What the runs show:

- **Selection.** Jev admitted the atoms the work needed from beyond the vector
  slots: Go concurrency exemplars, errgroup, the resource-leak checklist and the
  review exemplars for a race review; lipgloss layout for the TUI turn. It
  refused off-topic slots: another repository's broker docs, build-system notes
  and an unrelated wiring auditor.
- **Size.** On the Mangle explain, one off run admitted 128 flesh atoms (30.0k
  tokens), because an admitted head pulls in all of its sections; Jev refused
  those heads (8.3k tokens). Elsewhere Jev's prompts were 0.7k to 2.7k tokens
  larger.
- **Cost.** No dollars-per-verified-task win is shown. On the Opus reviewer,
  Jev-on sessions cost 8% more ($0.05) for the larger prompt. The other
  differences are inside run-to-run variance from tool-loop length.
- **Order effect.** The run order moves the cache share: a run started within
  the provider's cache window of a similar run reads that run's prefix.
- **Carry-over did not engage.** Each turn of the three-turn task went to a
  different agent, and different agents share no persona prefix.
- **The 400.** Every Opus reviewer turn died on the tool-result follow-up, on
  and off alike. `perception.pooledCallError` drops the provider's error body,
  so the cause is not in the logs.

## Learning and autopoiesis

The questions are in `internal/learnjudge`, one file per use. The rules are in
`policy/jev_learning.mg` (kernel decisions) or a truth table in the consuming
package. Each use has its own switch; a failed, timed-out or switched-off call
is never a "no": each use below names its fallback, and the fallback is logged.

### Correction judgment: switch `correction_judgments`

Whether the user's latest message corrects codeNERD's last turn. It decides
whether the turn is learned from (`learnFromCorrection`,
`cmd/nerd/chat/learning_auto.go`): beside the turn, which runs as asked. Jev
picks the verb the misread request asked for (`learnjudge.ChooseVerb`, a Choice
over the verb corpus with a "does any verb fit" Noul), and the mapping is
learned only when both read at or above yes; nothing asks the user. Until
2026-10-28 the learning loop (an LLM critic, then "Should I learn it?") ran in
place of the turn and ended "You can retry the request when ready". Until 2026-10-28 `handleSubmit` matched keywords before
perception, so a short "wrong" or "mistake" hijacked the turn whatever it meant.

- Question: `internal/learnjudge/correction.go`. One Noul, `corrects_last_turn`:
  "In `message`, is the user telling codeNERD that something in
  `codenerd_reply` was wrong?" The state is the request codeNERD's last turn
  answered, that reply (bounded at 24000 bytes, head and tail, with
  `codenerd_reply_omitted_bytes` when cut), and the message. With no reply yet
  there is nothing to correct: nothing is asked and nothing is learned.
- Call: `startCorrectionJudgment` beside perception (`cmd/nerd/chat/jev_correction.go`),
  bounded by `turn_judgment_timeout`; `recordCorrection` after the turn
  judgments land, before routing. It is a separate request from the turn
  judgments (a follow-up could fold it in, since the state overlaps).
- Facts: `correction_judgment(Percent)`; `correction_rebuke_phrase()` when
  `perception.IsRebuke` matches (the old keyword reading, kept as fallback
  evidence). Both are retracted every turn.
- Rule: `learning_correction(Source)` (`policy/jev_learning.mg`):

  | Jev judgment | Rebuke phrase | `learning_correction` |
  |---|---|---|
  | at or above yes | any | `/jev`: learned from, beside the turn |
  | below yes (no or unsettled) | any | none: the turn routes as usual |
  | none (off, switched off, failed, timed out) | yes | `/rebuke_phrase` |
  | none | no | none |

- What the user sees: "stop", "the tests are wrong, fix them" or "what's wrong
  with X?" route as ordinary turns; "no, I said Max, not Min" starts the
  learning loop with no rebuke word in it. Every fallback logs why
  (`correction: no Jev judgment ...`, `correction_judgments is off`).
- Probe (`jev-1.14.0`, 17 cases, `jev-probes/2026-10-28-learning/correction_judgments`):
  14 right, 0 wrong, 3 unsettled. The unsettled ones are messages about code
  that use "wrong" ("the expected values in stats_test.go are wrong", in a file
  codeNERD had just edited, 0.58; "what's wrong with returning an error
  instead?", 0.47; a new bug report after "thanks", 0.32). Unsettled learns
  nothing, so each routes as an ordinary turn. "stop" scored 0.02.
- Tests: `TestCorrection_TruthTable` (every row, Jev failing, switch off, no
  Jev), `TestCorrection_FirstTurnAsksNothing`, `TestLastExchange`
  (`cmd/nerd/chat/jev_correction_test.go`); `internal/learnjudge/correction_test.go`;
  `TestIsRebuke` (`internal/perception/rebuke_test.go`).

### Task verdicts: switch `task_verdicts`

How a past task went, for prompt evolution (`RunEvolutionCycle`, step 1). It
replaces, for every record Jev settles, the LLM judge that parsed a PASS/FAIL
verdict and a category out of prose (`judge.go`, `mapToErrorCategory`).

- Questions (`internal/learnjudge/taskverdict.go`), one request per record:
  `did_what_was_asked` and `stopped_early` (Nouls); `file_needed_N`, one Noul per
  changed file ("Does `request` call for changing `file`?"); `failure_kind`, a
  Choice over seven named kinds (logic_error, syntax_error, api_misuse,
  edge_case, context_miss, instruction_miss, hallucination); and its companion
  `failure_kind_fits`. The state is the request, the result (reported success,
  test counts, build errors, the final output bounded at 12000 bytes), the
  changed files and the actions. Code facts are state, not questions.
- Decision: `verdictFromJudgments` (`prompt_evolution/jev_verdict.go`):

  | failed (kernel, build or tests) | did | stopped early | changed a file not asked for | verdict |
  |---|---|---|---|---|
  | yes | any | any | any | FAIL |
  | no | no | any | any | FAIL |
  | no | any | yes | any | FAIL |
  | no | any | any | yes | FAIL |
  | no | yes | no | none, none unsettled | PASS |
  | no | otherwise | | | unsettled |

  A FAIL's category is the `failure_kind` Choice, or `NO_AGENT_MISTAKE` when the
  companion reads no kind fitting (a timeout, an outage). The explanation is
  composed from the readings; `improvement_rule` is left empty, because the atom
  generator writes the atom.
- Ruling: the LLM judge is kept as the cascade's second stage. It judges a
  record only when Jev's readings are unsettled, or when Jev gave no answer
  (off, `task_verdicts` off, failed). No record gets both, and each fallback is
  logged (`Task verdicts: Jev settled N of M`).
- In production every record that reaches the judge is a turn the kernel saw
  fail (`/hollow`, `/failed`; `/done` turns carry a kernel PASS). So the table's
  first row always applies, and what Jev adds there is the kind and whether it
  was the agent's mistake at all.
- Probe (`task_verdicts`, 10 records): 22 of 23 labeled readings right, 0 wrong,
  1 unsettled (a validator rejection, `failure_kind_fits` 0.68). Provider
  timeout and 429 read as no agent mistake (0.12, 0.16), so they no longer feed
  evolution. A stray edit to `kernel.go` during a README typo fix read 0.04 on
  `file_needed`.

### Problem types: switch `problem_types`

What kind of work a task is: the failure group a record joins and the
strategies the prompt compiler serves (`StrategyAtomProvider`).

- Questions (`internal/learnjudge/problemtype.go`): `problem_kind`, a Choice
  over the 16 `ProblemType` values with a description each, and the companion
  Noul `problem_kind_fits`.
- Decision: `classifyWithJev` (`prompt_evolution/classifier.go`): companion no
  gives `other` (`ProblemOther`); yes or unsettled gives the Choice. Answers are
  cached per task text (512 entries), because the compiler asks on every model
  call of a turn. On the compile path the call is bounded by
  `turn_judgment_timeout`.
- Fallback: the regex patterns and keyword boosters, logged
  (`Problem type: no Jev answer`), when Jev is off, `problem_types` is off, or
  the call fails.
- Probe (`problem_types`): 10 of 10. The regex classifier had misfiled 3 of the
  5 tasks recorded in `.nerd/prompts/evolution.db`: "fix all known bugs" as
  research, a codebase knowledge query as config_setup, and a retry of the
  northstar feature as code_review. Jev filed them as debugging, research and
  feature_creation. A greeting and a weather question scored 0.01 and 0.05 on
  the companion, so they became `other`.

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

### Tool-need trigger: switch `tool_checks`

Whether a request, and a failed attempt at it, is a capability gap that a
generated tool should fill (`ToolGenerator.DetectToolNeed`, reached from
`Orchestrator.DetectToolNeed`, `Analyze`, the chat `generate_tool` action, the
campaign gatherer and `nerd tool`). Until 2026-10-28 Go decided: a
`missingCapabilityPatterns` regex over the input, or any non-empty failed
attempt, made a need, and `toolTypePatterns` picked the kind.

- Questions (`internal/learnjudge/toolneed.go`), in one request:
  - `asks_for_tool`: is the user asking for a new tool?
  - `needs_missing_operation`: does the request need an operation that neither
    the harness (stated in the state) nor any generated tool provides?
  - `failed_for_missing_tool`, asked only with a failed attempt: did it fail
    because no tool could perform an operation, rather than a wrong argument,
    bad code, a failing test or a transient error?
  - `tool_kind`, a Choice: a hint to the spec writer.
- Facts and rule: `tool_need_judgment(Request, Name, Percent)` and the regex
  match as `tool_need_pattern(Request)`, keyed by a hash of the request, and
  retracted once read. `tool_gap_observed(Request, Source)` is derived in
  `policy/jev_learning.mg`:

  | asks_for_tool | failed_for_missing_tool | needs_missing_operation | gap |
  |---|---|---|---|
  | yes | any | any | `/jev` |
  | not yes | yes | yes | `/jev` |
  | otherwise | | | none |
  | no judgment (Jev off, `tool_checks` off, failed), pattern matched | | | `/pattern` |

  A failed attempt alone is never a gap, with or without Jev.
- The Orchestrator installs the decider when it attaches its kernel
  (`SetKernel`). A ToolGenerator with no kernel uses the pattern alone, and the
  log says so.
- The LLM (`refineToolNeedWithLLM`) now only writes the spec, after the kernel
  has derived a gap. It no longer returns `needs_new_tool`.
- Probe (`tool_need`, 10 cases): the kernel table on Jev's readings was right
  10 of 10; the old trigger 5 of 10. The old trigger made tool needs out of a
  provider timeout, a failing test, a bad regex argument, and the question "is
  there no tool for listing goroutines?". Jev scored all four at or below 0.10
  on `failed_for_missing_tool` or `asks_for_tool`. An Avro or PDF file the
  harness could not read scored 0.94 and 0.87.
- Tests: `TestToolGap_KernelTruthTable`,
  `TestDetectToolNeed_LLMWritesTheSpecOnlyForAGap` (`tool_need_test.go`), and
  `TestDetectToolNeed_FailedAttemptAloneIsNoNeed`.

### Tool value: switch `tool_checks`

Jev's observations about generated tools, as kernel evidence for the
tool-value rules. A tool earns its place by adding a capability the harness
lacks, condensing the search space, or doing far more work than it costs to
describe, call and verify. Cost, condensation and use counts are the measured
ledger's. Jev observes what code cannot (`internal/autopoiesis/tool_value.go`,
`internal/learnjudge/toolvalue.go`):

- Per call: `gives_what_was_asked`, "Does `output` give the caller what `input`
  asked the tool for?" It is asked in the same request as the tool-output
  judgments, so it costs no extra call. Fact: `tool_call_hit(Tool, Call, Percent)`.
- Per tool, on demand (`nerd autopoiesis tools`, which prints the evidence):
  - the generation-time duplicate check, reused with the tool's own
    description against every other tool: `tool_duplicate_judgment(Tool, Other, Percent)`;
  - `harness_covers`, "Could the harness, or one ordinary shell command,
    already do what `tool` does?": `tool_harness_judgment(Tool, Percent)`.
- Rule: `tool_value_evidence(Tool, Kind)` (`policy/jev_learning.mg`):
  - `/hits_mark`: some call landed and none missed;
  - `/misses_mark`: some call missed and none landed;
  - a mix of both derives nothing, and the ledger decides;
  - `/duplicate` and `/harness_covers`: at or above yes.
- No answer is no fact, so the rules then see only the measured ledger.
- Probe (`tool_value`): 14 of 14. `line_count`, `grep_todo` and `go_vet_pkg` read
  as harness-covered (0.91 to 0.96); Avro, Mangle-stratify and PDF-table tools
  did not (0.07 to 0.08). An error output, a wrong-format answer and a
  2-of-48000 partial result all scored at or below 0.07 on
  `gives_what_was_asked`.

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

The evolution trigger: whether a failure group buys a generated atom. Until
2026-10-28, any `MinFailuresForEvolution` (3) failures with the same
regex-classified problem type bought one.

- Code first (`agentMistakes`, `prompt_evolution/jev_evolution.go`): a failure
  whose verdict is `NO_AGENT_MISTAKE` is not evidence, and the group must still
  reach the count without it.
- Question (`internal/learnjudge/sharedcause.go`): `shares_one_cause`, "Do the
  failures in `failures` share one cause that a single instruction to the agent
  could prevent?" The state holds each failure's task, its explanation and its
  kind, at most 12 failures, each bounded at 2000 bytes.
- Decision (`groupWarrantsEvolution`): yes generates. No or unsettled buys
  nothing and counts the group in `EvolutionResult.GroupsWithoutSharedCause`.
  The evidence stays unconsumed, so later failures can join it. With no answer
  (Jev off, `atom_checks` off, the call failed), the count rule decides as
  before, and the log says so.
- Probe (`atom_checks-shared_cause`): 4 of 4. Three hollow test claims scored
  0.96 and three empty-slice panics 0.98. A timeout, a stray edit and a syntax
  error scored 0.03, and three provider outages 0.04.

The promotion check, under the same switch: before auto-promotion moves an
evolved atom into the agent's instructions (`RunEvolutionCycle`, step 4).

- Questions (`internal/learnjudge/atomcheck.go`): `describes_one_task` ("Is
  `candidate` about one specific past task rather than guidance for many
  tasks?"), and `contradicts_N`, one Noul per atom already promoted for the same
  shard ("Would following `candidate` mean going against `atom`?"). At most 40
  comparisons; the state says how many were left out.
- Decision (`promotionHoldReason`, `prompt_evolution/jev_promotion.go`): one
  task yes or unsettled holds; any contradiction yes or unsettled holds; a clean
  no on all promotes. A held atom stays pending for the operator. With no
  answer, the usage threshold (`ShouldPromote`) decides as before, logged. While
  the check is on, promotion at record time waits for the cycle, so nothing
  skips the check. An operator's `PromoteAtom` is not checked: the decision is
  the operator's.
- Scope: it compares against promoted evolved atoms only, not the shipped atom
  corpus. Comparing against the corpus would mean selecting from it, which is
  the JIT selector's job.
- Probe (`atom_checks-promotion`, 6 candidates against 3 promoted atoms): 23 of
  24 right. One wrong: "Max must return 0 ...; TestMaxEmpty covers it" read as
  going against "run a test before claiming it passes" (0.81). That candidate
  is held anyway, on `describes_one_task` 0.98.

### Commit authorization: switch `turn_judgments`

Whether the user's own message tells codeNERD to commit ("fix X, then commit
it"). Until 2026-10-28 the shell-effect gate refused every mutating command,
and nothing could grant the task scope, so a coder that finished its edits
could not commit them.

- Question: `asks_to_commit` in `internal/turnjudge/questions.go`, one of the
  per-turn judgments, about `message` only: an assistant line or pasted tool
  output that mentions committing is not the user asking.
- Rule: `jev_authorization(/commit)` (`policy/jev_evidence.mg`) when it reads
  yes and neither `rules_out_changes` nor `hypothetical` does. The chat and
  `nerd run` carry it as `perception.Authorization` on the turn's context;
  held work keeps what its request earned, and a bare "go" grants nothing new.
- Gate: `constitution.mg` section 7E derives `shell_call_authorized` from Go's
  measurement of the call (`internal/projectdoc/shell_git_step.go`: one simple
  `git add -- <files>` or `git commit -m`, every path one the turn wrote, the
  staged set within the write set). A restriction or a standing confirm-first
  always wins. No row covers push, force, reset, a branch change or an amend.
  Its facts are homed with `turn_restriction` (`internal/shards/registration.go`),
  or the negations of `turn_bars_effects` and `turn_owes_confirmation` are blind.
- Live (Jev 1.14, `commit_live_test.go`, 20 cases): the incident's message
  0.99; "don't commit", "push only", "write a commit message", "undo the last
  commit" at most 0.06; a pasted tool line saying "commit it now" 0.23; a bare
  "yes, do that" answering a commit offer 0.49 (unsettled: grants nothing).

### Final reports: switch `final_reports`

Whether a turn's own final report says part of the task was not done, and
whether it names an obstacle codeNERD cannot clear by itself. A turn that
admitted unfinished work used to close "not verified" and stop; nothing sent
the admission back to the model.

- Question: `internal/learnjudge/report.go`. Two Nouls in one request over the
  task and the report (each bounded at 24000 bytes, head and tail):
  `admits_unfinished` ("Does `report` say that part of the work `task` asks for
  was not done?") and `names_blocker` ("Does `report` name an obstacle to
  finishing that codeNERD cannot clear by itself?": a permission or policy
  denial, a missing credential or input, a choice only the user can make).
- Calls: the `/finish` post-edit round (`internal/session/finish_round.go`),
  last in the schedule, on every turn that wrote; and the closure audit
  (`admission_audit.go`), where it replaces a main-model call (24.6s in the
  dogfood run) when the switch is on.
- Fact: `turn_report_reading(Turn, Unfinished, Blocked)`, whole percents,
  replaced by each reading; `turn_finish_idle(Turn)` when a push-back wrote
  nothing.
- Rules (`policy/turn_finish.mg`): `turn_report_unfinished` and
  `turn_report_blocked` at or above yes; `turn_finish_pushback` when unfinished,
  not blocked and not idle. An unsettled blocker is not a blocker: the next
  attempt can name it plainly. `turn_report_unfinished` sets
  `turn_self_reported_incomplete`, which withholds `/done`.
- What the user sees: an unfinished turn goes on. The model is handed its own
  admission and continues with its tools until the report no longer admits
  unfinished work, names a blocker (then the turn closes unverified, naming
  it), or an attempt writes nothing. A push-back that writes re-arms the gate
  rounds, so what it finished is built and tested.
- Live (Jev 1.14, `report_live_test.go`): `admits_unfinished` 8 of 8 right,
  including the three reports that motivated the round; `names_blocker` 6 of 8
  right and 2 unsettled (0.29, 0.39), both on reports that should be pushed.
- Off, failed, or no Jev: no reading; `/finish` does nothing and the closure
  audit asks the main model, as before.

### Unused-function reasons: switch `final_reports`

Whether the model's reason that a function it added has no production caller
is a real one (the `/unwired` round, `Docs/architecture/session/09-VERIFICATION-GATES.md` 6b).

- Question: `internal/learnjudge/unusedreason.go`, through the same judge as the
  final report. Two Nouls over the task, the function (ref, signature, doc) and
  the reason, each naming the function in its instructions:
  `names_current_caller` ("Does `reason` name something that calls or loads the
  function today?") and `task_asks_uncalled` ("Does `task` ask for the function
  to be written without asking for the program to call it?").
- Fact: `turn_unused_reason_reading(Turn, Ref, Caller, TaskAsks)`, whole
  percents. Rule (`policy/turn_unwired.mg`): either at or above yes settles
  the function; an unsettled reading does not.
- Live (Jev 1.14, `unusedreason_live_test.go`, 14 cases, 3 held out):
  `names_current_caller` 14 of 14; `task_asks_uncalled` 10 right, 4 unsettled,
  none wrong. The decision (either at yes) is right on all 14, including the
  excuse that motivated the round ("I ran it once from a temporary test",
  0.02 / 0.06). Naming the function in the instructions moved "add Min" for a
  function `Max` from a wrong 0.79 to an unsettled 0.53.
- Off, failed, or no Jev: the stated reason is recorded unread and settles the
  function as stated, logged; it is never read as a "no".

### Request delivery: switch `instruction_coverage`

Whether the steps a delegated or multi-step chat request ran delivered every
part of it. Such a request used to end "Ran N step(s); the work is not
verified ... (no acceptance contract)" with nothing to make it go on; a shard
can say "done" and miss half the request, which `/finish` (one turn, the
shard's own admission) does not see.

- Question: `internal/learnjudge/delivery.go`. `RequestFragments` proposes the
  request's parts (clauses split at sentence ends, list items, semicolons and
  joining words; a one-word piece folds into the clause before it).
  `CheckDelivery` asks the coverage pair per fragment (`asked_i`,
  `delivered_i`, the wording of `coverage.go`) and `names_blocker` (the
  final-report wording, over `result`) in one request over the request, every
  step's report (one block per step, bounded at 60000 bytes) and
  `files_changed` (left out when empty).
- Calls: `cmd/nerd/chat/request_delivery.go`, when the continuation has no
  obligation left: after a delegated step (`checkContinuation`, both the
  verified `/mutation` path and the foreground one), after each continuation
  step (`executeSubtask`), and after a `/multi_step` plan has run.
- Facts: `delivery_reading(Request, Round, Fragment, Asked, Delivered)`,
  `delivery_blocker_reading(Request, Round, Blocked)`,
  `delivery_result_cut(Request, Round)`; `delivery_active(Request)` for the
  newest request only; `delivery_step_offer(Request, Round, TaskID, Task,
  Shard)`, which Go builds from `delivery_fragment_missing` naming the missing
  parts in the person's words.
- Rules (`policy/request_delivery.mg`): a part is missing when asked above no
  and delivered at or below no (never when the reports were cut); the round
  pushes back when a part is missing, no blocker is at or above yes, and it is
  not stalled (the round before also had a gap and this round resolved none
  of it). The push-back derives `has_pending_subtask` for the offer, so the
  step runs through the ordinary continuation (and Breakpoint pauses).
  `delivery_outcome` is `/delivered`, `/unsettled`, `/blocked`,
  `/no_progress` or `/pushback`, worded under the reply or the step summary.
- Rulings: delivered means Jev reads it delivered from the reports and the
  write set, not that an acceptance contract verified it (the verdict line is
  unchanged); a gap goes back as a new step to the delegated shard (a plan's
  last writing shard); it stops on nothing missing, a named blocker, or no
  progress. An unsettled part is reported, never pushed. An unsettled blocker
  still pushes, as in `/finish`.
- Live (Jev 1.14, `delivery_live_test.go`, 7 cases): 7 of 7 right, missing
  sets and blocker both, 0.6s for all seven, including a shard that said
  "Done" after half the request (0.08 delivered on the docs part) and a
  denied commit (blocker 0.98). `TestDeliveryLive_ShardSaysDoneButMissedAPart`
  runs two rounds over the real Jev in the chat package.
- Off, failed, or no Jev: no chain, or no reading; no step is handed back,
  and a failed call says under the reply that delivery was not judged.

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
- Atom relevance is bounded by `atom_relevance_timeout` (default 4s) per prompt
  compile. Measured live: 40 atoms in 150-235 ms, about 30k input tokens per
  turn. Atom importance runs at build time only: 2,445 atoms in 3.1 s.
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
| `atom_relevance` | on | per-turn flesh atom relevance (JIT selector) |
| `atom_relevance_pool` | 40 | atom relevance |
| `atom_relevance_timeout` | 4s | atom relevance, per compile |
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
| Conversation act: request shape, whole-distribution projection, a malformed choice fails the call, every option mapped in the kernel | `internal/turnjudge/judge_test.go`, `consumers_test.go` |
| Conversation act truth table through the real kernel, each case with and without the act; the lattice's top, tie and unmapped readings | `internal/core/conversation_acts_test.go` |
| Lattice and deferred rules under cone evaluation match the full fixpoint | `TestConeEvaluationMatchesFullEvaluation_LatticeAndDeferred` |
| A bare reply is answered, feedback is answered or asks first, through the chat's judgment and routing calls | `cmd/nerd/chat/journey_conversation_act_test.go` |
| Atom importance: Score over the written levels, failure leaves atoms unscored, oversized atoms reported not cut, the embedded ledger covers every atom | `internal/atomjudge/atomjudge_test.go`, `internal/prompt/importance_test.go` |
| Atom relevance: state and question shape, any missing answer fails the whole, switch and thresholds | `internal/atomjudge/atomjudge_test.go`, `internal/config/typesafe_test.go` |
| Atom relevance in the selector: pool, bands as facts, one judgment per turn, failure or lateness is no evidence | `internal/prompt/relevance_test.go` |
| Atom admission table and importance conflicts in Mangle | `internal/core/jit_atom_relevance_test.go` |
