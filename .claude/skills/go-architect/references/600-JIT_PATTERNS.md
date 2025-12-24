# 600 - JIT Compiler Patterns

Advanced Go patterns for codeNERD's JIT Prompt Compiler and multi-source compilation.

## go:embed Patterns for Compile-Time Assets

Use `go:embed` to embed resources at compile time, enabling portable binaries with baked-in assets.

### Embedding Database Files

```go
import _ "embed"

// Embedding prompt corpus at compile time
//go:embed prompt_corpus.db
var embeddedCorpusDB []byte

// Embedding with placeholder for CI/CD
//go:embed prompt_corpus.db.placeholder
var embeddedCorpusPlaceholder []byte

// Loading embedded corpus to temp file for SQLite access
func LoadEmbeddedCorpus() (*sql.DB, error) {
    if len(embeddedCorpusDB) == 0 {
        return nil, fmt.Errorf("embedded corpus not available")
    }

    // Write to temp file for SQLite access
    tmpFile, err := os.CreateTemp("", "prompt_corpus_*.db")
    if err != nil {
        return nil, fmt.Errorf("create temp file: %w", err)
    }

    if _, err := tmpFile.Write(embeddedCorpusDB); err != nil {
        tmpFile.Close()
        os.Remove(tmpFile.Name())
        return nil, fmt.Errorf("write corpus: %w", err)
    }

    if err := tmpFile.Close(); err != nil {
        os.Remove(tmpFile.Name())
        return nil, fmt.Errorf("close temp file: %w", err)
    }

    // Open with SQLite
    db, err := sql.Open("sqlite3", tmpFile.Name())
    if err != nil {
        os.Remove(tmpFile.Name())
        return nil, fmt.Errorf("open database: %w", err)
    }

    return db, nil
}
```

**Key Points:**

- Always check `len(embeddedCorpusDB) == 0` before using
- Clean up temp files on errors with `defer` or explicit cleanup
- Return meaningful errors with context wrapping

### Embedding Text Files

```go
//go:embed templates/*.txt
var templateFS embed.FS

func LoadTemplate(name string) (string, error) {
    data, err := templateFS.ReadFile("templates/" + name)
    if err != nil {
        return "", fmt.Errorf("load template %s: %w", name, err)
    }
    return string(data), nil
}
```

## Callback Wiring Pattern (Dependency Injection)

Enable loose coupling between components through function-typed callbacks.

### Callback Type Definitions

```go
// Callback types for dependency injection
type JITDBRegistrar func(shardName string, dbPath string) error
type JITDBUnregistrar func(shardName string) error
type PromptLoader func(shardName, yamlPath, dbPath string) error

// ShardManager with JIT callbacks
type ShardManager struct {
    promptLoader    PromptLoader
    jitRegistrar    JITDBRegistrar
    jitUnregistrar  JITDBUnregistrar
    mu              sync.RWMutex
}

// Setters for dependency injection (called during bootstrap)
func (sm *ShardManager) SetPromptLoader(loader PromptLoader) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.promptLoader = loader
}

func (sm *ShardManager) SetJITRegistrar(registrar JITDBRegistrar) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.jitRegistrar = registrar
}

func (sm *ShardManager) SetJITUnregistrar(unregistrar JITDBUnregistrar) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.jitUnregistrar = unregistrar
}
```

### Factory Functions for Creating Callbacks

```go
// Factory creates a callback that closes over the compiler instance
func CreateJITDBRegistrar(compiler *JITPromptCompiler) JITDBRegistrar {
    return func(shardName string, dbPath string) error {
        return RegisterAgentDBWithJIT(compiler, shardName, dbPath)
    }
}

func CreateJITDBUnregistrar(compiler *JITPromptCompiler) JITDBUnregistrar {
    return func(shardName string) {
        compiler.UnregisterShardDB(shardName)
        logging.Debug("Unregistered agent DB: %s", shardName)
    }
}

// Usage in bootstrap code
func Bootstrap(kernel *Kernel, shardMgr *ShardManager) error {
    // Create JIT compiler
    compiler, err := prompt.NewJITPromptCompiler(
        prompt.WithEmbeddedCorpus(embeddedCorpus),
        prompt.WithKernel(kernel),
    )
    if err != nil {
        return err
    }

    // Wire callbacks
    shardMgr.SetJITRegistrar(prompt.CreateJITDBRegistrar(compiler))
    shardMgr.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(compiler))

    return nil
}
```

