# Failure Modes

## Overview
The `cmd/query-kb` subsystem represents a relatively fragile diagnostic pathway relying aggressively on OS-level directory evaluations and unstructured SQLite schema allocations, without deep resilience layers mapped across concurrent reads.

## Known Failure Modes

### Failure Mode 1: Directory Access Failure
- **Trigger**: `os.ReadDir(shardsDir)` executes locating fundamentally missing `.nerd/shards/` paths.
- **Symptoms**: The program prints `Error reading shards dir: ...` and halts execution completely. Output vanishes before any SQL queries begin.
- **Impact**: Completely halts database discovery; analysts receive zero analytical insights across the architecture.
- **Likelihood**: occasional
- **Severity**: high
- **Current Mitigation**: none (the binary explicitly executes `os.Exit(1)` directly terminating).
- **Recommended Fix**: Add directory existence checks preceding standard `os.ReadDir()` triggers, or support manual directory flags.

### Failure Mode 2: Dynamic Schema Extraction Panic
- **Trigger**: Executing `rows.Scan(valuePtrs...)` dynamically over SQLite columns carrying deeply nested BLOB or structural data beyond standard mapped scalar types.
- **Symptoms**: Output channel prints `Scan error: ...` repeatedly inside the bounded execution terminal.
- **Impact**: Localizes failures specifically row-by-row parsing skipping valid data chunks via standard `continue` loops.
- **Likelihood**: rare
- **Severity**: low
- **Current Mitigation**: The `err := rows.Scan` conditionally skips explicit malformations logging them directly.
- **Recommended Fix**: Implement explicit type assertions specifically checking string length constraints (`len(s) > 100`) preventing memory panics across `byte` chunks.

### Failure Mode 3: Concurrent SQLite Locking
- **Trigger**: `sql.Open("sqlite", dbPath)` executes against active databases currently actively generating Mangle transactional updates within external ShardAgents.
- **Symptoms**: Fails aggressively generating standard locking warnings `Error opening DB: database is locked`.
- **Impact**: Drops single SQL target evaluations entirely dropping shards from output buffers.
- **Likelihood**: frequent
- **Severity**: medium
- **Current Mitigation**: Localized extraction drop; prevents executing `os.Stdout.Write` completely bypassing the shard cleanly.
- **Recommended Fix**: Provide exponential SQLite connect backoffs isolating concurrent write locks logically.

## Inefficiencies

| Inefficiency | Impact | Optimization Opportunity |
|:---|:---|:---|
| Unbounded `os.ReadDir()` thread initiation | RAM and open-file-descriptor spikes reading massive `.nerd` directories | Concurrently pool goroutines via buffered channel limits |
| Direct string concatenation slicing `s[:100] + "..."` | Garbage collector thrashing over arbitrary arrays | Apply `strings.Builder` optimizations globally |

## Single Points of Failure
The `sync.Mutex` directly locking `os.Stdout.Write` creates a singular monolithic blocking point. If a goroutine manages to lock `mu.Lock()` and panics or hangs unpredictably, the total sequential read output blocks permanently across the entire execution loop awaiting `wg.Wait()`.

## Cascading Failure Risks
Panics triggering inherently inside `queryDB` dynamically map out directly crashing the parent process OS execution, preventing all parallel threads completing standard database processing executions via `wg.Done()`. 

## Resilience Recommendations
1. Isolate the internal testing architecture wrapping `rows.Scan` operations capturing bounded defer-recover closures.
2. Install standard command-line flags passing dynamic paths replacing `.nerd/shards` mapping directory risks strictly away from structural hardcodes.
3. Build locking backoffs mapping SQL execution queries supporting heavily sharded Mangle environments reliably.


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
