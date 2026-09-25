<div align="center">

```
 ▄████████  ▄██████▄  ████████▄     ▄████████ ███▄▄▄▄      ▄████████    ▄████████ ████████▄
███    ███ ███    ███ ███   ▀███   ███    ███ ███▀▀▀██▄   ███    ███   ███    ███ ███   ▀███
███    █▀  ███    ███ ███    ███   ███    █▀  ███   ███   ███    █▀    ███    ███ ███    ███
███        ███    ███ ███    ███  ▄███▄▄▄     ███   ███  ▄███▄▄▄      ▄███▄▄▄▄██▀ ███    ███
███        ███    ███ ███    ███ ▀▀███▀▀▀     ███   ███ ▀▀███▀▀▀     ▀▀███▀▀▀▀▀   ███    ███
███    █▄  ███    ███ ███    ███   ███    █▄  ███   ███   ███    █▄  ▀███████████ ███    ███
███    ███ ███    ███ ███   ▄███   ███    ███ ███   ███   ███    ███   ███    ███ ███   ▄███
████████▀   ▀██████▀  ████████▀    ██████████  ▀█   █▀    ██████████   ███    ███ ████████▀
                                                                       ███    ███
            ─────────────  the model imagines  ·  the kernel decides  ─────────────
```

## Logic determines reality. The model merely describes it.

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Kernel](https://img.shields.io/badge/Executive-Google_Mangle-4285F4?style=for-the-badge&logo=google&logoColor=white)](https://github.com/google/mangle)
[![Paradigm](https://img.shields.io/badge/Neuro--Symbolic-8B5CF6?style=for-the-badge)](#part-ii--the-inversion)
[![Status](https://img.shields.io/badge/Status-Experimental-F59E0B?style=for-the-badge)](#part-vii--where-it-stands)
[![License](https://img.shields.io/badge/AGPL--3.0-22C55E?style=for-the-badge)](LICENSE)

**codeNERD is a coding agent whose executive is a deductive database, not a language model.**<br>
The LLM does the creative work. A kernel of logic rules decides what happens, remembers what is true,<br>
refuses what isn't permitted, and won't accept "done" without evidence.

[North Star](#-the-north-star) ·
[Why](#part-i--the-category-error) ·
[The Idea](#part-ii--the-inversion) ·
[Creed](#part-iii--the-creed) ·
[Anatomy](#part-iv--anatomy) ·
[A Turn, Start to Finish](#part-v--one-turn-from-keystroke-to-verdict) ·
[Techniques](#part-vi--the-techniques) ·
[Where It Stands](#part-vii--where-it-stands) ·
[Get Started](#part-ix--get-started)

</div>

---

> [!IMPORTANT]
> **codeNERD is an experimental research harness, built in the open.** Parts of it work well, parts
> are half-wired, and much of the ambition below has not been demonstrated yet. The
> [North Star](#-the-north-star) describes where the system is headed, not what it does today.
> [Where It Stands](#part-vii--where-it-stands) shows today's state, backed by evidence.

---

## ✦ The North Star

```
                                     *
                                    /|\
                       ────────── *──◆──* ──────────     T H E   N O R T H   S T A R
                                    \|/                  a final state, never "reached"
                                     *
                                     ┊
                                     ┊   the distance between what the evidence shows
                                     ┊   and how the finished system behaves
                                     ┊        is derived on every turn,
                                     ┊        and applied as pressure on every agent.
                                     ┊
                                     ┊   ▲  every turn moves toward it
                                     ┊   │  the pressure never lets go
                                     ◉
                                   today
```

A north star is **a final state, not a goal to hit**. It describes how the *finished* system
behaves. Nothing "achieves" it. The harness keeps measuring how far the evidence is from that state,
and that distance becomes pressure on every agent it runs.

When codeNERD is finished, this is what it does:

| | The finished system |
|:--|:--|
| ⏳ **Runs unattended for weeks** | It stays trustworthy the whole time. Nothing stops work that is still making progress; what does stop it is *derived from evidence*: a stall, a repeated failure, a cancel. Logs, tables and caches stay bounded. A failure becomes a repair obligation, not a log line someone has to grep for. |
| 🧾 **Every claim is a citation** | When it says a thing is done, a derivation shows it. Its own status report is worthless as a claim and valuable only as a citation. The fact store records what is *true*, not what was *attempted*. |
| 🎯 **Closes 100% of SWE-bench** | This is the target, not a result. It takes the same discipline, scaled down to one task: a patch that compiles is not a patch that fixes the issue, and a harness that can't tell those apart tops out well short of 100%. |
| 🧭 **Never greps around** | The kernel gives the model what it needs, when it needs it and for a stated reason: domain knowledge from SQLite, vector recall, CodeDOM structure and history. The job finishes in as few turns as possible. |
| 🔒 **The harness forces** | Tests are not optional. The agent keeps working until the behaviour the project's own north star describes actually holds, not until the model *feels* finished. |
| ♻️ **Improves itself** | It hunts its own bugs and extends its own tooling, in Mangle and in Go. Each self-improvement cycle leaves the gates strictly better and nothing worse. |

The rest of this document explains why that target makes sense, how the system is built to move
toward it, and how far there still is to go.

---

## Part I · The category error

```
                       ╭───────────────────────────────────────────╮
                       │      ONE MODEL  ·  TWO INCOMPATIBLE JOBS  │
                       ├─────────────────────┬─────────────────────┤
                       │      CREATIVE       │      EXECUTIVE      │
                       │                     │                     │
                       │  read the stack     │  remember what was  │
                       │  trace, guess the   │  decided forty      │
                       │  bug, invent a fix  │  turns ago; refuse  │
                       │  nobody wrote down  │  the rm -rf; keep   │
                       │                     │  the plan coherent  │
                       │                     │                     │
                       │   LLMs: superb      │   LLMs: attention   │
                       │                     │   is not a ledger   │
                       ╰─────────────────────┴─────────────────────╯
```

Every mainstream coding agent asks one model to do both of these jobs.

Language models are extraordinary at the first job and structurally unsuited to the second. A
context window is not a memory. "Please don't delete anything important" is not an access-control
policy. A plan re-derived from scratch every turn is not a plan.

So agents drift. They forget constraints they agreed to. They confidently do the thing you told
them never to do. They report success over work they quietly dropped. The usual remedy is always the
same loop: a longer prompt, a sterner tone, a bigger window. None of that changes the architecture.

**codeNERD changes the architecture.**

---

## Part II · The inversion

```
    ╔═════════════════════════════╗                   ╔═════════════════════════════╗
    ║      THE CREATIVE CENTER    ║                   ║        THE EXECUTIVE        ║
    ║           the LLM           ║                   ║      the Mangle kernel      ║
    ╟─────────────────────────────╢                   ╟─────────────────────────────╢
    ║   understands   invents     ║  ════ atoms ════▶ ║   decides      remembers    ║
    ║   synthesizes   explains    ║                   ║   forbids      proves       ║
    ║                             ║  ◀═══ plans ════  ║                             ║
    ║   ~ probabilistic ~         ║                   ║   = deterministic =         ║
    ╚══════════════╤══════════════╝                   ╚══════════════╤══════════════╝
                   │                                                 │
          ┌────────┴────────────────── TRANSDUCERS ──────────────────┴────────┐
          │   perception    language ▸ atoms    requests become facts         │
          │   articulation  atoms ▸ language    conclusions become words      │
          └───────────────────────────────────────────────────────────────────┘
```

The model never *decides*. It perceives and it articulates. Between the two, natural language and
code are **transduced** into logical atoms. [Google Mangle](https://github.com/google/mangle), a
deductive database programming language with recursion, stratified negation, typed declarations and
aggregation pipelines, then derives what happens next from rules you can read, test, diff and
version.

| | A conventional agent | codeNERD |
|:--|:--|:--|
| **Who decides** | The model, token by token | Rules, fact by fact |
| **Memory** | The context window, which evaporates | A fact store, which persists |
| **Safety** | Instructions in a prompt | A `permitted/3` derivation, deny by default |
| **Planning** | Improvised again every turn | Stratified rules over a fact base |
| **Context** | Whatever the model decided to grep | What the kernel derived the model needs |
| **"Done"** | The model says so | The gates' evidence says so |
| **Audit** | "Explain your reasoning" (and it confabulates) | `nerd why`: the actual proof tree |
| **Failure mode** | Confident nonsense | A refusal that cites its rule |

**The consequence everything else depends on:** an action that cannot derive `permitted(...)` does
not execute. It isn't "discouraged". It does not run. Safety is a property of the evaluator, not a
request to the model.

---

## Part III · The creed

These are the rules every architectural change is judged by. The canonical text lives in
[`agents.md`](agents.md).

<table>
<tr><td width="50"><h3>Ⅰ</h3></td><td><b>Clean fixpoint, not clean loop.</b> Go can be as big as the work needs: it is the FFI, the drivers, the tools. But the <i>executive decisions</i> (what a turn is, whether it's done, what gets delegated with which task, what enters the window, what the verdict is) are the kernel's fixpoint over the facts. The drift to hunt for is <b>a decision computed in Go instead of derived</b>.</td></tr>
<tr><td><h3>Ⅱ</h3></td><td><b>The harness decides, not the model's discretion.</b> Mangle plus JIT prompt compilation replaces free-form subagents and skills. Domain knowledge is <i>pushed</i> into the window when the harness decides it's needed, never left to the model to fetch.</td></tr>
<tr><td><h3>Ⅲ</h3></td><td><b>The harness forces.</b> A turn that wrote Go owes the tests that pin it. A turn that deleted tests owes them back. A verdict its evidence contradicts is not a verdict. Obligations are derived and forced, not suggested.</td></tr>
<tr><td><h3>Ⅳ</h3></td><td><b>Verify outcomes, not process.</b> "Exit 0" is not evidence. "The content is there, and the test fails without it" is. A merge routine here once reported 77 branches merged. Every command returned zero, and it had silently dropped a dozen optimizations. That bug is the North Star's exact antagonist.</td></tr>
<tr><td><h3>Ⅴ</h3></td><td><b>Where a fact sits in the window is a decision.</b> Lost-in-the-middle is fought with compression, pruning and ordering, not with bigger windows. Quality comes first. Tokens are the constraint, not the goal.</td></tr>
<tr><td><h3>Ⅵ</h3></td><td><b>Tools exist for exactly three things:</b> to condense the search space, to reduce the number of turns, and to offload cognition to deterministic code. A change with a blast radius is carried out by a tool, not hand-edited by an LLM. Anything else is cruft.</td></tr>
<tr><td><h3>Ⅶ</h3></td><td><b>Progress, not arbitrary cutoffs.</b> No run-level wall clock and no tool-call quota stands in for "done". A limit you set is an opt-in constraint. What stops a run is a <i>derived</i> stall or a repeated failure.</td></tr>
<tr><td><h3>Ⅷ</h3></td><td><b>Complexity is not a defect.</b> What gets scored is whether a decision is <i>derived</i> and whether an obligation is <i>forced</i>.</td></tr>
</table>

---

## Part IV · Anatomy

```
 ┌───────────────────────────────────────────────────────────────────────────────────────┐
 │  YOU    nerd (TUI)  ·  nerd run  ·  nerd fix  ·  nerd campaign  ·  nerd.md  ·  config │
 └──────────────────────────────────────────┬────────────────────────────────────────────┘
                                            ▼
 ┌──────────────────────── SESSION · the turn loop  (internal/session) ──────────────────┐
 │   JIT compile ──▶ model proposes ──▶ permitted/3 ──▶ VirtualStore ──▶ forcing gates   │
 └───────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────┘
         ▼                     ▼                     ▼                     ▼
 ┌───────────────┐     ┌───────────────┐     ┌───────────────┐     ┌───────────────┐
 │  PERCEPTION   │     │ MANGLE KERNEL │     │ JIT COMPILER  │     │ ARTICULATION  │
 │ lang ▸ atoms  │     │ facts + ~125  │     │ 350+ YAML     │     │ atoms ▸ lang  │
 │ 10 providers  │     │ .mg rule files│     │ atom files    │     │ Piggyback     │
 └───────────────┘     └───────┬───────┘     └───────────────┘     └───────────────┘
                               │ reads & writes facts
         ┌─────────────────────┼─────────────────────┬─────────────────────┐
         ▼                     ▼                     ▼                     ▼
 ┌───────────────┐     ┌───────────────┐     ┌───────────────┐     ┌───────────────┐
 │  WORLD MODEL  │     │    CONTEXT    │     │    MEMORY     │     │ VIRTUALSTORE  │
 │ AST · CodeDOM │     │  activation · │     │ SQLite tiers  │     │ the one door  │
 │ git · symbols │     │  compression  │     │ + sqlite-vec  │     │ to the world  │
 └───────────────┘     └───────────────┘     └───────────────┘     └───────┬───────┘
                                                                           ▼
                    filesystem · shell · git · CodeDOM tools · browser · MCP · Docker
```

| Organ | Lives in | Job |
|:--|:--|:--|
| **Session** | `internal/session/` | The turn loop: compile, propose, gate, execute, force, answer |
| **Perception** | `internal/perception/` | Language to `user_intent/5` atoms, plus multi-provider LLM clients |
| **Kernel** | `internal/core/`, `internal/mangle/` | The fact store, the rule corpus, the evaluator, the Dreamer |
| **Policy** | `internal/core/defaults/` | ~125 `.mg` files: schemas, constitution, routing, gates, campaigns |
| **JIT compiler** | `internal/prompt/` | Atoms to prompt: Mangle skeleton, vector flesh, token budget |
| **Articulation** | `internal/articulation/` | Atoms to language, and the Piggyback envelope |
| **World model** | `internal/world/`, `internal/tools/codedom/` | Code as structure: files, symbols, calls, coverage |
| **Context** | `internal/context/` | Spreading activation, compression, masking, ordering |
| **Memory** | `internal/store/` | Session, world, knowledge graph, traces, learnings, vectors, cold storage |
| **VirtualStore** | `internal/core/virtual_store*.go` | Every effect routes through here, and every result comes back as facts |
| **Campaigns** | `internal/campaign/` | Multi-phase work with checkpoints, leases, assaults, recurse |
| **Specialists** | `internal/shards/`, `internal/core/shards/` | Coder, tester, reviewer, researcher, and yours |
| **Autopoiesis** | `internal/autopoiesis/` | Ouroboros tool generation, prompt evolution |
| **Glass box** | `internal/transparency/` | Derivation traces, safety explanations, telemetry |

---

## Part V · One turn, from keystroke to verdict

This is what happens between typing a request and seeing a verdict. Every step is either a fact
or a rule.

```
  1 │ YOU            nerd fix "TestParseConfig fails: expected 3 providers, got 2"
    │
  2 │ PERCEIVE       user_intent("i1", /mutation, /fix, "TestParseConfig", "expected 3, got 2")
    │
  3 │ ORIENT         world facts light up:   file_topology(...)   code_defines(...)
    │                code_calls(...)   test_coverage(...)   →   activation scores
    │
  4 │ COMPILE        JIT: Mangle picks the skeleton atoms this shard + verb + language require,
    │                vectors add the most relevant flesh, everything is fit to the token budget
    │
  5 │ PROPOSE        the model answers with a Piggyback envelope:
    │                  surface_response  → words for you
    │                  control_packet    → tool_requests, mangle_updates, memory_operations
    │
  6 │ GATE           pending_action("a7", /edit_element, "config.go", <payload>, T)
    │                ⊢ permitted(/edit_element, "config.go", <payload>) ?   no proof → no action
    │
  7 │ ACT            VirtualStore executes → execution_result("a7", ..., /true, ...)
    │
  8 │ FORCE          turn_next_round(T, R) derives each round the turn still owes:
    │                /build → /test → /critic → /coverage → /pinned → /vet → /removed_tests → /test_run
    │                a failed round becomes a repair round, with the evidence in the prompt
    │
  9 │ VERDICT        /done only if the evidence shows it. Otherwise /unverified, and it says what's missing
    │
 10 │ ARTICULATE     the surface response reaches you; facts, traces and learnings persist
```

Step 8 is not a Go pipeline. It is a derivation (`internal/core/defaults/policy/turn_rounds.mg`):

```mangle
round_order(/build, 1).   round_order(/test, 2).     round_order(/critic, 3).
round_order(/coverage, 4). round_order(/pinned, 5).  round_order(/vet, 6).
round_order(/removed_tests, 7).                      round_order(/test_run, 8).

turn_round_owed(Turn, /pinned) :- turn_owes_gate(Turn, /pinned).
turn_round_owed(Turn, /vet)    :- turn_write_class(Turn, /go).

turn_round_pending(Turn, Round, Order) :-
    turn_round_owed(Turn, Round),
    round_order(Round, Order),
    !turn_round_ran(Turn, Round).

turn_next_round_order(Turn, Min) :-
    turn_round_pending(Turn, Round, Order)
    |> do fn:group_by(Turn), let Min = fn:min(Order).
```

The executor asks `turn_next_round`, runs the round it names, records `turn_round_ran`, and asks
again until nothing derives. Adding a round means writing a rule and a driver, not editing a
pipeline.

---

## Part VI · The techniques

This is how the creed turns into running code. Each technique names where it lives, so you can
check the claim against the source.

### 🛡️ 1 · The constitution: default deny

```
      proposed action
            │
            ▼
   pending_action(ID, Type, Target, Payload, T)
            │
            ▼
   ┌──────────────────────────────────┐
   │   can the kernel derive          │──── no ────▶  refused, citing the rule that failed
   │   permitted(Type, Target,        │
   │             Payload) ?           │
   └────────────────┬─────────────────┘
                    │ yes
                    ▼
     VirtualStore executes ──▶ execution_result(...) flows back in as a fact
```

```mangle
# internal/core/defaults/policy/constitution.mg — permitted must be positively derived.
permitted(Action, Target, Payload) :-
    safe_action(Action),
    pending_action(_, Action, Target, Payload, _),
    !dangerous_content(Action, Payload),
    !dangerous_content(Action, Target).

# The one delete allowed without sign-off: a file git can restore byte for byte.
permitted(/delete_file, Target, Payload) :-
    pending_action(_, /delete_file, Target, Payload, _),
    file_recoverable(Target),
    !dangerous_content(/delete_file, Payload),
    !dangerous_content(/delete_file, Target).
```

These layers sit on top:

- **The one door.** Every effect goes through the VirtualStore, is checked against the *exact*
  payload, and comes back as facts.
- **The commit barrier.** `block_commit("Build Broken")` and `block_commit("Tests Failing")`
  (`commit_gate.mg`). A red tree does not commit.
- **The workspace jail.** Paths are resolved against the workspace root, and escapes are refused.
- **Look before you leap.** `nerd dream`, `nerd shadow` and `nerd whatif` project an action's
  effects before anything real happens.
- **One definition, two gates.** The interactive path and the shard path share one matcher, so a
  delegated specialist can't write what the session refused.

### 🔁 2 · Transduction and the Piggyback protocol

Perception (`internal/perception/`) turns a request into `user_intent(ID, Category, Verb, Target,
Constraint)`. On the way out, the model's reply is an **envelope** with two channels that never mix:

```
  ┌──────────────────────────── the model's reply ────────────────────────────┐
  │  surface_response   "I found it: the loader skipped the third provider…"  │  ──▶ you
  ├───────────────────────────────────────────────────────────────────────────┤
  │  control_packet     intent_classification · tool_requests                 │  ──▶ the kernel
  │                     mangle_updates · memory_operations · self_correction  │
  └───────────────────────────────────────────────────────────────────────────┘
```

The model talks to you and to the kernel in the same breath, and the kernel only takes what's typed.

### ⚙️ 3 · JIT prompt compilation

Prompts are **compiled**, not concatenated. The library holds 350+ YAML files of *atoms* under
`internal/prompt/atoms/` (identity, protocol, capability, exemplar, methodology, language, campaign,
safety and more). Each atom is a versioned unit with a priority, dependencies and shard gating.

```
  atom library · 350+ YAML files
        │
        ▼
  ┌─ 1 · MANGLE SKELETON ───┐   what MUST apply, derived from shard · verb · language · phase
  ├─ 2 · VECTOR FLESH ──────┤   what is most relevant to this turn's facts
  ├─ 3 · DEPENDENCY ORDER ──┤   prerequisites come first
  ├─ 4 · TOKEN BUDGET ──────┤   fit, or drop by priority
  └────────────┬────────────┘
               ▼
       prompt  +  manifest       (the manifest records what went in)
```

To change behaviour, edit an atom, not a string hidden in a shard. `/jit` in the TUI shows the last
compilation: which atoms went in, and how long each stage took.

### 🌳 4 · CodeDOM: code is structure, not strings

The model reads and edits code **structurally** (`internal/tools/codedom/`):

```
  internal/auth/user.go
  ├── type User struct                              [12–30]
  ├── func (u *User) Validate() error               [32–55]  ◀── edit_element("User.Validate", …)
  │       callers_of  ──▶  handlers.go · Login
  │                   ──▶  handlers.go · Signup
  └── func NewUser(name string) *User               [57–64]
```

| Read | Search | Edit |
|:--|:--|:--|
| `get_elements` · `get_element` · `package_outline` · `predicate_outline` | `find_symbol` · `callers_of` · `callees_of` · `importers_of` · `unreferenced_symbols` | `edit_element` · `replace_element` · `insert_element` · `delete_element` · `repoint` · `apply_edits` |

Edits address symbols, not line coordinates that go stale. If a line tool is used, every mutation
reports its shift ("*File is now 377 lines (−11). Line numbers at or after 42 are now STALE.*"),
and a syntax guard refuses an edit that would unbalance the file. `repoint` and `apply_edits` carry
out a multi-file change as one transactional call, so a blast radius is handled by deterministic
code.

### 🌐 5 · The world model and holographic context

`internal/world/` projects the filesystem, ASTs and git into kernel facts (`file_topology`,
`code_defines`, `code_calls`, `test_coverage` …). Ask about one file, and the model gets that
file's **neighbourhood**: exported signatures, type definitions, who calls it, its role in the
architecture and its test coverage. It's assembled from the AST and the fact store, not guessed
from a grep.

### 🧠 6 · Context is a kernel decision

`internal/context/` scores candidate facts with **spreading activation**. Relevance starts at the
intent and flows outward along the links the facts define. The context layer then compresses,
prunes and orders the window. Masking policy (which old observations get condensed, and which
reasoning is always preserved) is derived in `context_compilation.mg`. The window is a decision
made by logic, not the leftovers of a truncation heuristic.

### ⚖️ 7 · Forcing gates: "done" has to be earned

```
      the model says "done"
                │
                ▼
      ┌───────────────────┐        each owed round is DERIVED (turn_rounds.mg);
      │ 1  build          │        a failure becomes a repair round,
      │ 2  test           │        with the evidence handed back to the model
      │ 3  critic         │ ─ fail ─────────────────────────┐
      │ 4  coverage       │                                 │
      │ 5  pinned         │                                 ▼
      │ 6  vet            │                         repair, then re-measure
      │ 7  removed tests  │                                 │
      │ 8  test run       │ ◀───────────────────────────────┘
      └─────────┬─────────┘
                ▼
       /done   or   /unverified        truthful, either way
```

- **Build and test** cover the packages the turn wrote *and every package that imports them*.
- **Coverage.** Changed code that no test executes is verdict evidence.
- **Pinning.** Remove any single function the turn changed, and a test the turn wrote must fail. A
  test that passes either way gets caught.
- **Removed-tests guard.** A turn that deleted tests is handed their source and must restore them,
  unless it also deleted the function they tested.
- **Vet and format.** Findings in the turn's own files are evidence, and formatting runs last.
- **A truthful verdict.** `/unverified` is an honest result. A failed gate is never reported as a
  success.

### 🗺️ 8 · Campaigns with checkpoints

```
  goal ─▶ decompose ─▶ ┌─ phase 1 ──┐ ─▶ checkpoint ─▶ ┌─ phase 2 ─┐ ─▶ checkpoint ─▶ … ─▶ --accept "<cmd>"
                       │ t1  t2  t3 │    (evidence)    │  t4  t5   │    (evidence)          exit 0, or a
                       └────────────┘                  └───────────┘                        remediation phase
                          write-set leases · rollback of the attempt's writes · replan between phases
```

`nerd campaign start` breaks a goal into phases and tasks (`internal/campaign/`), runs them in
dependency order under write-set leases, and **verifies each checkpoint before advancing**. A phase
that claims success without evidence fails. `--accept "<cmd>"` makes a command the campaign's
acceptance witness. In chat, `/campaign assault <scope>` turns the campaign adversarial: a
**Nemesis** persona hunts for panics, races and edge cases until it stops finding them.

### 🧑‍🔬 9 · Specialists, spawned on demand

Coder, tester, reviewer, researcher, plus any specialist you define (`nerd define-agent`, or atoms
under `.nerd/agents/`). Each gets a JIT-compiled identity, its own tool allowlist and an isolated
context, so a delegated task can't contaminate the session's history. `shard_profiles` routes each
persona to its own provider and model.

### 📜 10 · `nerd.md`: instructions with teeth

It works like `AGENTS.md`, except the machine-readable half is **enforced, not suggested**:

```yaml
---
forbid:
  - match: .nerd/config.json
    reason: user-owned runtime config
---
```

That frontmatter becomes `project_forbidden_path/2` in the fact store, and the write is denied
*before the tool runs*, on the interactive path and the shard path alike (`internal/projectdoc/`).

### 🐍 11 · Autopoiesis: it extends itself

```
   capability gap ─▶ generate tool ─▶ go_safety.mg check ─▶ compile ─▶ Thunderdome ─▶ register
         ▲                                                                              │
         └──────────────────────  the agent's surface grows  ◀──────────────────────────┘
```

When it hits a gap, the **Ouroboros** loop (`internal/autopoiesis/`) writes a tool, checks it
against a safety policy, compiles it, makes it survive an adversarial "Thunderdome", and registers
it. The **Northstar Guardian** (`internal/northstar/`) holds *your project's* vision and checks work
against it for drift. **Prompt evolution** tunes atoms from outcomes.

### 💾 12 · Memory in tiers

```
  ┌─ RAM ───────────  kernel facts: asserted (EDB) + derived (IDB) ─────────────────────┐
  ├─ SQLite ────────  session · world · knowledge graph · traces · learnings · tools ───┤
  ├─ vectors ───────  sqlite-vec ANN, with lexical fallback ────────────────────────────┤
  └─ cold ──────────  archived facts and snapshots ─────────────────────────────────────┘
```

Facts outlive the window (`internal/store/`). Evicted context stays recoverable, and stale evidence
is not supposed to survive a source change as current truth.

### 🔭 13 · The glass box

```bash
nerd why next_action   # the proof tree behind the decision, fact by fact
nerd query <pred>      # interrogate the kernel directly
nerd transparency      # glass-box telemetry and safety explanations
```

There's no "let me explain my reasoning" theatre. The reasoning *is* the artifact.

---

## Part VII · Where it stands

Being honest about the distance is part of the design. Two programs of record in this repo measure
how far the system is from the North Star.

### The elite-harness ladder

Each rung is climbed by pointing codeNERD at **real problems in this repository** and keeping only
results that pass a strict landing test:

- the brief states a symptom, not a diagnosis
- codeNERD alone edits the tree
- the tests fail before the change and pass after
- build, vet and test are green
- the verdict is truthful
- review finds no regression

Full record: [`Docs/journeys/05-elite-harness-ladder.md`](Docs/journeys/05-elite-harness-ladder.md).

```
   R9 ┤ disciplined specs   resumable Mangle DAG: idea → decision → work item   ·· not started
   R8 ┤ CodeDOM only        raw-text read/search/edit gone from the model       ·· not started
   R7 ┤ recursion           unattended cycles, each leaves the gates better     ·· not started
   R6 ┤ architecture docs   rewrite Docs/architecture from the code             ·· pilots landed
   R5 ┤ fifty files         one prompt, a whole class of findings               ·· probe passed *
   R4 ┤ ten files           a blast radius, through CodeDOM                     ·· probe: 1 of 2 *
   R3 ┤ five files          planned multi-step work                             ·· 1 unassisted landing
   R2 ┤ two files           follow a cause across a seam                        ·· probes, assisted
   R1 ┤ one file            three landings in a row                             ·· climbing  ◀── here
   R0 ┤ stable ground       build · vet · full suite green, 3 runs in a row     ·· PASSED
      ┴
        * a probe above the current rung doesn't count until the rungs below it pass
```

### Unattended hardening

[`Docs/journeys/06-unattended-hardening.md`](Docs/journeys/06-unattended-hardening.md) tracks the
"weeks unattended" half of the North Star. Run-level wall clocks are gone. Still open: an inventory
and ratchet test for each class of hardcoded limit, arbitrary truncation and logging blind spot,
with every limit hit asserted as a fact the kernel can turn into a repair obligation.

### SWE-bench

A bridge exists: `nerd swebench setup` routes an instance through the kernel and asserts its
expectation facts. **No SWE-bench score is claimed.** 100% is where the system is aimed, not a
number anyone has measured.

### Known gaps, tracked in the open

Partially wired subsystems and dormant integration points are expected here. They are tracked, not
hidden:

- [`Docs/journeys/`](Docs/journeys/): the code-grounded journey map, the forcing and completion
  audit, the external audit queue, and the seams where the kernel is still bypassed.
- `Docs/architecture/<subsystem>/WIRING-AND-NOT-BUILT.md` and `TODO.md`: for each subsystem, what
  is wired, what is dormant, and what was never built.
- [`AUDIT.md`](AUDIT.md): package-by-package correctness audit.

---

## Part VIII · It builds itself

```
        ┌──────────────── a brief: the symptom, never the diagnosis ────────────────┐
        │                                                                           ▼
  ┌───────────┐                                                            ┌─────────────┐
  │ reviewer  │ ◀──────── diff · tests · verdict · evidence ────────────── │  nerd fix   │
  └─────┬─────┘                                                            └─────────────┘
        │
        ├── landed?           keep it, record it in the ladder
        │
        └── harness failed?   THAT is the finding ──▶ fix the harness, test first ──▶ same brief again
```

codeNERD is developed by pointing codeNERD at codeNERD. When a run fails because of the harness
(a tool that refuses a correct edit, context that never reached the model, a verdict that lies),
the failure is the bug report. Some things that loop has caught:

- Line-range edits never reported the shift they caused, so a second edit landed at stale
  coordinates and silently duplicated declarations.
- `CloneForTask` copied three fields and forgot two, which quietly stripped `nerd.md` rules and
  holographic context from *every* delegated task.
- The test gate ran only the packages a turn wrote, so a change that broke an importer still
  reported `tests ok`.
- A repair prompt told the model "the tests pass" above a FAIL trace, and the model weakened its own
  assertion to escape.
- A campaign fallback could write files outside the turn's obligations. The only thing keeping that
  bypass latent was a permission check that never matched its payload.

Dogfooding isn't a slogan here. It's the test harness.

---

## Part IX · Get started

**You need** Go 1.26+ (for source builds) and an API key for any supported provider.

```bash
# 1 · build
go build -o nerd ./cmd/nerd

# 2 · point it at a model
export ANTHROPIC_API_KEY="…"   # or OPENAI_ · GEMINI_ · XAI_ · ZAI_ · DASHSCOPE_
                               #    META_ · MOONSHOT_ · OPENROUTER_ … _API_KEY

# 3 · wake it up
./nerd init     # scan the codebase, build the fact base, write .nerd/
./nerd          # the interactive TUI, with the Glass Box pane
```

**Providers:** Anthropic · OpenAI · Gemini · xAI · Z.AI · OpenRouter · DashScope · Meta · Moonshot ·
local Ollama. The `claude-cli` and `codex-cli` engines can drive a subscription CLI instead.

**Run models in tiers.** Reasoning-heavy verbs escape to an expensive model and the bulk stays
cheap. Which verbs count as reasoning-heavy is *policy* (`intent_requires_reasoning_model/1` in
`delegation.mg`), not a Go `switch`:

```jsonc
// .nerd/config.json
{
  "provider": "anthropic",
  "model":    "<frontier model>",                                // interactive turns
  "worker":   { "provider": "ollama",    "model": "qwen3:8b" },  // shards, delegated bulk work
  "planner":  { "provider": "anthropic", "model": "<frontier model>" } // /review /audit /campaign
}
```

`nerd config check` validates the file and refuses contradictions. `nerd config full` writes out
every configurable field, so nothing is silently defaulted.

### The command surface

<table>
<tr><th align="left" colspan="2">🏁 Core</th></tr>
<tr><td><code>nerd</code></td><td>Interactive TUI with the Glass Box pane</td></tr>
<tr><td><code>nerd run "…"</code></td><td>One full OODA loop, headless</td></tr>
<tr><td><code>nerd init</code> · <code>nerd scan</code></td><td>Build <code>.nerd/</code> · refresh the index without a full reinit</td></tr>
<tr><td><code>nerd status</code> · <code>nerd config check</code></td><td>System state · validate the configuration</td></tr>
<tr><th align="left" colspan="2">🔎 Interrogate</th></tr>
<tr><td><code>nerd query &lt;pred&gt;</code></td><td>Query derived facts straight from the kernel</td></tr>
<tr><td><code>nerd why [pred]</code></td><td>Print the proof tree behind a conclusion</td></tr>
<tr><td><code>nerd explain &lt;target&gt;</code></td><td>Explain code with full holographic context</td></tr>
<tr><td><code>nerd check-mangle &lt;files&gt;</code></td><td>Validate <code>.mg</code> syntax and stratification</td></tr>
<tr><td><code>nerd dream</code> · <code>nerd shadow</code> · <code>nerd whatif</code></td><td>Simulate an action's effects before anything real happens</td></tr>
<tr><th align="left" colspan="2">🛠️ Work</th></tr>
<tr><td><code>nerd fix "&lt;symptom&gt;"</code></td><td>One gated turn: fix, pin with tests, prove. <code>--acceptance</code> takes a failing-test contract</td></tr>
<tr><td><code>nerd review &lt;target&gt;</code></td><td>Review, routed to the reasoning tier</td></tr>
<tr><td><code>nerd spawn &lt;persona&gt; "…"</code></td><td>Delegate to a specialist in an isolated context</td></tr>
<tr><td><code>nerd campaign start "…"</code></td><td>Multi-phase campaign with verified checkpoints (<code>--type</code>, <code>--docs</code>, <code>--accept</code>)</td></tr>
<tr><td><code>nerd campaign recurse</code></td><td>Improve the workspace node by node, bottom to top: measure, fix, keep only what the gates prove (<code>--plan</code> to preview)</td></tr>
<tr><td><code>nerd regression run</code></td><td>Run the regression battery</td></tr>
</table>

### Building from source

```bash
go build -o nerd ./cmd/nerd
go test ./...
```

For vector search, build with `sqlite-vec`:

```bash
CGO_CFLAGS="-I$(pwd)/sqlite_headers" go build -tags sqlite_vec -o nerd ./cmd/nerd
```

> [!NOTE]
> `-tags sqlite_vec` selects `internal/store/vec_support_enabled.go`, which makes the store refuse to
> open rather than silently degrade. The tag alone does **not** supply the extension: `vec0` has to
> be compiled into the modernc SQLite driver (see `internal/store/local_core.go`). Without it, ANN
> search falls back to lexical matching and every boot logs
> `sqlite-vec not available; falling back from ANN to lexical search`. Because the prompt selector is
> part logic and part vector, that fallback quietly turns the vector share into keyword matching.

### Writing rules

```mangle
# Variables are UPPERCASE. Constants are /lowercase. Every predicate needs a Decl.
# Negation applies only to variables a positive atom has already bound.
Decl next_action(ActionType) bound [/name].

next_action(/generate_code) :-
    user_intent(ID, /mutation, /generate, Target, _),
    !block_action(ID, _).
```

Aggregation uses the `|> do … let …` pipeline. Don't use Mangle for fuzzy matching: retrieve with
embeddings first, then assert structured facts. Read [`internal/mangle/agents.md`](internal/mangle/agents.md)
before editing any `.mg` file.

---

## Part X · The map

| Layer | Tech |
|:--|:--|
| Executive | [Mangle](https://github.com/google/mangle), deductive database programming, run by the [`mangle-go`](https://codeberg.org/TauCeti/mangle-go) engine |
| CLI / TUI | [Cobra](https://github.com/spf13/cobra) · [Bubble Tea](https://github.com/charmbracelet/bubbletea) |
| Code structure | [Tree-sitter](https://github.com/smacker/go-tree-sitter) + Go AST → CodeDOM |
| Memory | [modernc SQLite](https://gitlab.com/cznic/sqlite) + optional `sqlite-vec` |
| Browser | [Rod](https://github.com/go-rod/rod) (CDP) |
| Sandbox | Docker, auto-provisioned |
| Logging | [Zap](https://github.com/uber-go/zap), structured and categorized |

```
codenerd/
├── cmd/nerd/              the CLI and the Bubble Tea TUI
├── internal/
│   ├── core/              kernel · VirtualStore · Dreamer · shard manager
│   │   └── defaults/      the Mangle corpus: schemas + policy/ (~125 .mg files)
│   ├── mangle/            engine binding · parse lock · differential engine · LSP
│   ├── session/           the turn loop · forcing gates · spawner
│   ├── prompt/            JIT compiler · atoms/ (350+ YAML files)
│   ├── perception/        language ▸ atoms · provider clients
│   ├── articulation/      atoms ▸ language · Piggyback
│   ├── tools/             registry · codedom/ · shell · research
│   ├── world/             world model · holographic context
│   ├── context/           spreading activation · compression
│   ├── store/             tiered memory · vectors
│   ├── campaign/          phases · checkpoints · assault · recurse
│   ├── verification/      the verification gate
│   ├── autopoiesis/       Ouroboros · prompt evolution
│   ├── northstar/         the vision Guardian
│   ├── projectdoc/        nerd.md parsing and enforcement
│   ├── transparency/      the glass box
│   └── system/            boots it all into one Cortex
└── Docs/
    ├── journeys/          code-grounded maps, the ladder, hardening, audits
    └── architecture/      per-subsystem corpora (being rewritten from the code under R6)
```

### Going deeper

| Read | For |
|:--|:--|
| [`agents.md`](agents.md) | The repo contract, the vision, the working map. **Read before any architectural change** |
| [`Docs/journeys/`](Docs/journeys/) | How things really flow, cited to `path:line`: the ladder, hardening, audits |
| [`internal/mangle/agents.md`](internal/mangle/agents.md) | Before editing any `.mg` file |
| [`internal/prompt/agents.md`](internal/prompt/agents.md) | Before changing prompt behaviour |
| [`internal/core/agents.md`](internal/core/agents.md) | Kernel and execution internals |
| [`Docs/architecture/`](Docs/architecture/) | Per-subsystem corpora. The July 2026 generation is orientation only, not authority, and is being rewritten from the code |

### Glossary

| Term | Meaning |
|:--|:--|
| **Mangle** | Google's deductive database programming language, and codeNERD's executive |
| **EDB / IDB** | Asserted facts (the extensional database) and facts derived by rules (the intensional database) |
| **Fixpoint** | The state where applying every rule derives nothing new. Executive decisions live here |
| **Atom (prompt)** | A versioned YAML unit of prompt text, selected per turn by the JIT compiler |
| **Transducer** | Perception (language ▸ atoms) or articulation (atoms ▸ language) |
| **Piggyback** | The reply envelope: a `surface_response` for you and a `control_packet` for the kernel |
| **VirtualStore** | The single routed door through which every effect happens and every result returns |
| **CodeDOM** | Code as addressable elements (symbols, calls, importers) instead of lines of text |
| **Shard** | A specialist agent (coder, tester, reviewer, researcher, or yours) with its own context |
| **Campaign** | A multi-phase plan with dependency ordering, write-set leases and verified checkpoints |
| **Forcing gate** | A round a turn owes (build, test, pin, vet, …) before its verdict can say done |
| **Dreamer** | Simulates an action's consequences before it runs |
| **Ouroboros** | The loop that generates, hardens and registers new tools |
| **Glass box** | The ability to ask *why*, and get the actual derivation back |
| **North Star** | The final state the system is measured against, forever and on every turn |

---

<div align="center">

```
          ╭──────────────────────────────────────────────────────────────────╮
          │                          T H E   W A G E R                       │
          │                                                                  │
          │   A model that can't remember its constraints will eventually    │
          │   violate them.  A kernel that can't be creative will never      │
          │   surprise you.  Give each one the job it's actually good at,    │
          │   and keep measuring the distance to the North Star.             │
          ╰──────────────────────────────────────────────────────────────────╯
```

[![GitHub](https://img.shields.io/badge/theRebelliousNerd%2Fcodenerd-181717?style=for-the-badge&logo=github)](https://github.com/theRebelliousNerd/codenerd)

</div>

## License

Copyright (C) 2026 theRebelliousNerd

Licensed under the **GNU Affero General Public License v3.0**. See [LICENSE](LICENSE).

If you fork codeNERD, or run a modified version as a network service, the AGPL requires you to
publish your source and keep the attribution. Section 13 is the part most licenses lack: offering a
modified version over a network counts as distribution, so a hosted derivative owes its source to
its users too.
