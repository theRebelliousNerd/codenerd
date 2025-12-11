# Grouping Patterns

## Problem Description

Grouping aggregates data by common attributes. Common needs:
- Group by single attribute
- Group by multiple attributes
- Nested grouping
- Grouping with complex aggregations
- Post-aggregation filtering (HAVING)

## Core Pattern: Basic Grouping

### Template
```mangle
# Group by single attribute
result(Group, AggValue) :-
  data(Item, Group, Value)
  |> do fn:group_by(Group),
     let AggValue = fn:Sum(Value).
```

### Complete Working Example
```mangle
# Schema
Decl sale(SaleId.Type<string>, Department.Type<string>, Amount.Type<int>).
Decl dept_total(Department.Type<string>, Total.Type<int>).

# Facts
sale("s1", "electronics", 1000).
sale("s2", "electronics", 1500).
sale("s3", "books", 500).
sale("s4", "books", 300).
sale("s5", "electronics", 2000).

# Group by department, sum amounts
dept_total(Department, Total) :-
  sale(SaleId, Department, Amount)
  |> do fn:group_by(Department),
     let Total = fn:Sum(Amount).

# Query: dept_total(Dept, Total)
# Results:
# dept_total("electronics", 4500)  # 1000 + 1500 + 2000
# dept_total("books", 800)         # 500 + 300
```

## Variation 1: Multi-Attribute Grouping

### Problem
Group by multiple attributes simultaneously.

### Solution
```mangle
# Schema
Decl sale(SaleId.Type<string>, Department.Type<string>, Region.Type<string>, Amount.Type<int>).
Decl dept_region_total(Department.Type<string>, Region.Type<string>, Total.Type<int>).

# Group by both department AND region
dept_region_total(Department, Region, Total) :-
  sale(SaleId, Department, Region, Amount)
  |> do fn:group_by(Department, Region),
     let Total = fn:Sum(Amount).
```

### Example
```mangle
sale("s1", "electronics", "west", 1000).
sale("s2", "electronics", "west", 1500).
sale("s3", "electronics", "east", 2000).
sale("s4", "books", "west", 500).
sale("s5", "books", "east", 300).

# Results:
# dept_region_total("electronics", "west", 2500)
# dept_region_total("electronics", "east", 2000)
# dept_region_total("books", "west", 500)
# dept_region_total("books", "east", 300)
```

## Variation 2: Multiple Aggregations per Group

### Problem
Calculate several aggregates for each group.

### Solution
```mangle
# Schema
Decl employee(EmpId.Type<string>, Department.Type<string>, Salary.Type<int>).
Decl dept_stats(Department.Type<string>, TotalSalary.Type<int>, AvgSalary.Type<float>, EmpCount.Type<int>).

# Multiple aggregations
dept_stats(Department, TotalSalary, AvgSalary, EmpCount) :-
  employee(EmpId, Department, Salary)
  |> do fn:group_by(Department),
     let TotalSalary = fn:Sum(Salary),
     let AvgSalary = fn:Avg(Salary),
     let EmpCount = fn:Count().
```

### Example
```mangle
employee("e1", "eng", 100000).
employee("e2", "eng", 120000).
employee("e3", "eng", 110000).
employee("e4", "sales", 80000).
employee("e5", "sales", 90000).

# Results:
# dept_stats("eng", 330000, 110000.0, 3)
# dept_stats("sales", 170000, 85000.0, 2)
```

## Variation 3: Conditional Aggregation

### Problem
Aggregate only values that meet certain criteria.

### Solution
```mangle
# Schema
Decl transaction(TxId.Type<string>, AccountId.Type<string>, Amount.Type<int>, Type.Type<atom>).
Decl account_balance(AccountId.Type<string>, Balance.Type<int>).

# Sum deposits, subtract withdrawals
Decl deposits(AccountId.Type<string>, Total.Type<int>).
Decl withdrawals(AccountId.Type<string>, Total.Type<int>).

deposits(AccountId, Total) :-
  transaction(TxId, AccountId, Amount, /deposit)
  |> do fn:group_by(AccountId),
     let Total = fn:Sum(Amount).

withdrawals(AccountId, Total) :-
  transaction(TxId, AccountId, Amount, /withdrawal)
  |> do fn:group_by(AccountId),
     let Total = fn:Sum(Amount).

# Calculate balance
account_balance(AccountId, Balance) :-
  deposits(AccountId, Dep),
  withdrawals(AccountId, With),
  Balance = fn:minus(Dep, With).

# Handle accounts with only deposits
account_balance(AccountId, Balance) :-
  deposits(AccountId, Balance),
  not withdrawals(AccountId, _).

# Handle accounts with only withdrawals
account_balance(AccountId, Balance) :-
  withdrawals(AccountId, With),
  not deposits(AccountId, _),
  Balance = fn:times(With, -1).
```

