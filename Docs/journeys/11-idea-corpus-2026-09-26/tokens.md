# Token economy: idea corpus for codeNERD

Lane: get the most correct work out of every token the LLMs consume, and never make the written code worse.
Repo: `/home/user/codenerd` @ `d61dbc6` (read-only). Date: 2026-09-26.
Every `path:line` citation below was read at that commit. Where a number is an estimate, the basis is given next to it. Where nothing could be measured, the text says so and proposes a measurement.

**What the reader should know first.** codeNERD has already done serious token work: a cache-aware context ledger (`internal/session/working_context.go:420-517`), observation codecs for search, file reads and subagent returns (`internal/observation/*`), a prefix-fingerprint epoch meter (`internal/broker/epoch.go:45-72`, `nerd meter epochs`), a promptCacheKey that is aware of Meta's route overflow (`internal/perception/client_meta_responses.go:484-506`), and a prompt assembler ordered for the prefix cache (`internal/prompt/assembler.go:50-144`). The ideas below target what is **still** leaking after that work. Several of the biggest leaks are one layer below the ledger, in the provider adapters and in how the tool catalog is rendered.

---

## Top 7 by bang-for-buck

| # | ID | Title | Est. token impact | Effort | Needs Jev? | Confidence |
|---|----|-------|-------------------|--------|-----------|------------|
| 1 | T-01 | Turn on Anthropic prompt caching in the main loop and meter cache tokens honestly | Native Anthropic tool loop: input cost drops about 3-4x. A 20-round turn goes from ~1.0M full-price input tokens to ~0.25M full-price equivalents (arithmetic in T-01). The CLI engine gains truthful metering. | S-M | n | high |
| 2 | T-02 | Freeze the tool catalog for the life of a working loop (steer in-band; enforcement already exists) | Removes 1-3 full-prefix cache misses per long turn, and one of them is always the largest request (the forced final). About 45-55k tokens re-billed at full or write price per miss, on every caching provider. | S | n | high |
| 3 | T-03 | Critic: review a packet anchored on the diff and scoped to elements, and skip trivially safe changes | Today the critic reads up to 6 × 24,000 B (~36k tokens) on the planner slot. With the codec, the anchored regions come to ~6-12k tokens. Changes past byte 24,000, which the critic cannot currently see, become visible (quality gain). | S-M | n (optional pre-screen) | high |
| 4 | T-05 | Make the ledger tell the truth: purposes for critic, articulation, repair and planning, per-segment receipts, cache fields | 0 tokens saved directly. It is the only way to prove every other row. Today the critic and chat interpretation are booked as "session" or "unattributed". | S | n | high |
| 5 | T-07 | Compact executed write payloads and guard whole-file rewrites | Rewriting a 107 KB file at round 10 of 25 costs ~27k output tokens plus ~400k resent input tokens. The guard keeps edits element-sized (~0.3-2k tokens). | M | n | med-high |
| 6 | T-11 | Taxonomy learning: stop re-reading the same 5-turn window every turn; learn only on correction | ~1 background LLM call per turn (up to ~20 KB transcript) drops to calls on actual corrections only. Removes a 5x overlap in the window. | S | optional | med-high |
| 7 | T-06 | Perception decides first and generates second: one Jev map, a world-model target Choice, and a surface reply only on the `perception_answer` route | Per-turn classification drops from ~8k in / ~300 out on a frontier or CLI model to ~8k in at $0.042/M, and latency from seconds to 70-500 ms. On the executor path, `surface_response` is generated and never read. | M | y | med |

The substrate for every Jev site is T-09 (neural predicate) plus T-10 (shadow graduation). Build those before T-06, T-12 or T-15 go live.

---

## Where the tokens go today

### A. Anatomy of one tool-loop round (the dominant multiplier)

Each round of `runToolLoopPass` (`internal/session/executor_tools.go:69-443`) resends tools, system prompt and ledger through `completeWithWorkingContext` (`internal/session/working_context.go:733-747`).

| Segment | Size | Basis | Stable within the loop? |
|---|---|---|---|
| JIT system prompt (coder, /fix) | **21,985 tokens** (2026-09-17). Earlier: 25,609 and 25,842 tokens, of which skeleton 11,786 and flesh 13,823-14,056 | `Docs/journeys/impl/S19-compilation-scope-measurement.md:195` (jit.log `compilation_complete`); `Docs/audits/coder_token_cost_proposal.md:7-12` | Yes, the same string every round (`executor.go:1146`) |
| nerd.md section | ~1.7k tokens | 6,685 B `nerd.md` at the repo root; appended at `executor_tools.go:1190-1201` | Yes |
| Tool catalog (coder) | **~5-6k tokens (estimate)** | The coder catalog has 43 tools (`internal/prompt/config_factory.go:265-345`). String literals of 39 of them total ~17.6k chars (read-only awk over `internal/tools/**`, a heuristic); add JSON schema framing. Measure the exact figure with `broker.Segments.Tools` (`internal/broker/measure.go:40-107`). | **No.** It is narrowed mid-loop (see T-02). |
| Context ledger (transcript) | ≤ 65,536 B (~16k tokens) carried whole, plus the newest results whole | `internal/config/working.go:59` (`LedgerCeilingBytes`), `internal/context/working_set.mg:237-252` | Append-only between compactions (by design) |
| **Measured round total** | **55.4k input tokens per round, 22.3k of them uncached (Meta, before the ledger rewrite). Mean input per call went from 37.5k to 44.7k when the file outline was duplicated.** | `working_context.go:431-434`; `executor_tools.go:1226-1229` | |
| Rounds per turn | No count bound. Policy spans: nudge 8, commit 16, finalize 16, stall 24 rounds | `internal/config/working.go:57-68` | |

**Implication.** A 20-30 round write turn sends about 1-1.5M input tokens. Whether those tokens are billed at 1.0x or 0.1x depends on prefix stability, so caching and catalog stability outweigh every prompt-trimming idea.

### B. Per-turn and per-event calls outside the loop

| Call | When | Size per call | Where | Metered as |
|---|---|---|---|---|
| Perception `Understand` | every interactive turn | embedded system prompt 7,239 B (~1.8k tokens; awk over `understanding_adapter.go:629-790`), plus user prompt capped at 50,000 chars of input, 5×2,000 history, 8×500 exemplars and 4 KB strategic context (`02-PROMPT-BUDGET.md` §2.7). Output is a full JSON envelope including `surface_response` | `internal/perception/transducer_llm.go:127` | perception |
| Critic review | every write turn that touches source | up to 6 files × 24,000 B, head-truncated, plus removals and gopls/LSP diagnostics. Runs on the **planner slot** for 2-3 min | `internal/session/build_verify.go:675-760`, `critic.go:317-400`, `policy/turn_rounds.mg:38-45` | **session** (untagged). See T-05. |
| Critic uplift round | when there are high/medium findings | full transcript + findings | `build_verify.go` (`formatUpliftPrompt`, `critic.go:405`) | session |
| Repair rounds | build or test failure | full transcript + failing output (the latest output whole) | `build_verify.go:646`, `repair_loop.go:18` | session |
| Taxonomy learning | **every turn** once ≥2 exchanges exist | last 5 traces × (2,000 + 2,000) chars + critic system prompt | `session/executor_learning.go:228-248` → `perception/consolidation.go:62-90` → `perception/learning.go:153-200` | autopoiesis |
| Chat shard-output interpretation | every delegated chat turn | a **full JIT compile** (`/analysis_translator`) + the raw shard `result` + a 3-7 step answer | `cmd/nerd/chat/process_dream_delegation.go:305-440`, called at `process.go:583` and `process_dream.go:752` | **unattributed** (chat tags no purpose except campaign, `cmd/nerd/chat/campaign.go:199`) |
| Step planning | write turns naming ≥ `step_plan_min_sites` sites | planner call | `session/work_steps.go:152-175` (already gated: 13 wasted calls measured 2026-09-21) | session |
| Init doc relevance filter | `nerd init` | ~196 calls for ~1,960 docs, "~54 minutes" | `internal/init/strategic_knowledge.go:407-431` | unattributed |

