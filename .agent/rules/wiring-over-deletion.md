---
trigger: always_on
description: Wire unused code, don't delete it
---

# Wiring Over Deletion

> **Unused code often represents missing features, not garbage.**

**You are FORBIDDEN from deleting "unused" code without first proving it is truly obsolete.**

## Go Wiring

| Warning | Resolution |
|---------|------------|
| `var declared but not used` | Log it, return it, or trace it |
| `ctx parameter unused` | Pass to all DB/HTTP calls |
| `err variable unused` | Handle it: `if err != nil { return fmt.Errorf("context: %w", err) }` |

**❌ FORBIDDEN**: `_ = score` or `_ = err`

## React/TS Wiring

| Warning | Resolution |
|---------|------------|
| `'X' defined but never used` | Render it, use in JSX |
| `Prop 'x' defined but unused` | Apply in conditional, class, or logic |

## Mangle/Datalog Wiring

| Warning | Resolution |
|---------|------------|
| `Predicate 'foo' unused` | Chain to other rules: `bar(X) :- foo(X, Y)` |

## Before Deleting Anything

1. **Semantic value?** – Does name suggest meaningful data? → Wire to UI/logs
2. **Structural integrity?** – Is it `ctx` or `config`? → Pass it down
3. **Recent addition?** – Added last turn? → **MUST use it**

> A passing linter on a broken feature is a failure. A failing linter on a working feature is just a TODO.
