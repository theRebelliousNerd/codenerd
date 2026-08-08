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
[![Kernel](https://img.shields.io/badge/Kernel-Mangle_Datalog-4285F4?style=for-the-badge&logo=google&logoColor=white)](https://github.com/google/mangle)
[![Architecture](https://img.shields.io/badge/Neuro--Symbolic-8B5CF6?style=for-the-badge)]()
[![License](https://img.shields.io/badge/MIT-22C55E?style=for-the-badge)](LICENSE)

**A coding agent that cannot take an action it can't prove it's allowed to take.**

*Built to run unattended for weeks — and to solve 100% of SWE-bench.*

[Why](#the-category-error) · [How](#the-inversion) · [North Star](#the-north-star) · [Install](#sixty-seconds) · [Commands](#the-surface) · [Safety](#the-constitution) · [Docs](#going-deeper)

</div>

---

## The category error

Every coding agent you've used asks one model to do two incompatible jobs.

It has to be **creative** — read a gnarly stack trace, intuit the bug, invent a fix nobody wrote down. And it has to be an **executive** — remember what it decided forty turns ago, refuse the destructive command, keep the plan coherent across an hour of work.

Language models are extraordinary at the first job. They are structurally unsuited to the second. Attention is not a ledger. A context window is not a memory. "Please don't delete anything important" is not an access-control policy.

So agents drift. They forget constraints they agreed to. They confidently do the thing you told them never to do. And the fix is always the same sad loop — a longer prompt, a sterner tone, a bigger window — because the *architecture* never changed.

codeNERD changes the architecture.

---

## The inversion

> The LLM is the **creative center**. A Datalog kernel is the **executive**.

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

The model never *decides*. It perceives and it articulates. In between, natural language is transduced into logical atoms, and a Mangle (Datalog) kernel derives what happens next from rules you can read, test, and version.

| | Conventional agent | codeNERD |
|---|---|---|
| **Who decides** | The model, per token | Datalog rules, per fact |
| **Memory** | Context window (evaporates) | Fact store (persists) |
| **Safety** | Instructions in a prompt | `permitted/1` derivation, default-deny |
| **Planning** | Vibes, re-derived each turn | Stratified rules over a fact base |
| **Auditability** | "Explain your reasoning" (it confabulates) | `nerd why` — the actual derivation chain |
| **Failure mode** | Confident nonsense | Refusal with a citation |

**The load-bearing consequence:** an action that cannot derive `permitted(...)` does not execute. Not "is discouraged from executing." Does not execute. Safety is a property of the evaluator, not a request to the model.

---

## The loop

```
   ┌────────────────────────────────────────────────────────────────┐
   │  you: "refactor the auth middleware to use the new token type" │
   └───────────────────────────────┬────────────────────────────────┘
                                   │
              ╔════════════════════▼════════════════════╗
   OBSERVE    ║  PERCEPTION — language ▸ logical atoms   ║
              ║  user_intent(id, /mutation, /refactor…) ║
              ╚════════════════════╤════════════════════╝
                                   │
        ╔══════════════════════════▼══════════════════════════╗
        ║                    MANGLE KERNEL                    ║
        ║   ┌───────────┐   ┌────────────┐   ┌────────────┐   ║
ORIENT  ║   │   FACTS   │◀─▶│   RULES    │──▶│  POLICY    │   ║  DECIDE
        ║   │  (memory) │   │ (planning) │   │(default    │   ║
        ║   └───────────┘   └────────────┘   │  deny)     │   ║
        ║         ▲                          └─────┬──────┘   ║
        ╚═════════╪════════════════════════════════╪══════════╝
                  │                                │
                  │                    next_action(/refactor)
                  │                                │
              ╔═══╧════════════════════════════════▼════════╗
   ACT        ║  VIRTUAL STORE — the only door to the world  ║
              ║  CodeDOM · shell · fs · git · browser · MCP  ║
              ╚════════════════════╤════════════════════════╝
                                   │
              ╔════════════════════▼════════════════════╗
              ║  ARTICULATION — atoms ▸ language        ║
              ╚═════════════════════════════════════════╝
```

Every turn is **Observe → Orient → Decide → Act**. Facts flow in; a plan is *derived*, not improvised; and the only path to the filesystem runs through a gate that checks the constitution first.

---

## What that buys you

### 🧬 Prompts are compiled, not concatenated

Prompts aren't strings glued together. They're **atoms** — versioned YAML units with priorities, dependencies, and shard gating — selected per turn by a JIT compiler against a token budget.

```
internal/prompt/atoms/
├── protocol/     the control-stream contract
├── capability/   how to actually use each tool
├── exemplar/     worked examples, gated per persona
└── campaign/     multi-phase orchestration behavior
```

Change behavior by editing an atom, not by hunting for a hardcoded string in a shard. Ask `/jit` and it shows you exactly which atoms were selected and why.

### 🔬 Surgical edits, not string roulette

Text-matching edits break on whitespace. codeNERD reads code as **structure**:

```bash
get_elements  path=internal/auth/user.go        # symbol index + real line ranges
edit_lines    path=… start_line=42 end_line=55  # replace a precise extent
```

Every mutation reports the shift it caused — *"File is now 377 lines (−11). Line numbers at or after 42 are now STALE."* — so the next edit can't land at coordinates the previous one invalidated.

### 🌐 Holographic context

Ask about one file and the kernel hands the model that file's **neighborhood**: exported signatures, type definitions, who calls it, its architectural role, and its test coverage — assembled from the AST and the fact store, not guessed from a grep.

### 📜 `nerd.md` — instructions with teeth

Like `CLAUDE.md` or `AGENTS.md`, but the machine-readable half is **enforced, not suggested**. Strict YAML frontmatter becomes kernel facts:

```yaml
---
forbid:
  - match: .nerd/config.json
    reason: user-owned runtime config
---
```

That doesn't become a polite sentence in a system prompt. It becomes `project_forbidden_path/2` in the fact store, and the write is denied *before the tool runs* — on the interactive path and the shard path alike.

### 🎭 Specialists, spawned on demand

Coder, reviewer, tester, researcher, and any specialist you define. Each gets its own JIT-compiled identity, its own tool allowlist, and its own isolated context — so a delegated task can't contaminate your session's history.

### ♻️ Ouroboros — it writes its own tools

Hit a capability gap and codeNERD can generate, compile, and register a new tool into its own registry. The agent's surface is not fixed at build time.

### 🔭 Glass box, all the way down

```bash
nerd why permitted    # the derivation chain, fact by fact
nerd query <pred>     # interrogate the kernel directly
```

No "let me explain my reasoning" theater. The reasoning *is* the artifact.

---

## Sixty seconds

**Prerequisites:** Go 1.26+ (source builds only) and an API key for any supported provider.

```bash
# 1. get a binary (or build: go build -o nerd ./cmd/nerd)
#    drop it in your project root

# 2. point it at a model
export ANTHROPIC_API_KEY="…"      # or OPENAI_ / GEMINI_ / XAI_ / ZAI_
                                  #    DASHSCOPE_ / META_ / MOONSHOT_ / OPENROUTER_

# 3. wake it up
./nerd init      # scans the codebase, builds the fact base, writes .nerd/
./nerd           # interactive TUI
```

Ten providers are supported — Anthropic, OpenAI, Gemini, xAI, Z.AI, OpenRouter, DashScope, Meta, Moonshot, and local Ollama — plus `claude-cli` and `codex-cli` engines if you'd rather drive a subscription CLI.

**Run models in tiers.** Reasoning-heavy verbs escape to an expensive model; everything else stays cheap. Which verbs qualify is *policy*, not a Go switch — it lives in `delegation.mg` as `intent_requires_reasoning_model/1`:

```jsonc
{
  "provider": "anthropic",
  "model": "claude-opus-5",                                    // interactive turns
  "worker":  { "provider": "ollama", "model": "qwen3:8b" },    // bulk: shards, delegated tasks
  "planner": { "provider": "anthropic", "model": "claude-opus-5" } // /review /audit /campaign
}
```

---

## The surface

<table>
<tr><th align="left">Core</th><th align="left">What it does</th></tr>
<tr><td><code>nerd</code></td><td>Interactive TUI with the Glass Box pane</td></tr>
<tr><td><code>nerd run "…"</code></td><td>One full OODA loop, headless</td></tr>
<tr><td><code>nerd init</code></td><td>Scan the codebase, build <code>.nerd/</code></td></tr>
<tr><td><code>nerd scan</code></td><td>Refresh the index without a full reinit</td></tr>
<tr><td><code>nerd status</code></td><td>System state and loaded facts</td></tr>
</table>

<table>
<tr><th align="left">Interrogate</th><th align="left">What it does</th></tr>
<tr><td><code>nerd query &lt;pred&gt;</code></td><td>Query derived facts straight from the kernel</td></tr>
<tr><td><code>nerd why [pred]</code></td><td>Print the derivation chain</td></tr>
<tr><td><code>nerd explain &lt;file&gt;</code></td><td>Explain a file with full holographic context</td></tr>
<tr><td><code>nerd check-mangle &lt;files&gt;</code></td><td>Validate <code>.mg</code> syntax and stratification</td></tr>
</table>

<table>
<tr><th align="left">Work</th><th align="left">What it does</th></tr>
<tr><td><code>nerd review &lt;path&gt;</code></td><td>Review — routed to the reasoning tier</td></tr>
<tr><td><code>nerd fix &lt;file&gt; "…"</code></td><td>Targeted mutation through the write gate</td></tr>
<tr><td><code>nerd spawn &lt;persona&gt; "…"</code></td><td>Delegate to a specialist in isolated context</td></tr>
<tr><td><code>nerd campaign start "…"</code></td><td>Multi-phase campaign with verified checkpoints</td></tr>
<tr><td><code>nerd campaign start --docs ./specs/</code></td><td>Build straight from specification documents</td></tr>
</table>

Campaigns decompose a goal into phases and tasks, run them with dependency ordering, and **verify each checkpoint before advancing** — a phase that claims success without evidence fails.

In chat mode, `/campaign assault internal/core` turns the whole thing adversarial: a Nemesis persona hunts for panics, races, and edge cases in a module until it stops finding them.

---

## The constitution

Safety isn't a paragraph in a prompt. It's a derivation that has to succeed.

```mangle
# Default deny. If you can't derive it, it doesn't run.
permitted(Action) :- safe_action(Action).

permitted(Action) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action).

dangerous_action(Action) :-
    action_type(Action, /exec_cmd),
    cmd_string(Action, Cmd),
    fn:string_contains(Cmd, "rm -rf").
```

Layered on top of that:

- **Commit barrier** — a broken build blocks the commit. `block_commit(R) :- diagnostic(/error, …)`
- **Shadow mode** — `nerd run --shadow "…"` projects the effects without touching anything
- **Dreamer** — destructive actions get simulated before they're real
- **Workspace jail** — every path is resolved against the workspace root; escapes are refused
- **Two gates, one definition** — the interactive path and the VirtualStore path share the *same* matcher, so a shard can never write what the session refused

---

## The north star

Two targets. Neither is reached yet. Both are stated here because they're what every design decision in this repo is being measured against.

### ⏳ Run unattended for weeks — and be trustworthy the whole time

Not "survive a long session." *Weeks.* Which is a fundamentally different engineering problem, because the thing that kills long autonomy isn't crashing — it's **silent drift**. An agent that reports success while quietly dropping work doesn't fail loudly at hour 200; it fails at hour 3 and spends the next 197 building on a lie.

Real example, from this repo, this month: a merge routine reported 77 branches successfully merged. Every command it ran returned zero. It had silently dropped five performance optimizations, four refactors, and dozens of test declarations — because it verified **process** ("did the command succeed") instead of **outcome** ("is the content actually there"). An hour of work is recoverable. Weeks are not.

That's the entire argument for the kernel. A fact store records what is *true*, not what was *attempted*. Campaign checkpoints must produce evidence, not a return code — an early build of that system counted "verifier was nil, so nothing failed" as a pass, and every phase reported verified having verified nothing. That bug is the north star's exact antagonist.

> **The invariant:** the agent's own status report must be worthless as a claim and valuable only as a citation. If it says a thing is done, there is a derivation showing it.

### 🎯 100% on SWE-bench

Not "competitive." All of it.

The same discipline scaled to a single task: a patch that compiles is not a patch that fixes the issue, and an agent that cannot tell those apart has a ceiling well under 100%. Getting there means never confusing *plausible* with *verified* — which is why the model proposes and the kernel disposes.

---

## It builds itself

codeNERD is developed by pointing codeNERD at codeNERD.

Its own architecture documentation under `Docs/architecture/` was written by the agent reading its own source. When a wiring gap or a dead subsystem is found, the fix is handed *back* to the agent — and the failures that produces are the real bug reports. A few that came out of exactly that loop:

- The line-range edit tools never reported the line shift they caused, so a second edit landed at stale coordinates and silently duplicated declarations. Now every mutation states the delta and invalidates the old index.
- `CloneForTask` copied three fields and forgot two, quietly stripping `nerd.md` rules and holographic context from *every* delegated task — invisible in interactive testing.
- Output contracts were inferred by grepping the prompt for a marker, so a prompt that *forbade* an envelope got the envelope schema attached.

Dogfooding isn't a slogan here. It's the test harness.

---

## Under the hood

| Layer | Tech |
|---|---|
| Logic kernel | [Mangle](https://github.com/google/mangle) — Datalog with stratified negation & aggregation |
| CLI / TUI | [Cobra](https://github.com/spf13/cobra) · [Bubble Tea](https://github.com/charmbracelet/bubbletea) |
| Code structure | [Tree-sitter](https://github.com/smacker/go-tree-sitter) + Go AST |
| Persistence | [SQLite](https://github.com/modernc/sqlite) + `sqlite-vec` for embeddings |
| Browser | [Rod](https://github.com/go-rod/rod) (CDP) |
| Sandbox | Docker, auto-provisioned |
| Logging | [Zap](https://github.com/uber-go/zap), structured |

```
codenerd/
├── cmd/nerd/            CLI + Bubble Tea TUI
└── internal/
    ├── core/            kernel, VirtualStore, policy corpus
    ├── session/         the clean execution loop, spawner, subagents
    ├── prompt/          JIT compiler + the atom library
    ├── perception/      language ▸ atoms, multi-provider clients
    ├── articulation/    atoms ▸ language, Piggyback protocol
    ├── mangle/          the Datalog engine binding
    ├── world/           holographic context, codebase model
    ├── campaign/        multi-phase orchestration
    ├── autopoiesis/     Ouroboros, prompt evolution
    └── projectdoc/      nerd.md parsing and enforcement
```

---

## Going deeper

| | |
|---|---|
| [`CLAUDE.md`](CLAUDE.md) | Repo contract and working map |
| [`Docs/architecture/`](Docs/architecture/) | Subsystem specifications |
| [`internal/mangle/agents.md`](internal/mangle/agents.md) | Read this before editing any `.mg` file |
| [`internal/prompt/agents.md`](internal/prompt/agents.md) | Read this before changing prompt behavior |
| [`internal/core/agents.md`](internal/core/agents.md) | Kernel and execution internals |

**Building:**

```bash
CGO_CFLAGS="-I$(pwd)/sqlite_headers" go build -o nerd ./cmd/nerd
go test ./...
```

**Writing rules** — variables are `UPPERCASE`, constants are `/lowercase`, every predicate needs a `Decl`, and negation only binds over variables a positive atom already bound:

```mangle
next_action(/generate_code) :-
    user_intent(ID, /mutation, /generate, Target, _),
    !block_action(ID, _).
```

---

<div align="center">

### The wager

**A model that can't remember its constraints will eventually violate them.**
**A kernel that can't be creative will never surprise you.**

Give each one the job it's actually good at.

<br>

[![GitHub](https://img.shields.io/badge/theRebelliousNerd%2Fcodenerd-181717?style=for-the-badge&logo=github)](https://github.com/theRebelliousNerd/codenerd)

</div>
