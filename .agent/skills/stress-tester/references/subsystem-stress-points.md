# Subsystem Stress Points Reference

Comprehensive catalog of all codeNERD subsystems with entry points, failure modes, config limits, and log categories for stress testing.

## Quick Reference: CLI Entry Points by Subsystem

| Subsystem | CLI Command | Log Category |
|-----------|-------------|--------------|
| Kernel | `nerd query`, `nerd run` | `/kernel` |
| ShardManager | `nerd spawn`, `nerd agents` | `/shards` |
| VirtualStore | `nerd run` (actions) | `/virtual_store` |
| Perception | `nerd perception` | `/perception` |
| Articulation | All shard outputs | `/articulation` |
| Campaign | `nerd campaign *` | `/campaign` |
| Ouroboros | `nerd tool generate` | `/autopoiesis` |
| Thunderdome | NemesisShard activation | `/autopoiesis` |
| World Scanner | `nerd scan`, `nerd init` | `/world` |
| Dream State | `nerd dream` | `/dream` |
| Shadow Mode | `nerd shadow`, `nerd whatif` | `/shadow` |
| Browser | `nerd browser *` | `/browser` |
| TDD Loop | `nerd test`, TesterShard | `/tester` |
| JIT Compiler | `nerd jit`, all prompts | `/jit` |
| Context | All LLM interactions | `/context` |
| Store | Persistence operations | `/store` |

---

## 1. KERNEL & CORE RUNTIME

### 1.1 RealKernel (internal/core/kernel.go)

**Entry Points:**
```bash
nerd query "predicate_name"     # Query facts
nerd run "instruction"          # Full OODA loop
nerd check-mangle file.mg       # Validate Mangle syntax
```

**Critical Methods:**
- `LoadFacts(facts)` - Batch fact loading
- `Query(predicate)` - Fact querying
- `Assert(fact)` - Single fact insertion
- `Retract(predicate)` - Fact removal

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Panic on boot | Corrupted .mg files | `CRITICAL: Kernel failed to boot` |
| Derivation explosion | Cyclic rules | Timeout, memory exhaustion |
| Gas limit exceeded | 100k+ derived facts | Query returns partial results |
| Undeclared predicate | Missing Decl | Runtime error |

**Config Limits:**
- `max_facts_in_kernel: 250000` - Hard limit on EDB size
- `max_derived_facts_limit: 100000` - Gas limit per derivation

**Stress Commands:**
```bash
# Conservative: Query existing facts
nerd query "user_intent"
nerd query "shard_executed"

# Aggressive: Load many facts
nerd run "analyze the entire codebase and remember everything"

# Chaos: Cyclic derivation
# Load assets/cyclic_rules.mg then query
```

---

### 1.2 ShardManager (internal/core/shard_manager.go)

**Entry Points:**
```bash
nerd spawn coder "task"         # Spawn ephemeral shard
nerd spawn reviewer "task"      # Spawn reviewer
nerd agents                     # List agents
nerd define-agent --name X --topic Y  # Create Type U shard
```

**Critical Methods:**
- `Spawn(shardType, task)` - Create shard
- `SpawnWithContext(ctx, type, task, sessionCtx)` - Full spawn
- `GetActiveShards()` - List active
- `StopAll()` - Shutdown

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Queue overflow | 100+ concurrent spawns | `ErrQueueFull` |
| Shard limit hit | >4 concurrent shards | Queued/rejected |
| Injection failure | Missing dependencies | Nil pointer in shard |
| Unknown shard type | Typo in type | `unknown shard type` error |

**Config Limits:**
- `max_concurrent_shards: 4` - Concurrent shard limit
- `max_queue_size: 100` - Spawn queue capacity

**Stress Commands:**
```bash
# Conservative: Spawn one shard
nerd spawn coder "write a hello world"

# Aggressive: Rapid spawning
nerd spawn coder "task 1" && nerd spawn tester "task 2" && nerd spawn reviewer "task 3"

# Chaos: Exceed queue
# Rapidly spawn 100+ shards in loop
```

---

### 1.3 SpawnQueue (internal/core/spawn_queue.go)

**Entry Points:**
- Indirect via ShardManager.Spawn()

