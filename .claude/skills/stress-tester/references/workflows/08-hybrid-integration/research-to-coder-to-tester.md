# Research to Coder to Tester Pipeline Stress Test

Stress test for the full development pipeline.

## Overview

Tests the shard handoff chain:

- ResearcherShard gathers knowledge
- CoderShard implements
- TesterShard validates
- ReviewerShard checks

**Expected Duration:** 30-45 minutes

## Quick Reference

### Test Commands

```bash
# Trigger full pipeline
./nerd.exe run "research best practices for Go error handling, implement them in our codebase, write tests, and review"

# Monitor pipeline
./nerd.exe query "shard_executed"
./nerd.exe query "knowledge_atom"
./nerd.exe query "test_result"
./nerd.exe query "hypothesis"
```

### Expected Behavior

- Research completes first
- Code uses research findings
- Tests cover new code
- Review catches issues

---

## Severity Levels

### Conservative
- Simple feature, full pipeline

### Aggressive
- Complex feature, many files

### Chaos
- Conflicting research, ambiguous requirements

### Hybrid
- Multiple pipelines concurrent

