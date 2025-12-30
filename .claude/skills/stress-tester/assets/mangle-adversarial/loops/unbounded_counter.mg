# Unbounded Counter Tests
# Error Type: Counter/sequence generation without termination
# Expected: Infinite sequence generation, memory exhaustion

# Test 1: Simple unbounded counter
Decl count(N.Type<int>).
count(0).
# ERROR: Generates 0, 1, 2, 3, ... infinitely
count(N) :- count(M), N = fn:plus(M, 1).

# Test 2: Counter with step
Decl step_count(N.Type<int>).
step_count(0).
# ERROR: Generates 0, 5, 10, 15, ... infinitely
step_count(N) :- step_count(M), N = fn:plus(M, 5).

# Test 3: Fibonacci-style unbounded
Decl fib(N.Type<int>, Val.Type<int>).
fib(0, 0).
fib(1, 1).
# ERROR: Generates infinite Fibonacci sequence
fib(N, Val) :-
  fib(N1, V1),
  fib(N2, V2),
  N2 = fn:plus(N1, 1),
  N = fn:plus(N2, 1),
  Val = fn:plus(V1, V2).

# Test 4: Factorial unbounded
Decl factorial(N.Type<int>, Result.Type<int>).
factorial(0, 1).
# ERROR: Computes factorial for all N infinitely
factorial(N, Result) :-
  factorial(Prev, PrevResult),
  N = fn:plus(Prev, 1),
  Result = fn:times(N, PrevResult).

# Test 5: Power sequence unbounded
Decl power_of_two(N.Type<int>, Val.Type<int>).
power_of_two(0, 1).
# ERROR: Generates 2^0, 2^1, 2^2, ... infinitely
power_of_two(N, Val) :-
  power_of_two(Prev, PrevVal),
  N = fn:plus(Prev, 1),
  Val = fn:times(PrevVal, 2).

# Test 6: Counter with condition (still problematic)
Decl bounded_count(N.Type<int>).
bounded_count(0).
# ERROR: Generates 0..999 then tries to continue
# Will fill fixed point with 1000 values
bounded_count(N) :- bounded_count(M), N = fn:plus(M, 1), N < 1000.

# Test 7: Negative counter (descending)
Decl countdown(N.Type<int>).
countdown(100).
# ERROR: Generates 100, 99, 98, ... infinitely downward
countdown(N) :- countdown(M), N = fn:minus(M, 1).

# Test 8: Multiple counters interleaved
Decl counter_a(N.Type<int>).
Decl counter_b(N.Type<int>).
counter_a(0).
counter_b(1000).
# ERROR: Both count infinitely
counter_a(N) :- counter_a(M), N = fn:plus(M, 1).
counter_b(N) :- counter_b(M), N = fn:minus(M, 1).

# Test 9: Counter with aggregation
Decl sequence(N.Type<int>).
sequence(0).
# ERROR: Sequence grows infinitely
sequence(N) :- sequence(M), N = fn:plus(M, 1).
# Trying to aggregate infinite predicate
sum_sequence(Total) :-
  sequence(S) |>
  do fn:group_by(),
  let Total = fn:Sum(S).

# Test 10: Geometric progression
Decl geometric(N.Type<int>).
geometric(1).
# ERROR: 1, 2, 4, 8, 16, ... infinite exponential growth
geometric(N) :- geometric(M), N = fn:times(M, 2).

# Test 11: Collatz-like sequence (might terminate, might not)
Decl collatz(N.Type<int>).
collatz(27).  # Famous starting point
# ERROR: May or may not terminate (unsolved problem)
collatz(N) :- collatz(M), N = fn:div(M, 2), fn:mod(M, 2) = 0.
collatz(N) :- collatz(M), N = fn:plus(fn:times(M, 3), 1), fn:mod(M, 2) = 1.

# Test 12: Correct bounded counter (finite domain)
Decl digit(D.Type<int>).
digit(0).
digit(1).
digit(2).
digit(3).
digit(4).
digit(5).
digit(6).
digit(7).
digit(8).
digit(9).
# CORRECT: Finite base facts, no recursion
sum_digits(S) :-
  digit(D) |>
  do fn:group_by(),
  let S = fn:Sum(D).

# Test 13: Range generation (unbounded)
Decl range(Start.Type<int>, End.Type<int>, Current.Type<int>).
range(0, 100, 0).
# ERROR: Even with end bound, generates infinitely if not careful
range(Start, End, Current) :-
  range(Start, End, Prev),
  Current = fn:plus(Prev, 1).
# This will try to go beyond End

# Test 14: Timestamp sequence
Decl tick(T.Type<int>).
tick(0).
# ERROR: Simulating time ticks, infinite
tick(T) :- tick(Prev), T = fn:plus(Prev, 1).

# Test 15: ID generator
Decl next_id(ID.Type<int>).
next_id(1).
# ERROR: Generates sequential IDs infinitely
next_id(ID) :- next_id(Prev), ID = fn:plus(Prev, 1).
