# Mangle Builtin Functions: Arithmetic Operations

## Integer Arithmetic

### fn:plus

**Signature**: `fn:plus(X, Y) → Z`

**Type**: `(number, number) → number`

**Purpose**: Adds two integers.

**Examples**:
```mangle
# Simple addition
total(T) :- a(X), b(Y), T = fn:plus(X, Y).

# Increment
next(N) :- current(M), N = fn:plus(M, 1).

# Sum multiple values
total_price(P) :-
    leg1_price(P1),
    leg2_price(P2),
    P = fn:plus(P1, P2).

# Chaining additions
sum_three(S) :-
    a(A), b(B), c(C),
    Temp = fn:plus(A, B),
    S = fn:plus(Temp, C).
```

**Note**: For summing over groups, use the `fn:Sum` aggregator instead.

---

### fn:minus

**Signature**: `fn:minus(X, Y) → Z`

**Type**: `(number, number) → number`

**Purpose**: Subtracts Y from X (X - Y).

**Examples**:
```mangle
# Difference
diff(D) :- max_val(M), min_val(N), D = fn:minus(M, N).

# Decrement
prev(N) :- current(M), N = fn:minus(M, 1).

# Age difference
age_gap(P1, P2, Gap) :-
    age(P1, A1),
    age(P2, A2),
    A1 > A2,
    Gap = fn:minus(A1, A2).
```

**Warning**: Result can be negative. Ensure types support negative numbers.

---

### fn:mult

**Signature**: `fn:mult(X, Y) → Z`

**Type**: `(number, number) → number`

**Purpose**: Multiplies two integers.

**Examples**:
```mangle
# Area calculation
area(A) :- width(W), height(H), A = fn:mult(W, H).

# Double a value
double(D) :- value(V), D = fn:mult(V, 2).

# Product of multiple values
product(P) :-
    a(A), b(B), c(C),
    Temp = fn:mult(A, B),
    P = fn:mult(Temp, C).
```

---

### fn:div

**Signature**: `fn:div(X, Y) → Z`

**Type**: `(number, number) → number`

**Purpose**: Integer division (X / Y, rounded down).

**Examples**:
```mangle
# Half value (rounded down)
half(H) :- value(V), H = fn:div(V, 2).

# Average of two numbers (integer division)
avg(A) :- x(X), y(Y), Sum = fn:plus(X, Y), A = fn:div(Sum, 2).

# Per-item cost
per_item(Cost) :- total(T), count(C), Cost = fn:div(T, C).
```

**Warning**: Division by zero likely causes an error. Ensure denominator is non-zero.

**Note**: This is integer division. For exact division with decimals, use float operations.

---

### Absolute Value (abs)

**Internal Function**: `abs(x int64) → int64`

**Purpose**: Computes absolute value of an integer.

**Note**: This appears in the Go source but may not be directly exposed as `fn:abs`. Check if it's available as a builtin function.

**Special Case**: Returns MaxInt64 when input is MinInt64 (to avoid overflow).

---

## Floating-Point Arithmetic

### fn:float_plus

**Signature**: `fn:float_plus(X, Y) → Z`

**Type**: `(float64, float64) → float64`

**Purpose**: Adds two floating-point numbers.

**Examples**:
```mangle
# Precise addition
total(T) :- a(X), b(Y), T = fn:float_plus(X, Y).

# Increment by fractional amount
next(N) :- current(M), N = fn:float_plus(M, 0.1).
```

**Note**: Both arguments must be floats (e.g., `3.0` not `3`).

---

### fn:float_mult

**Signature**: `fn:float_mult(X, Y) → Z`

**Type**: `(float64, float64) → float64`

**Purpose**: Multiplies two floating-point numbers.

**Examples**:
```mangle
# Precise area
area(A) :- width(W), height(H), A = fn:float_mult(W, H).

# Apply percentage (e.g., 15% tax)
with_tax(T) :- price(P), T = fn:float_mult(P, 1.15).
```

---

### fn:float_div

**Signature**: `fn:float_div(X, Y) → Z`

**Type**: `(float64, float64) → float64`

**Purpose**: Divides two floating-point numbers (exact division).

**Examples**:
```mangle
# Exact average
avg(A) :-
    x(X), y(Y),
    Sum = fn:float_plus(X, Y),
    A = fn:float_div(Sum, 2.0).

# Percentage
percentage(P) :- part(Part), whole(Whole), P = fn:float_div(Part, Whole).
```

