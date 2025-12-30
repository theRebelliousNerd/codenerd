# List in Scalar Context Tests
# Error Type: Using list operations on scalars or vice versa
# Expected: Type errors, operation failures

# Test 1: List declared but scalar inserted
Decl items(I.Type<list>).
items(/single_item).  # ERROR: Should be list like [/single_item]
items([/item1, /item2]).  # Correct

# Test 2: Scalar declared but list inserted
Decl value(V.Type<int>).
value([1, 2, 3]).  # ERROR: Should be single int

# Test 3: List operation on scalar
Decl number(N.Type<int>).
number(42).
first(F) :- number(N), :match_cons(N, F, _).  # ERROR: N is int, not list

# Test 4: Scalar operation on list
Decl numbers(N.Type<list>).
numbers([1, 2, 3]).
double(D) :- numbers(N), D = fn:times(N, 2).  # ERROR: Can't multiply list

# Test 5: Index access on scalar
Decl item(I.Type<atom>).
item(/sword).
get_first(F) :- item(I), :match_cons(I, F, _).  # ERROR: I is atom, not list

# Test 6: Treating list as single value
Decl tags(T.Type<list>).
tags([/tag1, /tag2]).
check_tag() :- tags(T), T = /tag1.  # ERROR: T is list, /tag1 is atom

# Test 7: List in arithmetic
Decl values(V.Type<list>).
values([10, 20, 30]).
total(T) :- values(V), T = fn:plus(V, 5).  # ERROR: Can't add to list directly

# Test 8: Appending to scalar
Decl base(B.Type<int>).
base(10).
extend(E) :- base(B), E = :cons(B, 20).  # ERROR: cons needs list as second arg

# Test 9: List comparison as scalar
Decl list_a(L.Type<list>).
Decl list_b(L.Type<list>).
list_a([1, 2]).
list_b([1, 2]).
same() :- list_a(A), list_b(B), A = B.  # Might work, but type-wise questionable

# Test 10: Empty list vs nil confusion
Decl data(D.Type<list>).
data([]).
is_empty() :- data(D), D = nil.  # ERROR: [] is empty list, nil might not exist

# Test 11: Nested list type confusion
Decl matrix(M.Type<list>).
matrix([[1, 2], [3, 4]]).
get_cell(C) :- matrix(M), :match_cons(M, Row, _), :match_cons(Row, C, _).  # Complex but might work

# Test 12: List in struct field
Decl record(R.Type<struct>).
record({ /items: [/a, /b, /c] }).  # This might be OK
record({ /value: /single }).        # Mixing list and scalar in same field position

# Test 13: Aggregating lists
Decl collection(C.Type<list>).
collection([1, 2]).
collection([3, 4]).
# ERROR: Can't sum lists
total(T) :-
  collection(C) |>
  do fn:group_by(),
  let T = fn:Sum(C).

# Test 14: Correct list handling
Decl proper_list(L.Type<list>).
proper_list([1, 2, 3]).
first_element(F) :- proper_list(L), :match_cons(L, F, _).  # CORRECT
