# Prompt System - Unified Storage Architecture

## Overview

The prompt system uses a **unified storage model** where prompt atoms are stored directly in each agent's knowledge database, NOT in a separate database.

## Storage Architecture

### Before (Incorrect)
```
.nerd/
├── prompts/
│   └── atoms.db          ❌ WRONG: Separate DB for prompts
├── shards/
│   └── coder_knowledge.db
```

### After (Correct)
```
.nerd/
├── agents/
│   └── coder/
│       └── prompts.yaml   ← Human-editable source
├── shards/
│   └── coder_knowledge.db ← Unified DB with knowledge_atoms + prompt_atoms tables
```

## Key Principles

1. **One Database Per Agent**: Each agent has ONE knowledge database at `.nerd/shards/{name}_knowledge.db`
2. **Two Tables**: The knowledge DB contains:
   - `knowledge_atoms` - Domain knowledge (facts, concepts, examples)
   - `prompt_atoms` - JIT-compiled prompt fragments
3. **YAML Source of Truth**: `.nerd/agents/{name}/prompts.yaml` is the human-editable source
4. **Build-Time Loading**: Prompts are loaded from YAML into SQLite at build/init time

## File Locations

| Type | Location | Purpose |
|------|----------|---------|
| YAML Source | `.nerd/agents/{name}/prompts.yaml` | Human-editable prompt definitions |
| Unified DB | `.nerd/shards/{name}_knowledge.db` | SQLite with knowledge + prompts |
| Loader Code | `internal/prompt/loader.go` | YAML → SQLite ingestion |
| JIT Compiler | `internal/prompt/compiler.go` | Runtime prompt assembly |

## Schema

### prompt_atoms table
```sql
CREATE TABLE prompt_atoms (
    id INTEGER PRIMARY KEY,
    atom_id TEXT UNIQUE,           -- e.g. "identity/coder/mission"
    version INTEGER,
    content TEXT,
    token_count INTEGER,
    content_hash TEXT,

    -- Classification
    category TEXT,                 -- identity, protocol, safety, etc.
    subcategory TEXT,

    -- Contextual Selectors (JSON arrays)
    operational_modes TEXT,        -- ["/active", "/debugging", etc.]
    campaign_phases TEXT,
    build_layers TEXT,
    init_phases TEXT,
    intent_verbs TEXT,
    shard_types TEXT,
    languages TEXT,
    frameworks TEXT,

    -- Composition
    priority INTEGER DEFAULT 50,
    is_mandatory BOOLEAN,
    depends_on TEXT,               -- JSON array of atom IDs
    conflicts_with TEXT,           -- JSON array of atom IDs

    -- Embeddings (for semantic search)
    embedding BLOB,
    embedding_task TEXT,

    created_at DATETIME
);
```

## Usage

### Loading Prompts (Build Time)

```go
import "codenerd/internal/prompt"

// Load prompts for an agent from YAML into their knowledge DB
count, err := prompt.LoadAgentPrompts(
    ctx,
    "coder",              // Agent name
    ".nerd",              // Nerd directory
    embeddingEngine,      // Optional: for semantic search
)
```

This will:
1. Read `.nerd/agents/coder/prompts.yaml`
2. Open `.nerd/shards/coder_knowledge.db`
3. Create `prompt_atoms` table if needed
4. Insert/update atoms

### Compiling Prompts (Runtime)

```go
import "codenerd/internal/prompt"

// Create compiler
compiler, err := prompt.NewJITPromptCompiler(
    prompt.WithProjectDB(projectDB),
)

// Register agent's knowledge DB (contains prompts)
compiler.RegisterShardDB("coder", coderKnowledgeDB)

// Compile prompt for context
context := &prompt.CompilationContext{
    ShardID: "coder",
    ShardType: "coder",
    IntentVerb: "/refactor",
    Language: "/go",
    TokenBudget: 100000,
}

result, err := compiler.Compile(ctx, context)
// result.Prompt contains assembled system prompt
```

## Why Unified Storage?

1. **Single Source of Truth**: One DB per agent, not scattered across multiple files
2. **Simpler Lifecycle**: Knowledge and prompts have same lifecycle as the agent
3. **Atomic Operations**: Transactions span both knowledge and prompts
4. **Easier Backup**: Backup one file to backup everything for an agent
5. **Clearer Ownership**: Prompt atoms belong to the agent, not the project

## Migration Path

If you have old `.nerd/prompts/atoms.db`:
1. Delete `.nerd/prompts/atoms.db`
2. Move YAML files to `.nerd/agents/{name}/prompts.yaml`
3. Run `LoadAgentPrompts()` for each agent
4. Prompts will be stored in unified knowledge DBs

## LocalStore Integration

The `internal/store.LocalStore` type already has `StorePromptAtom()` and `LoadPromptAtoms()` methods that work with the unified schema. Use these when working with agent knowledge databases.

```go
// Example: Store a prompt atom via LocalStore
localStore, _ := store.NewLocalStore(".nerd/shards/coder_knowledge.db")
atom := &store.PromptAtom{
    AtomID: "identity/coder/mission",
    Category: "identity",
    Content: "You are a code generation specialist...",
    TokenCount: 50,
    IsMandatory: true,
    Priority: 100,
}
localStore.StorePromptAtom(atom)
```
