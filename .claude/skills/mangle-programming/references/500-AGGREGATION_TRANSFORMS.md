# 500: Aggregation & Transforms - Complete Guide

**Purpose**: Master transform pipelines and complex aggregations for analytics and data consolidation.

## Transform Pipeline Architecture

### Basic Pipeline
```mangle
Decl sale(Category, Value) bound [/name, /number].
Decl category_total(Category, Total) bound [/name, /number].
category_total(Cat, Total) :-
    sale(Cat, Value)
    |> do fn:group_by(Cat), let Total = fn:sum(Value).
```

### Multi-Stage Pipeline
```mangle
# Filter in the body, then one stage: a do, then lets. A let may use the lets
# before it in the same stage. There is no fn:filter, fn:transform or fn:divide
# (division is fn:div), and a second |> stage of only lets does not bind.
Decl sale(Category, Item, Value) bound [/name, /name, /number].
Decl category_average(Category, Avg) bound [/name, /number].
category_average(Cat, Avg) :-
    sale(Cat, Item, Value), Value > 10
    |> do fn:group_by(Cat), let Count = fn:count(), let Total = fn:sum(Value), let Avg = fn:div(Total, Count).
```

## Aggregation Functions

### Count
```mangle
Decl item(X, Y).
items_per_category(Cat, N) :- 
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:count().
```

### Sum, Min, Max
```mangle
Decl item(Cat, Value).
category_stats(Cat, Total, Min, Max) :- 
    item(Cat, Value) |>
    do fn:group_by(Cat),
    let Total = fn:sum(Value),
    let Min = fn:min(Value),
    let Max = fn:max(Value).
```

### Average (Derived)
```mangle
Decl item(Category, Value) bound [/name, /number].
Decl average_per_category(Category, Avg) bound [/name, /number].
# Integer average: fn:div truncates. fn:avg(Value) yields a float instead.
average_per_category(Cat, Avg) :-
    item(Cat, Value) |>
    do fn:group_by(Cat),
    let Total = fn:sum(Value),
    let Count = fn:count(),
    let Avg = fn:div(Total, Count).
```

## Conditional Aggregation

### Filter Before Aggregation
```mangle
# The filter is an ordinary comparison in the body; the transform sees only the rows that pass.
Decl item(Category, Value) bound [/name, /number].
Decl high_value_count(Category, N) bound [/name, /number].
high_value_count(Cat, N) :-
    item(Cat, Value), Value > 1000
    |> do fn:group_by(Cat), let N = fn:count().
```

### Multiple Conditional Aggregations
```mangle
Decl item(Category, Value) bound [/name, /number].
Decl high_value_count(Category, N) bound [/name, /number].
Decl low_value_count(Category, N) bound [/name, /number].
Decl category_breakdown(Category, High, Low) bound [/name, /number, /number].

# Only categories with rows on both sides appear: an aggregate over no rows derives
# nothing, not zero (add a separate negation rule for the zero case).
category_breakdown(Cat, HighCount, LowCount) :-
    high_value_count(Cat, HighCount),
    low_value_count(Cat, LowCount).

high_value_count(Cat, N) :-
    item(Cat, V), V > 1000
    |> do fn:group_by(Cat), let N = fn:count().

low_value_count(Cat, N) :-
    item(Cat, V), V <= 1000
    |> do fn:group_by(Cat), let N = fn:count().
```

## Multi-Dimensional Grouping

```mangle
# Group by multiple variables
Decl sale(Region, Product, Amount).
sales_summary(Region, Product, Count, Revenue) :- 
    sale(Region, Product, Amount) |>
    do fn:group_by(Region, Product),
    let Count = fn:count(),
    let Revenue = fn:sum(Amount).
```

## Nested Aggregation

### Aggregate Then Aggregate
```mangle
Decl item(Category, Value) bound [/name, /number].
Decl category_total(Category, Total) bound [/name, /number].
Decl overall_stats(GrandTotal, AvgCategoryTotal) bound [/number, /number].

# Step 1: Category totals
category_total(Cat, Total) :-
    item(Cat, Value) |>
    do fn:group_by(Cat),
    let Total = fn:sum(Value).

# Step 2: Overall statistics from category totals (name every column: no `_`
# in a transform rule's body)
overall_stats(GrandTotal, AvgCategoryTotal) :-
    category_total(Cat, Total) |>
    do fn:group_by(),
    let GrandTotal = fn:sum(Total),
    let Count = fn:count(),
    let AvgCategoryTotal = fn:div(GrandTotal, Count).
```

## Window Functions (Simulated)

### Running Total
```mangle
# Ordered by assuming natural order
Decl item(Item, Value).
running_total(Item, RunningSum) :- 
    item(Item, Value),
    item(PrevItem, PrevValue),
    PrevItem < Item |>  # Ordering assumption
    do fn:group_by(Item),
    let RunningSum = fn:sum(PrevValue).
```

### Rank (Simulated)
```mangle
# Count items with higher value
Decl item(Item, Value).
rank(Item, Rank) :- 
    item(Item, Value),
    item(OtherItem, OtherValue),
    OtherValue > Value |>
    do fn:group_by(Item),
    let Rank = fn:plus(fn:count(), 1).
```

---

**See also**: 300-PATTERN_LIBRARY.md for aggregation pattern examples.