### C. Caching status per provider (as the code stands)

| Provider path | Prefix caching today | Evidence |
|---|---|---|
| Anthropic API, **main loop** | **None.** No `cache_control`, no `thinking` or `effort`, and `Usage` parses only `input_tokens` and `output_tokens` | `client_anthropic.go:442-512` (`postMessages` marshals `AnthropicRequest` as is), `client_types.go:285-293`, `:337-340` |
| Anthropic API, classification client | system block cached | `client_factory.go:337-354` (the only `EnableSystemCaching` call) |
| Claude CLI engine | Claude Code caches internally, but **cache reads are parsed and never metered** | `claude_cli_client.go:88-91`, `:468-476` |
| Meta (Responses) | `prompt_cache_key` = hash(model, **tool catalog**) with 24h retention | `client_meta_responses.go:479-506`, `:589` |
| OpenAI-compatible | automatic prefix caching at ≥1024 tokens. `prompt_cache_key` is not sent | `prompt_cache_key` appears only in `client_meta_responses.go` |
| Gemini | implicit caching (automatic on 2.5+), plus explicit `cachedContent` for files | `client_gemini.go:437-481` |

### D. Measurement blind spots (these block proving any saving)

1. **Cache contract violated on both Anthropic paths.** `broker.go:180-194` states that InputTokens INCLUDES cache reads and that Anthropic's client "must add them into InputTokens". Neither the API client (`client_anthropic.go:554`, `:620`) nor the CLI client (`claude_cli_client.go:468-476`) does this. On the CLI engine, where Claude Code caches heavily, the ledger therefore under-counts input and the calibrator learns the wrong ratio (`broker.go:233-257`).
2. **Stale purpose exemptions.** `internal/broker/purpose_wiring_test.go:29-33` exempts `PurposeCritic` ("no critic subsystem issues inference of its own yet") and `PurposeArticulation` ("makes no LLM calls today"). Both now issue inference: `build_verify.go:755` and `process_dream_delegation.go:408`.
3. **No segment split inside History.** `measure()` folds tool_use inputs, tool_results, thinking and text into one `History` number (`measure.go:52-94`), so T-07 cannot be sized from receipts today.
4. **No receipts on disk in this checkout** (`.nerd/meter/` is absent). Every number above comes from measurements recorded in code comments and docs, not from a fresh run. **Unmeasured:** rounds-per-turn distribution, share of turns that hit each regime flip, critic hit rate, share of repairs that are build vs test.

---

## Ideas

### T-01: Turn on Anthropic prompt caching in the main loop, and meter cache tokens honestly
- **Pitch:** The biggest provider-level leak. The native Anthropic tool loop resends ~45-55k identical prefix tokens every round at full price.
- **Mechanism:**
  1. In `postMessages`, send `system` as a block array with an explicit `cache_control` breakpoint on the last system block (this caches tools and system together, since tools render first), plus top-level automatic `cache_control` for the growing ledger tail. This is the "robust combination for agent loops" in Anthropic's caching guide.
  2. Parse `cache_read_input_tokens` and `cache_creation_input_tokens`. Add both into `InputTokens` per the broker contract, and put the read count on `UsageMetadata.CachedContentTokens` so `Receipt.Actual.CachedTokens` fills. Do the same in the CLI client.
  3. Make "cache or not" a **per-call-shape decision in Mangle**, not a client flag. The broker TODO already derives break-even at 1.28 calls (`Docs/architecture/broker/TODO.md`, `epoch.go` `BreakEvenCalls`). For example: `cache_mode(/tool_loop, /auto). cache_mode(/single_shot, /none).` The adapter reads the call shape from ctx the way it reads `Purpose`.
  4. Pick the TTL from policy: 1h when a long gate sits between rounds. The critic takes "2 to 3 minutes" on the planner slot (`build_verify.go:745-750`), and Anthropic's 5-minute TTL runs from the *start* of the last request, so the loop's entry can expire during the critic plus its uplift round.
- **Plugs in at:** `internal/perception/client_anthropic.go:442` (`postMessages`), `:574-637` (`CompleteWithToolResults`), `client_types.go:285-340` (request type and Usage struct), `claude_cli_client.go:468-476`, `internal/broker/broker.go:180-194`, `internal/broker/epoch.go` (economics already per provider).
- **Impact and basis:** Per round, ~22k system + ~5.5k tools + ≤16k ledger + the newest results ≈ 45-55k (section A). Uncached, a 20-round turn costs 20 × 50k = 1.0M input tokens. Cached, it costs round 1 at 1.25 × 50k = 62.5k, then 19 rounds × (~46k read at 0.1x + ~4k new at 1.25x) ≈ 19 × 9.6k ≈ 182k, for ~245k full-price-equivalent. That is **~4x cheaper input**. On Opus 5.5, reads cost 0.05x (per the claude-api skill), so the saving is larger. Anthropic also does not count cache reads toward input rate limits on most models, which raises throughput.
- **Measure it by:** `nerd meter epochs` (fingerprint = tools + system, `epoch.go:45`), plus the healthy-loop signature: `cache_read` grows each round while `cache_creation` stays about equal to the last round's delta. Receipts per purpose, before and after, on the same task set.
- **Risks / failure modes:**
  - The 20-block lookback can miss when a round appends >20 non-collapsed blocks. Mitigation: an intermediate breakpoint.
  - **Preserved thinking on Opus 5.5 / Fable 5.1.** The ledger's compaction rewrites old tool_results, which is a history edit. On accounts created on or after 2026-08-31 this can 400, or silently drop replayed thinking. Verify with the claude-api `preserved-thinking-migration` flow before enabling compaction on those models. A possible alternative there is Anthropic's server-side `clear_tool_uses_20250919`.
  - On 1-call epochs, caching is a flat +25% write premium. That is why the decision is per shape.
- **Effort:** S (adapter) + S (metering) + S (.mg decision).
- **Needs Jev?** n. **Confidence:** high.
- **Cross-over:** None negative. Lower TTFT also leaves more wall-clock budget for the verify gates under `toolExplorationCutoff`.

### T-02: Freeze the tool catalog for the life of a working loop
- **Pitch:** Every regime change rewrites the *first* bytes of the request, which is the most expensive place to change anything.
- **Mechanism:** Tools render at position 0 on Anthropic and OpenAI. On Meta, the route key is a hash of the catalog. Today the loop narrows the catalog on four events:
  1. Structural-first withholds raw search until `working_search_open` (`working_context.go:85-105`, applied at `:738-740`).
  2. The commit regime strips read tools (`:117-126`, `:735-737`).
  3. The forced final keeps only write tools (`executor_tools.go:759`, sent at `:773`), and the same applies to the deadline final.
  4. The Piggyback channel re-renders the narrowed catalog *into the system prompt* (`piggyback_channel.go:69-71`).

  All four are **already enforced at execution**. `executeToolCall` answers a withheld read under commit with the regime text (`executor_tools.go:2245-2250`) and a withheld search with `structuralFirstText` (`:2253-2256`), and it never runs them. So the fix is to offer the session's full catalog on every round, and to announce each transition in-band once (a text block after the tool_results; on Opus 5+, a mid-conversation `role:"system"` message) using the texts that already exist (`searchOpenedText`, `workingRegimeText`). Two related fixes:
  - **(c) Retry hygiene:** `retryWithNoToolNudge` recompiles the whole system prompt to inject one atom (`executor_tools.go:2185-2217`). Append the nudge as a user turn instead.
  - **(d) Cross-turn stability:** Sort the catalog with the verb-invariant core first (`coreTools` then `codeDomTools` in `config_factory.go:265-318`), so automatic-prefix providers (OpenAI, Meta, Gemini, DeepSeek) keep the core even when turns differ in verb tools.
