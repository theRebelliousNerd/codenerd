# Soufflé Syntax Errors
# Error Type: Using Soufflé/SQL-style declarations instead of Mangle
# Expected: Parse errors due to wrong declaration syntax

# Test 1: Soufflé-style declaration with lowercase .decl
.decl edge(x:number, y:number).  # ERROR: Should be Decl edge(X.Type<int>, Y.Type<int>).

# Test 2: Soufflé type syntax
.decl person(name:symbol, age:number).  # ERROR: Wrong syntax entirely

# Test 3: SQL-style CREATE TABLE
CREATE TABLE users (id INT, name TEXT);  # ERROR: This is not SQL

# Test 4: Soufflé input directive
.input edge  # ERROR: Mangle doesn't use .input

# Test 5: Soufflé output directive
.output path  # ERROR: Mangle doesn't use .output

# Test 6: Soufflé component syntax
.comp Graph {
  .decl vertex(v:number)
}  # ERROR: Mangle has different module syntax

# Test 7: Soufflé aggregation syntax
count(n) :- n = count : { edge(x, y) }.  # ERROR: Wrong aggregation syntax

# Test 8: Soufflé choice construct
.decl chosen(x:number) choice-domain x  # ERROR: No choice-domain in Mangle

# Test 9: Soufflé type definition
.type Node = number  # ERROR: Should use Decl with Type<>

# Test 10: Soufflé functor syntax
.functor distance(x:number, y:number):number  # ERROR: External predicates use different syntax
