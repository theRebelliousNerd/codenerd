# Cartesian Product Explosion Tests
# Error Type: Unfiltered joins creating massive intermediate results
# Expected: Performance degradation, memory exhaustion

# Test 1: Simple cartesian product
Decl table_a(A.Type<int>).
Decl table_b(B.Type<int>).
# Create 1000 x 1000 = 1,000,000 combinations
table_a(0).  # Imagine this repeated 1000 times
table_a(1).
table_a(2).
# ... (abbreviated)
table_b(0).  # Imagine this repeated 1000 times
table_b(1).
table_b(2).
# ... (abbreviated)
# ERROR: No filter, generates all combinations
bad_join(A, B) :- table_a(A), table_b(B).

# Test 2: Cartesian before filter
Decl user(ID.Type<int>, Name.Type<atom>).
Decl order(OrderID.Type<int>, UserID.Type<int>).
user(1, /alice).
user(2, /bob).
# Assume 10,000 users
order(1, 1).
order(2, 1).
# Assume 100,000 orders
# ERROR: Joins all users with all orders, then filters
# Should filter UserID first
inefficient(Name, OrderID) :-
  user(ID, Name),
  order(OrderID, UserID),
  ID = UserID.  # Filter comes too late

# Test 3: Triple join without filters
Decl table_x(X.Type<int>).
Decl table_y(Y.Type<int>).
Decl table_z(Z.Type<int>).
table_x(1).
table_x(2).
# ... 100 rows
table_y(1).
table_y(2).
# ... 100 rows
table_z(1).
table_z(2).
# ... 100 rows
# ERROR: 100 x 100 x 100 = 1,000,000 combinations
triple_join(X, Y, Z) :- table_x(X), table_y(Y), table_z(Z).

# Test 4: Self-join without conditions
Decl person(P.Type<atom>).
person(/alice).
person(/bob).
person(/charlie).
# ... 1000 people
# ERROR: All pairs of people (1000 x 1000 = 1,000,000)
all_pairs(P1, P2) :- person(P1), person(P2).
# Should be: all_pairs(P1, P2) :- person(P1), person(P2), P1 != P2.

# Test 5: Multiple unrelated predicates
Decl item(I.Type<atom>).
Decl location(L.Type<atom>).
Decl time(T.Type<int>).
item(/sword).
# ... 100 items
location(/shop).
# ... 50 locations
time(0).
# ... 24 hours
# ERROR: 100 x 50 x 24 = 120,000 combinations
unrelated(I, L, T) :- item(I), location(L), time(T).

# Test 6: Recursive cartesian explosion
Decl node(N.Type<atom>).
node(/n1).
node(/n2).
node(/n3).
# ... 100 nodes
# ERROR: All paths without cycle detection
# Generates exponential combinations
all_paths(From, To) :- node(From), node(To).
all_paths(From, To) :- all_paths(From, Mid), all_paths(Mid, To).

# Test 7: Aggregation on cartesian product
Decl product(P.Type<atom>).
Decl region(R.Type<atom>).
product(/widget).
# ... 1000 products
region(/north).
# ... 50 regions
# ERROR: Counts 1000 x 50 = 50,000 combinations
count_combos(C) :-
  product(P),
  region(R) |>
  do fn:group_by(),
  let C = fn:Count(P).

# Test 8: Correct join with filter first
Decl employee(EID.Type<int>, Dept.Type<atom>).
Decl salary(EID.Type<int>, Amount.Type<int>).
employee(1, /sales).
employee(2, /engineering).
# 10,000 employees
salary(1, 50000).
salary(2, 80000).
# 10,000 salary records
# CORRECT: Join condition prevents cartesian product
good_join(EID, Dept, Amount) :-
  employee(EID, Dept),
  salary(EID, Amount).  # EID matches, not cartesian

# Test 9: Filter then join (selectivity)
Decl transaction(TID.Type<int>, Amount.Type<int>).
Decl detail(TID.Type<int>, Description.Type<atom>).
transaction(1, 100).
# ... 1,000,000 transactions
detail(1, /purchase).
# ... 1,000,000 detail records
# WRONG: Cartesian first
bad_selective(TID, Amount, Desc) :-
  transaction(TID, Amount),
  detail(DID, Desc),
  TID = DID.
# CORRECT: Bind first
good_selective(TID, Amount, Desc) :-
  Amount > 1000,  # Filter first (if possible)
  transaction(TID, Amount),
  detail(TID, Desc).  # Join with bound TID

# Test 10: Cross product in recursion
Decl edge(From.Type<atom>, To.Type<atom>).
edge(/a, /b).
edge(/b, /c).
# Small graph
# ERROR: Exponential path explosion
# Every path connects with every other path
bad_paths(From, To) :- edge(From, To).
bad_paths(From, To) :-
  bad_paths(From, Mid1),
  bad_paths(Mid2, To).  # No connection between Mid1 and Mid2!
# Cartesian between all paths

# Test 11: Multiple aggregations with cross product
Decl sales(Region.Type<atom>, Amount.Type<int>).
Decl costs(Region.Type<atom>, Cost.Type<int>).
sales(/north, 1000).
costs(/north, 500).
# ... many regions
# ERROR: If not grouped properly, cartesian explosion
bad_profit(Region, Profit) :-
  sales(Region, S),
  costs(Region, C),
  Profit = fn:minus(S, C).  # OK if regions match
# But wrong if written as:
bad_total(Total) :-
  sales(R1, S),
  costs(R2, C),  # Different variables = cartesian!
  Total = fn:minus(S, C).

# Test 12: Nested loops equivalent
Decl outer(O.Type<int>).
Decl inner(I.Type<int>).
outer(1).
# ... 1000 values
inner(1).
# ... 1000 values
# ERROR: Nested loop, O(n²) complexity
nested(O, I, Product) :-
  outer(O),
  inner(I),
  Product = fn:times(O, I).
# Generates 1,000,000 products

# Test 13: Correct filtered cartesian (intentional)
Decl vertex(V.Type<atom>).
vertex(/v1).
vertex(/v2).
vertex(/v3).
# CORRECT: Want all pairs but with condition
distinct_pairs(V1, V2) :-
  vertex(V1),
  vertex(V2),
  V1 != V2.  # Intentional cartesian, but filtered