- **Plugs in at:** `internal/session/working_context.go:733-747`, `executor_tools.go:755-775`, `:2185-2217`, `piggyback_channel.go:59-71`, `client_meta_responses.go:499-506`.
- **Impact and basis:**
  - Each flip makes the next request re-pay tools + system + ledger, ~45-55k tokens (section A), at full price (Meta: a new route) or at 1.25x write (Anthropic after T-01).
  - Flip points per long turn: search opens after `StructuralTrials` 4 queries or `StructuralMissLimit` 2 misses (`config/working.go:63-64`); commit at 16 rounds; the forced final **always** runs when the policy finalizes. That gives ~1-3 misses per long turn.
  - Against the ~245k post-T-01 cost of a 20-round turn, two extra misses add ≈ 2 × 50k × (1.25-0.1) ≈ 115k, **~30-45% of the remaining input bill**.
  - Meta's own measurement: "7% of later tool-loop rounds lost their whole stable prefix for no reason visible in the request" (`client_meta_responses.go:491-493`) before the route-key fix. Catalog churn is the remaining visible reason.
- **Measure it by:** `nerd meter epochs` → calls per epoch for the `tool_loop` shape. The epoch count per turn should fall to 1 plus the number of compactions. Also count calls to withheld tools (answered by steering text), which is the cost side.
- **Risks / failure modes:**
  - The model sees tools it may not use yet and may try them. Each try costs one round that returns steering text. That is cheaper than a full miss once caching works, and cheaper still when the round is batched with other calls.
  - Anthropic's `tool_removal` blocks (beta `mid-conversation-tool-changes-2026-07-01`) are an alternative that keeps cache on Opus 5+.
- **Effort:** S. **Needs Jev?** n. **Confidence:** high.
- **Cross-over:** A stable catalog also stops the model from seeing a tool vanish and hallucinating that it failed. This is neutral to positive for quality.

### T-03: Critic that reviews an anchored, element-scoped packet and skips trivially safe changes
- **Pitch:** The critic pays up to ~36k planner-slot tokens to read the *top* of each file. For any edit past byte 24,000 it never sees the change at all.
- **Mechanism:**
  1. **Anchor on the diff.** Replace `content[:criticMaxFileBytes]` (`critic.go:390-392`) with the file-read codec's projection around each changed span: the enclosing element whole, with an outline of the rest. `observation.ProjectRead` (`internal/observation/fileread.go:116`) and `PreWriteContents` (already used for removals at `build_verify.go:723-741`) give the exact spans. Keep `turnRemovals` as is.
  2. **Skip the critic when the change is trivial.** Add a Mangle guard to `turn_round_owed(Turn, /critic)` (`turn_rounds.mg:38-45`): `!turn_change_trivial(Turn)`. Derive `turn_change_trivial` from measured facts the executor asserts: comment- or doc-only hunks, a gofmt-only diff, deletion of `unreferenced_symbols` only, changed lines ≤ N in non-exported code, and so on. The thresholds live in `.mg` per `TestExecutiveLiteralBudget`.
  3. **Optional Jev pre-screen** (shadow first, via T-09 and T-10). Ask a Noul question, "Could this diff change runtime behavior in a way the build and tests would not catch?", with the diff plus diagnostics as state. Run the frontier critic only when p ≥ threshold. It is advisory already (`build_verify.go:656-672`), so a false skip costs one missed opinion, never a wrong verdict.
- **Plugs in at:** `internal/session/critic.go:73-160` (prompt), `:313-400` (file loading), `build_verify.go:675-760`, `internal/core/defaults/policy/turn_rounds.mg:38-45`, `internal/observation/fileread.go:103-200`.
- **Impact and basis:** Worst case today is 6 × 24,000 B = 144 KB ≈ 36k tokens per review (`critic.go:317`, `:322`). Codec measurement on this repo: `executor.go` whole is 106,933 B, 21,281 B through the codec, and 5,695 B for a 20-line region (`Docs/architecture/broker/TODO.md`, file-read codec entry). Typical anchored packets should be ~6-12k tokens, a ~3-6x reduction on the priciest per-token slot. The skip rule removes whole 2-3 minute planner calls on trivial turns (fraction unmeasured). There is a second saving: a critic reading unrelated head code can raise findings on code the turn did not write, and each of those triggers a paid uplift round (`findingsWorthUplift`, `critic.go:284`).
- **Measure it by:** First tag `PurposeCritic` (T-05). Then track critic receipts per write turn, uplift rounds per turn, and the share of uplift findings whose `file:line` falls inside the turn's changed spans (the precision proxy). Offline, from the turn journals, compute the share of past reviews whose change lay beyond byte 24,000.
- **Risks / failure modes:** Element scoping can hide cross-element contract breaks. Mitigation: include the callers of changed exported elements (`callers_of`) as signatures only. The skip rule may skip a one-line logic bug. Keep "exported or concurrent code changed" as non-trivial.
- **Effort:** S-M. **Needs Jev?** n (the pre-screen is optional). **Confidence:** high.
- **Cross-over:** **Quality up.** The reviewer finally sees what changed. Tell the sibling lane.

### T-04: Thinking and effort chosen per round from the working regime, not per agent name
- **Pitch:** Exploration rounds ("read the next file") are billed at the same reasoning depth as the edit and repair rounds that need it.
- **Mechanism:** Today effort is a static hint per agent. `subagent.go:246-247` sets `capabilityHintForAgentName`, `manager_spawn.go:429` sets it from config, and `client_openai_compat.go:359-377` maps the hint to Meta effort. The Anthropic client sends **no** `thinking` or `effort`, so Opus 5 runs adaptive at its default `high` and Opus 5.5 at `medium` (claude-api skill). Proposed rule in `working_set.mg`: `working_effort(/low) :- working_regime_now(/explore), !working_wrote_since(...)`, `working_effort(/high) :- working_regime_now(/commit)`, `working_effort(/high) :- working_regime_now(/repair)`, capped by a turn-level ceiling that a Jev Score sets at turn start ("how hard is this task?", 5 described levels). Deliver it:
  - on Anthropic Opus 5 / 5.5 / Fable 5.1, as a **per-message effort** system message (beta `mid-conversation-output-config-2026-07-01`), which keeps the messages cache (a top-level effort change invalidates it);
  - on Meta, as `reasoning.effort` per request (`newResponsesRequest`, `client_meta_responses.go:510-540`).
- **Plugs in at:** `internal/context/working_set.mg` (next to `working_regime`), `session/working_context.go:733`, `client_openai_compat.go:359`, `client_anthropic.go:574`.
- **Impact and basis:** Thinking tokens are output tokens (5x input price on Anthropic). Meta once billed 2,177 and 2,668 output tokens for empty answers (`client_openai_compat.go:735-738`), which shows reasoning dominates output on that surface. **Not measurable today**, because receipts carry `ThinkingTokens` (`broker.go:199`) but nothing splits them by round kind. Measure thinking tokens per round tagged with `working_regime` first.
- **Measure it by:** Receipt `ThinkingTokens` grouped by regime. Turn verdict (`/done` vs `/unverified`) and repair attempts per turn under A/B.
- **Risks / failure modes:** Low effort in exploration can mean worse localization and so more rounds. Judge by cost per completed task, never per request. Changing top-level effort mid-loop breaks the cache, so use only the per-message form where it exists and otherwise pin effort per turn.
- **Effort:** M. **Needs Jev?** optional (turn ceiling). **Confidence:** med.
- **Cross-over:** It can raise quality if repair and commit rounds get `xhigh` funded by cheap exploration.