**Critical Methods:**
- `Submit(request)` - Add to queue
- `CanAccept(priority)` - Check capacity
- `selectNextRequest()` - Priority selection

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Fast fail | Queue at 100 | Immediate rejection |
| Priority inversion | All high-priority | Low-priority starved |
| Deadline expiration | Long queue wait | Timeout after submission |
| Worker contention | 2 workers, many requests | 50ms polling latency |

**Config Limits:**
- `maxQueueSize: 100` - Queue capacity
- `numWorkers: 2` - Concurrent workers
- `backpressureThreshold: 0.7` - 70% utilization warning

---

### 1.4 VirtualStore (internal/core/virtual_store.go)

**Entry Points:**
- All action execution flows through VirtualStore
- `nerd run "instruction"` triggers action routing

**Critical Methods:**
- `RouteAction(action)` - Main dispatcher
- `ExecuteTool(name, input)` - Run generated tools
- `Get(predicate)` - Virtual predicate resolution

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Permission denied | Unauthorized action | `permission denied` error |
| Action timeout | Long-running action | Partial completion |
| Tool not found | Unknown tool name | `tool not found` |
| Action crash | Tool panic | Action failure |

**Stress Commands:**
```bash
# Conservative: Simple action
nerd run "read the README"

# Aggressive: Complex action chain
nerd run "refactor all Go files to use context.Context"

# Chaos: Invalid actions
nerd run "execute rm -rf /"  # Should be blocked by constitution
```

---

### 1.5 LimitsEnforcer (internal/core/limits.go)

**Entry Points:**
- Automatic enforcement during operations

**Critical Methods:**
- `CheckMemory()` - RAM usage validation
- `CheckSessionDuration()` - Time limit check
- `CheckShardLimit()` - Concurrent shard check
- `CheckAll()` - Composite check

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Memory exceeded | >12GB usage | Warning logged (not enforced!) |
| Session timeout | >120 min | Session terminates |
| Shard limit | >4 concurrent | Queue blocking |

**Config Limits:**
- `max_total_memory_mb: 12288` - 12GB RAM limit
- `max_session_duration_min: 120` - 2 hour session limit
- `max_concurrent_shards: 4` - Shard limit

---

### 1.6 Mangle Self-Healing System

The Mangle Self-Healing system provides validation and auto-repair for Mangle rules.

#### 1.6.1 PredicateCorpus (internal/core/predicate_corpus.go)

**Entry Points:**
- Loaded automatically at kernel boot
- Used by MangleRepairShard for validation
- Used by check-mangle CLI command

**Critical Methods:**
- `NewPredicateCorpus()` - Load embedded corpus
- `IsDeclared(name)` - Check if predicate exists
- `ValidatePredicates(names)` - Bulk validation
- `GetByDomain(domain)` - Domain-specific predicates
- `GetErrorPatterns()` - Common error patterns

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Corpus not loaded | Embedded DB missing | `Predicate corpus not available` |
| Query failure | Corrupted DB | SQL errors |
| Stats unavailable | Missing tables | Stats return zeros |

**Data:**
- 799 predicates (EDB + IDB)
- 474 examples
- 12 error patterns

**Stress Commands:**
```bash
# Conservative: Check corpus loaded
nerd status  # Should show kernel initialized

# Aggressive: Validate many predicates
nerd check-mangle .nerd/mangle/learned.mg
nerd check-mangle internal/mangle/*.gl

# Chaos: Invalid predicate detection
echo "bad(X) :- hallucinated_pred(X)." > /tmp/bad.mg
nerd check-mangle /tmp/bad.mg
```

---

#### 1.6.2 MangleRepairShard (internal/shards/system/mangle_repair.go)

**Entry Points:**
- Registered as Type S system shard (auto-start)
- Intercepts rule validation via ValidateAndRepair()
- Called by autopoiesis before rule persistence

**Critical Methods:**
- `ValidateAndRepair(ctx, rule)` - Validate and auto-repair
- `validateRule(rule, kernel, corpus)` - Multi-phase validation
- `classifyErrors(errors)` - Error categorization
- `repairRule(ctx, rule, errors)` - LLM-powered repair

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Repair loop exceeded | Max 3 retries | Rule rejected |
| LLM unavailable | No client configured | Falls back to rejection |
| Corpus not attached | Missing SetCorpus() | Schema check skipped |
| Kernel not attached | Missing SetParentKernel() | Syntax check skipped |

