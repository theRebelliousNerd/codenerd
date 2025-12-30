# safety/ - Constitutional Safety Atoms

Safety constraints enforced by the Constitution Gate.

## Files

| File | Purpose |
|------|---------|
| `constitution.yaml` | Core constitutional rules (dangerous commands, forbidden patterns) |
| `constitutional.yaml` | Extended constitutional guidance |

## Key Constraints

- Dangerous command blocking (`rm -rf`, `sudo`, etc.)
- Credential/secret protection
- File scope enforcement
- Rate limiting guidance

## Integration

Safety atoms are **mandatory** (skeleton) and always included. The Constitution Gate validates `permitted(Action)` before any action executes.
