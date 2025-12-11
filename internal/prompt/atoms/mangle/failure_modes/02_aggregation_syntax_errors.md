# Failure Mode 2: Aggregation Syntax Errors

## Category
Syntactic Hallucination (SQL/Souffle Bias)

## Severity
HIGH - Parse errors or incorrect results

## Error Pattern
Using SQL-style implicit grouping or missing required pipeline keywords (`|>`, `do`, `let`). Mangle requires **explicit pipeline syntax** for all aggregations.

## Wrong Code
```mangle
# WRONG - SQL mental model (implicit grouping)
region_sales(Region, Total) :-
    sales(Region, Amount),
    Total = sum(Amount).  # This is NOT how Mangle works!

# WRONG - Prolog findall mental model
region_sales(Region, Total) :-
    findall(Amount, sales(Region, Amount), Amounts),
    sum_list(Amounts, Total).

# WRONG - Missing `do` keyword
region_sales(Region, Total) :-
    sales(Region, Amount) |>
    fn:group_by(Region),  # Missing `do`!
    let Total = fn:Sum(Amount).

# WRONG - Function name casing error
total_count(N) :-
    item(X) |>
    do fn:group_by(),
    let N = fn:count().  # Should be fn:Count() with capital C

# WRONG - Using aggregation without pipeline
max_price(P) :- product(_, Price), P = fn:Max(Price).
```

## Correct Code
```mangle
# CORRECT - Full pipeline syntax with proper keywords
region_sales(Region, Total) :-
    sales(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:Sum(Amount).

# CORRECT - Count with no grouping
total_items(N) :-
    item(X) |>
    let N = fn:Count().

# CORRECT - Multiple aggregations
stats(Region, Total, Avg, Count) :-
    sales(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:Sum(Amount),
    let Avg = fn:Avg(Amount),
    let Count = fn:Count().

# CORRECT - Min/Max aggregation
price_range(Min, Max) :-
    product(_, Price) |>
    let Min = fn:Min(Price),
    let Max = fn:Max(Price).

# CORRECT - Collect into list
region_products(Region, Products) :-
    product(ID, Region, _) |>
    do fn:group_by(Region),
    let Products = fn:collect(ID).
```

## Detection
- **Symptom**: Parse error mentioning "unexpected token" near aggregation
- **Pattern**: Aggregation function used without `|>` pipeline
- **Pattern**: Missing `do` keyword before `fn:group_by`
- **Pattern**: Lowercase `fn:count()` or `fn:sum()` instead of capitalized
- **Test**: Search for `sum(`, `count(`, `max(`, `min(` not in pipeline context

## Prevention

### Required Keywords
| Keyword | Purpose | Required? | Example |
|---------|---------|-----------|---------|
| `\|>` | Pipeline operator | Always | `source() \|> ...` |
| `do` | Apply transform | With group_by | `do fn:group_by(X)` |
| `let` | Bind result | Always | `let N = fn:Count()` |
| `fn:` | Function namespace | Always | `fn:Sum`, `fn:Max` |

### Function Casing Rules
**CRITICAL**: Aggregation functions use specific casing:
- `fn:Count()` - Capital C
- `fn:Sum(X)` - Capital S
- `fn:Min(X)`, `fn:Max(X)` - Capital M
- `fn:Avg(X)` - Capital A
- `fn:group_by(X)` - lowercase g (exception!)
- `fn:collect(X)` - lowercase c

### Complete Aggregation Template
```mangle
# Template: Aggregation with grouping
result(GroupKey, AggValue) :-
    source_predicate(GroupKey, ValueToAggregate) |>
    do fn:group_by(GroupKey),
    let AggValue = fn:Sum(ValueToAggregate).

# Template: Aggregation without grouping
result(AggValue) :-
    source_predicate(ValueToAggregate) |>
    let AggValue = fn:Count().

# Template: Multiple aggregations
result(Key, Sum, Count, Avg) :-
    source(Key, Value) |>
    do fn:group_by(Key),
    let Sum = fn:Sum(Value),
    let Count = fn:Count(),
    let Avg = fn:Avg(Value).
```

## Training Bias Origins
| Language | Syntax | Leads to Wrong Mangle |
|----------|--------|----------------------|
| SQL | `SELECT SUM(amount) GROUP BY region` | `Total = sum(Amount)` |
| Prolog | `findall(X, pred(X), Xs)` | `findall(...)` |
| Souffle | Implicit grouping | Missing `do fn:group_by` |
| Python | `sum(amounts)` | `fn:sum()` lowercase |

## Quick Check
Before writing aggregation code:
1. Does it start with `|>` pipeline? (YES required)
2. Is grouping needed? → Add `do fn:group_by(Keys)`
3. Are aggregation functions capitalized? (`fn:Count`, `fn:Sum`)
4. Is `let` used to bind the result? (YES required)
5. Are all keywords present? (`|>`, `do`, `let`, `fn:`)
