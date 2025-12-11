# Anti-Pattern: Side Effects

## Category
Paradigm Mismatch (Pure Logic vs Effectful Computation)

## Description
Expecting side effects like I/O, logging, database updates, network requests, or state changes from within Mangle rules when Mangle is purely functional.

---

## Anti-Pattern 1: Printing/Logging from Rules

### Wrong Approach
```python
def process(x):
    print(f"Processing {x}")  # Side effect
    return x * 2
```

Attempting:
```mangle
# WRONG - no print/log statements
process(X, Result) :-
    input(X),
    print("Processing", X),
    Result = fn:times(X, 2).
```

### Why It Fails
Mangle rules are **pure**. No `print`, `log`, `console.log`, or any output operations.

### Correct Mangle Way
```mangle
# Just derive facts:
process_result(X, Result) :-
    input(X),
    Result = fn:times(X, 2).

# Log in Go after querying:
// results := store.Query("process_result", X, Result)
// for _, r := range results {
//     log.Printf("Processing %v -> %v", r.X, r.Result)
// }

# Or derive "log messages" as facts to extract later:
Decl log_entry(Level.Type</atom>, Message.Type<string>).

log_entry(/info, Msg) :-
    process_result(X, Result),
    fn:string_concat(["Processed ", fn:to_string(X)], Msg).

# Query log entries in Go and actually log them
```

---

## Anti-Pattern 2: File I/O

### Wrong Approach
```python
def save_result(data):
    with open("output.txt", "w") as f:
        f.write(data)
```

Attempting:
```mangle
# WRONG - no file I/O
save_result(Data) :-
    result(Data),
    write_file("output.txt", Data).
```

### Why It Fails
No file operations exist in Mangle. I/O must happen in Go.

### Correct Mangle Way
```mangle
# Derive what should be saved:
Decl result_to_save(Id.Type</atom>, Data.Type<string>).

result_to_save(/r1, "Important data").

# In Go, execute the I/O:
// results := store.Query("result_to_save", id, data)
// for _, r := range results {
//     os.WriteFile("output.txt", []byte(r.Data), 0644)
// }

# For reading files - load as facts:
// data, _ := os.ReadFile("input.txt")
// store.Add(engine.NewAtom("file_content", engine.String(string(data))))

# Then query in Mangle:
Decl file_content(Content.Type<string>).

process_file(Result) :-
    file_content(Content),
    # ... process Content ...
    Result = Content.
```

---

## Anti-Pattern 3: Database Operations

### Wrong Approach
```python
def save_user(user):
    db.execute("INSERT INTO users VALUES (?, ?)", user.id, user.name)
```

Attempting:
```mangle
# WRONG - no database operations
save_user(Id, Name) :-
    user(Id, Name),
    db_insert("users", Id, Name).
```

### Why It Fails
No database operations in Mangle rules.

### Correct Mangle Way
```mangle
# Derive what should be saved:
Decl user_to_persist(Id.Type</atom>, Name.Type<string>).

user_to_persist(Id, Name) :-
    user(Id, Name),
    not already_persisted(Id).

# In Go, execute database operations:
// results := store.Query("user_to_persist", id, name)
// for _, r := range results {
//     db.Exec("INSERT INTO users VALUES (?, ?)", r.Id, r.Name)
//     // Mark as persisted
//     store.Add(engine.NewAtom("already_persisted", r.Id))
// }

# For reading from database - load as facts:
// rows := db.Query("SELECT id, name FROM users")
// for rows.Next() {
//     var id, name string
//     rows.Scan(&id, &name)
//     store.Add(engine.NewAtom("user", engine.Atom(id), engine.String(name)))
// }
```

---

## Anti-Pattern 4: Network Requests

### Wrong Approach
```javascript
async function fetchData(url) {
    const response = await fetch(url);
    return response.json();
}
```

Attempting:
```mangle
# WRONG - no HTTP requests
fetch_data(Url, Data) :-
    api_url(Url),
    Data = http_get(Url).
```

### Why It Fails
No network operations in Mangle.

