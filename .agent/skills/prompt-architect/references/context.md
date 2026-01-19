he skill-creator skill is now loaded. I can help you with:
Creating a new skill - Following the 7-step process from understanding use cases through packaging
Updating an existing skill - Iterating on SKILL.md or bundled resources
Packaging a skill - Validating and creating a distributable zip file
Registering a codeNERD skill - Adding to the skill registry
What would you like to do?
okay, i need a skill for the art of prompt engineering... specifically tailored to our system where some parts of the prompt are static, and some are dynamic, and we have piggybacking, see C:\CodeProjects\codeNERD\.claude\skills\codenerd-builder\SKILL.md... we need a prompt engineering skill that will enable deterministic tool use more effectively, steer the model more effectively per task, and we have a lot more context to apply to system instructions and descriptions because of our context compression system, so we want to ensure that these shard agents and main agent know what the fuck they are doing and are god tier specialists... the combination of our dyamic mangle prompt "injection" or whatever we call it, and the static prompts have more legroom to be more effective, so research and make a plan for creating skill for creating prompts and auditing them for this codebase, ultrathink
I'll explore the codebase to understand your prompt architecture before designing this skill. Let me investigate the static/dynamic prompt system, piggybacking protocol, and shard prompt configuration.
Read c:\CodeProjects\codeNERD\.claude\skills\codenerd-builder\SKILL.md
Task:Explore static prompt locations
IN
Explore the codeNERD codebase to find where static prompts are defined. Look for:

1. System prompts, shard prompts, constitution prompts
2. Files in internal/shards/ that define prompt templates
3. Any .txt, .md, or embedded string prompts
4. Prompt constants or template files
5. How different shards (CoderShard, TesterShard, ReviewerShard, etc.) define their specialized instructions

Search patterns:

- "system prompt", "SystemPrompt", "systemPrompt"
- "instruction", "Instructions"
- Heredocs or multiline strings in Go files
- Any prompt-related configuration

Return: File paths, key prompt structures, and how prompts are organized.
Task:Explore piggybacking/articulation
IN
Explore the codeNERD piggybacking protocol and articulation system. Focus on:

1. internal/articulation/emitter.go - How Mangle atoms become NL output
2. How the dual-channel output works (surface for user, control for kernel)
3. How dynamic Mangle facts/atoms are injected into LLM prompts
4. The perception/transducer system that converts NL to atoms
5. Any "prompt injection" or context building from Mangle

Look for:

- Piggyback*, ControlChannel, SurfaceChannel patterns
- How context_atom/1 facts are selected and formatted for LLM
- Template rendering or string interpolation for dynamic content
- The flow from Mangle facts → LLM prompt construction

Return: The architecture of dynamic prompt injection, key files, and the piggybacking protocol implementation.
Task:Explore shard/tool configuration
IN
Explore how shards and tools are configured in codeNERD. Focus on:

1. How ShardManager configures different shard types
2. Tool definitions and descriptions - how they're written to steer model behavior
3. Any shard-specific prompts or configurations in internal/shards/
4. The constitution/safety gate prompts
5. How "specialist" knowledge is loaded into shards

Look for:

- Tool schemas, function calling definitions
- Shard initialization with custom prompts
- Constitution.go or safety-related prompts
- Any prompt templates or prompt builders
- How Type B (Persistent) shards get their specialized knowledge

Return: How shards are configured with prompts, tool definitions, and specialist knowledge injection patterns.
I've launched 3 explore agents. Let me also read key reference documents for the piggybacking protocol and examine any existing prompt patterns while they run.
Read c:\CodeProjects\codeNERD\.claude\skills\codenerd-builder\references\piggyback-protocol.md, c:\CodeProjects\codeNERD\.claude\skills\codenerd-builder\references\shard-agents.md
Grep "SystemPrompt|system prompt|system_prompt|Instructions|CRITICAL" (in c:\CodeProjects\codeNERD\internal, glob: *.go)
34 lines of output
Excellent, I found key files. Let me read specific prompt implementations while waiting for agents.
Read c:\CodeProjects\codeNERD\internal\shards\system\constitution.go, c:\CodeProjects\codeNERD\internal\shards\coder\generation.go
Read c:\CodeProjects\codeNERD\internal\shards\system\planner.go
Let me check on the explore agents and read the articulation/emitter file to understand output format steering.
AgentOutputTool
AgentOutputTool
AgentOutputTool
Read c:\CodeProjects\codeNERD\internal\articulation\emitter.go
I have a comprehensive picture now. Let me wait for the explore agents to complete and then synthesize everything.
AgentOutputTool
AgentOutputTool
AgentOutputTool
I now have comprehensive understanding of the prompt architecture. Let me write the initial plan.
Write C:\Users\smoor\.claude\plans\generic-orbiting-tulip.md
185 lines

# Prompt Engineering Skill for codeNERD

## Overview

Create a specialized skill (`prompt-architect`) for designing, writing, and auditing prompts within the codeNERD neuro-symbolic architecture. The skill addresses the unique constraints of a system with:

