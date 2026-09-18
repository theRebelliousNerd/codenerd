# S12 — What happens when `context_budget/2` becomes a real fact

## Status

- [x] 1. Derivation graph (fact -> rule -> rule -> Go consumer)
- [x] 2. What each derived predicate changes
- [x] 3. The thresholds
- [x] 4. Tool catalogue today vs after
- [x] 5. `final_injectable` vs `injectable_context`
- [x] 6. Recommendation with evidence
- [x] 7. Open

Author: study only, read-only on the repo except this file. No builds, no git writes, no subagents.
Branch `dogfood/c2-closure`. Repo `C:\CodeProjects\codeNERD`.

---

## 1. Every rule that reads `context_budget/2` or its derivatives

**The fact.** `Decl context_budget(ShardID, Budget) bound [/string, /number]` —
`internal/core/defaults/schemas_prompts.mg:136`. Zero production Go assertors anywhere
in the tree (confirmed by grep across `internal/` and `cmd/`, excluding the crash-dump
copies under stray `.nerd/debug/debug_program_ERROR.mg` files, which are not live policy).
The only places that write this predicate are tests:
`internal/core/stage_context_test.go:66` (`context_budget("shard-1", 20000)`) and `:90`
(same value, different test). This matches C-11's "Side B: none" exactly. A second,
similarly-named predicate exists and is a red herring: `prompt_context_budget(ShardID,
TokensUsed, TokensAvailable)` (`schemas_prompts.mg:91`) is declared, commented "context
window tracking", and never asserted or read anywhere — it is a second, independent piece
of dead schema, not an alternate name for the fact this study is about.

**Tier 1 — the two budget-state predicates (`prompt_context.mg`).**

```
context_budget_constrained(ShardID) :-              # prompt_context.mg:113-117
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget < 5000.

context_budget_sufficient(ShardID) :-                # prompt_context.mg:119-123
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget >= 5000.
```

Both require `active_shard(ShardID, _)`, which **is** live — asserted every turn by
`internal/core/shards/manager_spawn.go` and `internal/shards/registration.go` (grep
confirms `Predicate: "active_shard"` in both, plus `mustAssert(..., "active_shard", ...)`
in tests). So the only missing precondition, today, is `context_budget` itself; the
moment it is asserted, exactly one of these two derives per shard, every turn.

**Tier 2a — the context-injection branch (`prompt_context.mg`), C-11 Side A #1.**

```
final_injectable(ShardID, Atom) :-                   # prompt_context.mg:125-127
    context_budget_sufficient(ShardID),
    injectable_context(ShardID, Atom).

final_injectable(ShardID, Atom) :-                   # prompt_context.mg:130-132
    context_budget_constrained(ShardID),
    injectable_context_priority(ShardID, Atom, /high).
```

`injectable_context(ShardID, Atom)` itself is populated by eight `shard_context_atom`
producers threshold-filtered at `Relevance > 50` (`prompt_context.mg:81-84`), **plus** a
ninth producer in `stage_context.mg:263-267` (the stage-guidance bridge, unconditional —
does not read budget at all). None of `injectable_context`'s own producers read
`context_budget`; the budget split only happens one layer up, in `final_injectable`.

**Tier 2b — the tool-catalogue branch (`stage_context.mg`), C-11 Side A #2.**

```
stage_shard_tool_allowed(ShardID, ToolName) :-        # stage_context.mg:285-292
    active_shard(ShardID, ShardType),
    context_budget_sufficient(ShardID),
    task_stage(Stage),
    tool_capability(ToolName, Capability),
    stage_permits_capability(Stage, Capability),
    tool_registered(ToolName, _),
    !stage_tool_suppressed(Stage, Capability).

stage_shard_tool_allowed(ShardID, ToolName) :-        # stage_context.mg:294-300
    active_shard(ShardID, ShardType),
    context_budget_constrained(ShardID),
    task_stage(Stage),
    tool_capability(ToolName, Capability),
    stage_tool_preferred(Stage, Capability),
    tool_registered(ToolName, _).
```

