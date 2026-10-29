# Observability

## Overview
The observability strategy for `cmd/query-kb` focuses on immediate, terminal-bound visibility of the SQLite extraction process without integrating external telemetry systems like OpenTelemetry or Prometheus.

## Logging Implementation
* **Standard Output Formatting:** 
  - The primary function `queryDB` writes diagnostic state logs directly to an `io.Writer`.
  - In concurrent execution triggered exclusively by `len(os.Args) < 2`, the isolated buffer logs prepend execution environments via `fmt.Fprintf(&buf, "\n=== %s ===\n", e.Name())`.
  - In targeted execution, logs push dynamically to `os.Stdout`.
* **Execution Trace:** 
  - Standard outputs trace exactly 6 steps:
    1. Table array discovery (`Tables: %v`).
    2. Missing `knowledge_atoms` schema detection (`No knowledge_atoms table`).
    3. Schema column definitions extraction (`- %s (%s)`).
    4. Target columns extraction limits (`Columns: %v`).
    5. Scalar table total counts (`Total knowledge_atoms: %d`).
    6. Expected embedded metric counts (`Total vectors: %d`).

## Error Visibility
* Error conditions directly abort localized functions but trace errors verbosely leveraging `%v`.
* Example implementations checking `sql.Open`, `db.Query`, and `rows.Scan` failures expose raw SQLite string outputs explicitly formatted utilizing `fmt.Fprintf(w, "Error ... : %v\n", err)`. 

## Metrics & Tracing
* **Metrics:** 
  - Only raw quantitative properties are calculated, such as string boundaries applying `len(s) > 100`.
  - Metric aggregations rely exclusively on standard SQL functions like `COUNT(*)` over `vectors` executing directly via `db.QueryRow`.
* **Tracing:** 
  - Deep query logic tracks active loop cycles via standard mutable variables `count++`. When execution constraints cross bounds (`count == 51`), it prints explicit context tracing the omitted vector records (`... and %d more vectors`).


## Source File References

The behavior documented above strictly grounds itself in the logic found within `main.go`. Additionally, deep extraction queries are evaluated based on functions inside `deep_query.go` and verified by `main_test.go`.

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
