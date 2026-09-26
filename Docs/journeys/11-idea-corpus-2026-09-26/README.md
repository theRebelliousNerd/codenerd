# 11: Idea corpus: token economy and code quality (2026-09-26)

**Status:** the ideas that need no Jev and fit one change each were built on 2026-09-26. The
[Implementation status](#implementation-status-2026-09-26) section below says which, what was
decided against and why, and what remains. The rest of this folder is still a backlog.

**The ask:**
- the biggest bang for token efficiency;
- the model at its best, writing the best code it can.

**Where it was researched:** two independent lanes read the code at `d61dbc6` (read-only) and
current external work:

| File | Lane | Contents |
|---|---|---|
| [tokens.md](tokens.md) | Token economy: the most correct work per token | 21 ideas (T-01…T-21), 6 wild cards, a sized map of where tokens go |
| [quality.md](quality.md) | Code quality: first-time-right, well-tested code that fits the codebase | 22 ideas (Q-01…Q-22), 5 wild cards, a map of where quality is lost, with run evidence |

Every idea cites the `path:line` it plugs into. Estimates show their arithmetic, and anything
unmeasured says so.

## Claims checked by hand before committing

These headline claims were re-read against the code, not taken on the lanes' word:

- **No prompt caching in the Anthropic tool loop.**
  - `cache_control` is set only in `CompleteWithSystem` (`internal/perception/client_anthropic.go:166`).
  - `CompleteWithTools` and `CompleteWithToolResults` go through `postMessages` (`:442`) with no
    cache marker.
  - The only `EnableSystemCaching` call is the classification client.
- **Cache reads are left out of input tokens on the CLI engine.** This contradicts the broker's
  contract: `claude_cli_client.go:468-476` vs `internal/broker/broker.go:186-189`.
- **The critic is invisible in the ledger.**
  - Its call is never tagged with `PurposeCritic`.
  - `internal/broker/purpose_wiring_test.go:31-32` still exempts critic and articulation as
    "no LLM calls".
- **The critic reads only the head of each file.** It gets the first `criticMaxFileBytes = 24000`
  bytes (`internal/session/critic.go:317`). It is never given the request:
  `buildCriticPrompt(files, removals, grounding)`.
- **The pin gate's surviving mutants are discarded when pinning passes.**
  - They are saved to `result.PinAdvisory` (`internal/session/pin_gate.go:587`).
  - They reach the model only in the repair prompt for *failed* pinning (`:773`, `:782`).
  - The critic runs at round 3 and pinning at round 5 (`turn_rounds.mg`).
  - In R1-13, two of the 12 surviving conditions sat on the two lines where the review later
    found regressions (dogfood component ledger, lines 5074-5078).
- **Campaign requirement coverage is a keyword match that nothing reads.**
  - Requirements are linked to tasks when two words longer than 3 characters match
    (`internal/campaign/decomposer.go:906-925`).
  - `requirement_coverage` is only declared (`schemas_campaign.mg:296`). No rule consumes it.
- **Judges invent confidence.**
  - The prompt-evolution judge defaults to 0.85/0.80 when the model omits one
    (`internal/autopoiesis/prompt_evolution/judge.go:305-313`).
  - The north-star guardian defaults its score to 0.7 (`internal/northstar/guardian.go:550`).
- **A run closed `/done` while admitting it was unfinished.** R5-3 said "the remaining 177
  deletions" were unreachable, then closed `/done` (`Docs/journeys/05-elite-harness-ladder.md:526`,
  N41).

## What the two lanes found, in one paragraph

The biggest wins do not need Jev. On the token side, the dominant cost is the tool loop, which
resends a ~45-55k-token prefix every round. Whether those tokens bill at 1.0x or 0.1x depends on
prefix caching, and the native Anthropic loop does not cache at all (T-01). The tool catalog also
changes mid-loop, which forces a full cache miss on the largest request (T-02). On the quality
side, the system already produces the signals that locate hidden defects, then throws them away:
- the surviving mutants (Q-10);
- the model's own admissions of unfinished work (Q-14);
- the reverted recurse diffs (Q-17).

The critic is where both lanes meet. It is expensive, it reads the wrong bytes, and it does not
know the request. A diff-anchored, request-aware critic saves tokens *and* finds more (T-03 /
Q-11).

## The verdict on Jev

Jev is useful, but as an enabler rather than the headline.
- **Why it's cheap:** calibrated yes/no, choice and score answers cost about $0.042 per million
  input tokens, arrive in 70-500 ms, and never produce malformed output.
- **What that makes affordable:** checking every requirement (Q-01), every survivor (Q-10),
  every final answer (Q-14) and every judge (Q-20). With LLM calls, these were too slow or costly
  to run on every turn.

Both lanes independently designed the same safe substrate:
- **A demand/answer oracle, not a live Mangle external** (T-09 / Q-21). Rules derive the
  questions owed. A Go driver outside evaluation asks them in one batch and asserts the answers
  as facts. Those facts are blocked for models and kept off the grant path.
