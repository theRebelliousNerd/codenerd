# 05 — Internal Architecture: `internal/types`

> Last verified: **2026-07-13**

## 1. Component map

```
internal/types
├── [Conversion]   Fact / MangleAtom / ToAtom / String
├── [Extract]      Extract* / Arg* / StripAtomPrefix
├── [Kernel API]   Kernel · KernelInterface · KernelFact
├── [Transactions] KernelTransactor · KernelTransaction · KernelTx
├── [LLM API]      LLMClient · tools/messages · optional caps
├── [Shards]       ShardAgent · ShardConfig · enums · priorities
├── [Blackboard]   SessionContext · summaries · intent
└── [Markers]      VirtualStore · GraphQuery · LearningStore · …
```

There is **no runtime daemon** and almost no mutable package state. The only package-level vars are the private session context key.

## 2. Data flow: assert path

```
Caller builds types.Fact{Predicate, Args}
        │
        ▼
  Fact.ToAtom()
        │  per-arg type switch
        │  ─ MangleAtom / string / numbers / time / bool
        │  ─ containers → json.Marshal → ast.String
        │  ─ nil / poison → error
        ▼
  ast.Atom
        │
        ▼
  Kernel store / engine (outside this package)
```

## 3. Data flow: query path

```
Kernel.Query(predicate) → []types.Fact
        │
        ▼
  ArgString / ExtractBool / …
        │
        ▼
  Go control logic (articulation, campaign, VirtualStore routing)
```

Mangle constant → Go conversion happens in the engine (`constantToInterface` referenced by extract comments); extract helpers assume the post-conversion type set documented in `extract.go`.

## 4. Data flow: session blackboard

```
Chat / SessionExecutor populates SessionContext
        │
        ├─► WithSessionContext(ctx, sc)   // std context
        ├─► ShardAgent.SetSessionContext(sc)
        └─► PromptAssembler PromptContext.SessionCtx
```

Dream mode: `SessionContext.DreamMode == true` signals shards to describe, not execute (enforced by consumers).

## 5. Data flow: transactional EDB update

```
k := types.Kernel (must implement KernelTransactor)
tx := types.NewKernelTx(k)   // panics if not
tx.Retract("pred")
tx.Assert(Fact{...})
tx.Commit()                  // single rebuild
```

Used by campaign fact sync, context pager, TDD loop, etc.

## 6. Key type relationships

```
ShardConfig ──permissions──► ShardPermission
     │
     ├── Type: ShardType
     ├── Model: ModelConfig (ModelCapability)
     ├── Policy: string (Mangle fragment for AppendPolicy)
     └── SessionContext *

ShardAgent ──uses──► Kernel, LLMClient, SessionContext

LLMClient ◄──optional── ToolResultsProvider
         ◄──optional── GroundingController ⊃ GroundingProvider
         ◄──optional── ThinkingProvider, ThoughtSignatureProvider
         ◄──optional── PiggybackToolProvider
         ◄──optional── FileProvider, CacheProvider, TokenCounter

Kernel ◄──optional── KernelTransactor ──► KernelTransaction
KernelInterface uses KernelFact ──ToFact()──► Fact
```

## 7. State machines

### Shard state (data only)

```
idle → running → completed
              ↘ failed
```

Transitions are owned by shard manager / agents, not by `types`.

### Spawn priority (scheduling hint)

```
PriorityLow (0) < Normal (1) < High (2) < Critical (3)
```

Stringer for logs: `low|normal|high|critical|unknown`.

## 8. Conversion edge cases (architecture-relevant)

| Input | Architectural intent |
|-------|----------------------|
| Hierarchical atom `/a/b` | Allowed (≤2 slashes) |
| Deep path `/a/b/c/d` | Treated as non-atom (likely FS path) |
| `//` in name | Invalid |
| File extension | Force string path interpretation |
| JSON containers | Opaque payload in pending_action / virtual_store style facts |
| Unexported-only struct | Fails marshal / empty → error |

## 9. Concurrency model

- Package is stateless aside from context values.
- `SessionContext` is a plain struct — **no mutex**; callers must treat as immutable snapshot or own synchronization when sharing across goroutines.
- `KernelTx` is not documented as concurrent-safe; one tx per sequential mutation sequence.

## 10. Extension points

| Extension | How |
|-----------|-----|
| New LLM capability | New optional interface + consumer assertion |
| New shard permission | Add `ShardPermission` const + policy Decl elsewhere |
| New SessionContext section | Add fields + populate at boot/session + prompt assembler |
| New VirtualStore method | Add to interface only with real multi-package need |
| New extract type | Add ExtractX + tests mirroring engine conversion |
