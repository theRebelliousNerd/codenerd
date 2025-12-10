# Perception to Campaign Integration Stress Test

Stress test for NL input flowing through to campaign execution.

## Overview

Tests the full path:

- Perception transduction under load
- Intent → Campaign mapping
- Multi-phase campaign execution
- Articulation of results

**Expected Duration:** 25-40 minutes

## Quick Reference

### Test Commands

```bash
# Complex NL that triggers campaign
./nerd.exe run "implement a complete REST API with authentication, rate limiting, and database integration"

# Monitor the flow
./nerd.exe query "user_intent"
./nerd.exe query "campaign_phase"
./nerd.exe query "shard_executed"
```

### Expected Behavior

- Intent correctly classified
- Campaign phases planned
- Shards execute in order
- Results articulated properly

---

## Severity Levels

### Conservative
- Simple NL → small campaign

### Aggressive
- Complex NL → 20+ phase campaign

### Chaos
- Ambiguous NL → campaign chaos

### Hybrid
- Multiple campaigns overlapping

