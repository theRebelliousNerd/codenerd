# The conversation-first main agent

Status: built 2026-10-28 on `feat/conversation-brief`. This doc describes how
the chat's main agent sits between the user and the kernel: it talks first,
builds up what the work is, and starts work only when the kernel admits it.
Every decision named here is a Mangle rule; Go only moves facts in and
results out.

## The brief (`internal/perception/brief.go`, `policy/brief.mg`)

Each turn, perception receives the running brief and returns it whole and
updated. The brief has a goal, constraints, decisions, open questions, and
`ready`, which is the agent's own judgment that the brief is enough to start.

The chat keeps the brief as facts (`brief_goal`, `brief_constraint`,
`brief_decision`, `brief_open_question`, `brief_marked_ready`):

- `injectable_context` puts the brief in front of every shard and the main
  agent, so big work runs from the brief.
- `brief_ready()` never holds beside an open question.
- `/brief` shows the brief, and `/go` with nothing held starts its goal.

## Work modes (`internal/config/work_mode.go`, `policy/routing_arbitration.mg`)

One switch, `work_mode` in config, is cycled by Shift+Tab, set by `/yolo`, and
held for a session by `--yolo` (debug-self never changes it):

| Mode | Work starts | A multi-step run pauses |
|---|---|---|
| yolo | when the brief has no open question and is ready (the agent's judgment); human-approval prompts are approved as `codenerd (yolo)` | never |
| propose | on your go; the work is shown as a proposal | before each mutation |
| explicit | on your go; the work is held quietly | after every step |

Read-only work is never held. The held turn says what is held and how to start
it. "go", or `/go`, runs it with the intent it was held under, through the
kernel's `approve_lane`.

Hard safety holds in every mode. Yolo approves only approvable prompts. The
Dreamer's critical-path warning is lifted only by an approval of that exact
request (`dream_reason_liftable`), and deleting `go.mod`/`go.sum` is never
lifted.

## Background read-only work (`cmd/nerd/chat/background.go`)

`route_background()` holds for a delegation whose intent only reads. Such a
delegation runs off the turn:

- The chat answers at once.
- The footer shows `bg: reviewer`.
- The kernel tells the model what is already being worked on.
- The result arrives as its own message, with shard facts.

"Stop" cancels running background jobs (Jev `wants_to_stop`).

## Talking while work runs (`cmd/nerd/chat/side_turn.go`, `policy/side_turns.mg`)

A message typed while a turn runs work is a side turn. A full turn would
rewrite the per-turn facts the running coder's gates read, so the side turn
asserts none of them.

- Perception reads the message with the brief, and its reply is shown.
- The brief it returns replaces the kernel's, so a mid-work clarification
  steers the coder on its next compile.
- The kernel decides what to do with it:
  - `/stop` cancels the turn;
  - `/later` queues the message as a full turn for when the work ends;
  - `/converse` is just the reply.

A message sent before work starts is queued whole. Ctrl+X cancels the running
turn. Before this branch it only hid the spinner.

## "Debug yourself" (`cmd/nerd/chat/self_diagnosis.go`, `policy/self_diagnosis.mg`)

This is a chain the kernel lays out:

1. The problems log becomes `self_problem` facts (`internal/problemlog`,
   `policy/self_problems.mg`).
2. A reviewer root-causes every hot problem in codeNERD's own source, in the
   background, with whole messages. Jev triages codeNERD's own defects ahead
   of failures around it.
3. Its findings come back as a turn whose fix the work mode runs, proposes or
   holds.

`self_diagnosis_lane()` holds for the `/self_diagnose` verb. It also holds for
read-only work Jev judges to be about codeNERD itself.

## Jev

See `jev-turn-judgments.md`. Typed judgments from TypeSafe's System One model
add evidence to every turn, to retrieval, to reviewer findings and to
self-diagnosis triage. They never replace perception or the LLM.

## Tests

- `cmd/nerd/chat/journey_conversation_test.go`: propose holds until go, yolo
  waits on an open question, a review reports back from the background, and
  debug-yourself ends in a proposed fix.
- `cmd/nerd/chat/journey_side_turn_test.go`: conversation while work runs
  steers the brief, stop cancels, new work queues then runs, a message during
  perception queues, and Ctrl+X cancels.
- `cmd/nerd/chat/journey_jev_test.go`: the Jev evidence journeys.
- `cmd/nerd/chat/work_admission_test.go`, `background_test.go`: kernel lanes.
