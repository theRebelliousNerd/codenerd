# autopoiesis/ - Meta-Atom Generation

Atoms for the prompt evolution system.

## Files

| File | Purpose |
|------|---------|
| `meta_atom_generator.yaml` | Generate new prompt atoms from execution feedback |

## Prompt Evolution System

The autopoiesis system learns from execution:

```text
Execute -> Evaluate (LLM-as-Judge) -> Evolve (Meta-Prompt) -> Integrate (JIT)
```

## Meta-Atom Generation

When patterns of failure are detected:
1. Classify error type (LOGIC_ERROR, API_MISUSE, etc.)
2. Generate corrective atom
3. Store in `evolved/pending/`
4. Promote after user approval

## Selection

Selected during prompt evolution cycles in `internal/autopoiesis/prompt_evolution/`.


> *[Archived & Reviewed by The Librarian on 2026-01-25]*