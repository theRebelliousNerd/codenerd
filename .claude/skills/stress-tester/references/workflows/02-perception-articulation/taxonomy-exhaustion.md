# Taxonomy Exhaustion Stress Test

Stress test for verb corpus and intent classification.

## Overview

Tests the VerbCorpus with:

- Every known verb
- Unknown/novel verbs
- Verb combinations

**Expected Duration:** 15-25 minutes

## Quick Reference

### Test Commands

```bash
# Known verbs
./nerd.exe perception "review"
./nerd.exe perception "fix"
./nerd.exe perception "test"
./nerd.exe perception "explain"
./nerd.exe perception "create"
./nerd.exe perception "refactor"
./nerd.exe perception "deploy"
./nerd.exe perception "analyze"

# Unknown verbs
./nerd.exe perception "yeet the code"
./nerd.exe perception "vibe check the repository"
./nerd.exe perception "uwu the functions"

# Combinations
./nerd.exe perception "review and test and fix"
```

### Expected Behavior

- Known verbs: High confidence classification
- Unknown verbs: Fallback to closest match or generic
- Combinations: May split into multiple intents or choose primary

---

See `intent-fuzzing.md` for detailed workflow structure.
