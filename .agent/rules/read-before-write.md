---
trigger: always_on
description: Match the codebase, don't fight it
---

# Read Before Write

> **Match the codebase, don't fight it.**

## Before Writing, Find

| Writing | Find First |
|---------|------------|
| New struct/type | Existing similar types |
| New function | Existing utilities |
| New endpoint | Existing handler patterns |
| New test | Existing test helpers |
| New error | Existing error conventions |

## Pattern Matching

1. **Use same naming** – If codebase uses `FooHandler`, don't create `handleFoo`
2. **Use same organization** – If handlers go in `handlers/`, follow suit
3. **Use same error handling** – If codebase wraps errors, wrap yours
4. **Use existing utilities** – Don't write `formatTime()` if `utils.FormatTime()` exists

## Before Creating Anything

- Does this function already exist? → **Use it**
- Does a similar type exist? → **Extend it**
- Is there a helper for this? → **Call it**
- Is there an established pattern? → **Follow it**

> The codebase was here before you. Make it more consistent, not less.
