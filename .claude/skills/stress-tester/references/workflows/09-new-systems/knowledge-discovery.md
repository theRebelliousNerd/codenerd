# Knowledge Discovery System Stress Test

Stress test for the LLM-First Knowledge Discovery and Semantic Knowledge Bridge.

## Overview

Tests the Knowledge Discovery System's handling of:

- Semantic Knowledge Bridge integration with JIT compiler
- Document ingestion via `/refresh-docs`
- Knowledge atom extraction and storage
- Vector embedding generation and search
- Shard-specific knowledge filtering
- Cross-session knowledge persistence

**Expected Duration:** 20-35 minutes total

### Key Files

- `internal/articulation/prompt_assembler.go` - Semantic Knowledge Bridge
- `internal/shards/researcher/researcher.go` - Document ingestion
- `internal/store/local.go` - Knowledge atom persistence
- `internal/embedding/` - Vector operations
- `internal/retrieval/` - Semantic retrieval

### Storage

- Agent knowledge: `.nerd/shards/{agent}_knowledge.db`
- Shared corpus: `.nerd/prompts/corpus.db`
- Vector embeddings: Integrated in SQLite with sqlite-vec

---

## Conservative Test (6-10 min)

Test basic knowledge discovery and retrieval.

### Step 1: Verify Knowledge Store (wait 2 min)

```bash
./nerd.exe status
```

Check knowledge store initialization:

```bash
Select-String -Path ".nerd/logs/*store*.log" -Pattern "knowledge|corpus|embedding"
Get-ChildItem .nerd/prompts/*.db
Get-ChildItem .nerd/shards/*_knowledge.db -ErrorAction SilentlyContinue
```

### Step 2: Document Ingestion (wait 4 min)

Ingest documents:

```bash
./nerd.exe refresh-docs
```

Monitor ingestion:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "refresh-docs|ingesting|markdown|atom"
```

### Step 3: Verify Atom Storage (wait 2 min)

Check atoms were stored:

```bash
./nerd.exe query "knowledge_atom"
Select-String -Path ".nerd/logs/*store*.log" -Pattern "atom stored|knowledge insert"
```

### Step 4: Knowledge Retrieval (wait 2 min)

Trigger knowledge-aware task:

```bash
./nerd.exe spawn coder "explain the JIT prompt compiler"
```

Check knowledge was retrieved:

```bash
Select-String -Path ".nerd/logs/*jit*.log" -Pattern "knowledge bridge|semantic search|retrieved atom"
```

### Success Criteria

- [ ] Knowledge store initialized
- [ ] Documents ingested successfully
- [ ] Knowledge atoms stored
- [ ] Semantic retrieval functional

---

## Aggressive Test (8-12 min)

Push knowledge system with volume and complexity.

### Step 1: Clear Logs (wait 1 min)

```bash
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Large-Scale Ingestion (wait 5 min)

Ingest all documentation:

```bash
./nerd.exe refresh-docs --recursive
```

Monitor for:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "ingesting|processed|atom count"
```

### Step 3: High-Volume Retrieval (wait 4 min)

Spawn multiple shards using knowledge:

```bash
Start-Job { ./nerd.exe spawn coder "implement kernel query optimization" }
Start-Job { ./nerd.exe spawn reviewer "review Mangle integration" }
Start-Job { ./nerd.exe spawn researcher "research spreading activation" }

Get-Job | Wait-Job -Timeout 300
Get-Job | Receive-Job -ErrorAction SilentlyContinue
Get-Job | Remove-Job
```

### Step 4: Verify Shard-Specific Filtering (wait 2 min)

Check different knowledge for different shards:

```bash
Select-String -Path ".nerd/logs/*jit*.log" -Pattern "shard_type|filtered for"
```

### Step 5: Embedding Search Performance (wait 2 min)

Check vector search timing:

```bash
Select-String -Path ".nerd/logs/*embedding*.log" -Pattern "search time|vector query|top-k"
```

### Success Criteria

- [ ] Large ingestion completed
- [ ] Multiple shards retrieved knowledge
- [ ] Shard-specific filtering applied
- [ ] Vector search performant (<1s)

---

## Chaos Test (10-15 min)

Stress test with edge cases and failures.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Concurrent Ingestion (wait 5 min)

Attempt concurrent document refresh:

```bash
Start-Job { ./nerd.exe refresh-docs }
Start-Sleep 2
Start-Job { ./nerd.exe refresh-docs }

Get-Job | Wait-Job -Timeout 300
Get-Job | Receive-Job -ErrorAction SilentlyContinue
Get-Job | Remove-Job
```

Check for database contention:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "lock|contention|SQLITE_BUSY"
```

### Step 3: Malformed Document Handling (wait 3 min)

