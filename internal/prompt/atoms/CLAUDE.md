# internal/prompt/atoms - JIT Prompt Atom Library

**279 YAML files** across **26 categories** powering codeNERD's JIT Prompt Compiler.

## What Are Prompt Atoms?

Atoms are small, composable prompt fragments selected at runtime based on context. Instead of hardcoded system prompts, the JIT compiler assembles task-specific prompts from relevant atoms.

**Key Insight:** Mangle logic decides *what* the model should know; atoms provide the *language* to express it.

## Directory Structure

| Category | Files | Purpose |
|----------|-------|---------|
| [identity/](identity/) | 10 | Agent personas (coder, tester, reviewer, etc.) |
| [system/](system/) | 7 | System shard prompts (executive, router, legislator) |
| [protocol/](protocol/) | 3 | Communication protocols (Piggyback, reasoning traces) |
| [safety/](safety/) | 2 | Constitutional constraints |
| [methodology/](methodology/) | 7 | Problem-solving approaches (TDD, OODA, debugging) |
| [capability/](capability/) | 6 | Tool capabilities (CodeDOM, knowledge discovery) |
| [campaign/](campaign/) | 18 | Multi-phase campaign orchestration |
| [hallucination/](hallucination/) | 6 | Anti-hallucination guards per shard type |
| [exemplar/](exemplar/) | 6 | Few-shot examples |
| [language/](language/) | 14 | Language-specific guidance (Go, Python, Rust, etc.) |
| [framework/](framework/) | 17 | Framework guidance (Bubbletea, React, Rod, etc.) |
| [mangle/](mangle/) | 109 | Mangle/Datalog language reference |
| [context/](context/) | 5 | Dynamic context injection (files, symbols, errors) |
| [domain/](domain/) | 3 | Project/codebase context |
| [intent/](intent/) | 6 | Intent-specific guidance (create, refactor, test) |
| [init/](init/) | 10 | Workspace initialization |
| [knowledge/](knowledge/) | 4 | Knowledge extraction and retrieval |
| [perception/](perception/) | 2 | NL transduction |
| [northstar/](northstar/) | 6 | Goal alignment and vision tracking |
| [ouroboros/](ouroboros/) | 7 | Self-modification and tool generation |
| [autopoiesis/](autopoiesis/) | 1 | Meta-atom generation |
| [build_layer/](build_layer/) | 6 | Layered architecture guidance |
| [eval/](eval/) | 1 | LLM-as-Judge evaluation |
| [reviewer/](reviewer/) | 1 | Code review enhancement |
| [shards/](shards/) | 17 | Legacy shard-specific atoms |
| [world_state/](world_state/) | 5 | World state triggers (diagnostics, churn, security) |

## How Atoms Are Selected

```
CompilationContext (mode, intent, language, shard type, world states)
    |
    v
Skeleton Selection (mandatory atoms via Mangle rules)
    |
    v
Flesh Selection (optional atoms via vector similarity)
    |
    v
Budget Fitting (polymorphic content: standard -> concise -> min)
    |
    v
Assembly (category-ordered concatenation)
```

## Atom YAML Structure

```yaml
- id: "identity/coder/mission"
  category: "identity"
  priority: 100
  is_mandatory: true

  # Contextual Selectors (11 dimensions)
  shard_types: ["/coder"]
  intent_verbs: ["/fix", "/implement", "/refactor"]
  languages: ["/go", "/python"]
  frameworks: ["/bubbletea"]
  operational_modes: ["/active", "/debugging"]
  world_states: ["/failing_tests"]

  # Composition
  depends_on: ["protocol/piggyback/format"]
  conflicts_with: ["identity/reviewer/mission"]

  content: |
    You are the Coder...

  # Budget polymorphism
  content_concise: |
    Coder: generate/modify code...
  content_min: |
    Coder agent.
```

## Adding New Atoms

1. Choose the correct category folder
2. Use stable IDs (other code may reference them)
3. Add contextual selectors to limit when atom appears
4. For large atoms, add `content_concise` and `content_min` variants
5. Use `depends_on` for ordering, `conflicts_with` for exclusivity

## Related Files

- `internal/prompt/compiler.go` - JIT compilation pipeline
- `internal/prompt/atoms.go` - AtomCategory constants
- `internal/prompt/loader.go` - YAML loading
- `internal/prompt/selector.go` - Skeleton/flesh selection
