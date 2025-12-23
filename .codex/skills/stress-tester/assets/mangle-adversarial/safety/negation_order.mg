# Negation Order Tests
# Error Type: Using negation before binding all its variables
# Expected: Safety violations due to incorrect atom ordering

# Test 1: Negation before positive binding
Decl user(U.Type<atom>).
Decl blocked(B.Type<atom>).
user(/alice).
blocked(/bob).
# ERROR: not blocked(X) comes before user(X) binds X
bad_order(X) :- not blocked(X), user(X).

# Test 2: Correct order for comparison
Decl person(P.Type<atom>).
Decl banned(B.Type<atom>).
person(/alice).
banned(/charlie).
# CORRECT: person(X) binds X before negation
good_order(X) :- person(X), not banned(X).

# Test 3: Multiple negations, wrong order
Decl active(A.Type<atom>).
Decl suspended(S.Type<atom>).
Decl deleted(D.Type<atom>).
active(/user1).
suspended(/user2).
deleted(/user3).
# ERROR: Both negations come before binding
wrong(X) :- not suspended(X), not deleted(X), active(X).

# Test 4: Partial binding before negation
Decl edge(From.Type<atom>, To.Type<atom>).
Decl blocked_edge(F.Type<atom>, T.Type<atom>).
edge(/a, /b).
blocked_edge(/x, /y).
# ERROR: Y not bound before negation
partial(X, Y) :- edge(X, Z), not blocked_edge(X, Y), edge(Y, W).

# Test 5: Negation interleaved incorrectly
Decl node(N.Type<atom>).
Decl bad_node(B.Type<atom>).
Decl value(N.Type<atom>, V.Type<int>).
node(/n1).
bad_node(/n2).
value(/n1, 10).
# ERROR: not bad_node(N) before value(N, V) binds V
interleaved(N, V) :- node(N), not bad_node(N), value(N, V).
# Actually this one is SAFE (N is bound by node(N))
# Better example:
bad_interleaved(N, V) :- not bad_node(N), node(N), value(N, V).

# Test 6: Function call doesn't bind for negation
Decl item(I.Type<atom>, Price.Type<int>).
item(/sword, 100).
# ERROR: Expensive not bound before negation
wrong_func(I, Expensive) :-
  item(I, Price),
  not item(I, Expensive),
  Expensive = fn:times(Price, 2).

# Test 7: Negation in aggregation with wrong order
Decl transaction(ID.Type<int>, Type.Type<atom>).
Decl ignored_type(T.Type<atom>).
transaction(1, /sale).
ignored_type(/refund).
# ERROR: Type not bound before negation in pipe
bad_agg(Count) :-
  not ignored_type(Type),
  transaction(ID, Type) |>
  do fn:group_by(),
  let Count = fn:Count(ID).

# Test 8: Negation of conjunction (complex)
Decl vertex(V.Type<atom>).
Decl has_edge(V.Type<atom>).
vertex(/v1).
has_edge(/v2).
# ERROR: Variables in negated conjunction not bound
isolated(X) :- vertex(X), not (has_edge(X), vertex(Y)).

# Test 9: Correct complex negation
Decl person(P.Type<atom>).
Decl friend(P1.Type<atom>, P2.Type<atom>).
person(/alice).
person(/bob).
friend(/alice, /bob).
# CORRECT: Both X and Y bound before negation
not_friends(X, Y) :- person(X), person(Y), not friend(X, Y).

# Test 10: Negation before join
Decl employee(E.Type<atom>).
Decl department(E.Type<atom>, D.Type<atom>).
Decl banned_dept(D.Type<atom>).
employee(/alice).
department(/alice, /sales).
banned_dept(/fraud).
# ERROR: D not bound before negation
wrong_join(E, D) :- employee(E), not banned_dept(D), department(E, D).