- **Static prompts** (Go const strings in shard files)
- **Dynamic injection** (Mangle-derived context_atoms, SessionContext)
- **Piggybacking protocol** (dual-channel surface + control output)
- **Context compression** (>100:1 ratio enables richer prompts)

## User Requirements

1. Enable **deterministic tool use** more effectively
2. **Steer model behavior** per task/shard type
3. Create **"god tier specialists"** via rich context injection
4. Cover **static + dynamic prompt architecture**
5. Provide **prompt auditing** capability

## Key Findings from Exploration

### Current Prompt Architecture

| Component | Count | Location |
|-----------|-------|----------|
| Static System Prompts | 13+ | `internal/shards/system/*.go`, `internal/shards/coder/generation.go` |
| Dynamic Context Builders | 5+ | `buildSessionContextPrompt()`, `buildCodeDOMContext()` |
| Piggybacking Protocol | 1 | `internal/articulation/emitter.go` |
| Spreading Activation | 1 | `internal/core/defaults/policy.mg` |

### Critical Patterns Identified

1. **Thought-First Ordering** - control_packet before surface_response
2. **Artifact Classification** - project_code vs self_tool vs diagnostic
3. **Session Context Injection** - 15+ context categories (diagnostics, test state, git, campaign, etc.)
4. **Spreading Activation** - context_atom(Fact) :- activation(Fact, Score), Score > 30
5. **Constitutional Override** - kernel can block/rewrite LLM output

## Skill Structure

```
.claude/skills/prompt-architect/
├── SKILL.md                     # Main skill instructions
├── references/
│   ├── prompt-anatomy.md        # Static vs dynamic, piggybacking, output schemas
│   ├── tool-steering.md         # Tool descriptions, affinity, Mangle predicates
│   ├── context-injection.md     # SessionContext, spreading activation, compression
│   ├── shard-prompts.md         # Per-shard prompt patterns (coder, tester, reviewer, etc.)
│   ├── audit-checklist.md       # Prompt quality checklist with examples
│   └── anti-patterns.md         # Common failures and fixes
└── scripts/
    └── audit_prompts.py         # Scan Go files for prompt patterns, validate structure
```

## Implementation Plan

### Phase 1: Create Core SKILL.md

Define when/how to use the skill:

- Trigger conditions (writing new shards, auditing prompts, debugging LLM behavior)
- Core concepts overview
- Reference to bundled resources
- Quick-start patterns

### Phase 2: Create Reference Documents

#### 2.1 prompt-anatomy.md

- Static prompt structure (Go const patterns)
- Dynamic injection points (SessionContext, CodeDOM, context_atom)
- Piggybacking protocol (PiggybackEnvelope schema, thought-first ordering)
- Output format steering (JSON schemas, artifact classification)
- Reasoning trace directives

#### 2.2 tool-steering.md

- Tool definition structure (Name, Description, ShardAffinity, Capabilities)
- Mangle predicates for tool routing (relevant_tool, tool_capability)
- Deterministic tool selection patterns
- Tool description writing guidelines

#### 2.3 context-injection.md

- SessionContext fields (15+ categories)
- Spreading activation rules
- context_atom derivation
- Token budget allocation
- Compression patterns

#### 2.4 shard-prompts.md

- Per-shard prompt templates:
  - CoderShard (artifact classification, language detection)
  - TesterShard (framework detection, coverage)
  - ReviewerShard (12-point holistic review)
  - System shards (autopoiesis patterns)
- Model capability matching

#### 2.5 audit-checklist.md

- Structural checks (JSON output, thought-first, artifact type)
- Safety checks (constitutional compliance, dangerous patterns)
- Effectiveness checks (context utilization, specificity)
- Common issues and fixes

#### 2.6 anti-patterns.md

- Premature articulation (surface before control)
- Context starvation (missing session context)
- Overly generic prompts
- Missing artifact classification
- Unsafe tool steering

### Phase 3: Create Audit Script

Python script to:

- Scan Go files for prompt const definitions
- Validate JSON output schemas
- Check for piggybacking compliance
- Report missing patterns
- Generate audit report

### Phase 4: Register in Skill Registry

Add to `.claude/skills/codenerd-builder/references/skill-registry.md`:

- SK-010: prompt-architect
- Trigger conditions, capabilities, integration points

## Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `.claude/skills/prompt-architect/SKILL.md` | Create | Main skill instructions |
| `.claude/skills/prompt-architect/references/prompt-anatomy.md` | Create | Core prompt structure |
| `.claude/skills/prompt-architect/references/tool-steering.md` | Create | Tool description patterns |
| `.claude/skills/prompt-architect/references/context-injection.md` | Create | Dynamic injection guide |
| `.claude/skills/prompt-architect/references/shard-prompts.md` | Create | Per-shard templates |
| `.claude/skills/prompt-architect/references/audit-checklist.md` | Create | Quality checklist |
| `.claude/skills/prompt-architect/references/anti-patterns.md` | Create | Failure modes |
| `.claude/skills/prompt-architect/scripts/audit_prompts.py` | Create | Automated audit |
| `.claude/skills/codenerd-builder/references/skill-registry.md` | Modify | Register SK-010 |
| `.claude/skills/codenerd-builder/SKILL.md` | Modify | Add to skill index |