### T-05: Make the ledger tell the truth
- **Pitch:** You cannot prove a saving on a purpose the ledger cannot see.
- **Mechanism:**
  1. Tag `PurposeCritic` at `build_verify.go:755`. Tag `PurposeArticulation` at `cmd/nerd/chat/process_dream_delegation.go:408`. Add sub-purposes, or a `Phase` label on the receipt, for `step_plan` (`work_steps.go`), `repair`, `uplift`, `forced_final` and `no_tool_retry`, all of which inherit "session" (`executor.go:980`). Delete the stale exemptions at `purpose_wiring_test.go:31-32`; the test then fails until the tags exist, which is the point.
  2. Extend `Segments` (`measure.go:40-107`) with `ToolUseInput`, `ToolResult`, `Thinking` and `Text` inside History.
  3. Add cache read and write fields for Anthropic (T-01).
  4. Add the `working_regime` and ledger epoch to each receipt so T-02 and T-04 can be sliced.
- **Plugs in at:** `internal/broker/types.go:104-118`, `measure.go`, `receipt.go`, `purpose_wiring_test.go:26-40`, `cmd/nerd/cmd_meter.go`.
- **Impact and basis:** 0 direct. It enables every other claim in this document.
- **Measure it by:** `nerd meter --json` shows non-zero critic and articulation rows, and the unattributed share drops.
- **Risks:** None material.
- **Effort:** S. **Needs Jev?** n. **Confidence:** high.
- **Cross-over:** none.

### T-06: Perception decides first and generates second
- **Pitch:** On most turns the classifier writes a friendly reply nobody reads, plus fields nothing consumes. The rest is a typed decision.
- **Mechanism (going further than "use Jev for Understand"):**
  1. **Split decision from prose.** Every categorical field of the envelope (`understanding_adapter.go:750-776`) is a Jev question: semantic_type, action_type, domain, scope.level, mode, primary_shard and urgency are Choice; is_question, is_hypothetical, is_multi_step, is_negated and requires_confirmation are Noul; confidence is Score. All go in one request with one `state` (input + capped history), so the ~12 answers arrive in a single 70-500 ms pass.
  2. **Target as a Choice over the world model, not free text.** Build ≤255 candidates from the structure index (`internal/world/structure_index.go`, which holds every declaration with span and ref), paths mentioned in the input, and the ambient active file. Jev picks among them. A regex for explicit paths short-circuits. Free-text `user_constraints` stays as the raw clause span, which needs no generation.
  3. **Generate `surface_response` only when the kernel routes `perception_answer`** (`cmd/nerd/chat/process.go:285-289`). The session executor path (`executor.go:1224` `observe`) never reads `intent.Response`: grep finds no consumer in `internal/session`. Only then call the LLM for a reply.
  4. **Today, without Jev:** remove `implicit_assumptions` (0 consumers; grep over `internal`, `cmd`) and drop `surface_response` from the schema when the caller is the executor path.
- **Plugs in at:** `internal/perception/transducer_llm.go:74-200` (`Understand`), `understanding_adapter.go:110-176` (contract check), `:629-790` (schema), `cmd/nerd/chat/process.go:280-290`. This answers `Docs/architecture/perception/OPEN-QUESTIONS.md` Q3: CLI engines currently classify every turn on the main engine.
- **Impact and basis:** Input ~1.8k (embedded system prompt, 7,239 B) + user prompt up to ~6k, and output ~250-400 tokens of JSON per interactive turn on a frontier or CLI model. With Jev: ~8k × $0.042/M ≈ $0.0003 per turn, output free. **Honest scale note:** perception is ~1 call per interactive turn against 20+ loop rounds of ~50k, so it is a *latency and CLI-engine* win more than a bulk-token win.
- **Measure it by:** Shadow (T-10) with agreement per field against the current LLM labels. Downstream: route decision agreement (`route_decision/2`) and turn verdicts.
- **Risks / failure modes:** Jev is literal and weak at negation ordering and counting. The kernel's vocabulary-miss rules (`understanding_vocab_miss`) remain as a guard. Adversarial state can move answers, but perception never grants anything: `permitted/3` is Mangle-only.
- **Effort:** M. **Needs Jev?** y (parts 1-3); part 4 is n. **Confidence:** med.
- **Cross-over:** A target that comes from the world model is a valid ref by construction, so fewer mis-scoped turns (quality up).

### T-07: Compact executed write payloads and guard whole-file rewrites
- **Pitch:** The ledger compacts tool *results*. The *inputs* of executed writes (whole files, element bodies) are resent on every later round of the turn.
- **Mechanism:**
  1. `prepareWorkingRequest` rewrites only `ToolResults` of evicted calls (`working_context.go:466-479`). Assistant messages carrying `tool_use.input.content` are never touched. At a compaction epoch, which already breaks the prefix, replace executed write inputs over K bytes with a handle: `{"path":..., "content":"[written N bytes at rev R; recall_context id=...]"}`. The observation was already saved at the effect boundary (`recordWorkingResult`, `:298-388`).
  2. **Whole-file rewrite guard in Mangle.** `write_file` overwrites existing files with no size or diff check (`internal/tools/core/file_ops.go:250-330`). Assert measured facts `write_existing(Call, OldBytes, NewBytes, ChangedLines)`. Derive `write_rewrite_wasteful(Call)` when the file exists, OldBytes > T, and the ChangedLines/total ratio < R. Answer with steering text that names `edit_element`, `apply_edits` or `edit_lines`, which already exist (`config_factory.go:299-318`).
- **Plugs in at:** `session/working_context.go:456-517`, `executor_tools.go:955` (`executeAndRecordToolCall`), `internal/tools/core/file_ops.go:279-330`, a new rule next to `modular_tool_allowed`.
- **Impact and basis:** Example: rewrite `executor.go` (106,933 B ≈ 27k tokens at ~4 chars/token) at round 10 of 25. That is ~27k output tokens (5x price on Anthropic) plus 15 × 27k ≈ 405k input tokens resent (0.1x if cached, 1x if not). An `edit_element` of one function is ~0.3-2k tokens. **Frequency unmeasured.** Count `write_file` on pre-existing paths in the audit log (`logging.Audit().FileOp`, `file_ops.go:325`).
- **Measure it by:** T-05's `ToolUseInput` segment share of History, and a histogram of write_file payload sizes.
- **Risks / failure modes:**
  - Editing an assistant turn can invalidate replayed thinking signatures (preserved thinking on Opus 5.5 / Fable 5.1). Apply only on surfaces where `BlockFidelity.ReplaysReasoning` is false (`internal/perception/provider_fidelity.go`) or no thinking block precedes; on Anthropic prefer server-side `clear_tool_uses_20250919` with `clear_tool_inputs: true`.
  - The guard may block legitimate large rewrites such as generated files. Exempt files the kernel marks generated or new.
- **Effort:** M. **Needs Jev?** n. **Confidence:** med-high.
- **Cross-over:** **Quality up.** Element edits trip the syntax guard and removal checks less often, and the critic's removal rule exists because whole rewrites silently drop lines (`critic.go:151-153`).

