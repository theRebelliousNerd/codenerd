# Bracket Notation Error Tests
# Error Type: Using array/map bracket access instead of Mangle operations
# Expected: Parse errors, undefined operators

# Test 1: List index access with brackets
Decl items(I.Type<list>).
items([/a, /b, /c, /d]).
# ERROR: No bracket indexing
get_first(F) :- items(I), F = I[0].

# Test 2: Negative index (Python style)
Decl values(V.Type<list>).
values([1, 2, 3, 4, 5]).
# ERROR: No negative indexing
get_last(L) :- values(V), L = V[-1].

# Test 3: Map/struct access with brackets
Decl config(C.Type<struct>).
config({ /host: "localhost", /port: 8080 }).
# ERROR: No bracket accessor for structs
get_host(H) :- config(C), H = C[/host].

# Test 4: String key in brackets
Decl data(D.Type<struct>).
data({ /key: /value }).
# ERROR: Bracket notation with string
get_value(V) :- data(D), V = D["key"].

# Test 5: List slicing (Python style)
Decl sequence(S.Type<list>).
sequence([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]).
# ERROR: No slice notation
get_slice(Slice) :- sequence(S), Slice = S[2:5].

# Test 6: Multi-dimensional array access
Decl matrix(M.Type<list>).
matrix([[1, 2], [3, 4], [5, 6]]).
# ERROR: No chained brackets
get_cell(C) :- matrix(M), C = M[1][0].

# Test 7: List assignment with brackets (imperative)
Decl mutable_list(L.Type<list>).
mutable_list([1, 2, 3]).
# ERROR: Can't assign by index (immutable)
update_list() :- mutable_list(L), L[1] = 99.

# Test 8: Range in brackets
Decl numbers(N.Type<list>).
numbers([0, 1, 2, 3, 4, 5]).
# ERROR: No range slicing
evens(E) :- numbers(N), E = N[::2].  # Python every-other

# Test 9: Computed index
Decl items(I.Type<list>).
items([/a, /b, /c]).
# ERROR: Bracket with expression
get_middle(M) :- items(I), Len = fn:length(I), M = I[fn:div(Len, 2)].

# Test 10: Bracket on string (character access)
Decl text(T.Type<string>).
text("hello").
# ERROR: No character indexing
first_char(C) :- text(T), C = T[0].

# Test 11: Correct list access with :match_cons
Decl proper_list(L.Type<list>).
proper_list([/first, /second, /third]).
# CORRECT: Use pattern matching
get_first_proper(F) :- proper_list(L), :match_cons(L, F, _).

# Test 12: Attempting to access list length with brackets
Decl collection(C.Type<list>).
collection([1, 2, 3, 4]).
# ERROR: No .length or ['length']
size(S) :- collection(C), S = C['length'].

# Test 13: JavaScript-style property access
Decl object(O.Type<struct>).
object({ /property: /value }).
# ERROR: Mixing bracket and property
get_prop(P) :- object(O), P = O['property'].

# Test 14: Attempting to append with brackets
Decl base_list(L.Type<list>).
base_list([1, 2, 3]).
# ERROR: Can't append this way
append_item(L2) :- base_list(L), L2 = L[fn:length(L)] = 4.

# Test 15: Tuple-like access
Decl pair(P.Type<list>).
pair([/key, /value]).
# ERROR: No tuple unpacking with brackets
get_key(K) :- pair(P), K = P[0].
get_value(V) :- pair(P), V = P[1].
# Should use :match_cons twice or pattern matching

# Test 16: Hash map simulation
Decl hashmap(K.Type<atom>, V.Type<int>).
hashmap(/key1, 100).
hashmap(/key2, 200).
# ERROR: Treating predicate like array
lookup(V) :- V = hashmap[/key1].  # Wrong, should be: hashmap(/key1, V)

# Test 17: Correct struct field access
Decl proper_struct(S.Type<struct>).
proper_struct({ /name: "alice", /age: 30 }).
# CORRECT: Use :match_field
get_name_proper(N) :- proper_struct(S), :match_field(S, /name, N).

# Test 18: List comprehension style (doesn't exist)
Decl source(S.Type<list>).
source([1, 2, 3, 4, 5]).
# ERROR: No list comprehension syntax
doubled(D) :- source(S), D = [fn:times(X, 2) | X <- S].
