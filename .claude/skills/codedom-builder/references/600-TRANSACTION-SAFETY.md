# 600: Transaction Safety & Shadow Mode Integration

## Overview

Multi-file refactoring introduces the **Split Brain Problem**: if some edits succeed and others fail, the codebase enters an inconsistent state. This document covers the Two-Phase Commit (2PC) protocol, shadow mode integration, and test impact analysis.

## The Split Brain Problem

### Failure Scenario

```text
User Request: "Rename user_id to sub_id across backend and frontend"

Step 1: Edit api.go     ✓ Success
Step 2: Edit client.py  ✗ Mangle blocks (decorator stripped accidentally)

Result: Split Brain State
  - Go backend expects field "sub_id"
  - Python client still sends "user_id"
  - System is broken, agent enters panic loop
```

### Why This Matters

Traditional text editors don't have this problem because humans edit one file at a time and can mentally track state. The Code DOM's power (multi-file, cross-language edits) creates new failure modes that require transaction semantics.

## Two-Phase Commit Protocol

### Architecture

```text
┌─────────────────────────────────────────────────────────────────────┐
│                        TRANSACTION MANAGER                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐         │
│   │   PHASE 1    │    │   PHASE 2    │    │   ROLLBACK   │         │
│   │   PREPARE    │ => │   COMMIT     │ or │   ABORT      │         │
│   └──────────────┘    └──────────────┘    └──────────────┘         │
│         │                    │                    │                  │
│         ▼                    ▼                    ▼                  │
│   ┌────────────────────────────────────────────────────────────┐   │
│   │                     SHADOW MODE                             │   │
│   │    (In-memory or temp dir - zero side effects on real fs)   │   │
│   └────────────────────────────────────────────────────────────┘   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Phase 1: Prepare

```go
// internal/core/transaction_manager.go
package core

type TransactionManager struct {
    shadow     *ShadowMode
    kernel     *Kernel
    parser     *CodeElementParser
    pending    []PendingEdit
}

type PendingEdit struct {
    File        string
    OldContent  []byte
    NewContent  []byte
    Elements    []CodeElement
    Facts       []MangleFact
}

// Begin starts a new transaction
func (tm *TransactionManager) Begin(ctx context.Context) (*Transaction, error) {
    // Create isolated shadow environment
    shadow, err := NewShadowMode(tm.projectRoot)
    if err != nil {
        return nil, err
    }

    return &Transaction{
        id:       uuid.New().String(),
        manager:  tm,
        shadow:   shadow,
        pending:  make([]PendingEdit, 0),
        prepared: false,
    }, nil
}

// Prepare validates all edits in shadow mode
func (tx *Transaction) Prepare(ctx context.Context, edits []EditRequest) error {
    for _, edit := range edits {
        // 1. Read original content
        original, err := os.ReadFile(edit.File)
        if err != nil {
            return fmt.Errorf("prepare failed: cannot read %s: %w", edit.File, err)
        }

        // 2. Apply edit to shadow filesystem
        if err := tx.shadow.WriteFile(edit.File, edit.NewContent); err != nil {
            return fmt.Errorf("prepare failed: shadow write %s: %w", edit.File, err)
        }

        // 3. Re-parse shadow file
        elements, err := tx.manager.parser.ParseFile(tx.shadow.Path(edit.File))
        if err != nil {
            return &PrepareError{
                Phase: "parse",
                File:  edit.File,
                Err:   err,
            }
        }

        // 4. Generate candidate facts
        facts := tx.manager.parser.EmitFacts(elements)

        // 5. Check Mangle safety rules against shadow state
        violations, err := tx.checkSafetyRules(ctx, edit.File, facts)
        if err != nil {
            return err
        }
        if len(violations) > 0 {
            return &SafetyViolationError{
                File:       edit.File,
                Violations: violations,
            }
        }

        // 6. Store pending edit
        tx.pending = append(tx.pending, PendingEdit{
            File:       edit.File,
            OldContent: original,
            NewContent: edit.NewContent,
            Elements:   elements,
            Facts:      facts,
        })
    }

    // 7. Attempt shadow compilation/lint
    if err := tx.validateShadowBuild(ctx); err != nil {
        return &PrepareError{
            Phase: "build",
            Err:   err,
        }
    }

    tx.prepared = true
    return nil
}

