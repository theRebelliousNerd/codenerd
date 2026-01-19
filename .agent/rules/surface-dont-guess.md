---
trigger: always_on
description: When unsure, ask
---

# Surface Don't Guess

> **When unsure, ask—don't bet the codebase.**

## Ask When

- Requirements are ambiguous
- Multiple valid approaches exist
- Breaking changes are possible
- Scope is unclear
- Context is missing

## The Protocol

1. **Self-resolve (60s max)** – Search codebase, check docs
2. **If still uncertain → ASK**:
   - What I understand: [X]
   - What's unclear: [Y]
   - Options: [A tradeoffs] vs [B tradeoffs]
3. **Wait** – Do NOT proceed with a guess

## Cost Comparison

| Guessing Wrong | Asking |
|----------------|--------|
| Days of refactoring | 30 seconds |
| User-facing bugs | Clarity |
| Wasted effort | Right direction |

> Your confidence is not a substitute for their knowledge. Ask.
