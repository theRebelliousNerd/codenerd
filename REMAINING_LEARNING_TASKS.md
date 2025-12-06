# Remaining Learning Loop Tasks - Implementation Roadmap

**Priority**: CRITICAL - Complete the learning loop from "Execute → Store → Learn → Apply"

**Status**: Foundation built (knowledge persistence + embeddings), 4 tasks remain

---

## Task 1: Virtual Predicates for Mangle Kernel Queries ⏳ CRITICAL

### Problem
Mangle kernel cannot query knowledge.db - learned facts are stored but not accessible to logic rules.

### Solution
Add virtual predicates that FFI to LocalStore, allowing Mangle to query learned knowledge.

### Implementation

#### Step 1: Add Virtual Predicate Declarations to schemas.gl

```mangle
# File: internal/mangle/schemas.gl

# Virtual predicates for knowledge queries (Bound - resolved by VirtualStore)
Decl query_learned(Predicate:String, Args:List) Bound.
Decl query_session(SessionID:String, Turn:Number, Input:String) Bound.
Decl recall_similar(Query:String, Limit:Number, Results:List) Bound.
Decl knowledge_graph_link(EntityA:String, Relation:String, EntityB:String) Bound.
Decl recent_activations(Predicate:String, Score:Number) Bound.
```

#### Step 2: Implement Virtual Predicate Handlers in VirtualStore

**File**: `internal/core/virtual_store.go`

```go
// Add LocalStore field
type VirtualStore struct {
    // ... existing fields ...
    localDB *store.LocalStore  // NEW: Knowledge database
}

// SetLocalDB configures the knowledge database
func (vs *VirtualStore) SetLocalDB(db *store.LocalStore) {
    vs.mu.Lock()
    defer vs.mu.Unlock()
    vs.localDB = db
}

// ResolveVirtualPredicate handles Bound predicates from Mangle
func (vs *VirtualStore) ResolveVirtualPredicate(predicate string, args []interface{}) ([]Fact, error) {
    switch predicate {
    case "query_learned":
        return vs.queryLearned(args)
    case "query_session":
        return vs.querySession(args)
    case "recall_similar":
        return vs.recallSimilar(args)
    case "knowledge_graph_link":
        return vs.knowledgeGraphLink(args)
    case "recent_activations":
        return vs.recentActivations(args)
    default:
        return nil, fmt.Errorf("unknown virtual predicate: %s", predicate)
    }
}

// queryLearned retrieves learned facts from cold_storage
func (vs *VirtualStore) queryLearned(args []interface{}) ([]Fact, error) {
    if len(args) < 1 {
        return nil, fmt.Errorf("query_learned requires predicate argument")
    }

    predicateName, ok := args[0].(string)
    if !ok {
        return nil, fmt.Errorf("predicate must be string")
    }

    if vs.localDB == nil {
        return nil, nil // No knowledge DB configured
    }

    // Query cold storage
    storedFacts, err := vs.localDB.LoadFacts(predicateName)
    if err != nil {
        return nil, err
    }

    // Convert to Mangle facts
    var facts []Fact
    for _, sf := range storedFacts {
        facts = append(facts, Fact{
            Predicate: sf.Predicate,
            Args:      sf.Args,
        })
    }

    return facts, nil
}

// querySession retrieves session history
func (vs *VirtualStore) querySession(args []interface{}) ([]Fact, error) {
    if len(args) < 1 {
        return nil, fmt.Errorf("query_session requires session_id argument")
    }

    sessionID, ok := args[0].(string)
    if !ok {
        return nil, fmt.Errorf("session_id must be string")
    }

    if vs.localDB == nil {
        return nil, nil
    }

    history, err := vs.localDB.GetSessionHistory(sessionID, 50)
    if err != nil {
        return nil, err
    }

    var facts []Fact
    for _, turn := range history {
        facts = append(facts, Fact{
            Predicate: "session_turn",
            Args: []interface{}{
                sessionID,
                turn["turn_number"],
                turn["user_input"],
                turn["response"],
            },
        })
    }

    return facts, nil
}

// recallSimilar performs semantic vector search
func (vs *VirtualStore) recallSimilar(args []interface{}) ([]Fact, error) {
    if len(args) < 2 {
        return nil, fmt.Errorf("recall_similar requires query and limit")
    }

    query, ok := args[0].(string)
    if !ok {
        return nil, fmt.Errorf("query must be string")
    }

    limit, ok := args[1].(int)
    if !ok {
        limit = 10
    }

    if vs.localDB == nil {
        return nil, nil
    }

    ctx := context.Background()
    results, err := vs.localDB.VectorRecallSemantic(ctx, query, limit)
    if err != nil {
        // Fallback to keyword search
        results, err = vs.localDB.VectorRecall(query, limit)
        if err != nil {
            return nil, err
        }
    }

    var facts []Fact
    for _, result := range results {
        similarity := 0.0
        if sim, ok := result.Metadata["similarity"].(float64); ok {
            similarity = sim
        }

        facts = append(facts, Fact{
            Predicate: "similar_content",
            Args:      []interface{}{query, result.Content, similarity},
        })
    }

    return facts, nil
}

// knowledgeGraphLink queries the knowledge graph
func (vs *VirtualStore) knowledgeGraphLink(args []interface{}) ([]Fact, error) {
    if len(args) < 1 {
        return nil, fmt.Errorf("knowledge_graph_link requires entity argument")
    }

    entity, ok := args[0].(string)
    if !ok {
        return nil, fmt.Errorf("entity must be string")
    }

    if vs.localDB == nil {
        return nil, nil
    }

    links, err := vs.localDB.QueryLinks(entity, "both")
    if err != nil {
        return nil, err
    }

    var facts []Fact
    for _, link := range links {
        facts = append(facts, Fact{
            Predicate: "graph_link",
            Args:      []interface{}{link.EntityA, link.Relation, link.EntityB, link.Weight},
        })
    }

    return facts, nil
}

// recentActivations queries activation log
func (vs *VirtualStore) recentActivations(args []interface{}) ([]Fact, error) {
    if vs.localDB == nil {
        return nil, nil
    }

    activations, err := vs.localDB.GetRecentActivations(100, 5.0)
    if err != nil {
        return nil, err
    }

    var facts []Fact
    for factID, score := range activations {
        facts = append(facts, Fact{
            Predicate: "activation",
            Args:      []interface{}{factID, score},
        })
    }

    return facts, nil
}
```

