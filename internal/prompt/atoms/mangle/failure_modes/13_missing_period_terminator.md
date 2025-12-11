# Failure Mode 13: Missing Period Terminator

## Category
Syntactic Hallucination (Basic Syntax)

## Severity
HIGH - Parse errors preventing program load

## Error Pattern
Forgetting to end Mangle statements with a period (`.`). Every declaration, fact, and rule MUST end with `.`

## Wrong Code
```mangle
# WRONG - Missing period on declaration
Decl parent(X.Type<n>, Y.Type<n>)

# WRONG - Missing period on fact
parent(/alice, /bob)

# WRONG - Missing period on rule
ancestor(X, Y) :- parent(X, Y)

# WRONG - Newline doesn't replace period
ancestor(X, Y) :- parent(X, Y)
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z)

# WRONG - Semicolon instead of period
parent(/alice, /bob);

# WRONG - Comma at end (thinking it's a list)
edge(/a, /b),
edge(/b, /c),
edge(/c, /d),
```

## Correct Code
```mangle
# CORRECT - Period on declaration
Decl parent(X.Type<n>, Y.Type<n>).

# CORRECT - Period on fact
parent(/alice, /bob).

# CORRECT - Period on rule
ancestor(X, Y) :- parent(X, Y).

# CORRECT - Each rule needs its own period
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).

# CORRECT - Multiple facts, each with period
edge(/a, /b).
edge(/b, /c).
edge(/c, /d).

# CORRECT - Multiline rule still needs one period at end
complex_rule(X, Y, Z) :-
    condition1(X),
    condition2(Y),
    condition3(Z),
    X != Y.
# Period here ^
```

## Detection
- **Symptom**: Parse error like "unexpected token" or "expected '.'"
- **Symptom**: "Unexpected end of file"
- **Pattern**: Statement not ending with period
- **Test**: Each declaration, fact, and rule should end with `.`

## Prevention
1. **Every statement ends with period** - No exceptions
2. **Newlines don't replace periods** - You need both
3. **Multiline statements need one period** - At the very end
4. **Comments don't need periods** - Only code statements

## Quick Check
- [ ] Does every `Decl` line end with `.`?
- [ ] Does every fact end with `.`?
- [ ] Does every rule end with `.`?
- [ ] Are periods at the END of statements (not middle)?

## Training Bias Origins
| Language | Terminator | Leads to Wrong Mangle |
|----------|------------|----------------------|
| Python | Newline | Forgetting period |
| JavaScript | Semicolon (optional) | Using `;` or omitting |
| Prolog | Period (similar!) | Should work, but AI forgets |
| SQL | Semicolon | Using `;` instead |

## Mnemonic
**"Every Mangle statement is a sentence. Sentences end with periods."**
