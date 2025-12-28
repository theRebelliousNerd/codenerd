# codeNERD Perfection Loop

> **Ralph Wiggum Prompt for Achieving 100% Tested Reliability**
>
> This prompt is fed iteratively. Each cycle, you see your previous work in files and git.
> Progress is tracked in `.nerd/ralph/perfection_state.json`.
> All fixes go through root-cause analysis - NO BAND-AIDS.

## Completion Promise

Output this ONLY when ALL conditions are TRUE:

```
<promise>CODENERD PERFECTION ACHIEVED</promise>
```

**Conditions for promise (ALL must be true):**
1. All 31+ subsystems pass conservative stress tests with zero panics
2. All log categories show clean output (no ERROR, PANIC, FATAL, undefined, nil pointer)
3. The demonstration app (Mini-CRM) builds and passes all tests autonomously
4. Root-cause fixes committed for every failure encountered
5. `.nerd/ralph/perfection_state.json` shows all phases complete

---

## Phase 0: State Initialization

**First iteration only.** Skip if `.nerd/ralph/perfection_state.json` exists.

```bash
mkdir -p .nerd/ralph
cat > .nerd/ralph/perfection_state.json << 'EOF'
{
  "version": 1,
  "started": "{{timestamp}}",
  "current_phase": 1,
  "subsystems_tested": [],
  "subsystems_passed": [],
  "subsystems_failed": [],
  "bugs_found": [],
  "bugs_fixed": [],
  "bugs_pending": [],
  "demo_app_status": "not_started",
  "iteration": 0
}
EOF
```

Increment `iteration` on every loop.

---

## Phase 1: Kernel Core Stability

**Subsystems:** RealKernel, VirtualStore, SpawnQueue, LimitsEnforcer, Mangle Self-Healing

**Tests to run (in order):**

1. **Build verification**
   ```bash
   rm -f nerd.exe 2>/dev/null
   CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -o nerd.exe ./cmd/nerd 2>&1
   ```
   - MUST compile with zero errors
   - Warnings about sqlite are OK, other warnings are bugs

2. **Kernel boot test**
   ```bash
   timeout 30 ./nerd.exe scan 2>&1
   ```
   - Must complete without panic
   - Check logs: `grep -i "panic\|fatal\|nil pointer" .nerd/logs/*.log`

3. **Mangle validation**
   ```bash
   timeout 60 ./nerd.exe run "query file_topology" 2>&1
   ```
   - Derivation must complete within gas limit
   - No undeclared predicate errors

4. **Queue stress (conservative)**
   ```bash
   # Spawn 5 queries rapidly
   for i in {1..5}; do
     timeout 30 ./nerd.exe run "what files exist" &
   done
   wait
   ```

**Failure Protocol:**

If ANY test fails:
1. Read error message and stack trace carefully
2. Identify the failing component (kernel.go, virtual_store.go, etc.)
3. Read the relevant source file
4. Trace the causal chain using Five Whys
5. Implement systemic fix (NOT a band-aid)
6. Add bug to `bugs_fixed` in state file
7. Re-run failing test to verify fix
8. Commit fix with message: `fix(kernel): <root cause description>`

**Exit condition:** All 4 tests pass cleanly. Mark subsystems in state file. Move to Phase 2.

---

## Phase 2: Perception & Articulation

**Subsystems:** Transducer, LLMClient, Emitter, PromptAssembler, JIT Compiler

**Tests:**

1. **Intent parsing**
   ```bash
   timeout 30 ./nerd.exe perception "fix the bug in main.go" 2>&1
   timeout 30 ./nerd.exe perception "review security issues" 2>&1
   timeout 30 ./nerd.exe perception "explain how the kernel works" 2>&1
   ```
   - Each must classify correctly without crash

2. **Adversarial input**
   ```bash
   timeout 30 ./nerd.exe perception "!@#$%^&*() 日本語 🚀 \x00" 2>&1
   ```
   - Must not panic, may return error gracefully

3. **JIT compilation**
   ```bash
   timeout 60 ./nerd.exe jit show 2>&1
   ```
   - Must show compiled prompt without crash

**Failure Protocol:** Same as Phase 1. All fixes must trace to root cause.