- **Shadow graduation** (T-10). A Jev question goes live only when a Mangle rule sees enough
  agreement with today's call or with real outcomes.

**Key point:** every Jev idea has an LLM fallback. The oracle can therefore be built now
against a cheap LLM adapter, with Jev swapped in if and when access, accuracy and data terms
check out.

## Recommended order

"Jev?" = whether Jev is needed; "later" means an LLM fallback works until then.

**Phase 0: measure first (nothing can be proven without these)**

| # | Idea | Effort | Jev? | Why first |
|---|---|---|---|---|
| 1 | T-05 Ledger truth: tag critic, articulation, repair and planning; per-segment receipts; cache fields; drop the stale exemptions | S | n | Every saving below has to show up here |
| 2 | Q-22 Defect-replay bench: replay the post-edit machinery on saved diffs with known defects, plus clean landings | S-M | n | Scores every quality idea without a coding model |

**Phase 1: small, no Jev, biggest wins**

| # | Idea | Effort | Jev? | Expected effect |
|---|---|---|---|---|
| 3 | T-01 Anthropic prompt caching in the tool loop, plus honest cache metering | S-M | n | Native Anthropic loop input cost ~3-4x lower (arithmetic in T-01) |
| 4 | T-02 Freeze the tool catalog for a loop's life (enforcement already exists at execution) | S | n | Removes 1-3 full-prefix misses per long turn |
| 5 | T-03 + Q-11 Diff-anchored, request-aware critic; trivial-change skip; drop findings on untouched lines | S-M | n | Tokens down; sees edits past byte 24,000; fewer off-target uplift rounds |
| 6 | Q-10 Survivor triage round (an LLM classifies until Jev is available) | S | later | Charges the defect class behind R1-13 |
| 7 | Q-14 Admission audit of the final response (one small LLM call until Jev is available) | S | later | No more `/done` over self-admitted unfinished work (N41) |
| 8 | Q-17 + T-15(c) Recurse failure memory: a reverted attempt tells the next one why | S | n | Second attempts differ from first ones |
| 9 | T-11 Taxonomy learning only on user corrections | S | n | ~1 background call per turn becomes calls on corrections only |
| 10 | T-07 Compact executed write payloads; Mangle guard against wasteful whole-file rewrites | M | n | Largest single-edit token sink |

**Phase 2: larger quality programs (no Jev strictly needed)**

| Idea | Effort | Note |
|---|---|---|
| Q-01 Requirement ledger: `/done` needs evidence for each requirement | M | Closes the external audit's open "scope" gap |
| Q-02 Reproduce-first contract for `/fix` | M | Uses the existing `internal/evidence` contract |
| Q-12 Counterexample critic: probes run on old vs new code | M | A failing probe is an executed regression witness, not an opinion |
| Q-18 Anti-Goodhart recurse metric: mutants killed, not tests counted | M | Stops empty tests from counting as improvement |
| Q-06 + T-13 + T-19 Change-impact context: callers by symbol, tests, pinned observations, localization | M | Fewer exploration rounds, fewer mis-scoped edits |
| T-08 + Q-09 Repair from a fresh brief; restart from a new hypothesis when a repair stalls | M | R1-12 burned 746.7k tokens with no edit |
| T-14 Test-output codec (`go test -json`, failures only) | S-M | |
| Q-07, Q-15, Q-16, Q-19 | M | Mined conventions, polyglot pinning, input-class and property obligations, observed-failure atoms |

**Phase 3: Jev, once access and its accuracy are confirmed**

| Order | What |
|---|---|
| First | T-09 / Q-21 oracle substrate (can be built earlier on the LLM adapter), then T-10 shadow graduation |
| Then | T-06 perception split: decide first, generate the reply only on the perception route. This also answers perception OPEN-QUESTIONS Q3 |
| Then | Q-20 calibrated judges; Q-04 / T-04 difficulty-routed compute and thinking; Q-05 plan coverage; T-19 localization prefetch; T-16 init doc triage and a batch lane |
| Wild cards | tokens.md W-1…W-6 and quality.md W-1…W-5 |

## Where the lanes overlap (merge before building)

| Token idea | Quality idea | Shared core |
|---|---|---|
| T-03 | Q-11 | One critic redesign: anchored on the diff, aware of the request, with a pre-screen |
| T-09, T-10 | Q-21, Q-22 | Oracle substrate, shadow graduation and the defect-replay bench are one measurement system |
| T-15 | Q-17, Q-18 | The recurse loop: failure memory, expected-value ordering, mutation-based metrics |
| T-04 | Q-04 | Difficulty-routed model slot and thinking budget |
| T-08 | Q-09 | The repair episode: fresh brief, model escalation, hypothesis restart |
| T-13, T-19 | Q-06 | The context the edit needs, chosen by relevance and symbol identity rather than age or substring |

## Verify before building

