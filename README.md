<div align="center">

```
                    _      _   _ _____ ____  ____
   ___ ___   __| | ___| \ | | ____|  _ \|  _ \
  / __/ _ \ / _` |/ _ \  \| |  _| | |_) | | | |
 | (_| (_) | (_| |  __/ |\  | |___|  _ <| |_| |
  \___\___/ \__,_|\___|_| \_|_____|_| \_\____/

    Logic determines Reality; the Model merely describes it.
```

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Mangle](https://img.shields.io/badge/Kernel-Google_Mangle-4285F4?style=for-the-badge&logo=google&logoColor=white)](https://github.com/google/mangle)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Architecture](https://img.shields.io/badge/Architecture-Neuro--Symbolic-purple?style=for-the-badge)]()

**A high-assurance Logic-First CLI coding agent that separates creative intelligence from deterministic control.**

[Quick Start](#-quick-start) · [Architecture](#-architecture) · [Commands](#-commands) · [Shards](#-shardagents) · [Documentation](#-documentation)

</div>

---

## The Problem With AI Agents

Current AI coding agents make a **category error**: they ask LLMs to handle *everything*—creativity AND planning, insight AND memory, problem-solving AND self-correction—when LLMs excel at the former but fundamentally struggle with the latter.

**codeNERD inverts the hierarchy:**

| Traditional Agents | codeNERD |
|---|---|
| LLM makes all decisions | Logic kernel makes decisions |
| Probabilistic planning | Deterministic Datalog rules |
| Context window = memory | Infinite memory via fact store |
| Hope the model doesn't hallucinate | Constitutional safety gates |
| Black box reasoning | Glass box traceability |

---

## The Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         USER / TERMINAL                             │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                    ┌─────────────────▼─────────────────┐
                    │    PERCEPTION TRANSDUCER (LLM)    │
                    │    Natural Language → Mangle Atoms │
                    └─────────────────┬─────────────────┘
                                      │
┌─────────────────────────────────────▼───────────────────────────────┐
│                         CORTEX KERNEL                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │
│  │   FACT STORE    │  │  MANGLE ENGINE  │  │   VIRTUAL STORE     │  │
│  │   (Working Mem) │◄─┤  (Logic CPU)    │─►│   (FFI Gateway)     │  │
│  └─────────────────┘  └─────────────────┘  └──────────┬──────────┘  │
│                              ▲                        │             │
│                              │                        ▼             │
│                    ┌─────────┴─────────┐    ┌─────────────────────┐ │
│                    │   POLICY RULES    │    │   SHARD MANAGER     │ │
│                    │   (policy.gl)     │    │   ┌─────┬─────┐     │ │
│                    └───────────────────┘    │   │Coder│Test │     │ │
│                                             │   ├─────┼─────┤     │ │
│                                             │   │Revw │Rsrch│     │ │
│                                             │   └─────┴─────┘     │ │
└─────────────────────────────────────────────┴─────────────────────┴─┘
                                      │
                    ┌─────────────────▼─────────────────┐
                    │   ARTICULATION TRANSDUCER (LLM)   │
                    │   Mangle Atoms → Natural Language  │
                    │   + Piggyback Control Protocol     │
                    └───────────────────────────────────┘
```

### The OODA Loop

Every interaction flows through:

1. **Observe** — Perception Transducer converts NL to intent atoms
2. **Orient** — Spreading Activation selects relevant context
3. **Decide** — Mangle Engine derives `next_action` from policy rules
4. **Act** — Virtual Store executes via ShardAgents

---

## Quick Start

### Prerequisites

- **Go 1.24+** — [Download](https://go.dev/dl/)
- **API Key** — Gemini (`ZAI_API_KEY` or `GEMINI_API_KEY`)

### Install

```bash
# Clone the repository
git clone https://github.com/theRebelliousNerd/codenerd.git
cd codenerd

# Build
go build -o nerd.exe ./cmd/nerd

# Set API key
export ZAI_API_KEY="your-key-here"
```

### Initialize a Workspace

```bash
# Run once per project directory
nerd init
```

This creates `.nerd/`, scans your codebase, builds a profile, and preloads facts into the kernel.

### Launch

```bash
# Interactive chat TUI (Bubble Tea)
nerd

# Or single-shot command
nerd run "explain the authentication flow"
```

---

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `nerd` | Launch interactive chat TUI |
| `nerd run "<instruction>"` | Execute single OODA loop |
| `nerd init` | Initialize workspace (creates `.nerd/`) |
| `nerd query <predicate>` | Query derived facts from kernel |
| `nerd why [predicate]` | Explain derivation chain |
| `nerd status` | Show system status and loaded facts |

### Shard Commands

| Command | Description |
|---------|-------------|
| `nerd spawn coder "<task>"` | Invoke CoderShard for code generation |
| `nerd spawn tester "<task>"` | Invoke TesterShard for test creation |
| `nerd spawn reviewer "<task>"` | Invoke ReviewerShard for code review |
| `nerd spawn researcher "<topic>"` | Invoke ResearcherShard for deep research |

### Browser Commands

| Command | Description |
|---------|-------------|
| `nerd browser launch` | Launch headless Chrome session |
| `nerd browser session` | Show active browser sessions |
| `nerd browser snapshot` | Capture DOM snapshot |

### Chat Mode Commands

Type `/help` in chat mode for full command list:

| Command | Description |
|---------|-------------|
| `/query <pred>` | Query the Mangle kernel |
| `/shadow` | Enter shadow mode (simulated execution) |
| `/whatif <action>` | Project effects without executing |
| `/approve` | Approve pending changes |
| `/agents` | Show active ShardAgents |
| `/config set-key <key>` | Set API key |

---

## ShardAgents

ShardAgents are ephemeral sub-kernels for parallel task execution:

### Type A: Ephemeral Generalists
> Spawn → Execute → Die. RAM only.

| Shard | Purpose |
|-------|---------|
| **CoderShard** | Code generation, refactoring, bug fixes |
| **TesterShard** | Test creation, TDD loops, coverage analysis |
| **ReviewerShard** | Code review, security audit, best practices |

### Type B: Persistent Specialists
> Pre-populated with domain knowledge. SQLite-backed.

| Shard | Purpose |
|-------|---------|
| **ResearcherShard** | Deep research, knowledge ingestion, llms.txt parsing |

### Type S: System Shards
> Built-in capabilities, always available.

| Shard | Purpose |
|-------|---------|
| **FileShard** | Filesystem operations (read, write, search) |
| **ShellShard** | Command execution with safety gates |
| **GitShard** | Version control operations |
| **ToolGeneratorShard** | Self-generating tools (Ouroboros Loop) |

---

## Safety Model

### Constitutional Safety

Every action must derive `permitted(Action)` through the policy rules:

```mangle
permitted(Action) :- safe_action(Action).

permitted(Action) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action).

dangerous_action(Action) :-
    action_type(Action, /exec_cmd),
    cmd_string(Action, Cmd),
    fn:string_contains(Cmd, "rm").
```

### Commit Barrier

Build errors block commits:

```mangle
block_commit("Build Broken") :-
    diagnostic(/error, _, _, _, _).
```

### Shadow Mode

Project effects before acting:

```bash
nerd run --shadow "delete all test files"
# Shows what WOULD happen without executing
```

---

## Project Structure

```
codenerd/
├── cmd/
│   └── nerd/              # CLI entrypoint (Cobra)
│       └── chat/          # Interactive TUI (Bubble Tea)
├── internal/
│   ├── core/              # Kernel, VirtualStore, ShardManager
│   ├── perception/        # NL → Mangle transduction
│   ├── articulation/      # Mangle → NL + Piggyback Protocol
│   ├── shards/            # CoderShard, TesterShard, etc.
│   │   ├── researcher/    # Deep research subsystem
│   │   └── system/        # Built-in system shards
│   ├── mangle/            # .gl schema and policy files
│   ├── store/             # Memory tiers (RAM, Vector, Graph)
│   ├── campaign/          # Multi-phase goal orchestration
│   ├── browser/           # Rod-based browser automation
│   ├── world/             # Filesystem and AST projection
│   └── tactile/           # Tool execution layer
└── .nerd/                 # Workspace state (created by init)
```

---

## Key Technologies

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Logic Kernel** | [Google Mangle](https://github.com/google/mangle) | Datalog-based reasoning engine |
| **CLI Framework** | [Cobra](https://github.com/spf13/cobra) | Command-line interface |
| **TUI Framework** | [Bubble Tea](https://github.com/charmbracelet/bubbletea) | Terminal user interface |
| **Browser Automation** | [Rod](https://github.com/go-rod/rod) | Chrome DevTools Protocol |
| **Persistence** | [SQLite](https://github.com/modernc/sqlite) | Specialist knowledge storage |
| **Logging** | [Zap](https://github.com/uber-go/zap) | Structured logging |

---

## Documentation

| Document | Description |
|----------|-------------|
| [CLAUDE.md](CLAUDE.md) | Agent instructions and project context |
| [Architecture](/.claude/skills/codenerd-builder/references/architecture.md) | Theoretical foundations |
| [Mangle Schemas](/.claude/skills/codenerd-builder/references/mangle-schemas.md) | Complete schema reference |
| [Implementation Guide](/.claude/skills/codenerd-builder/references/implementation-guide.md) | Go patterns and components |
| [Piggyback Protocol](/.claude/skills/codenerd-builder/references/piggyback-protocol.md) | Control stream specification |
| [Campaign System](/.claude/skills/codenerd-builder/references/campaign-orchestrator.md) | Multi-phase orchestration |

---

## Development

### Building

```bash
go build -o nerd.exe ./cmd/nerd
```

### Testing

```bash
go test ./...
```

### Mangle Rules

All predicates require `Decl` in `schemas.gl` before use:

```mangle
# Variables are UPPERCASE, constants are /lowercase
next_action(/generate_code) :-
    user_intent(ID, /mutation, /generate, Target, _),
    !block_action(ID, _).
```

---

## Philosophy

> **"Logic determines Reality; the Model merely describes it."**

The LLM is the creative center—it understands problems, generates solutions, and crafts novel approaches. But it does not *decide*. The Mangle kernel holds the ground truth, enforces invariants, and derives the next action through formal logic.

This separation means:
- **No hallucinated actions** — only logically permitted operations execute
- **Perfect memory** — facts persist beyond context windows
- **Glass box reasoning** — every decision is traceable via `nerd why`
- **Self-correction** — abductive hypotheses trigger automatic recovery

---

<div align="center">

**Built for developers who demand deterministic safety with creative power.**

[![GitHub](https://img.shields.io/badge/GitHub-theRebelliousNerd%2Fcodenerd-181717?style=flat-square&logo=github)](https://github.com/theRebelliousNerd/codenerd)

</div>
