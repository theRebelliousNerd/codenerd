# Stratification Cycle Tests
# Error Type: Recursion through negation, creating unstratifiable programs
# Expected: Stratification errors, non-monotonic recursion

# Test 1: Classic direct cycle through negation
Decl p_base(X.Type<atom>).
p_base(/start).
# ERROR: p defined in terms of not q, q in terms of not p
p(X) :- p_base(X).
p(X) :- not q(X).
q(X) :- not p(X).

# Test 2: Three-way cycle
Decl a_base(X.Type<atom>).
a_base(/init).
# ERROR: a -> not b -> not c -> not a
a(X) :- a_base(X).
a(X) :- not b(X).
b(X) :- not c(X).
c(X) :- not a(X).

# Test 3: Self-negation
Decl start(S.Type<atom>).
start(/x).
# ERROR: paradox cannot be defined in terms of its own negation
paradox(X) :- start(X), not paradox(X).

# Test 4: Indirect self-negation through helper
Decl init(I.Type<atom>).
init(/begin).
# ERROR: weird depends on not weird through helper
weird(X) :- init(X), not helper(X).
helper(X) :- weird(X).

# Test 5: Cycle with positive recursion mixed in
Decl node(N.Type<atom>).
Decl edge(From.Type<atom>, To.Type<atom>).
node(/a).
edge(/a, /b).
# ERROR: reachable uses not blocked, blocked uses reachable
reachable(X) :- node(X).
reachable(Y) :- reachable(X), edge(X, Y), not blocked(Y).
blocked(X) :- reachable(X), node(X).

# Test 6: Negation in transitive closure
Decl person(P.Type<atom>).
Decl trust(P1.Type<atom>, P2.Type<atom>).
person(/alice).
trust(/alice, /bob).
# ERROR: trusted uses not suspicious, suspicious uses trusted
trusted(X) :- person(X), not suspicious(X).
trusted(Y) :- trusted(X), trust(X, Y), not suspicious(Y).
suspicious(X) :- trusted(X).

# Test 7: Even-odd cycle (classic unstratifiable)
Decl number(N.Type<int>).
number(0).
number(1).
# ERROR: even/odd defined through mutual negation
even(N) :- number(N), N = 0.
even(N) :- odd(M), N = fn:plus(M, 1).
odd(N) :- number(N), N = 1.
odd(N) :- even(M), N = fn:plus(M, 1), not even(N).

# Test 8: Aggregation cycle through negation
Decl value(V.Type<int>).
value(5).
# ERROR: high uses not low, low uses high
high(X) :- value(X), not low(X).
low(X) :- value(X), not high(X).

# Test 9: Longer cycle (4 predicates)
Decl base(B.Type<atom>).
base(/origin).
# ERROR: a -> not b -> c -> not d -> a
cycle_a(X) :- base(X), not cycle_b(X).
cycle_b(X) :- base(X), cycle_c(X).
cycle_c(X) :- base(X), not cycle_d(X).
cycle_d(X) :- base(X), cycle_a(X).

# Test 10: Stratifiable example for comparison (CORRECT)
Decl item(I.Type<atom>).
Decl category(I.Type<atom>, C.Type<atom>).
item(/sword).
category(/sword, /weapon).
# CORRECT: No recursion through negation
valid_item(I) :- item(I), category(I, C).
invalid_item(I) :- item(I), not valid_item(I).
# This is stratifiable: compute valid_item first, then invalid_item