- **Provider API details.** The token lane's Anthropic specifics came from its reading of the
  claude-api skill reference, not from this repo:
  - beta names such as `mid-conversation-tool-changes-2026-07-01` and
    `mid-conversation-output-config-2026-07-01`;
  - server-side `clear_tool_uses_20250919`;
  - preserved-thinking rules on Opus 5.5 / Fable 5.1 for newer accounts;
  - cache TTL semantics.

  Confirm each against current Anthropic docs before T-01, T-02, T-04, T-07 or T-18.
- **Jev claims are TypeSafe's own.**
  - Accuracy (≈ Sonnet 5 on consensus labels) and calibration have not been reproduced.
  - It is waitlist-only.
  - Using it sends workspace code to a third party, and no data-retention policy was found.

  Take the endpoint from https://docs.typesafe.ai/api only. One community guide lists a
  different host.
- **Every Jev-backed decision may withhold or oblige work. None may grant.** Both lanes
  specified a guard that keeps oracle facts out of `permitted/3`, and it is not optional.

## Sources for Jev

- https://typesafe.ai/blog/introducing-system-one-models-and-jev
- https://docs.typesafe.ai/api
- https://dev.to/valyuai/how-to-use-jev-a-practical-guide-to-typesafes-system-one-model-g5e
- https://en.wikipedia.org/wiki/Jev_(AI_model)

Each lane's own sources are listed at the end of its file.

## Implementation status (2026-09-26)

### Built
None of these needs Jev. Each has a test that fails without it.

| Idea | What changed |
|---|---|
| T-01 | The Anthropic tool loop places two cache breakpoints: one on the system block, which caches tools and system together, and one on the newest turn. Metered input is now uncached + cache write + cache read, on the API (sync and streaming) and the Claude CLI. Cache writes are a new sub-count through the broker to `nerd meter`. |
| T-05 | The critic's call is tagged `PurposeCritic`, and chat's shard interpretation `PurposeArticulation`. Stale exemptions were removed. Receipts carry a `Phase` (repair, uplift, step_plan, forced_final, no_tool_retry, admission_audit, survivors), and `nerd meter` lists the named rounds. |
| T-03 / Q-11 | The critic reviews the change: each changed region is widened to its enclosing element, and files over the cap show only those regions. It is handed the request. Findings on untouched lines buy no uplift round. |
| Q-10 | A pin gate that passes with surviving condition mutants owes an advisory `/survivors` round (round 6). The round names them and asks for a test that takes the other side of each real decision. |
| Q-14 | One small model call reads a writing turn's final report. An admission of unfinished work asserts `turn_self_reported_incomplete`, which withholds `/done` and names why (N41). |
| Q-17 | A reverted recurse attempt's bounded patch and the reason it was reverted are kept in the journal and handed to the next attempt at the same work, across restarts. |
| T-11 | Taxonomy learning reads every consecutive pair of exchanges once, with about a quarter of the calls, instead of re-reading each exchange up to five times. |
| T-13 | The context ledger holds current observations of the focus file and of written files past the age cut. The hold lasts while those pinned results fit in half the ceiling. |
| Q-09 | Two edits that leave the same failure restart the repair episode once: its edits are undone, and the next attempt must name a different cause. It gives up after that, as before. |
| T-14 (reduced) | A failed test run drops the summary lines of packages that passed. The failing output is kept whole. |
| T-06 (the part that needs no Jev) | The classifier is no longer asked for `implicit_assumptions`, which nothing read. |

### Decided against, with the reason from the code
| Idea | Why not |
|---|---|
| T-02 (freeze the tool catalog) | Withholding tools is load-bearing. The commit regime withholds read tools because a repair round once spent 20 reads and made no edit (`build_repair_regime_test.go`). A tool that is offered and then refused brings that failure back. Removing tools from the forced final call is what makes it answer instead of calling tools. |
| T-07 (whole-file rewrite guard) | The coder atoms already make line-range edits mandatory for existing code. A refusal cannot recover output the model already generated. Compacting old write payloads edits assistant turns, which preserved thinking on Opus 5.5 and Fable 5.1 rejects. |
| T-03's skip for trivial changes | The critic also checks that comments match the code, so even a comment-only change is not safe to skip. |
| T-12 (interpret shard output only when owed) | A UX choice. For a coder shard the interpretation repeats prose that was already written for the user. For reports it adds a summary and next steps. Left to the owner. |
| T-15(a) (order findings by keep rate) | The kernel keeps only a finding's last two attempts. Keep-rate ordering needs history the loop has not produced yet. |
| T-20 (diff instead of "read again") | A stored observation is the tool's projected output, not the file's text, so there is no clean diff to show. The edit tools already report how lines moved. |

### Remaining (each its own change)
- Q-22, the defect-replay bench: the defective diffs were not preserved, so the fixtures need
  rebuilding.
- Q-01 requirement ledger, Q-02 reproduce-first, Q-12 counterexample critic.
- Q-18 mutants-killed recurse metric. The mutation machinery is Go-only and lives in the
  session pin gate.
- Q-06 impact context, Q-07 mined conventions, Q-15 polyglot pinning, Q-16 input classes,
  Q-19 observed-failure atoms.
- T-04, T-08, T-16, T-17, T-18, T-21.
- Everything that needs Jev.
