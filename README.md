<div align="center">

```
 ██████╗ ██████╗ ██████╗ ███████╗███╗   ██╗███████╗██████╗ ██████╗
██╔════╝██╔═══██╗██╔══██╗██╔════╝████╗  ██║██╔════╝██╔══██╗██╔══██╗
██║     ██║   ██║██║  ██║█████╗  ██╔██╗ ██║█████╗  ██████╔╝██║  ██║
██║     ██║   ██║██║  ██║██╔══╝  ██║╚██╗██║██╔══╝  ██╔══██╗██║  ██║
╚██████╗╚██████╔╝██████╔╝███████╗██║ ╚████║███████╗██║  ██║██████╔╝
 ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝╚══════╝╚═╝  ╚═╝╚═════╝
```

### **Logic determines reality. The model merely describes it.**

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Kernel](https://img.shields.io/badge/Kernel-Google_Mangle-4285F4?style=for-the-badge&logo=google&logoColor=white)](https://github.com/google/mangle)
[![Architecture](https://img.shields.io/badge/Neuro--Symbolic-8B5CF6?style=for-the-badge)]()
[![Status](https://img.shields.io/badge/Status-Experimental-F59E0B?style=for-the-badge)](#where-it-stands)
[![License](https://img.shields.io/badge/AGPL--3.0-22C55E?style=for-the-badge)](LICENSE)

**A coding agent whose executive is a deductive database, not a language model.**

*The North Star: an agent you can leave running for weeks, whose every "done" comes with a derivation,<br>
and which closes every task in SWE-bench. We haven't reached it yet, and every design decision here is measured against it.*

[North Star](#the-north-star) · [Ideology](#the-category-error) · [Principles](#the-principles) · [Techniques](#the-techniques) · [Where it stands](#where-it-stands) · [Install](#sixty-seconds) · [Commands](#the-surface) · [Docs](#going-deeper)

</div>

---

> **Read this first.** codeNERD is an experimental research harness. It is being built in the open.
> Parts of it work well, parts are half-wired, and some of the ambition below has not been
> demonstrated. The [North Star](#the-north-star) describes where the system is going, not what it
> does today. [Where it stands](#where-it-stands) gives the current state, backed by evidence.

---

## The North Star

A north star is a **final state, not a goal to be hit**. It describes how the finished system
behaves. The harness keeps measuring the distance between what the evidence shows and that state,
and turns the distance into pressure on every agent it runs. Nothing "achieves" the north star.
Every turn moves toward it, and the pressure stays on.

When codeNERD is finished, it behaves like this:

| | The finished system |
|---|---|
| ⏳ **Unattended for weeks** | It runs for weeks without supervision and stays trustworthy the whole time. Nothing stops work that is still making progress. It stops only for a reason derived from evidence: a stall, a repeated failure, or a cancel. Logs, tables and caches stay bounded. Each failure becomes a repair obligation, not a log line someone has to grep for. |
| 🧾 **Every claim is a citation** | When it reports that something is done, a derivation shows it. Its own status report counts for nothing as a claim and only as a citation. A fact store records what is *true*, not what was *attempted*. |
| 🎯 **100% of SWE-bench** | This is a target, not a result. Hitting it requires the same discipline at the scale of one task: a patch that compiles is not a patch that fixes the issue, and a harness that can't tell those apart tops out well below 100%. |
| 🧭 **It never greps around** | The kernel gives the model what it needs, when it needs it and for a stated reason: domain knowledge from SQLite, vector recall, CodeDOM structure and history. The job finishes in as few turns as possible. |
| 🔒 **The harness forces** | Test coverage is not optional. The agent keeps working until the behaviour the project's own north star describes actually holds, not until it *feels* finished. |
| ♻️ **It improves itself** | It hunts its own bugs and extends its own tooling, in Mangle and in Go. Each cycle leaves the gates strictly better and nothing worse. |

The rest of this README covers the ideology behind that target, the techniques that move toward it,
and an honest account of how far there is still to go.

---

## The category error

Every mainstream coding agent asks one model to do two incompatible jobs.

It has to be **creative**: read a gnarly stack trace, guess the bug, invent a fix nobody wrote
down. It also has to be an **executive**: remember what it decided forty turns ago, refuse the
destructive command, keep the plan coherent through an hour of work.

Language models are extraordinary at the first job and structurally unsuited to the second.
Attention is not a ledger. A context window is not a memory. "Please don't delete anything
important" is not an access-control policy.

So agents drift. They forget constraints they agreed to. They confidently do what you told them
never to do. The usual fix is always the same loop: a longer prompt, a sterner tone, a bigger
window. None of that changes the *architecture*.

codeNERD changes the architecture.

---

## The inversion

> The LLM is the **creative center**. A Mangle kernel is the **executive**.

```
        ╭──────────────╮                    ╭──────────────╮
        │   THE MODEL  │                    │  THE KERNEL  │
        ├──────────────┤                    ├──────────────┤
        │ understands  │  ── facts ──▶      │ decides      │
        │ synthesizes  │                    │ remembers    │
        │ invents      │      ◀── plan ──   │ forbids      │
        │ explains     │                    │ proves       │
        ╰──────────────╯                    ╰──────────────╯
           probabilistic                       deterministic
```

The model never *decides*. It perceives and it articulates. In between, natural language and code
are **transduced** into logical atoms. A [Mangle](https://github.com/google/mangle) kernel
(Google's deductive database language, with stratified negation, aggregation and recursion) then
derives what happens next from rules you can read, test and version.

| | Conventional agent | codeNERD |
|---|---|---|
| **Who decides** | The model, token by token | Rules, fact by fact |
| **Memory** | The context window, which evaporates | A fact store, which persists |
| **Safety** | Instructions in a prompt | A `permitted/3` derivation, deny by default |
| **Planning** | Re-improvised every turn | Stratified rules over a fact base |
| **Context** | Whatever the model chose to grep | What the kernel derived the model needs |
| **"Done"** | The model says so | The gates' evidence says so |
| **Auditability** | "Explain your reasoning" (and it confabulates) | `nerd why`, which prints the actual derivation chain |
| **Failure mode** | Confident nonsense | A refusal that cites its reason |

**The consequence everything depends on:** an action that cannot derive `permitted(...)` does not
execute. Not "is discouraged from executing". It does not execute. Safety is a property of the
evaluator, not a request to the model.

---

## The principles

These are the rules every architectural change in this repo is judged against. The full
statement lives in [`agents.md`](agents.md).

**1. Clean fixpoint, not clean loop.** Go can be as big as the work needs, because it is the FFI,
the drivers and the tools. The *executive decisions* must be the kernel's fixpoint over the
facts: what a turn is, whether it is done, what gets delegated and with which task, what enters
the window, and what the verdict is. The drift to hunt for is **a decision computed in Go instead
of derived**.

**2. The harness decides, not the model's discretion.** Mangle plus JIT prompt compilation is meant
to replace free-form subagents and skills. Domain knowledge is *pushed* into the window when the
harness decides it's needed: by user request, task type, or the history of what has been built. It
is never left to the model to go fetch.

**3. The harness forces.** A turn that wrote Go owes the tests that pin it. A turn that deleted
tests owes them back. A verdict that its evidence contradicts is not a verdict. Obligations are
derived and forced, not suggested.

**4. Verify outcomes, not process.** "The command exited 0" is not evidence. "The content is there
and the test fails without it" is. (Early in this repo's life, a merge routine reported 77 branches
merged and had silently dropped a dozen optimizations. Every command it ran had returned zero. That
bug is the North Star's exact antagonist.)

**5. Where a fact sits in the window is a decision.** Lost-in-the-middle is fought with compression,
pruning and ordering, not with bigger windows. Quality and capability come first. Tokens are the
constraint, not the goal.

**6. Tools exist for exactly three things:** to condense the search space, to reduce the number of
turns, and to offload cognition to deterministic code. A change with a blast radius is carried out
by a tool, not hand-edited by the LLM. A tool that does none of the three is cruft.

**7. Progress without arbitrary cutoffs.** There is no run-level wall clock and no tool-call quota
standing in for "done". A limit the user sets is an opt-in constraint. What stops a run is a
derived stall or repeated failure.

**8. Complexity is not a defect.** This can be a very complex system. What gets scored is whether a
decision is *derived* and whether an obligation is *forced*.

---

## The loop

```
   ┌────────────────────────────────────────────────────────────────┐
   │  you: "refactor the auth middleware to use the new token type" │
   └───────────────────────────────┬────────────────────────────────┘
                                   │
              ╔════════════════════▼════════════════════╗
   OBSERVE    ║  PERCEPTION: language ▸ logical atoms   ║
              ║  user_intent(id, /mutation, /refactor…) ║
              ╚════════════════════╤════════════════════╝
                                   │
        ╔══════════════════════════▼══════════════════════════╗
        ║                    MANGLE KERNEL                    ║
        ║   ┌───────────┐   ┌────────────┐   ┌────────────┐   ║
ORIENT  ║   │   FACTS   │◀─▶│   RULES    │──▶│  POLICY    │   ║  DECIDE
        ║   │  (memory) │   │ (planning) │   │ (default   │   ║
        ║   └───────────┘   └────────────┘   │   deny)    │   ║
        ║         ▲                          └─────┬──────┘   ║
        ╚═════════╪════════════════════════════════╪══════════╝
                  │                                │
                  │                    next_action(/refactor)
                  │                                │
              ╔═══╧════════════════════════════════▼════════╗
   ACT        ║  VIRTUAL STORE: the only door to the world  ║
              ║  CodeDOM · shell · fs · git · browser · MCP ║
              ╚════════════════════╤════════════════════════╝
                                   │
              ╔════════════════════▼════════════════════╗
              ║  ARTICULATION: atoms ▸ language         ║
              ╚═════════════════════════════════════════╝
```

Every turn runs **Observe → Orient → Decide → Act**. Facts flow in, a plan is *derived* rather than
improvised, and the only path to the filesystem goes through a gate that checks the constitution
first. Every result flows back in as a fact.

---

## The techniques

This is how the ideology is built. Each item names where it lives, so you can check the claim
against the code.

### 1 · Transduction: language in, atoms out, language back

**Perception** (`internal/perception/`) turns a request into typed intent:
`user_intent(ID, Category, Verb, Target, Constraint)`. **Articulation**
(`internal/articulation/`) runs the Piggyback protocol. The model's reply is an envelope that
separates the `surface_response` a human reads from a typed `control_packet` that the kernel
ingests. The model talks to you and to the kernel at once, and the two channels never mix.

### 2 · A policy corpus, not a prompt

The executive is about 125 `.mg` files under `internal/core/defaults/`, covering schemas plus a
modular policy corpus for delegation, campaigns, the commit gate, context compilation, JIT
selection, CodeDOM safety, repair episodes and more. Routing, delegation and gating are
**derivations**. Which verbs escape to the expensive reasoning model is policy
(`intent_requires_reasoning_model/1` in `delegation.mg`), not a Go `switch`.

### 3 · The constitution: default deny

```mangle
# internal/core/defaults/policy/constitution.mg
# Default deny: permitted must be positively derived.
permitted(Action, Target, Payload) :-
    safe_action(Action),
    pending_action(_, Action, Target, Payload, _),
    !dangerous_content(Action, Payload),
    !dangerous_content(Action, Target).

# A delete the repository can undo (git-tracked, clean) is the one delete allowed without approval.
permitted(/delete_file, Target, Payload) :-
    pending_action(_, /delete_file, Target, Payload, _),
    file_recoverable(Target),
    !dangerous_content(/delete_file, Payload),
    !dangerous_content(/delete_file, Target).
```

Other defences are layered on top:

- **VirtualStore** (`internal/core/virtual_store*.go`): the single door to the world. Every
  effect is routed as a `next_action`, checked against the exact payload, and reported back as
  facts.
- **Commit barrier**: `block_commit("Build Broken")` and `block_commit("Tests Failing")`
  (`commit_gate.mg`). A red tree does not commit.
- **Workspace jail**: paths are resolved against the workspace root, and escapes are refused.
- **Dreamer and shadow mode**: `nerd dream`, `nerd shadow` and `nerd whatif` project an action's
  effects before anything real happens.
- **One definition, two gates**: the interactive path and the VirtualStore path share the same
  matcher, so a delegated shard can't write what the session refused.

### 4 · JIT prompt compilation

Prompts are *compiled*, not concatenated. The library is 350+ YAML files of **atoms** under
`internal/prompt/atoms/`. Each atom is a versioned unit with a priority, dependencies and shard
gating. Every turn, the compiler (`internal/prompt/compiler.go`) picks a skeleton by Mangle
derivation and fleshes it out by vector similarity. It then orders the result by dependency and
fits it into a token budget. To change behaviour, you edit an atom, not a string hidden in a shard.
`/jit` shows the last compilation: which atoms it included, and how long each stage took.

### 5 · CodeDOM: code as structure, not strings

The model reads and edits code **structurally** (`internal/tools/codedom/`):

```
get_elements · get_element · find_symbol · callers_of · callees_of · importers_of
package_outline · predicate_outline · unreferenced_symbols
edit_element · replace_element · insert_element · delete_element · repoint · apply_edits
```

Edits address symbols, not line coordinates that go stale. When a line tool is used, every mutation
reports the shift it caused ("File is now 377 lines (−11). Line numbers at or after 42 are now
STALE."), and a syntax guard refuses edits that would unbalance the file. `repoint` and
`apply_edits` carry out a multi-file change as one tool call, so a blast radius is handled by
deterministic code.

### 6 · The world model and holographic context

`internal/world/` projects the filesystem, ASTs and git history into kernel facts. Ask about one
file and the model gets that file's **neighbourhood**: exported signatures, type definitions, who
calls it, its architectural role and its test coverage. It's assembled from the AST and the fact
store, not guessed from a grep.

### 7 · Context as a kernel decision

`internal/context/` scores every candidate fact with spreading activation, then compresses, prunes
and orders the window. Masking policy (which observations are condensed and which reasoning is
preserved) is derived in `context_compilation.mg`, so the window's contents come from logic rather
than a truncation heuristic.

### 8 · Forcing gates: "done" has to be earned

A `/fix`, `/create` or `/implement` turn cannot close on the model's word
(`internal/session/`, `internal/verification/`):

- **Build and test gates.** They cover the packages the turn wrote *and the packages that import
  them*.
- **Coverage.** Changed code that no test executes is verdict evidence, and a repair round runs
  first.
- **Pinning.** Take out any single function the turn changed, and a test the turn wrote has to
  fail. A test that passes either way is caught.
- **Removed-tests guard.** A turn that deleted tests gets their source handed back and must
  restore them, unless it also deleted the function they test.
- **Vet and format.** `go vet` findings in the turn's own files are evidence. The turn's Go is
  formatted last, before the gates re-measure.
- **Truthful verdicts.** A turn ends `/done` only when the evidence shows it. `/unverified` is an
  honest result, and a failed gate is never reported as a success.

### 9 · Campaigns with checkpoints

`nerd campaign start` breaks a goal into phases and tasks (`internal/campaign/`), runs them in
dependency order under write-set leases, and **verifies each checkpoint before advancing**. A phase
that claims success without evidence fails. `--accept "<cmd>"` makes a command the campaign's
acceptance witness: exit 0 is the only pass, and a failure appends a remediation phase. In chat,
`/campaign assault <scope>` turns the campaign adversarial. A Nemesis persona hunts for panics,
races and edge cases until it stops finding them.

### 10 · Specialists, spawned on demand

Coder, tester, reviewer, researcher, plus any specialist you define with `nerd define-agent` or put
in `.nerd/agents/`. Each one gets a JIT-compiled identity, its own tool allowlist and an isolated
context, so a delegated task can't contaminate the session's history. Per-persona model routing
lives in `shard_profiles`.

### 11 · `nerd.md`: instructions with teeth

It's like `AGENTS.md`, except the machine-readable half is **enforced, not suggested**. Strict YAML
frontmatter becomes kernel facts:

```yaml
---
forbid:
  - match: .nerd/config.json
    reason: user-owned runtime config
---
```

That frontmatter becomes `project_forbidden_path/2` in the fact store, and the write is denied
*before the tool runs*, on the interactive path and the shard path alike (`internal/projectdoc/`).

### 12 · Autopoiesis: it extends itself

When it hits a capability gap, the **Ouroboros** loop (`internal/autopoiesis/`) can generate a new
tool, safety-check it against `go_safety.mg`, compile it, test it in a "Thunderdome" and register
it. The **Northstar Guardian** (`internal/northstar/`) stores a project's own vision and checks work
against it for drift. Prompt evolution tunes atoms from outcomes.

### 13 · The glass box

```bash
nerd why next_action   # the derivation chain (proof tree), fact by fact
nerd query <pred>      # interrogate the kernel directly
nerd transparency      # glass-box telemetry, safety explanations
```

There's no "let me explain my reasoning" theatre. The reasoning *is* the artifact.

---

## Where it stands

Honesty about distance is part of the design. Two programs of record, both kept in the repo,
measure how far the system is from the North Star.

### The elite-harness ladder

[`Docs/journeys/05-elite-harness-ladder.md`](Docs/journeys/05-elite-harness-ladder.md) defines a
ladder of rungs. Each one is climbed by pointing codeNERD at real problems **in this repository**
and keeping only results that pass a strict landing test: the brief states a symptom, not a
diagnosis; codeNERD alone edits the tree; the tests fail before the change and pass after; the
build, vet and test gates are green; the verdict is truthful; and review finds no regression.

| Rung | What passing means | State |
|---|---|---|
| **R0** Stable ground | build, vet and full test suite green on `main` three uncached runs in a row | ✅ passed |
| **R1** One file | three consecutive one-file landings | 🔄 in progress: several landings, the streak keeps resetting on harness defects that each get fixed |
| **R2** Two files | three consecutive landings across a seam (producer + consumer, rule + Go assertor) | 🔄 probes landed assisted |
| **R3** Five files | two five-file landings | 🔄 one unassisted landing (R3-3) |
| **R4** Ten files | two ten-file changes executed through CodeDOM | 🔄 probe: 1 of 2 (assisted) |
| **R5** Fifty files | one fifty-file change from a single prompt | 🔄 probe passed (189 of 189 files correct), but it doesn't count until the rungs below it pass |
| **R6** Architecture docs | codeNERD rewrites every doc in `Docs/architecture` from the code, with every citation checked | 🔄 pilots landed on individual packages |
| **R7** Recursion | unattended cycles that each leave the gates strictly better | ⏳ not started: the instrument doesn't exist yet |
| **R8** CodeDOM only | raw-text read/search/edit removed from the model's tool surface with no regression | ⏳ not started |
| **R9** Disciplined specs | ideas become a resumable Mangle DAG of requirements → decisions → work items with witnesses, and codeNERD builds a feature from it | ⏳ not started |

Rungs are climbed in order. A later rung's probe can run early, but it doesn't count until the
rungs below it pass.

### Unattended hardening

[`Docs/journeys/06-unattended-hardening.md`](Docs/journeys/06-unattended-hardening.md) tracks
the "weeks unattended" half of the North Star. Run-level wall clocks are gone (H1). Still open: an
inventory and ratchet test for each class of hardcoded limit, arbitrary truncation and logging
blind spot, with limit hits asserted as facts the kernel can derive repair obligations from.

### SWE-bench

There is a SWE-bench bridge (`nerd swebench setup` routes an instance through the kernel and asserts
its expectation facts). **No SWE-bench score is claimed.** 100% is where the system is aimed, not a
number anyone has measured.

### Known gaps

Partially wired subsystems and dormant integration points are expected in this codebase, and they
are tracked rather than hidden:

- [`Docs/journeys/`](Docs/journeys/): the code-grounded journey map, the forcing and completion
  audit, the external audit queue, and the seams where the kernel is still bypassed.
- [`Docs/architecture/UNFINISHED-FEATURES.md`](Docs/architecture/UNFINISHED-FEATURES.md): the
  per-subsystem backlog of wiring gaps, by priority.
- [`AUDIT.md`](AUDIT.md): package-by-package correctness audit.

---

## It builds itself

codeNERD is developed by pointing codeNERD at codeNERD. Each ladder run is a real brief against
this repository. When a run fails because of the harness (a tool that refuses a correct edit,
context that never reached the model, a verdict that lies), **that failure is the finding**. It
gets fixed test-first, and the same brief runs again.

A few defects that loop has surfaced:

- The line-range edit tools never reported the shift they caused, so a second edit landed at
  stale coordinates and silently duplicated declarations.
- `CloneForTask` copied three fields and forgot two, quietly stripping `nerd.md` rules and
  holographic context from *every* delegated task.
- The test gate ran only the packages a turn wrote, so a change that broke an importer reported
  `tests ok`.
- A repair round's prompt told the model "the tests pass" above a FAIL trace, and the model
  weakened its own assertion to escape.
- A campaign fallback wrote files outside the turn's obligations. The permission check never
  matched its payload, which is the only reason the bypass stayed latent.

Dogfooding isn't a slogan here. It's the test harness.

---

## Sixty seconds

**Prerequisites:** Go 1.26+ (for source builds) and an API key for any supported provider.

```bash
# 1. build (see "Building" below for the sqlite-vec variant)
go build -o nerd ./cmd/nerd

# 2. point it at a model
export ANTHROPIC_API_KEY="…"      # or OPENAI_ / GEMINI_ / XAI_ / ZAI_ / DASHSCOPE_
                                  #    META_ / MOONSHOT_ / OPENROUTER_ …_API_KEY

# 3. wake it up
./nerd init      # scan the codebase, build the fact base, write .nerd/
./nerd           # interactive TUI
```

Supported providers: Anthropic, OpenAI, Gemini, xAI, Z.AI, OpenRouter, DashScope, Meta, Moonshot
and local Ollama. The `claude-cli` and `codex-cli` engines drive a subscription CLI instead.

**Run models in tiers.** Reasoning-heavy verbs escape to an expensive model and everything else stays
cheap. Which verbs qualify is policy, not code:

```jsonc
// .nerd/config.json
{
  "provider": "anthropic",
  "model":    "<frontier model>",                              // interactive turns
  "worker":   { "provider": "ollama", "model": "qwen3:8b" },   // bulk: shards, delegated tasks
  "planner":  { "provider": "anthropic", "model": "<frontier model>" } // /review /audit /campaign
}
```

`nerd config check` validates the file and refuses contradictions. `nerd config full` writes out
every configurable field, so nothing is silently defaulted.

---

## The surface

<table>
<tr><th align="left">Core</th><th align="left">What it does</th></tr>
<tr><td><code>nerd</code></td><td>Interactive TUI with the Glass Box pane</td></tr>
<tr><td><code>nerd run "…"</code></td><td>One full OODA loop, headless</td></tr>
<tr><td><code>nerd init</code> · <code>nerd scan</code></td><td>Build <code>.nerd/</code> · refresh the index without a full reinit</td></tr>
<tr><td><code>nerd status</code></td><td>System state and loaded facts</td></tr>
<tr><td><code>nerd config check</code></td><td>Validate the configuration and list implicit fields</td></tr>
</table>

<table>
<tr><th align="left">Interrogate</th><th align="left">What it does</th></tr>
<tr><td><code>nerd query &lt;pred&gt;</code></td><td>Query derived facts straight from the kernel</td></tr>
<tr><td><code>nerd why [pred]</code></td><td>Print the derivation chain</td></tr>
<tr><td><code>nerd explain &lt;target&gt;</code></td><td>Explain code with full holographic context</td></tr>
<tr><td><code>nerd check-mangle &lt;files&gt;</code></td><td>Validate <code>.mg</code> syntax and stratification</td></tr>
<tr><td><code>nerd dream</code> · <code>nerd shadow</code> · <code>nerd whatif</code></td><td>Simulate an action's effects before anything real happens</td></tr>
</table>

<table>
<tr><th align="left">Work</th><th align="left">What it does</th></tr>
<tr><td><code>nerd fix "&lt;symptom&gt;"</code></td><td>One gated turn: fix, pin with tests, prove. <code>--acceptance</code> takes a failing-test contract</td></tr>
<tr><td><code>nerd review &lt;target&gt;</code></td><td>Review, routed to the reasoning tier</td></tr>
<tr><td><code>nerd spawn &lt;persona&gt; "…"</code></td><td>Delegate to a specialist in an isolated context</td></tr>
<tr><td><code>nerd campaign start "…"</code></td><td>Multi-phase campaign with verified checkpoints (<code>--type</code>, <code>--docs</code>, <code>--accept</code>)</td></tr>
<tr><td><code>nerd campaign recurse</code></td><td>Waves of campaigns over the subsystem DAG</td></tr>
<tr><td><code>nerd regression run</code></td><td>Run the regression battery</td></tr>
</table>

---

## Under the hood

| Layer | Tech |
|---|---|
| Logic kernel | [Mangle](https://github.com/google/mangle): deductive database language with stratified negation, aggregation and recursion |
| CLI / TUI | [Cobra](https://github.com/spf13/cobra) · [Bubble Tea](https://github.com/charmbracelet/bubbletea) |
| Code structure | [Tree-sitter](https://github.com/smacker/go-tree-sitter) + Go AST → CodeDOM |
| Persistence | [SQLite](https://gitlab.com/cznic/sqlite) (modernc) + optional `sqlite-vec` for embeddings |
| Browser | [Rod](https://github.com/go-rod/rod) (CDP) |
| Sandbox | Docker, auto-provisioned |
| Logging | [Zap](https://github.com/uber-go/zap), structured |

```
codenerd/
├── cmd/nerd/            CLI + Bubble Tea TUI
└── internal/
    ├── core/            kernel, VirtualStore, Dreamer, the policy corpus (defaults/)
    ├── mangle/          the Mangle engine binding, parse lock, differential engine
    ├── session/         the execution loop, forcing gates, spawner
    ├── prompt/          JIT compiler + the atom library
    ├── perception/      language ▸ atoms, multi-provider clients
    ├── articulation/    atoms ▸ language, Piggyback protocol
    ├── tools/           tool registry, CodeDOM, shell, research
    ├── world/           world model, holographic context
    ├── context/         spreading activation, compression
    ├── campaign/        multi-phase orchestration, assaults, recurse
    ├── verification/    the verification gate
    ├── autopoiesis/     Ouroboros, prompt evolution
    ├── northstar/       the vision Guardian
    ├── projectdoc/      nerd.md parsing and enforcement
    ├── store/           multi-tier durable memory
    └── system/          boots everything into one Cortex
```

---

## Going deeper

| | |
|---|---|
| [`agents.md`](agents.md) | The repo contract, the vision and the working map. Read this before any architectural change |
| [`Docs/journeys/`](Docs/journeys/) | Code-grounded journey maps, the ladder, the hardening program, audits |
| [`internal/mangle/agents.md`](internal/mangle/agents.md) | Read before editing any `.mg` file |
| [`internal/prompt/agents.md`](internal/prompt/agents.md) | Read before changing prompt behaviour |
| [`internal/core/agents.md`](internal/core/agents.md) | Kernel and execution internals |
| [`Docs/architecture/`](Docs/architecture/) | Per-subsystem corpora. The July 2026 generation is orientation only and not authoritative; it is being rewritten from the code under ladder rung R6 |

### Building

```bash
go build -o nerd ./cmd/nerd
go test ./...
```

For vector search, build with `sqlite-vec`:

```bash
CGO_CFLAGS="-I$(pwd)/sqlite_headers" go build -tags sqlite_vec -o nerd ./cmd/nerd
```

`-tags sqlite_vec` selects `internal/store/vec_support_enabled.go`, which makes the store refuse to
open rather than silently degrade. The tag alone does **not** provide the extension: `vec0` has to be
compiled into the modernc SQLite driver (see `internal/store/local_core.go`). Without it, ANN search
falls back to lexical matching and each boot logs
`sqlite-vec not available; falling back from ANN to lexical search`. Because the prompt selector is
part logic and part vector, that fallback quietly turns the vector share into keyword matching.

### Writing rules

Variables are `UPPERCASE`. Constants are `/lowercase`. Every predicate needs a `Decl`. Negation only
applies to variables a positive atom has already bound. Aggregation uses the `|> do … let …`
pipeline:

```mangle
next_action(/generate_code) :-
    user_intent(ID, /mutation, /generate, Target, _),
    !block_action(ID, _).
```

Don't use Mangle for fuzzy matching. Retrieve with embeddings first, then assert structured facts.

---

<div align="center">

### The wager

**A model that can't remember its constraints will eventually violate them.**
**A kernel that can't be creative will never surprise you.**

Give each one the job it's actually good at. Then keep measuring the distance to the North Star.

<br>

[![GitHub](https://img.shields.io/badge/theRebelliousNerd%2Fcodenerd-181717?style=for-the-badge&logo=github)](https://github.com/theRebelliousNerd/codenerd)

</div>

## License

Copyright (C) 2026 theRebelliousNerd

Licensed under the **GNU Affero General Public License v3.0**. See [LICENSE](LICENSE).

If you fork codeNERD, or run a modified version as a network service, the AGPL requires you to
publish your source and keep the attribution. Section 13 is the part most licenses lack: offering a
modified version over a network counts as distribution, so a hosted derivative owes its source to
its users too.
