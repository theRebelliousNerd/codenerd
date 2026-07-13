# Unsafe Negation Tests
# Error Type: Variables in negated atoms not bound elsewhere first
# Expected: Safety violations, unbounded variable errors

# Test 1: Classic unbound negation
Decl person(P.Type<atom>).
Decl bad(B.Type<atom>).
person(/alice).
person(/bob).
bad(/charlie).
# ERROR: X is not bound before negation
good_person(X) :- not bad(X).

# Test 2: Negation as only source of variable
Decl blocked(B.Type<atom>).
blocked(/spam).
# ERROR: User appears only in negated atom
allowed_user(User) :- not blocked(User).

# Test 3: Multiple variables, only some bound
Decl edge(From.Type<atom>, To.Type<atom>).
Decl forbidden(F.Type<atom>, T.Type<atom>).
edge(/a, /b).
forbidden(/x, /y).
# ERROR: Y not bound before negation
safe_edge(X, Y) :- edge(X, Z), not forbidden(X, Y).

# Test 4: Nested negation with unbound vars
Decl active(A.Type<atom>).
Decl suspended(S.Type<atom>).
active(/user1).
suspended(/user2).
# ERROR: X not bound
complex(X) :- not (suspended(X), not active(X)).

# Test 5: Negation in aggregation context
Decl item(I.Type<atom>).
Decl excluded(E.Type<atom>).
item(/sword).
excluded(/poison).
# ERROR: X not bound before negation in pipe
count_allowed(Count) :-
  not excluded(X) |>
  do fn:group_by(),
  let Count = fn:Count(X).

# Test 6: Double negation with unbound variable
Decl valid(V.Type<atom>).
valid(/token1).
# ERROR: X never bound
weird(X) :- not not valid(X).

# Test 7: Negation before binding in conjunction
Decl user(U.Type<atom>).
Decl admin(A.Type<atom>).
user(/alice).
admin(/root).
# ERROR: X used in negation before being bound by user(X)
regular(X) :- not admin(X), user(X).

# Test 8: Negation with only constants (tricky - actually safe but confusing)
Decl flag(F.Type<atom>).
flag(/enabled).
# This is actually safe (no variables) but tests edge case
check() :- not flag(/disabled).

# Test 9: Cross-rule negation safety violation
Decl member(M.Type<atom>).
member(/alice).
helper(X) :- not member(X).  # ERROR: Unbound
caller() :- helper(/bob).

# Test 10: Negation in rule with multiple predicates
Decl node(N.Type<atom>).
Decl value(N.Type<atom>, V.Type<int>).
node(/n1).
value(/n1, 10).
# ERROR: V not bound before negation
check(N, V) :- node(N), not value(N, V).
