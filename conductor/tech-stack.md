# Technology Stack: codeNERD

## Core Language & Runtime
- **Go (1.24.0):** Primary programming language, chosen for its strong concurrency model (CSP), performance, and static typing.

## Logic & Reasoning
- **Google Mangle:** A Datalog-based logic engine used as the deterministic "Executive" for the system. It handles planning, memory, and orchestration.

## Interface & Interaction
- **Cobra:** CLI framework for command routing and argument parsing.
- **Bubble Tea (Charm):** TUI framework for building the interactive chat interface and progress visualization.

## Persistence & Memory
- **SQLite (ModernC):** Primary local database for shard knowledge, learned patterns, and system state.
- **sqlite-vec:** Used for vector embeddings to enable semantic search and JIT prompt compilation.
- **ArangoDB:** Graph database used for long-term relational knowledge and complex dependency mapping.

## Automation & Tools
- **Rod:** Browser automation framework for headless Chrome interactions and DOM analysis.
- **Tree-sitter:** Multi-language AST parsing for advanced data flow analysis across Go, Python, TS, JS, and Rust.