// checkSafetyRules runs deny_edit rules against candidate facts
func (tx *Transaction) checkSafetyRules(ctx context.Context, file string, candidateFacts []MangleFact) ([]SafetyViolation, error) {
    // Snapshot current facts
    snapshotFacts := tx.manager.kernel.GetFactsForFile(file)
    for _, f := range snapshotFacts {
        tx.shadow.kernel.Assert(prefixPredicate(f, "snapshot"))
    }

    // Assert candidate facts
    for _, f := range candidateFacts {
        tx.shadow.kernel.Assert(prefixPredicate(f, "candidate"))
    }

    // Query for violations
    results, err := tx.shadow.kernel.Query("deny_edit(Ref, Reason)")
    if err != nil {
        return nil, err
    }

    var violations []SafetyViolation
    for _, r := range results {
        violations = append(violations, SafetyViolation{
            Ref:    r["Ref"].(string),
            Reason: r["Reason"].(string),
        })
    }

    return violations, nil
}
```

### Phase 2: Commit or Abort

```go
// Commit flushes shadow changes to real filesystem
func (tx *Transaction) Commit(ctx context.Context) error {
    if !tx.prepared {
        return errors.New("cannot commit: transaction not prepared")
    }

    // Use file locking to prevent concurrent modifications
    locks := make([]*fslock.Lock, len(tx.pending))
    for i, edit := range tx.pending {
        lock := fslock.New(edit.File + ".lock")
        if err := lock.LockWithTimeout(5 * time.Second); err != nil {
            // Release already-acquired locks
            for j := 0; j < i; j++ {
                locks[j].Unlock()
            }
            return fmt.Errorf("commit failed: cannot lock %s", edit.File)
        }
        locks[i] = lock
    }
    defer func() {
        for _, lock := range locks {
            if lock != nil {
                lock.Unlock()
            }
        }
    }()

    // Atomic write loop
    var committed []string
    for _, edit := range tx.pending {
        // Write to temp file first
        tmpFile := edit.File + ".tmp"
        if err := os.WriteFile(tmpFile, edit.NewContent, 0644); err != nil {
            tx.rollback(committed)
            return fmt.Errorf("commit failed: write %s: %w", edit.File, err)
        }

        // Atomic rename
        if err := os.Rename(tmpFile, edit.File); err != nil {
            os.Remove(tmpFile)
            tx.rollback(committed)
            return fmt.Errorf("commit failed: rename %s: %w", edit.File, err)
        }

        committed = append(committed, edit.File)

        // Update kernel facts
        tx.manager.kernel.InvalidateFacts(edit.File)
        for _, fact := range edit.Facts {
            tx.manager.kernel.Assert(fact)
        }
    }

    // Cleanup shadow
    tx.shadow.Close()
    tx.pending = nil

    return nil
}

// Abort discards all pending changes
func (tx *Transaction) Abort() error {
    tx.shadow.Close()
    tx.pending = nil
    tx.prepared = false
    return nil
}

// rollback reverts already-committed files
func (tx *Transaction) rollback(committed []string) {
    for _, file := range committed {
        for _, edit := range tx.pending {
            if edit.File == file {
                os.WriteFile(file, edit.OldContent, 0644)
                break
            }
        }
    }
}
```

### Shadow Mode Integration

```go
// internal/core/shadow_mode.go (extension)

type ShadowMode struct {
    baseDir     string        // Original project root
    shadowDir   string        // Temp directory for shadow files
    kernel      *Kernel       // Isolated Mangle kernel for shadow state
    files       map[string]bool  // Tracked shadow files
}