#### Step 3: Wire VirtualStore to LocalDB

**File**: `cmd/nerd/chat/session.go`

```go
// After creating localDB (line ~124)
virtualStore.SetLocalDB(localDB)
```

#### Step 4: Use in Policy Rules

**File**: `internal/mangle/policy.gl`

```mangle
# §20: LEARNED KNOWLEDGE QUERIES
# =============================================================================

# User preference recall
user_prefers(Style) :-
    query_learned(/user_preference, Args),
    Args = [Style].

# Session context recall
recent_user_request(Request) :-
    current_session(SessionID),
    query_session(SessionID, Turn, Request, _),
    current_turn(CurrentTurn),
    Turn > CurrentTurn - 5.  # Last 5 turns

# Semantic knowledge recall
similar_past_solution(Query, Solution) :-
    recall_similar(Query, 5, Results),
    member([Query, Solution, Similarity], Results),
    Similarity > 0.7.  # High similarity threshold

# Knowledge graph traversal
user_project_uses(Technology) :-
    knowledge_graph_link(/user, /working_on, Project),
    knowledge_graph_link(Project, /uses, Technology).

# Trending facts (frequently activated)
trending_fact(Predicate) :-
    recent_activations(Predicate, Score),
    Score > 50.0.  # High activation score
```

**Status**: ⏳ TODO
**Effort**: 4-6 hours
**Blockers**: None (all infrastructure exists)

---

## Task 2: Session JSON + SQLite Integration ⏳ CRITICAL

### Problem
Dual session storage: `.nerd/sessions/*.json` files AND `knowledge.db.session_history` table exist but are not synchronized.

### Current State
- **JSON files**: Created by session.go, contain full chat history
- **SQLite table**: Populated by persistence.go, structured session data
- **Issue**: No synchronization, potential data loss

### Solution Options

#### Option A: Dual Persistence (RECOMMENDED)
Keep both, sync on every turn.

**Pros**:
- Backward compatible
- JSON for human inspection
- SQLite for queries
- Redundancy (backup)

**Cons**:
- 2x write operations
- Potential desync

#### Option B: SQLite Primary
Use SQLite as source of truth, JSON as cache/export.

**Pros**:
- Single source of truth
- Better queryability
- Proper transactions

**Cons**:
- Breaking change
- Requires migration

#### Option C: Migration on Startup
Migrate old JSON sessions to SQLite on first run.

**Pros**:
- One-time migration
- Clean SQLite-only future

**Cons**:
- Complex migration logic
- Old sessions potentially lost

