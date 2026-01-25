# identity/ - Agent Persona Atoms

Defines the core identity, mission, and behavioral constraints for each agent type.

## Files

| File | Agent | Purpose |
|------|-------|---------|
| `coder.yaml` | CoderShard | Code generation, bug fixing, refactoring |
| `tester.yaml` | TesterShard | Test generation, TDD loops, coverage |
| `reviewer.yaml` | ReviewerShard | Code review, security scanning |
| `researcher.yaml` | ResearcherShard | Documentation research, knowledge extraction |
| `nemesis.yaml` | NemesisShard | Adversarial analysis, attack generation |
| `librarian.yaml` | LibrarianShard | Knowledge curation, fact management |
| `legislator.yaml` | LegislatorShard | Mangle rule synthesis from feedback |
| `tool_generator.yaml` | ToolGenerator | Ouroboros tool creation |
| `panic_maker.yaml` | PanicMaker | Adversarial input fuzzing |
| `custom_shard.yaml` | User-defined | Template for custom specialists |

## Usage

Identity atoms are **mandatory** (skeleton) and selected via `shard_types` selector:

```yaml
shard_types: ["/coder", "/fixer"]
intent_verbs: ["/fix", "/implement", "/refactor"]
```

## Key Pattern

Each identity file typically contains:
- `identity/{agent}/mission` - Core purpose and responsibilities
- `identity/{agent}/constraints` - Behavioral boundaries
- `identity/{agent}/output_format` - Expected response structure


> *[Archived & Reviewed by The Librarian on 2026-01-25]*