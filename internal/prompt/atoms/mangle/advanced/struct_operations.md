# Mangle Struct Operations

## Struct Syntax

```mangle
# Struct literal
{ /key1: "value1", /key2: 42, /key3: /atom_value }

# Note: Keys are atoms (start with /)
# Values can be any type
```

## Struct Declaration

```mangle
Decl user_data(Id.Type<atom>, Data.Type<struct>).

# Fact with struct
user_data(/u1, { /name: "Alice", /age: 30, /role: /admin }).
```

## Accessing Struct Fields

### 1. :match_field (Single Field Access)

```mangle
Decl get_name(UserId.Type<atom>, Name.Type<string>).

get_name(UserId, Name) :-
  user_data(UserId, Data),
  :match_field(Data, /name, Name).

# Query: get_name(/u1, ?Name)
# Result: Name = "Alice"
```

### 2. Multiple Field Access

```mangle
Decl user_info(Id.Type<atom>, Name.Type<string>, Age.Type<int>).

user_info(Id, Name, Age) :-
  user_data(Id, Data),
  :match_field(Data, /name, Name),
  :match_field(Data, /age, Age).

# Query: user_info(/u1, ?Name, ?Age)
# Result: Name = "Alice", Age = 30
```

### 3. Nested Struct Access

```mangle
Decl employee(Id.Type<atom>, Data.Type<struct>).

# Nested struct
employee(/e1, {
  /name: "Bob",
  /contact: {
    /email: "bob@example.com",
    /phone: "555-1234"
  }
}).

Decl get_email(Id.Type<atom>, Email.Type<string>).

get_email(Id, Email) :-
  employee(Id, Data),
  :match_field(Data, /contact, ContactStruct),
  :match_field(ContactStruct, /email, Email).

# Query: get_email(/e1, ?Email)
# Result: Email = "bob@example.com"
```

## Struct Construction

### 1. Building Structs Dynamically

```mangle
Decl create_user_struct(Name.Type<string>, Age.Type<int>, Result.Type<struct>).

create_user_struct(Name, Age, Result) :-
  Result = { /name: Name, /age: Age, /active: /true }.

# Query: create_user_struct("Charlie", 25, ?Result)
# Result: Result = { /name: "Charlie", /age: 25, /active: /true }
```

### 2. Merging Structs (Manual)

```mangle
# Note: Mangle does not have built-in struct merge
# You must manually extract and reconstruct

Decl merge_user_data(Base.Type<struct>, Override.Type<struct>, Result.Type<struct>).

merge_user_data(Base, Override, Result) :-
  :match_field(Base, /name, BaseName),
  :match_field(Base, /age, BaseAge),
  :match_field(Override, /age, OverrideAge),  # Override age
  Result = { /name: BaseName, /age: OverrideAge }.

# This is tedious; prefer using separate predicates for different data shapes
```

## Struct Patterns

### 1. Partial Matching (Extract Subset of Fields)

```mangle
Decl user(Id.Type<atom>, Data.Type<struct>).

user(/u1, { /name: "Alice", /age: 30, /role: /admin, /dept: /eng }).

# Extract only specific fields
Decl user_role(Id.Type<atom>, Name.Type<string>, Role.Type<atom>).

user_role(Id, Name, Role) :-
  user(Id, Data),
  :match_field(Data, /name, Name),
  :match_field(Data, /role, Role).
  # Ignore /age and /dept

# Query: user_role(/u1, ?Name, ?Role)
# Result: Name = "Alice", Role = /admin
```

### 2. Struct Field Existence Check

```mangle
Decl has_field(Data.Type<struct>, Field.Type<atom>).

has_field(Data, Field) :-
  :match_field(Data, Field, _).  # If match succeeds, field exists

# Query: has_field({ /name: "Alice", /age: 30 }, /name)
# Result: TRUE

# Query: has_field({ /name: "Alice", /age: 30 }, /email)
# Result: FALSE
```

### 3. Conditional Field Access (Default Value)

```mangle
Decl get_email_or_default(UserId.Type<atom>, Email.Type<string>).

# If email exists, use it
get_email_or_default(UserId, Email) :-
  user_data(UserId, Data),
  :match_field(Data, /email, Email).

# If email doesn't exist, use default
get_email_or_default(UserId, "no-email@example.com") :-
  user_data(UserId, Data),
  not :match_field(Data, /email, _).
```