## Key Prompt Patterns to Document

### 1. Deterministic Tool Use Pattern

```
TOOL SELECTION:
When you need to [action], use [tool_name] via execute_tool(tool_name, args).
Available tools for this task:
- tool_1: description (use for X)
- tool_2: description (use for Y)

DO NOT improvise tools. If no tool matches, emit missing_tool_for(intent, capability).
```

### 2. Artifact Classification Pattern

```
ARTIFACT CLASSIFICATION (MANDATORY):
- "project_code": Code for user's codebase (default)
- "self_tool": Tool for codeNERD internal use → Ouroboros
- "diagnostic": One-time inspection script

If creating something for YOUR OWN USE, set artifact_type to "self_tool".
```

### 3. Thought-First Pattern

```
CRITICAL: You MUST write control_packet BEFORE surface_response.
Do NOT speak until AFTER control_packet is written.
If generation fails mid-stream, the user sees nothing instead of false promises.
```

### 4. Context Utilization Pattern

```
CONTEXT PROVIDED (use this information):
- CURRENT DIAGNOSTICS: [errors to fix]
- TEST STATE: [failing tests]
- RECENT FINDINGS: [from reviewer/tester]
- IMPACTED FILES: [transitive dependencies]
- GIT CONTEXT: [why code exists - Chesterton's Fence]
```

## Questions for User

1. **Scope priority**: Should the skill focus more on (a) writing new shard prompts, (b) auditing existing prompts, or (c) both equally?

2. **Audit depth**: Should the audit script check for semantic issues (e.g., missing context categories) or just structural issues (e.g., JSON schema compliance)?

3. **Examples**: Should references include full working examples from the codebase or synthesized examples?
I've drafted an initial plan based on comprehensive exploration of your prompt architecture. I found: 13+ static prompts across shards (coder, tester, reviewer, system shards) 5+ dynamic injection points (SessionContext with 15+ categories, spreading activation, CodeDOM) Piggybacking protocol with thought-first ordering (Bug #14 fix) Tool steering via descriptions, affinity, and Mangle predicates Before I finalize the design, I have a few questions to ensure the skill matches your needs:
Scope priority: Should the skill emphasize:
(a) Writing new shard prompts from scratch
(b) Auditing/improving existing prompts
(c) Both equally
Audit automation: For the audit script, should it:
(a) Just structural validation (JSON schema, piggybacking compliance)
(b) Semantic analysis (missing context categories, weak tool steering)
(c) Both with recommendations
Examples: Should references include:
(a) Real examples extracted from current codebase (constitution.go, generation.go, etc.)
(b) Synthesized "ideal" examples showing best practices
(c) Both side-by-side (current → improved)
Specialist focus: You mentioned "god tier specialists" - should there be a dedicated section on:
Writing prompts for Type B/U persistent specialists
Knowledge atom injection patterns
Domain expertise hydration strategies
1. C... both... 2. C.... 3... C.... 4.... all 3... lol ... ultrathink
Got it - maximum coverage across the board. Let me check the integration-auditor pattern you opened and then launch a Plan agent to design the comprehensive architecture.
Read c:\CodeProjects\codeNERD\.claude\skills\integration-auditor\scripts\audit_actions.py
Task:Design prompt-architect skill
IN
Design a comprehensive "prompt-architect" skill for the codeNERD neuro-symbolic coding agent. This skill should be the definitive guide for prompt engineering within this unique architecture.

## CONTEXT FROM EXPLORATION

### Current Architecture

The codeNERD system has a unique prompt architecture:

1. **Static Prompts** (13+ Go const strings):
   - System shards: perception.go (perceptionSystemPrompt), constitution.go (constitutionAutopoiesisPrompt), executive.go, router.go, planner.go (plannerSystemPrompt, decompositionPrompt), world_model.go, legislator.go
   - Specialist shards: coder/generation.go (buildSystemPrompt), tester/generation.go, reviewer/llm.go, requirements_interrogator.go

2. **Dynamic Injection** (Mangle-driven):
   - SessionContext with 15+ categories: diagnostics, test state, findings, impact analysis, git state, campaign context, prior shard outputs, domain knowledge, available tools, safety constraints, compressed history
   - Spreading Activation: context_atom(Fact) :- activation(Fact, Score), Score > 30
   - CodeDOM queries: generated_code, breaking_change_risk, api_client_function

