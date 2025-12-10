# Audit Workflows

Detailed step-by-step workflows for common integration audit scenarios.

---

## Workflow 1: New Feature Audit

Use this checklist for any new capability:

```markdown
## Feature: [Name]

### Phase 1: Entry Point
- [ ] **CLI Command** (if user-facing)
  - [ ] Handler in `cmd/nerd/chat/commands.go`
  - [ ] Help text updated
  - [ ] Error handling implemented

- [ ] **Transducer Verb** (if NL-triggered)
  - [ ] Verb added to `VerbCorpus` in `perception/transducer.go`
  - [ ] Synonyms defined
  - [ ] Pattern regex compiled
  - [ ] ShardType mapping set

- [ ] **System Shard** (if auto-start)
  - [ ] Added to `StartSystemShards()` list
  - [ ] Continuous operation loop implemented
  - [ ] Graceful shutdown handler

### Phase 2: Core Logic
- [ ] **Implementation File** created
- [ ] **Dependencies Injected** (Kernel, LLM, VirtualStore)
- [ ] **Error Handling** with context
- [ ] **Logging** using appropriate category

### Phase 3: Kernel Integration
- [ ] **Schema Declarations** (`schemas.mg`)
  - [ ] All predicates have `Decl` statements
  - [ ] Arity matches usage
  - [ ] Types specified if needed

- [ ] **Fact Generation** (Go→Mangle)
  - [ ] Structs implement `ToAtom()`
  - [ ] `LoadFacts()` called to add to kernel
  - [ ] Atoms use `/lowercase` for constants

- [ ] **Policy Rules** (if derived predicates)
  - [ ] Rules added to `policy.gl`
  - [ ] Safety checked (all vars bound)
  - [ ] Stratification correct (no negation cycles)

### Phase 4: Action Layer
- [ ] **VirtualStore Actions** (if external I/O)
  - [ ] Action constant defined (`ActionMyAction`)
  - [ ] Handler case in `Execute()` switch
  - [ ] Permission check implemented
  - [ ] Facts returned via `FactsToAdd`

- [ ] **Virtual Predicates** (if external data)
  - [ ] Decl in `schemas.mg`
  - [ ] Resolution in `VirtualStore.Get()`
  - [ ] Error handling

### Phase 5: Output
- [ ] **Logging** at appropriate levels
- [ ] **Kernel Facts** for cross-component communication
- [ ] **Piggyback Output** (if shard execution)

### Phase 6: Testing
- [ ] **Unit Tests** for isolated logic
- [ ] **Integration Test** for wiring
- [ ] **Live Test** (if external deps)

### Phase 7: Configuration
- [ ] **Config Struct** (if configurable)
- [ ] **Documentation** updated

### Verification
```bash
python audit_wiring.py --component myfeature --verbose
```
```

---

## Workflow 2: Debugging "Code Exists But Doesn't Run"

Systematic trace through wiring points:

### Step 1: Verify Entry Point

**Question:** How does execution start?

- [ ] **CLI Command Path**
  - Check: `cmd/nerd/chat/commands.go` has case for `/mycommand`
  - Test: Type `/mycommand` in TUI → should hit handler
  - Debug: Add logging at handler entry

- [ ] **Transducer Path**
  - Check: `VerbCorpus` has verb entry with correct `ShardType`
  - Test: NL input matches pattern regex
  - Debug: Enable `/perception` logging, verify `user_intent` fact

- [ ] **System Shard Path**
  - Check: Shard in `StartSystemShards()` list
  - Test: Boot cortex, check active shards
  - Debug: Add logging at shard `Execute()` entry

**If entry point missing:** Add appropriate handler, restart.

### Step 2: Verify Shard Registration

**Question:** Can the shard be found and spawned?

- [ ] Factory registered in `registration.go`
- [ ] Profile defined
- [ ] Dependencies injected

**If registration missing:** Add factory and profile, recompile.

### Step 3: Verify Kernel Wiring