## Struct with Lists

```mangle
Decl project(Id.Type<atom>, Data.Type<struct>).

project(/p1, {
  /name: "Project Alpha",
  /members: [/alice, /bob, /charlie],
  /tags: [/urgent, /backend]
}).

# Extract list field
Decl project_members(Id.Type<atom>, Members.Type<list<atom>>).

project_members(Id, Members) :-
  project(Id, Data),
  :match_field(Data, /members, Members).

# Query: project_members(/p1, ?Members)
# Result: Members = [/alice, /bob, /charlie]

# Check if user is member
Decl is_project_member(ProjectId.Type<atom>, UserId.Type<atom>).

is_project_member(ProjectId, UserId) :-
  project_members(ProjectId, Members),
  fn:list:contains(Members, UserId).

# Query: is_project_member(/p1, /bob)
# Result: TRUE
```

## Struct with Nested Lists

```mangle
Decl task(Id.Type<atom>, Data.Type<struct>).

task(/t1, {
  /title: "Implement feature",
  /subtasks: [
    { /id: /st1, /title: "Design", /done: /true },
    { /id: /st2, /title: "Code", /done: /false }
  ]
}).

# Extract subtasks
Decl task_subtasks(TaskId.Type<atom>, Subtasks.Type<list<struct>>).

task_subtasks(TaskId, Subtasks) :-
  task(TaskId, Data),
  :match_field(Data, /subtasks, Subtasks).

# Process each subtask (requires iteration)
Decl subtask_status(TaskId.Type<atom>, SubtaskId.Type<atom>, Done.Type<atom>).

# This is complex in pure Mangle; better to normalize data structure
# Store subtasks as separate facts:
Decl subtask(TaskId.Type<atom>, SubtaskId.Type<atom>, Title.Type<string>, Done.Type<atom>).

subtask(/t1, /st1, "Design", /true).
subtask(/t1, /st2, "Code", /false).
```

## Map-Like Operations (Struct as Dictionary)

### 1. Key-Value Lookup

```mangle
Decl config(Key.Type<atom>, Value.Type<string>).
Decl config_struct(Data.Type<struct>).

# Store config as struct
config_struct({
  /db_host: "localhost",
  /db_port: "5432",
  /api_key: "secret123"
}).

# Lookup by key
Decl get_config(Key.Type<atom>, Value.Type<string>).

get_config(Key, Value) :-
  config_struct(Data),
  :match_field(Data, Key, Value).

# Query: get_config(/db_host, ?Value)
# Result: Value = "localhost"
```

### 2. Dynamic Key Access (Limited)

```mangle
# Mangle requires keys to be known at rule-writing time
# You cannot do: :match_field(Data, DynamicKey, Value)
# where DynamicKey is computed at runtime

# WRONG: This doesn't work
bad_lookup(Data, Key, Value) :-
  :match_field(Data, Key, Value).  # Key must be literal atom

# CORRECT: Enumerate all possible keys
lookup(Data, /name, Value) :- :match_field(Data, /name, Value).
lookup(Data, /age, Value) :- :match_field(Data, /age, Value).
lookup(Data, /email, Value) :- :match_field(Data, /email, Value).
```

## Struct Validation

### 1. Required Fields

```mangle
Decl valid_user(Data.Type<struct>).

valid_user(Data) :-
  :match_field(Data, /name, Name),
  :match_field(Data, /age, Age),
  :match_field(Data, /email, Email),
  Name != "",
  Age > 0.

# Query: valid_user({ /name: "Alice", /age: 30, /email: "a@ex.com" })
# Result: TRUE

# Query: valid_user({ /name: "Alice", /age: 30 })
# Result: FALSE (missing /email)
```

### 2. Field Type Validation

```mangle
Decl valid_age(Data.Type<struct>).

valid_age(Data) :-
  :match_field(Data, /age, Age),
  Age >= 0,
  Age <= 150.

# Query: valid_age({ /age: 30 })
# Result: TRUE

# Query: valid_age({ /age: 200 })
# Result: FALSE
```

