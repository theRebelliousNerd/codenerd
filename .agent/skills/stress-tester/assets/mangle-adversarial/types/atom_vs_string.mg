# Atom vs String Type Tests
# Error Type: Mixing atom and string types, type unification failures
# Expected: Type errors, unification failures

# Test 1: Declaring as atom, inserting as string
Decl status(S.Type<atom>).
status("active").  # ERROR: Should be /active
status(/inactive).  # Correct

# Test 2: Declaring as string, inserting as atom
Decl message(M.Type<string>).
message(/hello).  # ERROR: Should be "hello"
message("world").  # Correct

# Test 3: Comparing atom to string in rule
Decl state(S.Type<atom>).
state(/running).
check_state(Result) :- state(X), X = "running", Result = /matched.  # ERROR: Won't unify

# Test 4: Mixed types in same predicate
Decl label(L.Type<atom>).
label(/tag1).
label("tag2").  # ERROR: Type inconsistency

# Test 5: Function expecting string, given atom
Decl name(N.Type<atom>).
name(/alice).
uppercase(U) :- name(N), U = fn:string_to_upper(N).  # ERROR: Needs string

# Test 6: Struct field type mismatch
Decl config(C.Type<struct>).
config({ /enabled: "true" }).  # Mixed: atom key (correct), string value (depends on intent)
config({ enabled: /true }).   # ERROR: Key should be /enabled

# Test 7: List of mixed types
Decl tags(T.Type<list>).
tags([/tag1, "tag2", /tag3]).  # ERROR: Inconsistent element types

# Test 8: Atom in string concatenation
Decl prefix(P.Type<atom>).
prefix(/user).
make_key(K) :- prefix(P), K = fn:string_concat(P, "_123").  # ERROR: P is atom

# Test 9: String in atom comparison
Decl category(C.Type<string>).
category("weapon").
is_weapon() :- category(C), C = /weapon.  # ERROR: String vs atom

# Test 10: Predicate name vs string confusion
Decl item(I.Type<atom>).
item(/sword).
find_item(X) :- "item"(X).  # ERROR: Predicate name can't be string

# Test 11: Correct mixed usage (both types, different predicates)
Decl atom_data(A.Type<atom>).
Decl string_data(S.Type<string>).
atom_data(/value).
string_data("text").
# CORRECT: Different predicates can use different types

# Test 12: Type promotion assumption (doesn't exist)
Decl identifier(ID.Type<atom>).
identifier(/id123).
as_string(S) :- identifier(ID), S = ID.  # ERROR: No automatic conversion
# Would need explicit conversion function
