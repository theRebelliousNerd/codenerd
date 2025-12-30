# Dot Notation Error Tests
# Error Type: Using object-oriented dot notation instead of Mangle struct access
# Expected: Parse errors, undefined operators

# Test 1: Map/struct field access with dot
Decl config(C.Type<struct>).
config({ /host: "localhost", /port: 8080 }).
# ERROR: Can't use dot notation
get_host(H) :- config(C), H = C./host.

# Test 2: Python/JavaScript style attribute access
Decl user(U.Type<struct>).
user({ /name: "alice", /age: 30 }).
# ERROR: No dot accessor
get_name(N) :- user(U), N = U.name.

# Test 3: Nested struct dot access
Decl settings(S.Type<struct>).
settings({ /database: { /host: "localhost", /port: 5432 } }).
# ERROR: Chained dot notation doesn't exist
get_db_host(H) :- settings(S), H = S.database.host.

# Test 4: Method call style
Decl item(I.Type<struct>).
item({ /name: /sword, /durability: 100 }).
# ERROR: No methods in Mangle
get_name(N) :- item(I), N = I.getName().

# Test 5: Java/C# property style
Decl person(P.Type<struct>).
person({ /firstName: "John", /lastName: "Doe" }).
# ERROR: No property syntax
full_name(F) :- person(P), F = P.FirstName.

# Test 6: Bracket notation (JavaScript)
Decl data(D.Type<struct>).
data({ /key: "value" }).
# ERROR: No bracket accessor
get_value(V) :- data(D), V = D[/key].

# Test 7: String key in dot notation
Decl record(R.Type<struct>).
record({ /field: 42 }).
# ERROR: Mixing dot with string key
get_field(F) :- record(R), F = R."field".

# Test 8: Attempting to call functions on values
Decl text(T.Type<string>).
text("hello").
# ERROR: No method chaining
uppercase(U) :- text(T), U = T.toUpperCase().

# Test 9: List element access with dot
Decl items(I.Type<list>).
items([/a, /b, /c]).
# ERROR: No dot indexing
first(F) :- items(I), F = I.0.

# Test 10: Correct struct access with :match_field
Decl proper_config(C.Type<struct>).
proper_config({ /host: "localhost", /port: 8080 }).
# CORRECT: Use :match_field or :match_entry
get_proper_host(H) :- proper_config(C), :match_field(C, /host, H).

# Test 11: Attempting to modify with dot
Decl mutable(M.Type<struct>).
mutable({ /count: 0 }).
# ERROR: Can't assign to properties (immutable)
increment() :- mutable(M), M.count = fn:plus(M.count, 1).

# Test 12: TypeScript-style optional chaining
Decl maybe_user(U.Type<struct>).
maybe_user({ /name: "alice" }).
# ERROR: No optional chaining
safe_age(A) :- maybe_user(U), A = U?.age.

# Test 13: Destructuring with dot
Decl complex(C.Type<struct>).
complex({ /inner: { /value: 42 } }).
# ERROR: Can't destructure with dots
extract(V) :- complex(C), { /inner: { /value: V } } = C.
# Even destructuring syntax might not work as expected

# Test 14: Attempting to access list length
Decl collection(C.Type<list>).
collection([1, 2, 3, 4, 5]).
# ERROR: No .length property
size(S) :- collection(C), S = C.length.
# Should use a function like fn:list:length if it exists
