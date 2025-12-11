# Joining Patterns

## Problem Description

Joining predicates is fundamental to relational logic. Common needs:
- Inner joins (matching records)
- Left joins (all from left, matching from right)
- Multi-way joins (3+ predicates)
- Self-joins (predicate joined with itself)
- Join optimization (avoiding cartesian products)

## Core Pattern: Inner Join

### Template
```mangle
# Join on common variable
result(X, A, B) :- pred1(X, A), pred2(X, B).

# Join on equality condition
result(X, Y) :- pred1(X, Key1), pred2(Y, Key2), Key1 = Key2.
```

### Complete Working Example
```mangle
# Schema
Decl user(UserId.Type<string>, Name.Type<string>).
Decl order(OrderId.Type<string>, UserId.Type<string>, Amount.Type<int>).
Decl user_orders(Name.Type<string>, OrderId.Type<string>, Amount.Type<int>).

# Facts
user("u1", "Alice").
user("u2", "Bob").
user("u3", "Charlie").

order("o1", "u1", 100).
order("o2", "u1", 200).
order("o3", "u2", 150).

# Join on UserId
user_orders(Name, OrderId, Amount) :-
  user(UserId, Name),
  order(OrderId, UserId, Amount).

# Query: user_orders(Name, OrderId, Amount)
# Results:
# ("Alice", "o1", 100)
# ("Alice", "o2", 200)
# ("Bob", "o3", 150)
# NOT Charlie - no orders
```

## Variation 1: Multi-Way Join (3+ Predicates)

### Problem
Join three or more predicates together.

### Solution
```mangle
# Schema
Decl user(UserId.Type<string>, Name.Type<string>).
Decl order(OrderId.Type<string>, UserId.Type<string>, ProductId.Type<string>).
Decl product(ProductId.Type<string>, ProductName.Type<string>, Price.Type<int>).
Decl order_details(UserName.Type<string>, ProductName.Type<string>, Price.Type<int>).

# Three-way join
order_details(UserName, ProductName, Price) :-
  user(UserId, UserName),
  order(OrderId, UserId, ProductId),
  product(ProductId, ProductName, Price).
```

### Example
```mangle
user("u1", "Alice").
order("o1", "u1", "p1").
product("p1", "Laptop", 1000).

user("u2", "Bob").
order("o2", "u2", "p2").
product("p2", "Mouse", 25).

# Results:
# order_details("Alice", "Laptop", 1000)
# order_details("Bob", "Mouse", 25)
```

## Variation 2: Left Join (Outer Join)

### Problem
Get all records from left predicate, with matching right records if they exist.

### Solution
```mangle
# Schema
Decl user(UserId.Type<string>, Name.Type<string>).
Decl email(UserId.Type<string>, Email.Type<string>).
Decl user_with_email(UserId.Type<string>, Name.Type<string>, HasEmail.Type<atom>).

# Users with email
user_with_email(UserId, Name, /yes) :-
  user(UserId, Name),
  email(UserId, _).

# Users without email
user_with_email(UserId, Name, /no) :-
  user(UserId, Name),
  not email(UserId, _).
```

### Example
```mangle
user("u1", "Alice").
user("u2", "Bob").
user("u3", "Charlie").

email("u1", "alice@example.com").
email("u3", "charlie@example.com").

# Results:
# user_with_email("u1", "Alice", /yes)
# user_with_email("u2", "Bob", /no)
# user_with_email("u3", "Charlie", /yes)
```

## Variation 3: Self-Join

### Problem
Join a predicate with itself to find related records.

### Solution
```mangle
# Schema
Decl employee(EmpId.Type<string>, Name.Type<string>, ManagerId.Type<string>).
Decl employee_manager(EmpName.Type<string>, ManagerName.Type<string>).

# Join employee with itself on manager relationship
employee_manager(EmpName, ManagerName) :-
  employee(EmpId, EmpName, ManagerId),
  employee(ManagerId, ManagerName, _).
```

### Example
```mangle
employee("e1", "Alice", "e3").
employee("e2", "Bob", "e3").
employee("e3", "Charlie", "e4").
employee("e4", "Diana", "e4").  # Self-managed (CEO)

# Results:
# employee_manager("Alice", "Charlie")
# employee_manager("Bob", "Charlie")
# employee_manager("Charlie", "Diana")
# employee_manager("Diana", "Diana")  # Self-loop
```

## Variation 4: Join with Aggregation

### Problem
Join and then aggregate the results.

### Solution
```mangle
# Schema
Decl user(UserId.Type<string>, Name.Type<string>).
Decl order(OrderId.Type<string>, UserId.Type<string>, Amount.Type<int>).
Decl user_total_spent(Name.Type<string>, Total.Type<int>).

# Join then sum
user_total_spent(Name, Total) :-
  user(UserId, Name),
  order(OrderId, UserId, Amount)
  |> do fn:group_by(Name),
     let Total = fn:Sum(Amount).
```

