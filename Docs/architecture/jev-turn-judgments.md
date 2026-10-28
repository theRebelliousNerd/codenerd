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
