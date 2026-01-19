# Context Compression Stress Test

Stress test for emergency context compression.

## Overview

Tests the context system with:

- 100+ turn conversations
- Token limit exhaustion
- Emergency compression triggers
- Context quality after compression

**Expected Duration:** 20-30 minutes

## Quick Reference

### Test Commands

```bash
# Long conversation simulation
for i in {1..100}; do
  ./nerd.exe run "explain step $i of implementing a complex feature"
  sleep 2
done

# Check context state
./nerd.exe query "context_tokens"
./nerd.exe query "compression_event"
```

### Expected Behavior

- Should trigger compression at threshold
- Context should remain coherent
- Important facts preserved
- No duplicate compression

---

## Severity Levels

### Conservative
- 50 turns, stay within limits

### Aggressive
- 100 turns, approach compression threshold

### Chaos
- 200+ turns, force multiple compressions

### Hybrid
- Combine with large fact injection

