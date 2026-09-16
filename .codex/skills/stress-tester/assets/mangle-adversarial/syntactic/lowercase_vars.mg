# Lowercase Variable Errors (Prolog Style)
# Error Type: Using lowercase for variables instead of UPPERCASE
# Expected: Variables treated as constants, unification failures

# Test 1: Basic Prolog-style lowercase variables
Decl parent(P, C) bound [/name, /name].
parent(/alice, /bob).
ancestor(x, y) :- parent(x, y).  # ERROR: x, y should be X, Y

# Test 2: Mixed case (some correct, some wrong)
Decl edge(From, To) bound [/name, /name].
edge(/a, /b).
path(X, y) :- edge(X, y).  # ERROR: y should be Y

# Test 3: Lowercase in aggregation
Decl value(V) bound [/number].
value(10).
value(20).
total(sum) :- value(X), sum = fn:Sum(X).  # ERROR: sum should be Sum

# Test 4: Lowercase in negation
Decl safe(S) bound [/name].
Decl dangerous(D) bound [/name].
dangerous(/fire).
result(x) :- safe(x), not dangerous(x).  # ERROR: x should be X

# Test 5: Lowercase in rule head only
Decl item(I) bound [/name].
item(/sword).
get_item(item) :- item(item).  # ERROR: Variable item shadows predicate name

# Test 6: Single letter lowercase (common in math)
Decl number(N) bound [/number].
number(5).
double(x, y) :- number(x), y = fn:times(x, 2).  # ERROR: x, y should be X, Y

# Test 7: Descriptive lowercase (looks intentional)
Decl user(U) bound [/name].
user(/alice).
active_user(username) :- user(username).  # ERROR: username should be Username

# Test 8: Lowercase in anonymous position (confusing)
Decl triple(A, B, C) bound [/number, /number, /number].
triple(1, 2, 3).
check(first, _, third) :- triple(first, _, third).  # ERROR: first, third should be uppercase
