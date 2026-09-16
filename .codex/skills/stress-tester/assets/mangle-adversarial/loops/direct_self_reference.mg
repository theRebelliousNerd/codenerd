# Direct Self-Reference Loop Tests
# Error Type: Rules that directly reference themselves infinitely
# Expected: Infinite recursion, stack overflow, non-termination
# DIALECT NOTE: non-termination is a RUNTIME property: every rule below is a
# VALID program that parses and analyzes cleanly. These files pin well-
# formedness only; the live danger (unbounded derivation) is contained by
# executor timeouts, not by static rejection.

# Test 1: Simplest infinite loop
Decl loop(L) bound [/number].
loop(0).
# ERROR: Infinite - loop(X) generates loop(X+1) forever
loop(X) :- loop(Y), X = fn:plus(Y, 1).

# Test 2: Unconditional self-reference
Decl infinite(I) bound [/name].
# ERROR: No base case, infinite expansion
infinite(X) :- infinite(X).

# Test 3: Self-reference with transformation
Decl grow(G) bound [/number].
grow(1).
# ERROR: Will generate grow(2), grow(4), grow(8)... infinitely
grow(X) :- grow(Y), X = fn:mult(Y, 2).

# Test 4: Self-reference through list building
Decl sequence(S) bound [/list].
sequence([]).
# ERROR: Builds longer and longer lists infinitely
sequence(L) :- sequence(Tail), L = fn:list(/item, Tail).

# Test 5: Self-reference with multiple bases
Decl multi(M) bound [/number].
multi(0).
multi(1).
# ERROR: Both bases generate infinite derivations
multi(X) :- multi(Y), X = fn:plus(Y, 2).

# Test 6: Self-reference with filtering (still infinite)
Decl filtered(F) bound [/number].
filtered(0).
# ERROR: Even with condition, generates infinitely many
filtered(X) :- filtered(Y), X = fn:plus(Y, 1), X < 1000000.

# Test 7: Mutual self-reference (A -> B -> A)
Decl ping(P) bound [/number].
Decl pong(P) bound [/number].
ping(0).
# ERROR: ping -> pong -> ping -> pong... infinite
ping(X) :- pong(Y), X = fn:plus(Y, 1).
pong(X) :- ping(Y), X = fn:plus(Y, 1).

# Test 8: Self-reference with accumulator pattern (wrong)
Decl counter(C) bound [/number].
counter(0).
# ERROR: Tries to count up infinitely
counter(N) :- counter(M), N = fn:plus(M, 1), N < 100.
# Even with bound, generates all values < 100 then tries to continue

# Test 9: Self-reference in aggregation
Decl expanding(E) bound [/number].
expanding(1).
# ERROR: Expands during aggregation computation
expanding(X) :- expanding(Y), X = fn:mult(Y, 2).
total(T) :-
  expanding(E) |>
  do fn:group_by(),
  let T = fn:count().

# Test 10: Correct bounded recursion (for comparison)
Decl number(N) bound [/number].
number(0).
number(1).
number(2).
number(3).
# CORRECT: Finite domain, computes transitive closure
sum_below(X, S) :- number(X), S = X.
sum_below(X, S) :- number(X), number(Y), Y < X, sum_below(Y, S2), S = fn:plus(S2, X).
# This terminates because number/1 is finite

# Test 11: Infinite descent
Decl descend(D) bound [/number].
descend(100).
# ERROR: Goes down infinitely (100, 99, 98, ...)
descend(X) :- descend(Y), X = fn:minus(Y, 1).

# Test 12: Self-reference with list building (no string-concat builtin exists)
Decl build_string(S) bound [/list].
build_string(["a"]).
# ERROR: Builds ["a"], [["a"], "a"], ... infinitely
build_string(S) :- build_string(Prev), S = fn:list(Prev, "a").