**Key Points:**

- Callbacks enable wiring without import cycles
- Factory functions close over dependencies
- Use mutex protection when setting callbacks on shared state
- Always check for nil before invoking callbacks

### Safe Callback Invocation

```go
// Invoke callback with nil-safety
func (sm *ShardManager) RegisterAgentDB(shardName, dbPath string) error {
    sm.mu.RLock()
    registrar := sm.jitRegistrar
    sm.mu.RUnlock()

    if registrar == nil {
        logging.Debug("No JIT registrar configured, skipping DB registration")
        return nil // Not an error - JIT is optional
    }

    return registrar(shardName, dbPath)
}
```

## SQLite Table Extension Pattern

Extend existing databases with new tables at runtime without breaking existing functionality.

### Idempotent Table Creation

```go
// ensurePromptAtomsTable adds prompt_atoms table to existing knowledge DB
func ensurePromptAtomsTable(db *sql.DB) error {
    schema := `
        CREATE TABLE IF NOT EXISTS prompt_atoms (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            atom_id TEXT NOT NULL UNIQUE,
            content TEXT NOT NULL,
            token_count INTEGER NOT NULL,
            category TEXT NOT NULL,
            priority INTEGER DEFAULT 50,
            is_mandatory BOOLEAN DEFAULT FALSE,
            embedding BLOB,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );

        CREATE INDEX IF NOT EXISTS idx_atoms_category ON prompt_atoms(category);
        CREATE INDEX IF NOT EXISTS idx_atoms_priority ON prompt_atoms(priority DESC);
    `

    _, err := db.Exec(schema)
    if err != nil {
        return fmt.Errorf("create prompt_atoms table: %w", err)
    }

    return nil
}

// Call before first use to ensure schema exists
func LoadAgentPrompts(db *sql.DB, yamlPath string) error {
    // Ensure table exists (safe to call multiple times)
    if err := ensurePromptAtomsTable(db); err != nil {
        return err
    }

    // Load YAML data into table
    return loadYAMLIntoTable(db, yamlPath)
}
```

**Key Points:**

- Use `CREATE TABLE IF NOT EXISTS` for idempotency
- Use `CREATE INDEX IF NOT EXISTS` for indexes
- Call schema creation before any table operations
- Safe to call multiple times (no-op if exists)

### Migrating Existing Databases

```go
// AddColumn adds a column if it doesn't exist (SQLite limitation workaround)
func AddColumnIfNotExists(db *sql.DB, table, column, datatype string) error {
    // Check if column exists
    rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
    if err != nil {
        return fmt.Errorf("query table info: %w", err)
    }
    defer rows.Close()

    exists := false
    var cid int
    var name, dataType string
    var notNull, pk int
    var dfltValue *string

    for rows.Next() {
        if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
            return fmt.Errorf("scan column info: %w", err)
        }
        if name == column {
            exists = true
            break
        }
    }

    if exists {
        return nil // Column already exists
    }

    // Add column
    _, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, datatype))
    return err
}
```

## Multi-Source Compilation Pattern

The JIT Prompt Compiler implements a **System 2 Architecture** for prompt engineering.

### Architectural Principles

| Principle | Implementation | Why It Matters |
|-----------|----------------|----------------|
| **Skeleton vs. Flesh** | Mandatory atoms (Mangle) + Optional atoms (Vector) | Prevents "Frankenstein Prompt" |
| **Mangle as Gatekeeper** | Use Datalog for inference, Go for arithmetic | Mangle isn't a calculator |
| **Normalized Tags** | `atom_context_tags` link table | Zero JSON parsing overhead |
| **Atom Polymorphism** | `content`, `content_concise`, `content_min` | Graceful degradation |
| **Prompt Manifest** | Flight recorder for every compilation | Ouroboros debugging |

### JIT Compiler with Tiered Sources

