# Large Codebase Scan Stress Test

Stress test for world model with massive codebases.

## Overview

Tests the World system with:

- 10,000+ file scanning
- Deep directory nesting
- Symlink loop handling
- AST projection at scale

**Expected Duration:** 25-40 minutes

## Quick Reference

### Test Commands

```bash
# Generate large project
python .claude/skills/stress-tester/scripts/fixtures/generate_large_project.py --files 10000 --depth 15

# Trigger full scan
./nerd.exe world scan --full

# Check topology
./nerd.exe query "file_topology"
./nerd.exe query "symbol_count"
```

### Expected Behavior

- Should complete without timeout
- Memory should stay bounded
- Symlink loops detected and skipped
- AST parsing should be incremental

---

See `context-compression.md` for detailed workflow structure.

