# Mutual Recursion Loop Tests
# Error Type: Multiple predicates calling each other infinitely
# Expected: Infinite mutual recursion, non-termination
# DIALECT NOTE: non-termination is a RUNTIME property: every rule below is a
# VALID program that parses and analyzes cleanly. These files pin well-
# formedness only; the live danger (unbounded derivation) is contained by
# executor timeouts, not by static rejection.

# Test 1: Simple A <-> B loop
Decl a(X) bound [/number].
Decl b(X) bound [/number].
a(0).
# ERROR: a -> b -> a -> b... infinite
a(X) :- b(Y), X = fn:plus(Y, 1).
b(X) :- a(Y), X = fn:plus(Y, 1).

# Test 2: Three-way mutual recursion
Decl first(F) bound [/number].
Decl second(S) bound [/number].
Decl third(T) bound [/number].
first(0).
# ERROR: first -> second -> third -> first... infinite
first(X) :- third(Y), X = fn:plus(Y, 1).
second(X) :- first(Y), X = fn:plus(Y, 1).
third(X) :- second(Y), X = fn:plus(Y, 1).

# Test 3: Mutual recursion with different operations
Decl double(D) bound [/number].
Decl triple(T) bound [/number].
double(1).
# ERROR: double -> triple -> double... infinite growth
double(X) :- triple(Y), X = fn:mult(Y, 2).
triple(X) :- double(Y), X = fn:mult(Y, 3).

# Test 4: Even/Odd mutual recursion (unbounded)
Decl even_num(E) bound [/number].
Decl odd_num(O) bound [/number].
even_num(0).
odd_num(1).
# ERROR: Generates all even/odd numbers infinitely
even_num(N) :- odd_num(M), N = fn:plus(M, 1).
odd_num(N) :- even_num(M), N = fn:plus(M, 1).

# Test 5: Mutual recursion through list building
Decl list_a(L) bound [/list].
Decl list_b(L) bound [/list].
list_a([]).
list_b([]).
# ERROR: Build infinitely long lists
list_a(L) :- list_b(Tail), L = fn:list(/a, Tail).
list_b(L) :- list_a(Tail), L = fn:list(/b, Tail).

# Test 6: Four-way cycle
Decl p1(X) bound [/number].
Decl p2(X) bound [/number].
Decl p3(X) bound [/number].
Decl p4(X) bound [/number].
p1(0).
# ERROR: p1 -> p2 -> p3 -> p4 -> p1... infinite
p1(X) :- p4(Y), X = fn:plus(Y, 1).
p2(X) :- p1(Y), X = fn:plus(Y, 1).
p3(X) :- p2(Y), X = fn:plus(Y, 1).
p4(X) :- p3(Y), X = fn:plus(Y, 1).

# Test 7: Mutual recursion with accumulation
Decl acc_a(Val, Acc) bound [/number, /number].
Decl acc_b(Val, Acc) bound [/number, /number].
acc_a(0, 0).
# ERROR: Accumulates infinitely
acc_a(V, A) :- acc_b(Vb, Ab), V = fn:plus(Vb, 1), A = fn:plus(Ab, V).
acc_b(V, A) :- acc_a(Va, Aa), V = fn:plus(Va, 1), A = fn:plus(Aa, V).

# Test 8: Mutual recursion with conditions (still infinite)
Decl conditional_a(X) bound [/number].
Decl conditional_b(X) bound [/number].
conditional_a(0).
# ERROR: Condition doesn't prevent infinite recursion
conditional_a(X) :- conditional_b(Y), X = fn:plus(Y, 1), X < 1000.
conditional_b(X) :- conditional_a(Y), X = fn:plus(Y, 2), X < 1000.

# Test 9: Mutual recursion through helper predicates
Decl start(S) bound [/number].
Decl helper1(H) bound [/number].
Decl helper2(H) bound [/number].
Decl end(E) bound [/number].
start(0).
# ERROR: start -> helper1 -> helper2 -> end -> start... infinite
start(X) :- end(Y), X = fn:plus(Y, 1).
helper1(X) :- start(Y), X = fn:plus(Y, 1).
helper2(X) :- helper1(Y), X = fn:plus(Y, 1).
end(X) :- helper2(Y), X = fn:plus(Y, 1).

# Test 10: Correct mutual recursion (bounded)
Decl node(N) bound [/name].
Decl edge_forward(From, To) bound [/name, /name].
Decl edge_backward(From, To) bound [/name, /name].
node(/a).
node(/b).
node(/c).
edge_forward(/a, /b).
edge_backward(/b, /a).
# CORRECT: Finite nodes, computes reachability
reach_forward(X, Y) :- edge_forward(X, Y).
reach_forward(X, Z) :- reach_forward(X, Y), edge_forward(Y, Z).
reach_backward(X, Y) :- edge_backward(X, Y).
reach_backward(X, Z) :- reach_backward(X, Y), edge_backward(Y, Z).
# This terminates because node/1 is finite

# Test 11: Mutual recursion with string alternation
Decl string_a(S) bound [/string].
Decl string_b(S) bound [/string].
string_a("a").
# ERROR: Builds "a", "ab", "aba", "abab"... infinitely
string_a(S) :- string_b(Sb), S = fn:list(Sb, "a").
string_b(S) :- string_a(Sa), S = fn:list(Sa, "b").

# Test 12: Mutual recursion with multiple bases
Decl multi_a(X) bound [/number].
Decl multi_b(X) bound [/number].
multi_a(0).
multi_a(1).
multi_b(2).
# ERROR: Multiple starting points, infinite expansion
multi_a(X) :- multi_b(Y), X = fn:plus(Y, 3).
multi_b(X) :- multi_a(Y), X = fn:plus(Y, 3).
