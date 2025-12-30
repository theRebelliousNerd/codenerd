# Int vs Float Type Tests
# Error Type: Mixing integer and float types, type coercion assumptions
# Expected: Type errors, arithmetic failures

# Test 1: Declaring as int, inserting as float
Decl count(C.Type<int>).
count(10.5).  # ERROR: Should be integer
count(20).    # Correct

# Test 2: Declaring as float, inserting as int
Decl temperature(T.Type<float>).
temperature(98).    # ERROR: Should be 98.0
temperature(98.6).  # Correct

# Test 3: Mixed int/float in same predicate
Decl value(V.Type<int>).
value(10).
value(10.5).  # ERROR: Type inconsistency

# Test 4: Integer division expecting float result
Decl numerator(N.Type<int>).
Decl denominator(D.Type<int>).
numerator(10).
denominator(3).
# ERROR: Result will be int 3, not float 3.333...
ratio(R.Type<float>) :- numerator(N), denominator(D), R = fn:div(N, D).

# Test 5: Float in integer context
Decl index(I.Type<int>).
index(1.5).  # ERROR: Index must be integer

# Test 6: Comparison type mismatch
Decl int_val(V.Type<int>).
Decl float_val(V.Type<float>).
int_val(10).
float_val(10.0).
# ERROR: Comparing different types
same() :- int_val(X), float_val(Y), X = Y.

# Test 7: Aggregation type confusion
Decl amount(A.Type<int>).
amount(100).
amount(200).
# ERROR: Sum of ints assigned to float
total(T.Type<float>) :-
  amount(A) |>
  do fn:group_by(),
  let T = fn:Sum(A).  # Sum of ints is int, not float

# Test 8: Function return type mismatch
Decl price(P.Type<int>).
price(100).
# ERROR: Multiplying int by float literal
discounted(D.Type<float>) :- price(P), D = fn:times(P, 0.9).

# Test 9: No automatic type promotion
Decl length(L.Type<int>).
Decl width(W.Type<float>).
length(10).
width(5.5).
# ERROR: Mixing int and float in arithmetic
area(A) :- length(L), width(W), A = fn:times(L, W).

# Test 10: String to number confusion
Decl quantity(Q.Type<int>).
quantity("42").  # ERROR: String not convertible to int automatically

# Test 11: Correct separate handling
Decl int_data(I.Type<int>).
Decl float_data(F.Type<float>).
int_data(10).
float_data(10.0).
# CORRECT: Keep types separate

# Test 12: Explicit conversion needed (if function exists)
Decl celsius(C.Type<int>).
celsius(25).
# Would need explicit int_to_float function
fahrenheit(F.Type<float>) :- celsius(C), F = fn:plus(fn:times(C, 1.8), 32.0).  # ERROR
