# Counting Patterns

## Problem Description

Counting is fundamental for metrics, statistics, and thresholds. Common needs:
- Count total items
- Count distinct values
- Conditional counts
- Count per group
- Percentages and ratios

## Core Pattern: Basic Count

### Template
```mangle
# Count all items
total_count(Count) :-
  item(X)
  |> do fn:group_by(),
     let Count = fn:Count().

# Count per group
group_count(Group, Count) :-
  item(X, Group)
  |> do fn:group_by(Group),
     let Count = fn:Count().
```

### Complete Working Example
```mangle
# Schema
Decl user(UserId.Type<string>, Country.Type<string>).
Decl total_users(Count.Type<int>).
Decl users_per_country(Country.Type<string>, Count.Type<int>).

# Facts
user("u1", "USA").
user("u2", "USA").
user("u3", "Canada").
user("u4", "USA").
user("u5", "Canada").

# Total count
total_users(Count) :-
  user(UserId, _)
  |> do fn:group_by(),
     let Count = fn:Count().

# Count per country
users_per_country(Country, Count) :-
  user(UserId, Country)
  |> do fn:group_by(Country),
     let Count = fn:Count().

# Query: total_users(C)
# Result: total_users(5)

# Query: users_per_country(Country, C)
# Results:
# users_per_country("USA", 3)
# users_per_country("Canada", 2)
```

## Variation 1: Count Distinct

### Problem
Count unique values (not total rows).

### Solution
```mangle
# Schema
Decl event(EventId.Type<string>, UserId.Type<string>, EventType.Type<string>).
Decl unique_users(Count.Type<int>).
Decl unique_users_per_type(EventType.Type<string>, Count.Type<int>).

# Count distinct users (total)
unique_users(Count) :-
  event(_, UserId, _)
  |> do fn:group_by(),
     let Count = fn:CountDistinct(UserId).

# Count distinct users per event type
unique_users_per_type(EventType, Count) :-
  event(_, UserId, EventType)
  |> do fn:group_by(EventType),
     let Count = fn:CountDistinct(UserId).
```

### Example
```mangle
event("e1", "u1", "login").
event("e2", "u1", "login").  # Same user
event("e3", "u2", "login").
event("e4", "u1", "click").

# Results:
# unique_users(2)  # u1 and u2
# unique_users_per_type("login", 2)  # u1 and u2
# unique_users_per_type("click", 1)  # u1 only
```

## Variation 2: Conditional Count

### Problem
Count items that meet certain criteria.

### Solution
```mangle
# Schema
Decl product(ProductId.Type<string>, Price.Type<int>, InStock.Type<atom>).
Decl expensive_count(Count.Type<int>).
Decl in_stock_count(Count.Type<int>).

# Count expensive items (Price > 1000)
expensive_count(Count) :-
  product(ProductId, Price, _),
  Price > 1000
  |> do fn:group_by(),
     let Count = fn:Count().

# Count in-stock items
in_stock_count(Count) :-
  product(ProductId, _, /yes)
  |> do fn:group_by(),
     let Count = fn:Count().
```

### Example
```mangle
product("p1", 1500, /yes).
product("p2", 500, /yes).
product("p3", 2000, /no).
product("p4", 800, /yes).

# Results:
# expensive_count(2)  # p1 and p3
# in_stock_count(3)   # p1, p2, p4
```

## Variation 3: Count with Zero (Include Empty Groups)

### Problem
Show count even for groups with zero items.

### Solution
```mangle
# Schema
Decl category(CategoryId.Type<string>).
Decl product(ProductId.Type<string>, CategoryId.Type<string>).
Decl product_count(CategoryId.Type<string>, Count.Type<int>).

# Categories with products
product_count(CategoryId, Count) :-
  product(ProductId, CategoryId)
  |> do fn:group_by(CategoryId),
     let Count = fn:Count().

# Categories with zero products
product_count(CategoryId, 0) :-
  category(CategoryId),
  not product(_, CategoryId).
```

### Example
```mangle
category("electronics").
category("books").
category("toys").

product("p1", "electronics").
product("p2", "electronics").
product("p3", "books").

# Results:
# product_count("electronics", 2)
# product_count("books", 1)
# product_count("toys", 0)  # Explicit zero
```

## Variation 4: Running Total / Cumulative Count

### Problem
Count how many items exist up to each point.

### Solution
```mangle
# Schema
Decl event(EventId.Type<string>, Timestamp.Type<int>).
Decl events_before(Timestamp.Type<int>, Count.Type<int>).

# Count events before or at each timestamp
events_before(T, Count) :-
  event(_, T),
  event(EventId, EventTime),
  EventTime <= T
  |> do fn:group_by(T),
     let Count = fn:Count().
```

