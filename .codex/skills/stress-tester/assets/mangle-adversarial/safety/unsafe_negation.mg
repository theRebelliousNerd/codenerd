# Unsafe Negation Tests
# Error Type: Variables in negated atoms not bound elsewhere first
# Expected: Safety violations, unbounded variable errors

# Test 1: Classic unbound negation
Decl person(P) bound [/name].
Decl bad(B) bound [/name].
person(/alice).
person(/bob).
bad(/charlie).
# ERROR: X is not bound before negation
good_person(X) :- ! bad(X).

# Test 2: Negation as only source of variable
Decl blocked(B) bound [/name].
blocked(/spam).
# ERROR: User appears only in negated atom
allowed_user(User) :- ! blocked(User).

# Test 3: Multiple variables, only some bound
Decl edge(From, To) bound [/name, /name].
Decl forbidden(F, T) bound [/name, /name].
edge(/a, /b).
forbidden(/x, /y).
# ERROR: Y not bound before negation
safe_edge(X, Y) :- edge(X, Z), ! forbidden(X, Y).

# Test 4: Nested negation with unbound vars
Decl active(A) bound [/name].
Decl suspended(S) bound [/name].
active(/user1).
suspended(/user2).
# Mangle has no !(...) grouping: conjunction under negation needs a helper.
# ERROR: X is not bound (a negated helper is still not a binding source)
nx_helper(X) :- suspended(X), !active(X).
complex(X) :- !nx_helper(X).

# Test 5: Negation in aggregation context
Decl item(I) bound [/name].
Decl excluded(E) bound [/name].
item(/sword).
excluded(/poison).
# ERROR: a negated atom cannot be a transform source (X unbound, no body)
count_allowed(Count) :-
  !excluded(X) |>
  do fn:group_by(),
  let Count = fn:count(X).
# CORRECT: bind positively first, then filter
count_allowed_fixed(Count) :-
  item(X), !excluded(X) |>
  do fn:group_by(),
  let Count = fn:count().

# Test 6: Double negation with unbound variable
Decl valid(V) bound [/name].
valid(/token1).
# Mangle has no !! double negation (parse error); the safety violation is
# the same unbound-through-negation shape as Test 1.
# ERROR: X never bound
weird(X) :- !valid(X).

# Test 7: Negation before binding in conjunction
Decl user(U) bound [/name].
Decl admin(A) bound [/name].
user(/alice).
admin(/root).
# ERROR: X used in negation before being bound by user(X)
regular(X) :- ! admin(X), user(X).

# Test 8: Negation with only constants (tricky - actually safe but confusing)
Decl flag(F) bound [/name].
flag(/enabled).
# This is actually safe (no variables) but tests edge case
check() :- ! flag(/disabled).

# Test 9: Cross-rule negation safety violation
Decl member(M) bound [/name].
member(/alice).
helper(X) :- ! member(X).  # ERROR: Unbound
caller() :- helper(/bob).

# Test 10: Negation in rule with multiple predicates
Decl node(N) bound [/name].
Decl value(N, V) bound [/name, /number].
node(/n1).
value(/n1, 10).
# ERROR: V not bound before negation
value_check(N, V) :- node(N), !value(N, V).
