# Wrong Comment Syntax
# Error Type: Using // or /* */ instead of #
# Expected: Parse errors or comments treated as code

# Test 1: C-style single-line comment
Decl item(I.Type<atom>).
item(/sword).
// This should be a comment but will cause parse error
item(/shield).

# Test 2: C-style multi-line comment
Decl user(U.Type<atom>).
user(/alice).
/*
This is a multi-line comment
in C style, which is WRONG
*/
user(/bob).

# Test 3: SQL-style comment
Decl edge(From.Type<atom>, To.Type<atom>).
edge(/a, /b).
-- This is SQL style, wrong for Mangle
edge(/b, /c).

# Test 4: Python-style docstring
Decl status(S.Type<atom>).
"""
This is a docstring
which doesn't exist in Mangle
"""
status(/active).

# Test 5: Mixed comment styles in same file
Decl value(V.Type<int>).
value(10).  # Correct Mangle comment
value(20).  // Wrong C++ style
value(30).  -- Wrong SQL style

# Test 6: Inline C-style comment
Decl node(N.Type<atom>).
node(/n1 /* inline comment */).  # ERROR: /* */ not supported

# Test 7: Nested wrong comments
Decl data(D.Type<int>).
// Outer comment
  // Nested comment
data(42).

# Test 8: Comment at end of line without #
Decl config(C.Type<atom>).
config(/enabled).  This is supposed to be a comment  # ERROR: No // or #
