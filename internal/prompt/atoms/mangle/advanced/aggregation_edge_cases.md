# Mangle Aggregation Edge Cases

## Aggregation Syntax Recap

```mangle
result(GroupKey, AggValue) :-
  predicate(GroupKey, Value)
  |> do fn:group_by(GroupKey),
  let AggValue = fn:sum(Value).
```

**Key components**:
- `|>` = Pipe operator (starts aggregation)
- `do fn:group_by(...)` = Define grouping keys
- `let X = fn:agg(Y)` = Compute aggregate

## Edge Case 1: Empty Groups

What happens when a group has no elements?

### Example
```mangle
Decl employee(Id.Type<atom>, Dept.Type<atom>, Salary.Type<int>).
Decl dept_total(Dept.Type<atom>, Total.Type<int>).

# Facts
employee(/e1, /engineering, 100000).
# No employees in /marketing!

# Aggregation
dept_total(Dept, Total) :-
  employee(Id, Dept, Salary)
  |> do fn:group_by(Dept),
  let Total = fn:sum(Salary).

# Query: dept_total(/marketing, ?Total)
# Result: NO RESULTS (not 0!)
```

**Behavior**: If a group has no members, **no result is produced** for that group.

### Solution: Explicit Group Universe
```mangle
Decl department(Dept.Type<atom>).
department(/engineering).
department(/marketing).

# Force all departments to appear
dept_total_with_zero(Dept, Total) :-
  department(Dept),
  employee_total(Dept, Total).

dept_total_with_zero(Dept, 0) :-
  department(Dept),
  not employee_total(Dept, _).

employee_total(Dept, Total) :-
  employee(Id, Dept, Salary)
  |> do fn:group_by(Dept),
  let Total = fn:sum(Salary).
```

## Edge Case 2: Null Handling

Mangle does NOT have null values. Under CWA, missing = doesn't exist.

### Common Mistake (Coming from SQL)
```mangle
# In SQL: SUM ignores NULLs
# In Mangle: There are no NULLs to ignore

# This is fine:
total(Sum) :-
  data(X)
  |> do fn:group_by(),
  let Sum = fn:sum(X).

# If data/1 has no facts, the rule produces NO results (not 0, not NULL)
```

### Solution: Default Values
```mangle
# To ensure a result even if empty:
total_or_zero(Sum) :-
  total(Sum).

total_or_zero(0) :-
  not total(_).
```

## Edge Case 3: Multiple Aggregations

Can you compute multiple aggregates in one rule?

**Yes!**

### Example
```mangle
Decl sale(Product.Type<atom>, Amount.Type<int>).
Decl product_stats(Product.Type<atom>, Total.Type<int>, Avg.Type<float>, Count.Type<int>).

sale(/widget, 100).
sale(/widget, 150).
sale(/widget, 200).

product_stats(Product, Total, Avg, Count) :-
  sale(Product, Amount)
  |> do fn:group_by(Product),
  let Total = fn:sum(Amount),
  let Avg = fn:mean(Amount),
  let Count = fn:count(Amount).

# Result: product_stats(/widget, 450, 150.0, 3)
```

**All aggregates share the same grouping.**

## Edge Case 4: Aggregation with Filtering

Filter **before** aggregation for performance.

### Example
```mangle
Decl transaction(Id.Type<atom>, User.Type<atom>, Amount.Type<int>, Status.Type<atom>).
Decl user_successful_total(User.Type<atom>, Total.Type<int>).

transaction(/t1, /alice, 100, /success).
transaction(/t2, /alice, 50, /failed).
transaction(/t3, /alice, 200, /success).

# BAD: Aggregate all, then filter
bad_total(User, Total) :-
  transaction(Id, User, Amount, Status)
  |> do fn:group_by(User),
  let Total = fn:sum(Amount),
  Status = /success.  # WRONG: Status is not accessible here!

# GOOD: Filter first, then aggregate
good_total(User, Total) :-
  Status = /success,  # Filter first
  transaction(Id, User, Amount, Status)
  |> do fn:group_by(User),
  let Total = fn:sum(Amount).

# Result: user_successful_total(/alice, 300)
```