### Example
```mangle
transaction("t1", "acc1", 1000, /deposit).
transaction("t2", "acc1", 500, /deposit).
transaction("t3", "acc1", 200, /withdrawal).
transaction("t4", "acc2", 300, /deposit).

# Results:
# deposits("acc1", 1500)
# withdrawals("acc1", 200)
# account_balance("acc1", 1300)
# deposits("acc2", 300)
# account_balance("acc2", 300)
```

## Variation 4: Hierarchical Grouping (Rollup)

### Problem
Aggregate at multiple levels of a hierarchy.

### Solution
```mangle
# Schema
Decl sale(SaleId.Type<string>, Region.Type<string>, City.Type<string>, Amount.Type<int>).
Decl city_total(Region.Type<string>, City.Type<string>, Total.Type<int>).
Decl region_total(Region.Type<string>, Total.Type<int>).
Decl grand_total(Total.Type<int>).

# Level 1: City totals
city_total(Region, City, Total) :-
  sale(SaleId, Region, City, Amount)
  |> do fn:group_by(Region, City),
     let Total = fn:Sum(Amount).

# Level 2: Region totals (sum of cities)
region_total(Region, Total) :-
  city_total(Region, City, CityTotal)
  |> do fn:group_by(Region),
     let Total = fn:Sum(CityTotal).

# Level 3: Grand total (sum of regions)
grand_total(Total) :-
  region_total(Region, RegionTotal)
  |> do fn:group_by(),
     let Total = fn:Sum(RegionTotal).
```

### Example
```mangle
sale("s1", "west", "LA", 1000).
sale("s2", "west", "LA", 1500).
sale("s3", "west", "SF", 2000).
sale("s4", "east", "NYC", 3000).
sale("s5", "east", "Boston", 1000).

# Results:
# city_total("west", "LA", 2500)
# city_total("west", "SF", 2000)
# city_total("east", "NYC", 3000)
# city_total("east", "Boston", 1000)
# region_total("west", 4500)
# region_total("east", 4000)
# grand_total(8500)
```

## Variation 5: Grouping with Filtering (HAVING)

### Problem
Filter groups based on aggregated values.

### Solution
```mangle
# Schema
Decl customer(CustomerId.Type<string>, Segment.Type<string>).
Decl purchase(PurchaseId.Type<string>, CustomerId.Type<string>, Amount.Type<int>).
Decl customer_total(CustomerId.Type<string>, Total.Type<int>).
Decl high_value_customers(CustomerId.Type<string>, Total.Type<int>).

# Calculate totals
customer_total(CustomerId, Total) :-
  purchase(PurchaseId, CustomerId, Amount)
  |> do fn:group_by(CustomerId),
     let Total = fn:Sum(Amount).

# Filter: only customers with total > 10000
high_value_customers(CustomerId, Total) :-
  customer_total(CustomerId, Total),
  Total > 10000.
```

### Example
```mangle
purchase("p1", "c1", 5000).
purchase("p2", "c1", 6000).  # Total: 11000
purchase("p3", "c2", 3000).
purchase("p4", "c2", 2000).  # Total: 5000
purchase("p5", "c3", 15000). # Total: 15000

# Results:
# customer_total("c1", 11000)
# customer_total("c2", 5000)
# customer_total("c3", 15000)
# high_value_customers("c1", 11000)
# high_value_customers("c3", 15000)
# NOT c2 (below threshold)
```

## Variation 6: Grouping with Min/Max

### Problem
Find minimum or maximum value per group.

### Solution
```mangle
# Schema
Decl product(ProductId.Type<string>, Category.Type<string>, Price.Type<int>).
Decl category_price_range(Category.Type<string>, MinPrice.Type<int>, MaxPrice.Type<int>).

category_price_range(Category, MinPrice, MaxPrice) :-
  product(ProductId, Category, Price)
  |> do fn:group_by(Category),
     let MinPrice = fn:Min(Price),
     let MaxPrice = fn:Max(Price).
```

### Example
```mangle
product("p1", "electronics", 1000).
product("p2", "electronics", 3000).
product("p3", "electronics", 500).
product("p4", "books", 20).
product("p5", "books", 50).

# Results:
# category_price_range("electronics", 500, 3000)
# category_price_range("books", 20, 50)
```