### T-08: Repair from a fresh brief, with model escalation at attempt boundaries
- **Pitch:** A compile error such as `undefined: x` does not need the 50k-token transcript that produced it.
- **Mechanism:** Repair rounds today append to the turn's history and resend everything (`build_verify.go:646`). For `/build` repairs, and for test repairs whose failure localizes to the turn's own elements, open a **new conversation** instead. Its brief holds the task, the model's last stated plan (the final assistant text), the failing output (whole for the latest), the changed elements through the file-read codec, and the diff. This is Agentless's localize → repair → validate. A fresh conversation also lifts the constraint that forbids switching models mid-loop (`executor.go:436-440`: "tool_use IDs cannot cross vendors"). So the kernel can derive `repair_attempt_slot(Episode, 1, /worker) :- repair_kind(Episode, /build)` and `repair_attempt_slot(Episode, 2, /planner)`: FrugalGPT-style escalation with a deterministic judge (the compiler or tests), not a model.
- **Plugs in at:** `session/build_verify.go:560-650` (repair round driver), `repair_loop.go:18-60` (`RepairCost` already records tokens per attempt), `executor.go:440` (`llmForVerb`), new `policy/repair_route.mg`.
- **Impact and basis:** A fresh brief is ~5-10k tokens (codec regions + errors) against ~45-55k per transcript round (section A), and the worker slot costs a fraction of the planner (README config: `worker: ollama qwen3:8b`, `planner: <frontier>`). The share of repairs that are build vs test is **unmeasured**. Read it from `RepairRecord.Kind` and `Cost` (`repair_loop.go:62-70`).
- **Measure it by:** repair pass rate per attempt number and kind, and `RepairCost.TokensIn` per episode, A/B against transcript continuation.
- **Risks / failure modes:** It loses the reasoning behind the change, so the model may "fix" the error by undoing the intent. Mitigation: include the plan text, and require the pinning and test gates to re-run (they do). Only escalate on failure.
- **Effort:** M. **Needs Jev?** n. **Confidence:** med.
- **Cross-over:** Can improve quality: a clean context avoids lost-in-the-middle on long transcripts (creed V).

### T-09: Jev as a neural predicate: ask-then-assert, cached, never on the grant path
- **Pitch:** Let `.mg` rules consume calibrated typed judgments the way they consume `code_calls`, without a model ever deciding.
- **Mechanism (concrete against this repo):**
  - **Why not a live external predicate.** The engine's `ExternalPredicateCallback` (`internal/core/external_predicates.go:29-85`, registered at `kernel_eval.go:305-320`) runs *inside* evaluate under the kernel lock. `ShouldQuery` "always returns true" because handlers are assumed cheap (`:39-45`), and a 70-500 ms HTTP call inside the fixpoint would stall every query.
  - **Instead, copy the `turn_next_round` driver pattern** (`turn_rounds.mg:47-56` plus `executor_tools.go:556-620`):
    1. **Questions are data in `.mg`:** `neural_question(Q, /noul, "Could this diff change behavior the tests miss?")`, `neural_option(Q, Opt, Descr)`, `neural_threshold(Q, 70)`.
    2. **Rules derive what is owed:** `neural_owed(Q, Subject) :- turn_round_pending(T, /critic, _), turn_diff_digest(T, Subject), !neural_answer(Q, Subject, _, _, _)`.
    3. **A Go driver** asks for all owed (Q, Subject) pairs, groups them by Subject (one `state`, many questions, one Jev pass), and asserts `neural_answer(Q, Subject, Answer, P100, Conf100)` as EDB. It caches by (pinned model `jev-1.13.0`, question digest, subject digest) in SQLite, so the same diff is never asked twice.
    4. **Rules consume** only `neural_trusted_answer`, which requires `neural_trusted(Q)` (T-10).
  - **Constitutional guard.** Add `neural_answer`, `neural_trusted_answer` and `neural_owed` to `hostWitnessPredicates` (`kernel_validation.go:74-95`) so no control packet or learned rule can write them. Add a boot-time check that none is reachable on the grant path via `mangle.GrantPathOfSource` (`internal/mangle/grant_path.go:92-196`). A neural answer can then *skip work* or *order work*, but can never make `permitted/3` derive.
  - **Failure policy.** No key, timeout or budget exhausted means no answer is asserted, and every consuming rule falls back to the current path. This matches the fail-closed style.
- **Plugs in at:** `internal/core/kernel_validation.go:74`, `internal/mangle/grant_path.go:92`, new `internal/neural/` driver, broker `PurposeNeural` for metering (T-05).
- **Impact and basis:** An enabler for T-03, T-06, T-11, T-12, T-13 and T-15. Per call: state tokens × $0.042/M, output free.
- **Measure it by:** answers asserted, cache hit rate of the neural cache, and consumer rules that fired.
- **Risks / failure modes:**
  - Degradation with irrelevant state. Keep `state` minimal per subject.
  - The 64k-token limit on state plus questions. The driver splits subjects.
  - `jev-latest` drift. Pin the model and fold it into the cache key.
- **Effort:** M. **Needs Jev?** y. **Confidence:** med.
- **Cross-over:** none directly. It is the safety substrate for the sibling lane's Jev ideas too.

