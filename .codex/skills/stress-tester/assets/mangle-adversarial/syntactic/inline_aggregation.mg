# Inline Aggregation Errors (SQL Style)
# Error Type: Using SQL-style aggregation instead of pipe operator
# Expected: Syntax errors or incorrect aggregation

# Test 1: Direct SQL SUM in rule body
Decl item(I, Price) bound [/name, /number].
item(/sword, 100).
item(/shield, 50).
total(Sum) :- item(X, Price), Sum = sum(Price).  # ERROR: Should use pipe

# Test 2: COUNT without transform
Decl user(U) bound [/name].
user(/alice).
user(/bob).
user_count(Count) :- user(X), Count = count(X).  # ERROR: Wrong syntax

# Test 3: AVG inline
Decl score(S) bound [/number].
score(80).
score(90).
average(Avg) :- score(X), Avg = avg(X).  # ERROR: Should use pipe

# Test 4: MAX/MIN inline
Decl value(V) bound [/number].
value(5).
value(10).
value(3).
maximum(Max) :- value(X), Max = max(X).  # ERROR: Wrong syntax

# Test 5: SQL GROUP BY syntax
Decl sale(Product, Amount) bound [/name, /number].
sale(/widget, 10).
sale(/widget, 20).
sale(/gadget, 15).
product_total(Product, Sum) :-
  sale(Product, Amount)
  GROUP BY Product
  Sum = sum(Amount).  # ERROR: No SQL GROUP BY

# Test 6: HAVING clause
Decl transaction(ID, Amount) bound [/number, /number].
transaction(1, 100).
transaction(2, 200).
large_total(Sum) :-
  transaction(ID, Amount),
  Sum = sum(Amount),
  HAVING Sum > 150.  # ERROR: No HAVING in Mangle

# Test 7: Nested aggregation SQL style
Decl measurement(M) bound [/number].
measurement(10).
measurement(20).
avg_of_sum(Result) :-
  measurement(X),
  Result = avg(sum(X)).  # ERROR: Double aggregation without proper syntax

# Test 8: COUNT DISTINCT
Decl event(Type, User) bound [/name, /name].
event(/login, /alice).
event(/login, /alice).
event(/login, /bob).
unique_users(Count) :-
  event(Type, User),
  Count = count(distinct User).  # ERROR: Wrong distinct syntax

# Test 9: Aggregation in WHERE-like position
Decl number(N) bound [/number].
number(5).
number(10).
above_average(X) :-
  number(X),
  X > avg(number).  # ERROR: Can't use aggregation in comparison directly