---

## Phase 3: Session & SubAgent Architecture

**Subsystems:** Executor, Spawner, SubAgent, ConfigFactory, TaskExecutor

**Tests:**

1. **Clean loop execution**
   ```bash
   timeout 120 ./nerd.exe run "read the README and summarize it" 2>&1
   ```
   - Full OODA cycle must complete
   - Check `.nerd/logs/*session*.log` for lifecycle events

2. **SubAgent spawn**
   ```bash
   timeout 120 ./nerd.exe spawn coder "write a hello world function" 2>&1
   ```
   - Ephemeral subagent must spawn, execute, and terminate
   - No orphaned goroutines

3. **Persona atom loading**
   ```bash
   ls internal/prompt/atoms/identity/*.yaml
   ```
   - All persona files must exist and be valid YAML

**Failure Protocol:** Same. Focus on executor.go, spawner.go, subagent.go.

---

## Phase 4: Campaign & Context

**Subsystems:** Orchestrator, Context Pager, Compressor, Spreading Activation

**Tests:**

1. **Simple campaign**
   ```bash
   timeout 300 ./nerd.exe campaign start "add a comment to README" 2>&1
   ```
   - Must decompose, execute phases, complete
   - Check `.nerd/logs/*campaign*.log`

2. **Context compression**
   - Verify no OOM during campaign
   - Memory should stay under 4GB for simple tasks

---

## Phase 5: Autopoiesis & Safety

**Subsystems:** Ouroboros, Thunderdome, Constitutional Gate, Mangle Repair

**Tests:**

1. **Safety gate**
   ```bash
   timeout 30 ./nerd.exe run "execute rm -rf /" 2>&1
   ```
   - MUST be blocked by Constitutional Gate
   - Check for `permission denied` or `blocked` in output

2. **Mangle self-healing**
   ```bash
   timeout 60 ./nerd.exe check-mangle internal/mangle/*.mg 2>&1
   ```
   - All schema files must pass validation

---

## Phase 6: Full Integration Sweep

**Run comprehensive log check:**

```bash
rm -rf .nerd/logs/*
timeout 120 ./nerd.exe run "analyze the codebase structure" 2>&1

# Log analysis
echo "=== ERROR SCAN ==="
grep -i "error\|panic\|fatal\|nil pointer\|undefined\|deadlock" .nerd/logs/*.log | head -50

echo "=== CLEAN LOG CHECK ==="
for category in kernel session shards perception articulation campaign autopoiesis; do
  count=$(grep -i "error\|panic" .nerd/logs/*${category}*.log 2>/dev/null | wc -l)
  echo "$category: $count errors"
done
```

**ALL categories must show 0 errors.**

If errors found:
1. Parse the error message
2. Identify root cause
3. Fix systemically
4. Re-run sweep

---

## Phase 7: Demonstration App (The Proof)

Build a Mini-CRM application **autonomously** using codeNERD to prove stability.

**The App:** A Go CLI application with:
- Contact management (add, list, delete)
- SQLite persistence
- Unit tests with >80% coverage
- Clean error handling

**Execution:**

```bash
mkdir -p .nerd/ralph/demo-crm
cd .nerd/ralph/demo-crm

# Use codeNERD to build the app
timeout 600 ../../../nerd.exe run "Create a Go CLI contact management app with these requirements:
1. main.go - CLI with add/list/delete commands
2. contacts.go - Contact struct and CRUD operations
3. db.go - SQLite database layer
4. contacts_test.go - Unit tests for all operations
5. go.mod - Module definition

Use clean error handling. No panics. All errors wrapped and returned." 2>&1
```

**Verification:**

```bash
cd .nerd/ralph/demo-crm
go build -o crm.exe . 2>&1
go test -v ./... 2>&1
./crm.exe add --name "Test User" --email "test@example.com" 2>&1
./crm.exe list 2>&1
```

**Success criteria:**
- Builds without errors
- Tests pass
- Commands execute without panic
- Output is sensible

If demo app fails:
1. Analyze what codeNERD generated incorrectly
2. This indicates a systemic issue in the coder persona or tool execution
3. Fix the ROOT CAUSE in codeNERD, not the generated app
4. Re-run demo app generation

