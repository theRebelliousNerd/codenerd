# Lowercase Variable Errors (Prolog Style)
# Error Type: Using lowercase for variables instead of UPPERCASE
# Expected: Variables treated as constants, unification failures

# Test 1: Basic Prolog-style lowercase variables
Decl parent(P.Type<atom>, C.Type<atom>).
parent(/alice, /bob).
ancestor(x, y) :- parent(x, y).  # ERROR: x, y should be X, Y

# Test 2: Mixed case (some correct, some wrong)
Decl edge(From.Type<atom>, To.Type<atom>).
edge(/a, /b).
path(X, y) :- edge(X, y).  # ERROR: y should be Y

# Test 3: Lowercase in aggregation
Decl value(V.Type<int>).
value(10).
value(20).
total(sum) :- value(X), sum = fn:Sum(X).  # ERROR: sum should be Sum

# Test 4: Lowercase in negation
Decl safe(S.Type<atom>).
Decl dangerous(D.Type<atom>).
dangerous(/fire).
result(x) :- safe(x), not dangerous(x).  # ERROR: x should be X

# Test 5: Lowercase in rule head only
Decl item(I.Type<atom>).
item(/sword).
get_item(item) :- item(item).  # ERROR: Variable item shadows predicate name

# Test 6: Single letter lowercase (common in math)
Decl number(N.Type<int>).
number(5).
double(x, y) :- number(x), y = fn:times(x, 2).  # ERROR: x, y should be X, Y

# Test 7: Descriptive lowercase (looks intentional)
Decl user(U.Type<atom>).
user(/alice).
active_user(username) :- user(username).  # ERROR: username should be Username

# Test 8: Lowercase in anonymous position (confusing)
Decl triple(A.Type<int>, B.Type<int>, C.Type<int>).
triple(1, 2, 3).
check(first, _, third) :- triple(first, _, third).  # ERROR: first, third should be uppercase
