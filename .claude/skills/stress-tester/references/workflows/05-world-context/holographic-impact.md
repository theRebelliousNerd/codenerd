# Holographic Impact Stress Test

Stress test for impact-aware context building.

## Overview

Tests the Holographic system with:

- Large change sets
- Cross-file impact analysis
- Priority scoring at scale
- Context selection under pressure

**Expected Duration:** 20-30 minutes

## Quick Reference

### Test Commands

```bash
# Generate project with dependencies
python .claude/skills/stress-tester/scripts/fixtures/generate_large_project.py --files 500 --cross-deps

# Make widespread changes
./nerd.exe run "refactor the core module affecting many dependents"

# Check impact analysis
./nerd.exe query "modified_function"
./nerd.exe query "context_priority"
./nerd.exe query "impacted_files"
```

### Expected Behavior

- Should identify all impacted files
- Priority scoring should work
- Context selection should be relevant
- Should not include unaffected files

---

See `context-compression.md` for detailed workflow structure.