**Config:**
- `maxRetries: 3` - Max repair attempts before rejection

**Stress Commands:**
```bash
# Logs show repair shard activity
Select-String -Path ".nerd/logs/*system*.log" -Pattern "MangleRepair"

# Check repair shard registered
nerd agents  # Should show mangle_repair in system shards
```

---

#### 1.6.3 PredicateSelector (internal/prompt/predicate_selector.go)

**Entry Points:**
- Used by FeedbackLoop for JIT predicate selection
- Called during Mangle rule generation prompts

**Critical Methods:**
- `Select(ctx)` - Context-aware selection
- `SelectForContext(shardType, intentVerb, domain)` - Interface for FeedbackLoop
- `FormatForPrompt(predicates)` - Format for LLM injection
- `SelectForMangleGeneration(shardType, intentVerb)` - Convenience method

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Empty corpus | Corpus not configured | `no predicate corpus configured` |
| No matching domain | Unknown domain | Falls back to core predicates |
| Selection overflow | MaxPredicates=0 | Default to 100 |

**Config:**
- `MaxPredicates: 100` - Default max predicates to select

**Stress Commands:**
```bash
# Check JIT selection in logs
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "JIT selected"

# Trigger rule generation (uses selector)
nerd run "create a rule to track file modifications"
```

---

#### 1.6.4 FeedbackLoop Integration (internal/mangle/feedback/loop.go)

**Entry Points:**
- GenerateAndValidate() - Main rule generation
- SetPredicateSelector() - Configure JIT selection

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Selector not set | Missing SetPredicateSelector() | Uses full predicate list |
| Selection fails | Corpus error | Falls back to validator.GetDeclaredPredicates() |

**Stress Commands:**
```bash
# Check predicate count in prompts
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "predicates for domain"
```

---

## 2. PERCEPTION LAYER

### 2.1 Transducer (internal/perception/transducer.go)

**Entry Points:**
```bash
nerd perception "natural language input"  # Direct test
nerd run "instruction"                     # Via OODA loop
```

**Critical Methods:**
- `ParseIntent(input)` - Main parser
- `classifyIntent(input)` - LLM classification
- `extractPiggyback(response)` - JSON extraction

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Parse failure | Ambiguous input | Fallback to default verb |
| Piggyback corruption | Truncated JSON | Control packet lost |
| Taxonomy miss | Unknown verb | Generic classification |
| LLM timeout | API issues | Parse failure |

**Stress Commands:**
```bash
# Conservative: Clear intent
nerd perception "review the code for security issues"

# Aggressive: Ambiguous input
nerd perception "maybe do something with the files if you want"

# Chaos: Malformed input
nerd perception "!@#$%^&*() 漢字 🚀 \x00\x01\x02"
```

---

### 2.2 LLMClient (internal/perception/client.go)

**Entry Points:**
- All LLM interactions

**Critical Methods:**
- `Complete(prompt)` - Query LLM
- `CompleteWithSystem(system, user)` - With system prompt
- `NewClientFromEnv()` - Auto-detect provider

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| API key missing | No env var | Client creation fails |
| Rate limit | Too many calls | 429 error |
| Timeout | Slow response | Context deadline |
| Provider mismatch | Wrong API key format | Auth error |

**Config:**
- Provider: `zai`, `anthropic`, `openai`, `gemini`, `xai`, `openrouter`
- No rate limiting configured (risk!)

---

### 2.3 SemanticClassifier (internal/perception/semantic_classifier.go)

**Entry Points:**
- Via Transducer.classifyIntent()

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Misclassification | Edge case input | Wrong shard selected |
| Confidence threshold | Ambiguous intent | Multiple matches |
| Learning loss | Not persisted | Repeats mistakes |

---

## 3. ARTICULATION LAYER

### 3.1 Emitter (internal/articulation/emitter.go)

**Entry Points:**
- All shard output processing