### Implementation (Option A - Dual Persistence)

**File**: `cmd/nerd/chat/session.go`

```go
// After saving session JSON (line ~somewhere)
func (s *Session) saveTurn(turn Message) error {
    // 1. Save to JSON (existing code)
    s.mu.Lock()
    s.Messages = append(s.Messages, turn)
    s.UpdatedAt = time.Now()
    s.mu.Unlock()

    if err := s.saveToFile(); err != nil {
        return err
    }

    // 2. Save to SQLite (NEW)
    if s.localDB != nil {
        intentJSON := "{}" // Parse from turn metadata
        atomsJSON := "[]"  // Parse from turn metadata

        err := s.localDB.StoreSessionTurn(
            s.ID,
            s.TurnNumber,
            turn.Content,      // user input or ""
            intentJSON,
            turn.Content,      // response
            atomsJSON,
        )
        if err != nil {
            // Log but don't fail
            fmt.Printf("[Session] Warning: Failed to store to SQLite: %v\n", err)
        }
    }

    return nil
}

// Migration function
func (s *Session) migrateToSQLite() error {
    if s.localDB == nil {
        return nil
    }

    // Check if already migrated
    existing, err := s.localDB.GetSessionHistory(s.ID, 1)
    if err == nil && len(existing) > 0 {
        return nil // Already migrated
    }

    // Migrate all turns
    for i := 0; i < len(s.Messages); i += 2 {
        if i+1 >= len(s.Messages) {
            break
        }

        userMsg := s.Messages[i]
        assistantMsg := s.Messages[i+1]

        err := s.localDB.StoreSessionTurn(
            s.ID,
            i/2,
            userMsg.Content,
            "{}",
            assistantMsg.Content,
            "[]",
        )
        if err != nil {
            return err
        }
    }

    return nil
}
```

**Status**: ⏳ TODO
**Effort**: 2-3 hours
**Blockers**: None

---

## Task 3: Post-Execution Verification Loop ⏳ CRITICAL

### Problem
Agent doesn't verify if it actually succeeded at the task - no self-assessment or learning from failures.

### Solution
Add verification step after execution: Agent checks its own work, learns from success/failure.

### Implementation

#### Step 1: Create TaskVerifier

**File**: `internal/verification/verifier.go` (NEW)

```go
package verification

import (
    "context"
    "codenerd/internal/perception"
    "fmt"
    "strings"
)

// TaskVerifier performs post-execution verification
type TaskVerifier struct {
    client perception.LLMClient
}

// NewTaskVerifier creates a new verifier
func NewTaskVerifier(client perception.LLMClient) *TaskVerifier {
    return &TaskVerifier{client: client}
}

// VerificationResult contains verification outcome
type VerificationResult struct {
    Success     bool
    Confidence  float64
    Reason      string
    Suggestions []string
    Evidence    []string
}

// VerifyTask checks if a task was completed successfully
func (v *TaskVerifier) VerifyTask(ctx context.Context, task, result string) (*VerificationResult, error) {
    systemPrompt := `You are a task completion verifier. Assess if the task was completed successfully.

Return JSON:
{
  "success": true/false,
  "confidence": 0.0-1.0,
  "reason": "explanation",
  "suggestions": ["improvement 1", ...],
  "evidence": ["evidence of success/failure"]
}`

    userPrompt := fmt.Sprintf(`Task: %s

Result: %s

Did the task succeed? Provide evidence.`, task, result)

    response, err := v.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
    if err != nil {
        return nil, err
    }

    // Parse JSON response
    var verification VerificationResult
    // ... JSON parsing ...

    return &verification, nil
}
```

#### Step 2: Wire into OODA Loop

**File**: `cmd/nerd/chat/process.go`

```go
// After shard execution (line ~58-65)
result, spawnErr := m.shardMgr.Spawn(ctx, shardType, task)
if spawnErr != nil {
    return errorMsg(fmt.Errorf("shard delegation failed: %w", spawnErr))
}

// VERIFICATION LOOP (NEW)
if m.verifier != nil {
    verification, verifyErr := m.verifier.VerifyTask(ctx, task, result)
    if verifyErr == nil {
        // Store verification results
        if m.localDB != nil {
            if verification.Success {
                // Reinforce successful pattern
                m.localDB.StoreFact("task_succeeded",
                    []interface{}{intent.Verb, intent.Target, verification.Confidence},
                    "success", 7)
            } else {
                // Learn from failure
                m.localDB.StoreFact("task_failed",
                    []interface{}{intent.Verb, intent.Target, verification.Reason},
                    "failure", 8)
            }
        }

        // Include verification in response
        if !verification.Success && verification.Confidence > 0.7 {
            result += fmt.Sprintf("\n\n⚠️ Verification: Task may not be complete. %s", verification.Reason)
        }
    }
}

// Format response...
```

