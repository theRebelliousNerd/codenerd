# codeNERD Architecture Map

## High-Level Structure

- **`/cmd`**: CLI entry points.
  - `/nerd`: The main `nerd` CLI binary.
  - `/query-kb`: Tool for querying the knowledge base.
  - `/verify_antigravity`: Verification tool for auth.
- **`/internal`**: Core application logic (Private Library).
  - `/mangle`: The logic engine (Datalog/Mangle implementation).
  - `/core`: The Kernel, FactStore, and VirtualStore.
  - `/shards`: The Shard Agents (Coder, Tester, Reviewer, etc.).
  - `/campaign`: Orchestration of long-running goals.
  - `/autopoiesis`: Self-learning and Ouroboros loop.
  - `/perception`: LLM Input Processing (Transducer).
  - `/articulation`: LLM Output Processing (Piggyback).
  - `/world`: Filesystem and AST projection.
- **`/.nerd`**: Runtime state (The "Brain" on disk).
  - `/logs`: Execution logs.
  - `/mangle`: Persisted Mangle facts (`learned.mg`, `scan.mg`).
  - `/config.json`: User configuration.
  - `/shards`: Shard-specific data.
- **`/.agent`**: AI Agent resources (Skills, Rules).

## Key Components

### The Kernel (`internal/core`)
The central nervous system. It holds the `FactStore` (RAM) and runs the `Mangle` engine to derive `next_action`.

### Shards (`internal/shards`)
Specialized agents that perform tasks.
- **CoderShard**: Writes code (`/coder`).
- **TesterShard**: Runs tests (`/tester`).
- **ReviewerShard**: Audits code (`/reviewer`).
- **ResearcherShard**: Gathers info (`/researcher`).

### Campaign Orchestrator (`internal/campaign`)
Manages multi-phase goals (`/greenfield`, `/migration`) and Context Paging.

### Autopoiesis (`internal/autopoiesis`)
The self-improvement loop.
- **PanicMaker**: Fuzz testing.
- **Learner**: Rule induction.
- **Ouroboros**: Tool generation.

## Data Flow

1. **Input**: User text -> `Perception Transducer` -> `user_intent` atoms.
2. **Processing**: `Kernel` + `Mangle` derive `next_action`.
3. **Execution**: `VirtualStore` dispatches to `Shards` or Tools.
4. **Output**: `Articulation Transducer` -> User text + `control_packet`.
