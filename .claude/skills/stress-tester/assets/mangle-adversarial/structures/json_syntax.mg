# JSON Syntax Error Tests
# Error Type: Using JSON syntax instead of Mangle struct syntax
# Expected: Parse errors due to incorrect structure literals

# Test 1: JSON-style struct (no atom keys)
Decl config(C.Type<struct>).
# ERROR: Keys should be atoms with /
config({"host": "localhost", "port": 8080}).

# Test 2: Trailing comma (JSON allows, Mangle might not)
Decl settings(S.Type<struct>).
# ERROR: Trailing comma
settings({ /host: "localhost", /port: 8080, }).

# Test 3: Single quotes instead of double quotes
Decl data(D.Type<struct>).
# ERROR: Single quotes not valid for strings
data({ /name: 'alice' }).

# Test 4: No quotes on string values (JavaScript object literal)
Decl user(U.Type<struct>).
# ERROR: String values need quotes
user({ /name: alice, /age: 30 }).

# Test 5: Colon spacing differences
Decl spacing(S.Type<struct>).
# This might work, but inconsistent with Mangle style
spacing({/key:/value}).  # No spaces
spacing({ /key : /value }).  # Excessive spaces

# Test 6: Nested JSON without atom keys
Decl nested(N.Type<struct>).
# ERROR: Inner keys not atoms
nested({
  "database": {
    "host": "localhost",
    "port": 5432
  }
}).

# Test 7: Array with JSON syntax (probably OK)
Decl items(I.Type<list>).
items([1, 2, 3]).  # This is correct
# But mixing with wrong struct syntax:
Decl mixed(M.Type<struct>).
mixed({"items": [1, 2, 3]}).  # ERROR: Key not atom

# Test 8: Boolean true/false (lowercase vs atoms)
Decl flags(F.Type<struct>).
# ERROR: Might need /true or different syntax
flags({ /enabled: true, /visible: false }).
# Should be: flags({ /enabled: /true, /visible: /false }). ?

# Test 9: Null value
Decl nullable(N.Type<struct>).
# ERROR: null might not exist in Mangle
nullable({ /value: null }).

# Test 10: Number keys (JSON allows string numbers)
Decl indexed(I.Type<struct>).
# ERROR: Numeric keys as strings
indexed({ "0": /first, "1": /second }).

# Test 11: Multi-line JSON formatting
Decl formatted(F.Type<struct>).
# Mangle might not support this formatting
formatted({
  /name: "alice",
  /age: 30,
  /active: /true
}).

# Test 12: Comments in JSON (not valid JSON, but attempted)
Decl commented(C.Type<struct>).
commented({
  /key: /value  // This is a comment - ERROR
}).

# Test 13: Correct Mangle struct syntax
Decl proper(P.Type<struct>).
# CORRECT: Atom keys with /, proper spacing
proper({ /host: "localhost", /port: 8080 }).

# Test 14: Escaped characters in JSON
Decl escaped(E.Type<struct>).
# Might work, but testing edge case
escaped({ /path: "C:\\Users\\alice\\file.txt" }).

# Test 15: Unicode in JSON
Decl unicode(U.Type<struct>).
# Testing if unicode works
unicode({ /emoji: "🚀", /chinese: "你好" }).

# Test 16: Empty object
Decl empty(E.Type<struct>).
empty({}).  # Should be valid

# Test 17: Comparing to JSON string
Decl json_string(J.Type<string>).
json_string("{\"key\": \"value\"}").
# ERROR: Can't parse JSON string into struct automatically
parse_json(S) :- json_string(J), S = fn:json_parse(J).  # Likely no such function
