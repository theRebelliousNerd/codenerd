# Stratification Cycle Tests
# Error Type: Recursion through negation, creating unstratifiable programs
# Expected: Stratification errors, non-monotonic recursion

# Test 1: Classic direct cycle through negation
Decl p_base(X) bound [/name].
p_base(/start).
# ERROR: p defined in terms of !q, q in terms of !p (bound, so analysis
# passes and STRATIFICATION rejects the cycle).
p(X) :- p_base(X).
p(X) :- p_base(X), !q(X).
q(X) :- p_base(X), !p(X).

# Test 2: Three-way cycle
Decl a_base(X) bound [/name].
a_base(/init).
# ERROR: a -> !b -> !c -> !a (bound cycle; stratification rejects it).
a(X) :- a_base(X).
a(X) :- a_base(X), !b(X).
b(X) :- a_base(X), !c(X).
c(X) :- a_base(X), !a(X).

# Test 3: Self-negation
Decl start(S) bound [/name].
start(/x).
# ERROR: paradox cannot be defined in terms of its own negation
paradox(X) :- start(X), ! paradox(X).

# Test 4: Indirect self-negation through helper
Decl init(I) bound [/name].
init(/begin).
# ERROR: weird depends on not weird through helper
weird(X) :- init(X), ! helper(X).
helper(X) :- weird(X).

# Test 5: Cycle with positive recursion mixed in
Decl node(N) bound [/name].
Decl edge(From, To) bound [/name, /name].
node(/a).
edge(/a, /b).
# ERROR: reachable uses not blocked, blocked uses reachable
reachable(X) :- node(X).
reachable(Y) :- reachable(X), edge(X, Y), ! blocked(Y).
blocked(X) :- reachable(X), node(X).

# Test 6: Negation in transitive closure
Decl person(P) bound [/name].
Decl trust(P1, P2) bound [/name, /name].
person(/alice).
trust(/alice, /bob).
# ERROR: trusted uses not suspicious, suspicious uses trusted
trusted(X) :- person(X), ! suspicious(X).
trusted(Y) :- trusted(X), trust(X, Y), ! suspicious(Y).
suspicious(X) :- trusted(X).

# Test 7: Even-odd cycle (classic unstratifiable)
Decl number(N) bound [/number].
number(0).
number(1).
# ERROR: even/odd defined through mutual negation
even(N) :- number(N), N = 0.
even(N) :- odd(M), N = fn:plus(M, 1).
odd(N) :- number(N), N = 1.
odd(N) :- even(M), N = fn:plus(M, 1), ! even(N).

# Test 8: Aggregation cycle through negation
Decl value(V) bound [/number].
value(5).
# ERROR: high uses not low, low uses high
high(X) :- value(X), ! low(X).
low(X) :- value(X), ! high(X).

# Test 9: Longer cycle (4 predicates)
Decl base(B) bound [/name].
base(/origin).
# ERROR: a -> not b -> c -> not d -> a
cycle_a(X) :- base(X), ! cycle_b(X).
cycle_b(X) :- base(X), cycle_c(X).
cycle_c(X) :- base(X), ! cycle_d(X).
cycle_d(X) :- base(X), cycle_a(X).

# Test 10: Stratifiable example for comparison (CORRECT)
Decl item(I) bound [/name].
Decl category(I, C) bound [/name, /name].
item(/sword).
category(/sword, /weapon).
# CORRECT: No recursion through negation
valid_item(I) :- item(I), category(I, C).
invalid_item(I) :- item(I), ! valid_item(I).
# This is stratifiable: compute valid_item first, then invalid_item