**Rule**: Variables used in filtering must be bound **before** the pipe `|>`.

## Edge Case 5: Nested Aggregations

Can you aggregate over aggregate results?

**Yes, but requires two strata.**

### Example: Average of Department Averages
```mangle
Decl employee(Id.Type<atom>, Dept.Type<atom>, Salary.Type<int>).
Decl dept_avg(Dept.Type<atom>, Avg.Type<float>).
Decl company_avg_of_avgs(Avg.Type<float>).

employee(/e1, /eng, 100000).
employee(/e2, /eng, 120000).
employee(/e3, /sales, 80000).

# Stratum 1: Compute department averages
dept_avg(Dept, Avg) :-
  employee(Id, Dept, Salary)
  |> do fn:group_by(Dept),
  let Avg = fn:mean(Salary).

# Stratum 2: Aggregate over department averages
company_avg_of_avgs(CompanyAvg) :-
  dept_avg(Dept, Avg)
  |> do fn:group_by(),
  let CompanyAvg = fn:mean(Avg).

# Result: company_avg_of_avgs(100000.0)
#   dept_avg(/eng, 110000.0)
#   dept_avg(/sales, 80000.0)
#   mean([110000.0, 80000.0]) = 95000.0
```

**Important**: Each aggregation is a separate stratum.

## Edge Case 6: Aggregation Over Empty Input

What if the input predicate has no facts?

```mangle
Decl data(X.Type<int>).
# No facts!

total(Sum) :-
  data(X)
  |> do fn:group_by(),
  let Sum = fn:sum(X).

# Query: total(?Sum)
# Result: NO RESULTS (not 0!)
```

**Mangle behavior**: No input → No output (not even a 0).

### Solution: Provide Default
```mangle
total_or_zero(Sum) :-
  total(Sum).

total_or_zero(0) :-
  not total(_).

# Query: total_or_zero(?Sum)
# Result: total_or_zero(0)
```

## Edge Case 7: GROUP BY without Aggregation

Can you group without aggregating?

**No direct syntax**, but you can use `fn:collect` to gather values.

### Example: Group Users by Department
```mangle
Decl employee(Id.Type<atom>, Dept.Type<atom>).
Decl dept_employees(Dept.Type<atom>, Employees.Type<list<atom>>).

employee(/e1, /eng).
employee(/e2, /eng).
employee(/e3, /sales).

dept_employees(Dept, Employees) :-
  employee(Id, Dept)
  |> do fn:group_by(Dept),
  let Employees = fn:collect(Id).

# Result:
#   dept_employees(/eng, [/e1, /e2])
#   dept_employees(/sales, [/e3])
```

## Edge Case 8: Count Distinct

Count unique values in a group.

### Example
```mangle
Decl purchase(User.Type<atom>, Product.Type<atom>).
Decl user_product_count(User.Type<atom>, Count.Type<int>).

purchase(/alice, /widget).
purchase(/alice, /widget).  # Duplicate purchase
purchase(/alice, /gadget).

# Count distinct products per user
user_product_count(User, Count) :-
  purchase(User, Product)
  |> do fn:group_by(User),
  let Count = fn:count_distinct(Product).

# Result: user_product_count(/alice, 2)
#   (widget appears twice, but counted once)
```

## Edge Case 9: Aggregation with Inequality

Filter aggregated results with inequality.

### Example: High-Value Customers
```mangle
Decl purchase(User.Type<atom>, Amount.Type<int>).
Decl high_value_customer(User.Type<atom>, Total.Type<int>).

purchase(/alice, 100).
purchase(/alice, 200).
purchase(/bob, 50).

# Customers with total > 150
high_value_customer(User, Total) :-
  purchase(User, Amount)
  |> do fn:group_by(User),
  let Total = fn:sum(Amount),
  Total > 150.  # Filter on aggregate result

# Result: high_value_customer(/alice, 300)
#   (Bob's total is 50, excluded)
```

## Edge Case 10: Aggregation Over Computed Values

Aggregate over expressions, not just raw values.