### T-10: Shadow agreement ledger, with graduation derived in Mangle
- **Pitch:** No Jev site goes live on a vendor's self-reported accuracy. It earns trust per question from agreement with what the system already does.
- **Mechanism:** Wrap each decision-shaped call site so that, when shadow is enabled for its question, the current LLM path runs as today and the T-09 driver asks Jev in parallel. Both answers go to `.nerd/meter/shadow.jsonl` through `internal/jsonl` (the same appender as receipts, `broker/filesink.go`). A `nerd meter shadow` readout shows per-question agreement, confusion and cost per call. Graduation is a rule, not a flag: `neural_trusted(Q) :- neural_agreement(Q, N, Agree), N >= 200, Agree * 100 >= neural_min_agreement(Q) * N.` with thresholds in `.mg`. Demotion is symmetric: when a later window drops below the threshold, `neural_trusted` stops deriving. Where free labels exist, grade against outcomes rather than the LLM: turn verdicts (`/done` vs `/unverified`), the recurse ratchet (`keep` / `revert`), critic uplift acceptance, and atom co-use lift (`internal/prompt/couse.go`).
- **Plugs in at:** `internal/broker/wrap.go`, `filesink.go`, `cmd/nerd/cmd_meter.go`, new rules alongside T-09.
- **Impact and basis:** 0 direct. It gates everything Jev.
- **Measure it by:** its own output.
- **Risks:** Shadow doubles decision-call cost while it runs (Jev's share is tiny). Agreement with a wrong incumbent is not correctness, which is why outcome labels are preferred.
- **Effort:** M. **Needs Jev?** y. **Confidence:** high (as a measurement).
- **Cross-over:** It is also the A/B harness the sibling lane needs.

### T-11: Taxonomy learning: stop re-reading the same window every turn
- **Pitch:** One background frontier call per turn re-analyzes a sliding 5-trace window, and it mostly answers "nothing to learn".
- **Mechanism:** `queueTaxonomyLearning` enqueues on **every** turn once ≥2 exchanges exist (`executor_learning.go:210-248`). `LearnFromInteraction` analyzes the last 5 traces, each field clamped to 2,000 chars (`learning.go:175-189`), so each exchange is re-read in up to 5 consecutive windows (stride 1). The empty answer is the expected common case, which is why the call opts into `WithAllowEmptyCompletion` (`learning.go:192-196`). Gate it on a derived signal instead: `learning_due(Turn) :- turn_user_correction(Turn)`. Derive that signal from facts already present (the same target re-asked with a different verb, `Signals.IsNegated`, a hollow-success verdict followed by a rephrase) or from a Jev Noul, "Does the latest user message correct or redirect the agent's previous interpretation?" Analyze only the correcting exchange plus its predecessor.
- **Plugs in at:** `internal/session/executor_learning.go:228`, `internal/perception/consolidation.go:62`, `learning.go:153`, new rule in a learning `.mg`.
- **Impact and basis:** From ~1 call per turn (input up to ~20 KB transcript + `CriticSystemPrompt`) to calls on corrections only. The correction rate is unmeasured; the autopoiesis receipts show current spend (`learning.go:154` tags `PurposeAutopoiesis`).
- **Measure it by:** `nerd meter` autopoiesis row before and after, and the number of learned facts persisted (`PersistLearnedFact`) per 100 turns. That count should not drop.
- **Risks:** A missed correction goes unlearned. Shadow the gate (T-10) against the ungated critic's non-empty outputs.
- **Effort:** S. **Needs Jev?** optional. **Confidence:** med-high.
- **Cross-over:** A sharper signal teaches the classifier on actual corrections, not noise.

### T-12: Chat interpretation only when owed, fed the projection instead of the raw output
- **Pitch:** After a delegated shard answers, chat makes a second full-prompt call to rephrase it, then shows the raw output anyway.
- **Mechanism:** `formatInterpretedResult` always calls `interpretShardOutput`, which compiles a full `/analysis_translator` prompt and sends the whole raw `result` (`process_dream_delegation.go:305-340`, `:402-452`). The raw text is then shown in `<details>` anyway (`:452`). Make it a kernel decision in `routing_arbitration.mg`: `interpretation_owed(Turn)` holds when the result carries failures, spans more than one shard, or exceeds N chars. Otherwise show `result.Response` directly. When interpretation is owed, feed it the subagent codec's projection (`internal/observation/subagent.go`), measured at 3.0% of raw bytes over 67 real outputs (broker TODO), rather than the raw transcript. An optional Jev Noul: "Is OUTPUT already a direct answer to REQUEST?"
- **Plugs in at:** `cmd/nerd/chat/process_dream_delegation.go:402-452`, `process.go:583`, `process_dream.go:752`, `policy/routing_arbitration.mg`.
- **Impact and basis:** One JIT-compiled call per delegated chat turn (system prompt at `translatorJITBudget` scale plus the raw result plus a 3-7 step answer). Frequency is **unmeasured** because the call is unattributed (section D). Tag it first (T-05).
- **Measure it by:** Articulation-purpose receipts, and user follow-up rate on non-interpreted answers.
- **Risks:** Some shard outputs are unreadable to users. Hence the owed rule, not a blanket skip.
- **Effort:** S. **Needs Jev?** optional. **Confidence:** med.
- **Cross-over:** Fewer paraphrases also mean fewer places where the answer drifts from the evidence (creed IV).

### T-13: Pin relevant observations in the ledger; evict the irrelevant ones first
- **Pitch:** Eviction by age forces recall round-trips for exactly the files being edited, and each round-trip re-sends the whole request.
- **Mechanism:** `working_evict` takes results older than `LedgerKeepRounds` (2), plus stale and superseded ones (`working_set.mg:245-252`). Relevance is not a term. Add `working_pinned(ID)` for observations whose entity is the focus, a written element, or a direct caller or callee of a written element (join on the `code_calls` facts the structure index already maintains, `world/structure_index.go:25-40`). Evict unpinned first. Pinned ones leave only when stale or when pins alone exceed the ceiling. An optional Jev Noul per archived observation ("Will the next edit need this?") can break ties, in shadow first. This is the given "observation relevance masking" idea, carried into the **session** ledger rather than only the chat compressor (`internal/context` is chat-only for masking, `02-PROMPT-BUDGET.md` §4.7).
- **Plugs in at:** `internal/context/working_set.mg:217-268`, `session/working_context.go:598-631` (`updateLedger` entries), `recordWorkingResult` (`:298-388`, which already knows element entities).
- **Impact and basis:** Each avoided recall or re-read is one full request of ~45-55k input tokens (mostly cached after T-01 and T-02) plus latency. Recorded incidents: "24 rounds reading the file, recalling a 2000-character page of it and reading it again" (`working_context.go:442-447`), and "eleven recalls … then a read-only stall" (R1-9, `:315-318`).
- **Measure it by:** `recall_context` calls per turn, rounds to first write, and compaction epochs per turn.
- **Risks:** Pins could hold the ledger above its ceiling. Cap pinned bytes, with a derived fallback to age eviction.
- **Effort:** S-M. **Needs Jev?** n (Jev for ties only). **Confidence:** med.
- **Cross-over:** **Quality up.** The evidence for the edit stays in view (creed V).

### T-14: Codec for build and test output: failures only, workspace frames only, remainder under a handle
- **Pitch:** Test output "goes back whole" (`test_verify.go:268-271`), including every passing package's `ok` line across the importer fan-out.
- **Mechanism:** Run `go test -json` (currently plain `go test`, `test_verify.go:257-259`). Project the output: failed tests with their output, panics cut to the first frames inside the workspace, and a one-line tally of passing packages. Retain the whole log under a handle through `internal/retain`, redeemable by a read-only verb in the same style as `search_expand` and `subagent_expand` (broker TODO Phase 1). For non-Go runners or unknown formats, map-reduce with Jev: chunk the log and ask a Noul per chunk, "Does this chunk contain the cause of the failure?" Keep chunks with p > t verbatim and retain the rest. This is the "structural test-output codec still worth building" the broker TODO names.
- **Plugs in at:** `internal/session/test_verify.go:230-290`, `build_verify.go` (build output head clamp around `:160`), `repair_loop.go:18` (older attempts already capped at 2,000), `internal/observation/` (new codec), `testoutput.Parse` (already used at `observation/subagent.go:715`).
- **Impact and basis:** It scales with importer fan-out ("the packages the turn wrote *and every package that imports them*", README §7). The size is **unmeasured**. Measure repair-prompt bytes of test output before and after the codec.
- **Measure it by:** the T-05 ToolResult segment in repair rounds, and repair pass rate (must not drop).
- **Risks:** Hiding a passing package's warning is harmless. Hiding interleaved output of a failing test is not, so keep all output of failing tests.
- **Effort:** S-M. **Needs Jev?** n (Go); y (unknown formats). **Confidence:** med.
- **Cross-over:** Quality up: less noise around the real failure.

### T-15: Recurse loop: expected-value finding order, a tractability pre-screen, and failure memory
- **Pitch:** Every reverted attempt is a whole executor turn. Order and gate attempts by predicted success, and do not repeat the same failure.
- **Mechanism:**
  - **(a)** `recurse_next` takes the best kind rank, with ties broken by lowest ID (`recurse.mg:141-156`). Aggregate historical keep-rate per (gate, kind, signature class) from `recurse_attempt` facts with `fn:count` and order candidates by it. This is pure Mangle.
  - **(b)** A Jev Score "tractability" on finding message plus evidence, in shadow first (T-10), labelled for free by ratchet outcomes. `finding_deferred(ID)` below threshold waits for a pass in which its node changed.
  - **(c)** `finding_stalled` needs two same-signature failures (`recurse.mg:125-131`), and the first attempt's diff is discarded by `git.revert` (`recurse_cycle.go:760-765`). Retain the reverted diff plus signature and pass it into the next attempt of that finding as "tried X, failed with Y", so attempt 2 is a different attempt.
- **Plugs in at:** `internal/core/defaults/policy/recurse.mg:82-156`, `internal/campaign/recurse_cycle.go:580-790`, `recurse_policy.go`.
- **Impact and basis:** Tokens saved = attempts avoided × turn cost (tens of rounds × ~50k). Keep rate is readable from the journal (`recurse_journal.go`) and **was not measured here**.
- **Measure it by:** keep-rate per attempt, tokens per kept commit (receipts joined to the journal by cycle), and time to stall.
- **Risks:** Deferral can starve hard but valuable findings. Keep a floor that re-admits deferred findings each N passes.
- **Effort:** M. **Needs Jev?** optional (b). **Confidence:** med.
- **Cross-over:** A higher kept-per-token ratio means the improvement loop improves more per dollar.

### T-16: Batch lane for non-interactive purposes, with Jev for init doc triage
- **Pitch:** Work nobody waits on should not pay the interactive price.
- **Mechanism:** Add a Mangle decision `llm_lane(Purpose, /batch)` for purposes with no interactive consumer: init doc extraction (`strategic_documents.go:271`), taxonomy learning (T-11), prompt-evolution judging (`autopoiesis/prompt_evolution/judge.go:80`), and periodic north-star checks (`northstar/guardian.go:838`). The broker submits these through the provider batch APIs (Anthropic Message Batches, OpenAI Batch and Gemini batch are each ~50% off) and resolves them later. Separately, init's relevance filter (`strategic_knowledge.go:407-431`: ~196 LLM calls, "~54 minutes", yes/no per doc) is a pure Noul classification. Jev can label ~1,960 docs at ~2k tokens each for ≈ 3.9M × $0.042/M ≈ $0.16, in minutes.
- **Plugs in at:** `internal/broker/wrap.go` (lane), `init/strategic_knowledge.go:540-580` (`analyzeDocBatch` parse), new `policy/llm_lane.mg`.
- **Impact and basis:** ~50% on the batched purposes. Their share of the total is unmeasured (most are unattributed, T-05).
- **Measure it by:** per-purpose spend before and after, and time-to-result SLA.
- **Risks:** Batch latency of up to 24h. Only purposes whose results feed later sessions qualify. Provider availability varies.
- **Effort:** M. **Needs Jev?** y for doc triage, n for batching. **Confidence:** med.
- **Cross-over:** none.

### T-17: Piggyback envelope diet on tool rounds
- **Pitch:** On envelope engines (the CLI engines, Gemini with grounding), every tool round writes a user-facing `surface_response` that is never shown, plus a reasoning trace.
- **Mechanism:** `piggybackChannel.CompleteWithToolResults` notes that a round which calls tools "is not the turn's last word and will not be shown" (`piggyback_channel.go:75-85`). The schema still *requires* `surface_response` (`client_schema.go:28-32`), and the protocol atom asks for a `reasoning_trace` showing "complete analysis" (`internal/prompt/atoms/protocol/piggyback.yaml:33`, `:70`). Add a protocol rule: when `tool_requests` is non-empty, `surface_response` is `""` and `reasoning_trace` is ≤ N words. The final round keeps both. Select it through an atom gated on `working_regime`, not in Go.
- **Plugs in at:** `internal/prompt/atoms/protocol/piggyback.yaml`, `internal/perception/client_schema.go:28-32` (make `surface_response` optional), `session/piggyback_channel.go:59-90`.
- **Impact and basis:** Output tokens per tool round. Size is **unmeasured**. Receipts carry `OutputTokens` per call; split them by whether `tool_requests` was non-empty. For CLI subscription engines the saving is time and rate-limit headroom rather than dollars.
- **Measure it by:** output tokens per Piggyback round, and turn verdicts (the trace may carry quality).
- **Risks:** The reasoning trace may help the model plan. A/B it and do not assume.
- **Effort:** S. **Needs Jev?** n. **Confidence:** med.
- **Cross-over:** Possible quality impact (trace-as-scratchpad). Hand to the sibling lane for the eval.

### T-18: Two-tier system prompt: session-stable constitution in `system`, per-turn selection in `messages`
- **Pitch:** The persona tier and nerd.md are identical across turns but sit behind or around volatile content, so no cross-turn prefix survives.
- **Mechanism:** The assembler already tiers atoms as persona → turn → relevance → injected → retrieved (`assembler.go:128-144`). nerd.md is appended *after* all of them (`executor.go:1146`, `executor_tools.go:1190-1201`), which strands the most stable text behind the most volatile. The broker TODO measured the same thing. Emit `system` = persona tier + nerd.md, byte-identical across a session's turns for a persona. Deliver the turn, relevance, injected and retrieved tiers as a `role:"system"` message at the head of the turn (Opus 5 / 5.5 / 4.8, Fable) or as the anchor's leading block elsewhere. The anchor is already the ledger's first user message (`working_context.go:466-467`). Choose the TTL (5m vs 1h) by the measured gap between a user's turns.
- **Plugs in at:** `internal/prompt/compiler.go` (return tiers separately), `session/executor.go:1146`, `working_context.go:466`.
- **Impact and basis:** The skeleton was 11,786 tokens (46%) of a 25.7k coder prompt (`coder_token_cost_proposal.md:10-12`). Tools (~5.5k) also become cross-turn cacheable once T-02(d) orders them stably. That is up to ~17k tokens per turn's first round read at 0.1x instead of written at 1.25x, when turns are within the TTL.
- **Measure it by:** first-round `cache_read` per turn, and adherence evals (nerd.md rules followed).
- **Risks:** Instruction position changes adherence. The broker TODO deliberately deferred this pending an eval. Treat it as experiment-gated.
- **Effort:** M. **Needs Jev?** n. **Confidence:** med.
- **Cross-over:** Could help or hurt nerd.md adherence. The sibling lane should own the eval.

### T-19: Localize before the loop: a Jev-picked prefetch of the elements the edit will need
- **Pitch:** Exploration rounds are the most frequent and least productive. Hand the model its targets up front.
- **Mechanism:** At loop start, build candidates from the structure index: symbols named in the brief, their callers and callees, and the issue retrieval hits (`retrieveForTurn`, `session/issue_retrieval.go:43-71`). Ask Jev one Choice with ≤255 candidates, "Which elements will the change edit?", plus a Noul per top-k, "Needed to understand the change?" Attach the top elements' source (codec-projected) to the anchor via `withRetrievalBrief` (`working_context.go:212`). This is Agentless's hierarchical localization done by a System-One model instead of an LLM ladder.
- **Plugs in at:** `session/issue_retrieval.go:43-71`, `working_context.go:173-223` (`beginWorkingLoop` anchor), `internal/world/structure_index.go`.
- **Impact and basis:** A campaign measured on 2026-09-21 made 166 raw filesystem calls against 14 `get_elements` (`world/structure_index.go:31-35`), so exploration dominates. Each avoided round is one ~45-55k request. The rounds saved are unmeasured.
- **Measure it by:** rounds to first write, prefetched elements actually edited (precision), and elements edited that were not prefetched (recall).
- **Risks:** A wrong prefetch anchors the model on the wrong code. Present it as evidence, not instruction, and keep search available.
- **Effort:** M. **Needs Jev?** y. **Confidence:** low-med.
- **Cross-over:** Quality up when localization is right. That is the Agentless result.

### T-20: Diff restatement instead of "read it again"
- **Pitch:** When a carried observation goes stale, the harness tells the model to re-read the file (a round plus a full read), even when the change was the model's own edit.
- **Mechanism:** `updateLedger` appends `staleObservationNotice`, "read again what you still need" (`working_context.go:651-661`, `:690-692`). When the revision change came from this loop's own edit (known from `recordWorkingResult`'s element entity and revision), append the element-level unified diff between the observed revision and now, capped. Keep the notice only for external changes.
- **Plugs in at:** `session/working_context.go:598-692`, `internal/diff/`.
- **Impact and basis:** Avoided re-read rounds (each ~45-55k). Frequency is unmeasured; count `read_file` or `get_element` calls on entities written earlier in the same turn.
- **Risks:** Diffs of large rewrites can be bigger than a re-read. Fall back to the notice above a size cap.
- **Effort:** S. **Needs Jev?** n. **Confidence:** med.
- **Cross-over:** Fewer stale-coordinate edits.

