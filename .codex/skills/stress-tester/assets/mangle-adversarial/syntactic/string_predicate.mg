# String Predicate Name Tests
# Error Type: Using a string literal where a predicate name belongs
# Expected: Parse errors (predicate symbols are NAME tokens, never strings)

# Test 1: String as predicate
Decl item(I) bound [/name].
item(/sword).
# ERROR: predicate name cannot be a string
find_item(X) :- "item"(X).

# CORRECT: bare predicate name
find_item_ok(X) :- item(X).
