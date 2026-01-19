---
trigger: always_on
description: Production-ready, not MVP
---

# Comprehensiveness Over Simplification

> **If it ships incomplete, it didn't ship—it leaked.**

## Every Feature Must Have

| Dimension | Requirement |
|-----------|-------------|
| **Error Handling** | No swallowed errors, wrap with context |
| **Edge Cases** | Handle nil, empty, boundary, concurrent |
| **Config** | No hardcoded values that might change |
| **Tests** | Happy path + error paths + edge cases |
| **Logging** | Structured logging with fields |
| **Validation** | All inputs validated at boundaries |

## Go Checklist

- [ ] All errors checked and wrapped
- [ ] Context propagated through calls
- [ ] Nil checks on pointer derefs
- [ ] Mutex protection for shared state
- [ ] Proper resource cleanup (defer)

## API Checklist

- [ ] Input validation with clear errors
- [ ] Auth/authz checks
- [ ] Request/response logging
- [ ] Pagination for lists
- [ ] Timeout handling

## The Mantra

> **Complete is better than clever. Robust is better than fast.**

When in doubt, do MORE—add the error handling, the test, the validation.