`stage_permits_capability(Stage, Capability) :- stage_tool_preferred(Stage, Capability).`
(`stage_context.mg:249-250`) — so under "sufficient" the allowed set is *every stage's
preferred capability minus that stage's suppressed ones*; under "constrained" it is
*preferred capabilities only*, with no suppression subtraction (there is nothing to
subtract from a smaller set). Feeding this: `task_stage(Stage)` — live, derived in
`internal/core/defaults/policy/task_stage.mg:121` from `user_intent`, so this precondition
also already fires every turn. `tool_capability/2` and `tool_registered/2` are live EDB,
asserted by `internal/core/tool_registry.go:263-277` when tools register.

**Tier 3 — what `final_injectable` itself feeds, beyond the compiler.**

```
activation(Atom, 95) :- final_injectable(_, Atom).    # prompt_context.mg:137-138
...
context_injection_effective(ShardID, Atom) :-         # prompt_context.mg:187-189
    final_injectable(ShardID, Atom),
    shard_success(ShardID).
learning_signal(/effective_context, Atom) :-          # prompt_context.mg:192-193
    context_injection_effective(_, Atom).
promote_to_long_term(/context_pattern, Atom) :-       # prompt_context.mg:197-199
    context_effective_count(Atom, N), N >= 3.
```

So `final_injectable` has two downstream forks, not one: the compiler-facing fork (which
turns out to be moot — see §5) and a learning-signal fork (`activation` /
`context_injection_effective` / `learning_signal` / `promote_to_long_term`). Grepped for
Go consumers of all four: **none** in production code; `activation` and
`learning_signal`/`promote_to_long_term` are queried only from
`internal/campaign/orchestrator_behavior_test.go:181` and
`internal/mangle/mangle_validation_test.go` respectively. This whole learning fork is
dead independent of whether `context_budget` ever gets asserted — worth knowing so nobody
credits S12 with reviving it.

**Full graph:**

```
context_budget(ShardID, Budget)                         [schemas_prompts.mg:136 — NO PRODUCTION ASSERTOR]
  |-- Budget <5000  --> context_budget_constrained(ShardID)   [prompt_context.mg:113]
  |-- Budget >=5000 --> context_budget_sufficient(ShardID)    [prompt_context.mg:119]
        |
        +--> final_injectable(ShardID, Atom)                  [prompt_context.mg:125,130]
        |       |--> activation(Atom, 95)                     [prompt_context.mg:137]  -- dead (test-only reader)
        |       +--> context_injection_effective/learning_signal/promote_to_long_term    -- dead (test-only reader)
        |       (compiler.go:1076 reads injectable_context DIRECTLY, not final_injectable -- see S5)
        |
        +--> stage_shard_tool_allowed(ShardID, ToolName)       [stage_context.mg:285,294]  -- dead (test-only reader:
                                                                                               stage_context_test.go:68,71,74,92;
                                                                                               mg_decl_body_literal_wiring_test.go:130
                                                                                               asserts final_injectable directly, same story)
```

## 2. What each derived predicate changes

Grepped `internal/` and `cmd/` for `kernel.Query("<pred>")` / `kernel.Query("<pred>`
(both quoting styles) for every predicate name in the graph above. Findings:

| Predicate | Production Go consumer | Decision it makes | Path | What the model/user sees differently |
|---|---|---|---|---|
| `context_budget_sufficient` / `_constrained` | **none** | n/a | n/a | nothing — read only from other Mangle rule bodies |
| `final_injectable` | **none** | n/a | n/a | nothing — `internal/core/mg_decl_body_literal_wiring_test.go:130` and `internal/core/stage_context_test.go` are the only readers, both tests |
| `injectable_context` | **`internal/prompt/compiler.go:1076`** (`collectKernelInjectedAtoms`) | builds the "KERNEL-INJECTED CONTEXT" prompt atom (mandatory, priority 95) from every row matching the compiling shard's id or `*`/`/_all` | every JIT-compiled prompt, i.e. every chat turn and every shard spawn (`internal/articulation/prompt_assembler.go` -> `internal/prompt/compiler.go`) | this is the ONE real consumer in the whole graph, and it reads `injectable_context` directly — never `final_injectable`, so it is unaffected by `context_budget` today. See §5. |
| `activation` | **none** | n/a | n/a | `internal/campaign/orchestrator_behavior_test.go:181` only |
| `context_injection_effective` / `learning_signal` / `promote_to_long_term` | **none** | n/a | n/a | `internal/mangle/mangle_validation_test.go` only |
| `stage_shard_tool_allowed` | **none** | n/a | n/a | `internal/core/stage_context_test.go:68,71,74,92` only |