### Example
```mangle
event("e1", 1000).
event("e2", 1005).
event("e3", 1010).

# Results:
# events_before(1000, 1)  # Just e1
# events_before(1005, 2)  # e1 and e2
# events_before(1010, 3)  # All three
```

## Variation 5: Percentage / Ratio

### Problem
Calculate percentage of items in each group.

### Solution
```mangle
# Schema
Decl user(UserId.Type<string>, Country.Type<string>).
Decl total_users(Count.Type<int>).
Decl users_per_country(Country.Type<string>, Count.Type<int>).
Decl country_percentage(Country.Type<string>, Percentage.Type<float>).

# Total (from earlier pattern)
total_users(Count) :-
  user(UserId, _)
  |> do fn:group_by(),
     let Count = fn:Count().

# Per country
users_per_country(Country, Count) :-
  user(UserId, Country)
  |> do fn:group_by(Country),
     let Count = fn:Count().

# Percentage
country_percentage(Country, Percentage) :-
  users_per_country(Country, CountryCount),
  total_users(TotalCount),
  Percentage = fn:div(fn:times(CountryCount, 100.0), TotalCount).
```

### Example
```mangle
user("u1", "USA").
user("u2", "USA").
user("u3", "USA").
user("u4", "Canada").

# Results:
# total_users(4)
# users_per_country("USA", 3)
# users_per_country("Canada", 1)
# country_percentage("USA", 75.0)
# country_percentage("Canada", 25.0)
```

## Variation 6: Threshold Filtering (HAVING)

### Problem
Filter groups based on their count.

### Solution
```mangle
# Schema
Decl purchase(UserId.Type<string>, Amount.Type<int>).
Decl purchase_count(UserId.Type<string>, Count.Type<int>).
Decl frequent_buyers(UserId.Type<string>).

# Count purchases per user
purchase_count(UserId, Count) :-
  purchase(UserId, Amount)
  |> do fn:group_by(UserId),
     let Count = fn:Count().

# Only users with 3+ purchases
frequent_buyers(UserId) :- purchase_count(UserId, Count), Count >= 3.
```

### Example
```mangle
purchase("u1", 100).
purchase("u1", 200).
purchase("u1", 150).
purchase("u2", 300).
purchase("u2", 400).

# Results:
# purchase_count("u1", 3)
# purchase_count("u2", 2)
# frequent_buyers("u1")  # Only u1 has 3+
```

## Variation 7: Multi-Level Count

### Problem
Count at multiple granularity levels.

### Solution
```mangle
# Schema
Decl sale(SaleId.Type<string>, StoreId.Type<string>, Region.Type<string>).
Decl sales_per_store(StoreId.Type<string>, Count.Type<int>).
Decl sales_per_region(Region.Type<string>, Count.Type<int>).

# Count per store
sales_per_store(StoreId, Count) :-
  sale(SaleId, StoreId, _)
  |> do fn:group_by(StoreId),
     let Count = fn:Count().

# Count per region
sales_per_region(Region, Count) :-
  sale(SaleId, _, Region)
  |> do fn:group_by(Region),
     let Count = fn:Count().
```

### Example
```mangle
sale("s1", "store1", "west").
sale("s2", "store1", "west").
sale("s3", "store2", "west").
sale("s4", "store3", "east").

# Results:
# sales_per_store("store1", 2)
# sales_per_store("store2", 1)
# sales_per_store("store3", 1)
# sales_per_region("west", 3)
# sales_per_region("east", 1)
```

## Variation 8: Count Combinations

### Problem
Count occurrences of value pairs.

### Solution
```mangle
# Schema
Decl event(UserId.Type<string>, Action.Type<string>).
Decl action_count(UserId.Type<string>, Action.Type<string>, Count.Type<int>).

# Count per user-action pair
action_count(UserId, Action, Count) :-
  event(UserId, Action)
  |> do fn:group_by(UserId, Action),
     let Count = fn:Count().
```

### Example
```mangle
event("u1", "login").
event("u1", "login").
event("u1", "click").
event("u2", "login").

# Results:
# action_count("u1", "login", 2)
# action_count("u1", "click", 1)
# action_count("u2", "login", 1)
```

## Variation 9: Count with Join

### Problem
Count items after joining multiple predicates.

### Solution
```mangle
# Schema
Decl user(UserId.Type<string>, Country.Type<string>).
Decl order(OrderId.Type<string>, UserId.Type<string>).
Decl orders_per_country(Country.Type<string>, Count.Type<int>).

# Join then count
orders_per_country(Country, Count) :-
  user(UserId, Country),
  order(OrderId, UserId)
  |> do fn:group_by(Country),
     let Count = fn:Count().
```