### Example: Total Tax
```mangle
Decl sale(Product.Type<atom>, Price.Type<int>).
Decl total_tax(TaxAmount.Type<float>).

sale(/widget, 100).
sale(/gadget, 200).

# Compute tax per item, then sum
total_tax(TaxAmount) :-
  sale(Product, Price),
  Tax = fn:mult(fn:to_float(Price), 0.08)  # 8% tax
  |> do fn:group_by(),
  let TaxAmount = fn:sum(Tax).

# Result: total_tax(24.0)
#   Tax on widget: 8.0
#   Tax on gadget: 16.0
#   Total: 24.0
```

## Edge Case 11: Multiple GROUP BY Keys

Group by multiple columns.

### Example: Department and Job Title
```mangle
Decl employee(Id.Type<atom>, Dept.Type<atom>, Title.Type<atom>, Salary.Type<int>).
Decl dept_title_avg(Dept.Type<atom>, Title.Type<atom>, Avg.Type<float>).

employee(/e1, /eng, /senior, 120000).
employee(/e2, /eng, /senior, 130000).
employee(/e3, /eng, /junior, 80000).

# Group by both department AND title
dept_title_avg(Dept, Title, Avg) :-
  employee(Id, Dept, Title, Salary)
  |> do fn:group_by(Dept, Title),
  let Avg = fn:mean(Salary).

# Result:
#   dept_title_avg(/eng, /senior, 125000.0)
#   dept_title_avg(/eng, /junior, 80000.0)
```

## Edge Case 12: Aggregation Safety

All variables in `fn:group_by()` must be bound in the rule body.

### Unsafe (WRONG)
```mangle
# WRONG: Dept is not bound before grouping
bad_agg(Dept, Total) :-
  employee(Id, D, Salary)
  |> do fn:group_by(Dept),  # Dept is unbound!
  let Total = fn:sum(Salary).
```

### Safe (CORRECT)
```mangle
# CORRECT: Dept is bound in body
good_agg(Dept, Total) :-
  employee(Id, Dept, Salary)
  |> do fn:group_by(Dept),
  let Total = fn:sum(Salary).
```

## Available Aggregate Functions

| Function | Purpose | Example |
|----------|---------|---------|
| `fn:sum(X)` | Sum of values | `fn:sum(Salary)` |
| `fn:count(X)` | Count of values (including duplicates) | `fn:count(Id)` |
| `fn:count_distinct(X)` | Count of unique values | `fn:count_distinct(Product)` |
| `fn:mean(X)` | Average (mean) | `fn:mean(Score)` |
| `fn:min(X)` | Minimum value | `fn:min(Price)` |
| `fn:max(X)` | Maximum value | `fn:max(Price)` |
| `fn:collect(X)` | Collect values into list | `fn:collect(Name)` |

## Performance Tips

1. **Filter before aggregation**
   ```mangle
   # GOOD
   result(Total) :-
     X > 100,  # Filter first
     data(X)
     |> do fn:group_by(),
     let Total = fn:sum(X).
   ```

2. **Avoid re-aggregating**
   ```mangle
   # BAD: Computes same aggregate twice
   rule1(Sum) :- data(X) |> do fn:group_by(), let Sum = fn:sum(X).
   rule2(Sum) :- data(X) |> do fn:group_by(), let Sum = fn:sum(X).

   # GOOD: Materialize once
   total(Sum) :- data(X) |> do fn:group_by(), let Sum = fn:sum(X).
   rule1(Sum) :- total(Sum).
   rule2(Sum) :- total(Sum).
   ```

3. **Use count instead of collecting + length**
   ```mangle
   # BAD
   count_bad(N) :-
     data(X)
     |> do fn:group_by(),
     let List = fn:collect(X),
     N = fn:length(List).

   # GOOD
   count_good(N) :-
     data(X)
     |> do fn:group_by(),
     let N = fn:count(X).
   ```

## Checklist

- [ ] Empty groups handled (default values if needed)
- [ ] Filtering happens before `|>` pipe
- [ ] All grouping variables are bound in rule body
- [ ] Nested aggregations use separate strata
- [ ] Using appropriate aggregate function (count vs count_distinct)
- [ ] Not re-computing same aggregate multiple times
- [ ] Handling case where input predicate has no facts