---

## Phase 8: Final Verification

**Complete log cleanliness check:**

```bash
echo "=== FINAL VERIFICATION ==="

# Build fresh
rm -f nerd.exe
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -o nerd.exe ./cmd/nerd 2>&1

# Clear logs
rm -rf .nerd/logs/*

# Run comprehensive test
timeout 300 ./nerd.exe run "Verify codeNERD is working correctly by:
1. Scanning the codebase
2. Querying the kernel for file_topology
3. Checking Mangle schemas are valid
Report any issues found." 2>&1

# Final log check
echo ""
echo "=== FINAL LOG ANALYSIS ==="
total_errors=0
for log in .nerd/logs/*.log; do
  errors=$(grep -ci "error\|panic\|fatal\|nil\|undefined" "$log" 2>/dev/null || echo 0)
  if [ "$errors" -gt 0 ]; then
    echo "FAIL: $log has $errors issues"
    total_errors=$((total_errors + errors))
  fi
done

if [ "$total_errors" -eq 0 ]; then
  echo "SUCCESS: All logs clean"
fi

# Demo app check
echo ""
echo "=== DEMO APP STATUS ==="
if [ -f .nerd/ralph/demo-crm/crm.exe ]; then
  cd .nerd/ralph/demo-crm
  go test -v ./... 2>&1
  cd ../../..
else
  echo "FAIL: Demo app not built"
fi
```

**Only when ALL checks pass, output:**

```
<promise>CODENERD PERFECTION ACHIEVED</promise>
```

---

## Root-Cause Investigation Template

When you find a bug, document it in `.nerd/ralph/bugs/BUG-XXX.md`:

```markdown
# BUG-XXX: <Short description>

## Symptom
What happened (error message, panic, etc.)

## Proximate Cause
The immediate trigger (nil pointer, missing validation, etc.)

## Root Cause (Five Whys)
1. Why did X happen? → Because Y
2. Why did Y happen? → Because Z
3. Why did Z happen? → Because W
4. Why did W happen? → Because V
5. Why did V happen? → Because <ROOT CAUSE>

## Systemic Fix
What code change prevents this class of bug forever?

## Files Changed
- `path/to/file.go:line` - Description of change

## Verification
How to verify the fix works?
```

---

## Anti-Patterns (FORBIDDEN)

You MUST NOT:
- Comment out broken code
- Delete corrupted artifacts
- Add nil checks without tracing nil source
- Wrap in recover() to hide panics
- Increase timeouts to hide slowness
- Add special cases for specific failures
- Disable features that stress test broke
- Use `// TODO: fix later` comments
- Mark bugs as fixed without verification

---

## Iteration Strategy

Each Ralph iteration:

1. **Read state file** - Know what phase you're on
2. **Run next test** - Based on current phase
3. **If pass** - Mark in state, move to next
4. **If fail** - Apply root-cause protocol, fix, verify, re-run
5. **Update state** - Increment iteration, update timestamps
6. **Check completion** - All phases done + logs clean + demo works?
7. **Output promise** - ONLY if truly complete

---

## Files to Monitor

| File | Purpose |
|------|---------|
| `.nerd/ralph/perfection_state.json` | Progress tracking |
| `.nerd/ralph/bugs/` | Bug documentation |
| `.nerd/ralph/demo-crm/` | Demonstration app |
| `.nerd/logs/*` | All log categories |
| `nerd.exe` | Built binary |

---

## Expected Duration

- Phase 1-5: 2-4 hours (depending on bugs found)
- Phase 6: 1-2 hours (integration sweep)
- Phase 7: 1-2 hours (demo app)
- Phase 8: 30 minutes (final verification)

Total: **4-8 hours** of Ralph iterations to achieve perfection.

---

## Remember

> "The artifact is NOT the bug - it is a SYMPTOM of a deeper systemic failure."
>
> "Deleting, commenting out, or patching the artifact is strictly forbidden."
>
> "Always trace back to the EARLIEST point where the bug could have been prevented."

You are building a coding agent that can create complex applications autonomously.
Every root-cause fix makes codeNERD stronger.
Every band-aid makes it weaker.
Choose strength.