#### Step 3: Add to Model

**File**: `cmd/nerd/chat/model.go`

```go
type Model struct {
    // ... existing fields ...
    verifier *verification.TaskVerifier  // NEW
}
```

**File**: `cmd/nerd/chat/session.go`

```go
// After creating client (line ~80ish)
verifier := verification.NewTaskVerifier(llmClient)

// In Model initialization (line ~300ish)
Model{
    // ... existing fields ...
    verifier: verifier,  // NEW
}
```

**Status**: ⏳ TODO
**Effort**: 3-4 hours
**Blockers**: None

---

## Task 4: Reasoning Trace Capture for Main OODA ⏳ CRITICAL

### Problem
Reasoning traces are only captured for autopoiesis tool generation, not for main OODA tasks. No way to analyze "what was the agent thinking?"

### Solution
Extend reasoning trace system to capture chain-of-thought for all main tasks.

### Implementation

#### Step 1: Add reasoning_traces Table

**File**: `internal/store/local.go` (modify initialize())

```go
// Add to initialize() method
reasoningTable := `
CREATE TABLE IF NOT EXISTS reasoning_traces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    turn_number INTEGER NOT NULL,
    task TEXT NOT NULL,
    intent_json TEXT,
    chain_of_thought TEXT,
    key_decisions TEXT,
    assumptions TEXT,
    alternatives TEXT,
    outcome TEXT,
    success BOOLEAN,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_reasoning_session ON reasoning_traces(session_id);
CREATE INDEX IF NOT EXISTS idx_reasoning_turn ON reasoning_traces(turn_number);
`
```

#### Step 2: Capture Reasoning from Articulation

**File**: `cmd/nerd/chat/process.go`

```go
// In articulation call (line ~212)
artOutput, err := articulateWithContextFull(ctx, m.client, intent, payload, contextFacts, warnings, systemPrompt)

// NEW: Capture reasoning trace
if m.localDB != nil {
    reasoningTrace := extractReasoningTrace(artOutput)

    err := m.localDB.StoreReasoningTrace(
        m.sessionID,
        m.turnCount,
        input,
        string(intentJSON),
        reasoningTrace.ChainOfThought,
        reasoningTrace.KeyDecisions,
        reasoningTrace.Assumptions,
        reasoningTrace.Alternatives,
        response,
        true, // success (or check verification result)
    )
    if err != nil {
        fmt.Printf("[Reasoning] Warning: Failed to store trace: %v\n", err)
    }
}
```

#### Step 3: Add Storage Method

**File**: `internal/store/reasoning_traces.go` (NEW)

```go
package store

// StoreReasoningTrace persists reasoning trace for a turn
func (s *LocalStore) StoreReasoningTrace(
    sessionID string,
    turnNumber int,
    task string,
    intentJSON string,
    chainOfThought string,
    keyDecisions string,
    assumptions string,
    alternatives string,
    outcome string,
    success bool,
) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    _, err := s.db.Exec(`
        INSERT INTO reasoning_traces
        (session_id, turn_number, task, intent_json, chain_of_thought,
         key_decisions, assumptions, alternatives, outcome, success)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        sessionID, turnNumber, task, intentJSON, chainOfThought,
        keyDecisions, assumptions, alternatives, outcome, success,
    )

    return err
}

// GetReasoningTraces retrieves reasoning traces for analysis
func (s *LocalStore) GetReasoningTraces(sessionID string, limit int) ([]ReasoningTrace, error) {
    // ... implementation ...
}

