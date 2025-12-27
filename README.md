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
2. **Orient** — Intent Routing logic determines persona and config atoms
3. **Decide** — JIT Compiler assembles prompt and AgentConfig
4. **Act** — Session Executor runs the unified execution loop

---

## Quick Start

### Prerequisites

- **Go 1.24+** — [Download](https://go.dev/dl/) (only for building from source)
- **API Key** — Set `ZAI_API_KEY` environment variable

### Option 1: Pre-built Binary (Recommended)

1. **Download** the latest `nerd.exe` from [Releases](https://github.com/theRebelliousNerd/codenerd/releases)

2. **Drop it in your project root:**

   ```text
   your-project/
   ├── nerd.exe       # ← Drop the binary here
   ├── src/
   ├── package.json
   └── ...
   ```

3. **Set your API key:**

   ```bash
   # Windows PowerShell
   $env:ZAI_API_KEY="your-key-here"

   # Windows CMD
   set ZAI_API_KEY=your-key-here

   # Linux/macOS
   export ZAI_API_KEY="your-key-here"
   ```

4. **Initialize and run:**

   ```bash
   ./nerd init    # Creates .nerd/ directory, scans codebase
   ./nerd         # Launch interactive chat TUI
## Quickstart

### 1. Install & Build
**Prerequisites:**
- Go 1.22+
- Docker (must be running for sandboxed execution)

**Build:**
```bash
# Clone the repo
git clone https://github.com/google/codenerd
cd codenerd

# Build the CLI
go build -o nerd ./cmd/nerd
```

### 2. Configure
The CLI needs API keys for the intelligence layer.
1. Run `nerd` once to generate the config file at `~/.codenerd/config.json`, OR manually create it.
2. Set your keys via environment variables or the config file:

```bash
export ZAI_API_KEY="your_key_here"
# Optional: Context retrieval key if applicable
export CONTEXT7_API_KEY="your_ctx_key"
```

### 3. Initialize Your Project
Navigate to your project directory (e.g., your Python app) and initialize CodeNERD. This creates a local `.nerd` directory and indexes your codebase.

```bash
cd /path/to/your/project
/path/to/codenerd/nerd init
```
*Tip: Add `nerd` to your PATH for easier access.*

## How to Use

### Interactive Chat (TUI)
The primary way to use CodeNERD is the interactive terminal UI.
```bash
nerd
```
Inside the TUI:
- **Chat**: Type natural language requests (e.g., "Add a new endpoint to the API").
- **Commands**: Use `/` commands like `/help`, `/status`, or `/apply`.
- **Review**: The TUI shows you the "Glass Box" view of the agent's reasoning.

### CLI Commands
For headless or single-shot tasks:
- **Run a task**: `nerd run "Analyze the security of this repo"`
- **Query facts**: `nerd query "func:main"` (Inspect the knowledge graph)

### Supported Languages
CodeNERD has deep support for **Python** and **Go**, including:
- **Symbol Indexing**: Tree-sitter integrated parsing.
- **Sandboxed Execution**: Auto-provisioned Docker containers for running tests.
- **Test Integration**: Automated `pytest` and `go test` execution.

See [Docs/guides/getting_started_python.md](Docs/guides/getting_started_python.md) for a detailed Python workflow.

## Known Limitations
- **Browser Snapshots**: Persistence is currently experimental.
- **Specialist Agents**: Deep research agents are in active development.

---

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `nerd` | Launch interactive chat TUI |
| `nerd run "<instruction>"` | Execute single OODA loop |
| `nerd init` | Initialize workspace (creates `.nerd/`) |
| `nerd init --force` | Reinitialize (preserves learned preferences) |
| `nerd scan` | Refresh codebase index without full reinit |
| `nerd query <predicate>` | Query derived facts from kernel |
| `nerd why [predicate]` | Explain derivation chain |
| `nerd status` | Show system status and loaded facts |
| `nerd check-mangle <files>` | Validate Mangle (.mg) syntax |

### Shard Commands

| Command | Description |
|---------|-------------|
| `nerd spawn coder "<task>"` | Invoke CoderShard for code generation |
| `nerd spawn tester "<task>"` | Invoke TesterShard for test creation |
| `nerd spawn reviewer "<task>"` | Invoke ReviewerShard for code review |
| `nerd spawn researcher "<topic>"` | Invoke ResearcherShard for deep research |
| `nerd define-agent --name X --topic Y` | Define a new specialist agent |

### Campaign Commands

| Command | Description |
|---------|-------------|
| `nerd campaign start "<goal>"` | Start a multi-phase campaign |
| `nerd campaign start --docs ./specs/` | Start from spec documents |
| `nerd campaign status` | Show current campaign progress |
| `nerd campaign pause` | Pause the current campaign |
| `nerd campaign resume` | Resume a paused campaign |
| `nerd campaign list` | List all campaigns |

**Adversarial assault (soak/stress) campaigns** run from chat mode:
- Slash command: `/campaign assault [repo|module|subsystem|package] [include...] [--batch N] [--cycles N] [--timeout N] [--race] [--vet] [--no-nemesis]`
- Natural language: `run an assault campaign on internal/core`

### Browser Commands

| Command | Description |
|---------|-------------|
| `nerd browser launch` | Launch headless Chrome instance |
| `nerd browser session <url>` | Create browser session for a URL |
| `nerd browser snapshot <id>` | Capture DOM as Mangle facts |

### Chat Mode Commands

Type `/help` in chat mode for full command list. Help is **progressive** — it shows commands appropriate for your experience level.

| Command | Description |
|---------|-------------|
| `/help [all\|basic\|advanced\|expert]` | Progressive help by experience level |
| `/query <pred>` | Query the Mangle kernel |
| `/why <fact>` | **Enhanced** — Explain derivation with proof trees |
| `/transparency [on\|off]` | Toggle visibility into codeNERD operations |
| `/shadow` | Enter shadow mode (simulated execution) |
| `/whatif <action>` | Project effects without executing |
| `/campaign <start\|assault\|status\|pause\|resume\|list> [...]` | Start/manage campaigns (including adversarial assault sweeps) |
| `/approve` | Approve pending changes |
| `/agents` | Show active ShardAgents |
| `/config` | Configuration menu |
| `/clear` | Clear chat history |
| `/quit` | Exit TUI |

---

## Agent Execution Model

codeNERD uses a **JIT-driven universal executor** that replaces hardcoded shard classes with dynamic, config-based agents. All agent behavior is now determined by:

- **JIT-compiled prompts** from `internal/prompt/atoms/`
- **Mangle intent routing rules** in `internal/mangle/intent_routing.mg`
- **ConfigFactory-generated AgentConfig** specifying tools and policies

### The Session Executor

The **Session Executor** (`internal/session/executor.go`) is a unified execution loop that:

1. Receives intent atoms from the Perception Transducer
2. Queries Mangle logic to determine persona (coder, tester, reviewer, researcher)
3. JIT-compiles a system prompt and AgentConfig
4. Executes the LLM interaction with appropriate tools and safety gates
5. Routes actions through VirtualStore

### SubAgents

**SubAgents** (`internal/session/subagent.go`) are dynamically spawned execution contexts with configurable lifecycle:

| Type | Lifespan | Description |
|------|----------|-------------|
| **Ephemeral** | Single task | Spawn → Execute → Terminate (RAM only) |
| **Persistent** | Multi-turn | Maintains conversation history and state |
| **System** | Long-running | Background services (indexing, learning) |

### Intent → Persona Mapping

Mangle rules automatically route intents to the appropriate persona:

| Intent Verbs | Persona | Tools | Policies |
|--------------|---------|-------|----------|
| fix, implement, refactor, create | **Coder** | file_write, shell_exec, git | code_safety.mg |
| test, cover, verify, validate | **Tester** | test_exec, coverage_analyzer | test_strategy.mg |
| review, audit, check, analyze | **Reviewer** | hypothesis_gen, impact_analysis | review_policy.mg |
| research, learn, document, explore | **Researcher** | web_fetch, doc_parse, kb_ingest | research_strategy.mg |

### No More Hardcoded Shards

The previous architecture required separate Go implementations for each shard type (CoderShard, TesterShard, etc.), totaling ~35,000 lines of code. The new JIT-driven model eliminates this boilerplate, replacing it with:

- **391 lines** in `session/executor.go` (universal loop)
- **385 lines** in `session/spawner.go` (dynamic spawning)
- **339 lines** in `session/subagent.go` (lifecycle management)
- **228 lines** in `mangle/intent_routing.mg` (declarative routing)

---

## Advanced Features

### User Experience & Transparency

codeNERD includes a comprehensive UX system that adapts to your experience level:

#### Progressive Disclosure

- **Beginner**: Core commands only with explanations
- **Intermediate**: Basic + advanced shortcuts
- **Advanced**: Full command set + keyboard shortcuts
- **Expert**: All commands + internals access

#### Transparency Mode (`/transparency on`)

- See shard execution phases (Initializing → Analyzing → Generating → Complete)
- View safety gate explanations when actions are blocked
- Get verbose error context with remediation suggestions

#### Enhanced `/why` Command

```
/why next_action

## Explanation

**Query**: `next_action`

- `next_action(/spawn_coder)`
  *derived via action selection strategy*
  **Because:**
  - `user_intent("id_123", /mutation, /fix, "auth.go", /none)` **(base fact)**
```

#### First-Run Onboarding

- Automatic detection of new users
- Interactive wizard for API setup and experience level
- "Wow moment" demo of unique capabilities

### Adversarial Co-Evolution (Nemesis)

codeNERD includes a built-in adversary that tries to break your patches before they ship:

```
Patch Submitted → Nemesis Analyzes → Attack Tools Generated → Thunderdome Battle
                                                                    ↓
                                    ← Patch Hardened ← Vulnerabilities Found?
```

- **NemesisShard** generates targeted chaos tools to expose weaknesses
- **VulnerabilityDB** tracks successful attacks and lazy patterns
- **Thunderdome** arena runs attack vectors in isolated sandboxes
- Integrated into `/review` command for adversarial code review

### Adversarial Assault Campaign (Soak/Stress)

The chat `/campaign assault ...` workflow runs staged `go test`/`go vet`/Nemesis sweeps over your repo (or selected modules/subsystems) and persists artifacts under `.nerd/campaigns/<campaign>/assault/` for long-horizon triage and remediation.

### Multi-Language Data Flow Analysis

Data flow extraction now supports 5 languages via Tree-sitter:

| Language | Parser | Tracks |
|----------|--------|--------|
| Go | Native AST | Taint propagation, nil checks, error handling |
| Python | Tree-sitter | Variable flow, imports, function calls |
| TypeScript | Tree-sitter | Type-aware data flow, async chains |
| JavaScript | Tree-sitter | Variable flow, closure analysis |
| Rust | Tree-sitter | Ownership, borrowing, unsafe blocks |

### JIT Prompt Compiler & Configuration System

Runtime prompt and configuration assembly with context-aware atom selection:

**Prompt Compilation:**
- **Storage** - Agent prompts in `.nerd/shards/{agent}_knowledge.db`; shared corpus in `.nerd/prompts/corpus.db` (seeded from baked `internal/core/defaults/prompt_corpus.db`)
- **Token Budget** - Automatic prompt trimming to stay within context limits
- **Contextual Selection** - Atoms selected by intent verb, language, campaign phase
- **Semantic Search** - Embedding-based retrieval of relevant prompt fragments

**Configuration Generation (`internal/prompt/config_factory.go`):**
- **ConfigAtom** - Fragments specifying tools, policies, and priority for each intent
- **Config Merging** - Multiple ConfigAtoms merge to create comprehensive AgentConfig
- **Tool Selection** - Only necessary tools are exposed to the LLM for each task
- **Policy Loading** - Mangle policy files hot-loaded based on agent persona

**Architecture:**
```
User Intent → Intent Routing (.mg) → ConfigFactory → AgentConfig
                                   ↓
                            JIT Compiler → System Prompt
                                   ↓
                            Session Executor → LLM + VirtualStore
```

This eliminates the need for hardcoded shard configurations, enabling fully dynamic agent specialization at runtime

### MCP (Model Context Protocol) Integration

JIT Tool Compiler for intelligent MCP tool serving:

- **Skeleton/Flesh Bifurcation** - Core tools always available, context-dependent tools scored
- **Three-Tier Rendering** - Full (≥70), Condensed (40-69), Minimal (20-39) based on relevance
- **LLM Tool Analysis** - Automatic metadata extraction for new tools
- **Mangle Integration** - Tool selection rules in logic

### Prompt Evolution System (System Prompt Learning)

Automatic evolution of prompt atoms based on execution feedback:

- **LLM-as-Judge** - Evaluates task execution with detailed error categorization
- **Strategy Database** - Problem-type-specific strategies that improve over time
- **Atom Generator** - Creates new prompt atoms from failure patterns
- **JIT Integration** - Evolved atoms immediately available at runtime

### Impact-Aware Code Review

ReviewerShard uses Mangle to build surgical review context:

1. **Pre-flight Checks** — `go build` + `go vet` before LLM review
2. **Hypothesis Generation** — Mangle rules flag potential issues (nil deref, SQL injection, race conditions)
3. **Impact Analysis** — Queries call graph to include affected callers
4. **LLM Verification** — Model confirms/refutes hypotheses with semantic understanding

```mangle
# Example: Mangle flags unsafe dereference
hypothesis(/unsafe_deref, File, Line, Var) :-
    data_flow_sink(File, Line, Var, /deref),
    !null_checked(File, Line, Var).
```

### Holographic Context

X-Ray vision for AI agents analyzing code:

- **Package Scope** — Sibling files, exported symbols, type definitions
- **Architectural Layer** — Module, role, system purpose
- **Dependency Graph** — Direct imports, importers, external deps
- **Impact Priority** — Callers sorted by Mangle-derived importance

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
│   └── nerd/              # CLI entrypoint (Cobra, 80+ files)
│       └── chat/          # Interactive TUI (Bubble Tea, modularized)
├── internal/              # 32 packages, ~105K LOC (35K lines removed in JIT refactor)
│   ├── core/              # Kernel, VirtualStore (modularized, ShardManager removed)
│   ├── session/           # NEW: Session Executor, Spawner, SubAgent
│   ├── jit/               # NEW: JIT configuration types and validation
│   │   └── config/        # AgentConfig schema
│   ├── perception/        # NL → Intent transduction, multi-provider LLM
│   ├── articulation/      # Response generation + Piggyback Protocol
│   ├── autopoiesis/       # Self-modification, Ouroboros, Prompt Evolution
│   ├── mcp/               # MCP integration, JIT Tool Compiler
│   ├── prompt/            # JIT Prompt Compiler, ConfigFactory, atoms
│   ├── shards/            # Registration only (implementations removed)
│   ├── mangle/            # .mg schema, policy, and intent routing files
│   │   └── intent_routing.mg  # NEW: Declarative persona/action routing
│   ├── store/             # Memory tiers (RAM, Vector, Graph)
│   ├── campaign/          # Multi-phase goal orchestration
│   ├── browser/           # Rod-based browser automation
│   ├── world/             # Filesystem, AST, and GraphQuery interface
│   ├── tactile/           # Tool execution layer
│   ├── config/            # Configuration with LLM timeout consolidation
│   ├── ux/                # User experience & journey tracking
│   └── transparency/      # Operation visibility & explanations
└── .nerd/                 # Workspace state (created by init)
```

**Major Architectural Changes (Dec 2024):**
- **Removed** 35,000+ lines of hardcoded shard implementations
- **Added** unified session-based execution model (~1,100 lines)
- **Replaced** Go-based shard logic with Mangle intent routing rules
- **Centralized** agent configuration through JIT ConfigFactory

---

## Key Technologies

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Logic Kernel** | [Google Mangle](https://github.com/google/mangle) | Datalog-based reasoning engine |
| **CLI Framework** | [Cobra](https://github.com/spf13/cobra) | Command-line interface |
| **TUI Framework** | [Bubble Tea](https://github.com/charmbracelet/bubbletea) | Terminal user interface |
| **Browser Automation** | [Rod](https://github.com/go-rod/rod) | Chrome DevTools Protocol |
| **Multi-Lang Parsing** | [Tree-sitter](https://github.com/smacker/go-tree-sitter) | AST parsing for Python, TS, JS, Rust |
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