## Variation 7: Grouping with First/Last

### Problem
Get the first or last item in each group (by some ordering).

### Solution
```mangle
# Schema
Decl event(EventId.Type<string>, UserId.Type<string>, Timestamp.Type<int>, Action.Type<string>).
Decl user_latest_event(UserId.Type<string>, LatestTime.Type<int>).
Decl user_latest_action(UserId.Type<string>, Action.Type<string>).

# Find latest timestamp per user
user_latest_event(UserId, LatestTime) :-
  event(EventId, UserId, Timestamp, Action)
  |> do fn:group_by(UserId),
     let LatestTime = fn:Max(Timestamp).

# Get action at latest timestamp
user_latest_action(UserId, Action) :-
  user_latest_event(UserId, LatestTime),
  event(EventId, UserId, LatestTime, Action).
```

### Example
```mangle
event("e1", "u1", 1000, "login").
event("e2", "u1", 1005, "click").
event("e3", "u1", 1010, "logout").
event("e4", "u2", 2000, "login").
event("e5", "u2", 2005, "purchase").

# Results:
# user_latest_event("u1", 1010)
# user_latest_event("u2", 2005)
# user_latest_action("u1", "logout")
# user_latest_action("u2", "purchase")
```

## Variation 8: Distinct Values per Group

### Problem
Find all distinct values of an attribute per group.

### Solution
```mangle
# Schema
Decl user_action(UserId.Type<string>, Action.Type<string>, Timestamp.Type<int>).
Decl user_action_count(UserId.Type<string>, DistinctActions.Type<int>).

# Count distinct actions per user
user_action_count(UserId, DistinctActions) :-
  user_action(UserId, Action, Timestamp)
  |> do fn:group_by(UserId),
     let DistinctActions = fn:CountDistinct(Action).
```

### Example
```mangle
user_action("u1", "login", 1000).
user_action("u1", "login", 1100).  # Duplicate action
user_action("u1", "click", 1200).
user_action("u1", "logout", 1300).
user_action("u2", "login", 2000).
user_action("u2", "login", 2100).  # Duplicate action

# Results:
# user_action_count("u1", 3)  # login, click, logout
# user_action_count("u2", 1)  # only login
```

## Variation 9: Grouping with Percentiles

### Problem
Calculate percentile values per group.

### Solution
```mangle
# Note: Mangle doesn't have built-in percentile functions
# Workaround: Use Min/Max as approximations or implement in Go

# Schema
Decl test_score(StudentId.Type<string>, Class.Type<string>, Score.Type<int>).
Decl class_stats(Class.Type<string>, MinScore.Type<int>, MaxScore.Type<int>, AvgScore.Type<float>).

# Approximate range with min/max
class_stats(Class, MinScore, MaxScore, AvgScore) :-
  test_score(StudentId, Class, Score)
  |> do fn:group_by(Class),
     let MinScore = fn:Min(Score),
     let MaxScore = fn:Max(Score),
     let AvgScore = fn:Avg(Score).
```

### Example
```mangle
test_score("s1", "math", 85).
test_score("s2", "math", 92).
test_score("s3", "math", 78).
test_score("s4", "science", 88).
test_score("s5", "science", 95).

# Results:
# class_stats("math", 78, 92, 85.0)
# class_stats("science", 88, 95, 91.5)
```

## Variation 10: Window Functions (Running Aggregates)

### Problem
Calculate running totals or moving averages within groups.

### Solution
```mangle
# Schema
Decl sale(SaleId.Type<string>, Day.Type<int>, Amount.Type<int>).
Decl cumulative_sales(Day.Type<int>, CumulativeTotal.Type<int>).

# Running total up to each day
cumulative_sales(Day, CumulativeTotal) :-
  sale(SaleId1, Day, Amount),
  sale(SaleId2, Day2, Amount2),
  Day2 <= Day
  |> do fn:group_by(Day),
     let CumulativeTotal = fn:Sum(Amount2).
```

### Example
```mangle
sale("s1", 1, 100).
sale("s2", 2, 150).
sale("s3", 3, 200).
sale("s4", 3, 50).  # Two sales on day 3

# Results:
# cumulative_sales(1, 100)   # Day 1: 100
# cumulative_sales(2, 250)   # Days 1-2: 100 + 150
# cumulative_sales(3, 500)   # Days 1-3: 100 + 150 + 200 + 50
```

## Variation 11: Grouping After Join

### Problem
Join multiple predicates then group the results.

