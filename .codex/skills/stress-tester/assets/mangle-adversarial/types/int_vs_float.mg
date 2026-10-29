# Int vs Float Type Tests
# Error Type: Mixing integer and float types, type coercion assumptions
# Expected: Type errors, arithmetic failures
# DIALECT NOTE: this engine has ONE numeric type (/number): 10 and 10.5 are
# the same type, so int/float mixing is not an error here at all. Every test
# below is a VALID program; the file documents pitfalls that do not apply
# (and the one runtime question, fn:div semantics, that does).

# Test 1: Declaring as int, inserting as float
Decl count(C) bound [/number].
count(10.5).  # VALID here: no int/float split, both are /number
count(20).    # Correct

# Test 2: Declaring as float, inserting as int
Decl temperature(T) bound [/float].
temperature(98).    # VALID here: 98 and 98.0 are the same type
temperature(98.6).  # Correct

# Test 3: Mixed int/float in same predicate
Decl value(V) bound [/number].
value(10).
value(10.5).  # VALID here: mixed literals are one type

# Test 4: Integer division expecting float result
Decl numerator(N) bound [/number].
Decl denominator(D) bound [/number].
numerator(10).
denominator(3).
# VALID program; RUNTIME question: does fn:div(10, 3) yield 3 or 3.333?
# (Head annotations like R.Type<float> are not Mangle syntax.)
ratio(R) :- numerator(N), denominator(D), R = fn:div(N, D).

# Test 5: Float in integer context
Decl index(I) bound [/number].
index(1.5).  # VALID here: still just a /number

# Test 6: Comparison type mismatch
Decl int_val(V) bound [/number].
Decl float_val(V) bound [/float].
int_val(10).
float_val(10.0).
# VALID here: X and Y are both /numbers, so this unifies 10 with 10.0.
# (Whether 10 equals 10.0 at runtime is engine semantics, not a static error.)
same() :- int_val(X), float_val(Y), X = Y.

# Test 7: Aggregation type confusion
Decl amount(A) bound [/number].
amount(100).
amount(200).
# There is no fn:Sum in this engine (and no int/float split to violate).
# VALID counting form:
total(T) :-
  amount(A) |>
  do fn:group_by(),
  let T = fn:count().

# Test 8: Function return type mismatch
Decl price(P) bound [/number].
price(100).
# VALID here (fn:times does not exist; fn:mult does; no type split).
discounted(D) :- price(P), D = fn:mult(P, 0.9).

# Test 9: No automatic type promotion
Decl length(L) bound [/number].
Decl width(W) bound [/float].
length(10).
width(5.5).
# VALID here: one numeric type, real multiply function.
area(A) :- length(L), width(W), A = fn:mult(L, W).

# Test 10: String to number confusion
Decl quantity(Q) bound [/number].
quantity("42").  # WRONG DATA (silent): facts are not type-checked; still a string at query time

# Test 11: Correct separate handling
Decl int_data(I) bound [/number].
Decl float_data(F) bound [/float].
int_data(10).
float_data(10.0).
# CORRECT: Keep types separate

# Test 12: Explicit conversion needed (if function exists)
Decl celsius(C) bound [/number].
celsius(25).
# VALID here: no conversion needed or possible; one numeric type.
fahrenheit(F) :- celsius(C), F = fn:plus(fn:mult(C, 1.8), 32.0).
