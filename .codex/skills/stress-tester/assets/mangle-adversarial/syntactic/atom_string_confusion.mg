# Atom vs String Confusion Tests
# Error Type: Using strings when atoms are required, or vice versa
# Expected: Parse/type errors due to improper atom/string usage

# Test 1: String instead of atom in enum-like value
# WRONG: Using "active" when schema requires /active
Decl status(S.Type<atom>).
status("active").  # ERROR: Should be /active

# Test 2: Atom instead of string in text field
Decl message(M.Type<string>).
message(/hello_world).  # ERROR: Should be "hello world"

# Test 3: Mixed atom/string in same predicate position
Decl item_type(T.Type<atom>).
item_type(/weapon).
item_type("armor").  # ERROR: Inconsistent typing

# Test 4: String in predicate name position (extreme confusion)
"pred"(X) :- other(X).  # ERROR: Predicate names must be atoms/identifiers

# Test 5: Atom key without slash in struct
Decl config(C.Type<struct>).
config({ active: true }).  # ERROR: Should be /active: true

# Test 6: String comparison with atom
Decl state(S.Type<atom>).
state(/running).
bad_check(X) :- state(X), X = "running".  # ERROR: Won't unify, atom vs string

# Test 7: Atom in string concatenation context
result(R) :-
  R = fn:string_concat(/prefix, /suffix).  # ERROR: concat requires strings

# Test 8: Mixed identifiers
Decl edge(From.Type<atom>, To.Type<atom>).
edge("node1", /node2).  # ERROR: Inconsistent types