### Example
```mangle
user("u1", "USA").
user("u2", "USA").
user("u3", "Canada").

order("o1", "u1").
order("o2", "u1").
order("o3", "u2").

# Results:
# orders_per_country("USA", 3)  # 2 from u1, 1 from u2
# orders_per_country("Canada", 0)  # No orders from u3 - WAIT, this won't show!
```

### Note on Zero Counts After Joins
```mangle
# To include zero counts:
Decl country(C.Type<string>).
country("USA").
country("Canada").

orders_per_country(Country, Count) :-
  user(UserId, Country),
  order(OrderId, UserId)
  |> do fn:group_by(Country),
     let Count = fn:Count().

# Add explicit zero case
orders_per_country(Country, 0) :-
  country(Country),
  not (user(UserId, Country), order(_, UserId)).
```

## Variation 10: Top N by Count

### Problem
Find the N groups with the highest counts.

### Solution
```mangle
# Schema
Decl event(UserId.Type<string>, Action.Type<string>).
Decl action_count(Action.Type<string>, Count.Type<int>).
Decl top_actions(Action.Type<string>, Count.Type<int>, Rank.Type<int>).

# Count per action
action_count(Action, Count) :-
  event(UserId, Action)
  |> do fn:group_by(Action),
     let Count = fn:Count().

# Rank by count (descending)
# Note: Mangle doesn't have built-in LIMIT/TOP N
# Workaround: find max, then next max excluding previous, etc.

# Find maximum count
Decl max_count(MaxCount.Type<int>).
max_count(MaxCount) :-
  action_count(Action, Count)
  |> do fn:group_by(),
     let MaxCount = fn:Max(Count).

# Actions with max count
top_actions(Action, Count, 1) :-
  action_count(Action, Count),
  max_count(Count).
```

### Example
```mangle
event("u1", "login").
event("u2", "login").
event("u3", "login").
event("u4", "click").
event("u5", "click").
event("u6", "purchase").

# Results:
# action_count("login", 3)
# action_count("click", 2)
# action_count("purchase", 1)
# max_count(3)
# top_actions("login", 3, 1)
```

## Anti-Patterns

### WRONG: Counting Without Grouping
```mangle
# Bad - this counts the same item multiple times if it appears in multiple rules
count(N) :- item(X), N = 1.
# Fix: Use aggregation
count(N) :- item(X) |> do fn:group_by(), let N = fn:Count().
```

### WRONG: Forgetting CountDistinct
```mangle
# Bad - counts duplicate user IDs
user_count(Count) :- event(_, UserId) |> do fn:group_by(), let Count = fn:Count().

# Fix - use CountDistinct for unique users
user_count(Count) :- event(_, UserId) |> do fn:group_by(), let Count = fn:CountDistinct(UserId).
```

### WRONG: Counting Before Filtering
```mangle
# Inefficient - count everything then filter
all_count(Category, Count) :- item(X, Category) |> do fn:group_by(Category), let Count = fn:Count().
high_count(Category, Count) :- all_count(Category, Count), Count > 100.

# Better - filter first if possible
# (Though for counts, you usually need to count first then filter)
```

## Performance Tips

1. **Filter Before Counting**: Reduce dataset size before aggregation
2. **Use CountDistinct Wisely**: More expensive than Count
3. **Materialize Counts**: If reused, compute once and store
4. **Index Group Keys**: Ensure grouping columns are indexed

## Common Use Cases in codeNERD

### Test Coverage Metrics
```mangle
Decl test_covers(TestId.Type<string>, SourceFile.Type<string>).
Decl coverage_count(SourceFile.Type<string>, Count.Type<int>).

coverage_count(SourceFile, Count) :-
  test_covers(TestId, SourceFile)
  |> do fn:group_by(SourceFile),
     let Count = fn:Count().
```

### Function Call Frequency
```mangle
Decl function_call(CallSite.Type<string>, FuncName.Type<string>).
Decl call_count(FuncName.Type<string>, Count.Type<int>).

call_count(FuncName, Count) :-
  function_call(CallSite, FuncName)
  |> do fn:group_by(FuncName),
     let Count = fn:Count().
```

### Shard Execution Statistics
```mangle
Decl shard_executed(ShardId.Type<string>, TaskId.Type<string>, Status.Type<atom>).
Decl shard_success_count(ShardId.Type<string>, Count.Type<int>).

shard_success_count(ShardId, Count) :-
  shard_executed(ShardId, TaskId, /success)
  |> do fn:group_by(ShardId),
     let Count = fn:Count().
```
