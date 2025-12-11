# Anti-Pattern: Prolog Thinking

## Category
Syntax Translation Error (Prolog Bias)

## Description
Attempting to use Prolog syntax, conventions, and patterns that don't translate to Mangle.

---

## Anti-Pattern 1: Lowercase Variables

### Wrong Approach
```prolog
ancestor(x, y) :- parent(x, y).
ancestor(x, y) :- parent(x, z), ancestor(z, y).
```

### Why It Fails
Mangle requires **UPPERCASE** variables. Lowercase identifiers are treated as constants or atoms (when prefixed with `/`).

### Correct Mangle Way
```mangle
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Y) :- parent(X, Z), ancestor(Z, Y).
```

**Rule:** Variables = UPPERCASE. Constants/Atoms = `/lowercase`.

---

## Anti-Pattern 2: Atom Syntax (No Slash)

### Wrong Approach
```prolog
status(user123, active).
category(item42, books).
```

### Why It Fails
Mangle treats unquoted lowercase identifiers as **string variables** unless prefixed with `/`.

### Correct Mangle Way
```mangle
status(/user123, /active).
category(/item42, /books).
```

**Rule:** Atoms (constants/symbols) require `/` prefix. Strings need quotes: `"hello"`.

---

## Anti-Pattern 3: Anonymous Variable Anywhere

### Wrong Approach
```prolog
has_child(Parent) :- parent(Parent, _).
```

Then later trying to use `_` to refer to the same value:

```prolog
has_child(Parent) :- parent(Parent, _), age(_, Age).  % WRONG
```

### Why It Fails
In Prolog, each `_` is truly anonymous. In Mangle, **each `_` is independent** and means "I don't care about this value." You cannot reuse `_` to refer to the same binding.

### Correct Mangle Way
```mangle
# If you truly don't need the child's name:
has_child(Parent) :- parent(Parent, _).

# If you need to reference the child later:
has_child(Parent) :-
    parent(Parent, Child),
    age(Child, Age).
```

---

## Anti-Pattern 4: Cut Operator (!)

### Wrong Approach
```prolog
max(X, Y, X) :- X >= Y, !.
max(X, Y, Y).
```

### Why It Fails
Mangle has **no cut operator**. Datalog semantics are declarative and deterministic - all rules that match will derive facts.

### Correct Mangle Way
```mangle
# Use explicit conditions in each rule
max_value(X, Y, X) :- X >= Y.
max_value(X, Y, Y) :- Y > X.

# Or use conditional logic in a single rule
max_value(X, Y, Max) :-
    X >= Y,
    Max = X.

max_value(X, Y, Max) :-
    Y > X,
    Max = Y.
```

**Note:** Both rules may fire. If you need exactly one answer, structure your logic to make rules mutually exclusive.

---

## Anti-Pattern 5: Assert/Retract Dynamic Facts

### Wrong Approach
```prolog
add_fact(X) :- assert(dynamic_data(X)).
remove_fact(X) :- retract(dynamic_data(X)).
```

### Why It Fails
Mangle rules are **pure and declarative**. You cannot assert or retract facts from within Mangle code.

### Correct Mangle Way
```mangle
# Fact manipulation must happen in Go:
# store.Add(newFact)
# store.Retract(oldFact)

# In Mangle, just define what's true:
derived_fact(X) :- base_fact(X), condition(X).
```

**Design Principle:** Mangle derives facts from existing facts. Mutation happens at the engine level (Go API), not in rules.

---

## Anti-Pattern 6: Findall/Bagof/Setof

### Wrong Approach
```prolog
all_children(Parent, Children) :-
    findall(Child, parent(Parent, Child), Children).
```

### Why It Fails
Mangle has no `findall`, `bagof`, or `setof` meta-predicates.

### Correct Mangle Way
```mangle
# Use aggregation with transforms:
all_children(Parent, Children) :-
    parent(Parent, Child)
    |> do fn:group_by(Parent)
    |> let Children = fn:List(Child).

# Or just derive individual facts:
child_of(Parent, Child) :- parent(Parent, Child).
```

**Explanation:** Mangle's transform system (`|>`) replaces Prolog's meta-predicates for collecting results.

---

## Anti-Pattern 7: Is Operator for Arithmetic

### Wrong Approach
```prolog
double(X, Y) :- Y is X * 2.
```

### Why It Fails
Mangle uses **unification and function calls**, not the `is` operator.

### Correct Mangle Way
```mangle
# Use fn: built-in functions
double(X, Y) :-
    Y = fn:times(X, 2).

# Or inline arithmetic (if supported):
double(X, Y) :- Y = X * 2.
```

**Note:** Check Mangle's builtin package for available arithmetic functions.

---

## Anti-Pattern 8: List Syntax [H|T]