**Critical Methods:**
- `ProcessResponse(raw)` - Parse LLM output
- `ExtractPiggyback(response)` - JSON extraction
- `ValidateControlPacket(packet)` - Structure validation

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Premature articulation | surface_response first | Control packet lost |
| Truncated JSON | Long response | Parse failure |
| Invalid ControlPacket | Missing fields | Action not executed |
| Memory exhaustion | Huge response | OOM |

**Stress Commands:**
```bash
# Conservative: Simple output
nerd spawn coder "write hello world"

# Aggressive: Complex output
nerd spawn coder "generate a 1000-line Go file with documentation"

# Chaos: Force truncation
nerd spawn coder "generate maximum possible output"
```

---

### 3.2 PromptAssembler (internal/articulation/prompt_assembler.go)

**Entry Points:**
- JIT compilation, all prompts

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Template error | Missing variable | Nil dereference |
| Context injection | Malicious context | Prompt corruption |
| Budget overflow | Too much context | Truncation |

---

## 4. SHARDS

### 4.1 CoderShard (internal/shards/coder/coder.go)

**Entry Points:**
```bash
nerd spawn coder "task"
nerd fix "issue"
nerd refactor "target"
nerd run "create X"
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Edit atomicity | Error mid-edit | File corruption |
| Build timeout | Hanging build | Incomplete |
| Language detection | Unusual extension | Wrong tooling |
| Parallel edits | Same file | Race condition |

**Stress Commands:**
```bash
# Aggressive: Large refactor
nerd spawn coder "refactor all 500 Go files to use new error handling"

# Chaos: Conflicting edits
# Spawn multiple coders editing same file
```

---

### 4.2 TesterShard (internal/shards/tester/tester.go)

**Entry Points:**
```bash
nerd test "target"
nerd spawn tester "task"
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Framework detection | Unusual setup | Wrong test runner |
| Test timeout | Infinite loop | Hung process |
| Coverage parsing | Format variation | Incomplete metrics |
| TDD infinite loop | Unfixable test | Retry exhaustion |

---

### 4.3 ReviewerShard (internal/shards/reviewer/reviewer.go)

**Entry Points:**
```bash
nerd review "path"
nerd review "path" --andEnhance
nerd security "path"
nerd analyze "path"
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Finding explosion | Large codebase | Memory exhaustion |
| Custom rules error | Invalid JSON | Rules not loaded |
| Specialist cascade | Deep analysis | Too many sub-shards |
| Division by zero | Complexity metrics | Panic |

---

### 4.4 ResearcherShard (internal/shards/researcher/researcher.go)

**Entry Points:**
```bash
nerd spawn researcher "topic"
nerd explain "code"
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| HTML parsing bomb | Malformed page | Parser crash |
| Connection exhaustion | 100 concurrent | Pool drained |
| Rate limit | Context7 API | 429 errors |
| Domain filter bypass | URL manipulation | Security risk |

---

### 4.5 NemesisShard (internal/shards/nemesis/nemesis.go)

**Entry Points:**
- Automatic during code review gauntlet

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Attack explosion | Many vectors | Resource exhaustion |
| Tool nesting | Recursive generation | Unbounded tools |
| Vulnerability DB growth | Long session | Disk exhaustion |

---

## 5. AUTOPOIESIS

### 5.1 OuroborosLoop (internal/autopoiesis/ouroboros.go)

**Entry Points:**
```bash
nerd tool generate "description"
nerd tool list
nerd tool run "name" "input"
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Generation timeout | Complex tool | Incomplete code |
| Safety bypass | Forbidden imports | Unsafe tool |
| Compile failure | Invalid Go syntax | Tool not created |
| Infinite nesting | Tool generates tool | Unbounded loop |

**Stress Commands:**
```bash
# Conservative: Simple tool
nerd tool generate "a tool that counts lines in a file"

# Aggressive: Complex tool
nerd tool generate "a tool that analyzes Go AST and reports complexity"

