# Atom vs String Type Tests
# Error Type: Mixing atom and string types, type unification failures
# Expected: Type errors, unification failures
# DIALECT NOTE: fact literals are not checked against Decl bound types, so a
# mistyped fact is a silent data pollutant, not a static error (Tests 1, 2,
# 4). The executable teeth in this file are the unknown-function calls.

# Test 1: Declaring as atom, inserting as string
Decl status(S) bound [/name].
status("active").  # WRONG DATA (silent): should be /active; not statically checked
status(/inactive).  # Correct

# Test 2: Declaring as string, inserting as atom
Decl message(M) bound [/string].
message(/hello).  # WRONG DATA (silent): should be "hello"; not statically checked
message("world").  # Correct

# Test 3: Comparing atom to string in rule
Decl state(S) bound [/name].
state(/running).
# Comparing a name against a string literal is silently never-true.
check_state() :- state(X), X != "running".

# Test 4: Mixed types in same predicate
Decl label(L) bound [/name].
label(/tag1).
label("tag2").  # WRONG DATA (silent): inconsistent with /tag1; not statically checked

# Test 5: Function expecting string, given atom
Decl name(N) bound [/name].
name(/alice).
uppercase(U) :- name(N), U = fn:string_to_upper(N).  # ERROR: no such function (nearest: fn:string)

# Test 6: Struct field type mismatch
Decl config(C) bound [/struct].
config({ /enabled: "true" }).  # Name key (correct shape); value type is unchecked
# (A bare JSON-style key, { enabled: /true }, does not parse at all and lives
# as a demo in structures/json_syntax.mg instead.)

# Test 7: List of mixed types
Decl tags(T) bound [/list].
tags([/tag1, "tag2", /tag3]).  # ERROR: Inconsistent element types

# Test 8: Atom in string concatenation
Decl prefix(P) bound [/name].
prefix(/user).
make_key(K) :- prefix(P), K = fn:string_concat(P, "_123").  # ERROR: no such function (and P is a name)

# Test 9: String in atom comparison
Decl category(C) bound [/string].
category("weapon").
# A name in a string predicate's position never matches (silent logic gap).
is_weapon() :- category(/weapon).

# Test 10: Predicate name vs string confusion lives in
# syntactic/string_predicate.mg: "item"(X) does not parse at all.

# Test 11: Correct mixed usage (both types, different predicates)
Decl atom_data(A) bound [/name].
Decl string_data(S) bound [/string].
atom_data(/value).
string_data("text").
# CORRECT: Different predicates can use different types

# Test 12: Type promotion assumption (doesn't exist)
Decl identifier(ID) bound [/name].
identifier(/id123).
as_string(S) :- identifier(ID), S = ID.  # VALID but pointless: S unifies with the name; no string is produced
# Would need explicit conversion function
