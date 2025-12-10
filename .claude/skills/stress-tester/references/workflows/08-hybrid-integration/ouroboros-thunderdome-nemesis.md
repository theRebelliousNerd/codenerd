# Ouroboros-Thunderdome-Nemesis Chain Stress Test

Stress test for the adversarial tool evolution loop.

## Overview

Tests the full adversarial chain:

- Ouroboros generates tool
- Thunderdome attacks it
- Nemesis finds weaknesses
- Tool is hardened or rejected

**Expected Duration:** 35-50 minutes

## Quick Reference

### Test Commands

```bash
# Generate tool that needs hardening
./nerd.exe tool generate "file system utility with user input"

# Run adversarial gauntlet
./nerd.exe thunderdome --auto
./nerd.exe nemesis --target generated_tool

# Check results
./nerd.exe query "tool_generated"
./nerd.exe query "thunderdome_result"
./nerd.exe query "nemesis_finding"
```

### Expected Behavior

- Tool generated successfully
- Attacks identify real vulnerabilities
- Nemesis provides actionable fixes
- Final tool is hardened

---

## Severity Levels

### Conservative
- Simple tool, standard attacks

### Aggressive
- Complex tool, 100+ attacks

### Chaos
- Tool with known vulnerabilities

### Hybrid
- Multiple tools through gauntlet

