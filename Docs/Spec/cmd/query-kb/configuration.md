# Configuration Options

## Overview
The `cmd/query-kb` subsystem accepts minimal runtime configuration, avoiding dedicated YAML or JSON configuration files. All configurations are passed directly as command-line arguments to the `main.go` and `deep_query.go` binaries.

## `main.go` Configuration

### Execution Modes
1. **Interactive Database Discovery:**
   - Input logic: triggered automatically when `len(os.Args) < 2`.
   - Behavior: Hardcoded parameter `shardsDir` path resolves to `.nerd/shards`.
   - Limit Settings: The internal `limit` passed to `queryDB` is hardcoded strictly to `5` iterations.
2. **Specific Target File Validation:**
   - Input logic: Triggered when an argument passes as `os.Args[1]`.
   - Scope Parameters: Target `dbPath` resolves directly to the first positional argument.
   - Limit Settings: Limits processing strictly to `10` iterations per query output.

## `deep_query.go` Configuration

### Flag Settings
This secondary tool manages output filtering based on specific string argument flags configured linearly without an explicit flag parser package.

#### Toggle Flags
* `--vectors`: Skips processing of SQL outputs for `knowledge_atoms`. Mutates the internal toggle boolean `showAtoms = false`.
* `--atoms`: Skips processing of SQL outputs for `vectors`. Mutates the internal toggle boolean `showVectors = false`.
* The tool requires explicitly defining an SQLite path target strictly located at `os.Args[1]`.

## Internal Dependencies
Configuration fundamentally connects to standard driver libraries:
* The standard DB string `sqlite` connects specifically into the `modernc.org/sqlite` package, rather than the CGo integrated `mattn/go-sqlite3` driver. This allows independent compilation without C compiler dependencies in CI execution.
* The system is explicitly configured for Windows systems in output terminal parsing environments, capturing raw output with the `fmt.Fprintf` interface without leveraging external coloring libraries.


## Extended Documentation Notes

The above specification details represent the functional baseline for the system. Code definitions in `main.go` establish the connection loops.
The sub-routines located in `deep_query.go` handle the iterative data parsing.
Validation checks rely on testing logic found inside `main_test.go` to ensure correctness.
This subsystem primarily reads from SQLite schemas without modifying any core tables directly.
Goroutine synchronization guarantees that standard output remains ungarbled across threads.
Database locks encountered during execution indicate activity from concurrent agents.
We maintain a strict zero CGo dependency policy for maximal cross-platform compatibility.
Future iterations will refine the command-line flags to offer structured output caps.

These constraints limit memory explosion when querying tables that store gigabytes of embedded vectors.
Overall, the query tools function as diagnostic read-only interfaces rather than state mutators.
