# Assignment vs Unification Errors
# Error Type: Using := or let outside transform blocks
# Expected: Syntax errors

# Test 1: := assignment in rule body
Decl value(V) bound [/number].
value(10).
double(Result) :- value(X), Result := fn:times(X, 2).  # ERROR: Use = not :=

# Test 2: let without pipe
Decl number(N) bound [/number].
number(5).
squared(S) :- number(X), let S = fn:times(X, X).  # ERROR: let only in transform

# Test 3: var keyword
Decl item(I) bound [/name].
item(/sword).
get_item(var X) :- item(X).  # ERROR: No var keyword

# Test 4: Multiple assignment styles mixed
Decl data(D) bound [/number].
data(42).
process(A, B, C) :-
  data(X),
  A = fn:plus(X, 1),      # Correct
  B := fn:times(X, 2),    # ERROR: Wrong operator
  let C = fn:minus(X, 1). # ERROR: let without pipe

# Test 5: += operator (imperative thinking)
Decl counter(C) bound [/number].
counter(0).
increment(New) :- counter(Old), New += 1.  # ERROR: No += in logic

# Test 6: Assignment in head
Decl input(I) bound [/number].
input(10).
output(X := fn:times(I, 2)) :- input(I).  # ERROR: Can't assign in head

# Test 7: Destructuring assignment (JS/Python style)
Decl pair(A, B) bound [/number, /number].
pair(1, 2).
process() :- pair(A, B), [X, Y] := [A, B].  # ERROR: Wrong syntax

# Test 8: let in regular rule (not transform)
Decl price(P) bound [/number].
price(100).
discounted(D) :-
  price(P),
  let Discount = fn:times(P, 0.1),  # ERROR: let needs transform context
  D = fn:minus(P, Discount).

# Test 9: Walrus operator (Python)
Decl value(V) bound [/number].
value(10).
check(X) :- value(V), (X := fn:plus(V, 5)) > 10.  # ERROR: No := operator
