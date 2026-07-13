# world — Failure Modes

> Last verified: **2026-07-13**

## FM1 — Stale hash / facts after same-second rewrite

| | |
|--|--|
| **Cause** | Historical second-resolution mtime |
| **Mitigation** | UnixNano in FileCache + fingerprints (current code) |
| **Residual** | Filesystems with coarse timestamps still rare-collide |
| **Detect** | Facts disagree with disk; re-scan |

## FM2 — Absolute vs relative path split-brain

| | |
|--|--|
| **Cause** | Full scan canonicalizes; incremental/dir paths may stay absolute |
| **Effect** | Duplicate topology; policy matches fail; restore broken |
| **Mitigation** | Prefer full rescan; fix emitters to shared `canonicalScanPath` |
| **Detect** | Query shows both `internal/foo.go` and `C:\...\internal\foo.go` |

## FM3 — Unbounded goroutines on huge trees

| | |
|--|--|
| **Cause** | Spawning workers without backpressure (fixed) |
| **Mitigation** | Semaphore acquire **before** spawn |
| **Detect** | Memory spike during scan |

## FM4 — Parse poison file

| | |
|--|--|
| **Cause** | Malformed source, tree-sitter error, huge file |
| **Effect** | Missing symbols for that file |
| **Mitigation** | Warn + continue; size gates |
| **Detect** | WorldWarn parse failed lines |

## FM5 — Full replace leaves orphan predicates

| | |
|--|--|
| **Cause** | `WorldPredicates` incomplete |
| **Effect** | Ghost `entry_point` / CodeDOM / git facts after Full apply |
| **Mitigation** | Extend set or explicit retract |
| **Detect** | Kernel query stale entry points after rescan |

## FM6 — Deep facts only for Go

| | |
|--|--|
| **Cause** | `EnsureDeepFacts` / `MapFile` filter |
| **Effect** | Python/TS projects lack `code_defines` impact edges |
| **Mitigation** | Fall back to `symbol_graph`; multi-lang MapFile work |
| **Detect** | Impact empty for non-Go campaign targets |

## FM7 — Cache corruption

| | |
|--|--|
| **Cause** | Corrupt `manifest.json` |
| **Mitigation** | Load starts fresh on unmarshal error |
| **Effect** | One-time full rehash cost |

## FM8 — Git missing / not a repo

| | |
|--|--|
| **Cause** | No git binary or non-repo root |
| **Mitigation** | Soft skip (nil facts) |
| **Effect** | No churn history facts |

## FM9 — Holographic package explosion

| | |
|--|--|
| **Cause** | Giant package directory |
| **Mitigation** | Cap 100 files parsed |
| **Effect** | Incomplete sibling signatures |

## FM10 — Impact query absent

| | |
|--|--|
| **Cause** | Kernel nil or no priority rules loaded |
| **Mitigation** | Graceful return standard context |
| **Effect** | Review prompts less targeted |

## FM11 — LocalStore write failures

| | |
|--|--|
| **Cause** | Disk full / locked DB |
| **Mitigation** | WorldWarn; in-memory facts may still load |
| **Effect** | Next incremental cannot retract old accurately |

## FM12 — Dual writer race (shard vs chat)

| | |
|--|--|
| **Cause** | WorldModelIngestor + chat ApplyIncremental concurrent |
| **Effect** | Interleaved retract/load; transient inconsistency |
| **Mitigation** | Operational: don’t dual-run without coordination |
| **Detect** | Flapping fact counts |

## FM13 — Import cycle regression

| | |
|--|--|
| **Cause** | Someone imports world from core or vice versa wrongly |
| **Effect** | Build break |
| **Mitigation** | types aliases + system bridge pattern |

## FM14 — Tree-sitter / CGO platform issues

| | |
|--|--|
| **Cause** | Grammar/native build failures |
| **Effect** | Multi-lang parse broken |
| **Mitigation** | Go path uses go/ast for CodeDOM/Cartographer independently for Go |

## Recovery checklist

1. Delete `.nerd/cache/manifest.json` and re-run full scan.  
2. Clear LocalStore world facts tables if available.  
3. `RemoveFactsByPredicateSet(WorldPredicateSet())` then full LoadFacts.  
4. Confirm path form with a sample `file_topology` query.  
5. For deep: re-open scope / EnsureDeepFacts on target Go files.
