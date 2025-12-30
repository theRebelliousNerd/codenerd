# Piggyback Corruption Stress Test

Stress test for articulation layer with malformed JSON payloads.

## Overview

Tests the Piggyback protocol parser's handling of:

- Truncated JSON responses
- Missing fields
- Invalid types
- Malformed structure

**Expected Duration:** 15-30 minutes

## Quick Reference

Use malformed payloads from `assets/malformed_piggyback.json`.

### Conservative

Test valid Piggyback parsing with standard LLM responses.

### Aggressive

Force large responses that may truncate.

```bash
./nerd.exe spawn coder "generate extremely detailed documentation for every function"
```

### Chaos

Use malformed inputs to test fallback parsers.

### Hybrid

Combine with campaign execution to test under load.

---

See `intent-fuzzing.md` for detailed workflow structure.
