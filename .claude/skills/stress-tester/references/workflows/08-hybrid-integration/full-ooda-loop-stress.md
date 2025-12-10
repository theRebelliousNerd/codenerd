# Full OODA Loop Stress Test

Stress test for the complete Observe-Orient-Decide-Act cycle.

## Overview

Tests all OODA phases under load:

- **Observe**: Perception transduction at scale
- **Orient**: Spreading activation with many facts
- **Decide**: Mangle derivation under pressure
- **Act**: VirtualStore execution concurrently

**Expected Duration:** 40-60 minutes

## Quick Reference

### Test Commands

```bash
# Complex task requiring full OODA
./nerd.exe run "analyze the entire codebase, identify 10 areas for improvement, implement fixes, test them, and report results"

# Monitor each phase
./nerd.exe query "user_intent"        # Observe
./nerd.exe query "context_atom"       # Orient
./nerd.exe query "next_action"        # Decide
./nerd.exe query "action_result"      # Act
```

### Expected Behavior

- All phases execute correctly
- No phase becomes bottleneck
- Feedback loops work
- Final result is coherent

---

## Severity Levels

### Conservative
- Single OODA cycle, simple task

### Aggressive
- 10+ OODA cycles, complex task

### Chaos
- Conflicting inputs, rapid cycles

### Hybrid
- OODA + Dream + Shadow combined

---

## Post-Test Analysis

### Log Analysis (via log-analyzer skill)
```bash
cd .claude/skills/log-analyzer/scripts
python parse_log.py .nerd/logs/* --no-schema | grep "^log_entry" > /tmp/stress.mg
cd logquery
./logquery.exe /tmp/stress.mg --builtin errors
./logquery.exe /tmp/stress.mg --builtin kernel-errors
```

### Success Criteria
- [ ] All OODA phases completed
- [ ] No phase timeouts
- [ ] Memory stayed bounded
- [ ] Results were coherent

### Known Issues to Watch For
- Orient phase slowdown with many facts
- Decide phase gas limit hits
- Act phase VirtualStore contention

