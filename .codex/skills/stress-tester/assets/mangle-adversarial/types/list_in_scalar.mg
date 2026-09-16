# List in Scalar Context Tests
# Error Type: Using list operations on scalars or vice versa
# Expected: Type errors, operation failures
# DIALECT NOTE: function-argument types are not checked statically, so most
# of these are VALID programs that fail at query time (RUNTIME). List
# decomposition is :match_cons(List, HeadVar, TailVar) with VARIABLES ONLY;
# there is no nil literal and no [H|T] syntax.

# Test 1: List declared but scalar inserted
Decl items(I) bound [/list].
items(/single_item).  # WRONG DATA (silent): facts are not type-checked
items([/item1, /item2]).  # Correct

# Test 2: Scalar declared but list inserted
Decl value(V) bound [/number].
value([1, 2, 3]).  # WRONG DATA (silent): facts are not type-checked

# Test 3: List operation on scalar
Decl number(N) bound [/number].
number(42).
first(F) :- number(N), :match_cons(N, F, _).  # RUNTIME: match fails, N is not a list

# Test 4: Scalar operation on list
Decl numbers(N) bound [/list].
numbers([1, 2, 3]).
double(D) :- numbers(N), D = fn:mult(N, 2).  # RUNTIME: mult on a list fails at query time

# Test 5: Index access on scalar
Decl item(I) bound [/name].
item(/sword).
get_first(F) :- item(I), :match_cons(I, F, _).  # RUNTIME: match fails, I is a name

# Test 6: Treating list as single value
Decl tags(T) bound [/list].
tags([/tag1, /tag2]).
# T = /tag1 does not parse (= rejects /name on the right); destructure,
# then filter positionally by calling with the constant.
check_tag(H) :- tags(T), :match_cons(T, H, _).
is_tag1() :- check_tag(/tag1).

# Test 7: List in arithmetic
Decl values(V) bound [/list].
values([10, 20, 30]).
total(T) :- values(V), T = fn:plus(V, 5).  # RUNTIME: plus on a list fails at query time

# Test 8: Appending to scalar
Decl base(B) bound [/number].
base(10).
# :cons is not a value constructor; fn:list builds lists.
extend(E) :- base(B), E = fn:list(B, 20).

# Test 9: List comparison as scalar
Decl list_a(L) bound [/list].
Decl list_b(L) bound [/list].
list_a([1, 2]).
list_b([1, 2]).
same() :- list_a(A), list_b(B), A = B.  # VALID: same type, unifies [1,2] with [1,2]

# Test 10: Empty list vs nil confusion
Decl data(D) bound [/list].
data([]).
# There is no nil literal; match the empty list directly.
is_empty() :- data([]).

# Test 11: Nested list type confusion
Decl matrix(M) bound [/list].
matrix([[1, 2], [3, 4]]).
get_cell(C) :- matrix(M), :match_cons(M, Row, _), :match_cons(Row, C, _).  # VALID: nested destructure

# Test 12: List in struct field
Decl record(R) bound [/struct].
record({ /items: [/a, /b, /c] }).  # This might be OK
record({ /value: /single }).        # VALID syntax: field types are unchecked (doc-level smell)

# Test 13: Aggregating lists
Decl collection(C) bound [/list].
collection([1, 2]).
collection([3, 4]).
# No fn:Sum exists; counting form (summing a list column is a runtime error).
total(T) :-
  collection(C) |>
  do fn:group_by(),
  let T = fn:count().

# Test 14: Correct list handling
Decl proper_list(L) bound [/list].
proper_list([1, 2, 3]).
first_element(F) :- proper_list(L), :match_cons(L, F, _).  # CORRECT
