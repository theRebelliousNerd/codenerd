# intent/ - Intent-Specific Guidance Atoms

Guidance tailored to specific user intents.

## Files

| File | Intent | Purpose |
|------|--------|---------|
| `create.yaml` | `/create` | New code/file generation |
| `refactor.yaml` | `/refactor` | Code restructuring |
| `test.yaml` | `/test` | Test generation and execution |
| `review.yaml` | `/review` | Code review |
| `research.yaml` | `/research` | Documentation lookup |
| `explain.yaml` | `/explain` | Code explanation |

## Selection

Intent atoms are selected via `intent_verbs`:

```yaml
intent_verbs: ["/refactor", "/restructure"]
```

## Purpose

Intent atoms provide:
- Intent-specific methodology
- Success criteria for the intent
- Common pitfalls to avoid
- Output format expectations


> *[Archived & Reviewed by The Librarian on 2026-01-25]*