func NewShadowMode(baseDir string) (*ShadowMode, error) {
    shadowDir, err := os.MkdirTemp("", "codedom-shadow-*")
    if err != nil {
        return nil, err
    }

    // Create isolated kernel with same schema but no facts
    kernel, err := NewKernel(WithSchemaOnly())
    if err != nil {
        os.RemoveAll(shadowDir)
        return nil, err
    }

    return &ShadowMode{
        baseDir:   baseDir,
        shadowDir: shadowDir,
        kernel:    kernel,
        files:     make(map[string]bool),
    }, nil
}

func (s *ShadowMode) WriteFile(path string, content []byte) error {
    // Map real path to shadow path
    shadowPath := s.Path(path)

    // Ensure parent directory exists
    if err := os.MkdirAll(filepath.Dir(shadowPath), 0755); err != nil {
        return err
    }

    s.files[path] = true
    return os.WriteFile(shadowPath, content, 0644)
}

func (s *ShadowMode) Path(realPath string) string {
    rel, _ := filepath.Rel(s.baseDir, realPath)
    return filepath.Join(s.shadowDir, rel)
}

func (s *ShadowMode) Close() {
    os.RemoveAll(s.shadowDir)
    s.kernel.Close()
}
```

## Test Impact Analysis

### The Problem

Running `go test ./...` after every edit is expensive. With a dependency graph, we can surgically select only impacted tests.

### Mangle Rules for Test Selection

```mangle
# =============================================================================
# TEST IMPACT ANALYSIS
# Identify which tests need to run when code changes
# =============================================================================

# 1. Identify test files
is_test_file(File) :- file_path(File), fn:contains(File, "_test.go").
is_test_file(File) :- file_path(File), fn:contains(File, "_test.py").
is_test_file(File) :- file_path(File), fn:contains(File, ".test.ts").
is_test_file(File) :- file_path(File), fn:contains(File, ".spec.ts").

# 2. Identify test functions
is_test_function(Ref) :-
    code_element(Ref, /function, File, _, _),
    is_test_file(File),
    element_name(Ref, Name),
    fn:starts_with(Name, "Test").

is_test_function(Ref) :-
    code_element(Ref, /function, File, _, _),
    is_test_file(File),
    py_decorator(Ref, "pytest.mark").

# 3. Direct test dependencies
test_depends_on(TestRef, TargetRef) :-
    is_test_function(TestRef),
    calls(TestRef, TargetRef).

test_depends_on(TestRef, TargetRef) :-
    is_test_function(TestRef),
    code_element(TestRef, _, TestFile, _, _),
    file_imports(TestFile, TargetFile),
    code_element(TargetRef, _, TargetFile, _, _).

# 4. Transitive test dependencies
test_depends_on_transitive(TestRef, TargetRef) :-
    test_depends_on(TestRef, TargetRef).

test_depends_on_transitive(TestRef, TargetRef) :-
    test_depends_on(TestRef, MidRef),
    depends_on_transitive(MidRef, TargetRef).

# 5. Impact query: What tests need to run if target changes?
impacted_test(TestRef) :-
    plan_edit(TargetRef),
    test_depends_on_transitive(TestRef, TargetRef).

impacted_test_file(TestFile) :-
    impacted_test(TestRef),
    code_element(TestRef, _, TestFile, _, _).

# 6. Aggregate impacted tests
impacted_tests_for_edit(TargetRef, Tests) :-
    plan_edit(TargetRef),
    impacted_test(TestRef) |>
    do fn:group_by(TargetRef),
    let Tests = fn:collect(TestRef).

# 7. No-op detection: Edit has no impacted tests (needs full test)
needs_full_test_run(TargetRef) :-
    plan_edit(TargetRef),
    not impacted_test(_).