**The one thing that IS live and already narrows the tool catalogue** is a completely
different mechanism that the contract registry's C-11 entry does not mention because it
does not touch `context_budget` at all:

- `relevant_tool(ShardType, ToolName)` (`internal/core/defaults/policy/tool_routing.mg`,
  derived from `shard_capability_affinity` + `tool_base_relevance >= 30`, or
  `tool_intent_relevance >= 70`, or `/system` shards seeing everything) is queried by
  `internal/core/shards/manager_tools.go:99` (`queryRelevantTools`), called from
  `internal/core/shards/manager_spawn.go:241` on every shard spawn.
- The result is then cut with `sm.trimToTokenBudget(tools, query.TokenBudget)`
  (`manager_tools.go:232-254`), pure Go arithmetic (`len(name+description+path)/4 + 20`
  tokens per tool, greedily packed until the budget is exceeded).
- `query.TokenBudget` is **hardcoded to `2000`** at `manager_spawn.go:229` — it does not
  come from `jit.token_budget` or `context_window.max_tokens` at all. (This 2000 constant
  is a known separate issue — see `feedback_tui_dogfood_no_answers.md` in project
  memory, "nuke the tool budget" — out of scope for this study but worth flagging because
  it sits directly on the tool-catalogue path §4 is about.)

So today's real narrowing is: capability-affinity threshold (Mangle) -> fixed 2000-token
greedy pack (Go). `context_budget`/`stage_shard_tool_allowed` do not participate in it at
all, because nothing in `manager_tools.go` or `manager_spawn.go` queries
`stage_shard_tool_allowed`. Wiring S12 exactly as scoped (assert `context_budget`; switch
the compiler to read `final_injectable`) leaves this tool-catalogue path completely
untouched — `stage_shard_tool_allowed` would start deriving real facts, but nothing reads
them, so no shard's actual tool list changes. The registry's phrasing ("switches on a
tool-catalogue narrowing nobody has measured") is accurate about the *risk of the next
step* (wiring `manager_tools.go` to `stage_shard_tool_allowed` once `context_budget` is
real makes it tempting to also cut over the tool source) but not about the literal S12
diff by itself — see §6.

## 3. The thresholds

**The rules, quoted exactly** (`internal/core/defaults/policy/prompt_context.mg:113-123`):

```
context_budget_constrained(ShardID) :-
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget < 5000.

context_budget_sufficient(ShardID) :-
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget >= 5000.
```

5000 tokens is the entire cutoff. There is no third band and no hysteresis.

**Live config** (`C:\CodeProjects\codeNERD\.nerd\config.json`, read-only):

```
"jit": { "token_budget": 200000, "reserved_tokens": 8000, ... }
"context_window": { "max_tokens": 200000, ... }
```

**Code defaults** (both agree with the live config, coincidentally or not):

- `internal/config/jit.go: DefaultJITConfig().TokenBudget = 200000`
- `internal/config/memory.go: DefaultContextWindowConfig().MaxTokens = 200000`

**How the effective number is computed today** (the number an S12 wiring would plausibly
assert as `Budget`): `UserConfig.GetEffectiveJITConfig()`
(`internal/config/user_config.go:1599-1612`) takes `GetJITConfig().TokenBudget` and clamps
it down to `context_window.max_tokens` if the latter is smaller — with the live values
both at 200000, no clamp happens. `internal/system/factory.go:861` sets
`bctx.jitCfg = appCfg.GetEffectiveJITConfig()`, which flows into
`compilerCfg.DefaultTokenBudget` (`factory.go:1565-1566`) and into
`pa.SetJITBudgets(bctx.jitCfg.TokenBudget, ...)` (`factory.go:1631`), landing as
`cc.TokenBudget` in `internal/articulation/prompt_assembler.go:135-137`. This is the one
number in the whole system that plausibly deserves the name "the shard's context budget
this turn" — and it is **200000**, forty times the constrained/sufficient cutoff.

**Which branch a real turn takes:** `context_budget_sufficient` — always, under both the
live config and the code defaults, for every shard, every turn. `context_budget_constrained`
(`Budget < 5000`) requires a `jit.token_budget` (post-clamp) under five thousand tokens —
smaller than most single system prompts, let alone a full JIT-compiled turn. No test in
the repo exercises that branch either: `internal/core/stage_context_test.go:66` and `:90`
both assert `context_budget("shard-1", 20000)` — four times the threshold, deliberately in
the "sufficient" band. A grep for any assertion of `context_budget` below 5000, anywhere
in the tree (tests included), returns nothing.

**Campaign-side `ContextBudget` (Go, not a fact) tells the same story.** `ContextBudget`
on `internal/campaign/assault_types.go:56/74` and `internal/campaign/recurse_plan.go:90`
also defaults to 200000; it feeds `internal/campaign/context_pager.go`'s
`NewContextPager`/`SetBudget`, which just splits that number into percentage reserves
(5/30/15/40/10) and tracks `usedTokens` for a *usage ratio*
(`orchestrator_control.go:138-139`) — it never derives anything resembling "sufficient vs
constrained" and never touches Mangle. (Note for the registry: this file lives at
`internal/campaign/context_pager.go`, not `internal/context/context_pager.go` as C-11's
prose names it — there is no `internal/context/` package in this tree; worth a small
registry correction.)

