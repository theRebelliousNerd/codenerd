# Jev turn judgments

Status: built 2026-10-28. Code: `internal/typesafe` (client), `internal/turnjudge`
(questions and facts), `cmd/nerd/chat/jev_turn.go` (wiring),
`internal/core/defaults/policy/jev_evidence.mg` (rules), `cmd/tools/jev_probe`
(live scoring).

## What it is

Jev is TypeSafe's System One model. It answers typed questions about a state
with calibrated probabilities in roughly 100–500 ms. On every chat turn
codeNERD asks it a set of yes/no questions **in parallel with perception**, so
it adds no latency. The answers reach the kernel as `jev_judgment(Name, Percent)`
before routing.

It **enhances perception and never replaces it**:

- Perception still reads the turn.
- The LLM still writes every reply.
- The rules use the evidence only to ask first, recognise a go-ahead, close an
  answered brief question, start the self-diagnosis chain, or stop background
  work.
- Without the `typesafe` config section, or when a call fails, there is no
  evidence and the turn routes exactly as before. A missing judgment is never a "no".

## The questions (`internal/turnjudge/questions.go`)

| Judgment | Asked | Kernel use |
|---|---|---|
| `changes_files` | every turn | a mutation Jev reads as no change asks first |
| `wants_answer` | every turn | evidence, logged |
| `names_target` | every turn | a mutation with no target named asks first |
| `hypothetical` | every turn | a "what if" read as a mutation asks first |
| `about_codenerd` | every turn | read-only work about codeNERD itself becomes the self-diagnosis chain |
| `wants_to_stop` | every turn | stops running background jobs |
| `approves_held_work` | while work is held | the approve lane, together with... |
| `adds_to_held_work` | while work is held | ...no additions ("go, and also X" is a new request) |
| `answered_N` | one per open brief question | closes the question, so yolo may start |

The thresholds are `typesafe.judgment_yes_percent` (default 70) and
`judgment_no_percent` (default 30). Between the two, a judgment is unsettled and
acts on nothing.

## Config (`.nerd/config.json`)

```json
"typesafe": {
  "model": "jev-latest",
  "base_url": "https://api.typesafe.ai",
  "api_key": "<key>",
  "request_timeout": "30s"
}
```

Nothing is defaulted except the timeout and the thresholds. An absent section
means Jev is off.

## Live probe

`go run ./cmd/tools/jev_probe -config .nerd/config.json -runs 3` scores every
judgment against the labeled cases in `internal/turnjudge/testdata/cases.json`.

First tuning pass, 2026-10-28, `jev-1.14.0`, 46 cases × 3 runs:

- 294 of 303 right, **0 wrong**, 9 unsettled (all in the middle band, where the
  rules act on nothing).
- 114k input tokens, 8.2 s.

The first pass showed that `changes_files` reads yes for a bare "go" over held
work that changes files, which is fair. So approval asks `adds_to_held_work`
instead of reusing `changes_files`.

Unsettled cases worth watching:

- "go, and also add a test" on `approves_held_work`: it lands in the middle,
  which is the safe outcome, since that is a new request.
- "review my changes in internal/approval" on `names_target` (≈0.65): read-only,
  so no rule reads it.

## Tests

- `internal/turnjudge/judge_test.go`: request shape, the fact projection, and
  failure as error.
- `cmd/nerd/chat/journey_jev_test.go`: a misread go-ahead still approves, a
  disputed mutation asks first, an answered question unblocks yolo, stop
  cancels background work, and a failed call changes nothing.

## Retrieval relevance (`internal/turnjudge/relevance.go`, `internal/retrieval/relevance.go`)

When Jev is on, Tier 2 does not stop at the keyword ranking. Jev judges a pool
six times the size of the tier: "Would reading this file help answer or carry
out `request`?", one question per file, with the file's head. The pool goes out
as parallel requests of 20 files each.

- Tier 2 keeps the files in Jev's order, with the keyword score breaking ties.
- A file at or below `judgment_no_percent` is left out.
- A file you mentioned is never judged.
- The verdicts reach the kernel as `context_relevance(File, Percent, Kept)`.
- A failed judgment leaves the keyword ranking as it was.

Live, on codeNERD's own repository, 2026-10-28:

| Request | Keyword Tier 2 | Jev-judged Tier 2 |
|---|---|---|
| "why does the approval broker open a second request for a duplicate call" | a skill ledger, audit docs, `sparse.go`… | `approval/broker.go`, `session/human_approval.go`, `mcp/remote_approval_test.go`, `approval/request.go`… (120 judged, 98 dropped, 756 ms) |
| "the footer does not wrap in the TUI" | 20 files, mostly agent and QA docs | `cmd/nerd/chat/view.go` |

The probe also exposed a keyword bug. `ExtractKeywords` only took code syntax
(capitalized symbols, `name(` calls, quoted identifiers), so a plain-English
request searched for nothing. It now also takes prose identifiers
(`renderFooter`, `work_mode`) and domain terms (lowercase content words of four
letters or more), at the lowest weight.

## Review findings checked against their code (`internal/turnjudge/findings.go`)

When a reviewer shard returns, in the foreground or the background, each finding
that cites a file and a line goes to Jev with the 25 lines around the citation:
"Does `code` show the problem that `finding` describes?" The verdicts become
`review_finding_check(Review, File, Line, Percent)`. At or below
`judgment_no_percent`, the kernel derives `review_finding_refuted`, and
`review_suspect(Review, "jev_refuted_finding")` feeds the existing
`reviewer_needs_validation`.

The user sees a "Findings Jev could not see in the code they cite" list beside
the review, and the model is told to verify them before acting. Nothing is
hidden or dropped on Jev's word.

Test: `TestJevJourney_RefutedReviewFindingIsFlagged`, `TestCheckFindings_JudgesCitedCodeAndSkipsUncited`.