### Correct Mangle Way
```mangle
# Derive what URLs to fetch:
Decl url_to_fetch(Url.Type<string>).

url_to_fetch("https://api.example.com/users").

# In Go, perform the request:
// results := store.Query("url_to_fetch", url)
// for _, r := range results {
//     resp, _ := http.Get(r.Url)
//     body, _ := io.ReadAll(resp.Body)
//     // Parse and load as facts
//     store.Add(engine.NewAtom("api_response", engine.String(r.Url), engine.String(string(body))))
// }

# Query the loaded response:
Decl api_response(Url.Type<string>, Body.Type<string>).

process_response(Data) :-
    api_response(_, Body),
    # ... process Body ...
    Data = Body.
```

---

## Anti-Pattern 5: Random Number Generation

### Wrong Approach
```python
def random_value():
    return random.randint(1, 100)
```

Attempting:
```mangle
# WRONG - no random number generation
random_value(R) :- R = random(1, 100).
```

### Why It Fails
Mangle is **deterministic**. Same facts always produce same results. No random().

### Correct Mangle Way
```mangle
# Generate random numbers in Go and load as facts:
// for i := 0; i < 10; i++ {
//     r := rand.Intn(100) + 1
//     store.Add(engine.NewAtom("random_value", engine.Number(i), engine.Number(r)))
// }

Decl random_value(Id.Type<int>, Value.Type<int>).

# Use the loaded random values:
process_random(Id, Result) :-
    random_value(Id, Val),
    Result = fn:times(Val, 2).
```

**Key Insight:** Randomness is an external input, loaded as facts. Derivation is deterministic.

---

## Anti-Pattern 6: Current Time/Date

### Wrong Approach
```python
def timestamp():
    return datetime.now()

def is_expired(date):
    return date < datetime.now()
```

Attempting:
```mangle
# WRONG - no current time
is_expired(Date) :-
    expiry_date(Date),
    Date < now().
```

### Why It Fails
No `now()`, `current_time()`, or date functions. Time must be injected.

### Correct Mangle Way
```mangle
# Load current time as a fact in Go:
// now := time.Now().Unix()
// store.Add(engine.NewAtom("current_time", engine.Number(now)))

Decl current_time(Timestamp.Type<int>).
Decl expiry_date(Id.Type</atom>, Timestamp.Type<int>).

is_expired(Id) :-
    current_time(Now),
    expiry_date(Id, Expiry),
    Expiry < Now.

# Each evaluation uses the loaded "current time" - deterministic!
```

---

## Anti-Pattern 7: Modifying Global State

### Wrong Approach
```javascript
let cache = {};

function addToCache(key, value) {
    cache[key] = value;  // Mutate global state
}
```

Attempting:
```mangle
# WRONG - no global state mutation
add_to_cache(Key, Value) :-
    cache_update(Key, Value),
    global_cache[Key] = Value.
```

### Why It Fails
No global variables or mutable state.

### Correct Mangle Way
```mangle
# Cache is just facts:
Decl cache(Key.Type<string>, Value.Type<int>).

# "Add to cache" = add a fact (in Go):
// store.Add(engine.NewAtom("cache", engine.String(key), engine.Number(value)))

# Query cache:
cached_value(Key, Value) :- cache(Key, Value).

# "Invalidate cache" = retract all cache facts:
// results := store.Query("cache", _, _)
// for _, r := range results {
//     store.Retract(r)
// }
```

---

## Anti-Pattern 8: Sending Messages/Events

### Wrong Approach
```python
def notify_user(user_id, message):
    send_email(user_id, message)  # Side effect
    send_sms(user_id, message)    # Side effect
```

Attempting:
```mangle
# WRONG - no message sending
notify_user(UserId, Msg) :-
    user(UserId),
    send_email(UserId, Msg),
    send_sms(UserId, Msg).
```

### Why It Fails
No side effects. Can't send emails, SMS, push notifications, etc.

### Correct Mangle Way
```mangle
# Derive what notifications should be sent:
Decl notification(UserId.Type</atom>, Type.Type</atom>, Message.Type<string>).

notification(UserId, /email, Msg) :-
    user(UserId),
    alert_message(UserId, Msg),
    user_prefers_email(UserId).

notification(UserId, /sms, Msg) :-
    user(UserId),
    alert_message(UserId, Msg),
    user_prefers_sms(UserId).

# In Go, execute the notifications:
// results := store.Query("notification", userId, notifType, message)
// for _, r := range results {
//     switch r.Type {
//     case "email":
//         sendEmail(r.UserId, r.Message)
//     case "sms":
//         sendSMS(r.UserId, r.Message)
//     }
// }
```

