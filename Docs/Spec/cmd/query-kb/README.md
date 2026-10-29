# query-kb — Subsystem Overview

> **Location**: `cmd/query-kb/`
> **Spec Status**: 🟢 Complete
> **Last Updated**: 2026-03-07

---

## What Is This?

The `cmd/query-kb` subsystem consists of standalone developer utilities designed to quickly inspect and query codeNERD SQLite knowledge databases (shards). It provides lightweight CLI tools—`main.go` and `deep_query.go`—that perform read-only interactions with `.db` files located in the project's shards directory. Developers use these tools to inspect database schemas, sample `knowledge_atoms` and `vectors`, without booting the entire agent topology or needing to install extensive generic SQLite viewers.

## Quick Facts

| Property | Value |
|----------|-------|
| **Package** | `cmd/query-kb` |
| **Owner** | Core Architecture Team |
| **Maturity** | Internal Developer Utility |
| **Test Coverage** | ~10% (Diagnostic scripts generally lack heavy tests) |
| **Critical Path** | No |

## Key Concepts

* **Standalone Execution**: Designed to be compiled or run strictly via `go run` without invoking the `internal/core` systems, heavily preventing circular dependencies.
* **Database Discovery**: Discovers all databases located inside `.nerd/shards/` recursively when invoked without arguments (via `main.go`).
* **Schema Agnostic**: Operates safely without hard-coded structs for dynamic tables, heavily utilizing `PRAGMA table_info` metadata for structural reads.
* **Deep Queries**: Uses `deep_query.go` for specific structured knowledge interactions like querying explicit concept atoms or vector embedding metadata via limits.

## How to Navigate This Spec

| Document | What You'll Learn |
|----------|------------------|
| [North Star](north-star.md) | Where this subsystem is heading |
| [Current State](current-state.md) | Where it stands today |
| [Gap Analysis](gap-analysis.md) | Delta between vision and reality |
| [Data Flow](data-flow.md) | How data moves through the system |
| [Dependencies](dependencies.md) | What depends on this and vice versa |
| [Wiring](wiring.md) | Integration points and protocols |
| [API Contract](api-contract.md) | What this subsystem promises |
| [Test Strategy](test-strategy.md) | Testing philosophy and coverage |
| [Failure Modes](failure-modes.md) | Known risks and fragilities |
| [Error Taxonomy](error-taxonomy.md) | Error types and propagation |
| [Safety Model](safety-model.md) | Constitutional safety boundaries |
| [Performance Profile](performance-profile.md) | Hot paths and scaling |
| [Observability](observability.md) | Debugging and monitoring |
| [Configuration](configuration.md) | Knobs, defaults, and ranges |
| [Design Decisions](design-decisions.md) | Why things are the way they are |
| [TODOs](todos.md) | Prioritized work items |
| [Glossary](glossary.md) | Subsystem-specific terminology |