type ReasoningTrace struct {
    ID             int64
    SessionID      string
    TurnNumber     int
    Task           string
    ChainOfThought string
    KeyDecisions   string
    Assumptions    string
    Alternatives   string
    Outcome        string
    Success        bool
    CreatedAt      time.Time
}
```

#### Step 4: Extract Reasoning from LLM Response

**File**: `cmd/nerd/chat/articulation.go`

```go
// extractReasoningTrace parses reasoning from articulation output
func extractReasoningTrace(artOutput *ArticulationOutput) ReasoningTrace {
    trace := ReasoningTrace{}

    // If using thinking mode, extract thought content
    if artOutput.ThinkingContent != "" {
        trace.ChainOfThought = artOutput.ThinkingContent
    }

    // Parse key decisions from surface response
    // Look for decision markers: "I decided to...", "The approach is...", etc.
    decisions := extractDecisions(artOutput.Surface)
    trace.KeyDecisions = strings.Join(decisions, "\n")

    // Parse assumptions: "Assuming...", "Given that...", etc.
    assumptions := extractAssumptions(artOutput.Surface)
    trace.Assumptions = strings.Join(assumptions, "\n")

    // Parse alternatives: "Alternatively...", "Another option...", etc.
    alternatives := extractAlternatives(artOutput.Surface)
    trace.Alternatives = strings.Join(alternatives, "\n")

    return trace
}
```

**Status**: ⏳ TODO
**Effort**: 4-5 hours
**Blockers**: None

---

## Integration Checklist

### Phase 1: Virtual Predicates (Week 1)
- [ ] Add virtual predicate declarations to schemas.gl
- [ ] Implement handlers in VirtualStore
- [ ] Wire LocalDB to VirtualStore
- [ ] Add policy rules using virtual predicates
- [ ] Test with `/query` commands
- [ ] Verify learned knowledge is accessible

### Phase 2: Session Sync (Week 1)
- [ ] Choose sync strategy (Option A recommended)
- [ ] Implement dual persistence in session.go
- [ ] Add migration function for old sessions
- [ ] Test session save/load
- [ ] Verify JSON and SQLite stay in sync

### Phase 3: Verification (Week 2)
- [ ] Create TaskVerifier class
- [ ] Wire into OODA loop after shard execution
- [ ] Store success/failure learnings
- [ ] Add verification feedback to responses
- [ ] Test with various task types
- [ ] Measure verification accuracy

### Phase 4: Reasoning Traces (Week 2)
- [ ] Add reasoning_traces table
- [ ] Implement storage methods
- [ ] Extract reasoning from articulation
- [ ] Capture for all OODA turns
- [ ] Create analysis queries
- [ ] Build reasoning trace viewer

---

## Success Metrics

**Virtual Predicates**:
- [ ] Mangle can query learned preferences
- [ ] Knowledge graph traversal works
- [ ] Semantic search accessible from policy

**Session Sync**:
- [ ] JSON and SQLite match 100%
- [ ] Old sessions migrated successfully
- [ ] No data loss

**Verification**:
- [ ] 90%+ accuracy on success detection
- [ ] Failure patterns captured
- [ ] Suggestions improve outcomes

**Reasoning Traces**:
- [ ] Every turn has reasoning captured
- [ ] Chain-of-thought retrievable
- [ ] Analysis queries work

---

## Dependencies

**All 4 tasks depend on**:
- ✅ Knowledge persistence (DONE)
- ✅ LocalStore infrastructure (DONE)
- ✅ OODA loop hooks (DONE)

**Task 1 depends on**:
- ✅ VirtualStore architecture (EXISTS)
- ✅ Mangle Bound predicates (SUPPORTED)

**Task 2 depends on**:
- ✅ Session management (EXISTS)
- ✅ SQLite session_history table (EXISTS)

**Task 3 depends on**:
- ✅ LLM client (EXISTS)
- ✅ Fact storage (EXISTS)

**Task 4 depends on**:
- ✅ Articulation output (EXISTS)
- ✅ Database schema extensibility (EXISTS)

**All tasks are READY TO IMPLEMENT** - no blockers!

---

## Estimated Timeline

**Week 1**:
- Days 1-2: Virtual Predicates
- Days 3-4: Session Sync
- Day 5: Testing

**Week 2**:
- Days 1-2: Verification Loop
- Days 3-4: Reasoning Traces
- Day 5: Integration Testing

**Total**: ~2 weeks for complete learning loop

---

## Testing Plan

### Test 1: Virtual Predicates
```bash
./nerd.exe
> Tell me you prefer TDD
> /query user_prefers(?Style)
# Expected: user_prefers("TDD")
```

### Test 2: Session Sync
```bash
./nerd.exe
> What is codeNERD?
^C

# Check JSON
cat .nerd/sessions/latest.json

# Check SQLite
python -c "import sqlite3; conn = sqlite3.connect('.nerd/knowledge.db'); \
    print(conn.execute('SELECT * FROM session_history').fetchall())"

# Should match
```

### Test 3: Verification
```bash
./nerd.exe
> Fix the nonexistent bug in fake.go
# Expected: "⚠️ Verification: Task may not be complete. File fake.go does not exist"
```

### Test 4: Reasoning Traces
```bash
./nerd.exe
> Explain the OODA loop
^C