### Solution
```mangle
# Schema
Decl order(OrderId.Type<string>, CustomerId.Type<string>, Amount.Type<int>).
Decl customer(CustomerId.Type<string>, Segment.Type<string>).
Decl segment_revenue(Segment.Type<string>, TotalRevenue.Type<int>).

# Join customer with orders, then group by segment
segment_revenue(Segment, TotalRevenue) :-
  customer(CustomerId, Segment),
  order(OrderId, CustomerId, Amount)
  |> do fn:group_by(Segment),
     let TotalRevenue = fn:Sum(Amount).
```

### Example
```mangle
customer("c1", "premium").
customer("c2", "premium").
customer("c3", "standard").

order("o1", "c1", 1000).
order("o2", "c1", 1500).
order("o3", "c2", 2000).
order("o4", "c3", 500).

# Results:
# segment_revenue("premium", 4500)  # c1: 2500, c2: 2000
# segment_revenue("standard", 500)   # c3: 500
```

## Anti-Patterns

### WRONG: Grouping Variable Not Bound
```mangle
# Bad - Category appears in grouping but not in body first
result(Category, Total) :-
  data(X, Value)
  |> do fn:group_by(Category),  # Category is unbound!
     let Total = fn:Sum(Value).

# Fix - ensure grouping variables are bound
result(Category, Total) :-
  data(X, Category, Value)
  |> do fn:group_by(Category),
     let Total = fn:Sum(Value).
```

### WRONG: Using Non-Grouped Variables in Result
```mangle
# Bad - X is not grouped, so which X should appear in result?
result(Category, X, Total) :-
  data(X, Category, Value)
  |> do fn:group_by(Category),
     let Total = fn:Sum(Value).

# Fix - only use grouped variables or aggregates
result(Category, Total) :-
  data(X, Category, Value)
  |> do fn:group_by(Category),
     let Total = fn:Sum(Value).
```

### WRONG: Multiple Aggregations Without Common Grouping
```mangle
# Bad - conflicting grouping levels
sum1(Total1) :- data(X, Y) |> do fn:group_by(X), let Total1 = fn:Sum(Y).
sum2(Total2) :- data(X, Y) |> do fn:group_by(Y), let Total2 = fn:Sum(X).
final(Total) :- sum1(Total1), sum2(Total2), Total = fn:plus(Total1, Total2).
# These are at different granularities - results will be misleading
```

## Performance Tips

1. **Group Early**: Reduce data volume before further processing
2. **Materialize Intermediate Groups**: For multi-level rollups
3. **Index Group Keys**: Ensure grouping attributes are indexed
4. **Minimize Group Count**: Fewer groups = faster aggregation
5. **Filter Before Grouping**: Reduce input size

## Common Use Cases in codeNERD

### Test Results by Suite
```mangle
Decl test_result(TestId.Type<string>, Suite.Type<string>, Status.Type<atom>).
Decl suite_stats(Suite.Type<string>, Passed.Type<int>, Failed.Type<int>).

Decl suite_passed(Suite.Type<string>, Count.Type<int>).
suite_passed(Suite, Count) :-
  test_result(TestId, Suite, /passed)
  |> do fn:group_by(Suite),
     let Count = fn:Count().

Decl suite_failed(Suite.Type<string>, Count.Type<int>).
suite_failed(Suite, Count) :-
  test_result(TestId, Suite, /failed)
  |> do fn:group_by(Suite),
     let Count = fn:Count().
```

### Code Complexity by Module
```mangle
Decl function(FuncId.Type<string>, Module.Type<string>, Complexity.Type<int>).
Decl module_complexity(Module.Type<string>, TotalComplexity.Type<int>, AvgComplexity.Type<float>).

module_complexity(Module, TotalComplexity, AvgComplexity) :-
  function(FuncId, Module, Complexity)
  |> do fn:group_by(Module),
     let TotalComplexity = fn:Sum(Complexity),
     let AvgComplexity = fn:Avg(Complexity).
```

### Shard Performance Metrics
```mangle
Decl shard_execution(ShardId.Type<string>, TaskId.Type<string>, DurationMs.Type<int>).
Decl shard_perf(ShardId.Type<string>, MinMs.Type<int>, MaxMs.Type<int>, AvgMs.Type<float>).

shard_perf(ShardId, MinMs, MaxMs, AvgMs) :-
  shard_execution(ShardId, TaskId, DurationMs)
  |> do fn:group_by(ShardId),
     let MinMs = fn:Min(DurationMs),
     let MaxMs = fn:Max(DurationMs),
     let AvgMs = fn:Avg(DurationMs).
```
