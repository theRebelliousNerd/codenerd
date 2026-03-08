# Safety Model

## Overview
The safety model inside the `cmd/query-kb` subsystem addresses two core domains: concurrent terminal buffer synchronization and dynamic unstructured data parsing from external `sqlite` sources.

## Concurrent Data Writing Safety
* **Context:** Running `main()` dynamically launches arbitrary numbers of goroutines across `filepath.Ext(entry.Name()) == ".db"` files.
* **Threat:** Attempting to `fmt.Printf` or `os.Stdout.Write` concurrently corrupts terminal lines combining characters unpredictably.
* **Mitigation:** 
  - Thread creation utilizes `wg.Add(1)` ensuring the OS process won't exit prematurely.
  - The actual writer injected into `queryDB` is a locally scoped `bytes.Buffer`.
  - Terminal string delivery commits atomically relying on a global thread locker initialized as `var mu sync.Mutex` inside the primary loop to enforce serial output extraction.

## Memory Bounding
* **Context:** The `vectors` and `knowledge_atoms` tables contain arbitrary massive embedding sets or extensive text bodies. Reading these entirely memory-bombs the system executing the process.
* **Mitigation:**
  - Standard output is permanently constrained by `limit = 10` or `5`. Deep query constraints lock printing at `count <= 50`.
  - Character memory constraints dynamically evaluate extracted interface values formatting long sequences using `s[:100] + "..."`.

## Read-Only Driver Constraints
* **Context:** Accidental mutations to production databases from diagnostic tools.
* **Mitigation:** The application exclusively relies on `SELECT` queries (`SELECT * FROM`, `SELECT name FROM sqlite_master`). The query model guarantees diagnostic operations never alter the Mangle atomic states stored dynamically underneath `.nerd/shards/`. The only structural mutations (`INSERT INTO`) occur exclusively isolated within ephemeral paths (`t.TempDir()`) generated solely during `main_test.go` execution.


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