---

### fn:sqrt

**Signature**: `fn:sqrt(X) → Y`

**Type**: `(float64) → float64`

**Purpose**: Computes square root.

**Examples**:
```mangle
# Euclidean distance (simplified)
distance(D) :-
    dx_squared(DX2),
    dy_squared(DY2),
    sum_squares(S),
    S = fn:plus(DX2, DY2),  # This would need conversion to float
    D = fn:sqrt(S).

# Standard deviation computation (partial)
std_dev_component(S) :- variance(V), S = fn:sqrt(V).
```

---

## Type Conversion

### Number to Float

**Pattern**: Need to convert integers to floats for float operations.

**Workaround**: The documentation doesn't show explicit conversion functions. May need to:
- Use float literals (e.g., `3.0` instead of `3`)
- Check if `fn:to_float` or similar exists

### Float to Number

**Pattern**: Convert float to integer (truncation or rounding).

**Workaround**: Not explicitly documented. May need external predicate or special function.

---

## Comparison with Aggregators

### Functions vs Aggregators

**Functions** (e.g., `fn:plus`):
- Operate on **individual values**
- Return a single result per invocation
- Used in rule bodies for computation

**Aggregators** (e.g., `fn:Sum`):
- Operate on **groups of values**
- Used in **transform blocks** with `|>`
- Compute over multiple facts

### Example Comparison

```mangle
# Function: Add two specific values
total_of_two(T) :- a(X), b(Y), T = fn:plus(X, Y).

# Aggregator: Sum all values
total_of_all(T) :-
    item(X)
    |> do fn:group_by(),
       let T = fn:Sum(X).
```

---

## Arithmetic Safety

### Overflow

Integer arithmetic may overflow:
- `fn:plus(MaxInt, 1)` → overflow
- `fn:mult(LargeNum, LargeNum)` → overflow

Mangle likely wraps or errors on overflow (not documented).

### Division by Zero

Dividing by zero likely causes:
- Runtime error
- Evaluation failure
- Undefined behavior

**Best Practice**: Ensure denominator is non-zero:
```mangle
safe_div(Result) :-
    numerator(N),
    denominator(D),
    D != 0,
    Result = fn:div(N, D).
```

### Type Mismatches

Mixing integers and floats fails:
```mangle
# ERROR: Can't mix types
bad(X) :- a(A), X = fn:plus(A, 3.0).  # A is int, 3.0 is float
```

**Fix**: Use consistent types:
```mangle
# GOOD: Both integers
good(X) :- a(A), X = fn:plus(A, 3).

# GOOD: Both floats
good(X) :- a(A), X = fn:float_plus(A, 3.0).
```

---

## Complete Arithmetic Function List

| Function | Arguments | Return | Purpose |
|----------|-----------|--------|---------|
| `fn:plus` | (int, int) | int | Addition |
| `fn:minus` | (int, int) | int | Subtraction |
| `fn:mult` | (int, int) | int | Multiplication |
| `fn:div` | (int, int) | int | Integer division |
| `fn:float_plus` | (float, float) | float | Float addition |
| `fn:float_mult` | (float, float) | float | Float multiplication |
| `fn:float_div` | (float, float) | float | Float division |
| `fn:sqrt` | (float) | float | Square root |

---

## Usage Patterns

### Computing Derived Values

```mangle
# Total cost with tax
final_price(Item, Final) :-
    base_price(Item, Base),
    tax_rate(Rate),  # e.g., 0.15 for 15%
    Tax = fn:float_mult(Base, Rate),
    Final = fn:float_plus(Base, Tax).
```

### Counters and Sequences

```mangle
# Generate sequence (bounded!)
sequence(N) :- N = 0.
sequence(N) :-
    sequence(M),
    M < 100,  # IMPORTANT: bound the recursion
    N = fn:plus(M, 1).
```

**Warning**: Unbounded counters cause non-termination!

### Conditional Arithmetic

```mangle
# Discount for large orders
discounted_price(Item, Price) :-
    base_price(Item, Base),
    quantity(Item, Q),
    Q >= 10,
    Discount = fn:div(Base, 10),  # 10% off
    Price = fn:minus(Base, Discount).

discounted_price(Item, Price) :-
    base_price(Item, Price),
    quantity(Item, Q),
    Q < 10.  # No discount
```