# Check reasoning
python -c "import sqlite3; conn = sqlite3.connect('.nerd/knowledge.db'); \
    traces = conn.execute('SELECT chain_of_thought FROM reasoning_traces').fetchall(); \
    print(traces[0][0] if traces else 'No traces')"

# Should show LLM's thought process
```

---

---

## Task 5: Embedding System Integration ⏳ CRITICAL

### Problem
Embedding engine is built but not integrated into config system or wired into persistence layer.

### Solution
Complete embedding integration: config → initialization → persistence → TUI commands.

### Implementation

#### Step 1: Add Embedding Configuration to Config System

**File**: `internal/config/config.go`

```go
// Add to Config struct (line ~38)
type Config struct {
    // ... existing fields ...

    // Embedding engine configuration
    Embedding EmbeddingConfig `yaml:"embedding" json:"embedding"`
}

// Add new config struct
type EmbeddingConfig struct {
    Provider       string `yaml:"provider" json:"provider"`             // "ollama" or "genai"
    OllamaEndpoint string `yaml:"ollama_endpoint" json:"ollama_endpoint"` // Default: "http://localhost:11434"
    OllamaModel    string `yaml:"ollama_model" json:"ollama_model"`       // Default: "embeddinggemma"
    GenAIAPIKey    string `yaml:"genai_api_key" json:"genai_api_key"`
    GenAIModel     string `yaml:"genai_model" json:"genai_model"`         // Default: "gemini-embedding-001"
    TaskType       string `yaml:"task_type" json:"task_type"`             // Default: "SEMANTIC_SIMILARITY"
}

// Add to DefaultConfig() (line ~196)
func DefaultConfig() Config {
    return Config{
        // ... existing defaults ...

        Embedding: EmbeddingConfig{
            Provider:       "ollama",
            OllamaEndpoint: "http://localhost:11434",
            OllamaModel:    "embeddinggemma",
            GenAIModel:     "gemini-embedding-001",
            TaskType:       "SEMANTIC_SIMILARITY",
        },
    }
}
```

**File**: `.nerd/config.json` (User's config)

```json
{
  "provider": "zai",
  "zai_api_key": "b669cee5811e48389056bd7f68757fe8.YFRqFAvC8icCGePD",
  "model": "glm-4.6",
  "theme": "light",
  "context7_api_key": "ctx7sk-4a42a7f3-78c2-472f-b73a-61c62bead461",

  "embedding": {
    "provider": "ollama",
    "ollama_endpoint": "http://localhost:11434",
    "ollama_model": "embeddinggemma"
  }
}
```

#### Step 2: Initialize Embedding Engine on Startup

**File**: `cmd/nerd/chat/session.go`

```go
import (
    "codenerd/internal/embedding"
    // ... other imports ...
)

// After creating localDB (line ~124)
localDB := db

// Initialize embedding engine from config
var embeddingEngine embedding.EmbeddingEngine
if appCfg.Embedding.Provider != "" {
    embCfg := embedding.Config{
        Provider:       appCfg.Embedding.Provider,
        OllamaEndpoint: appCfg.Embedding.OllamaEndpoint,
        OllamaModel:    appCfg.Embedding.OllamaModel,
        GenAIAPIKey:    appCfg.Embedding.GenAIAPIKey,
        GenAIModel:     appCfg.Embedding.GenAIModel,
        TaskType:       appCfg.Embedding.TaskType,
    }

    engine, err := embedding.NewEngine(embCfg)
    if err != nil {
        initialMessages = append(initialMessages, Message{
            Role:    "assistant",
            Content: fmt.Sprintf("Warning: Failed to initialize embedding engine: %v\nUsing keyword search fallback.", err),
            Time:    time.Now(),
        })
    } else {
        localDB.SetEmbeddingEngine(engine)
        initialMessages = append(initialMessages, Message{
            Role:    "assistant",
            Content: fmt.Sprintf("✓ Embedding engine: %s", engine.Name()),
            Time:    time.Now(),
        })
    }
}
```

#### Step 3: Update Persistence to Use Intelligent Task Selection

**File**: `cmd/nerd/chat/persistence.go`

```go
import (
    "codenerd/internal/embedding"
    // ... other imports ...
)

