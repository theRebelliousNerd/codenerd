# Reviewer Finding Explosion Stress Test

Stress test for ReviewerShard with massive finding generation.

## Overview

Tests the ReviewerShard with:

- Large codebase review (1000+ issues)
- Finding aggregation limits
- Hypothesis generation at scale
- Memory pressure from findings

**Expected Duration:** 20-30 minutes

## Quick Reference

### Test Commands

```bash
# Generate large codebase with issues
python .claude/skills/stress-tester/scripts/fixtures/generate_large_project.py --files 500 --issues-per-file 10

# Run comprehensive review
./nerd.exe review --deep

# Check finding count
./nerd.exe query "hypothesis_count"
```

### Expected Behavior

- Should handle 1000+ findings
- Finding aggregation should work
- Should not OOM from findings
- Should prioritize critical issues

---

See `campaign-marathon.md` for detailed workflow structure.