---

## Anti-Pattern 9: Throwing Exceptions

### Wrong Approach
```java
public void validate(int x) {
    if (x < 0) {
        throw new IllegalArgumentException("x must be positive");
    }
}
```

Attempting:
```mangle
# WRONG - no exceptions
validate(X) :-
    X < 0,
    throw("x must be positive").
```

### Why It Fails
No exceptions or error throwing in rules.

### Correct Mangle Way
```mangle
# Derive validation results:
Decl validation_error(Id.Type</atom>, Message.Type<string>).

validation_error(Id, "x must be positive") :-
    input(Id, X),
    X < 0.

valid_input(Id, X) :-
    input(Id, X),
    X >= 0.

# In Go, check for errors:
// errors := store.Query("validation_error", id, message)
// if len(errors) > 0 {
//     return fmt.Errorf("%v", errors[0].Message)
// }
```

---

## Anti-Pattern 10: Calling External APIs

### Wrong Approach
```python
def process_payment(amount):
    response = stripe.charge(amount)  # External API call
    return response.success
```

Attempting:
```mangle
# WRONG - no API calls
process_payment(Amount, Success) :-
    payment_request(Amount),
    Success = stripe_charge(Amount).
```

### Why It Fails
No external function calls from rules.

### Correct Mangle Way
```mangle
# Derive what payments to process:
Decl payment_to_process(Id.Type</atom>, Amount.Type<int>).

payment_to_process(Id, Amount) :-
    payment_request(Id, Amount),
    not payment_processed(Id).

# In Go, call the API:
// results := store.Query("payment_to_process", id, amount)
// for _, r := range results {
//     success := stripe.Charge(r.Amount)
//     if success {
//         store.Add(engine.NewAtom("payment_processed", r.Id))
//         store.Add(engine.NewAtom("payment_success", r.Id))
//     } else {
//         store.Add(engine.NewAtom("payment_failed", r.Id))
//     }
// }

# Query results in Mangle:
Decl payment_success(Id.Type</atom>).
Decl payment_failed(Id.Type</atom>).

successful_payment(Id, Amount) :-
    payment_request(Id, Amount),
    payment_success(Id).
```

---

## Anti-Pattern 11: Timing/Sleeping

### Wrong Approach
```python
def delayed_action():
    time.sleep(5)  # Wait 5 seconds
    return "done"
```

Attempting:
```mangle
# WRONG - no sleep/delay
delayed_result(R) :-
    sleep(5),
    R = /done.
```

### Why It Fails
No timing or delay operations. Mangle evaluation is instant (until fixed point).

### Correct Mangle Way
```mangle
# Model time explicitly:
Decl event(Time.Type<int>, Type.Type</atom>).

event(0, /start).
event(5, /trigger).
event(10, /complete).

# Derive what should happen at each time:
action_at_time(Time, /delayed_action) :-
    event(Time, /trigger),
    Time >= 5.

# In Go, execute at the right time:
// time.Sleep(5 * time.Second)
// store.Add(engine.NewAtom("current_time", engine.Number(5)))
// results := store.Query("action_at_time", 5, action)
```

---

## Anti-Pattern 12: Callbacks/Event Handlers

### Wrong Approach
```javascript
button.onClick(() => {
    console.log("Clicked!");
});
```

Attempting:
```mangle
# WRONG - no callbacks
on_click(Button) :-
    button(Button),
    register_callback(Button, fn() { log("Clicked!") }).
```

### Why It Fails
No callback registration or event handlers.

### Correct Mangle Way
```mangle
# Derive what handlers should trigger:
Decl click_event(Button.Type</atom>, Time.Type<int>).
Decl handler(Button.Type</atom>, Action.Type</atom>).

handler(/btn1, /log_message).

should_execute(Action) :-
    click_event(Button, _),
    handler(Button, Action).

# In Go, load events and execute handlers:
// Load click event
// store.Add(engine.NewAtom("click_event", engine.Atom("/btn1"), engine.Number(time.Now().Unix())))
// Query what should execute
// actions := store.Query("should_execute", action)
// for _, a := range actions {
//     executeAction(a.Action)
// }
```