// Modify persistTurnToKnowledge (line ~60-75)
func (m Model) persistTurnToKnowledge(turn ctxcompress.Turn, intent perception.Intent, response string) {
    // ... existing session_id and turn_number logic ...

    // 2. VECTORS: Store with intelligent task type selection
    userMeta := map[string]interface{}{
        "type":       "user_input",
        "session_id": sessionID,
        "turn":       turnNumber,
        "verb":       intent.Verb,
        "category":   intent.Category,
    }

    // Use embedding if available, otherwise fallback to keyword
    ctx := context.Background()
    if err := m.localDB.StoreVectorWithEmbedding(ctx, turn.UserInput, userMeta); err != nil {
        // Fallback to non-embedded storage
        m.localDB.StoreVector(turn.UserInput, userMeta)
    }

    responseMeta := map[string]interface{}{
        "type":       "assistant_response",
        "session_id": sessionID,
        "turn":       turnNumber,
    }

    if err := m.localDB.StoreVectorWithEmbedding(ctx, response, responseMeta); err != nil {
        m.localDB.StoreVector(response, responseMeta)
    }

    // ... rest of method ...
}
```

#### Step 4: Wire Shard Agents to Their Knowledge DBs

**File**: `cmd/nerd/chat/session.go`

```go
// After registering shards (line ~126-143)
shardMgr.RegisterShard("coder", func(id string, config core.ShardConfig) core.ShardAgent {
    shard := shards.NewCoderShard()
    shard.SetVirtualStore(virtualStore)
    shard.SetLLMClient(llmClient)

    // NEW: Set shard's own knowledge DB
    shardDBPath := filepath.Join(workspace, ".nerd", "shards", "coder_knowledge.db")
    if shardDB, err := store.NewLocalStore(shardDBPath); err == nil {
        if embeddingEngine != nil {
            shardDB.SetEmbeddingEngine(embeddingEngine)
        }
        shard.SetLocalDB(shardDB)
    }

    return shard
})

// Repeat for tester, reviewer, researcher
```

**File**: `internal/shards/coder.go` (and tester.go, reviewer.go)

```go
type CoderShard struct {
    // ... existing fields ...
    localDB *store.LocalStore  // NEW
}

func (c *CoderShard) SetLocalDB(db *store.LocalStore) {
    c.localDB = db
}

// In Execute method, store learnings
func (c *CoderShard) Execute(ctx context.Context, task string) (string, error) {
    // ... existing execution logic ...

    // Store code snippets generated
    if c.localDB != nil {
        metadata := map[string]interface{}{
            "type": "code",
            "language": detectedLanguage,
            "task": task,
        }
        c.localDB.StoreVectorWithEmbedding(ctx, generatedCode, metadata)
    }

    return result, nil
}
```

#### Step 5: Add TUI Commands for Embedding Configuration

**File**: `cmd/nerd/chat/commands.go` (NEW or add to existing command handler)

```go
// Add to command switch
case "/set-embedding":
    return m.handleSetEmbedding(parts[1:])

case "/reembed":
    return m.handleReembed()

case "/embedding-stats":
    return m.handleEmbeddingStats()

// Handler implementations
func (m Model) handleSetEmbedding(args []string) tea.Cmd {
    if len(args) < 1 {
        return responseMsg("Usage: /set-embedding <ollama|genai> [api-key]")
    }

    provider := args[0]

    // Update config
    configPath := filepath.Join(m.workspace, ".nerd", "config.json")
    configData, err := os.ReadFile(configPath)
    if err != nil {
        return errorMsg(fmt.Errorf("failed to read config: %w", err))
    }

    var config map[string]interface{}
    json.Unmarshal(configData, &config)

    embeddingConfig := map[string]interface{}{
        "provider": provider,
    }

    if provider == "ollama" {
        embeddingConfig["ollama_endpoint"] = "http://localhost:11434"
        embeddingConfig["ollama_model"] = "embeddinggemma"
    } else if provider == "genai" {
        if len(args) < 2 {
            return responseMsg("GenAI requires API key: /set-embedding genai YOUR_API_KEY")
        }
        embeddingConfig["genai_api_key"] = args[1]
        embeddingConfig["genai_model"] = "gemini-embedding-001"
        embeddingConfig["task_type"] = "SEMANTIC_SIMILARITY"
    }

    config["embedding"] = embeddingConfig

    // Save config
    updatedData, _ := json.MarshalIndent(config, "", "  ")
    os.WriteFile(configPath, updatedData, 0644)

    return responseMsg(fmt.Sprintf("✓ Embedding provider set to: %s\nRestart to apply changes.", provider))
}

