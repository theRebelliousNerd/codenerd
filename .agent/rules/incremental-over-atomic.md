---
trigger: always_on
description: Small verified steps, not big-bang changes
---

# Incremental Over Atomic

> **Small steps that individually work > Large steps that eventually work.**

## Each Increment Should Be

- **Verifiable** – Testable independently
- **Reversible** – Rollback-able without cascade
- **Reviewable** – Understandable in <5 min
- **Deployable** – Doesn't break the build

**If an increment takes >30 minutes, slice smaller.**

## The Pattern

```
1. Add new alongside old → verify → commit
2. Migrate one caller → verify → commit  
3. Repeat until complete
4. Remove old → commit
```

## Warning Signs

- Changing >5 files at once → slice smaller
- Working 1+ hour without commit → checkpoint now
- Can't describe change in one sentence → too big
- Afraid to run tests → you've gone too far

> Every commit leaves the codebase better, never worse.
