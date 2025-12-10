# Shard Explosion Stress Test

Stress test for spawning all shard types in rapid succession.

## Overview

Tests the ShardManager with:

- Rapid shard spawning (all types)
- Concurrent execution limits
- Queue backpressure behavior
- Shard lifecycle management

**Expected Duration:** 15-25 minutes

## Quick Reference

### Test Commands

```bash
# Spawn multiple shards rapidly
./nerd.exe spawn coder "task 1"
./nerd.exe spawn tester "task 2"
./nerd.exe spawn reviewer "task 3"
./nerd.exe spawn researcher "task 4"
./nerd.exe spawn nemesis "task 5"

# Check shard status
./nerd.exe query "shard_state"

# Monitor queue
./nerd.exe query "spawn_queue_depth"
```

### Expected Behavior

- Queue should accept up to 100 pending spawns
- Max 4 concurrent shards executing
- Backpressure when queue full
- Graceful degradation under load

---

See `campaign-marathon.md` for detailed workflow structure.