---

## Pattern: Effect Systems in Mangle

### The Standard Pattern

1. **Derive effect descriptions** (what should happen)
2. **Execute effects in Go** (actually do it)
3. **Load results as facts** (feed back to Mangle)

```mangle
# Step 1: Derive effects
Decl file_to_write(Path.Type<string>, Content.Type<string>).

file_to_write("/tmp/output.txt", Data) :-
    processed_data(Data).
```

```go
// Step 2: Execute in Go
results := store.Query("file_to_write", path, content)
for _, r := range results {
    err := os.WriteFile(r.Path, []byte(r.Content), 0644)

    // Step 3: Load results
    if err == nil {
        store.Add(engine.NewAtom("write_success", engine.String(r.Path)))
    } else {
        store.Add(engine.NewAtom("write_error", engine.String(r.Path), engine.String(err.Error())))
    }
}
```

```mangle
# Use results
Decl write_success(Path.Type<string>).
Decl write_error(Path.Type<string>, Error.Type<string>).

completed(Path) :- write_success(Path).
failed(Path, Err) :- write_error(Path, Err).
```

---

## Key Principle: Purity

| Effectful Code | Pure Mangle |
|----------------|-------------|
| `print(x)` | Derive log facts, print in Go |
| `write_file(path, data)` | Derive file_to_write facts, execute in Go |
| `db.insert(...)` | Derive data_to_persist facts, insert in Go |
| `http.get(url)` | Derive urls_to_fetch facts, fetch in Go |
| `random()` | Load random values as facts |
| `now()` | Load current time as fact |
| `cache[x] = y` | Add cache facts |
| `send_email(...)` | Derive notification facts, send in Go |
| `throw new Error()` | Derive error facts, handle in Go |
| `api.call(...)` | Derive API call facts, execute in Go |
| `sleep(n)` | Model time explicitly |
| `register_callback(...)` | Derive handler facts, execute in Go |

---

## External Predicates: The Exception

Mangle *does* support **external predicates** implemented in Go:

```go
// Define external predicate
func readFile(query engine.Query, cb func(engine.Fact)) error {
    path := query.Args[0].(engine.String).Value
    content, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    cb(engine.NewAtom(query.Predicate, query.Args[0], engine.String(string(content))))
    return nil
}

// Register it
store.AddExternalPredicate("read_file", readFile)
```

```mangle
# Use in Mangle
Decl read_file(Path.Type<string>, Content.Type<string>) :external.

process_config(Config) :-
    read_file("/etc/config.json", Content),
    # ... parse Content ...
    Config = Content.
```

**Warning:** External predicates CAN have side effects, but should be:
- **Idempotent**: Same input = same output
- **Pure (ideally)**: No observable side effects
- **Deterministic**: No randomness

Use sparingly! Most I/O should happen in Go before/after Mangle evaluation.

---

## Migration Checklist

When translating effectful code to Mangle:

- [ ] Remove print/log statements - derive log facts
- [ ] Remove file I/O - derive file operations, execute in Go
- [ ] Remove database operations - derive persistence facts, execute in Go
- [ ] Remove network requests - derive URL fetch facts, execute in Go
- [ ] Remove random() - load random values as facts
- [ ] Remove now()/current_time() - load time as fact
- [ ] Remove global state mutation - use facts
- [ ] Remove message sending - derive notification facts
- [ ] Remove exceptions - derive error facts
- [ ] Remove API calls - derive call facts, execute in Go
- [ ] Remove timing/sleep - model time explicitly
- [ ] Remove callbacks - derive handler facts
- [ ] Consider external predicates only for truly unavoidable I/O
- [ ] Remember: Mangle describes effects, Go executes them

---

## Pro Tip: Separate Concerns

**Mangle's job:** "What should happen based on the facts?"

**Go's job:** "Actually make it happen in the real world."

Think of Mangle as a declarative specification of effects, and Go as the executor.

This separation gives you:
- **Testability**: Test Mangle logic without I/O
- **Auditability**: See what *would* happen before doing it
- **Composability**: Combine effects declaratively
- **Reproducibility**: Same facts = same effect specifications

Embrace the purity!