### 3. Conditional Field Requirements

```mangle
Decl valid_employee(Data.Type<struct>).

# If role is manager, must have /team field
valid_employee(Data) :-
  :match_field(Data, /role, /manager),
  :match_field(Data, /team, Team),
  fn:length(Team) > 0.

# If role is not manager, /team is optional
valid_employee(Data) :-
  :match_field(Data, /role, Role),
  Role != /manager.
```

## Struct Update (Immutable)

Structs are immutable. To "update", create a new struct.

```mangle
Decl update_age(OldData.Type<struct>, NewAge.Type<int>, NewData.Type<struct>).

update_age(OldData, NewAge, NewData) :-
  :match_field(OldData, /name, Name),
  :match_field(OldData, /email, Email),
  # Extract other fields as needed
  NewData = { /name: Name, /age: NewAge, /email: Email }.

# Query: update_age({ /name: "Alice", /age: 30, /email: "a@ex.com" }, 31, ?New)
# Result: New = { /name: "Alice", /age: 31, /email: "a@ex.com" }
```

**Note**: This is tedious for large structs. Better to use separate facts for mutable data.

## Struct vs. Multiple Facts

### Struct Approach
```mangle
Decl user_struct(Id.Type<atom>, Data.Type<struct>).
user_struct(/u1, { /name: "Alice", /age: 30, /role: /admin }).
```

**Pros**:
- Single fact per entity
- Natural JSON-like representation

**Cons**:
- Field access is verbose (`:match_field` for each)
- Hard to update (must reconstruct entire struct)
- Cannot easily query "all users with age > 25"

### Multiple Facts Approach
```mangle
Decl user_name(Id.Type<atom>, Name.Type<string>).
Decl user_age(Id.Type<atom>, Age.Type<int>).
Decl user_role(Id.Type<atom>, Role.Type<atom>).

user_name(/u1, "Alice").
user_age(/u1, 30).
user_role(/u1, /admin).
```

**Pros**:
- Direct field access (no `:match_field` needed)
- Easy to update (add/retract single fact)
- Easy to query by field (`user_age(Id, Age), Age > 25`)

**Cons**:
- Multiple facts per entity
- Harder to ensure all fields exist together

**Recommendation**: Use multiple facts for most cases. Use structs for:
- External data (JSON APIs)
- Nested/hierarchical data
- Data you never query by field

## Advanced: Struct in Aggregation

```mangle
Decl event(Id.Type<atom>, Data.Type<struct>).

event(/e1, { /user: /alice, /action: /login, /timestamp: 100 }).
event(/e2, { /user: /alice, /action: /logout, /timestamp: 200 }).
event(/e3, { /user: /bob, /action: /login, /timestamp: 150 }).

# Count events per user
Decl user_event_count(User.Type<atom>, Count.Type<int>).

user_event_count(User, Count) :-
  event(Id, Data),
  :match_field(Data, /user, User)
  |> do fn:group_by(User),
  let Count = fn:count(Id).

# Result:
#   user_event_count(/alice, 2)
#   user_event_count(/bob, 1)
```

## Struct Equality

Struct equality is structural (deep equality).

```mangle
Decl same_data(A.Type<struct>, B.Type<struct>).

same_data(A, B) :- A = B.

# Query: same_data({ /x: 1, /y: 2 }, { /x: 1, /y: 2 })
# Result: TRUE

# Query: same_data({ /x: 1, /y: 2 }, { /y: 2, /x: 1 })
# Result: TRUE (key order doesn't matter)

# Query: same_data({ /x: 1, /y: 2 }, { /x: 1, /y: 3 })
# Result: FALSE
```

## Checklist

- [ ] Use `:match_field(Struct, /key, Value)` for field access
- [ ] Struct keys are atoms (start with `/`)
- [ ] Field access is verbose; consider normalized facts instead
- [ ] Cannot dynamically access fields (keys must be literal atoms)
- [ ] Structs are immutable; "updates" create new structs
- [ ] Use structs for hierarchical/nested data or external formats
- [ ] Use facts for queryable, updatable data
- [ ] Check field existence with `:match_field` success/failure
- [ ] Nested structs require chained `:match_field` calls
