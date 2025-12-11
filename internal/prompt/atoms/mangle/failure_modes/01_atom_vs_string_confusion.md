# Failure Mode 1: Atom vs String Confusion

## Category
Syntactic Hallucination (Souffle/SQL Bias)

## Severity
CRITICAL - Silent failure with empty results

## Error Pattern
Using string literals `"value"` when Mangle requires atom syntax `/value`. This is the most common and dangerous error because **it compiles successfully but returns no results**.

## Wrong Code
```mangle
# Facts stored with atoms
status(/user1, /active).
status(/user2, /inactive).

# WRONG - will return NOTHING (string doesn't match atom)
active_users(U) :- status(U, "active").

# WRONG - Enum-style notation (Java/Clojure bias)
active_users(U) :- status(U, status.active).

# WRONG - Using true/false for flags
enabled(X) :- feature(X, true).
```

## Correct Code
```mangle
# Facts stored with atoms
status(/user1, /active).
status(/user2, /inactive).

# CORRECT - Use atom syntax
active_users(U) :- status(U, /active).

# CORRECT - Enum values are atoms
priority(Task, /critical).
priority(Task, /warning).
priority(Task, /info).

# CORRECT - Flags are atoms
enabled(X) :- feature(X, /enabled).
disabled(X) :- feature(X, /disabled).
```

## Detection
- **Symptom**: Query returns empty results despite facts existing
- **Pattern**: Look for quoted strings in rule bodies where constants are used
- **Test**: `grep -E '"\w+"' *.mg | grep -v '^#'` to find suspicious strings
- **Diagnostic**: If a fact like `status(/user, /active)` exists but `status(U, "active")` returns nothing

## Prevention
1. **Use `/atom` syntax for:**
   - Identifiers (`/user_id`, `/project_name`)
   - Enum values (`/critical`, `/warning`, `/info`)
   - Status flags (`/active`, `/pending`, `/done`)
   - Category labels (`/frontend`, `/backend`, `/database`)

2. **Only use `"string"` for:**
   - Human-readable text (`"John Doe"`, `"Error message"`)
   - External data that genuinely varies (`"CVE-2021-44228"`)
   - Content with spaces/special characters (`"my file.txt"`)

3. **Mental model**: If it's a **symbol** or **label**, use `/atom`. If it's **prose** or **data**, use `"string"`.

## Type Unification Table
| Mangle Type | Will Unify With | Will NOT Unify With |
|-------------|-----------------|---------------------|
| `/active` | `/active`, `X` (variable) | `"active"`, `/inactive` |
| `"active"` | `"active"`, `X` (variable) | `/active`, `"inactive"` |
| `42` | `42`, `X` (variable) | `"42"`, `/42` |

## Training Bias Origins
| Language | Syntax | Leads to Wrong Mangle |
|----------|--------|----------------------|
| Python | `status = "active"` | `status(U, "active")` |
| SQL | `WHERE status = 'active'` | `status(U, 'active')` |
| Java | `Status.ACTIVE` | `status(U, status.active)` |
| JSON | `{"status": "active"}` | `status(U, "active")` |

## Quick Check
Before writing any Mangle constant:
1. Is this a category/label/identifier? → Use `/atom`
2. Could there be 50+ variations of this value? → Use `"string"`
3. Does it look like an enum value? → Use `/atom`
4. Is it prose or error messages? → Use `"string"`