### Example
```mangle
user("u1", "Alice").
user("u2", "Bob").

order("o1", "u1", 100).
order("o2", "u1", 200).
order("o3", "u2", 150).

# Results:
# user_total_spent("Alice", 300)
# user_total_spent("Bob", 150)
```

## Variation 5: Conditional Join

### Problem
Join only when a condition is met.

### Solution
```mangle
# Schema
Decl product(ProductId.Type<string>, CategoryId.Type<string>, Price.Type<int>).
Decl category(CategoryId.Type<string>, Name.Type<string>, TaxRate.Type<float>).
Decl expensive_taxable_products(ProductId.Type<string>, CategoryName.Type<string>).

# Join only expensive products with high-tax categories
expensive_taxable_products(ProductId, CategoryName) :-
  product(ProductId, CategoryId, Price),
  Price > 1000,
  category(CategoryId, CategoryName, TaxRate),
  TaxRate > 0.1.
```

### Example
```mangle
product("p1", "c1", 1500).
product("p2", "c1", 500).
product("p3", "c2", 2000).

category("c1", "Electronics", 0.15).
category("c2", "Books", 0.05).

# Results:
# expensive_taxable_products("p1", "Electronics")
# NOT p2 (too cheap)
# NOT p3 (low tax rate)
```

## Variation 6: Star Join (Fact Table + Dimensions)

### Problem
Join a central fact table with multiple dimension tables.

### Solution
```mangle
# Schema - Data warehouse style
Decl sale(SaleId.Type<string>, ProductId.Type<string>, CustomerId.Type<string>, StoreId.Type<string>, Amount.Type<int>).
Decl product(ProductId.Type<string>, Name.Type<string>).
Decl customer(CustomerId.Type<string>, Name.Type<string>).
Decl store(StoreId.Type<string>, Location.Type<string>).
Decl sale_detail(Amount.Type<int>, Product.Type<string>, Customer.Type<string>, Location.Type<string>).

# Star join
sale_detail(Amount, ProductName, CustomerName, Location) :-
  sale(SaleId, ProductId, CustomerId, StoreId, Amount),
  product(ProductId, ProductName),
  customer(CustomerId, CustomerName),
  store(StoreId, Location).
```

### Example
```mangle
sale("s1", "p1", "c1", "st1", 100).

product("p1", "Laptop").
customer("c1", "Alice").
store("st1", "NYC").

# Result:
# sale_detail(100, "Laptop", "Alice", "NYC")
```

## Variation 7: Join with Inequality

### Problem
Join on inequality conditions (not just equality).

### Solution
```mangle
# Schema
Decl employee(EmpId.Type<string>, Salary.Type<int>).
Decl higher_paid_pairs(Emp1.Type<string>, Emp2.Type<string>).

# Find pairs where Emp1 earns more than Emp2
higher_paid_pairs(Emp1, Emp2) :-
  employee(Emp1, Salary1),
  employee(Emp2, Salary2),
  Salary1 > Salary2,
  Emp1 != Emp2.  # Avoid self-pairs
```

### Example
```mangle
employee("e1", 100000).
employee("e2", 80000).
employee("e3", 90000).

# Results:
# higher_paid_pairs("e1", "e2")
# higher_paid_pairs("e1", "e3")
# higher_paid_pairs("e3", "e2")
```

## Variation 8: Anti-Join (Exclude Matching)

### Problem
Find records from left that do NOT have matches in right.

### Solution
```mangle
# Schema
Decl user(UserId.Type<string>, Name.Type<string>).
Decl order(OrderId.Type<string>, UserId.Type<string>).
Decl users_without_orders(UserId.Type<string>, Name.Type<string>).

# Users with no orders
users_without_orders(UserId, Name) :-
  user(UserId, Name),
  not order(_, UserId).
```

### Example
```mangle
user("u1", "Alice").
user("u2", "Bob").
user("u3", "Charlie").

order("o1", "u1").
order("o2", "u1").

# Results:
# users_without_orders("u2", "Bob")
# users_without_orders("u3", "Charlie")
```

## Variation 9: Semi-Join (Exists Check)

### Problem
Find records from left that have at least one match in right.

### Solution
```mangle
# Schema
Decl user(UserId.Type<string>, Name.Type<string>).
Decl order(OrderId.Type<string>, UserId.Type<string>).
Decl users_with_orders(UserId.Type<string>, Name.Type<string>).

# Users with at least one order
users_with_orders(UserId, Name) :-
  user(UserId, Name),
  order(_, UserId).
```

### Example
```mangle
user("u1", "Alice").
user("u2", "Bob").

order("o1", "u1").
order("o2", "u1").

# Results:
# users_with_orders("u1", "Alice")
# NOT u2 (no orders)
```

