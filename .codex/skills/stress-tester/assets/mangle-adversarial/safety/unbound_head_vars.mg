# Unbound Head Variable Tests
# Error Type: Variables in rule head not appearing in positive body atoms
# Expected: Safety errors, infinite/undefined results

# Test 1: Variable in head not in body at all
Decl input(I.Type<int>).
input(10).
# ERROR: Y appears in head but not in body
bad_rule(X, Y) :- input(X).

# Test 2: Variable only in negated atom
Decl person(P.Type<atom>).
Decl excluded(E.Type<atom>).
person(/alice).
excluded(/bob).
# ERROR: X only appears in negation
unsafe(X) :- person(Y), not excluded(X).

# Test 3: Multiple unbound head variables
Decl data(D.Type<int>).
data(42).
# ERROR: A, B, C all unbound
generate(A, B, C) :- data(D).

# Test 4: Variable in head, only in function call
Decl number(N.Type<int>).
number(5).
# ERROR: Result in head but not bound in body (function doesn't bind)
compute(X, Result) :- number(X), Result = fn:plus(X, Unbound).

# Test 5: Aggregation with unbound head variable
Decl value(V.Type<int>).
value(10).
value(20).
# ERROR: X in head but not in grouping
bad_agg(X, Sum) :-
  value(V) |>
  do fn:group_by(),
  let Sum = fn:Sum(V).

# Test 6: Unbound variable in complex head structure
Decl item(I.Type<atom>).
item(/sword).
# ERROR: Price not bound
make_pair(I, Price) :- item(I).

# Test 7: Some variables bound, some not
Decl edge(From.Type<atom>, To.Type<atom>).
edge(/a, /b).
# ERROR: Z unbound
path(X, Y, Z) :- edge(X, Y).

# Test 8: Unbound in nested structure
Decl config(C.Type<atom>).
config(/setting).
# ERROR: Value unbound
make_config(C, { /key: Value }) :- config(C).

# Test 9: Variable only in comparison
Decl threshold(T.Type<int>).
threshold(100).
# ERROR: X only in comparison, not bound
check(X, T) :- threshold(T), X > T.

# Test 10: Cross-predicate unbound
Decl source(S.Type<atom>).
source(/data).
helper(X) :- source(S).  # ERROR: X unbound
caller(R) :- helper(R).