3. **Piggybacking Protocol** (dual-channel):
   - PiggybackEnvelope: { control_packet (FIRST), surface_response }
   - Thought-First Ordering (Bug #14 Fix): control before surface prevents premature articulation
   - Constitutional Override: kernel can block/rewrite unsafe output

4. **Tool Steering**:
   - Tool definitions: Name, Description, ShardAffinity, Capabilities
   - Mangle predicates: relevant_tool, tool_capability, tool_description
   - Deterministic routing via kernel queries

5. **Context Compression**:
   - >100:1 ratio via semantic compression
   - Token budgets: 128k total, 38.4k for atoms, threshold 30.0

### Key Files

- internal/articulation/emitter.go - Piggybacking protocol, JSON parsing
- internal/perception/transducer.go - NL→Intent, VerbCorpus
- internal/shards/coder/generation.go - CoderShard prompts, session context
- internal/shards/system/constitution.go - Safety gate, autopoiesis
- internal/core/defaults/policy.mg - Spreading activation rules
- cmd/nerd/chat/helpers.go - Dynamic prompt building

## USER REQUIREMENTS

1. **Scope**: Both writing NEW prompts AND auditing EXISTING prompts
2. **Audit**: Both structural (JSON, piggybacking) AND semantic (context utilization, tool steering)
3. **Examples**: Both REAL codebase examples AND synthesized "ideal" patterns (side-by-side current→improved)
4. **Specialist Focus**:
   - Type B/U persistent specialist prompts
   - Knowledge atom injection patterns
   - Domain expertise hydration strategies

## DESIGN TASK

Create a detailed design for the prompt-architect skill including:

1. **SKILL.md Structure**:
   - Frontmatter (name, description with trigger conditions)
   - When to use (writing new shards, auditing, debugging LLM behavior)
   - Core concepts overview
   - Quick-start patterns
   - Reference navigation

2. **Reference Documents** (detailed outline for each):
   - prompt-anatomy.md: Static vs dynamic, piggybacking, output schemas, reasoning traces
   - tool-steering.md: Descriptions, affinity, Mangle predicates, deterministic selection
   - context-injection.md: SessionContext fields, spreading activation, token budgets
   - shard-prompts.md: Per-shard templates with real examples
   - specialist-prompts.md: Type B/U patterns, knowledge hydration, domain expertise
   - audit-checklist.md: Structural + semantic checks with examples
   - anti-patterns.md: Common failures with fixes

3. **Audit Script Design**:
   - What to scan (Go files, const patterns)
   - Structural checks (JSON schema, piggybacking compliance, artifact classification)
   - Semantic checks (context utilization, tool steering quality, specificity)
   - Report format
   - Integration with existing audit patterns

4. **Key Patterns to Document**:
   - Deterministic tool use
   - Artifact classification
   - Thought-first ordering
   - Context utilization
   - Constitutional compliance
   - Specialist knowledge injection
   - Autopoiesis rule proposal

Provide a comprehensive design that would make this the definitive prompt engineering guide for codeNERD development.
Good, I have the audit pattern. Let me also check the integration-auditor skill structure while the Plan agent works.
Read c:\CodeProjects\codeNERD\.claude\skills\integration-auditor\SKILL.md
Globpattern: "**/.claude/skills/integration-auditor/scripts/*.py"
Found 5 files
I now understand the audit pattern well - they have multiple specialized audit scripts. Let me check on the Plan agent and look at one more audit script for the pattern.
Read c:\CodeProjects\codeNERD\.claude\skills\integration-auditor\scripts\audit_wiring.py
AgentOutputTool
The Plan agent is still working on the comprehensive design. Let me wait for it to complete.
AgentOutputTool
Excellent! The Plan agent returned a comprehensive design. Let me update the plan file with the final detailed implementation plan.
Write C:\Users\smoor\.claude\plans\generic-orbiting-tulip.md
307 lines

# Prompt Architect Skill - Implementation Plan

## Overview

Create `prompt-architect` - the definitive prompt engineering skill for codeNERD's neuro-symbolic architecture. Covers static prompts, dynamic Mangle-driven injection, Piggybacking protocol, tool steering, and specialist knowledge hydration.

## Skill Structure

```
.claude/skills/prompt-architect/
├── SKILL.md                              # Main skill (triggers, concepts, quick-start)
├── references/
│   ├── prompt-anatomy.md                 # Static vs dynamic, piggybacking, output schemas
│   ├── tool-steering.md                  # Descriptions, affinity, Mangle predicates
│   ├── context-injection.md              # SessionContext fields, spreading activation
│   ├── shard-prompts.md                  # Per-shard templates with real examples
│   ├── specialist-prompts.md             # Type B/U patterns, knowledge hydration
│   ├── audit-checklist.md                # Structural + semantic checks
│   └── anti-patterns.md                  # Common failures with fixes
└── scripts/
    └── audit_prompts.py                  # Automated prompt auditor
```

## Files to Create

### 1. SKILL.md (~300 lines)

**Frontmatter:**

```yaml
name: prompt-architect
description: Master prompt engineering for codeNERD's neuro-symbolic architecture. Use when writing new shard prompts, auditing existing prompts, debugging LLM behavior, or optimizing context injection. Covers static prompts, dynamic injection, Piggybacking protocol, tool steering, and specialist knowledge hydration.
```

**Sections:**

- Purpose (why codeNERD prompts are different)
- When to use (writing, auditing, optimizing)
- Core concepts table (7 concepts with locations)
- Quick-start patterns (4 patterns)
- Reference navigation table

### 2. references/prompt-anatomy.md (~600 lines)

**Content:**

1. The Dual-Layer Architecture (static + dynamic)
2. The Piggyback Output Schema (JSON, thought-first ordering)
3. Reasoning Trace Directive (full vs short)
4. System Prompt Structure (standard sections)
5. Output Schema Patterns (code, review, test)
6. Prompt Composition Flow diagram

**Real Examples From:**

- `internal/shards/coder/generation.go:64` - buildSystemPrompt()
- `internal/perception/transducer.go:608` - transducerSystemPrompt
- `internal/articulation/emitter.go:524` - ReasoningTraceDirective

### 3. references/tool-steering.md (~400 lines)

**Content:**

1. Why Deterministic Tool Selection (kernel decides, not LLM)
2. Tool Definition Anatomy (ToolDefinition struct)
3. Writing Effective Descriptions (AIME framework)
4. Mangle Predicates (tool_capability, relevant_tool)
5. Adding a New Tool (4-step process)
6. Debugging Tool Selection (activation queries)

**Patterns:**

- Action + Target + Constraint descriptions
- Capability tag taxonomy
- Disambiguation clauses

### 4. references/context-injection.md (~500 lines)

**Content:**

1. The Context Pipeline (diagram)
2. SessionContext Structure (15+ categories)
3. Priority Ordering (13 priorities)
4. Spreading Activation (policy.mg Section 1)
5. Predicate Weights (from activation.go)
6. Token Budgets (128k total, 38.4k atoms)
7. Adding a New Context Category (4 steps)

**Real Examples From:**

- `internal/core/shard_manager.go:361-505` - SessionContext struct
- `internal/core/defaults/policy.mg:6-32` - activation rules
- `internal/context/activation.go` - predicate weights

### 5. references/shard-prompts.md (~700 lines)

**Content:**

1. CoderShard (generation.go) - with annotations
2. TransducerShard (transducer.go) - 70+ canonical examples
3. ConstitutionShard (constitution.go) - safety prompts
4. PlannerShard (planner.go) - decomposition
5. ReviewerShard (llm.go) - 12-point review
6. TesterShard (generation.go) - TDD awareness
7. New Shard Template (minimal + checklist)

**Side-by-Side:**

- Current prompt → Improved version (where applicable)

### 6. references/specialist-prompts.md (~500 lines)

**Content:**

1. Type B/U Specialist Architecture
2. Lifecycle: Research → Store → Spawn → Inject
3. KnowledgeAtom Structure
4. Injection Pattern (from buildSessionContextPrompt)
5. Confidence Thresholds table
6. Domain Expertise Hydration Strategies:
   - Known Source Priority
   - llms.txt First
   - Context7-Style Enrichment
7. Specialist Prompt Template
8. Viva Voce Examination pattern
9. Common Specialist Patterns:
   - Anti-Pattern Injection
   - Code Example Injection

### 7. references/audit-checklist.md (~400 lines)

**Content:**

1. Quick Audit (5 minutes) - structural + semantic
2. Full Audit (30 minutes):
   - Piggyback Protocol Compliance
   - Context Utilization
   - Tool Steering Quality
   - Artifact Classification
   - Constitutional Compliance
   - Specialist Knowledge (Type B/U)
3. Audit Report Template
4. Automated Script Integration

**Checklists:**

- JSON schema completeness
- Thought-first ordering
- Context category coverage
- Tool description quality

### 8. references/anti-patterns.md (~400 lines)

**Content:**

1. Category 1: Structural Failures
   - Surface Before Control (Bug #14)
   - Missing Reasoning Trace
   - Hardcoded Tool Lists
2. Category 2: Semantic Failures
   - Ignored Context
   - Verb Guessing
   - Unclassified Artifacts
3. Category 3: Safety Failures
   - No Constitutional Acknowledgment
   - Bypass Instructions
4. Category 4: Specialist Failures
   - Empty Knowledge Injection
   - Low-Quality Knowledge

**Quick Reference Table:** Pattern → Fix

### 9. scripts/audit_prompts.py (~500 lines)

**Features:**

- Scan Go files for `*Prompt` const patterns
- Structural checks:
  - JSON schema completeness
  - Piggyback field presence/ordering
  - Reasoning directive presence
  - Artifact classification rules
- Semantic checks:
  - SessionContext usage
  - Context injection patterns
  - Tool steering quality
  - Specificity score

**Output:**

- Console report (colored, verbose option)
- JSON output (for tooling)
- Markdown report

**Integration:**

- Follow `integration-auditor/scripts/audit_*.py` pattern
- Severity: ERROR, WARN, INFO, OK
- Exit codes: 0 (pass), 1 (errors)

## Key Patterns to Document

### 1. Deterministic Tool Use

```
AVAILABLE TOOLS (kernel-selected):
- tool_name: description

You MUST use one of these tools. Do not invent tools.
If no tool matches, emit missing_tool_for(intent, capability).
```

### 2. Artifact Classification

```
ARTIFACT CLASSIFICATION (MANDATORY):
- "project_code": Code for user's codebase (default)
- "self_tool": Tool for codeNERD internal use → Ouroboros
- "diagnostic": One-time inspection script
```

### 3. Thought-First Ordering

```json
{
  "control_packet": { ... },  // MUST be first
  "surface_response": "..."   // After control completes
}
```

### 4. Context Utilization

```
CURRENT DIAGNOSTICS (must address these FIRST):
  - error at line 42

You MUST address the diagnostics above before other changes.
```

### 5. Constitutional Compliance

```
Note: The kernel may block certain actions. If blocked,
explain to user and suggest alternatives.
```

### 6. Specialist Knowledge Injection

```
DOMAIN KNOWLEDGE:
- [Title]: [Content]
  Example: [CodePattern]
- HINT: [SpecialistHint]
```

### 7. Autopoiesis Rule Proposal

```json
{
  "memory_operations": [{
    "op": "promote_to_long_term",
    "key": "user_preference:X",
    "value": "Y"
  }]
}
```

## Critical Files to Reference

| File | What to Extract |
|------|-----------------|
| `internal/shards/coder/generation.go` | buildSystemPrompt(), buildSessionContextPrompt() |
| `internal/perception/transducer.go` | transducerSystemPrompt (614 lines), VerbCorpus |
| `internal/articulation/emitter.go` | PiggybackEnvelope, ReasoningTraceDirective |
| `internal/core/shard_manager.go` | SessionContext struct (15+ categories) |
| `internal/core/defaults/policy.mg` | Spreading activation rules (Section 1) |
| `internal/shards/system/constitution.go` | Safety gate prompts |
| `internal/shards/system/planner.go` | plannerSystemPrompt, decompositionPrompt |
| `.claude/skills/integration-auditor/` | Pattern for skill structure and audit scripts |

## Registry Updates

### skill-registry.md

Add SK-010 entry for prompt-architect with:

- Trigger conditions
- Capabilities table
- Integration points (mangle-programming, codenerd-builder, go-architect)

### codenerd-builder/SKILL.md

Add to:

- Skill Index table
- When to Use Each Skill
- Related Skill Documentation links

## Implementation Order

1. **SKILL.md** - Core skill file with frontmatter and quick-start
2. **prompt-anatomy.md** - Foundation document
3. **context-injection.md** - Critical for understanding dynamic prompts
4. **shard-prompts.md** - Real examples with annotations
5. **tool-steering.md** - Deterministic tool patterns
6. **specialist-prompts.md** - Type B/U knowledge hydration
7. **anti-patterns.md** - What to avoid
8. **audit-checklist.md** - Verification procedures
9. **audit_prompts.py** - Automated auditor
10. **Registry updates** - skill-registry.md, codenerd-builder SKILL.md

## Estimated Size

| File | Lines |
|------|-------|
| SKILL.md | ~300 |
| prompt-anatomy.md | ~600 |
| tool-steering.md | ~400 |
| context-injection.md | ~500 |
| shard-prompts.md | ~700 |
| specialist-prompts.md | ~500 |
| audit-checklist.md | ~400 |
| anti-patterns.md | ~400 |
| audit_prompts.py | ~500 |
| **Total** | **~4,300** |

## Success Criteria

1. All 7 reference documents cover their topics comprehensively
2. Real code examples extracted from codebase (not synthesized only)
3. Side-by-side current→improved patterns where applicable
4. Audit script catches structural and semantic issues
5. Skill registered in skill-registry.md
6. Can write a new shard prompt using only this skill as reference
The plan is complete and comprehensive. The markdown linting warnings are cosmetic issues in the plan file itself - they won't affect implementation. Summary of what will be created:
Component Description
SKILL.md Main skill with triggers, concepts, quick-start
7 Reference Docs prompt-anatomy, tool-steering, context-injection, shard-prompts, specialist-prompts, audit-checklist, anti-patterns
audit_prompts.py Automated auditor (structural + semantic checks)
Registry Updates SK-010 in skill-registry.md
Key patterns documented:
Thought-first ordering (Bug #14 fix)
Artifact classification (project_code/self_tool/diagnostic)
Deterministic tool use via Mangle
Context utilization (15+ categories)
Specialist knowledge hydration
Constitutional compliance
~4,300 lines total with real codebase examples + synthesized ideal patterns.
Claude’s Plan
Prompt Architect Skill - Implementation Plan
Overview
Create prompt-architect - the definitive prompt engineering skill for codeNERD's neuro-symbolic architecture. Covers static prompts, dynamic Mangle-driven injection, Piggybacking protocol, tool steering, and specialist knowledge hydration.
Skill Structure
.claude/skills/prompt-architect/
├── SKILL.md                              # Main skill (triggers, concepts, quick-start)
├── references/
│   ├── prompt-anatomy.md                 # Static vs dynamic, piggybacking, output schemas
│   ├── tool-steering.md                  # Descriptions, affinity, Mangle predicates
│   ├── context-injection.md              # SessionContext fields, spreading activation
│   ├── shard-prompts.md                  # Per-shard templates with real examples
│   ├── specialist-prompts.md             # Type B/U patterns, knowledge hydration
│   ├── audit-checklist.md                # Structural + semantic checks
│   └── anti-patterns.md                  # Common failures with fixes
└── scripts/
    └── audit_prompts.py                  # Automated prompt auditor
Files to Create
1. SKILL.md (~300 lines)
Frontmatter:
name: prompt-architect
description: Master prompt engineering for codeNERD's neuro-symbolic architecture. Use when writing new shard prompts, auditing existing prompts, debugging LLM behavior, or optimizing context injection. Covers static prompts, dynamic injection, Piggybacking protocol, tool steering, and specialist knowledge hydration.
Sections:
Purpose (why codeNERD prompts are different)
When to use (writing, auditing, optimizing)
Core concepts table (7 concepts with locations)
Quick-start patterns (4 patterns)
Reference navigation table
2. references/prompt-anatomy.md (~600 lines)
Content:
The Dual-Layer Architecture (static + dynamic)
The Piggyback Output Schema (JSON, thought-first ordering)
Reasoning Trace Directive (full vs short)
System Prompt Structure (standard sections)
Output Schema Patterns (code, review, test)
Prompt Composition Flow diagram
Real Examples From:
internal/shards/coder/generation.go:64 - buildSystemPrompt()
internal/perception/transducer.go:608 - transducerSystemPrompt
internal/articulation/emitter.go:524 - ReasoningTraceDirective
3. references/tool-steering.md (~400 lines)
Content:
Why Deterministic Tool Selection (kernel decides, not LLM)
Tool Definition Anatomy (ToolDefinition struct)
Writing Effective Descriptions (AIME framework)
Mangle Predicates (tool_capability, relevant_tool)
Adding a New Tool (4-step process)
Debugging Tool Selection (activation queries)
Patterns:
Action + Target + Constraint descriptions
Capability tag taxonomy
Disambiguation clauses
4. references/context-injection.md (~500 lines)
Content:
The Context Pipeline (diagram)
SessionContext Structure (15+ categories)
Priority Ordering (13 priorities)
Spreading Activation (policy.mg Section 1)
Predicate Weights (from activation.go)
Token Budgets (128k total, 38.4k atoms)
Adding a New Context Category (4 steps)
Real Examples From:
internal/core/shard_manager.go:361-505 - SessionContext struct
internal/core/defaults/policy.mg:6-32 - activation rules
internal/context/activation.go - predicate weights
5. references/shard-prompts.md (~700 lines)
Content:
CoderShard (generation.go) - with annotations
TransducerShard (transducer.go) - 70+ canonical examples
ConstitutionShard (constitution.go) - safety prompts
PlannerShard (planner.go) - decomposition
ReviewerShard (llm.go) - 12-point review
TesterShard (generation.go) - TDD awareness
New Shard Template (minimal + checklist)
Side-by-Side:
Current prompt → Improved version (where applicable)
6. references/specialist-prompts.md (~500 lines)
Content:
Type B/U Specialist Architecture
Lifecycle: Research → Store → Spawn → Inject
KnowledgeAtom Structure
Injection Pattern (from buildSessionContextPrompt)
Confidence Thresholds table
Domain Expertise Hydration Strategies:
Known Source Priority
llms.txt First
Context7-Style Enrichment
Specialist Prompt Template
Viva Voce Examination pattern
Common Specialist Patterns:
Anti-Pattern Injection
Code Example Injection
7. references/audit-checklist.md (~400 lines)
Content:
Quick Audit (5 minutes) - structural + semantic
Full Audit (30 minutes):
Piggyback Protocol Compliance
Context Utilization
Tool Steering Quality
Artifact Classification
Constitutional Compliance
Specialist Knowledge (Type B/U)
Audit Report Template
Automated Script Integration
Checklists:
JSON schema completeness
Thought-first ordering
Context category coverage
Tool description quality
8. references/anti-patterns.md (~400 lines)
Content:
Category 1: Structural Failures
Surface Before Control (Bug #14)
Missing Reasoning Trace
Hardcoded Tool Lists
Category 2: Semantic Failures
Ignored Context
Verb Guessing
Unclassified Artifacts
Category 3: Safety Failures
No Constitutional Acknowledgment
Bypass Instructions
Category 4: Specialist Failures
Empty Knowledge Injection
Low-Quality Knowledge
Quick Reference Table: Pattern → Fix
9. scripts/audit_prompts.py (~500 lines)
Features:
Scan Go files for *Prompt const patterns
Structural checks:
JSON schema completeness
Piggyback field presence/ordering
Reasoning directive presence
Artifact classification rules
Semantic checks:
SessionContext usage
Context injection patterns
Tool steering quality
Specificity score
Output:
Console report (colored, verbose option)
JSON output (for tooling)
Markdown report
Integration:
Follow integration-auditor/scripts/audit_*.py pattern
Severity: ERROR, WARN, INFO, OK
Exit codes: 0 (pass), 1 (errors)
Key Patterns to Document
1. Deterministic Tool Use
AVAILABLE TOOLS (kernel-selected):

- tool_name: description

You MUST use one of these tools. Do not invent tools.
If no tool matches, emit missing_tool_for(intent, capability).
2. Artifact Classification
ARTIFACT CLASSIFICATION (MANDATORY):

- "project_code": Code for user's codebase (default)
- "self_tool": Tool for codeNERD internal use → Ouroboros
- "diagnostic": One-time inspection script

3. Thought-First Ordering
{
  "control_packet": { ... },  // MUST be first
  "surface_response": "..."   // After control completes
}
4. Context Utilization
CURRENT DIAGNOSTICS (must address these FIRST):

- error at line 42

You MUST address the diagnostics above before other changes.
5. Constitutional Compliance
Note: The kernel may block certain actions. If blocked,
explain to user and suggest alternatives.
6. Specialist Knowledge Injection
DOMAIN KNOWLEDGE:
  Example: [CodePattern]
- HINT: [SpecialistHint]

7. Autopoiesis Rule Proposal
{
  "memory_operations": [{
    "op": "promote_to_long_term",
    "key": "user_preference:X",
    "value": "Y"
  }]
}
Critical Files to Reference
File What to Extract
internal/shards/coder/generation.go buildSystemPrompt(), buildSessionContextPrompt()
internal/perception/transducer.go transducerSystemPrompt (614 lines), VerbCorpus
internal/articulation/emitter.go PiggybackEnvelope, ReasoningTraceDirective
internal/core/shard_manager.go SessionContext struct (15+ categories)
internal/core/defaults/policy.mg Spreading activation rules (Section 1)
internal/shards/system/constitution.go Safety gate prompts
internal/shards/system/planner.go plannerSystemPrompt, decompositionPrompt
.claude/skills/integration-auditor/ Pattern for skill structure and audit scripts
Registry Updates
skill-registry.md
Add SK-010 entry for prompt-architect with:
Trigger conditions
Capabilities table
Integration points (mangle-programming, codenerd-builder, go-architect)
codenerd-builder/SKILL.md
Add to:
Skill Index table
When to Use Each Skill
Related Skill Documentation links
Implementation Order
SKILL.md - Core skill file with frontmatter and quick-start
prompt-anatomy.md - Foundation document
context-injection.md - Critical for understanding dynamic prompts
shard-prompts.md - Real examples with annotations
tool-steering.md - Deterministic tool patterns
specialist-prompts.md - Type B/U knowledge hydration
anti-patterns.md - What to avoid
audit-checklist.md - Verification procedures
audit_prompts.py - Automated auditor
Registry updates - skill-registry.md, codenerd-builder SKILL.md
Estimated Size
File Lines
SKILL.md ~300
prompt-anatomy.md ~600
tool-steering.md ~400
context-injection.md ~500
shard-prompts.md ~700
specialist-prompts.md ~500
audit-checklist.md ~400
anti-patterns.md ~400
audit_prompts.py ~500
Total ~4,300
Success Criteria
All 7 reference documents cover their topics comprehensively
Real code examples extracted from codebase (not synthesized only)
Side-by-side current→improved patterns where applicable
Audit script catches structural and semantic issues
Skill registered in skill-registry.md
Can write a new shard prompt using only this skill as reference
User approved the plan
go on
Perfect! Let me start implementing the prompt-architect skill. I'll track progress and begin with the core structure.
Update Todos

Create .claude/skills/prompt-architect/ directory structure

Write SKILL.md (~300 lines)

Write references/prompt-anatomy.md (~600 lines)

Write references/context-injection.md (~500 lines)

Write references/shard-prompts.md (~700 lines)

Write references/tool-steering.md (~400 lines)

Write references/specialist-prompts.md (~500 lines)

Write references/anti-patterns.md (~400 lines)

Write references/audit-checklist.md (~400 lines)

Write scripts/audit_prompts.py (~500 lines)

Update skill-registry.md with SK-010

Update codenerd-builder/SKILL.md
Bash Create prompt-architect skill directory structure
IN
mkdir -p "c:\CodeProjects\codeNERD\.claude\skills\prompt-architect\references" "c:\CodeProjects\codeNERD\.claude\skills\prompt-architect\scripts"
OUT
0
go on
Update Todos

Create .claude/skills/prompt-architect/ directory structure

Write SKILL.md (~300 lines)

Write references/prompt-anatomy.md (~600 lines)

Write references/context-injection.md (~500 lines)

Write references/shard-prompts.md (~700 lines)

Write references/tool-steering.md (~400 lines)

Write references/specialist-prompts.md (~500 lines)

Write references/anti-patterns.md (~400 lines)

Write references/audit-checklist.md (~400 lines)

Write scripts/audit_prompts.py (~500 lines)

Update skill-registry.md with SK-010

Update codenerd-builder/SKILL.md
