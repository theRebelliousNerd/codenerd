# Gap Analysis

## Spec vs Reality Matrix

| Feature / Capability | Spec'd In | Implemented? | Notes |
|:---|:---|:---|:---|
| Concurrent Output Safety | safety-model.md | âœ… Yes | Uses `bytes.Buffer` and `sync.Mutex` |
| Deep Query Logic | api-contract.md | âœ… Yes | The `deep_query.go` file handles this |
| Flag Parsing System | configuration.md | âŒ No | The code uses hardcoded `os.Args` indexes |
| Exponential Backoff | failure-modes.md | âŒ No | Queries fail on `SQLITE_BUSY` errors |

## Built But Not Spec'd
* The `deep_query.go` script multiplies the `confidence` score by 100 to display percentage values. This math logic lacks documentation.
* The `main_test.go` script uses `t.TempDir()` to build isolated test databases.

## Spec'd But Not Built
* The system does not use the standard `flag` package. The arguments rely on length checks via `len(os.Args)`.
* Connection retry logic is missing. SQLite databases encounter lock states when interacting with the main Mangle engine, but the tool provides no retry loops.

## Partially Implemented

| Feature | What Works | What's Missing |
|:---|:---|:---|
| Concurrent Executions | The `wg.Add(1)` function tracks goroutines | The system spawns one thread per file without a cap |
| Dynamic Schema Scans | The code uses `make([]interface{}, len(cols))` | It only handles string truncation via `s[:100] + "..."` |

## North-Star Alignment Map

| Gap | Category | North-Star Goal Blocked | Impact on Goal |
|:---|:---|:---|:---|
| Configuration Flexibility | Spec'd But Not Built | Goal 3: Zero-Dependency Architecture | Blocks flag integrations |
| Deep Query Flag Validation | Built But Not Spec'd | Goal 2: Bounded Vector Inspection | Degrades test coverage on vector truncation |
| Unbounded Goroutines | Partially Implemented | Goal 1: Scalable Shard Discovery | Blocks memory scaling on large directory paths |

## Priority Assessment

### Critical
* **No explicit flag usage limits target flexibility:** The tool reads `os.Args[1]` blindly. This approach prevents users from specifying output limits. (Relates to Goal 1: Scalable Shard Discovery)
* **Unbounded Goroutines:** The `os.ReadDir` loop creates threads for every database file. This design causes memory bottlenecks during the `mu.Lock()` step. (Relates to Goal 1: Scalable Shard Discovery)

### Important
* **No locked database backoffs:** Diagnostic processes fail when reading shards locked by the Mangle engine. (Relates to Goal 3: Zero-Dependency Architecture)
* **Missing testing for deep query logic:** The `deep_query.go` file operates without test functions in `main_test.go`. (Relates to Goal 2: Bounded Vector Inspection)

### Nice-to-Have
* **Structured color-coded CLI logging:** Terminal outputs look uniform and lack contrast for visual debugging. (Relates to Goal 2: Bounded Vector Inspection)

## Recommendations
1. **Immediate actions** â€” Replace the `os.Args` checks with the `flag` package in `main.go`. This closes the Configuration Flexibility gap.
2. **Short-term improvements** â€” Add a worker pool to limit goroutines spawned by the `os.ReadDir` loop. This change advances Goal 1.
3. **Strategic initiatives** â€” Merge the `deep_query.go` code into `main.go` using subcommands. This step aligns with the Phase 2 Roadmap.


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