```

### Go Integration

```go
// GetImpactedTests returns test files that need to run for pending edits
func (k *Kernel) GetImpactedTests(editRefs []string) ([]string, error) {
    // Assert pending edits
    for _, ref := range editRefs {
        k.Assert(MangleFact{
            Predicate: "plan_edit",
            Args:      []interface{}{ref},
        })
    }
    defer func() {
        // Clean up plan_edit facts
        for _, ref := range editRefs {
            k.Retract(MangleFact{
                Predicate: "plan_edit",
                Args:      []interface{}{ref},
            })
        }
    }()

    // Query impacted test files
    results, err := k.Query("impacted_test_file(File)")
    if err != nil {
        return nil, err
    }

    var testFiles []string
    seen := make(map[string]bool)
    for _, r := range results {
        file := r["File"].(string)
        if !seen[file] {
            seen[file] = true
            testFiles = append(testFiles, file)
        }
    }

    return testFiles, nil
}

// TesterShard integration
func (t *TesterShard) RunImpactedTests(ctx context.Context, editRefs []string) (*TestResult, error) {
    // Get impacted tests from kernel
    testFiles, err := t.kernel.GetImpactedTests(editRefs)
    if err != nil {
        return nil, err
    }

    if len(testFiles) == 0 {
        // No specific tests impacted, run full suite
        return t.RunFullTestSuite(ctx)
    }

    // Run only impacted tests
    return t.RunTestFiles(ctx, testFiles)
}
```

## Incremental Graph Maintenance

### The State Drift Problem

```text
Turn 1: Agent adds function NewFeature()
Turn 2: Agent tries to call NewFeature()
        → Mangle blocks: "Unknown reference" (graph is stale)
```

### Single-File Refresh

```go
// Refresh atomically updates facts for a single file
func (p *CodeElementParser) Refresh(ctx context.Context, filePath string) error {
    // 1. Read current content
    content, err := os.ReadFile(filePath)
    if err != nil {
        return err
    }

    // 2. Parse file
    elements, err := p.ParseFile(filePath, content)
    if err != nil {
        return err
    }

    // 3. Generate new facts
    newFacts := p.EmitFacts(elements)

    // 4. Atomic fact update
    p.kernel.Transaction(func(tx *KernelTx) error {
        // Remove old facts for this file
        tx.RetractByFile(filePath)

        // Assert new facts
        for _, fact := range newFacts {
            tx.Assert(fact)
        }

        return nil
    })

    // 5. Update element cache
    p.elementCache.Set(filePath, elements)

    return nil
}

