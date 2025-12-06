# Learning Architecture - Current Status & Missing Components

**Date**: 2025-12-06
**Analysis by**: Claude Sonnet 4.5

---

## Executive Summary

You're absolutely right - there are **critical missing pieces** in the learning loop! The architecture has all the building blocks, but they're **not fully wired together** for continuous learning. Here's what exists vs. what's missing:

### ✅ What EXISTS
1. **Reasoning Trace Capture** - Captures LLM thought process ([internal/autopoiesis/traces.go](internal/autopoiesis/traces.go))
2. **Learning Store** - SQLite persistence for learnings ([internal/store/learning.go](internal/store/learning.go))
3. **Feedback System** - Execution tracking ([internal/autopoiesis/feedback.go](internal/autopoiesis/feedback.go))
4. **Per-Shard Knowledge DBs** - `.nerd/shards/{shard}_learnings.db`

### ❌ What's MISSING
1. **Main agent does NOT use `.nerd/knowledge.db`** - Only shard-specific DBs exist
2. **No post-execution verification loop** - Outputs are NOT fed back to LLM to verify success
3. **Learning happens only in autopoiesis (tool generation)** - NOT in main OODA loop
4. **No self-reflection after task completion** - Missing the "did I actually succeed?" check
5. **Memory operations from control_packet are NOT persisted to SQLite** - They're captured but not stored long-term

---

## Current Architecture

### 1. Learning Stores (What Exists)

#### Per-Shard Learning DBs
**Location**: `.nerd/shards/{shard_type}_learnings.db`