**Conclusion:** under every config value on disk or in code, `context_budget_constrained`
is unreachable in practice. The 5000-token threshold looks like it was picked for a much
smaller assumed budget (a single-purpose sub-agent call, maybe, or an early-project 8k-32k
context window) and was never revisited when the JIT budget grew to 200k. Wiring S12
verbatim would not exercise the constrained branch at all under current configuration —
it would just make `context_budget_sufficient` derive for every shard, every turn, which
(per §1/§2) changes nothing downstream because nothing reads `final_injectable` or
`stage_shard_tool_allowed` in production.

## 4. The tool catalogue today vs after

**Producer of the real, live tool catalogue today:** `internal/core/shards/manager_tools.go`
`queryRelevantTools` (called from `manager_spawn.go:241` on every shard spawn, and from
`manager_tools.go:117` when `relevant_tool` comes back empty for the shard type). It
queries `relevant_tool(ShardType, ToolName)`, and `relevant_tool` only derives three ways
(`internal/core/defaults/policy/tool_routing.mg:141-155`):

1. `tool_base_relevance(ShardType, ToolName, BaseScore) :- tool_capability(ToolName, Cap), shard_capability_affinity(ShardType, Cap, _), tool_registered(ToolName,_)`, `BaseScore >= 30`.
2. `tool_intent_relevance(ToolName, Score) :- ..., intent_requires_capability(Verb, Cap, Score), tool_capability(ToolName, Cap)`, `Score >= 70`.
3. `relevant_tool(/system, ToolName) :- tool_registered(ToolName, _)` — `/system` shards see everything, unconditionally.

