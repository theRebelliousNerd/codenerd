---
trigger: always_on
description: Commit early, push often
---

# Checkpoint Regularly

> **Commit early, push often, never lose work.**

## When to Checkpoint

- Feature/fix complete
- Tests passing  
- Before risky changes
- Every ~30 min of work
- Before ending session

## Commit Format

```bash
git commit -m "feat(aegis): wire initializeCentroids to config"
git commit -m "fix(wormhole): integrate attention pipeline"
git commit -m "test(vector): add clustering edge cases"
```

**Always push after commit. Unpushed work is at risk.**