# Chaos: Self-referential
nerd tool generate "a tool that generates other tools"
```

---

### 5.2 Thunderdome (internal/autopoiesis/thunderdome.go)

**Entry Points:**
- NemesisShard.RunGauntlet()

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Attack parallelization | Concurrent attacks | Race conditions |
| Sandbox escape | Resource limits | Memory exhaustion |
| Artifact growth | Kept artifacts | Disk exhaustion |
| TODO: Incomplete execution | Attack not run | False security |

---

## 6. CAMPAIGN ORCHESTRATION

### 6.1 Orchestrator (internal/campaign/orchestrator.go)

**Entry Points:**
```bash
nerd campaign start "goal"
nerd campaign status
nerd campaign pause
nerd campaign resume
nerd campaign list
nerd launchcampaign "goal"
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Decomposition explosion | Huge goal | 1000+ tasks |
| Phase timeout | Long phase | Campaign abort |
| Checkpoint corruption | Crash during save | Lost state |
| Context overflow | Many phases | Memory exhaustion |

**Stress Commands:**
```bash
# Conservative: Small campaign
nerd campaign start "add a logout button"

# Aggressive: Large campaign
nerd campaign start "implement a complete e-commerce platform"

# Chaos: Impossible goal
nerd campaign start "rewrite the Linux kernel in Rust in 10 minutes"
```

---

## 7. WORLD MODEL

### 7.1 Scanner (internal/world/fs.go)

**Entry Points:**
```bash
nerd scan
nerd init
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Symlink loop | Circular links | Infinite traversal |
| Permission denied | Restricted dirs | Cascade failures |
| Deep nesting | 1000+ depth | Stack overflow |
| Large file | >100MB file | Memory exhaustion |

---

### 7.2 Holographic (internal/world/holographic.go)

**Entry Points:**
- Context building for all prompts

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Impact explosion | Many changes | Memory exhaustion |
| Cycle in graph | Circular deps | Infinite loop |
| Timeout | Large codebase | Incomplete context |

---

## 8. ADVANCED FEATURES

### 8.1 Dream State (internal/core/dream_router.go)

**Entry Points:**
```bash
nerd dream "scenario"
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Consultant overload | 4 × 100 perspectives | API exhaustion |
| Queue overflow | Ouroboros queue full | Tools dropped |
| Learning cascade | Many store writes | Database lock |

---

### 8.2 Shadow Mode (cmd/nerd/chat/shadow.go)

**Entry Points:**
```bash
nerd shadow "action"
nerd whatif "change"
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Simulation timeout | Complex derivation | 2-min timeout |
| Kernel crash | Invalid hypothesis | Shadow fails |

---

### 8.3 Browser (internal/browser/)

**Entry Points:**
```bash
nerd browser launch
nerd browser session "url"
nerd browser snapshot "session-id"
```

**Failure Modes:**
| Failure | Trigger | Symptom |
|---------|---------|---------|
| Process crash | Chrome OOM | Orphan processes |
| Connection timeout | Slow page | 60s timeout |
| DOM explosion | Large page | Memory exhaustion |

---

## Log Categories Reference

All 22 log categories for stress test monitoring:

| Category | File Pattern | What to Watch |
|----------|--------------|---------------|
| `/boot` | `*boot*.log` | Initialization errors |
| `/kernel` | `*kernel*.log` | Derivation, facts |
| `/shards` | `*shards*.log` | Spawn, lifecycle |
| `/perception` | `*perception*.log` | Intent parsing |
| `/articulation` | `*articulation*.log` | Output processing |
| `/campaign` | `*campaign*.log` | Phase execution |
| `/autopoiesis` | `*autopoiesis*.log` | Tool generation |
| `/dream` | `*dream*.log` | Consultations |
| `/world` | `*world*.log` | File scanning |
| `/context` | `*context*.log` | Compression |
| `/store` | `*store*.log` | Persistence |
| `/api` | `*api*.log` | LLM calls |
| `/coder` | `*coder*.log` | Code generation |
| `/tester` | `*tester*.log` | Test execution |
| `/reviewer` | `*reviewer*.log` | Code review |
| `/researcher` | `*researcher*.log` | Research |
| `/tools` | `*tools*.log` | Tool execution |
| `/routing` | `*routing*.log` | Action dispatch |
| `/virtual_store` | `*virtual_store*.log` | Action execution |
| `/embedding` | `*embedding*.log` | Vector ops |
| `/session` | `*session*.log` | Session lifecycle |
| `/system_shards` | `*system_shards*.log` | Type S shards |
