# codeNERD Skill Registry

This registry provides detailed documentation for all skills in the codeNERD development ecosystem. Each entry includes the skill's purpose, trigger conditions, key capabilities, bundled resources, and integration points with other skills.

---

## Registry Index

| ID | Skill | Domain | Integration Level |
|----|-------|--------|-------------------|
| [SK-001](#sk-001-codenerd-builder) | codenerd-builder | Architecture | Core |
| [SK-002](#sk-002-mangle-programming) | mangle-programming | Logic Language | Core |
| [SK-003](#sk-003-go-architect) | go-architect | Go Patterns | Core |
| [SK-004](#sk-004-charm-tui) | charm-tui | Terminal UI | Component |
| [SK-005](#sk-005-research-builder) | research-builder | Knowledge Systems | Component |
| [SK-006](#sk-006-rod-builder) | rod-builder | Browser Automation | Component |
| [SK-007](#sk-007-log-analyzer) | log-analyzer | Debugging | Utility |
| [SK-008](#sk-008-skill-creator) | skill-creator | Meta/Tooling | Utility |
| [SK-009](#sk-009-integration-auditor) | integration-auditor | Integration/QA | Core |
| [SK-010](#sk-010-cli-engine-integration) | cli-engine-integration | LLM Integration | Core |
| [SK-011](#sk-011-prompt-architect) | prompt-architect | Prompt Engineering | Core |
| [SK-012](#sk-012-stress-tester) | stress-tester | Testing & QA | Utility |
| [SK-013](#sk-013-gemini-features) | gemini-features | LLM Integration | Component |

---

## SK-001: codenerd-builder

**Name:** `codenerd-builder`

**Domain:** codeNERD Architecture & Implementation

**Description:** Build the codeNERD Logic-First Neuro-Symbolic coding agent framework. This skill should be used when implementing components of the codeNERD architecture including the Mangle kernel, Perception/Articulation Transducers, ShardAgents, Virtual Predicates, TDD loops, Piggyback Protocol, Dream State, Dreamer (Precog Safety), Legislator, Ouroboros Loop, and DifferentialEngine. Use for tasks involving Google Mangle logic, Go runtime integration, or any neuro-symbolic agent development following the Creative-Executive Partnership pattern.

### Trigger Conditions

- Implementing kernel, transducers, or shard components
- Working with the OODA loop (Observe → Orient → Decide → Act)
- Building Virtual Predicates or VirtualStore extensions
- Implementing Piggyback Protocol dual-channel output
- Campaign orchestration or multi-phase goal execution
- Autopoiesis, Ouroboros, or self-improvement systems
- Constitutional safety or permission systems
- Dream State or Dreamer (Precog Safety) implementation
- Semantic classification or vector-based intent matching
- Adversarial co-evolution or battle-testing systems (Nemesis, Thunderdome)
- Attack generation, armory persistence, or regression attack suites

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Architecture Patterns | Neuro-symbolic component structure, Creative-Executive Partnership |
| Kernel Development | Mangle engine integration, fact management, rule derivation |
| Shard Lifecycle | Type A/B/U/S shard patterns, ShardManager API |
| Transduction | NL↔Mangle atom conversion patterns |
| Semantic Classification | Vector-based intent matching with Mangle inference rules |
| JIT Prompt Compilation | Dynamic prompt assembly from atomic units with dependency resolution |
| Prompt Atom Management | Category-based atom organization, unified storage in knowledge DBs |
| Safety Systems | Constitutional gates, Dreamer simulation, SafetyChecker |
| Adversarial Testing | NemesisShard, Thunderdome battles, PanicMaker attacks, Armory regression |
| Campaign System | Multi-phase goals, context paging, decomposer |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `references/architecture.md` | Reference | Theoretical foundations, neuro-symbolic principles |
| `references/mangle-schemas.md` | Reference | Complete Mangle schema documentation |
| `references/implementation-guide.md` | Reference | Go implementation patterns |
| `references/piggyback-protocol.md` | Reference | Dual-channel protocol specification |
| `references/campaign-orchestrator.md` | Reference | Multi-phase execution system |
| `references/autopoiesis.md` | Reference | Self-creation and Ouroboros state machine |
| `references/shard-agents.md` | Reference | Shard types and ShardManager API |
| `references/logging-system.md` | Reference | 22-category logging system |
| `references/skill-registry.md` | Reference | This file - skill ecosystem documentation |
| `references/semantic-classification.md` | Reference | Neuro-symbolic intent classification with vectors |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| mangle-programming | Required for writing kernel schemas and policy rules |
| go-architect | Required for all Go implementation work |
| prompt-architect | Used for JIT prompt compilation, atom management, shard prompt design |
| charm-tui | Used for CLI interface development |
| log-analyzer | Used for debugging kernel derivations |
| research-builder | Used for ResearcherShard implementation |

### Key Implementation Files

```text
internal/core/kernel.go                    - Mangle engine + fact management
internal/core/virtual_store.go             - FFI to external systems
internal/core/shard_manager.go             - Shard lifecycle management
internal/core/dreamer.go                   - Precog safety simulation
internal/perception/transducer.go          - NL → Atoms conversion
internal/perception/semantic_classifier.go - Vector-based intent classification
internal/articulation/emitter.go           - Atoms → NL (Piggyback)
internal/articulation/prompt_assembler.go  - JIT prompt compilation integration
internal/prompt/compiler.go                - JIT prompt compiler
internal/prompt/selector.go                - Atom selection (Vector + Mangle)
internal/prompt/resolver.go                - Dependency resolution
internal/prompt/budget.go                  - Token budget management
internal/prompt/loader.go                  - YAML→SQLite prompt loading
internal/autopoiesis/ouroboros.go          - Tool self-generation
internal/autopoiesis/thunderdome.go        - Adversarial battle arena
internal/autopoiesis/panic_maker.go        - Attack vector generation
internal/autopoiesis/chaos.mg              - Adversarial Mangle schema
internal/shards/nemesis/nemesis.go         - Adversarial testing specialist
internal/shards/nemesis/armory.go          - Attack persistence for regression
internal/shards/nemesis/attack_runner.go   - Sandboxed attack execution
cmd/tools/corpus_builder/main.go           - Build-time corpus generator
build/prompt_atoms/**/*.yaml               - Atomic prompt source files
```

---

## SK-002: mangle-programming

**Name:** `mangle-programming`

**Domain:** Logic Language & Datalog

**Description:** Master Google's Mangle declarative programming language for deductive database programming, constraint-like reasoning, and software analysis. From basic facts to production deployment, graph traversal to vulnerability detection, theoretical foundations to optimization. Includes comprehensive AI failure mode prevention (atom/string confusion, aggregation syntax, safety violations, stratification errors). Complete encyclopedic reference with progressive disclosure architecture.

### Trigger Conditions

- Writing or debugging Mangle schemas, rules, or queries
- Implementing aggregation pipelines with `|>` syntax
- Designing recursive predicates (transitive closure, graph traversal)
- Fixing safety violations or stratification errors
- Converting data to Mangle facts
- Understanding Datalog semantics in codeNERD context

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Syntax Reference | Facts, rules, queries, declarations, all operators |
| Aggregation | Pipe transform syntax `\|> let X = fn:Sum(Y)` |
| Negation | Safety requirements, bound variable rules |
| Stratification | Layer ordering, recursive rule design |
| Type System | Mangle's type declarations and constraints |
| AI Failure Prevention | Atom/string confusion, common syntax errors |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `references/010-FUNDAMENTALS.md` | Reference | Core syntax, facts, rules, queries |
| `references/020-TYPES_TERMS.md` | Reference | Type system, terms, declarations |
| `references/050-ADVANCED_FEATURES.md` | Reference | Aggregation, transforms, pipes |
| `references/080-PRACTICAL_PATTERNS.md` | Reference | Real-world usage patterns |
| `references/150-AI_FAILURE_MODES.md` | Reference | Critical AI anti-patterns |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| codenerd-builder | Mangle is the kernel's logic language |
| log-analyzer | Uses Mangle for declarative log queries |
| go-architect | Mangle facts created from Go structs via ToAtom() |

### Version Info

- **Mangle Version:** 0.4.0 (November 1, 2024)
- **Skill Version:** 0.6.0

---

## SK-003: go-architect

**Name:** `go-architect`

**Domain:** Production Go Patterns

**Description:** Write production-ready, idiomatic Go code following Uber Go Style Guide patterns. This skill prevents common AI coding agent failures including goroutine leaks, race conditions, improper error handling, context mismanagement, and memory leaks. Includes Mangle integration patterns for feeding codeNERD logic systems. Use when writing, reviewing, or refactoring any Go code.

### Trigger Conditions

- Writing any Go code in codeNERD
- Implementing concurrent patterns (goroutines, channels)
- Error handling design
- Context propagation
- Code review for Go safety issues
- Refactoring existing Go code

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Goroutine Safety | Leak prevention, lifecycle management, WaitGroups |
| Error Handling | Wrapping, sentinel errors, error types |
| Context Patterns | Cancellation, timeouts, value propagation |
| Concurrency | Channel patterns, mutex vs channels, select |
| Testing | Table-driven tests, mocks, benchmarks |
| Mangle Integration | ToAtom() patterns, fact generation |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `SKILL.md` | Core | Complete reference (self-contained) |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| codenerd-builder | All codeNERD Go code must follow these patterns |
| charm-tui | TUI code requires goroutine safety |
| rod-builder | Browser automation needs proper context handling |
| research-builder | Knowledge systems need proper error handling |

### Version Info

- **Go Version:** 1.21+
- **Skill Version:** 1.0.0

---

## SK-004: charm-tui

**Name:** `charm-tui`

**Domain:** Terminal User Interfaces

**Description:** Build production-ready terminal user interfaces with Bubbletea and Lipgloss. This skill should be used when creating CLI applications with rich interactive UIs, implementing the Model-View-Update (MVU) pattern, styling terminal output, or building components like forms, tables, lists, spinners, and progress bars. Covers stability patterns, goroutine safety, and the complete Charm ecosystem.

### Trigger Conditions

- Building the `nerd` CLI interface
- Creating interactive terminal components
- Implementing MVU (Model-View-Update) architecture
- Styling terminal output with colors and borders
- Building forms, tables, lists, or navigation
- Creating spinners, progress bars, or loading states

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Bubbletea | MVU framework, message handling, commands |
| Lipgloss | Styling system, colors, borders, layouts |
| Bubbles | Pre-built components (forms, lists, tables) |
| Glamour | Markdown rendering in terminal |
| Stability | Goroutine safety, resize handling, cleanup |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `SKILL.md` | Core | Quick start, patterns, component reference |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| codenerd-builder | Used for cmd/nerd CLI development |
| go-architect | TUI code must follow Go safety patterns |

---

## SK-005: research-builder

**Name:** `research-builder`

**Domain:** Knowledge Systems & Ingestion

**Description:** Build production-ready knowledge research and ingestion systems for codeNERD ShardAgents. Use when implementing ResearcherShard functionality, llms.txt/Context7-style documentation gathering, knowledge atom extraction, 4-tier memory storage, or specialist knowledge hydration. Includes deep research patterns, quality scoring, LLM enrichment, and persistence strategies.

### Trigger Conditions

- Implementing ResearcherShard functionality
- Building llms.txt parsing pipelines
- Knowledge atom extraction and storage
- 4-tier memory (RAM, Vector, Graph, Cold) integration
- Specialist agent hydration with domain knowledge
- Quality scoring for documentation

### Key Capabilities

| Capability | Description |
|------------|-------------|
| llms.txt Standard | AI-optimized documentation discovery |
| Context7 Patterns | Multi-stage processing, quality scoring |
| KnowledgeAtom Schema | Structured knowledge representation |
| 4-Tier Memory | RAM/Vector/Graph/Cold persistence |
| Specialist Hydration | Pre-loading expert agents |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `SKILL.md` | Core | Architecture, patterns, implementation guide |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| codenerd-builder | ResearcherShard is a core shard type |
| rod-builder | Browser automation for web scraping |
| mangle-programming | Knowledge atoms become Mangle facts |
| go-architect | Go patterns for concurrent fetching |

### Key Implementation Files

```text
internal/shards/researcher.go     - ResearcherShard (1,821 lines)
internal/store/local.go           - LocalStore with 4-tier memory
internal/store/learning.go        - LearningStore for autopoiesis
```

---

## SK-006: rod-builder

**Name:** `rod-builder`

**Domain:** Browser Automation

**Description:** Build production-ready browser automation with Rod (go-rod/rod). Use when implementing Chrome DevTools Protocol automation, web scraping, E2E testing, browser session management, or programmatic browser control. Includes Rod API patterns, CDP event handling, Chromium configuration, launcher flags, testing strategies, and production-grade best practices.

### Trigger Conditions

- Building web scrapers or data extraction
- Browser automation workflows
- E2E testing with real browsers
- Session management for multiple browser contexts
- CDP (Chrome DevTools Protocol) debugging
- Screenshot/PDF generation
- DOM projection for codeNERD's world model

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Rod API | Browser/Page/Element hierarchy |
| CDP Events | Real-time event handling, network monitoring |
| Launcher | Chromium configuration, flags, profiles |
| Session Management | Incognito contexts, cookies, storage |
| Testing | Hijacking, mocking, deterministic tests |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `SKILL.md` | Core | API reference, patterns, examples |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| research-builder | Browser automation for knowledge scraping |
| go-architect | Proper context handling, connection pooling |
| codenerd-builder | Browser peripheral for world model |

---

## SK-007: log-analyzer

**Name:** `log-analyzer`

**Domain:** Debugging & Diagnostics

**Description:** Analyze codeNERD system logs using Mangle logic programming. This skill should be used when debugging codeNERD execution, tracing cross-system interactions, identifying error patterns, analyzing performance bottlenecks, or correlating events across the 22 logging categories. Converts log files to Mangle facts for declarative querying.

### Trigger Conditions

- Debugging codeNERD execution issues
- Tracing cross-system interactions
- Identifying error patterns across categories
- Performance bottleneck analysis
- Correlating events from multiple log files
- Root cause analysis for failures

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Log Parsing | Convert logs to Mangle facts |
| 22 Categories | Query across all codeNERD subsystems |
| Pattern Detection | Recursive queries for error chains |
| Correlation | Cross-category event linking |
| Performance | Timing analysis, slow operation detection |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `scripts/parse_log.py` | Script | Log → Mangle fact conversion |
| `scripts/analyze_logs.py` | Script | Run Mangle queries on facts |
| `references/query-patterns.md` | Reference | Pre-built analysis queries |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| mangle-programming | Uses Mangle for declarative queries |
| codenerd-builder | Analyzes logs from all codeNERD components |

### The 22 Log Categories

```text
boot, session, kernel, api, perception, articulation, routing, tools,
virtual_store, shards, coder, tester, reviewer, researcher, system_shards,
dream, autopoiesis, campaign, context, world, embedding, store
```

---

## SK-008: skill-creator

**Name:** `skill-creator`

**Domain:** Meta / Skill Tooling

**Description:** Guide for creating effective skills. This skill should be used when users want to create a new skill (or update an existing skill) that extends Claude's capabilities with specialized knowledge, workflows, or tool integrations.

### Trigger Conditions

- Creating a new skill from scratch
- Updating or improving existing skills
- Packaging skills for distribution
- Understanding skill anatomy and best practices
- Writing effective SKILL.md files

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Skill Anatomy | SKILL.md structure, bundled resources |
| Creation Workflow | 6-step process from understanding to iteration |
| Resource Types | scripts/, references/, assets/ organization |
| Packaging | Validation and distribution |
| Registry Integration | Registering new skills in codenerd-builder |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `scripts/init_skill.py` | Script | Generate new skill template |
| `scripts/package_skill.py` | Script | Validate and package skill |
| `scripts/quick_validate.py` | Script | Fast validation check |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| codenerd-builder | New skills must be registered in the skill registry |

### Registry Integration Workflow

When creating a new skill for codeNERD:

1. Create the skill using the standard 6-step process
2. Add an entry to this registry file with:
   - Unique SK-XXX identifier
   - Complete metadata and trigger conditions
   - Bundled resources documentation
   - Integration points with other skills
3. Update the "Integrated Skills Ecosystem" section in `codenerd-builder/SKILL.md`
4. Update the Skill Index table in this file

---

## SK-009: integration-auditor

**Name:** `integration-auditor`

**Domain:** Integration & Quality Assurance

**Description:** Comprehensive audit and repair of integration wiring across all 39+ codeNERD systems. This skill should be used when implementing new features, creating new shards, debugging "code exists but doesn't run" issues, or verifying complete system integration. Covers Mangle kernel wiring, compression systems, Piggyback protocol, autopoiesis, storage tiers, campaign orchestration, TUI integration, and all 4 shard lifecycle types.

### Trigger Conditions

- Implementing any new feature in codeNERD
- Creating or modifying shards (Type A/B/U/S)
- Debugging "works in isolation but fails in system" issues
- Pre-commit integration verification
- Onboarding to codeNERD architecture
- Investigating silent failures or missing behavior
- Verifying all wiring points are connected

### Key Capabilities

| Capability | Description |
|------------|-------------|
| 39-System Coverage | Maps all integration points across the entire codebase |
| Shard Type Matrix | Complete wiring requirements for Type A/B/U/S shards |
| Automated Diagnostics | `audit_wiring.py` script for gap detection |
| Audit Workflows | Step-by-step checklists for features, shards, debugging |
| Boot Sequence | Documents 13-step initialization order and dependencies |
| Failure Catalog | 25+ common symptoms with root causes and fixes |
| Live Testing Patterns | Production-ready test code for wiring verification |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `scripts/audit_wiring.py` | Script | Automated wiring gap detection |
| `references/systems-integration-map.md` | Reference | Complete 39-system integration map |
| `references/shard-wiring-matrix.md` | Reference | Type A/B/U/S wiring requirements |
| `references/wiring-checklist.md` | Reference | 9-point integration checklist |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| codenerd-builder | Uses architecture patterns, validates component wiring |
| mangle-programming | Audits schema declarations and policy rules |
| go-architect | Validates Go patterns in shard implementations |
| log-analyzer | Traces wiring failures through logs |
| charm-tui | Audits TUI command and view wiring |
| research-builder | Validates ResearcherShard knowledge storage wiring |

### The 39 Integration Systems

```text
Kernel Layer (9):     Schema, Policy, Facts, Query, Virtual Predicates,
                      FactStore, Rule Compilation, Trace, Activation
Shard Layer (8):      Type A/B/U/S Registration, Injection, Spawning,
                      Messaging, Lifecycle
Memory Layer (4):     RAM, Vector, Graph, Cold Storage
Executive Layer (6):  Perception, WorldModel, Executive, Constitution,
                      Legislator, Router
I/O Layer (6):        CLI, Transducer, Articulation, VirtualStore, TUI, Session
Orchestration (4):    Campaign, TDD Loop, Autopoiesis, Piggyback
Cross-Cutting (2):    Logging (22 categories), Configuration
```

### Key Files Audited

```text
internal/shards/registration.go     - Shard factory registration
internal/core/defaults/schemas.mg   - Mangle schema declarations
internal/mangle/policy.gl           - Policy rules
internal/core/virtual_store.go      - Action handlers, virtual predicates
cmd/nerd/chat/commands.go           - CLI command handlers
internal/perception/transducer.go   - NL→atoms verb corpus
internal/system/factory.go          - Boot sequence (BootCortex)
```

---

## SK-010: cli-engine-integration

**Name:** `cli-engine-integration`

**Domain:** LLM Integration

**Description:** Integrate Claude Code CLI and OpenAI Codex CLI as subscription-based LLM backends for codeNERD. This skill should be used when implementing CLI subprocess clients, configuring engine authentication, extending the LLMClient interface for CLI backends, or troubleshooting CLI-based LLM integration issues.

### Trigger Conditions

- Implementing CLI subprocess clients for LLM backends
- Configuring engine selection (API vs Claude CLI vs Codex CLI)
- Setting up `nerd auth claude` or `nerd auth codex` authentication
- Troubleshooting CLI-based LLM integration issues
- Extending LLMClient interface for new CLI backends
- Configuring model selection for CLI engines
- Handling rate limits from subscription-based backends

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Claude CLI Client | Subprocess execution via `claude -p --output-format json` |
| Codex CLI Client | Subprocess execution via `codex exec - --json` |
| Engine Configuration | `.nerd/config.json` engine field routing |
| Model Selection | Claude: sonnet/opus/haiku, Codex: gpt-5/o4-mini/o3 |
| Rate Limit Handling | Error detection with user-facing switch prompts |
| Authentication | `nerd auth claude/codex/status` CLI commands |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `references/claude-cli-client.md` | Reference | Claude CLI Go implementation, JSON parsing |
| `references/codex-cli-client.md` | Reference | Codex CLI Go implementation, NDJSON parsing |
| `references/config-schema.md` | Reference | Extended config schema with engine fields |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| codenerd-builder | Extends LLMClient interface in perception layer |
| go-architect | CLI clients must follow subprocess safety patterns |
| charm-tui | `/config engine` command integration |

### Key Implementation Files

```text
internal/perception/claude_cli_client.go  - Claude CLI LLMClient implementation
internal/perception/codex_cli_client.go   - Codex CLI LLMClient implementation
internal/perception/client.go             - Client factory with engine routing
internal/config/config.go                 - Config schema with Engine fields
cmd/nerd/main.go                          - nerd auth CLI commands
cmd/nerd/chat/commands.go                 - /config engine command
```

### Configuration Example

```json
{
  "engine": "claude-cli",
  "claude_cli": {
    "model": "sonnet",
    "timeout": 300
  },
  "codex_cli": {
    "model": "gpt-5",
    "sandbox": "read-only",
    "timeout": 300
  }
}
```

### Engine Selection Priority

1. Check `engine` field in `.nerd/config.json`
2. If `"claude-cli"` → create `ClaudeCodeCLIClient`
3. If `"codex-cli"` → create `CodexCLIClient`
4. If `"api"` or empty → use existing provider-based selection

---

---

## SK-011: prompt-architect

**Name:** `prompt-architect`

**Domain:** Prompt Engineering

**Description:** Master prompt engineering for codeNERD's neuro-symbolic architecture. Use when writing new shard prompts, auditing existing prompts, debugging LLM behavior, or optimizing context injection. Covers static prompts, dynamic injection, JIT prompt compilation, Piggybacking protocol, tool steering, and specialist knowledge hydration.

### Trigger Conditions

- Writing new shard prompts (Type A/B/U/S)
- Auditing prompts for Piggyback Protocol compliance
- Debugging LLM behavior (hallucinations, ignored tools)
- Tuning Context Injection and Spreading Activation
- Creating Type B/U Specialist Agents
- Implementing or debugging JIT prompt compilation
- Managing prompt atoms in unified storage

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Prompt Anatomy | Static vs Dynamic, Piggyback Envelope, Reasoning Trace |
| JIT Compilation | Dynamic prompt assembly from atomic units with Mangle-based linking |
| Atomic Prompt Design | Breaking God Tier prompts into composable atoms |
| Unified Storage | Prompt atoms stored in agent knowledge DBs alongside domain knowledge |
| Dependency Resolution | Auto-inject safety constraints when tools are selected |
| Context-Aware Assembly | Phase gating, language/framework selection, token budgeting |
| Tool Steering | Deterministic binding of intent to kernel-granted tools |
| Context Injection | 15+ SessionContext fields, Spreading Activation logic |
| Specialist Hydration | Injecting KnowledgeAtoms, Viva Voce validation |
| Auditing | Structural (JSON schema) and Semantic (Steering) verification |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `SKILL.md` | Core | Architecture, patterns, quick-start, JIT compilation |
| `references/prompt-anatomy.md` | Reference | Dual-layer prompt structure details |
| `references/context-injection.md` | Reference | SessionContext and Spreading Activation |
| `references/tool-steering.md` | Reference | Writing kernel-compliant tool definitions |
| `references/specialist-prompts.md` | Reference | Creating God Tier specialist agents |
| `references/god-tier-templates.md` | Reference | Full 20,000+ character production templates |
| `scripts/audit_prompts.py` | Script | Automated prompt compliance checker |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| codenerd-builder | Implements JIT prompt compilation as core system component |
| mangle-programming | Defines linking rules for atom selection and dependency resolution |
| research-builder | Provides KnowledgeAtoms for injection |
| go-architect | Go implementation patterns for prompt system |

### Key Implementation Files

```text
internal/prompt/compiler.go                - JIT prompt compiler
internal/prompt/selector.go                - Atom selection (Vector + Mangle)
internal/prompt/resolver.go                - Dependency resolution
internal/prompt/budget.go                  - Token budget management
internal/prompt/loader.go                  - YAML→SQLite prompt loading
internal/prompt/atoms.go                   - Atom type definitions
internal/articulation/prompt_assembler.go  - Shard integration
build/prompt_atoms/**/*.yaml               - 50+ atomic prompt sources
.nerd/agents/{name}/prompts.yaml           - Human-editable agent prompts
.nerd/shards/{name}_knowledge.db           - Unified storage (knowledge + prompts)
```

---

## SK-012: stress-tester

**Name:** `stress-tester`

**Domain:** Testing & Quality Assurance

**Description:** Live stress testing of codeNERD via CLI. Use when testing system stability, finding panics, edge cases, and failure modes across all 25+ subsystems. Includes comprehensive multi-minute workflows with conservative, aggressive, chaos, and hybrid severity levels.

### Trigger Conditions

- Pre-release stability verification
- After major architectural changes
- Debugging intermittent failures
- Validating resource limits
- Finding panic vectors
- Testing system recovery after failures

### Key Capabilities

| Capability | Description |
|------------|-------------|
| 27 Workflow Tests | 8 categories covering all subsystems |
| 4 Severity Levels | Conservative, Aggressive, Chaos, Hybrid |
| Log Analysis | Integrated with log-analyzer for post-test Mangle queries |
| Panic Detection | Custom predicates for detecting failures |
| Fixture Generation | Synthetic projects, malformed inputs, cyclic rules |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `SKILL.md` | Core | Quick start, workflow catalog, severity guide |
| `references/subsystem-stress-points.md` | Reference | All 25+ subsystems with failure modes |
| `references/panic-catalog.md` | Reference | Known panic vectors with triggers |
| `references/resource-limits.md` | Reference | Config limits and safe/dangerous values |
| `references/workflows/**/*.md` | Reference | 27 stress test workflows |
| `scripts/analyze_stress_logs.py` | Script | Post-test log analysis |
| `scripts/fixtures/generate_large_project.py` | Script | Synthetic Go project generator |
| `scripts/fixtures/malformed_inputs.py` | Script | Fuzzing payload generator |
| `assets/cyclic_rules.mg` | Asset | Mangle rules for derivation explosion |
| `assets/stress_queries.mg` | Asset | Log analysis Mangle queries |
| `assets/malformed_piggyback.json` | Asset | Invalid JSON test cases |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| log-analyzer | Uses log-analyzer for post-test Mangle queries |
| mangle-programming | Stress tests use Mangle rules and queries |
| codenerd-builder | Tests all codeNERD subsystems |
| integration-auditor | Complements auditor with runtime verification |

### Workflow Categories

```text
01-kernel-core/           - Mangle engine, SpawnQueue, memory
02-perception-articulation/ - NL parsing, Piggyback protocol
03-shards-campaigns/       - Shard lifecycle, campaigns, TDD
04-autopoiesis-ouroboros/ - Tool generation, adversarial testing
05-world-context/         - Filesystem, context compression
06-advanced-features/     - Dream state, shadow mode, browser
07-full-system-chaos/     - System-wide stability tests
08-hybrid-integration/    - Cross-subsystem integration
```

---

## SK-013: gemini-features

**Name:** `gemini-features`

**Domain:** LLM Integration (Gemini-Specific)

**Description:** Master Google Gemini 3 API integration for codeNERD. This skill should be used when implementing Gemini-specific features including: thinking mode configuration (thinkingLevel vs thinkingBudget), thought signatures for multi-turn function calling, Google Search grounding, URL Context tool, document processing, and structured output. Covers Go implementation patterns, API request/response formats, and integration with codeNERD's perception layer.

### Trigger Conditions

- Writing or debugging Gemini client code (`client_gemini.go`)
- Configuring thinking modes (thinkingLevel for Gemini 3, thinkingBudget for 2.5)
- Implementing multi-turn function calling with thought signatures
- Enabling Google Search grounding or URL Context tools
- Processing PDF/image documents with Gemini
- Troubleshooting Gemini API errors (400 signature errors, etc.)
- Deciding between Deep Research Agent vs URL Context/Search

### Key Capabilities

| Capability | Description |
|------------|-------------|
| Thinking Mode | Configure thinkingLevel (minimal/low/medium/high) for Gemini 3 |
| Thought Signatures | Handle encrypted reasoning context for multi-turn calls |
| Google Search | Enable real-time web grounding with citation metadata |
| URL Context | Ground responses with up to 20 specific URLs (34MB each) |
| Document Processing | Process PDFs, images, and text with inline/GCS upload |
| API Types | Complete Go type definitions for request/response |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| `SKILL.md` | Core | Quick reference, patterns, common pitfalls |
| `references/api_reference.md` | Reference | Complete API type definitions |
| `references/thinking-mode.md` | Reference | Thinking configuration (levels vs budget) |
| `references/grounding-tools.md` | Reference | Google Search and URL Context |
| `references/thought-signatures.md` | Reference | Multi-turn function calling |
| `references/document-processing.md` | Reference | PDF and image handling |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| codenerd-builder | Implements GeminiClient as LLM provider |
| go-architect | Go patterns for client implementation |
| research-builder | Gemini grounding tools for ResearcherShard |
| cli-engine-integration | Alternative to CLI engines for API-based LLM |

### Key Implementation Files

```text
internal/perception/client_gemini.go     - Gemini client implementation
internal/perception/client_types.go      - Gemini type definitions
internal/perception/client_factory.go    - Provider wiring
internal/config/llm.go                   - GeminiProviderConfig
.nerd/config.json                        - User configuration
```

### Decision: Deep Research vs Built-in Tools

**Deep Research Agent** (`/interactions` endpoint):
- Async, up to 60 minutes execution
- Cannot use codeNERD tools (no custom function calling)
- Different API model

**Recommendation:** Use Google Search + URL Context instead:
- Synchronous, fits codeNERD flow
- Works with codeNERD's tool ecosystem
- Grounding metadata for citations

---

## Adding New Skills to the Registry

To register a new skill:

### 1. Choose the Next Identifier

Identifiers follow the pattern `SK-XXX` where XXX is a zero-padded sequential number. Check the Registry Index table for the next available ID.

### 2. Create the Registry Entry

Copy this template and fill in all sections:

```markdown
## SK-XXX: skill-name

**Name:** `skill-name`

**Domain:** [Category]

**Description:** [Copy from SKILL.md frontmatter]

### Trigger Conditions

- [When should this skill be used?]
- [What user requests trigger it?]

### Key Capabilities

| Capability | Description |
|------------|-------------|
| [Feature] | [What it does] |

### Bundled Resources

| Resource | Type | Purpose |
|----------|------|---------|
| [path] | [Script/Reference/Asset] | [Purpose] |

### Integration Points

| Integrates With | Relationship |
|-----------------|--------------|
| [skill-name] | [How they work together] |
```

### 3. Update the Registry Index

Add a row to the Registry Index table at the top of this file.

### 4. Update codenerd-builder SKILL.md

Add the new skill to:

- The "Skill Map" ASCII diagram (if applicable)
- The "When to Use Each Skill" table
- The "Related Skill Documentation" links section

---

## Registry Maintenance

### Version History

| Date | Change | Author |
|------|--------|--------|
| 2025-12-08 | Initial registry creation | Claude |
| 2025-12-29 | Added SK-013 gemini-features | Claude |

### Guidelines

1. **Keep entries current** - Update when skills change
2. **Document integration points** - Cross-skill relationships matter
3. **Include trigger conditions** - Help Claude know when to use each skill
4. **List all bundled resources** - Make discovery easy
