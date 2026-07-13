# Hallucinated Function Tests
# Error Type: Using non-existent functions from other languages
# Expected: Undefined function errors

# Test 1: Python-style string methods
Decl text(T.Type<string>).
text("hello world").
uppercase(U) :- text(T), U = fn:upper(T).        # ERROR: Might be fn:string_to_upper
split(S) :- text(T), S = fn:split(T, " ").       # ERROR: Might not exist
strip(S) :- text(T), S = fn:strip(T).            # ERROR: Likely doesn't exist

# Test 2: JavaScript-style array methods
Decl numbers(N.Type<list>).
numbers([1, 2, 3, 4, 5]).
filtered(F) :- numbers(N), F = fn:filter(N, fn:gt(3)).  # ERROR: No filter function
mapped(M) :- numbers(N), M = fn:map(N, fn:double).      # ERROR: No map function
reduced(R) :- numbers(N), R = fn:reduce(N, fn:plus).    # ERROR: No reduce function

# Test 3: SQL-style date functions
Decl timestamp(T.Type<string>).
timestamp("2024-01-01").
year(Y) :- timestamp(T), Y = fn:year(T).           # ERROR: No date functions
month(M) :- timestamp(T), M = fn:month(T).         # ERROR: No date functions
date_diff(D) :- D = fn:datediff("2024-01-01", "2024-12-31").  # ERROR

# Test 4: Math functions that don't exist
Decl value(V.Type<float>).
value(3.14159).
rounded(R) :- value(V), R = fn:round(V).           # ERROR: Might not exist
ceiling(C) :- value(V), C = fn:ceil(V).            # ERROR: Might not exist
floored(F) :- value(V), F = fn:floor(V).           # ERROR: Might not exist
power(P) :- value(V), P = fn:pow(V, 2).            # ERROR: Might not exist
sqrt(S) :- value(V), S = fn:sqrt(V).               # ERROR: Might not exist

# Test 5: String manipulation (advanced)
Decl message(M.Type<string>).
message("Hello World").
substring(S) :- message(M), S = fn:substring(M, 0, 5).     # ERROR: Might not exist
replaced(R) :- message(M), R = fn:replace(M, "Hello", "Hi").  # ERROR
trimmed(T) :- message(M), T = fn:trim(M).                  # ERROR

# Test 6: Collection functions
Decl items(I.Type<list>).
items([/a, /b, /c, /b]).
unique(U) :- items(I), U = fn:unique(I).           # ERROR: No unique function
length(L) :- items(I), L = fn:length(I).           # ERROR: Might be fn:list:length
contains(C) :- items(I), C = fn:contains(I, /b).   # ERROR: Different syntax likely

# Test 7: Type conversion functions
Decl number_string(S.Type<string>).
number_string("42").
to_int(I) :- number_string(S), I = fn:parseInt(S).     # ERROR: Wrong name
to_float(F) :- number_string(S), F = fn:parseFloat(S). # ERROR: Wrong name
to_string(S) :- S = fn:toString(42).                   # ERROR: Wrong name

# Test 8: Regular expressions (very unlikely to exist)
Decl pattern(P.Type<string>).
pattern("^[a-z]+$").
matches(M) :- pattern(P), M = fn:regex_match("hello", P).  # ERROR
extract(E) :- pattern(P), E = fn:regex_extract("test", P). # ERROR

# Test 9: JSON functions
Decl json_str(J.Type<string>).
json_str("{\"key\": \"value\"}").
parsed(P) :- json_str(J), P = fn:json_parse(J).        # ERROR
stringified(S) :- S = fn:json_stringify({ /key: "value" }).  # ERROR

# Test 10: Random/UUID functions
random_num(R) :- R = fn:random().                  # ERROR: Non-deterministic
uuid(U) :- U = fn:uuid().                          # ERROR: Non-deterministic
rand_int(R) :- R = fn:random_int(1, 100).         # ERROR

# Test 11: File/IO functions (very wrong - Mangle is pure)
read_file(Content) :- Content = fn:read_file("/path/to/file").  # ERROR: No IO
write_file() :- fn:write_file("/path", "data").                 # ERROR: No IO

# Test 12: Network functions (extremely wrong)
fetch(Data) :- Data = fn:http_get("http://api.example.com").   # ERROR: No network

# Test 13: Correct Mangle builtin usage (for comparison)
Decl x(X.Type<int>).
Decl y(Y.Type<int>).
x(10).
y(5).
# CORRECT: These are real Mangle functions
sum(S) :- x(X), y(Y), S = fn:plus(X, Y).
product(P) :- x(X), y(Y), P = fn:times(X, Y).
difference(D) :- x(X), y(Y), D = fn:minus(X, Y).
quotient(Q) :- x(X), y(Y), Q = fn:div(X, Y).
