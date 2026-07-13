# world — Internal Architecture

> Last verified: **2026-07-13**

## Component diagram

```
                    ┌──────────────┐
                    │ ScannerConfig│
                    └──────┬───────┘
                           │
              ┌────────────▼────────────┐
              │        Scanner          │
              │  pool: TreeSitterParser │
              └───┬──────────────┬──────┘
                  │              │
         ScanDirectory   ScanWorkspaceIncremental
                  │              │
          ┌───────▼──────┐  ┌───▼────────┐
          │  FileCache   │  │ LocalStore │
          └──────────────┘  └───┬────────┘
                  │              │
                  ▼              ▼
            file_topology   ApplyIncrementalResult
            symbol_graph ─────────────► Kernel
                  │
                  │ (on demand)
                  ▼
           EnsureDeepFacts ──► Cartographer ──► DataFlow*
                  │
                  ▼
              code_defines / code_calls / assigns / …

  ParserFactory ──► Go/Py/TS/RS/Mangle parsers ──► CodeElement
        ▲
        │
    FileScope.Open ──► active_file / file_in_scope / code_element
        │
        ▼
  HolographicCodeScope (system) keeps deep facts synced

  HolographicProvider.GetContext ──► package X-ray structs
        │
        └── BuildWithImpactPriorities ──► kernel context_priority_file

  lsp.Manager ──► symbol_defined / symbol_referenced / code_diagnostic
```

## Data-flow: full scan

```
root
  → Walk (skip ignore/hidden)
  → per file (sem):
       cache.Get(path, info)?
         hit  → hash
         miss → SHA256 → cache.Update
       emit file_topology
       if !test && size ok:
         TreeSitterParser.Parse* → symbol_graph
  → aggregate Facts
  → optional PersistFastSnapshotToDB
  → caller LoadFacts / ApplyIncremental Full
```

## Data-flow: incremental scan

```
load FileCache entries
walk → currentFiles set + dirFacts
if no prev → full scan branch (Full=true)
else diff changed/new/deleted
  load old fast facts from DB → RetractFacts
  reparse only changed/new
  delete DB rows for deleted
  return delta NewFacts (includes refreshed directory facts)
ApplyIncrementalResult:
  retract exact + Retract("directory") + LoadFacts
```

## Data-flow: deep / scope

```
Open(file):
  FileScope loads package + 1-hop imports (Go)
  parse CodeElements
  emit scope facts via callback
HolographicCodeScope.ensureDeepFacts(inScope):
  EnsureDeepFacts(go files)
  retract cached deep + load new into kernel
```

## State machines

### FileCache entry

```
absent ──Update──► present(hash, nanoMtime, size)
present ──Get mismatch──► treat as miss → rehash → Update
present ──Get match──► reuse hash
Dirty ──Save──► disk JSON, Dirty=false
```

### IncrementalResult modes

| Mode | Flags | Kernel apply |
|------|-------|--------------|
| Bootstrap | `Full=true` | Replace WorldPredicateSet |
| Delta | `Full=false`, facts nonempty | Exact retract + directory retract + load |
| Idle | `Unchanged=true` | No-op |

### FileScope lifecycle

```
NewFileScope → Open(path) → InScope+Elements populated → Refresh?* → Close/Open other
```

## Key type clusters

| Cluster | Types |
|---------|-------|
| Scan | `Scanner`, `ScannerConfig`, `ScanResult`, `IncrementalOptions`, `IncrementalResult`, `DeepResult` |
| Cache | `FileCache`, `CacheEntry`, `DataFlowCache`, `CacheStats` |
| Graph | `Cartographer`, `DataFlowExtractor`, `MultiLangDataFlowExtractor` |
| CodeDOM | `CodeParser`, `ParserFactory`, `CodeElement`, `CodeElementParser`, `ParseResult` |
| Scope | `FileScope`, `EncodingInfo`, `FileLoadResult` |
| Agent | `HolographicProvider`, `HolographicContext`, `PrioritizedCaller`, … |
| Meta | `WorldPredicates`, Fact aliases |

## Control points for operators

| Knob | Location |
|------|----------|
| Workers | `ScannerConfig.MaxConcurrency` / features FastScanWorkers |
| AST size | `MaxASTFileBytes` / FastASTMaxBytes |
| Ignore | `IgnorePatterns` |
| Deep workers | `EnsureDeepFacts` workers arg / HolographicCodeScope |
| Impact caps | `maxPrioritizedCallers=10`, `maxCallerBodyLines=50` |
| Package parse cap | `maxPackageFilesToParse=100` in holographic |