// RefreshMultiple batch-updates multiple files
func (p *CodeElementParser) RefreshMultiple(ctx context.Context, files []string) error {
    type fileResult struct {
        file     string
        elements []CodeElement
        facts    []MangleFact
        err      error
    }

    results := make(chan fileResult, len(files))

    // Parse in parallel
    var wg sync.WaitGroup
    for _, file := range files {
        wg.Add(1)
        go func(f string) {
            defer wg.Done()
            content, err := os.ReadFile(f)
            if err != nil {
                results <- fileResult{file: f, err: err}
                return
            }
            elements, err := p.ParseFile(f, content)
            if err != nil {
                results <- fileResult{file: f, err: err}
                return
            }
            facts := p.EmitFacts(elements)
            results <- fileResult{file: f, elements: elements, facts: facts}
        }(file)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect results
    var allFacts []MangleFact
    var filesToUpdate []string
    for r := range results {
        if r.err != nil {
            continue // Log and skip
        }
        filesToUpdate = append(filesToUpdate, r.file)
        allFacts = append(allFacts, r.facts...)
        p.elementCache.Set(r.file, r.elements)
    }

    // Single transaction for all updates
    return p.kernel.Transaction(func(tx *KernelTx) error {
        for _, file := range filesToUpdate {
            tx.RetractByFile(file)
        }
        for _, fact := range allFacts {
            tx.Assert(fact)
        }
        return nil
    })
}
```

### Kernel Transaction Support

```go
// Transaction executes multiple fact updates atomically
func (k *Kernel) Transaction(fn func(*KernelTx) error) error {
    k.mu.Lock()
    defer k.mu.Unlock()

    tx := &KernelTx{
        kernel:    k,
        toAssert:  make([]MangleFact, 0),
        toRetract: make([]MangleFact, 0),
    }

    if err := fn(tx); err != nil {
        return err  // Rollback: don't apply changes
    }

    // Apply changes
    for _, fact := range tx.toRetract {
        k.retractInternal(fact)
    }
    for _, fact := range tx.toAssert {
        k.assertInternal(fact)
    }

    return nil
}

type KernelTx struct {
    kernel    *Kernel
    toAssert  []MangleFact
    toRetract []MangleFact
}

func (tx *KernelTx) Assert(fact MangleFact) {
    tx.toAssert = append(tx.toAssert, fact)
}

func (tx *KernelTx) Retract(fact MangleFact) {
    tx.toRetract = append(tx.toRetract, fact)
}

func (tx *KernelTx) RetractByFile(file string) {
    // Mark all facts for this file for removal
    for _, fact := range tx.kernel.facts {
        if factBelongsToFile(fact, file) {
            tx.toRetract = append(tx.toRetract, fact)
        }
    }
}
```

## JIT Prompt Updates (Prompt Blindness Fix)

### The Problem

The LLM has powerful semantic capabilities but still thinks in terms of text/line edits because its prompt hasn't been updated.

### Updated Prompt Atoms

```yaml
# internal/prompt/atoms/capabilities/codedom.yaml
- id: "capabilities/codedom/semantic_editing"
  category: "capabilities"
  priority: 100
  is_mandatory: true
  intent_verbs: ["/fix", "/refactor", "/rename", "/implement"]
  content: |
    ## Semantic Code Editing

    You are operating on a **Semantic Code Graph**, not raw text files.

    ### Key Principle: Think in Symbols, Not Lines

    Instead of:
    - ❌ "Find the text `func foo` and replace with `func bar`"
    - ❌ "Delete lines 42-58 and insert new code"

    Use:
    - ✓ `rename_symbol(ref="go:user.go:User.Login", new_name="Authenticate")`
    - ✓ `edit_element(ref="py:auth.py:User.validate", new_body="...")`

    ### Cross-Language Refactoring

    When renaming or modifying code that has API dependencies (detected via wire names),
    the system automatically identifies and updates all coupled elements:

    Example: Renaming `user_id` to `sub_id`
    - System detects: Go struct tag `json:"user_id"`
    - System detects: Python Pydantic alias `alias="user_id"`
    - System detects: TypeScript interface property `userId`
    - All are updated atomically in a single transaction

    ### Available Operations

    | Operation | Use When |
    |-----------|----------|
    | `get_elements(file)` | Discover symbols in a file |
    | `get_element(ref)` | Inspect a specific symbol |
    | `edit_element(ref, body)` | Modify a symbol's implementation |
    | `rename_symbol(ref, name)` | Rename with cross-file propagation |
    | `delete_element(ref)` | Remove a symbol |
    | `insert_element(after_ref, body)` | Add new symbol |

    ### Safety Guarantees

    All edits are validated before application:
    - Security decorator preservation checked
    - Type safety verified
    - Cross-language API consistency enforced
    - Rollback on any failure (atomic transactions)

- id: "capabilities/codedom/cross_lang_example"
  category: "capabilities"
  priority: 90
  is_mandatory: false
  intent_verbs: ["/refactor", "/rename"]
  content: |
    ## Cross-Language Refactor Example

    User: "Rename the user_id field to subject_id across the entire codebase"

    Your approach:
    1. Query the graph for all elements with wire_name "user_id":
       ```
       Query: wire_name(Ref, "user_id")
       Results:
         - go:backend/models/user.go:User.UserID (json:"user_id")
         - py:backend/schemas/user.py:UserSchema.user_id (alias="user_id")
         - ts:frontend/types/user.ts:IUser.userId
         - kt:mobile/models/User.kt:User.userId (@SerializedName)
       ```

    2. Issue a single rename intent:
       ```
       Intent: rename_wire_name(old="user_id", new="subject_id")
       ```

    3. The system:
       - Creates transaction
       - Generates edits for all 4 files
       - Validates in shadow mode
       - Commits atomically or rolls back entirely

    You do NOT manually edit each file. The system handles propagation.
```

### ConfigFactory Update

```go
// internal/prompt/config_factory.go (extension)

// Add CodeDOM tools for refactoring intents
provider.atoms["/refactor"] = ConfigAtom{
    Tools: []string{
        "get_elements",
        "get_element",
        "edit_element",
        "rename_symbol",
        "delete_element",
        "insert_element",
        // Cross-language tools
        "query_wire_names",
        "rename_wire_name",
        "get_api_dependencies",
    },
    Priority: 100,
}
```

## Complete Workflow: Multi-File Rename

```text
1. User: "Rename user_id to sub_id"

2. Perception Transducer:
   user_intent(/refactor, /rename, "user_id", "sub_id")

3. Mangle Query:
   wire_name(Ref, "user_id") → [go:..., py:..., ts:..., kt:...]

4. Transaction Begin:
   tx = TransactionManager.Begin()

5. Generate Edits:
   for each ref:
     edit = GenerateRenameEdit(ref, "user_id", "sub_id")
     edits.append(edit)

6. Prepare (Shadow Mode):
   tx.Prepare(edits)
   - Apply to shadow filesystem
   - Re-parse all affected files
   - Run deny_edit rules
   - Attempt shadow build
   - If any failure → tx.Abort()

7. Get Impacted Tests:
   tests = kernel.GetImpactedTests(refs)

8. Commit:
   tx.Commit()
   - Atomic write to real filesystem
   - Update kernel facts

9. Run Tests:
   tester.RunTestFiles(tests)
   - Only run impacted tests, not full suite

10. Articulate:
    "Renamed user_id to sub_id across 4 files (Go, Python, TypeScript, Kotlin).
     Ran 3 impacted test files. All passed."
```

## Error Handling

### SafetyViolationError

```go
type SafetyViolationError struct {
    File       string
    Violations []SafetyViolation
}

func (e *SafetyViolationError) Error() string {
    var msgs []string
    for _, v := range e.Violations {
        msgs = append(msgs, fmt.Sprintf("%s: %s", v.Ref, v.Reason))
    }
    return fmt.Sprintf("safety violations in %s: %s", e.File, strings.Join(msgs, "; "))
}

func (e *SafetyViolationError) Remediation() string {
    var tips []string
    for _, v := range e.Violations {
        tips = append(tips, remediations[v.Reason])
    }
    return strings.Join(tips, "\n")
}
```

### PrepareError

```go
type PrepareError struct {
    Phase string  // "parse", "safety", "build"
    File  string
    Err   error
}

func (e *PrepareError) Error() string {
    return fmt.Sprintf("prepare failed at %s phase for %s: %v", e.Phase, e.File, e.Err)
}

func (e *PrepareError) IsRecoverable() bool {
    // Parse errors might be fixable; build errors need code changes
    return e.Phase == "parse"
}
```

## Mangle Facts for Transaction Tracking

```mangle
# =============================================================================
# TRANSACTION TRACKING
# =============================================================================

Decl transaction_started(TxID.Type<string>, Timestamp.Type<int>).
Decl transaction_prepared(TxID.Type<string>, FileCount.Type<int>).
Decl transaction_committed(TxID.Type<string>, Timestamp.Type<int>).
Decl transaction_aborted(TxID.Type<string>, Reason.Type<string>).

Decl transaction_edit(TxID.Type<string>, File.Type<string>, Ref.Type<string>).

# Detect stuck transactions (prepared but not committed/aborted)
stuck_transaction(TxID) :-
    transaction_prepared(TxID, _),
    not transaction_committed(TxID, _),
    not transaction_aborted(TxID, _),
    transaction_started(TxID, StartTime),
    CurrentTime = fn:now(),
    fn:minus(CurrentTime, StartTime) > 300.  # 5 minute timeout
```