### T-21: Prefix-warm fan-out for campaign shards and parallel subagents
- **Pitch:** N parallel requests with the same prefix all pay full price, because a cache entry becomes readable only once the first response starts streaming.
- **Mechanism:** When the orchestrator launches ≥2 tasks with the same persona, issue the first, wait for its first streamed token (or a `max_tokens:0` warm on Anthropic), then release the rest. Group same-persona tasks adjacently in the schedule so the persona tier stays warm within the TTL. On Meta, also respect route overflow above ~15 requests per minute per key (`client_meta_responses.go:486-489`) by rate-shaping per key.
- **Plugs in at:** campaign orchestrator task dispatch (`internal/campaign/orchestrator_*.go`), `internal/core/scheduled_llm_client.go` (the API scheduler).
- **Impact and basis:** (N-1) × shared-prefix tokens per fan-out at 0.1x instead of 1x/1.25x. Fan-out width is unmeasured.
- **Risks:** Adds first-token latency to the fan-out.
- **Effort:** M. **Needs Jev?** n. **Confidence:** low-med.
- **Cross-over:** none.

---

## Wild cards (bold and lower-confidence)

**W-1: Action-level speculative decoding with Jev as the draft model.** *Low confidence, potentially large.* Many rounds are predictable: after an edit comes a build or test, after a failing test comes a read of the failing element. Each round, a Jev Choice over a small menu of *deterministic* next actions ("run_impacted_tests", "get_element <failing ref>", "none") runs as the draft. When confidence ≥ θ and the action is read-only or verification, the harness executes it *before* asking the frontier model and attaches the result to the same tool_result turn. The frontier model then "accepts" the draft by using it, and one round disappears. The acceptance rate is logged per (regime, last action) and graduated in Mangle like T-10. Safety holds: the drafted action still goes through `permitted/3` and the effect table, and only read or verify effects are drafted.