**Question:** Are predicates declared and facts loading?

- [ ] Schema declarations in `schemas.mg`
- [ ] Fact generation calls `kernel.LoadFacts()`
- [ ] Policy rules derive expected predicates

**If kernel wiring missing:** Add Decl, LoadFacts, or rules.

### Step 4: Verify Action Layer

**Question:** Are external actions executing?

- [ ] Action type defined in `virtual_store.go`
- [ ] Handler case in `Execute()` switch
- [ ] Permission check passing

**If action layer missing:** Add action type and handler.

### Step 5: Verify Output Path

**Question:** Where do results go?

- [ ] Logging present
- [ ] Facts returned to kernel
- [ ] Piggyback output (for shards)

**If output missing:** Add logging, fact injection, or return values.

### Step 6: Verify Dependencies

**Question:** Are external systems available?

- [ ] SQLite connection (Type B/U)
- [ ] LLM client configured
- [ ] File access permissions

### Step 7: Check Tests

- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Live tests pass

### Quick Diagnostic Commands

```bash
# Check shard registered
./nerd.exe
> /spawn myshard "test"

# Check logging
# Enable debug in .nerd/config.json, check .nerd/logs/

# Run audit
python audit_wiring.py --component myshard --verbose
```

### Common Root Causes

| Symptom | Likely Cause | Quick Fix |
|---------|--------------|-----------|
| "unknown shard type" | Missing factory | Add to registration.go |
| "undeclared predicate" | Missing Decl | Add to schemas.mg |
| Nil pointer panic | Missing injection | Add Set* call in factory |
| Silent failure | No logging | Add logging statements |
| "permission denied" | Wrong permissions | Update profile permissions |

---

## Workflow 3: Pre-Commit Integration Check

Quick verification before committing:

### Fast Track (small changes)
1. Build passes
2. Related tests pass
3. Manual verification

### Full Track (new features/shards)

```markdown
## Pre-Commit Checklist

### 1. Build Passes
```bash
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go build ./cmd/nerd
```

### 2. Tests Pass
```bash
go test ./...
```

### 3. Wiring Audit Clean
```bash
python audit_wiring.py --verbose
```

### 4. Manual Smoke Test
```bash
./nerd.exe
# Test your feature manually
```

### 5. Git Status Clean
- Only intended files staged
- No secrets in config
```

---

## Workflow 4: System-Specific Audits

### Mangle System Audit

**Schema Audit:**
- Every predicate used in policy has a `Decl`
- Arities match between Decl and usage
- Types specified for predicates requiring them

**Policy Audit:**
- All derived predicates have rules
- Variables are safe (bound before negation)
- Stratification correct
- Aggregations use `|> do fn:group_by(...), let N = fn:Count()`

**Fact Generation Audit:**
- Go structs implement `ToAtom()` correctly
- Atoms use `/lowercase` for constants
- `LoadFacts()` called after fact creation

### Storage System Audit

**RAM Tier:** Kernel initialized, facts loading, cleared between sessions
**Vector Tier:** SQLite DB exists, embeddings populated, similarity search works
**Graph Tier:** knowledge_graph table exists, relations populated
**Cold Storage:** cold_storage table exists, patterns persisted

### Compression System Audit

- Budget configured in shard profiles
- Budget enforcement in context selection
- Spreading activation for relevance scoring

### Autopoiesis Audit

**Ouroboros Loop:**
- Tool generation enabled
- ToolGeneratorShard registered
- Generated tools persisted to `.nerd/tools/`
- Tools loaded on next boot

**Safety Checking:**
- Constitution Gate running
- Generated code reviewed before execution
- Rejection patterns logged

### TUI Integration Audit

- Commands registered in `handleCommand()` switch
- Help text updated
- Model state updated in `Update()`
- View rendering in `View()`

### Campaign System Audit

- Campaign created with phases
- Context paging configured
- Phase transitions working
- Dependencies respected