## Variation 10: Chain Join (A→B→C→D)

### Problem
Join through a chain of relationships.

### Solution
```mangle
# Schema
Decl order(OrderId.Type<string>, UserId.Type<string>).
Decl user(UserId.Type<string>, CompanyId.Type<string>).
Decl company(CompanyId.Type<string>, Name.Type<string>).
Decl order_company(OrderId.Type<string>, CompanyName.Type<string>).

# Chain: order → user → company
order_company(OrderId, CompanyName) :-
  order(OrderId, UserId),
  user(UserId, CompanyId),
  company(CompanyId, CompanyName).
```

### Example
```mangle
order("o1", "u1").
user("u1", "c1").
company("c1", "ACME Corp").

# Result:
# order_company("o1", "ACME Corp")
```

## Anti-Patterns

### WRONG: Cartesian Product (Missing Join Condition)
```mangle
# Bad - no shared variables!
bad_join(X, Y) :- table1(X), table2(Y).
# Result: Every X paired with every Y (explosion)

# Fix - join on common key
good_join(X, Y) :- table1(X, Key), table2(Y, Key).
```

### WRONG: Joining Before Filtering
```mangle
# Bad - join huge tables first
result(X, Y) :-
  huge_table1(X, Key),
  huge_table2(Y, Key),
  X = "specific_value".

# Fix - filter first
result(X, Y) :-
  X = "specific_value",
  huge_table1(X, Key),
  huge_table2(Y, Key).
```

### WRONG: Unintentional Duplicate Join
```mangle
# Bad - joining twice on same key creates duplicates
dup(X, Y1, Y2) :-
  table1(X, Key),
  table2(Y1, Key),
  table2(Y2, Key).  # This creates Y1 x Y2 pairs!

# Fix - only join once
no_dup(X, Y) :-
  table1(X, Key),
  table2(Y, Key).
```

### WRONG: Unbounded Self-Join
```mangle
# Bad - generates infinite pairs
same_value(X, Y) :- data(X), data(Y), X = Y.
# If data has 1000 rows, this generates 1000 * 1000 = 1M rows

# Fix - add distinctness or upper bound
different_value(X, Y) :- data(X), data(Y), X != Y, X < Y.  # Only one direction
```

## Performance Tips

1. **Selectivity First**: Filter before joining
   ```mangle
   # Good
   result(X, Y) :- X = "target", pred1(X, Key), pred2(Y, Key).

   # Bad
   result(X, Y) :- pred1(X, Key), pred2(Y, Key), X = "target".
   ```

2. **Join on Indexed Columns**: Ensure join keys are indexed in external stores

3. **Avoid Many-to-Many Joins**: They explode quickly
   ```mangle
   # Careful - M:N join
   result(X, Y) :- many_a(X, Key), many_b(Y, Key).
   ```

4. **Materialize Intermediate Results**: For complex multi-joins
   ```mangle
   # Stage 1
   intermediate(X, Y) :- pred1(X, Key), pred2(Y, Key).

   # Stage 2
   final(X, Y, Z) :- intermediate(X, Y), pred3(Y, Z).
   ```

5. **Use Semi/Anti Joins Over Count**: More efficient
   ```mangle
   # Good
   has_orders(User) :- user(User), order(_, User).

   # Slower
   has_orders(User) :- user(User), order(_, User) |> do fn:group_by(User), let Count = fn:Count(), Count > 0.
   ```

## Common Use Cases in codeNERD

### Symbol Resolution (Definition-Usage Join)
```mangle
Decl symbol_def(SymbolId.Type<string>, File.Type<string>, Line.Type<int>).
Decl symbol_use(SymbolId.Type<string>, File.Type<string>, Line.Type<int>).
Decl symbol_references(SymbolId.Type<string>, DefFile.Type<string>, UseFile.Type<string>).

symbol_references(SymbolId, DefFile, UseFile) :-
  symbol_def(SymbolId, DefFile, _),
  symbol_use(SymbolId, UseFile, _).
```

### File Dependency Graph
```mangle
Decl imports(FileA.Type<string>, FileB.Type<string>).
Decl file_metadata(File.Type<string>, Size.Type<int>).
Decl large_file_dependencies(FileA.Type<string>, FileB.Type<string>).

large_file_dependencies(FileA, FileB) :-
  imports(FileA, FileB),
  file_metadata(FileB, Size),
  Size > 10000.
```

### Test-Code Coverage Join
```mangle
Decl test(TestId.Type<string>, TestFile.Type<string>).
Decl covers(TestId.Type<string>, SourceFile.Type<string>).
Decl source_file(File.Type<string>).
Decl untested_files(File.Type<string>).

untested_files(File) :-
  source_file(File),
  not covers(_, File).
```
