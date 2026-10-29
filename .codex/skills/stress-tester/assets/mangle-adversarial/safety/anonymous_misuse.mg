# Anonymous Variable Misuse Tests
# Error Type: Using _ when the value is actually needed
# Expected: Logic errors, missing data, incorrect results

# Test 1: Discarding needed value
Decl person(Name, Age) bound [/name, /number].
person(/alice, 30).
person(/bob, 25).
# ERROR: Age is discarded but needed in head
get_person(Name, Age) :- person(Name, _).

# Test 2: Using _ in head (usually invalid)
Decl data(D) bound [/number].
data(42).
# ERROR: Can't have anonymous in head
bad_head(_) :- data(X).

# Test 3: Multiple _ that should be same variable
Decl edge(From, To) bound [/name, /name].
edge(/a, /b).
edge(/b, /c).
# ERROR: the endpoints were discarded, so X is unbound (the two _ are
# independent wildcards and cannot name the loop anyway).
self_loop(X) :- edge(_, _).
# Should be: self_loop(X) :- edge(X, X).

# Test 4: Discarding in aggregation
Decl sale(Product, Amount) bound [/name, /number].
sale(/widget, 100).
sale(/widget, 200).
# WRONG ANSWER (valid program): Product discarded, so this counts every
# row in one group instead of totaling per product.
product_total(Total) :-
  sale(_, Amount) |>
  do fn:group_by(),
  let Total = fn:count().
# Should group: sale(Product, Amount) |> do fn:group_by(Product), ...

# Test 5: Anonymous in comparison
Decl value(V) bound [/number].
value(10).
value(20).
# ERROR: Can't compare with anonymous
check() :- value(_), _ > 15.

# Test 6: Using _ when building result
Decl item(ID, Name, Price) bound [/number, /name, /number].
item(1, /sword, 100).
# ERROR: Price discarded but needed
make_tuple(ID, Name, Price) :- item(ID, Name, _).

# Test 7: Anonymous in negation (subtle)
Decl user(U) bound [/name].
Decl blocked(B) bound [/name].
user(/alice).
blocked(/bob).
# ERROR: The _ doesn't bind anything, negation is wrong
bad_check(U) :- user(U), ! blocked(_).
# This says "not blocked for ANY value" which is wrong logic

# Test 8: Repeated _ thinking they match
Decl triple(A, B, C) bound [/number, /number, /number].
triple(1, 2, 3).
triple(5, 5, 7).
# ERROR: The two _ are independent, not the same
find_duplicate() :- triple(_, _, _).
# Should be: find_duplicate(X) :- triple(X, X, Y).

# Test 9: Anonymous in function call
Decl number(N) bound [/number].
number(10).
# ERROR: _ in a function argument is not a bound variable.
compute(R) :-
  number(X) |>
  do fn:group_by(),
  let R = fn:plus(_, 5).

# Test 10: Mixed anonymous and variables
Decl record(ID, Type, Value) bound [/number, /name, /number].
record(1, /sale, 100).
record(2, /refund, 50).
# WRONG ANSWER (valid program): Type discarded, so refunds pass the
# filter too. Mangle filters positionally, by naming the value:
# get_sales(ID, Value) :- record(ID, /sale, Value).
get_sales(ID, Value) :- record(ID, _, Value).

# Test 11: Correct use of _ for comparison
Decl entry(Key, Value) bound [/name, /number].
entry(/a, 10).
entry(/b, 20).
# CORRECT: Only care that entry exists with value > 15
has_large_value() :- entry(_, Value), Value > 15.

# Test 12: Incorrect _ in multi-predicate rule
Decl employee(E, Dept) bound [/name, /name].
Decl salary(E, Amount) bound [/name, /number].
employee(/alice, /sales).
salary(/alice, 50000).
# ERROR: Employee ID discarded in first predicate
get_salary(Dept, Salary) :- employee(_, Dept), salary(_, Salary).
# This doesn't link employee to their salary!
