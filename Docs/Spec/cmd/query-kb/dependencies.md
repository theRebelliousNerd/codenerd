# Dependencies

## Overview
The `cmd/query-kb` subsystem explicitly imports standard standard libraries combined with specific single-package external database integrations.

## External Dependencies
* **`modernc.org/sqlite`:**
  - Purpose: CGo-free SQLite database driver.
  - Implementation: Injected via anonymous import `_ "modernc.org/sqlite"` to register against the `database/sql` driver mechanism.
  - Usage: Allows `sql.Open("sqlite", dbPath)` inside `main.go`, `deep_query.go` and `main_test.go` without compiling C headers explicitly, supporting pure Go tooling.

## Internal System Dependencies
* **`database/sql`:**
  - Purpose: Manages standard SQL connections, rows, and schema interaction queries.
  - Implementation: Controls query processing `db.Query(...)`, dynamic row iteration `rows.Next()`, and column evaluation `rows.Columns()`.
* **`sync`:**
  - Purpose: Manages threading models across `.db` iteration.
  - Implementation: `sync.WaitGroup` ensures the primary thread waits via `wg.Wait()` until all processes complete. `sync.Mutex` ensures `os.Stdout.Write` does not interleave log lines concurrently.
* **`io` & `bytes`:**
  - Purpose: Manages output targets for queries to maintain concurrent safety.
  - Implementation: `bytes.Buffer` receives `fmt.Fprintf(w, ...)` string modifications locally inside goroutines before committing to the standard output.
* **`os` & `filepath`:**
  - Purpose: Handles dynamic argument extraction and file processing paths.
  - Implementation: Utilizes `os.Args` for array bounds validation. `filepath.Join` standardizes paths targeting `.nerd`, `shards`, and specific `e.Name()` files.


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
