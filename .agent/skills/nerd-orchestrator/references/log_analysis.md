# codeNERD Log Forensics Guide

This guide explains how to investigate failures using the Mangle Log Analyzer.

## 1. Preparation

First, convert the raw text logs into a Mangle knowledge base (`.mg` file).

```powershell
# Parse all logs
python .agent/skills/log-analyzer/scripts/parse_log.py .nerd/logs/* --no-schema > debug.mg

# OR Parse specific subsystem
python .agent/skills/log-analyzer/scripts/parse_log.py .nerd/logs/kernel.log --no-schema > kernel_debug.mg
```

## 2. Interactive Analysis

Launch the Mangle REPL with your facts.

```powershell
.agent/skills/log-analyzer/scripts/logquery/logquery.exe debug.mg -i
```

## 3. Forensic Queries

Copy-paste these into the REPL to find issues.

### 🚨 Critical Failures

**Did anything crash?**
```mangle
?panic_detected(Time, Category, Message)
?oom_event(Time, Category, Message)
?nil_pointer_error(Time, Category, Message)
```

**Is the Kernel dying?**
```mangle
?kernel_panic_on_boot(Time, Message)
?derivation_explosion(Time, Message)
?gas_limit_hit(Time, Message)
```

### 📉 Resource Contention

**Is the API Scheduler blocked?**
```mangle
?slot_wait(Time, Shard, Message)
?api_scheduler_deadlock(Time, Message)
?slot_leak_warning(Time, Message)
```

**Is the Spawn Queue full?**
```mangle
?queue_full(Time, Message)
?shard_limit_hit(Time, Message)
```

### 🛡️ Mangle Self-Healing

**Did the system attempt to repair itself?**
```mangle
?repair_attempt(Time, Message)
?repair_success(Time, Message)
?repair_failure(Time, Message)
```

**Is the schema drifting?**
```mangle
?schema_drift(Time, Message)
?validation_error(Time, Message)
```

### 🧠 Autopoiesis & Tools

**Did tool generation fail?**
```mangle
?tool_compile_failure(Time, Message)
?safety_bypass(Time, Message)
```

## 4. Quick Grep (If you're in a hurry)

If you don't want to use Mangle, use these regex patterns on the raw logs:

- **Panics:** `grep -i "panic" .nerd/logs/*.log`
- **Deadlocks:** `grep -i "deadline exceeded" .nerd/logs/*.log`
- **Mangle Errors:** `grep -i "undeclared" .nerd/logs/*.log`
- **Queue Full:** `grep -i "queue full" .nerd/logs/*.log`
