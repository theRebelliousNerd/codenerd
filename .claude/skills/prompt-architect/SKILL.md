---
name: prompt-architect
description: Master prompt engineering for codeNERD's neuro-symbolic architecture. Use when writing new persona atoms, auditing existing prompts, debugging LLM behavior, or optimizing context injection. Covers JIT prompt compilation, persona atoms, dynamic injection, Piggybacking protocol, tool steering, and ConfigFactory.
---

# Prompt Architect Skill

Design, write, and audit prompts for codeNERD's neuro-symbolic architecture.

> **Architecture Update (Dec 2024):** Domain shards (coder, tester, reviewer, researcher) have been **deleted**. All persona/identity now comes from **JIT-compiled prompt atoms** in `internal/prompt/atoms/identity/*.yaml`. The `ConfigFactory` maps intents to tools and policies.

In codeNERD, a "prompt" is a **cybernetic control system** bridging the Stochastic (LLM) and Deterministic (Mangle Kernel).

## Core Concepts

| Concept | Description | Criticality |
|---------|-------------|-------------|
| **Static Prompts** | Base Go constants. Immutable bedrock. | High |
| **Dynamic Injection** | Real-time context via `SessionContext`. | High |
| **Piggybacking** | `{"control":..., "surface":...}` JSON protocol. | CRITICAL |
| **Thought-First** | Control packets MUST precede surface text. | CRITICAL |
| **Tool Steering** | Description-based affinity, kernel grants tools. | Medium |
| **Artifact Types** | `project_code` vs `self_tool` vs `diagnostic`. | High |

## When to Use

| Activity | Goal |
|----------|------|
| Writing New Persona Atoms | Define identity, capabilities, constraints in YAML |
| Creating ConfigFactory Mappings | Map intents to tools and policies |
| Auditing Prompts | Ensure Piggyback Protocol and Constitutional boundaries |
| Debugging LLM Behavior | Identify "Context Starvation" or "Ambiguous Steering" |
| Creating Specialists | Inject domain-specific Knowledge Atoms |

## Quick-Start Patterns

### 1. Thought-First Protocol (Anti-Hallucination)

```go
const SystemPrompt = `...
CRITICAL PROTOCOL:
You must ALWAYS output a JSON object containing "control_packet" and "surface_response".

THOUGHT-FIRST ORDERING:
You MUST output control_packet BEFORE surface_response.
Do NOT output text until the control packet is complete.
...`
```

### 2. Artifact Classification

```text
ARTIFACT CLASSIFICATION (MANDATORY):
- "project_code": Code for user's codebase (default).
- "self_tool": A temporary internal tool (Autopoiesis).
- "diagnostic": A one-time inspection script (Ephemeral).
```

### 3. Deterministic Tooling

```text
AVAILABLE TOOLS (Selected by Kernel):
- [tool_name]: [description]

You MUST use one of these tools. Do not invent tools.
If no tool matches, emit missing_tool_for(intent) in the control packet.
```

## God Tier Prompts

codeNERD's high compression ratio enables **maximalism over minimalism**:

- **Minimum 8,000 characters** for functional prompts
- **Minimum 15,000-20,000 characters** for shard agents
- **Production standard: 20,000+ characters** for specialists

### What to Include

| Section | Character Target |
|---------|------------------|
| Persona | 500+ chars |
| Core Responsibilities | 1,000+ chars |
| Protocol Definitions | 2,000+ chars |
| Methodology | 3,000+ chars |
| Context Schema | 1,500+ chars |
| Tool Catalog | 1,000+ chars |
| Edge Cases | 2,000+ chars |
| Output Examples | 2,000+ chars |
| Constitutional Boundaries | 1,000+ chars |
| Quality Standards | 1,500+ chars |

## Persona Atom Creation (New Standard)

> **Dec 2024:** All persona/identity now comes from YAML atoms in `internal/prompt/atoms/identity/`.

### Creating a Persona Atom

```yaml
# internal/prompt/atoms/identity/coder.yaml
- id: "identity/coder/mission"
  category: "identity"
  subcategory: "mission"
  priority: 100
  is_mandatory: true
  intent_verbs: ["/fix", "/implement", "/refactor", "/create"]
  content: |
    You are the Coder persona of codeNERD.
    ## Core Responsibilities
    1. Generate new code following project patterns
    2. Modify existing code to fix bugs or add features
    ...
```

### ConfigFactory Tool Mapping

```go
// internal/prompt/config_factory.go
// Maps intent verbs to allowed tools
provider.atoms["/fix"] = ConfigAtom{
    Tools: coderTools,  // read_file, write_file, edit_file, run_build, etc.
    Priority: 100,
}
```

### Legacy Specialist Types (for reference)

**Type B**: Project-specific specialists (created during `/init`)
**Type U**: User-defined specialists (created via `/define-agent`)

## JIT Prompt Compiler

The JIT Prompt Compiler dynamically assembles prompts from atomic components using a 10-stage pipeline. Instead of monolithic 20,000-character prompts, break them into composable YAML atoms.

**Status**: Phase 8 (Testing) - IN PROGRESS

See [jit-compiler.md](references/jit-compiler.md) for complete architecture.

**Key Components**:
- `internal/prompt/compiler.go` - Main orchestrator
- `internal/prompt/selector.go` - Mangle + vector selection
- `internal/prompt/budget.go` - Token budget management

**Enable**: `export USE_JIT_PROMPTS=true`

## Common Issues

| Symptom | Quick Fix |
|---------|-----------|
| Model ignores tools | Replace static tool list with dynamic injection via `SessionContext.AvailableTools` |
| Premature articulation | Add: `CRITICAL: Output control_packet BEFORE surface_response` |
| Generic specialist responses | Run `/init --force` to re-hydrate knowledge base |
| Context starvation | Add injection points: `{{.SessionContext}}` or `%s` |
| Tool hallucination | Add: `You MUST NOT invent tools. Kernel will reject hallucinated tools.` |

## Validation Tools

### audit_prompts.py

```bash
python .claude/skills/prompt-architect/scripts/audit_prompts.py --root .
```

**Checks**: Length minimums, Piggyback schema, Thought-First ordering, Artifact types

**Exit Codes**: `0` = pass, `1` = violations found

## Reference Documentation

| Reference | Contents |
|-----------|----------|
| [god-tier-templates.md](references/god-tier-templates.md) | 20,000+ char production-ready prompts |
| [prompt-anatomy.md](references/prompt-anatomy.md) | Static vs Dynamic layers, Piggyback Envelope |
| [context-injection.md](references/context-injection.md) | SessionContext schema, Spreading Activation |
| [tool-steering.md](references/tool-steering.md) | Tool descriptions, Shard Affinity, Mangle predicates |
| [shard-prompts.md](references/shard-prompts.md) | Coder, Tester, Reviewer templates |
| [specialist-prompts.md](references/specialist-prompts.md) | Type B/U architecture, Knowledge Atoms |
| [anti-patterns.md](references/anti-patterns.md) | 15+ failure modes with fixes |
| [audit-checklist.md](references/audit-checklist.md) | Structural, semantic, safety verification |
| [jit-compiler.md](references/jit-compiler.md) | JIT Prompt Compiler architecture |

## Integration Points

- **mangle-programming**: Logic predicates for dynamic injection
- **codenerd-builder**: System architecture, semantic classification
- **go-architect**: Go constraints in prompt templates
- **research-builder**: Knowledge hydration, Context7 protocol
