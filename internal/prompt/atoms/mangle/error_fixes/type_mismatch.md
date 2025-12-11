# Fix: Type Mismatch Error

## Error Pattern
```
type mismatch
cannot unify
expected Type<X> got Type<Y>
```

## Common Mismatches

### 1. Atom vs String
**Error:** Using `"string"` where `/atom` is expected

**Before:**
```mangle
# Schema: status(User.Type<name>, State.Type<name>)
check(X) :- status(X, "active").
```

**After:**
```mangle
check(X) :- status(X, /active).
```

### 2. String vs Atom
**Error:** Using `/atom` where `"string"` is expected

**Before:**
```mangle
# Schema: message(Id.Type<name>, Text.Type<string>)
log(X) :- message(X, /error_occurred).
```

**After:**
```mangle
log(X) :- message(X, "error_occurred").
```

### 3. Integer vs Float
**Error:** Using `5` where `5.0` is expected

**Before:**
```mangle
# Schema: metric(Name.Type<name>, Value.Type<float>)
high(X) :- metric(X, V), V > 50.
```

**After:**
```mangle
high(X) :- metric(X, V), V > 50.0.
```

### 4. Float vs Integer
**Error:** Using `5.0` where `5` is expected

**Before:**
```mangle
# Schema: count(Name.Type<name>, N.Type<int>)
many(X) :- count(X, N), N > 10.0.
```

**After:**
```mangle
many(X) :- count(X, N), N > 10.
```

### 5. List in Scalar Field
**Error:** Using `[1,2,3]` where single value expected

**Before:**
```mangle
# Schema: single(Name.Type<name>, Val.Type<int>)
single(/x, [1, 2, 3]).
```

**After:**
```mangle
# If you need multiple values, use multiple facts
single(/x, 1).
single(/x, 2).
single(/x, 3).
```

## Quick Reference

| Field Type | Correct Literal | Wrong Literal |
|------------|----------------|---------------|
| `name` | `/value` | `"value"`, `value` |
| `string` | `"value"` | `/value`, `'value'` |
| `int` | `42` | `42.0` |
| `float` | `42.0` | `42` |

## Diagnosis

1. Find the predicate declaration (`Decl`)
2. Match argument positions to types
3. Check each literal matches expected type
4. For variables, trace back to first binding
