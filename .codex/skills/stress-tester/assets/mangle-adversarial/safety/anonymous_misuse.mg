# Anonymous Variable Misuse Tests
# Error Type: Using _ when the value is actually needed
# Expected: Logic errors, missing data, incorrect results

# Test 1: Discarding needed value
Decl person(Name.Type<atom>, Age.Type<int>).
person(/alice, 30).
person(/bob, 25).
# ERROR: Age is discarded but needed in head
get_person(Name, Age) :- person(Name, _).

# Test 2: Using _ in head (usually invalid)
Decl data(D.Type<int>).
data(42).
# ERROR: Can't have anonymous in head
bad_head(_) :- data(X).

# Test 3: Multiple _ that should be same variable
Decl edge(From.Type<atom>, To.Type<atom>).
edge(/a, /b).
edge(/b, /c).
# ERROR: Two _ are independent, won't match
self_loop(X) :- edge(_, _), X = /found.
# Should be: self_loop(X) :- edge(X, X).

# Test 4: Discarding in aggregation
Decl sale(Product.Type<atom>, Amount.Type<int>).
sale(/widget, 100).
sale(/widget, 200).
# ERROR: Product discarded but needed for grouping
product_total(Amount) :-
  sale(_, Amount) |>
  do fn:group_by(),
  let Total = fn:Sum(Amount).

# Test 5: Anonymous in comparison
Decl value(V.Type<int>).
value(10).
value(20).
# ERROR: Can't compare with anonymous
check() :- value(_), _ > 15.

# Test 6: Using _ when building result
Decl item(ID.Type<int>, Name.Type<atom>, Price.Type<int>).
item(1, /sword, 100).
# ERROR: Price discarded but needed
make_tuple(ID, Name, Price) :- item(ID, Name, _).

# Test 7: Anonymous in negation (subtle)
Decl user(U.Type<atom>).
Decl blocked(B.Type<atom>).
user(/alice).
blocked(/bob).
# ERROR: The _ doesn't bind anything, negation is wrong
bad_check(U) :- user(U), not blocked(_).
# This says "not blocked for ANY value" which is wrong logic

# Test 8: Repeated _ thinking they match
Decl triple(A.Type<int>, B.Type<int>, C.Type<int>).
triple(1, 2, 3).
triple(5, 5, 7).
# ERROR: The two _ are independent, not the same
find_duplicate() :- triple(_, _, _).
# Should be: find_duplicate(X) :- triple(X, X, Y).

# Test 9: Anonymous in function call
Decl number(N.Type<int>).
number(10).
# ERROR: Can't use _ in function argument
compute(R) :- number(X), R = fn:plus(_, 5).

# Test 10: Mixed anonymous and variables
Decl record(ID.Type<int>, Type.Type<atom>, Value.Type<int>).
record(1, /sale, 100).
record(2, /refund, 50).
# ERROR: Type is discarded but needed for filtering
get_sales(ID, Value) :- record(ID, _, Value), _ = /sale.
# Should be: get_sales(ID, Value) :- record(ID, Type, Value), Type = /sale.

# Test 11: Correct use of _ for comparison
Decl entry(Key.Type<atom>, Value.Type<int>).
entry(/a, 10).
entry(/b, 20).
# CORRECT: Only care that entry exists with value > 15
has_large_value() :- entry(_, Value), Value > 15.

# Test 12: Incorrect _ in multi-predicate rule
Decl employee(E.Type<atom>, Dept.Type<atom>).
Decl salary(E.Type<atom>, Amount.Type<int>).
employee(/alice, /sales).
salary(/alice, 50000).
# ERROR: Employee ID discarded in first predicate
get_salary(Dept, Salary) :- employee(_, Dept), salary(_, Salary).
# This doesn't link employee to their salary!