**W-2: A cheap-draft, compiler-verified patch cascade.** *Medium-low confidence.* For bounded edits (one element, known target), the worker slot drafts 2-3 candidate element bodies in parallel through `replace_element` dry-runs. The syntax guard, build and `run_impacted_tests` filter them deterministically. Jev ranks the survivors with a Choice ("which candidate best satisfies the brief?"), and the frontier model runs only when none survive. This is Agentless's sample, filter and rank with FrugalGPT's cascade, and the judge is the compiler rather than a model. It needs worktree isolation per candidate (Phase 5 lanes in the broker TODO).

**W-3: Self-calibrating neural predicates from free labels.** *Medium confidence as research.* codeNERD produces ground truth nobody pays for: turn verdicts, ratchet keep or revert, critic uplift accepted or refuted, atom co-use lift (`internal/prompt/couse.go`), and pinning-gate results. Feed them back as labels to set each `neural_threshold(Q, T)` by maximizing expected tokens saved subject to a false-skip ceiling, recomputed nightly into a generated `.mg` fact file. The thresholds then live in policy (satisfying `TestExecutiveLiteralBudget`) and are derived from evidence rather than taste.

**W-4: Jev selects the flesh atoms, graded by co-use lift.** *Medium-low confidence.* The flesh tier is 54% of the coder prompt at a 0.10 similarity floor (`coder_token_cost_proposal.md:12`, `:25-30`). One Jev request carries the turn's state and up to 255 flesh candidates per Choice question (several questions for larger sets): "which of these would change what the model does on this task?" Keep only high-p atoms. Grade it against `nerd meter atoms` outcome lift instead of vector similarity. The prompt shrinks, and the prompt cache key (`Hash()`) stays stable if selections are quantized.

**W-5: A cache-aware gate scheduler.** *Medium confidence, small code.* The kernel knows each provider's TTL (`epoch.go` `CacheEconomics`) and each gate's typical duration. The critic runs 2-3 minutes on the planner slot while the main loop's 5-minute Anthropic entry, timed from its last request *start*, ages. Derive `keepalive_owed(Loop) :- gate_running(G), gate_expected_ms(G, D), loop_cache_age_ms(A), A + D > ttl_ms(...)` and send a `max_tokens:0` keep-alive at a fraction of a write, or set the 1h TTL on that loop. It turns "the cache expired during verification" from an invisible cost into a derived obligation.

**W-6: Negative prompts that pay for themselves.** *Low confidence.* Several atoms exist to stop costly behaviors: whole-file rewrites, grep storms, re-reading. Measure each atom's *token* effect, meaning receipts on turns with the atom against without (co-use already records selections), and let the atom's selection score include "tokens saved per token spent". The prompt then contains only instructions that are net-positive in tokens. That is JIT compilation with a cost function, not just a relevance function.

---

## Sources

Internal (read at `d61dbc6`): `Docs/audits/2026-09-09-system-hardening/02-PROMPT-BUDGET.md`; `Docs/architecture/broker/TODO.md`; `Docs/audits/coder_token_cost_proposal.md`; `Docs/journeys/impl/S19-compilation-scope-measurement.md`; `Docs/architecture/perception/OPEN-QUESTIONS.md`; the claude-api skill reference `shared/prompt-caching.md` (for caching mechanics, TTLs, invalidation hierarchy, per-message effort, and preserved-thinking constraints).

External:
- The Complexity Trap (observation masking vs LLM summarization), JetBrains Research: https://arxiv.org/abs/2508.21433 · https://github.com/JetBrains-Research/the-complexity-trap · https://blog.jetbrains.com/research/2025/12/efficient-context-management/
- OpenHands context condenser: https://docs.openhands.dev/sdk/guides/context-condenser · https://www.openhands.dev/blog/openhands-context-condensensation-for-more-efficient-ai-agents · https://arxiv.org/abs/2511.03690
- Aider repository map (tree-sitter + PageRank, token-budgeted): https://aider.chat/docs/repomap.html · https://aider.chat/2023/10/22/repomap.html
- SWE-agent agent-computer interface: https://arxiv.org/abs/2405.15793 · https://swe-agent.com/latest/background/aci/
- Agentless (localize, repair, validate; cost per issue): https://huggingface.co/papers/2407.01489 · https://github.com/openautocoder/agentless
- Anthropic, effective context engineering (tool-result clearing, compaction): https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents · https://platform.claude.com/cookbook/tool-use-context-engineering-context-engineering-tools
- Prompt caching, OpenAI (automatic, `prompt_cache_key`): https://developers.openai.com/api/docs/guides/prompt-caching · https://openai.com/index/api-prompt-caching/
- Prompt caching comparison and Gemini implicit caching: https://docs.litellm.ai/docs/completion/prompt_caching · https://openrouter.ai/docs/guides/best-practices/prompt-caching
- FrugalGPT and RouteLLM (cascades and routing): https://portkey.ai/blog/implementing-frugalgpt-smarter-llm-usage-for-lower-costs/ · https://arxiv.org/html/2605.18796
- Neural predicates in Datalog and logic programming (DeepProbLog, Scallop, Vieira): https://github.com/ML-KULeuven/deepproblog · https://www.cis.upenn.edu/~mhnaik/papers/neurips21.pdf · https://arxiv.org/html/2412.14515v1
- Jev (TypeSafe AI) API docs, as briefed: https://docs.typesafe.ai/api