**The capability-vocabulary gap (a bigger, independent finding).** Rules 1 and 2 both
require a `tool_capability(ToolName, Capability)` fact whose `Capability` is one of
`/generation /debugging /transformation /inspection /validation /execution /analysis
/knowledge` — the vocabulary `shard_capability_affinity` and `intent_requires_capability`
are defined over. Grepping every production Go path that asserts `tool_capability`
(`internal/core/tool_registry.go:443` via `collectToolFacts` at :275-280, the only
assigner of `Tool.Capabilities` outside test files) shows the value asserted is
`def.Category` from `internal/init/tools.go`'s `ToolDefinition` — literal strings like
`"build"`, `"test"`, `"lint"`, `"format"`, `"deps"`, `"typecheck"`, `"run"`, `"check"`,
`"docker"`, `"setup"` (the categories of *detected external build binaries*: `go_build`,
`go_test`, `npm_test`, `cargo_clippy`, etc.). None of these match the `/generation...`
vocabulary. A repo-wide grep for `MangleAtom("/generation")`, `MangleAtom("/inspection")`,
etc. (the way that vocabulary would have to be constructed to type-check against
`Decl tool_capability(ToolName, Capability) bound [/string, /name]`) returns exactly one
file: `internal/core/stage_context_test.go`. **No shipped `.mg` file asserts a ground
`tool_capability` fact in this vocabulary either** (grep for `^tool_capability\(` across
`internal/core/defaults/**/*.mg`: zero hits). So for the CLI's own model-facing tools
(the read/write/edit/execute/grep equivalents — the ones the tests stand in for with
`"read_file"`, `"write_file"`, `"run_tests"`), **no `tool_capability` fact of any kind is
ever asserted in production**, in this vocabulary or any other; `RegisterTool` (the plain
`name, command, shardAffinity` entry point most of the CLI's own tools would go through)
doesn't set `Capabilities` at all.

**Consequence for coder / tester / reviewer today:** rules 1 and 2 of `relevant_tool`
never fire for `/coder`, `/tester`, or `/reviewer` in production — there is no
`tool_capability` fact for them to join against. So `relevantFacts` filtered to the
shard's own type is always empty, and `queryRelevantTools` (`manager_tools.go:116-119`)
falls through to `sm.queryToolsFromKernel()` — **every tool the kernel has ever
registered**, `tool_registered` alone, no capability filter at all — then
`sortToolsByPriority` (a no-op here, since every `tool_base_relevance` score for these
shard types is also absent, so every score is 0 and the sort is stable-no-op), then
`trimToTokenBudget` with the **hardcoded 2000-token budget** from
`manager_spawn.go:229`. **Today's real tool catalogue for coder, tester, and reviewer
shards alike is: "every registered tool, in kernel-query order, greedily packed until
about 2000 tokens are spent" — not a capability- or stage-aware list at all.** The three
shard types are not differentiated from each other today, despite `tool_routing.mg`
declaring different affinities for each — those affinity rows are unreachable EDB.

**What `stage_shard_tool_allowed` would leave them if wired as the tool source.** Two
findings, in order of how surprising they are:

1. `stage_shard_tool_allowed(ShardID, ToolName)` (`stage_context.mg:285-300`) binds
   `ShardType` from `active_shard(ShardID, ShardType)` but **never uses it in the rest of
   the body** — the allowed set depends only on `task_stage(Stage)` (derived from the
   turn's intent, `task_stage.mg:121`), not on whether the shard is a coder, tester, or
   reviewer. If this became the tool source, a coder shard and a reviewer shard active in
   the same `/debug` stage would receive the **identical** tool list. That collapses the
   very shard-type differentiation `tool_routing.mg` was written to express — a different
   kind of narrowing than "fewer tools," and one the registry's C-11 entry does not call
   out.
2. Because of the vocabulary gap above, `tool_capability(ToolName, Capability)` never
   binds to any registered production tool with `Capability` in the `stage_tool_preferred`
   / `stage_tool_suppressed` vocabulary either (those tables use the exact same
   `/generation...` names). So **`stage_shard_tool_allowed` would derive the empty set for
   every shard, every stage, today** — sufficient or constrained makes no difference, both
   branches join through the same ungrounded `tool_capability`. Naming "every tool that
   would disappear": all of them, for every coder/tester/reviewer shard, in every stage —
   not a narrowing, a total blackout. That is a strictly worse outcome than today's
   "everything, capped at 2000 tokens" fallback.

Concretely, for the three shard types named in this study, assuming the capability gap
were separately fixed so the affinity tables in `tool_routing.mg` actually meant something
(not assumed true today — flagged as its own prerequisite in §6):

| Shard | Today (production, real) | `stage_shard_tool_allowed`, sufficient budget, hypothetical `/implement` stage | `stage_shard_tool_allowed`, constrained budget, same stage |
|---|---|---|---|
| coder | all registered tools, kernel order, capped ~2000 tokens | tools tagged `/generation`, `/validation`, `/execution` (stage_tool_preferred for `/implement`; no suppressed rows) | tools tagged `/generation`, `/validation`, `/execution` (`stage_tool_preferred` only — same set, because `/implement` has no forbidden rows) |
| tester | same fallback as coder | same set as coder (rule ignores `ShardType`) — even though `tester`'s own affinity table prefers `/validation`, `/execution`, `/inspection` | same as sufficient |
| reviewer | same fallback as coder | same set as coder (rule ignores `ShardType`) — even though `/review` stage (not `/implement`) would instead prefer `/inspection`, `/analysis` and suppress `/generation`, `/execution` | same as sufficient |

Today's real gap (vocabulary) makes every cell in that table empty in practice; the table
shows what the *intended* design would produce once that gap is fixed — which is the
shape of the risk the contract registry is warning about, just one prerequisite fix away
from being real rather than latent twice over.

## 5. `final_injectable` vs `injectable_context`

**Today:** `internal/prompt/compiler.go:1076` (`collectKernelInjectedAtoms`) queries
`injectable_context` directly. `injectable_context(ShardID, Atom)` is the union of nine
producers: eight `shard_context_atom`-threshold rules in `prompt_context.mg:29-84`
(`Relevance > 50` — intent match, specialist knowledge, trace/failure/learning recall,
campaign constraints, exemplars, tool descriptions, successful trace patterns) plus the
unconditional stage-guidance bridge at `stage_context.mg:263-267`. None of the nine reads
`context_budget`. The compiler takes every row matching the shard's id or `*`/`/_all`,
renders them into one prompt atom via `renderKernelContextBlock` (`compiler.go:1034-1051`,
capped at `maxKernelContextRows` and `maxKernelInjectedAtomChars` — Go-side truncation
markers, not a Mangle budget), marks it `IsMandatory = true`, `Priority = 95`, and injects
it. **No pruning by budget happens anywhere in this path today** — a shard with 3 relevant
atoms and a shard with 3,000 relevant atoms both get "everything," subject only to the Go
row/char caps.

**If the compiler switched to `final_injectable`:**

*Gained.* Exactly what `final_injectable`'s two rules add over `injectable_context`
(`prompt_context.mg:125-132`): a real budget gate. Under `context_budget_sufficient`
(§3: always, at current config), `final_injectable(ShardID, Atom) :- injectable_context(ShardID, Atom)`
— **byte-identical to what the compiler already gets**, so switching sources changes
*nothing* while the budget stays above 5000. The only behavior the switch could add is the
`context_budget_constrained` branch:
`final_injectable(ShardID, Atom) :- injectable_context_priority(ShardID, Atom, /high)` —
i.e. only atoms scoring `Relevance >= 80` in `shard_context_atom`
(`prompt_context.mg:88-91`) survive: intent-match (90), specialist knowledge (80),
trace-recall (85), failure-recall (90). Dropped in that branch: campaign constraints (70),
exemplars (60), tool descriptions (65), successful trace patterns (50), and — because the
`/high` filter is on `injectable_context_priority`, which is keyed off `shard_context_atom`
only, **not** the stage-guidance bridge — the entire `stage_context.mg:263-267` guidance
atom, since nothing tags stage guidance with a priority bucket at all. So a truly
budget-constrained turn would silently lose its one per-stage guidance string
("stage /debug: require intent target, failure recall...") along with four of the eight
relevance families, with no log line marking that drop (`renderKernelContextBlock`'s
truncation notice only fires on the Go-side row/char cap, not on this Mangle-side one).

*Lost.* Nothing is lost while `context_budget_sufficient` holds (the common case, per §3):
`final_injectable`'s sufficient-branch content is defined to equal `injectable_context`
exactly. The only loss condition is the unreachable `<5000` branch — reachable only by a
misconfiguration (`jit.token_budget` set absurdly low), at which point the loss is the
four relevance families above and the stage guidance atom, silently.

**Net effect of the compiler-side half of S12, standing alone, given current config:**
none. It is a no-op switch dressed as a safety mechanism, because the branch it exists to
protect against never derives. The only way it becomes consequential is either (a) a user
sets `jit.token_budget` below 5000 (which would already be catastrophic for reasons
unrelated to this predicate — an 8k-token system prompt alone would not fit), or (b) the
5000 threshold in `prompt_context.mg:113-123` is revisited to something that actually
corresponds to a fraction of a 200k-token budget (e.g. a percentage-of-remaining check
instead of an absolute floor picked when budgets were an order of magnitude smaller).

## 6. Recommendation with evidence

**Wire the fact and the compiler switch (`final_injectable`); do not wire
`stage_shard_tool_allowed` into the tool-assembly path in the same change.**

Evidence for each half:

- **Asserting `context_budget(ShardID, jit_token_budget_effective)` every turn is safe and
  low-value on its own, but closes a real contract gap.** §1/§3 show the precondition
  (`active_shard`) already fires every turn, so this is one `Assert` call in
  `internal/session/executor.go` (or wherever the per-turn fact-assertion pass lives) away
  from being true. §3 shows that under both the live config and the code defaults the
  turn will land on `context_budget_sufficient`, which (§5) makes `final_injectable`
  byte-identical to today's `injectable_context`. So this half is close to risk-free: it
  cannot silently narrow anything under any config seen on disk, and it retires the "Side
  B: none" half of C-11 and the two `TestEveryCompileAssertsAContextBudget`-shaped gaps
  the S-table (registry §3, row S12) already names as owed tests.
- **Switching the compiler to read `final_injectable` instead of `injectable_context` is
  also close to risk-free given current config** (§5: identical output while sufficient
  holds) but is not free of downside if the 5000 threshold is ever revisited without
  also looking at what a constrained turn drops — the stage-guidance atom disappearing
  silently (no truncation-notice-equivalent) is a real, if currently unreachable, defect
  in `prompt_context.mg`'s constrained branch that should be fixed in the same change that
  makes the branch reachable, not left for a future incident.
- **Do NOT additionally wire `stage_shard_tool_allowed` as the source `manager_tools.go`
  reads for the coder/tester/reviewer tool list, in this change.** §4's evidence is
  decisive: `stage_shard_tool_allowed` ignores `ShardType` entirely (collapsing the three
  shards to one shared per-stage list, which is not what `tool_routing.mg`'s own affinity
  tables intend), and — independent of `context_budget` — no production tool ever carries
  a `tool_capability` fact in the `/generation.../knowledge` vocabulary those tables key
  on, so the predicate would derive **empty for every shard, in every stage**, which is
  strictly worse than today's "every registered tool, capped at 2000 tokens" fallback.
  Wiring `context_budget` does not, by itself, trigger this — nothing currently queries
  `stage_shard_tool_allowed` in Go (§1, §2) — but the natural next PR is exactly this
  wiring, and it would ship a total tool blackout for three of the four domain shard types
  unless the capability-vocabulary gap is closed first (a separate piece of work: either
  assert real `/generation`-style capabilities for the CLI's built-in tools, or rewrite
  `tool_routing.mg`/`stage_context.mg` to key off the categories that are actually
  asserted today — `/build /test /lint /format /deps /typecheck /run /check /docker
  /setup` — which is a design decision, not a wiring fix, and belongs in its own study).
- **The hardcoded 2000-token tool budget at `manager_spawn.go:229`** is adjacent to this
  whole area (it is the one thing that currently, actually, narrows tool lists) and is
  already flagged in project memory (`feedback_tui_dogfood_no_answers.md`, "nuke the tool
  budget"). Fixing it is out of scope here but should not be conflated with S12 — it is a
  Go constant, not a Mangle wiring gap.

**What a live run should record to confirm this, once `context_budget` is asserted:**

- A log line at the assertion site (e.g. `session/executor.go`) of the exact
  `(ShardID, Budget)` pair asserted each turn — needed to confirm the value really is the
  post-`GetEffectiveJITConfig()` number (200000 today) and not some other candidate
  (raw `jit.token_budget` pre-clamp, or a per-turn *remaining* budget that nothing in the
  current codebase computes — §3 confirms no "remaining tokens this turn" concept exists
  outside the campaign-only `ContextPager`, which never reaches Mangle).
- `nerd meter` (or equivalent) fields for `context_budget_sufficient` vs
  `context_budget_constrained` derivation counts per turn — if `constrained` ever derives
  in a live run under normal configuration, that is itself a signal the 200k/5k ratio
  assumption in this study is stale and needs re-checking, not evidence the wiring works
  as intended.
- A before/after diff of the exact bytes fed to `renderKernelContextBlock` for a handful
  of real shard turns (coder, tester, reviewer, one campaign phase) — confirming §5's
  "byte-identical while sufficient" claim empirically rather than by rule-reading alone,
  since Mangle set semantics and the Go string-matching in `matchesShard` could interact
  in ways a static read misses.
- If `stage_shard_tool_allowed` is ever wired in a follow-up: a row count per shard type
  per stage before shipping — §4 predicts zero rows for every non-`/system` shard today;
  that count going from "N tools" to "0 tools" for a live coder shard is the regression
  signal this study exists to head off.

**Test debt this confirms, referencing the registry's own S12 row
(`04-contract-registry.md` §3):** `TestEveryCompileAssertsAContextBudget`,
`TestStageShardToolAllowedDerivesForALiveShard`, `TestCompilerReadsFinalInjectable`, and
`TestInjectableContextSlotOneIsSelectable` are all still TO WRITE. This study adds one
more that isn't yet named there: a test pinning that `tool_capability` is asserted, in
production, for at least the CLI's own built-in tools, in whatever vocabulary
`relevant_tool`/`stage_shard_tool_allowed` actually key on — without it, any future PR
that wires either predicate into a live tool list has no test that would fail before it
reaches a user.

## 7. Open

- Where should `context_budget`'s asserted value actually come from — the flat
  `GetEffectiveJITConfig().TokenBudget` (constant all turn, per §3), or a genuine
  per-turn *remaining* budget (total minus system prompt minus history minus atoms
  already selected)? Only the flat version is a one-line change; the remaining-budget
  version requires new Go accounting that does not exist yet anywhere in the per-shard
  (non-campaign) path.
- Is 5000 the right absolute-token threshold at all, given both config values on disk
  sit at 200000 (a 40x margin)? §3/§6 suspect it was sized for a much smaller assumed
  budget and never revisited. Nobody appears to own this number's provenance.
- The capability-vocabulary gap in §4 is bigger than S12: it means `tool_routing.mg`'s
  entire `shard_capability_affinity` / `intent_requires_capability` design has been inert
  since it was written, for reasons unrelated to context_budget. Does this deserve its own
  contract-registry entry and its own study before anyone touches `stage_shard_tool_allowed`?
  This document treats it as a blocking prerequisite for the tool-catalogue half of S12,
  not as something S12 should absorb into its own scope.
- `prompt_context.mg`'s constrained branch of `final_injectable` silently drops the
  stage-guidance atom (§5) because `injectable_context_priority` is never computed for it.
  If the constrained branch is ever made reachable (by revisiting the 5000 threshold),
  should stage guidance get its own priority tag, or should it be treated as
  always-mandatory regardless of budget (parallel to how the compiler already marks the
  whole kernel-context block `IsMandatory = true`)?
- This study did not check whether `internal/session/executor.go` (or wherever the S12
  row's Go consumer list points) has an existing per-turn fact-assertion pass that
  `context_budget` could be added to cheaply, versus needing a new one — that is an
  implementation-sizing question for whoever picks this up, not something a read-only
  study should guess at.