**Schema** ([internal/store/learning.go:83-99](internal/store/learning.go#L83-L99)):
```sql
CREATE TABLE IF NOT EXISTS learnings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    shard_type TEXT NOT NULL,
    fact_predicate TEXT NOT NULL,
    fact_args TEXT NOT NULL,
    learned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    source_campaign TEXT DEFAULT '',
    confidence REAL DEFAULT 1.0,
    UNIQUE(fact_predicate, fact_args)
);
```

**Usage**:
- CoderShard → `.nerd/shards/coder_learnings.db`
- ReviewerShard → `.nerd/shards/reviewer_learnings.db`
- TesterShard → `.nerd/shards/tester_learnings.db`
- ResearcherShard → `.nerd/shards/researcher_learnings.db`

**Purpose**: Store patterns, preferences, anti-patterns learned by each shard type.

#### Main Knowledge DB
**Expected Location**: `.nerd/knowledge.db`
**Status**: ❌ **NOT IMPLEMENTED** - The main agent does not have a knowledge store!

---

### 2. Reasoning Trace Capture (Tool Generation Only)

**File**: [internal/autopoiesis/traces.go](internal/autopoiesis/traces.go)

**What's Captured**:
```go
type ReasoningTrace struct {
    TraceID         string
    ToolName        string
    UserRequest     string

    // LLM Interaction
    SystemPrompt    string
    UserPrompt      string
    RawResponse     string

    // Extracted Reasoning
    ChainOfThought  []ThoughtStep  // "First I..., then I..., finally I..."
    KeyDecisions    []Decision     // "Chose X because Y"
    Assumptions     []string       // "Assuming that..."
    Alternatives    []Alternative  // "Instead of X, could do Y but..."

    // Outcome
    Success        bool
    CodeGenerated  string

    // Post-Execution Feedback
    QualityScore   float64
    IssuesFound    []string
}
```

**Storage**: `.nerd/autopoiesis/reasoning_traces.json`

**Limitation**: Only used for **tool generation**, NOT for main OODA loop tasks!

---

### 3. Control Packet Memory Operations

**Specification**: [CONTROL_PACKET_SPEC.md:157-177](CONTROL_PACKET_SPEC.md#L157-L177)

**What's Captured from LLM**:
```json
{
  "memory_operations": [
    {
      "op": "promote_to_long_term",
      "key": "preference:code_style",
      "value": "concise"
    },
    {
      "op": "store_vector",
      "key": "pattern:auth_fix_20251206",
      "value": "User prefers Bearer token validation"
    },
    {
      "op": "forget",
      "key": "temp:old_session_id",
      "value": ""
    }
  ]
}
```

**Current Implementation**: [cmd/nerd/chat/process.go:219-286](cmd/nerd/chat/process.go#L219-L286)
- ✅ Extracted from control packet
- ✅ Passed to semantic compressor
- ❌ **NOT persisted to SQLite** - Only used for in-session context compression

---

## Critical Missing Pieces

### Missing Piece #1: Post-Execution Verification Loop

**What Should Happen**:
```
User: "Fix the authentication bug in auth.go"
  ↓
[Perception] Intent: /fix, target: auth.go
  ↓
[Delegation] CoderShard spawns, edits file
  ↓
[Execution] File modified
  ↓
❌ MISSING: [Verification] Feed result back to LLM:
   "I asked you to fix auth.go. Here's what was changed:
    [diff output]
    Did this actually solve the bug? Was the fix correct?"
  ↓
❌ MISSING: [Learning] Based on LLM's verification:
   - If successful → Store pattern as "successful_fix(auth, bearer_token)"
   - If failed → Store as "failed_attempt(auth, approach_X)" and retry
```

**Current Flow** (BROKEN):
```
User request → Shard execution → Result returned to user → END
                                                          ↑
                                                  No verification!
                                                  No learning!
```

---

### Missing Piece #2: Main Agent Knowledge Store

**What Should Exist**:
```
.nerd/
├── knowledge.db          ❌ MISSING - Main agent learnings
├── session.json          ✅ EXISTS
├── config.json           ✅ EXISTS
└── shards/
    ├── coder_learnings.db      ✅ EXISTS
    ├── reviewer_learnings.db   ✅ EXISTS
    ├── tester_learnings.db     ✅ EXISTS
    └── researcher_learnings.db ✅ EXISTS
```

**Schema for knowledge.db**:
```sql
CREATE TABLE learnings (
    id INTEGER PRIMARY KEY,
    category TEXT,           -- 'preference', 'pattern', 'anti_pattern', 'heuristic'
    predicate TEXT,          -- e.g., 'user_prefers', 'successful_approach', 'avoid_pattern'
    args TEXT,               -- JSON array
    confidence REAL,         -- 0.0-1.0
    source TEXT,             -- 'user_feedback', 'self_verification', 'campaign'
    learned_at TIMESTAMP,
    last_used TIMESTAMP,
    use_count INTEGER
);

CREATE TABLE reasoning_history (
    id INTEGER PRIMARY KEY,
    intent_verb TEXT,
    target TEXT,
    reasoning_trace TEXT,   -- Full chain of thought
    outcome TEXT,           -- 'success', 'failure', 'partial'
    lessons TEXT,           -- What was learned
    timestamp TIMESTAMP
);
```

---

### Missing Piece #3: Self-Reflection After Tasks

**What Should Happen**:
After every significant action (fix, refactor, create), the system should:

1. **Capture State Before/After**:
```go
type TaskVerification struct {
    TaskID string
    Intent perception.Intent

    // Before state
    FilesBefore map[string]string  // file -> content hash
    TestsBefore string              // test output

    // After state
    FilesAfter  map[string]string
    TestsAfter  string

    // Changes
    Diff        string
}
```

2. **Ask LLM to Verify**:
```go
systemPrompt := `You are verifying your own work. Be honest about success/failure.`

userPrompt := fmt.Sprintf(`
You were asked to: %s
Target: %s

Changes made:
%s

Test output before: %s
Test output after: %s

Questions:
1. Did you successfully complete the requested task?
2. Are there any issues or edge cases you missed?
3. What did you learn from this task?

Return JSON:
{
  "success": true/false,
  "issues_found": ["..."],
  "lessons_learned": ["pattern: ...", "avoid: ..."],
  "confidence": 0.0-1.0
}
`, intent.Verb, intent.Target, diff, testsBefore, testsAfter)
```

3. **Store Learnings**:
```go
if verification.Success {
    // Store successful pattern
    learningStore.Save("main", "successful_approach",
        []any{intent.Verb, intent.Target, approach},
        "self_verification")
} else {
    // Store anti-pattern
    learningStore.Save("main", "avoid_pattern",
        []any{intent.Verb, intent.Target, approach},
        "self_verification")
}
```

---

### Missing Piece #4: Learning Integration in OODA Loop

**Current OODA Loop** ([cmd/nerd/chat/process.go:24-314](cmd/nerd/chat/process.go#L24-L314)):
```
1. PERCEPTION (Transducer)
2. CONTEXT LOADING (Scanner)
3. STATE UPDATE (Kernel)
4. DECISION & ACTION (Kernel → Executor)
5. CONTEXT SELECTION (Spreading Activation)
6. ARTICULATION (Response Generation)
7. SEMANTIC COMPRESSION (Process turn)
```

**What's Missing**:
```
8. ❌ VERIFICATION (Did it work?)
9. ❌ LEARNING (Store patterns)
10. ❌ KNOWLEDGE INJECTION (Load past learnings into context)
```

**Enhanced OODA Loop Should Be**:
```
1. PERCEPTION (Transducer)
2. KNOWLEDGE RETRIEVAL (Load relevant learnings from .nerd/knowledge.db)
3. CONTEXT LOADING (Scanner)
4. STATE UPDATE (Kernel + inject learned facts)
5. DECISION & ACTION (Kernel → Executor)
6. CONTEXT SELECTION (Spreading Activation)
7. ARTICULATION (Response Generation)
8. VERIFICATION (Feed result back to LLM for self-assessment)
9. LEARNING (Store successful patterns / anti-patterns)
10. SEMANTIC COMPRESSION (Process turn)
```

---

## Proposed Implementation

### Step 1: Create Main Knowledge Store

**File**: `internal/store/main_knowledge.go`

```go
package store

type MainKnowledgeStore struct {
    db *sql.DB
    path string
}

func NewMainKnowledgeStore(workspace string) (*MainKnowledgeStore, error) {
    dbPath := filepath.Join(workspace, ".nerd", "knowledge.db")
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, err
    }

    // Initialize schema
    schema := `
    CREATE TABLE IF NOT EXISTS learnings (
        id INTEGER PRIMARY KEY,
        category TEXT NOT NULL,
        predicate TEXT NOT NULL,
        args TEXT NOT NULL,
        confidence REAL DEFAULT 1.0,
        source TEXT,
        learned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        last_used TIMESTAMP,
        use_count INTEGER DEFAULT 0,
        UNIQUE(predicate, args)
    );

    CREATE TABLE IF NOT EXISTS reasoning_history (
        id INTEGER PRIMARY KEY,
        intent_verb TEXT,
        target TEXT,
        reasoning_trace TEXT,
        outcome TEXT,
        lessons TEXT,
        timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

    CREATE INDEX IF NOT EXISTS idx_learnings_predicate ON learnings(predicate);
    CREATE INDEX IF NOT EXISTS idx_learnings_category ON learnings(category);
    CREATE INDEX IF NOT EXISTS idx_reasoning_verb ON reasoning_history(intent_verb);
    `

    _, err = db.Exec(schema)
    return &MainKnowledgeStore{db: db, path: dbPath}, err
}

func (mks *MainKnowledgeStore) StoreReasoning(intent perception.Intent, trace string, outcome string, lessons []string) error {
    _, err := mks.db.Exec(`
        INSERT INTO reasoning_history (intent_verb, target, reasoning_trace, outcome, lessons)
        VALUES (?, ?, ?, ?, ?)
    `, intent.Verb, intent.Target, trace, outcome, strings.Join(lessons, "\n"))
    return err
}

func (mks *MainKnowledgeStore) StoreLearning(category, predicate string, args []any, source string) error {
    argsJSON, _ := json.Marshal(args)
    _, err := mks.db.Exec(`
        INSERT INTO learnings (category, predicate, args, source)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(predicate, args) DO UPDATE SET
            confidence = MIN(1.0, confidence + 0.1),
            last_used = CURRENT_TIMESTAMP,
            use_count = use_count + 1
    `, category, predicate, string(argsJSON), source)
    return err
}

func (mks *MainKnowledgeStore) LoadRelevantLearnings(verb string, target string, limit int) ([]Learning, error) {
    // Retrieve learnings that match the current task
    rows, err := mks.db.Query(`
        SELECT category, predicate, args, confidence
        FROM learnings
        WHERE predicate LIKE ? OR predicate LIKE ?
        ORDER BY confidence DESC, use_count DESC
        LIMIT ?
    `, fmt.Sprintf("%%%s%%", verb), fmt.Sprintf("%%%s%%", target), limit)

    if err != nil {
        return nil, err
    }
    defer rows.Close()

    learnings := []Learning{}
    for rows.Next() {
        var l Learning
        var argsJSON string
        rows.Scan(&l.Category, &l.Predicate, &argsJSON, &l.Confidence)
        json.Unmarshal([]byte(argsJSON), &l.Args)
        learnings = append(learnings, l)
    }
    return learnings, nil
}
```

---

### Step 2: Add Verification Loop to Process

**File**: `cmd/nerd/chat/verification.go` (NEW)

```go
package chat

type TaskVerifier struct {
    client perception.LLMClient
}

func (tv *TaskVerifier) VerifyTaskExecution(ctx context.Context, intent perception.Intent, result string) (*VerificationResult, error) {
    systemPrompt := `You are verifying your own work. Be brutally honest.

Report:
- success: Did you complete the requested task correctly?
- issues_found: Any problems, edge cases missed, or incomplete work
- lessons_learned: Patterns to remember (successful approaches or anti-patterns to avoid)
- confidence: 0.0-1.0 how confident you are this was successful
`

    userPrompt := fmt.Sprintf(`
Task: %s %s
Constraint: %s

Result:
%s

Return JSON:
{
  "success": true/false,
  "issues_found": ["issue 1", "issue 2"],
  "lessons_learned": ["lesson 1", "lesson 2"],
  "confidence": 0.85,
  "suggested_improvements": ["improvement 1"]
}
`, intent.Verb, intent.Target, intent.Constraint, result)

    resp, err := tv.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
    if err != nil {
        return nil, err
    }

    // Parse JSON response
    var verification VerificationResult
    jsonStr := extractJSON(resp)
    json.Unmarshal([]byte(jsonStr), &verification)

    return &verification, nil
}

type VerificationResult struct {
    Success               bool     `json:"success"`
    IssuesFound           []string `json:"issues_found"`
    LessonsLearned        []string `json:"lessons_learned"`
    Confidence            float64  `json:"confidence"`
    SuggestedImprovements []string `json:"suggested_improvements"`
}
```

---

### Step 3: Wire Verification into OODA Loop

**File**: `cmd/nerd/chat/process.go`

Add after line 312 (before `return responseMsg(response)`):

```go
// 8. VERIFICATION (Did the task succeed?)
if shardType != "" && (intent.Verb == "/fix" || intent.Verb == "/refactor" || intent.Verb == "/create") {
    verifier := &TaskVerifier{client: m.client}
    verification, err := verifier.VerifyTaskExecution(ctx, intent, response)
    if err == nil {
        // 9. LEARNING (Store patterns)
        if verification.Success && verification.Confidence >= 0.7 {
            // Store successful pattern
            for _, lesson := range verification.LessonsLearned {
                if strings.HasPrefix(lesson, "pattern:") {
                    m.knowledge.StoreLearning("pattern", intent.Verb,
                        []any{intent.Target, lesson}, "self_verification")
                }
            }
        } else if !verification.Success {
            // Store anti-pattern
            m.knowledge.StoreLearning("anti_pattern", intent.Verb,
                []any{intent.Target, verification.IssuesFound}, "self_verification")
        }

        // Store reasoning trace
        m.knowledge.StoreReasoning(intent, intent.Response,
            verification.Success ? "success" : "failure",
            verification.LessonsLearned)
    }
}
```

---

### Step 4: Inject Learnings into Context

**File**: `cmd/nerd/chat/process.go`

Add after line 89 (after Autopoiesis check, before Context Loading):

```go
// 2.5 KNOWLEDGE INJECTION (Load relevant learnings)
if m.knowledge != nil {
    learnings, err := m.knowledge.LoadRelevantLearnings(intent.Verb, intent.Target, 5)
    if err == nil && len(learnings) > 0 {
        // Convert learnings to Mangle facts
        learnedFacts := []core.Fact{}
        for _, learning := range learnings {
            learnedFacts = append(learnedFacts, core.Fact{
                Predicate: learning.Predicate,
                Args:      learning.Args,
            })
        }

        // Inject into kernel
        _ = m.kernel.LoadFacts(learnedFacts)

        warnings = append(warnings, fmt.Sprintf("[Learning] Loaded %d relevant patterns", len(learnings)))
    }
}
```

---

## Summary Table: What Needs to Be Built

| Component | Status | Priority | Estimated Effort |
|-----------|--------|----------|------------------|
| **Main Knowledge Store** (`.nerd/knowledge.db`) | ❌ Missing | 🔴 Critical | 4-6 hours |
| **Task Verification Loop** (self-assessment after actions) | ❌ Missing | 🔴 Critical | 3-4 hours |
| **Knowledge Injection** (load learnings into context) | ❌ Missing | 🟡 High | 2-3 hours |
| **Reasoning Trace Capture** (for main OODA, not just tools) | ❌ Missing | 🟡 High | 3-4 hours |
| **Memory Operation Persistence** (control_packet → SQLite) | ❌ Missing | 🟡 High | 2-3 hours |
| **Learning from User Feedback** (track accept/reject/modify) | ❌ Missing | 🟢 Medium | 3-4 hours |

**Total Estimated Effort**: 17-24 hours to complete the learning loop

---

## Immediate Next Steps

1. **Create `internal/store/main_knowledge.go`** - Main agent knowledge DB
2. **Add `Model.knowledge *store.MainKnowledgeStore`** to chat Model
3. **Implement `TaskVerifier`** in `cmd/nerd/chat/verification.go`
4. **Wire verification into OODA loop** after shard execution
5. **Add knowledge injection** before context loading
6. **Test the loop**: Fix a bug → Verify → Learn → Fix similar bug → Use learned pattern

---

## Long-Term Vision

Once complete, the learning loop will look like:

```
USER: "Fix the auth bug in user.go"
  ↓
[KNOWLEDGE RETRIEVAL]
  "Previously learned: 'When fixing auth, check Bearer token format'"
  ↓
[EXECUTION] CoderShard fixes bug
  ↓
[VERIFICATION] "Did I fix it correctly? Let me check..."
  LLM: "Yes, I added Bearer token validation. Success!"
  ↓
[LEARNING] Store: successful_fix(auth, bearer_token_validation)
  ↓
---NEXT TIME---
  ↓
USER: "Fix auth in admin.go"
  ↓
[KNOWLEDGE RETRIEVAL]
  "I remember: successful_fix(auth, bearer_token_validation)"
  ↓
[EXECUTION] Applies the same pattern automatically
  ↓
USER: "Wow, you remembered!"
```

This creates **true autopoiesis** - the system improves itself through experience without retraining the LLM.

---

**Status**: Architecture is 40% complete. Need to wire the verification and learning components into the main OODA loop.

**Recommendation**: Build these components incrementally:
1. Start with main knowledge store (foundation)
2. Add verification loop (captures success/failure)
3. Wire learning storage (persistence)
4. Add knowledge injection (utilization)
5. Expand to capture all reasoning traces (full observability)

