# Anti-Pattern: Null Handling

## Category
Semantic Mismatch (Closed World Assumption vs Open World / Null Semantics)

## Description
Trying to use null, None, undefined, or nil when Mangle follows the Closed World Assumption: facts either exist or they don't. There is no "null value."

---

## Anti-Pattern 1: Checking for Null

### Wrong Approach
```python
if user.email is not None:
    send_email(user.email)
```

Attempting:
```mangle
# WRONG - no null concept
valid_email(User, Email) :-
    user(User, Email),
    Email != null.
```

### Why It Fails
Mangle has no `null`, `None`, `nil`, or `undefined`. A fact either exists or it doesn't.

### Correct Mangle Way
```mangle
# If the fact exists, it's valid:
Decl user_email(UserId.Type</atom>, Email.Type<string>).

valid_email(UserId, Email) :-
    user_email(UserId, Email).

# To check for missing email:
missing_email(UserId) :-
    user(UserId, _),
    not user_email(UserId, _).

# Or use an optional pattern:
email_status(UserId, /some, Email) :- user_email(UserId, Email).
email_status(UserId, /none) :-
    user(UserId, _),
    not user_email(UserId, _).
```

**Key Insight:** Absence is represented by the non-existence of facts, not by null values.

---

## Anti-Pattern 2: Nullable Fields in Schema

### Wrong Approach
```sql
CREATE TABLE user (
    id INT,
    email VARCHAR(255) NULL,
    phone VARCHAR(20) NULL
);
```

Attempting:
```mangle
# WRONG - trying to model nullable fields
Decl user(Id.Type<int>, Email.Type<null | string>, Phone.Type<null | string>).
```

### Why It Fails
No nullable types or union types with null.

### Correct Mangle Way
```mangle
# Option 1: Separate predicates for optional fields
Decl user(Id.Type<int>).
Decl user_email(Id.Type<int>, Email.Type<string>).
Decl user_phone(Id.Type<int>, Phone.Type<string>).

# User with email and phone:
user(1).
user_email(1, "alice@example.com").
user_phone(1, "555-1234").

# User with only email:
user(2).
user_email(2, "bob@example.com").
# No user_phone(2, ...) fact = phone is "missing"

# Option 2: Use optional wrapper
Decl user(
    Id.Type<int>,
    Email.Type<{/tag: /atom, /value: string}>,
    Phone.Type<{/tag: /atom, /value: string}>
).

# With values:
user(1, {/tag: /some, /value: "alice@example.com"}, {/tag: /some, /value: "555-1234"}).

# Without phone:
user(2, {/tag: /some, /value: "bob@example.com"}, {/tag: /none}).
```

**Recommendation:** Use separate predicates (Option 1). It's more idiomatic in logic programming.

---

## Anti-Pattern 3: Null Coalescing / Default Values

### Wrong Approach
```javascript
const name = user.name ?? "Guest";
const email = user.email || "no-email@example.com";
```

Attempting:
```mangle
# WRONG - no ?? or || operators
display_name(User, Name) :-
    Name = user.name ?? "Guest".
```

### Why It Fails
No null coalescing or `||` operators.

### Correct Mangle Way
```mangle
# Use multiple rules:
display_name(UserId, Name) :-
    user_name(UserId, Name).

display_name(UserId, "Guest") :-
    user(UserId),
    not user_name(UserId, _).

# Or derive default explicitly:
display_name(UserId, Name) :-
    user_name(UserId, Name).

display_name(UserId, Default) :-
    user(UserId),
    not user_name(UserId, _),
    Default = "Guest".
```

---

## Anti-Pattern 4: Null Propagation / Safe Navigation

### Wrong Approach
```kotlin
val city = user?.address?.city
```

Attempting:
```mangle
# WRONG - no ?. operator
city(User, City) :-
    City = User?.address?.city.
```

### Why It Fails
No safe navigation operator. Use explicit checks.

### Correct Mangle Way
```mangle
# Chain predicates with explicit existence checks:
Decl user(Id.Type</atom>).
Decl user_address(UserId.Type</atom>, AddressId.Type</atom>).
Decl address_city(AddressId.Type</atom>, City.Type<string>).

user_city(UserId, City) :-
    user(UserId),
    user_address(UserId, AddressId),
    address_city(AddressId, City).

# Missing any link = no derivation
# Query user_city(/user1, City)? returns nothing if any link is missing
```

---

## Anti-Pattern 5: Three-Valued Logic (True/False/Null)

### Wrong Approach
```sql
-- SQL three-valued logic
SELECT * FROM users WHERE email = NULL;  -- Always false!
SELECT * FROM users WHERE email IS NULL; -- Correct
```

Attempting:
```mangle
# WRONG - no IS NULL
has_email(User) :-
    user(User, Email),
    Email IS NOT NULL.
```