### Wrong Approach
```prolog
first([H|T], H).
rest([H|T], T).
```

### Why It Fails
Mangle uses **:match_cons** for list destructuring, not `[H|T]` syntax.

### Correct Mangle Way
```mangle
# Use :match_cons(List, Head, Tail)
first(List, Head) :- :match_cons(List, Head, _).

rest(List, Tail) :- :match_cons(List, _, Tail).

# Example with recursion:
list_length([], 0).
list_length(List, Len) :-
    :match_cons(List, _, Tail),
    list_length(Tail, TailLen),
    Len = fn:plus(TailLen, 1).
```

---

## Anti-Pattern 9: DCG (Definite Clause Grammar)

### Wrong Approach
```prolog
sentence --> noun_phrase, verb_phrase.
noun_phrase --> determiner, noun.
```

### Why It Fails
Mangle has **no DCG support**. It's not a parser generator.

### Correct Mangle Way
```mangle
# Model grammar as predicates:
sentence(S) :-
    noun_phrase(NP),
    verb_phrase(VP),
    append(NP, VP, S).

# Or parse in Go and load facts:
# parse_result(/s1, "the cat sat").
```

---

## Anti-Pattern 10: Operator Definitions

### Wrong Approach
```prolog
:- op(500, yfx, 'likes').
john likes mary.
```

### Why It Fails
Mangle has **no operator definition system**. All predicates use standard function call syntax.

### Correct Mangle Way
```mangle
# Use normal predicate syntax:
likes(/john, /mary).
```

---

## Anti-Pattern 11: If-Then-Else ( -> ; )

### Wrong Approach
```prolog
sign(X, positive) :- X > 0 -> true ; fail.
sign(X, negative) :- X < 0.
```

### Why It Fails
Mangle has no `->` or `;` operators for if-then-else.

### Correct Mangle Way
```mangle
# Use separate rules with guards:
sign(X, /positive) :- X > 0.
sign(X, /negative) :- X < 0.
sign(X, /zero) :- X = 0.
```

---

## Anti-Pattern 12: Module System

### Wrong Approach
```prolog
:- module(mymodule, [exported_pred/2]).
```

### Why It Fails
Mangle uses **package imports**, not Prolog modules.

### Correct Mangle Way
```mangle
# At the top of your .gl file:
package mypackage

use github.com/google/mangle/builtin

# All predicates in the file are in this package
```

---

## Anti-Pattern 13: Trace/Debug Predicates

### Wrong Approach
```prolog
debug_rule(X) :- trace, my_rule(X).
```

### Why It Fails
No `trace`, `spy`, or `nospy`. Debugging happens via Go logging or query inspection.

### Correct Mangle Way
```mangle
# Add explicit debug predicates that derive facts:
debug_step(X, "entering rule") :- my_rule(X).

# Then query these facts after evaluation:
# Query: debug_step(X, Msg)?
```

---

## Anti-Pattern 14: Arithmetic Constraints (CLP)

### Wrong Approach
```prolog
:- use_module(library(clpfd)).
X #> 5, X #< 10.
```

### Why It Fails
Mangle is not a constraint solver. It's pure Datalog with stratified negation.

### Correct Mangle Way
```mangle
# Use explicit comparisons:
in_range(X) :-
    number(X),
    X > 5,
    X < 10.
```

**Note:** For constraint solving, use an external solver and load results as facts.

---

## Key Differences: Prolog vs Mangle

| Prolog | Mangle |
|--------|--------|
| Lowercase variables | UPPERCASE variables |
| `atom` | `/atom` |
| `[H\|T]` | `:match_cons(List, H, T)` |
| `!` (cut) | No cut - all rules fire |
| `is` for arithmetic | `fn:` functions or `=` |
| `assert/retract` | Go API: `store.Add/Retract` |
| `findall/bagof/setof` | `\|> do fn:group_by()` |
| `->` ; `;` | Separate rules with guards |
| Module system | Package imports |
| DCG | No DCG - parse in Go |
| CLP | No constraints - external solver |

---

## Migration Checklist

When translating Prolog to Mangle:

- [ ] Convert all variables to UPPERCASE
- [ ] Add `/` prefix to all atoms
- [ ] Replace `[H|T]` with `:match_cons(List, H, T)`
- [ ] Remove cut (`!`) - make rules mutually exclusive
- [ ] Replace `is` with `fn:` arithmetic functions
- [ ] Remove `assert/retract` - plan for Go API usage
- [ ] Replace `findall` with `|> do fn:group_by()`
- [ ] Replace `->` ; with multiple guarded rules
- [ ] Remove module declarations - use package imports
- [ ] Remove DCG - parse in Go or model as predicates
- [ ] Remove CLP - use external solver or explicit comparisons
- [ ] Replace trace/debug with explicit debug predicates
