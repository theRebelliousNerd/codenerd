# Negation Order Tests
# Error Type: Using negation before binding all its variables
# Expected: Safety violations due to incorrect atom ordering
# DIALECT NOTE: this engine checks binding EXISTENCE, not order: a variable
# bound anywhere in a positive body atom satisfies !-safety even when the
# binding comes textually after the negation. The wrong-order-but-bound
# rules below are therefore VALID here (marked SMELL, not ERROR); the file
# still fails analysis on the genuinely unbound variables.

# Test 1: Negation before positive binding
Decl user(U) bound [/name].
Decl blocked(B) bound [/name].
user(/alice).
blocked(/bob).
# SMELL (valid here): !blocked(X) precedes the user(X) binding
bad_order(X) :- ! blocked(X), user(X).

# Test 2: Correct order for comparison
Decl person(P) bound [/name].
Decl banned(B) bound [/name].
person(/alice).
banned(/charlie).
# CORRECT: person(X) binds X before negation
good_order(X) :- person(X), ! banned(X).

# Test 3: Multiple negations, wrong order
Decl active(A) bound [/name].
Decl suspended(S) bound [/name].
Decl deleted(D) bound [/name].
active(/user1).
suspended(/user2).
deleted(/user3).
# SMELL (valid here): both negations precede the active(X) binding
wrong(X) :- ! suspended(X), ! deleted(X), active(X).

# Test 4: Partial binding before negation
Decl edge(From, To) bound [/name, /name].
Decl blocked_edge(F, T) bound [/name, /name].
edge(/a, /b).
blocked_edge(/x, /y).
# SMELL (valid here): Y is bound by edge(Y, W), textually after the negation
partial(X, Y) :- edge(X, Z), ! blocked_edge(X, Y), edge(Y, W).

# Test 5: Negation interleaved incorrectly
Decl node(N) bound [/name].
Decl bad_node(B) bound [/name].
Decl value(N, V) bound [/name, /number].
node(/n1).
bad_node(/n2).
value(/n1, 10).
# ERROR: not bad_node(N) before value(N, V) binds V
interleaved(N, V) :- node(N), ! bad_node(N), value(N, V).
# Actually this one is SAFE (N is bound by node(N))
# Better example:
bad_interleaved(N, V) :- ! bad_node(N), node(N), value(N, V).

# Test 6: Function call doesn't bind for negation
Decl item(I, Price) bound [/name, /number].
item(/sword, 100).
# ERROR: Expensive is not bound anywhere, so the negation is unsafe. (An
# equation such as Expensive = fn:mult(Price, 2) would bind it; the original
# fixture used the nonexistent fn:times.)
wrong_func(I, Expensive) :-
  item(I, Price),
  !item(I, Expensive).

# Test 7: Negation in aggregation with wrong order
Decl transaction(ID, Type) bound [/number, /name].
Decl ignored_type(T) bound [/name].
transaction(1, /sale).
ignored_type(/refund).
# ERROR: Type not bound before negation in pipe
bad_agg(Count) :-
  ! ignored_type(Type),
  transaction(ID, Type) |>
  do fn:group_by(),
  let Count = fn:count(ID).

# Test 8: Negation of conjunction (complex)
Decl vertex(V) bound [/name].
Decl has_edge(V) bound [/name].
vertex(/v1).
has_edge(/v2).
# Mangle has no !(...) grouping, so a negated conjunction needs a helper.
# CORRECT: X bound before the negation; Y stays inside the helper.
has_conn(X) :- has_edge(X), vertex(Y).
isolated(X) :- vertex(X), !has_conn(X).
# ERROR shape: without vertex(X) first, X would be unbound here.
isolated_bad(X) :- !has_conn(X).

# Test 9: Correct complex negation
# (person/1 already declared in Test 2; a second Decl is an error)
Decl friend(P1, P2) bound [/name, /name].
person(/alice).
person(/bob).
friend(/alice, /bob).
# CORRECT: Both X and Y bound before negation
not_friends(X, Y) :- person(X), person(Y), ! friend(X, Y).

# Test 10: Negation before join
Decl employee(E) bound [/name].
Decl department(E, D) bound [/name, /name].
Decl banned_dept(D) bound [/name].
employee(/alice).
department(/alice, /sales).
banned_dept(/fraud).
# SMELL (valid here): D is bound by department(E, D), textually after the negation
wrong_join(E, D) :- employee(E), ! banned_dept(D), department(E, D).