### Why It Fails
Mangle uses **two-valued logic**: a fact is either true (exists) or false (doesn't exist).

### Correct Mangle Way
```mangle
# Existence = truth
has_email(UserId) :-
    user_email(UserId, _).

# Non-existence = check via negation
no_email(UserId) :-
    user(UserId),
    not user_email(UserId, _).
```

---

## Anti-Pattern 6: Null in Comparisons

### Wrong Approach
```sql
SELECT * FROM products WHERE price > NULL;  -- Always unknown!
```

Attempting:
```mangle
# WRONG - comparing to null
expensive(Product) :-
    product(Product, Price),
    Price > null.
```

### Why It Fails
No `null` value to compare against.

### Correct Mangle Way
```mangle
# Just check if the value exists and satisfies condition:
expensive(ProductId) :-
    product_price(ProductId, Price),
    Price > 100.

# If no price fact exists, this rule simply won't derive
```

---

## Anti-Pattern 7: Null Assignment / Clearing Values

### Wrong Approach
```python
user.email = None  # Clear the email
```

Attempting:
```mangle
# WRONG - can't set to null
clear_email(User) :-
    user_email(User, null).
```

### Why It Fails
No null assignment. To "clear" a value, retract the fact.

### Correct Mangle Way
```mangle
# In Go, retract the fact:
// fact := store.Query("user_email", userId, email)
// store.Retract(fact)

# Or derive "cleared" state:
Decl email_cleared(UserId.Type</atom>).

email_cleared(UserId) :-
    clear_request(UserId),
    user_email(UserId, _).

# Then in Go, retract old email facts where email_cleared derives
```

---

## Anti-Pattern 8: Optional Return Types

### Wrong Approach
```rust
fn find_user(id: i32) -> Option<User> {
    // Returns Some(user) or None
}
```

Attempting:
```mangle
# WRONG - no Option<T> type
find_user(Id, Result) :-
    Result = Some(User) if user(Id, User) else None.
```

### Why It Fails
No `Option`, `Some`, `None` types.

### Correct Mangle Way
```mangle
# Option 1: Query returns results or empty set
Decl user(Id.Type<int>, Name.Type<string>).

# Query in Go:
// results := store.Query("user", 123, X)
// if len(results) == 0 {
//     // None
// } else {
//     // Some(results[0])
// }

# Option 2: Explicit /some and /none tags
Decl find_result(Query.Type</atom>, Tag.Type</atom>, Value.Type<string>).

find_result(/query1, /some, Name) :-
    request(/query1, UserId),
    user(UserId, Name).

find_result(/query1, /none) :-
    request(/query1, UserId),
    not user(UserId, _).
```

---

## Anti-Pattern 9: Null Object Pattern (Revisited)

### Wrong Approach
```java
User user = repository.find(id);
if (user == null) {
    user = NullUser.INSTANCE;  // Null object
}
```

Attempting:
```mangle
# WRONG - no null to check
user_or_null(Id, User) :-
    User = if user(Id, U) then U else NullUser.
```

### Why It Fails
No null checks or ternary operators.

### Correct Mangle Way
```mangle
# Always derive a result:
user_or_default(Id, Name) :- user(Id, Name).

user_or_default(Id, "Anonymous") :-
    requested_id(Id),
    not user(Id, _).

# Usage:
# Load: requested_id(/user1).
# Query: user_or_default(/user1, Name)?
# Result: Name = actual name or "Anonymous"
```

---

## Anti-Pattern 10: Database NULL vs Empty String

### Wrong Approach
```sql
-- Distinguishing NULL from empty string
SELECT * FROM users WHERE email = '';       -- Empty string
SELECT * FROM users WHERE email IS NULL;    -- NULL
```

Attempting:
```mangle
# WRONG - trying to distinguish null from ""
has_email(User) :-
    user_email(User, Email),
    Email != null,
    Email != "".
```

### Why It Fails
No null. Empty string `""` is a valid value.

### Correct Mangle Way
```mangle
# Missing email = no fact
# Empty email = fact with ""

Decl user_email(UserId.Type</atom>, Email.Type<string>).

# Valid email (non-empty):
valid_email(UserId, Email) :-
    user_email(UserId, Email),
    Email != "".

# Empty email:
empty_email(UserId) :-
    user_email(UserId, "").

# No email fact at all:
missing_email(UserId) :-
    user(UserId),
    not user_email(UserId, _).
```

---

## Anti-Pattern 11: Null in Aggregation

### Wrong Approach
```sql
-- SQL: COUNT(*) includes NULLs, COUNT(column) excludes NULLs
SELECT COUNT(*), COUNT(email) FROM users;
```

Attempting:
```mangle
# WRONG - no null handling in aggregation
total_users(Count) :- Count = COUNT(*) including null.
total_emails(Count) :- Count = COUNT(email) excluding null.
```

### Why It Fails
No null to include/exclude. Aggregation operates on existing facts only.

### Correct Mangle Way
```mangle
# Count all users:
total_users(Count) :-
    user(UserId)
    |> do fn:group_by()
    |> let Count = fn:Count(UserId).

# Count users with email:
total_emails(Count) :-
    user_email(UserId, _)
    |> do fn:group_by()
    |> let Count = fn:Count(UserId).

# Count users without email:
total_missing_emails(Count) :-
    missing_email(UserId)  # From earlier definition
    |> do fn:group_by()
    |> let Count = fn:Count(UserId).
```

---

## Anti-Pattern 12: Null as Sentinel Value

### Wrong Approach
```c
int find_index(int arr[], int target) {
    // ...
    return -1;  // Sentinel for "not found"
}

// Or using NULL:
User* find_user(int id) {
    // ...
    return NULL;  // Not found
}
```

Attempting:
```mangle
# WRONG - using null as sentinel
find_index(Arr, Target, Index) :-
    search(Arr, Target, Index),
    Index = null if not found.
```

### Why It Fails
No null sentinel. Use explicit /found and /not_found tags.

### Correct Mangle Way
```mangle
# Option 1: Just don't derive if not found
Decl search_result(Query.Type</atom>, Index.Type<int>).

search_result(/q1, 5) :-
    search_request(/q1, Target),
    array_contains(5, Target).

# If not found, no search_result fact derives

# Option 2: Explicit tags
Decl search_status(Query.Type</atom>, Status.Type</atom>, Index.Type<int>).

search_status(/q1, /found, Index) :-
    search_request(/q1, Target),
    array_contains(Index, Target).

search_status(/q1, /not_found, -1) :-
    search_request(/q1, Target),
    not array_contains(_, Target).
```

---

## Closed World Assumption: Core Principle

**Open World Assumption (OWA):** "We don't know if something is true or false unless explicitly stated. Null represents unknown."

**Closed World Assumption (CWA):** "Anything not derivable from the facts is false. No unknown state."

### Example

Database with OWA:
```sql
user(id=1, email=NULL)  -- Unknown email
```

Mangle with CWA:
```mangle
user(1).  # User exists
# No user_email(1, ...) fact = user 1 has no email (not unknown, definitively absent)
```

---

## Migration Checklist

When translating null-aware code to Mangle:

- [ ] Replace `if (x != null)` with checking if fact exists
- [ ] Replace `x = null` with retracting the fact (in Go)
- [ ] Replace nullable fields with separate predicates
- [ ] Replace `x ?? default` with multiple rules
- [ ] Replace `x?.y?.z` with chained predicates
- [ ] Replace `IS NULL` checks with negation: `not pred(_)`
- [ ] Replace null comparisons with existence checks
- [ ] Replace `Option<T>` with tagged results or empty query results
- [ ] Replace null objects with default derivation rules
- [ ] Distinguish empty string `""` from missing fact
- [ ] Aggregation naturally excludes missing facts
- [ ] Replace sentinel nulls with explicit /found, /not_found tags
- [ ] Remember: absent = false, not unknown

---

## Pattern: Modeling Optional Data

Three approaches for optional/nullable data:

### 1. Separate Predicates (Recommended)

```mangle
Decl user(Id.Type<int>, Name.Type<string>).
Decl user_email(Id.Type<int>, Email.Type<string>).
Decl user_phone(Id.Type<int>, Phone.Type<string>).

# User might have email and/or phone, or neither
```

**Pros:** Idiomatic, efficient, easy to query
**Cons:** More predicates to manage

### 2. Tagged Unions

```mangle
Decl user(
    Id.Type<int>,
    Name.Type<string>,
    Email.Type<{/tag: /atom, /value: string}>
).

# Email present:
user(1, "Alice", {/tag: /some, /value: "alice@example.com"}).

# Email absent:
user(2, "Bob", {/tag: /none}).
```

**Pros:** Explicit optional markers
**Cons:** More verbose, need to unwrap

### 3. Derivation-based Defaults

```mangle
Decl user(Id.Type<int>, Name.Type<string>).
Decl user_email(Id.Type<int>, Email.Type<string>).

# Derive display email (with default):
display_email(Id, Email) :- user_email(Id, Email).
display_email(Id, "no-email@example.com") :-
    user(Id, _),
    not user_email(Id, _).
```

**Pros:** Clean query interface
**Cons:** Defaults in derivation rules

---

## Pro Tip: Think Existence, Not Nullability

| Null-Aware Thinking | Existence-Based Thinking (Mangle) |
|---------------------|-----------------------------------|
| "Does x have a value?" | "Does a fact exist for x?" |
| "Is email null?" | "Is there a user_email fact?" |
| "Set email to null" | "Retract the user_email fact" |
| "email ?? 'default'" | "Derive email if exists, else default" |
| "Three-valued logic" | "Two-valued logic: exists or doesn't" |

When in doubt: **No fact = No value = False in CWA**
