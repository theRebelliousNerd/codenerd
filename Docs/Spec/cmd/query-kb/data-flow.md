# Data Flow

## Overview
Data flow within the `cmd/query-kb` subsystem represents the extraction of binary database SQLite files into structured, human-readable terminal output.

## Primary Flow (`main.go`)

### 1. File Discovery & Initialization
* The process initiates at `main()`. Depending on `len(os.Args) < 2`, it lists directories reading all `.db` files from `.nerd/shards/` via `os.ReadDir()`.
* For a specific file execution, it grabs the argument from `os.Args[1]`.

### 2. Thread Coordination
* In concurrent fallback mode, the parent process injects work into Goroutines using `wg.Add(1)`.
* Each goroutine executes `queryDB(dbPath, 5, &buf)`, funneling the string results into an internal `bytes.Buffer`.
* Buffer dumping to global standard output is isolated by a `sync.Mutex` lock to prevent overlapping write outputs to the terminal.

### 3. Database Execution
* Connection via `sql.Open("sqlite", dbPath)` from the `modernc.org/sqlite` package.
* Pulls schema information, extracting `sqlite_master` row by row.
* Identifies column definitions for the `knowledge_atoms` table running `PRAGMA table_info`.
* Constructs a dynamic value array `valuePtrs` pointing to empty `interface{}` to handle an undetermined column schema for the table.
* Scans rows iteratively with `rows.Next()`. Values exceeding 100 characters in string interpretation get sliced via `s[:100] + "..."`.

### 4. Vector Extraction
* Issues a `COUNT(*)` to quantify raw metrics of `vectors`.
* Queries up to 5 embedded values specifically asking for `id`, `content`, and `metadata`.

## Secondary Flow (`deep_query.go`)
* Direct parameter parsing evaluates `--vectors` and `--atoms` flags.
* Connects similarly via SQLite, but queries explicitly typed scalar columns (`id`, `concept`, `content`, `confidence`).
* Multiplies the `confidence` float by `100` before outputting formatted records explicitly to `os.Stdout`. Limit execution caps printing at 50 loops while tracking remaining counts.


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
