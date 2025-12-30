# Shard Databases Reference

This directory contains SQLite databases for shard knowledge and autopoietic learnings. These databases form the persistent memory layer of codeNERD's neuro-symbolic architecture, enabling knowledge retention across sessions and learned behavior evolution through autopoiesis.

---

## Table of Contents

1. [Database Categories](#database-categories)
2. [Schema Details](#schema-details)
3. [Knowledge Databases](#knowledge-databases-knowledge_db)
4. [Learnings Databases](#learnings-databases-learnings_db)
5. [Core Concepts Database](#core-concepts-database)
6. [Store Interface Methods](#store-interface-methods)
7. [Initialization Flow](#initialization-flow)
8. [Autopoiesis Integration](#autopoiesis-integration)
9. [JIT Compiler Integration](#jit-compiler-integration)
10. [Vector Search Integration](#vector-search-integration)
11. [Maintenance Operations](#maintenance-operations)
12. [Wiring Gaps & TODOs](#wiring-gaps--todos)

---

## Database Categories

### Inventory Summary

| Category | Count | Size Range | Lifespan | Write Points | Read Points |
|----------|-------|------------|----------|--------------|-------------|
| **Knowledge DBs** | 12+ | 290KB - 11MB | Persistent | Init phase, Researcher | JIT compiler, Specialists |
| **Learnings DBs** | 7 | 24KB - 299KB | Persistent | Shard autopoiesis | Shard initialization |
| **Shared KB** | 1 | ~290KB | Persistent | Init phase (once) | All agents (inheritance) |
| **Total** | **20** | **~50MB** | **Persistent** | **Autopoiesis + Init** | **Boot/Compile time** |

### Complete Database List

| Database | Type | Purpose |
|----------|------|---------|
| `codebase_knowledge.db` | Knowledge | Project-specific patterns (language, framework, deps) |
| `coder_knowledge.db` | Knowledge | Code generation and refactoring expertise |
| `reviewer_knowledge.db` | Knowledge | Code review patterns and security checks |
| `tester_knowledge.db` | Knowledge | Testing strategies and TDD patterns |
| `campaign_knowledge.db` | Knowledge | Campaign orchestration concepts |
| `goexpert_knowledge.db` | Knowledge | Go idioms, concurrency, Uber style guide |
| `mangleexpert_knowledge.db` | Knowledge | Google Mangle/Datalog logic programming |
| `rodexpert_knowledge.db` | Knowledge | Rod browser automation, CDP protocol |
| `bubbleteaexpert_knowledge.db` | Knowledge | Bubbletea TUI, Elm architecture |
| `cobraexpert_knowledge.db` | Knowledge | Cobra CLI framework patterns |
| `securityauditor_knowledge.db` | Knowledge | OWASP top 10, vulnerability patterns |
| `testarchitect_knowledge.db` | Knowledge | Test architecture and coverage analysis |
| `coder_learnings.db` | Learnings | Coder avoid/preferred patterns |
| `coder_memory_learnings.db` | Learnings | Coder memory specialization |
| `reviewer_learnings.db` | Learnings | False positive filters, approved findings |
| `tester_learnings.db` | Learnings | Coverage patterns, test generation |
| `researcher_learnings.db` | Learnings | Research topics, source preferences |
| `perception_firewall_learnings.db` | Learnings | Intent classification patterns |
| `executive_learnings.db` | Learnings | Executive policy decisions |
| `core_concepts.db` | Shared | Universal patterns inherited by all agents |

---

## Schema Details

### Schema Version History

The system tracks schema evolution through `CurrentSchemaVersion = 4`:

| Version | Description | Migration Function |
|---------|-------------|-------------------|
| v1 | Basic `knowledge_atoms` table | Initial creation |
| v2 | Added `embedding` column for vector search | `MigrateV1ToV2()` |
| v3 | Added `vec_index` virtual table for sqlite-vec ANN | `MigrateV2ToV3()` |
| v4 | Added `content_hash` column for deduplication | `MigrateV3ToV4()` + `BackfillContentHashes()` |

### Knowledge Atoms Table

**Location:** `internal/store/local_core.go:118-150`

```sql
CREATE TABLE IF NOT EXISTS knowledge_atoms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    concept TEXT NOT NULL,              -- Category path: "pattern/error_handling"
    content TEXT NOT NULL,              -- The actual knowledge content
    confidence REAL DEFAULT 1.0,        -- 0.0-1.0 confidence score
    content_hash TEXT,                  -- SHA256 for deduplication (v4)
    source TEXT DEFAULT '',             -- Origin: "research", "inherited", "manual"
    tags TEXT DEFAULT '[]',             -- JSON array of tags
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_atoms_concept ON knowledge_atoms(concept);
CREATE INDEX IF NOT EXISTS idx_atoms_content_hash ON knowledge_atoms(content_hash);
```

### Learnings Table

**Location:** `internal/store/learning.go:45-65`

```sql
CREATE TABLE IF NOT EXISTS learnings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    shard_type TEXT NOT NULL,           -- "coder", "tester", "reviewer"
    fact_predicate TEXT NOT NULL,       -- "avoid_pattern", "preferred_pattern", etc.
    fact_args TEXT NOT NULL,            -- JSON array of arguments
    learned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    source_campaign TEXT DEFAULT '',    -- Campaign that triggered learning
    confidence REAL DEFAULT 1.0,        -- Decays over time, deleted < 0.1
    UNIQUE(fact_predicate, fact_args)   -- Prevents duplicates, enables upsert
);

CREATE INDEX IF NOT EXISTS idx_learnings_predicate ON learnings(fact_predicate);
CREATE INDEX IF NOT EXISTS idx_learnings_confidence ON learnings(confidence);
```

### Vector Store Table

**Location:** `internal/store/local_core.go:160-175`

```sql
CREATE TABLE IF NOT EXISTS vectors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content TEXT NOT NULL,              -- Original text content
    embedding TEXT,                     -- JSON-serialized float32[] vector
    metadata TEXT,                      -- JSON object with additional info
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_vectors_content ON vectors(content);

-- Optional: sqlite-vec virtual table for ANN search
CREATE VIRTUAL TABLE IF NOT EXISTS vec_index USING vec0(
    embedding float[1536]               -- Dimension from embedding engine
);
```

### Knowledge Graph Table

**Location:** `internal/store/local_core.go:180-200`

```sql
CREATE TABLE IF NOT EXISTS knowledge_graph (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_a TEXT NOT NULL,             -- Source entity
    relation TEXT NOT NULL,             -- Relationship type: "depends_on", "calls", etc.
    entity_b TEXT NOT NULL,             -- Target entity
    weight REAL DEFAULT 1.0,            -- Edge weight for spreading activation
    metadata TEXT,                      -- JSON additional properties
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entity_a, relation, entity_b)
);

CREATE INDEX IF NOT EXISTS idx_kg_entity_a ON knowledge_graph(entity_a);
CREATE INDEX IF NOT EXISTS idx_kg_entity_b ON knowledge_graph(entity_b);
CREATE INDEX IF NOT EXISTS idx_kg_relation ON knowledge_graph(relation);
```

### Cold Storage Table

**Location:** `internal/store/local_core.go:205-235`

```sql
CREATE TABLE IF NOT EXISTS cold_storage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    predicate TEXT NOT NULL,            -- Mangle predicate name
    args TEXT NOT NULL,                 -- JSON array of arguments
    fact_type TEXT DEFAULT 'fact',      -- "fact", "rule", "preference"
    priority INTEGER DEFAULT 0,         -- Higher = more important
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
    access_count INTEGER DEFAULT 0,     -- For archival decisions
    UNIQUE(predicate, args)
);

CREATE INDEX IF NOT EXISTS idx_cold_predicate ON cold_storage(predicate);
CREATE INDEX IF NOT EXISTS idx_cold_type ON cold_storage(fact_type);
CREATE INDEX IF NOT EXISTS idx_cold_last_accessed ON cold_storage(last_accessed);
CREATE INDEX IF NOT EXISTS idx_cold_access_count ON cold_storage(access_count);
```

### Archived Facts Table

**Location:** `internal/store/local_core.go:240-265`

```sql
CREATE TABLE IF NOT EXISTS archived_facts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    predicate TEXT NOT NULL,
    args TEXT NOT NULL,
    fact_type TEXT DEFAULT 'fact',
    priority INTEGER DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME,
    last_accessed DATETIME,
    access_count INTEGER DEFAULT 0,
    archived_at DATETIME DEFAULT CURRENT_TIMESTAMP,  -- When moved to archive
    UNIQUE(predicate, args)
);

CREATE INDEX IF NOT EXISTS idx_archived_predicate ON archived_facts(predicate);
CREATE INDEX IF NOT EXISTS idx_archived_type ON archived_facts(fact_type);
CREATE INDEX IF NOT EXISTS idx_archived_at ON archived_facts(archived_at);
```

### Additional Tables

**Session History** (`internal/store/local_core.go:270-290`)
```sql
CREATE TABLE IF NOT EXISTS session_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    turn_number INTEGER NOT NULL,
    user_input TEXT,
    intent_json TEXT,
    response TEXT,
    atoms_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(session_id, turn_number)
);
```

**Activation Log** (`internal/store/local_core.go:295-310`)
```sql
CREATE TABLE IF NOT EXISTS activation_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fact_id TEXT NOT NULL,
    activation_score REAL NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Reasoning Traces** (`internal/store/local_core.go:315-350`)
```sql
CREATE TABLE IF NOT EXISTS reasoning_traces (
    id TEXT PRIMARY KEY,
    shard_id TEXT NOT NULL,
    shard_type TEXT NOT NULL,
    shard_category TEXT NOT NULL,
    session_id TEXT NOT NULL,
    task_context TEXT,
    system_prompt TEXT NOT NULL,
    user_prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    model TEXT,
    tokens_used INTEGER,
    duration_ms INTEGER,
    success BOOLEAN NOT NULL,
    error_message TEXT,
    quality_score REAL,
    learning_notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Prompt Atoms** (`internal/store/local_prompt.go:57-132`)
```sql
CREATE TABLE IF NOT EXISTS prompt_atoms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    atom_id TEXT NOT NULL UNIQUE,
    version INTEGER DEFAULT 1,
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    content_hash TEXT NOT NULL,

    -- Polymorphism
    description TEXT,
    content_concise TEXT,
    content_min TEXT,

    -- Classification
    category TEXT NOT NULL,
    subcategory TEXT,

    -- Contextual Selectors (11 dimensions, JSON arrays)
    operational_modes TEXT,      -- ["/active", "/debugging", "/dream"]
    campaign_phases TEXT,        -- ["/planning", "/active"]
    build_layers TEXT,
    init_phases TEXT,
    northstar_phases TEXT,
    ouroboros_stages TEXT,
    intent_verbs TEXT,           -- ["/fix", "/debug", "/refactor"]
    shard_types TEXT,            -- ["/coder", "/tester", "/reviewer"]
    languages TEXT,              -- ["/go", "/python", "/typescript"]
    frameworks TEXT,
    world_states TEXT,

    -- Composition
    priority INTEGER DEFAULT 50,
    is_mandatory BOOLEAN DEFAULT FALSE,
    is_exclusive TEXT,
    depends_on TEXT,
    conflicts_with TEXT,

    -- Embeddings
    embedding BLOB,
    embedding_task TEXT DEFAULT 'RETRIEVAL_DOCUMENT',

    source_file TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## Knowledge Databases (`*_knowledge.db`)

### Origins

**Created by:** `internal/init/agents.go:createAgentKnowledgeBase()` (line ~487)

**Trigger:** During `nerd init`, `determineRequiredAgents()` analyzes the project structure to identify which specialist agents are needed based on:
- Primary language (Go, Python, TypeScript, Rust)
- Frameworks detected (Bubbletea, Cobra, Rod, Gin, etc.)
- Dependencies in go.mod/package.json/Cargo.toml
- Architectural patterns identified

**Creation Flow:**
```
nerd init
  └── internal/init/initializer.go:Run()
      └── Phase 6: determineRequiredAgents()
          └── Returns []RecommendedAgent with topics
      └── Phase 7: createType3Agents()
          └── For each agent:
              ├── kbPath := .nerd/shards/{agent}_knowledge.db
              ├── store.NewLocalStore(kbPath)  // Creates DB + schema
              ├── generateBaseKnowledgeAtoms() // Identity atoms
              ├── ResearcherShard.ResearchTopicsParallel()
              ├── InheritSharedKnowledge()     // From core_concepts.db
              └── generateAgentPromptsYAML()   // JIT atoms
```

**Key Code Path:**
```go
// internal/init/agents.go:487
kbPath := filepath.Join(shardsDir, fmt.Sprintf("%s_knowledge.db", strings.ToLower(agent.Name)))

// internal/init/agents.go:668-676
agentDB, err := store.NewLocalStore(kbPath)
if err != nil {
    return stats, fmt.Errorf("failed to open agent DB: %w", err)
}
```

### Consumers

**Primary Consumer:** JIT Prompt Compiler (`internal/prompt/compiler.go`)
- Reads atoms at shard spawn time via `GetAllKnowledgeAtoms()`
- Selects atoms based on `CompilationContext` matching
- Injects into LLM system prompt

**Secondary Consumer:** Specialist Shards
- `internal/shards/reviewer/knowledge.go:LoadAndQueryKnowledgeBase()`
- `internal/shards/reviewer/specialists.go:MatchSpecialistsForReview()`
- Query for domain-specific patterns via `GetKnowledgeAtoms(concept)`
- Semantic search via `VectorRecall(query, limit)`

**When Consumed:**
- On every shard invocation (JIT compilation)
- During specialist matching for code review
- When researcher augments knowledge

### Data Flow

**Write Operations:**
```go
// Single atom insertion with deduplication
store.StoreKnowledgeAtom(concept, content, confidence)
// - Computes content_hash = SHA256(content)
// - INSERT OR IGNORE (hash prevents duplicates)

// With embedding (if engine configured)
store.StoreKnowledgeAtomWithEmbedding(ctx, concept, content, confidence)
// - Also stores in vectors table for semantic search
```

**Read Operations:**
```go
// By concept
atoms, err := store.GetKnowledgeAtoms("pattern/error_handling")

// All atoms
atoms, err := store.GetAllKnowledgeAtoms()

// By prefix (for categories)
atoms, err := store.GetKnowledgeAtomsByPrefix("strategic/")

// Semantic search
atoms, err := store.SearchKnowledgeAtomsSemantic(ctx, "goroutine leak", 10)
```

**Cleanup:**
- `MaintenanceCleanup()` in `internal/store/local_cold.go`
- Archives atoms not accessed in 90+ days
- Purges archived atoms after 365 days

### Associated Files

| File | Purpose |
|------|---------|
| `internal/store/local_knowledge.go` | CRUD operations for knowledge_atoms |
| `internal/init/agents.go` | Agent KB creation during init |
| `internal/init/shared_kb.go` | core_concepts.db and inheritance |
| `internal/shards/reviewer/knowledge.go` | Example consumption pattern |
| `internal/prompt/compiler.go` | JIT consumption |

---

## Learnings Databases (`*_learnings.db`)

### Origins

**Created by:** `internal/store/learning.go:NewLearningStore()` (line ~45)

**Trigger:** On-demand when a shard first attempts to save a learning. The database file is created lazily:

```go
// internal/store/learning.go:74
func (ls *LearningStore) getDB(shardType string) (*sql.DB, error) {
    dbPath := filepath.Join(ls.basePath, fmt.Sprintf("%s_learnings.db", shardType))
    db, err := sql.Open("sqlite3", dbPath)
    // Creates file if not exists, initializes schema
}
```

**Naming Convention:** `.nerd/shards/{shardType}_learnings.db`

### Consumers

**Primary Consumer:** Shard initialization
- `LearningStore.Load(shardType)` called when shard spawns
- Returns learnings with `confidence > 0.3`
- Injected as `learned_preference` facts into SessionContext

**When Consumed:**
- Every shard spawn loads its learnings
- Kernel receives facts for policy evaluation
- Shards use learnings to avoid past mistakes

### Data Flow

**Write Operations (Autopoiesis Triggers):**

| Shard | Predicate | Trigger | Args |
|-------|-----------|---------|------|
| Coder | `avoid_pattern` | 2+ rejections of same action | [action, reason] |
| Coder | `preferred_pattern` | 3+ acceptances of action | [action] |
| Tester | `failure_pattern` | 3+ failures with same pattern | [pattern, message] |
| Tester | `success_pattern` | 5+ successes with pattern | [pattern, test_name] |
| Reviewer | `flagged_pattern` | 3+ critical findings | [pattern, category, severity] |
| Reviewer | `approved_pattern` | 5+ clean findings | [pattern] |
| Reviewer | `anti_pattern` | Immediate on detection | [pattern, reason] |
| Reviewer | `suppressed_rule` | User dismissal | [hypoType, file, line, reason] |
| Reviewer | `suppression_confidence` | Updated on dismissal | [hypoType, file, line, score] |
| Reviewer | `global_suppression_pattern` | Confidence > 90 | [hypoType, pattern] |

**Upsert Behavior:**
```go
// internal/store/learning.go:116
func (ls *LearningStore) Save(shardType, predicate string, args []any, campaign string) error {
    // ON CONFLICT: confidence = MIN(1.0, confidence + 0.1)
    // Reinforcement asymptotically approaches 1.0
}
```

**Read Operations:**
```go
// Load all learnings for a shard type
learnings, err := ls.Load("coder")  // confidence > 0.3, ordered by confidence DESC

// Load specific predicate
learnings, err := ls.LoadByPredicate("reviewer", "suppressed_rule")
```

**Cleanup (Decay):**
```go
// internal/store/learning.go:246
func (ls *LearningStore) DecayConfidence(shardType string, decayFactor float64) error {
    // For learnings older than 7 days:
    //   confidence = confidence * decayFactor (typically 0.9)
    // Delete where confidence < 0.1
}
```

**Decay Timeline Example (factor=0.9):**
| Days | Confidence | Status |
|------|------------|--------|
| 0 | 1.0 | Active |
| 7 | 0.9 | Active |
| 14 | 0.81 | Active |
| 21 | 0.73 | Active |
| 28 | 0.66 | Active |
| 56 | 0.43 | Active |
| 84 | 0.28 | Borderline |
| 112 | 0.19 | Borderline |
| 140 | 0.12 | Borderline |
| 168 | 0.08 | **Deleted** |

### Associated Files

| File | Purpose |
|------|---------|
| `internal/store/learning.go` | LearningStore implementation |
| `internal/shards/coder/autopoiesis.go` | Coder learning triggers |
| `internal/shards/tester/autopoiesis.go` | Tester learning triggers |
| `internal/shards/reviewer/autopoiesis.go` | Reviewer learning triggers (most complex) |

---

## Core Concepts Database

### Origins

**Created by:** `internal/init/shared_kb.go:CreateSharedKnowledgePool()` (line ~25)

**Trigger:** Created once during workspace initialization, BEFORE any agent knowledge bases. This ensures all agents can inherit from it.

**Content:** Hardcoded `BaseSharedAtoms` covering universal patterns:
- Error handling best practices
- Logging conventions
- Testing methodologies
- Documentation standards
- Code organization patterns

### Consumers

**Primary Consumer:** `InheritSharedKnowledge()` during agent KB creation
- Copies all atoms from core_concepts.db to each agent's KB
- Ensures baseline knowledge without redundant research

**Key Code:**
```go
// internal/init/shared_kb.go
func InheritSharedKnowledge(agentDB *store.LocalStore, coreDB *store.LocalStore) error {
    atoms, err := coreDB.GetAllKnowledgeAtoms()
    for _, atom := range atoms {
        agentDB.StoreKnowledgeAtom(atom.Concept, atom.Content, atom.Confidence)
    }
}
```

### Data Flow

**Write:** Once during init, never modified at runtime
**Read:** During agent creation for inheritance

### Associated Files

| File | Purpose |
|------|---------|
| `internal/init/shared_kb.go` | Creation and inheritance logic |
| `internal/store/local_knowledge.go` | Underlying storage operations |

---

## Store Interface Methods

### LocalStore (`internal/store/local_*.go`)

#### Initialization & Connection
```go
NewLocalStore(path string) (*LocalStore, error)  // Initialize store, create schema
Close() error                                     // Close database connection
GetDB() *sql.DB                                  // Return underlying SQL connection
GetTraceStore() *TraceStore                      // Get dedicated trace store
```

#### Fact Storage (Cold Storage Tier)
```go
StoreFact(predicate string, args []interface{}, factType string, priority int) error
LoadFacts(predicate string) ([]StoredFact, error)      // Updates access counters
LoadAllFacts(factType string) ([]StoredFact, error)
DeleteFact(predicate string, args []interface{}) error
```

#### Knowledge Atoms
```go
StoreKnowledgeAtom(concept, content string, confidence float64) error
GetKnowledgeAtoms(concept string) ([]KnowledgeAtom, error)
GetAllKnowledgeAtoms() ([]KnowledgeAtom, error)
GetKnowledgeAtomsByPrefix(conceptPrefix string) ([]KnowledgeAtom, error)
StoreKnowledgeAtomWithEmbedding(ctx context.Context, concept, content string, confidence float64) error
SearchKnowledgeAtomsSemantic(ctx context.Context, query string, limit int) ([]KnowledgeAtom, error)
```

#### Vector Store
```go
StoreVector(content string, metadata map[string]interface{}) error
VectorRecall(query string, limit int) ([]VectorEntry, error)                    // Keyword fallback
SetEmbeddingEngine(engine embedding.EmbeddingEngine)                             // Configure embeddings
StoreVectorWithEmbedding(ctx context.Context, content string, metadata map[string]interface{}) error
VectorRecallSemantic(ctx context.Context, query string, limit int) ([]VectorEntry, error)
VectorRecallSemanticFiltered(ctx context.Context, query string, limit int, metaKey string, metaValue interface{}) ([]VectorEntry, error)
```

#### Knowledge Graph
```go
StoreLink(entityA, relation, entityB string, weight float64, metadata map[string]interface{}) error
QueryLinks(direction string, entity string, relation string) ([]KnowledgeLink, error)
```

#### Archival & Maintenance
```go
ArchiveOldFacts(olderThanDays, maxAccessCount int) (int, error)
GetArchivedFacts(predicate string) ([]ArchivedFact, error)
RestoreArchivedFact(predicate string, args []interface{}) error
PurgeOldArchivedFacts(olderThanDays int) (int, error)
MaintenanceCleanup(config MaintenanceConfig) (MaintenanceStats, error)
```

#### Session & Activation
```go
LogActivation(factID string, activationScore float64) error
GetRecentActivations(limit int) ([]ActivationLog, error)
StoreSessionTurn(sessionID string, turnNumber int, userInput, response string, atomsJSON string) error
GetSessionHistory(sessionID string) ([]SessionTurn, error)
```

### LearningStore (`internal/store/learning.go`)

```go
NewLearningStore(basePath string) (*LearningStore, error)
Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error
Load(shardType string) ([]types.ShardLearning, error)              // confidence > 0.3
LoadByPredicate(shardType, predicate string) ([]types.ShardLearning, error)
DecayConfidence(shardType string, decayFactor float64) error       // Older than 7 days
Delete(shardType, factPredicate string, factArgs []any) error
GetStats(shardType string) (map[string]interface{}, error)
Close() error
```

---

## Initialization Flow

### Phase Sequence (`internal/init/initializer.go`)

```
Phase 1: Workspace Setup
    └── createDirectoryStructure() → creates .nerd/ tree
    └── Initialize LocalStore at .nerd/knowledge.db

Phase 2: Codebase Scan
    └── world.Scanner for file analysis
    └── Assert scan results as Mangle facts

Phase 3: Researcher Analysis
    └── Spawn ResearcherShard for deep analysis
    └── Generate tech/pattern/risk summary

Phase 4: Project Profile Build (profile.go)
    ├── detectLanguageFromFiles()
    ├── detectDependencies()
    ├── detectEntryPoints()
    └── detectBuildSystem()
    └── Save to .nerd/profile.json

Phase 5: Generate Mangle Facts (profile.go)
    └── Create .nerd/profile.mg with project facts
    └── Load facts into kernel

Phase 6: Determine Required Agents (agents.go)
    └── determineRequiredAgents() analyzes project
    └── Returns ~8-12 recommended specialists

Phase 7: Create Knowledge Bases & Agents (agents.go)
    └── For each agent:
        ├── createAgentKnowledgeBase()
        │   └── .nerd/shards/{agent}_knowledge.db
        ├── generateBaseKnowledgeAtoms()
        ├── ResearcherShard.ResearchTopicsParallel()
        ├── InheritSharedKnowledge()
        └── generateAgentPromptsYAML()
    └── Register with ShardManager

Phase 8: Initialize User Preferences
    └── Create .nerd/preferences.json

Phase 9: Create Session State
    └── Initialize .nerd/session.json

Phase 10: Generate Agent Registry
    └── Save .nerd/agents.json
```

### Directory Structure Created

```
.nerd/
├── profile.mg                    # Mangle facts
├── profile.json                  # Project metadata
├── preferences.json              # User preferences
├── session.json                  # Current session
├── agents.json                   # Agent registry
├── knowledge.db                  # Main knowledge store
├── shards/
│   ├── CLAUDE.md                 # This file
│   ├── core_concepts.db          # Shared knowledge pool
│   ├── codebase_knowledge.db     # Project-specific
│   ├── coder_knowledge.db        # Coder specialist
│   ├── reviewer_knowledge.db     # Reviewer specialist
│   ├── tester_knowledge.db       # Tester specialist
│   ├── campaign_knowledge.db     # Campaign patterns
│   ├── goexpert_knowledge.db     # Go expertise
│   ├── mangleexpert_knowledge.db # Mangle expertise
│   ├── rodexpert_knowledge.db    # Rod automation
│   ├── bubbleteaexpert_knowledge.db
│   ├── cobraexpert_knowledge.db
│   ├── securityauditor_knowledge.db
│   ├── testarchitect_knowledge.db
│   ├── coder_learnings.db        # Autopoiesis learnings
│   ├── coder_memory_learnings.db
│   ├── tester_learnings.db
│   ├── reviewer_learnings.db
│   ├── researcher_learnings.db
│   ├── perception_firewall_learnings.db
│   └── executive_learnings.db
├── prompts/
│   └── corpus.db                 # Project prompt atoms
├── sessions/                     # Session history
├── agents/                       # Per-agent config
│   └── {agent}/prompts.yaml
└── tools/                        # Ouroboros tools
```

---

## Autopoiesis Integration

### CoderShard (`internal/shards/coder/autopoiesis.go`)

**Tracking Methods:**

```go
// Called on user rejection
func (c *CoderShard) trackRejection(action, reason string) {
    key := action + ":" + reason
    c.rejectionCounts[key]++
    if c.rejectionCounts[key] >= 2 {
        c.learningStore.Save("coder", "avoid_pattern", []any{action, reason}, "")
    }
}

// Called on user acceptance
func (c *CoderShard) trackAcceptance(action string) {
    c.acceptanceCounts[action]++
    if c.acceptanceCounts[action] >= 3 {
        c.learningStore.Save("coder", "preferred_pattern", []any{action}, "")
    }
}

// Called on shard initialization
func (c *CoderShard) loadLearnedPatterns() {
    learnings, _ := c.learningStore.Load("coder")
    for _, l := range learnings {
        // Pre-populate counters at threshold to prevent re-learning
        if l.Predicate == "avoid_pattern" {
            c.rejectionCounts[l.Args[0]+":"+l.Args[1]] = 2
        }
    }
}
```

### TesterShard (`internal/shards/tester/autopoiesis.go`)

**Tracking Methods:**

```go
func (t *TesterShard) trackFailurePattern(result *TestResult) {
    pattern := normalizePattern(result.FailureMessage)  // Strip numbers, limit 100 chars
    t.failureCounts[pattern]++
    if t.failureCounts[pattern] >= 3 {
        t.learningStore.Save("tester", "failure_pattern", []any{pattern, result.FailureMessage}, "")
    }
}

func (t *TesterShard) trackSuccessPattern(result *TestResult) {
    pattern := normalizePattern(result.TestName)
    t.successCounts[pattern]++
    if t.successCounts[pattern] >= 5 {
        t.learningStore.Save("tester", "success_pattern", []any{pattern, result.TestName}, "")
    }
}
```

### ReviewerShard (`internal/shards/reviewer/autopoiesis.go`)

**Most Complex Learning System:**

**1. Pattern Tracking (lines 12-100)**
```go
func (r *ReviewerShard) trackReviewPatterns(result *ReviewResult) {
    for _, finding := range result.Findings {
        if finding.Severity == "critical" || finding.Severity == "error" {
            pattern := extractPattern(finding)
            r.flaggedCounts[pattern]++
            if r.flaggedCounts[pattern] >= 3 {
                r.learningStore.Save("reviewer", "flagged_pattern",
                    []any{pattern, finding.Category, finding.Severity}, "")
            }
        }
    }
}
```

**2. Anti-Pattern Learning (Immediate)**
```go
func (r *ReviewerShard) LearnAntiPattern(pattern, reason string) {
    // No threshold - immediate persistence
    r.learningStore.Save("reviewer", "anti_pattern", []any{pattern, reason}, "")
}
```

**3. Dismissal Learning (lines 146-190) - 4-Step Process**
```go
func (r *ReviewerShard) LearnFromDismissal(hypo Hypothesis, reason string) {
    // Step 1: Persist suppression fact
    r.learningStore.Save("reviewer", "suppressed_rule",
        []any{hypo.Type, hypo.File, hypo.Line, reason}, "")

    // Step 2: Update confidence score (sigmoid growth toward 100)
    currentScore := r.suppressionConfidenceCache[key]
    newScore := currentScore + (100 - currentScore) * 0.2
    r.learningStore.Save("reviewer", "suppression_confidence",
        []any{hypo.Type, hypo.File, hypo.Line, newScore}, "")

    // Step 3: If confidence > 90, promote to global pattern
    if newScore > 90 {
        pattern := r.extractSuppressionPattern(hypo, reason)
        r.learningStore.Save("reviewer", "global_suppression_pattern",
            []any{hypo.Type, pattern}, "")
    }

    // Step 4: Assert to kernel for immediate effect
    r.kernel.Assert(Fact{Predicate: "suppressed_rule", Args: [...]})
}
```

**4. Pattern Extraction Mapping (lines 265-302)**
```go
func (r *ReviewerShard) extractSuppressionPattern(hypo Hypothesis, reason string) string {
    switch {
    case strings.Contains(reason, "guard early return"):
        return "guarded_by_early_return"
    case strings.Contains(reason, "sync.once"):
        return "guarded_by_sync_once"
    case strings.Contains(reason, "test file"):
        return "test_file_acceptable"
    case strings.Contains(reason, "intentional"):
        return "intentional_by_design"
    case strings.Contains(reason, "error handled"):
        return "error_properly_handled"
    case strings.Contains(reason, "context cancel"):
        return "context_managed"
    case strings.Contains(reason, "mutex"):
        return "mutex_protected"
    case strings.Contains(reason, "defer"):
        return "deferred_cleanup"
    case strings.Contains(reason, "buffered channel"):
        return "channel_buffered_or_select"
    case strings.Contains(reason, "validated"):
        return "validated_externally"
    default:
        return "user_dismissed"
    }
}
```

**5. Loading Suppressions (lines 385-478)**
```go
func (r *ReviewerShard) LoadSuppressions() error {
    // Load suppressed_rule facts into kernel
    learnings, _ := r.learningStore.LoadByPredicate("reviewer", "suppressed_rule")
    for _, l := range learnings {
        r.kernel.Assert(...)
    }

    // Load confidence scores into cache
    confidences, _ := r.learningStore.LoadByPredicate("reviewer", "suppression_confidence")
    for _, c := range confidences {
        r.suppressionConfidenceCache[key] = c.Args[3].(float64)
    }

    // Load global patterns
    globals, _ := r.learningStore.LoadByPredicate("reviewer", "global_suppression_pattern")
    // Used for project-wide filtering
}
```

---

## JIT Compiler Integration

### PromptAtom Structure

**11 Contextual Selector Dimensions:**

```go
type PromptAtom struct {
    AtomID           string   // "identity/coder/mission"
    Category         string   // identity, capability, protocol, safety

    // When to include this atom
    OperationalModes []string // ["/active", "/debugging", "/dream"]
    CampaignPhases   []string // ["/planning", "/implementation"]
    BuildLayers      []string
    InitPhases       []string
    NorthstarPhases  []string
    OuroborosStages  []string
    IntentVerbs      []string // ["/fix", "/debug", "/refactor"]
    ShardTypes       []string // ["/coder", "/tester", "/reviewer"]
    Languages        []string // ["/go", "/python"]
    Frameworks       []string // ["/bubbletea", "/rod"]
    WorldStates      []string // ["failing_tests", "compile_error"]

    // Composition rules
    Priority         int      // 0-100
    IsMandatory      bool     // Always include
    IsExclusive      string   // Only one from group
    DependsOn        []string
    ConflictsWith    []string

    // Content
    Content          string
    TokenCount       int
    Embedding        []byte
}
```

### Consumption Flow

1. **Query:** `LoadPromptAtoms()` loads all atoms from database
2. **Filter:** Match atoms against current `CompilationContext`
3. **Score:** Combine Priority + ActivationScore
4. **Budget:** Select atoms until token budget exhausted
5. **Assembly:** Concatenate into system prompt

### Knowledge Atom Injection

```go
// internal/prompt/compiler.go (conceptual)
func (c *Compiler) AssembleSystemPrompt(ctx CompilationContext) string {
    // Load specialist knowledge
    if specialist := ctx.ActiveSpecialist; specialist != "" {
        kbPath := fmt.Sprintf(".nerd/shards/%s_knowledge.db", specialist)
        kb, _ := store.NewLocalStore(kbPath)
        atoms, _ := kb.GetAllKnowledgeAtoms()

        // Inject as specialist hints
        for _, atom := range atoms {
            c.addAtom(atom.Content, atom.Confidence * spreadingScore)
        }
    }
}
```

---

## Vector Search Integration

### Detection & Fallback

**Location:** `internal/store/local_core.go:84-95`

```go
func (s *LocalStore) detectVecExtension() {
    // Try CREATE VIRTUAL TABLE vec0(...)
    // If fails: s.hasVecExtension = false
    // Graceful degradation to keyword search
}
```

### Build Modes

| Build | sqlite-vec | Failure Mode |
|-------|-----------|--------------|
| Default | Optional | Keyword fallback |
| `-tags=sqlite_vec` | Required | Fail fast |

### Search Methods

**Semantic Search (with embeddings):**
```go
func (s *LocalStore) VectorRecallSemantic(ctx context.Context, query string, limit int) ([]VectorEntry, error) {
    // 1. Generate query embedding
    queryEmb, _ := s.embeddingEngine.Embed(ctx, query)

    // 2. If sqlite-vec: use ANN
    if s.hasVecExtension {
        return s.vecANNSearch(queryEmb, limit)
    }

    // 3. Fallback: brute-force cosine similarity
    return s.bruteForceSearch(queryEmb, limit)
}
```

**Cosine Similarity:**
```go
func CosineSimilarity(a, b []float64) float64 {
    dot := 0.0
    normA, normB := 0.0, 0.0
    for i := range a {
        dot += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }
    return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

---

## Maintenance Operations

### MaintenanceConfig

```go
type MaintenanceConfig struct {
    ArchiveOlderThanDays       int   // Archive facts not accessed in N days (default: 90)
    MaxAccessCount             int   // Only if access_count <= this (default: 5)
    PurgeArchivedOlderThanDays int   // Delete archived older than N days (default: 365)
    CleanActivationLogDays     int   // Delete activation logs older than N days (default: 30)
    VacuumDatabase             bool  // Run VACUUM to reclaim space
}
```

### Maintenance Cycle

```go
func (s *LocalStore) MaintenanceCleanup(config MaintenanceConfig) (MaintenanceStats, error) {
    stats := MaintenanceStats{}

    // 1. Archive old, rarely-accessed facts
    archived, _ := s.ArchiveOldFacts(config.ArchiveOlderThanDays, config.MaxAccessCount)
    stats.FactsArchived = archived

    // 2. Purge very old archived facts
    purged, _ := s.PurgeOldArchivedFacts(config.PurgeArchivedOlderThanDays)
    stats.FactsPurged = purged

    // 3. Clean activation logs
    s.db.Exec(`DELETE FROM activation_log WHERE datetime(timestamp) < datetime('now', '-30 days')`)

    // 4. VACUUM
    if config.VacuumDatabase {
        s.db.Exec("VACUUM")
    }

    return stats, nil
}
```

### Learning Decay

```go
func (ls *LearningStore) DecayConfidence(shardType string, decayFactor float64) error {
    // Decay learnings older than 7 days
    _, err := db.Exec(`
        UPDATE learnings
        SET confidence = confidence * ?
        WHERE datetime(learned_at) < datetime('now', '-7 days')
    `, decayFactor)

    // Delete forgotten learnings
    _, err = db.Exec(`DELETE FROM learnings WHERE confidence < 0.1`)

    return err
}
```

---

## Wiring Gaps & TODOs

During this deep dive, the following potential wiring gaps were identified:

### 1. Researcher Learnings Not Fully Wired

**Gap:** `researcher_learnings.db` exists but `internal/shards/researcher/` lacks an `autopoiesis.go` file.

**Evidence:**
- Database file exists: `.nerd/shards/researcher_learnings.db`
- No corresponding autopoiesis implementation found
- ResearcherShard doesn't call `LearningStore.Save()`

**Impact:** Researcher shard doesn't learn from successful/failed research patterns.

**Fix:** Create `internal/shards/researcher/autopoiesis.go` with:
- `trackSuccessfulSource(source)` - Learn which sources provide good results
- `trackFailedQuery(query)` - Learn which query patterns fail
- `trackTopicRelevance(topic, score)` - Learn topic usefulness

### 2. Perception Firewall Learnings Partially Wired

**Gap:** `perception_firewall_learnings.db` exists but learning integration unclear.

**Evidence:**
- Database exists in `.nerd/shards/`
- `internal/perception/transducer.go` doesn't reference LearningStore

**Impact:** Intent classification doesn't improve from corrections.

**Fix:** Add learning hooks to transducer for:
- `trackIntentCorrection(original, corrected)` - Learn from user corrections
- `trackConfidenceCalibration(intent, actual)` - Improve confidence scoring

### 3. Executive Learnings Not Wired

**Gap:** `executive_learnings.db` exists but no executive shard autopoiesis.

**Evidence:**
- Database exists
- No `internal/core/executive_autopoiesis.go` or similar

**Impact:** Policy decisions don't learn from outcomes.

**Fix:** Add executive learning for:
- `trackPolicySuccess(policy, outcome)` - Learn which policies work
- `trackActionSequence(actions, success)` - Learn effective action orderings

### 4. Coder Memory Learnings Unclear

**Gap:** `coder_memory_learnings.db` purpose and wiring unclear.

**Evidence:**
- Separate from `coder_learnings.db`
- No clear differentiation in code

**Impact:** Potential duplicate/orphaned database.

**Fix:** Either:
- Remove if redundant
- Document distinct purpose and wire up
- Merge into `coder_learnings.db`

### 5. Missing Decay Trigger

**Gap:** `DecayConfidence()` exists but no automatic trigger found.

**Evidence:**
- Method exists in `learning.go:246`
- No caller in session startup or maintenance routines

**Impact:** Old learnings never decay, potentially causing stale behavior.

**Fix:** Add decay call to:
- Session startup in `cmd/nerd/chat/session.go`
- Or periodic maintenance in a goroutine

### 6. Knowledge Atom Embedding Backfill

**Gap:** Existing knowledge atoms may lack embeddings after upgrade.

**Evidence:**
- `StoreKnowledgeAtomWithEmbedding` exists
- `StoreKnowledgeAtom` doesn't create embeddings
- Init uses `StoreKnowledgeAtom`

**Impact:** Semantic search over knowledge atoms returns fewer results.

**Fix:** Add migration or backfill routine:
```go
func BackfillKnowledgeAtomEmbeddings(store *LocalStore, engine EmbeddingEngine) error {
    atoms, _ := store.GetAllKnowledgeAtoms()
    for _, atom := range atoms {
        if !hasEmbedding(atom) {
            store.StoreKnowledgeAtomWithEmbedding(ctx, atom.Concept, atom.Content, atom.Confidence)
        }
    }
}
```

### 7. TraceStore Integration Incomplete

**Gap:** `reasoning_traces` table exists but consumption unclear.

**Evidence:**
- TraceStore has `StoreTrace()` and `QueryTraces()`
- No clear consumer for learning from traces

**Impact:** LLM reasoning traces collected but not analyzed.

**Fix:** Add trace analysis for:
- Pattern extraction from successful traces
- Error pattern detection from failed traces
- Quality score trends

---

## Summary Statistics

| Metric | Count |
|--------|-------|
| Total Tables | 17 main tables |
| Total Indexes | 40+ indexes |
| Schema Versions | 4 |
| LearningStore Predicates | 10 types |
| Prompt Atom Selectors | 11 dimensions |
| Archival Tiers | 2 (cold_storage, archived_facts) |
| Init Phases | 10+ major phases |
| Shard Types with Autopoiesis | 3 (coder, tester, reviewer) |
| Wiring Gaps Identified | 7 |

---

**Remember: Push to GitHub regularly!**