```go
// JITPromptCompiler aggregates atoms from multiple sources
type JITPromptCompiler struct {
    // Tier 1: Immutable embedded corpus (highest priority)
    embeddedCorpus *EmbeddedCorpus

    // Tier 2: Project-level database
    projectDB *sql.DB

    // Tier 3: Per-shard databases (keyed by shard ID)
    shardDBs map[string]*sql.DB

    // Kernel for selection logic
    kernel *core.Kernel

    mu sync.RWMutex
}

// Compile aggregates from all registered sources
func (c *JITPromptCompiler) Compile(ctx context.Context, cc *CompilationContext) (*CompilationResult, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    var allCandidates []*PromptAtom

    // 1. Load from embedded corpus (always first, highest priority)
    if c.embeddedCorpus != nil {
        allCandidates = append(allCandidates, c.embeddedCorpus.All()...)
    }

    // 2. Load from project DB (if available)
    if c.projectDB != nil {
        projectAtoms, err := c.loadAtomsFromDB(ctx, c.projectDB)
        if err != nil {
            logging.Warn("Failed to load project atoms: %v", err)
            // Continue without project atoms - not fatal
        } else {
            allCandidates = append(allCandidates, projectAtoms...)
        }
    }

    // 3. Load from shard-specific DB (if registered for this shard)
    if cc.ShardID != "" {
        if shardDB, ok := c.shardDBs[cc.ShardID]; ok {
            shardAtoms, err := c.loadAtomsFromDB(ctx, shardDB)
            if err != nil {
                logging.Warn("Failed to load shard atoms: %v", err)
            } else {
                allCandidates = append(allCandidates, shardAtoms...)
            }
        }
    }

    // 4. Run selection logic (Mangle + vector search)
    selected := c.selector.SelectAtoms(ctx, allCandidates, cc)

    // 5. Assemble final prompt
    return c.assembler.Assemble(selected, cc.TokenBudget)
}
```

**Key Points:**

- **Skeleton first**: Embedded corpus provides mandatory atoms—compilation fails if missing
- **Flesh second**: Project/shard DBs provide contextual atoms—failures are warnings
- **Graceful degradation**: Missing sources result in less helpful but functional prompts

### Registering Dynamic Sources

```go
// RegisterShardDB registers a shard-specific database
func (c *JITPromptCompiler) RegisterShardDB(shardID string, db *sql.DB) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.shardDBs[shardID] = db
    logging.Info("Registered shard DB: %s", shardID)
}

// UnregisterShardDB removes and closes a shard database
func (c *JITPromptCompiler) UnregisterShardDB(shardID string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if db, ok := c.shardDBs[shardID]; ok {
        if err := db.Close(); err != nil {
            logging.Warn("Failed to close shard DB %s: %v", shardID, err)
        }
        delete(c.shardDBs, shardID)
        logging.Info("Unregistered shard DB: %s", shardID)
    }
}
```

## FeedbackLoop Adapter Pattern

When integrating with the Mangle FeedbackLoop for LLM-generated rules.

### Adapter Implementation

```go
// feedback.LLMClient interface (internal/mangle/feedback/loop.go)
type LLMClient interface {
    Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// Adapter wrapping a shard's LLM client
type executiveLLMAdapter struct {
    shard *ExecutivePolicyShard
    ctx   context.Context
}

// Implement the interface with proper guard clause and cost guard integration
func (a *executiveLLMAdapter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
    // Guard: check LLM client availability
    if a.shard.LLMClient == nil {
        return "", fmt.Errorf("no LLM client configured")
    }

    // Cost guard check for each retry attempt
    if a.shard.CostGuard != nil {
        can, reason := a.shard.CostGuard.CanCall()
        if !can {
            return "", fmt.Errorf("LLM call blocked: %s", reason)
        }
    }

    // Delegate to the actual LLM client
    return a.shard.GuardedLLMCall(ctx, systemPrompt, userPrompt)
}
```

### Usage in Shards

```go
// Create adapter and call FeedbackLoop
llmAdapter := &executiveLLMAdapter{shard: e, ctx: ctx}

result, err := e.feedbackLoop.GenerateAndValidate(
    ctx,
    llmAdapter,           // Adapter satisfies feedback.LLMClient
    e.Kernel,             // Kernel satisfies feedback.RuleValidator
    systemPrompt,
    userPrompt,
    "executive",          // Domain for valid examples
)

if err != nil {
    logging.SystemShards("Generation failed after %d attempts: %v", result.Attempts, err)
    return err
}

// Use the validated rule
if result.AutoFixed {
    confidence = 0.75  // Sanitizer made corrections
}
validatedRule := result.Rule
```

**Key Pattern Points:**

1. **Adapter per shard** - Each shard creates its own adapter wrapping its LLM client
2. **Cost guard integration** - Check `CanCall()` on each retry, not just first attempt
3. **Context propagation** - Pass context through for cancellation
4. **Error wrapping** - Return meaningful errors with context
5. **Guard clauses** - Early return if LLM client unavailable