Create malformed test document:

```bash
echo "# Test Document" > .nerd/test_malformed.md
echo "```" >> .nerd/test_malformed.md  # Unclosed code block
./nerd.exe refresh-docs
```

Check error handling:

```bash
Select-String -Path ".nerd/logs/*.log" -Pattern "parse error|malformed|skip"
```

### Step 4: Empty Knowledge Retrieval (wait 3 min)

Query with no matching knowledge:

```bash
./nerd.exe spawn coder "implement quantum computing algorithms"
```

Check graceful fallback:

```bash
Select-String -Path ".nerd/logs/*jit*.log" -Pattern "no matching|fallback|empty result"
```

### Step 5: Embedding Dimension Mismatch (wait 2 min)

Check embedding consistency:

```bash
Select-String -Path ".nerd/logs/*embedding*.log" -Pattern "dimension|mismatch|vector size"
```

### Success Criteria

- [ ] Concurrent ingestion handled
- [ ] Malformed documents skipped gracefully
- [ ] Empty retrieval has fallback
- [ ] No embedding dimension errors

---

## Hybrid Test (10-12 min)

Test knowledge integration with full system.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Knowledge-Enhanced Campaign (wait 6 min)

Run campaign using discovered knowledge:

```bash
./nerd.exe campaign start "extend the JIT compiler with new atom selection"
```

Monitor knowledge usage:

```bash
Select-String -Path ".nerd/logs/*campaign*.log" -Pattern "knowledge|context atom|semantic"
```

### Step 3: Cross-Shard Knowledge Sharing (wait 3 min)

Check knowledge shared across shards:

```bash
./nerd.exe query "shard_knowledge_access"
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "shared knowledge|cross-shard"
```

### Step 4: Knowledge Evolution Integration (wait 3 min)

Check integration with prompt evolution:

```bash
Select-String -Path ".nerd/logs/*autopoiesis*.log" -Pattern "knowledge atom|evolved from|strategy"
```

### Success Criteria

- [ ] Campaign used knowledge effectively
- [ ] Knowledge shared across shards
- [ ] Evolution system integrated
- [ ] Persistent across session

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Knowledge-Specific Queries

```bash
# Count knowledge atoms
./nerd.exe query "knowledge_atom" | Measure-Object -Line

# Check embedding database size
Get-Item .nerd/prompts/corpus.db | Select-Object Length

# Find retrieval events
Select-String -Path ".nerd/logs/*.log" -Pattern "retrieved.*atom" | Measure-Object

# Check LLM filtering usage
Select-String -Path ".nerd/logs/*.log" -Pattern "LLM filter|relevance score"
```

### Vector Search Analysis

```bash
# Check search latency
Select-String -Path ".nerd/logs/*embedding*.log" -Pattern "search.*ms" |
    ForEach-Object { $_.Line -match "(\d+)ms" | Out-Null; [int]$matches[1] } |
    Measure-Object -Average -Maximum
```

### Success Metrics

| Metric | Conservative | Aggressive | Chaos | Hybrid |
|--------|--------------|------------|-------|--------|
| Panics | 0 | 0 | 0 | 0 |
| Ingestion failures | 0 | <5% | <10% | <5% |
| Retrieval failures | 0 | 0 | <5% | 0 |
| Avg search time | <500ms | <1s | <2s | <1s |
| DB locks | 0 | 0 | <3 | 0 |

---

## Known Issues to Watch For

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| Embedding mismatch | Search fails | Dimension changed | Regenerate embeddings |
| DB lock | SQLITE_BUSY | Concurrent writes | Use WAL mode |
| Memory overflow | OOM on ingestion | Too many docs | Batch ingestion |
| Stale knowledge | Old atoms retrieved | No refresh | Schedule refresh |
| Missing shard filter | Wrong knowledge | Filter not applied | Check shard context |

---

## Storage Schema Reference

### Knowledge Atom Storage

```sql
-- knowledge_atoms table
CREATE TABLE knowledge_atoms (
    id TEXT PRIMARY KEY,
    content TEXT,
    source TEXT,
    shard_type TEXT,
    embedding BLOB,  -- sqlite-vec compatible
    created_at INTEGER,
    accessed_at INTEGER
);
```

### Vector Search

```sql
-- Using sqlite-vec for similarity search
SELECT id, content
FROM knowledge_atoms
WHERE embedding MATCH ?
ORDER BY distance
LIMIT 10;
```

---

## Related Files

- [prompt-evolution.md](prompt-evolution.md) - Prompt Evolution System
- [mcp-jit-compiler.md](mcp-jit-compiler.md) - JIT Tool Compiler
- [context-compression.md](../05-world-context/context-compression.md) - Context management
