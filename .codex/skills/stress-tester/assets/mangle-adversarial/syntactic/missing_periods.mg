# Missing Period Errors
# Error Type: Forgetting to end statements with periods
# Expected: Parse errors, rules bleeding into each other

# Test 1: Missing period after fact
Decl item(I) bound [/name].
item(/sword)  # ERROR: Missing period
item(/shield).

# Test 2: Missing period after rule
Decl parent(P, C) bound [/name, /name].
Decl ancestor(A, D) bound [/name, /name].
parent(/alice, /bob).
ancestor(X, Y) :- parent(X, Y)  # ERROR: Missing period
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).

# Test 3: Missing period after declaration
Decl edge(From.Type<atom>, To.Type<atom>)  # ERROR: Missing period
edge(/a, /b).

# Test 4: Multiple missing periods
Decl value(V.Type<int>)  # ERROR: Missing period
value(10)  # ERROR: Missing period
value(20)  # ERROR: Missing period

# Test 5: Missing period in transform
Decl number(N) bound [/number].
number(1).
number(2).
total(Sum) :-
  number(X) |> do fn:group_by(), let Sum = fn:Sum(X)  # ERROR: Missing period

# Test 6: Missing period before comment
Decl status(S) bound [/name].
status(/active)  # This is a status  # ERROR: Missing period before comment

# Test 7: Newline instead of period (thinking newline is delimiter)
Decl node(N) bound [/name].
node(/n1)
node(/n2)  # ERROR: Each needs a period
node(/n3)

# Test 8: Missing period in multi-line rule
Decl path(From, To) bound [/name, /name].
Decl connected(A, B) bound [/name, /name].
connected(/x, /y).
path(X, Y) :-
  connected(X, Y)  # ERROR: Missing period even in multi-line