func (m Model) handleReembed() tea.Cmd {
    return func() tea.Msg {
        if m.localDB == nil {
            return errorMsg(fmt.Errorf("no knowledge database"))
        }

        ctx := context.Background()
        err := m.localDB.ReembedAllVectors(ctx)
        if err != nil {
            return errorMsg(err)
        }

        stats, _ := m.localDB.GetVectorStats()
        return responseMsg(fmt.Sprintf("✓ Re-embedding complete!\n%v", stats))
    }
}

func (m Model) handleEmbeddingStats() tea.Cmd {
    return func() tea.Msg {
        if m.localDB == nil {
            return errorMsg(fmt.Errorf("no knowledge database"))
        }

        stats, err := m.localDB.GetVectorStats()
        if err != nil {
            return errorMsg(err)
        }

        return responseMsg(fmt.Sprintf(`Embedding Statistics:
Total Vectors: %v
With Embeddings: %v
Without Embeddings: %v
Engine: %v
Dimensions: %v`,
            stats["total_vectors"],
            stats["with_embeddings"],
            stats["without_embeddings"],
            stats["embedding_engine"],
            stats["embedding_dimensions"]))
    }
}
```

#### Step 6: Add to Init System

**File**: `cmd/nerd/main.go` (modify runInit function)

```go
func runInit(cmd *cobra.Command, args []string) error {
    // ... existing init logic ...

    // NEW: Initialize embedding engine
    fmt.Println("Initializing embedding engine...")

    // Check if Ollama is available
    resp, err := http.Get("http://localhost:11434/api/tags")
    if err == nil && resp.StatusCode == 200 {
        // Ollama is running
        fmt.Println("✓ Ollama detected - configuring embeddinggemma")

        // Set in config
        embeddingConfig := map[string]interface{}{
            "provider": "ollama",
            "ollama_endpoint": "http://localhost:11434",
            "ollama_model": "embeddinggemma",
        }

        // Add to config file
        // ... save logic ...
    } else {
        fmt.Println("⚠ Ollama not detected - embeddings will use keyword search")
        fmt.Println("  To enable semantic search:")
        fmt.Println("  1. Install Ollama: https://ollama.ai")
        fmt.Println("  2. Run: ollama pull embeddinggemma")
        fmt.Println("  3. Run: nerd init --force")
    }

    return nil
}
```

**Status**: ⏳ TODO (6 sub-tasks)
**Effort**: 6-8 hours
**Blockers**: None (all infrastructure built)

---

## Priority Order

### Immediate (This Week)
1. **Embedding Integration** (HIGHEST) - Complete the embedding system
   - Config system
   - Persistence integration
   - Shard wiring
   - TUI commands
   - Init system

### Week 1-2
2. **Virtual Predicates** (CRITICAL) - Enables knowledge queries
3. **Session Sync** (HIGH) - Prevents data loss

### Week 2-3
4. **Verification** (MEDIUM) - Improves quality
5. **Reasoning Traces** (MEDIUM) - Enables debugging

**Recommendation**: Complete embedding integration first, then implement in order 2→3→4→5

---

## Files to Create

1. `internal/core/virtual_store.go` (modify - add virtual predicate handlers)
2. `internal/mangle/schemas.gl` (modify - add virtual predicate declarations)
3. `internal/mangle/policy.gl` (modify - add learned knowledge rules)
4. `cmd/nerd/chat/session.go` (modify - add dual persistence)
5. `internal/verification/verifier.go` (NEW - task verification)
6. `internal/store/reasoning_traces.go` (NEW - reasoning trace storage)
7. `cmd/nerd/chat/articulation.go` (modify - extract reasoning)

**Total**: ~1,000 lines of new code

---

## Documentation to Create

1. `VIRTUAL_PREDICATES_GUIDE.md` - How to query knowledge from Mangle
2. `VERIFICATION_SYSTEM.md` - How verification works
3. `REASONING_TRACES_GUIDE.md` - How to analyze agent reasoning
4. `COMPLETE_LEARNING_LOOP.md` - End-to-end learning architecture

---

## Current Status Summary

**Foundation**: ✅ COMPLETE
- Knowledge persistence: ✅
- Embedding engine: ✅
- Database schema: ✅
- OODA hooks: ✅

**Learning Loop**: 🟨 50% COMPLETE
- Execute: ✅ (OODA loop)
- Store: ✅ (persistence.go)
- Learn: ⏳ (need verification + traces)
- Apply: ⏳ (need virtual predicates)

**Next Session**: Start with Task 1 (Virtual Predicates) to close the loop!
