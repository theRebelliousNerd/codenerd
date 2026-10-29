# API Contract

## Overview
This document specifies the command-line interface for the `cmd/query-kb` subsystem. The program is executed via the command line and accesses SQLite database files.

## Primary CLI Commands

### Default Execution
* **Command:** `query-kb` (no arguments)
* **Function (`main.go`):** Executes `main()` which automatically targets `.nerd/shards/` directory.
* **Output:** Outputs to `os.Stdout` protected by `sync.Mutex` printing the database schema, table lists, and up to 5 sampled rows from `knowledge_atoms` and `vectors` for every `.db` file found.

### Targeted Execution
* **Command:** `query-kb <database.db>`
* **Parameters:** `os.Args[1]` takes the specific database path.
* **Function (`main.go`):** Executes `queryDB(dbPath, 10, os.Stdout)`, which increases the row limit to 10 for a single target database.

### Deep Query Utility
The `deep_query.go` script provides an advanced API.
* **Command:** `go run deep_query.go <database.db> [--vectors|--atoms]`
* **Parameters:**
  - `<database.db>`: SQLite database path.
  - `--vectors`: Skips printing `knowledge_atoms`.
  - `--atoms`: Skips printing `vectors`.

## Database Schema Dependencies
The API relies strictly on SQLite schema conventions:
* Checks for table names via `SELECT name FROM sqlite_master WHERE type='table'`
* Retrieves column names strictly via `PRAGMA table_info(knowledge_atoms)`
* Relies on specific table names: `knowledge_atoms` and `vectors`.

## Output Formats
* **`main.go`:** Uses `fmt.Fprintf(w, ...)` into an `io.Writer`. Columns are dynamically scanned into an array of `interface{}` pointers. String values over 100 characters are truncated with `...`.
* **`deep_query.go`:** Prints to `os.Stdout`, formats `knowledge_atoms` showing `id`, `concept`, `content`, and calculates `confidence * 100`. Long vector content is printed up to 50 rows, after which remaining vectors are counted but omitted from the print output.


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
