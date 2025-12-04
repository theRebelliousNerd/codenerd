# Mangle Context7 Reference

**Source**: Context7 Documentation (November 2025)
**Version**: Mangle v0.4.0+

Comprehensive patterns and examples from Context7's up-to-date Mangle documentation.

---

## Installation

### Go Implementation (Recommended)

```shell
# Install interpreter
GOBIN=~/bin go install github.com/google/mangle/interpreter/mg@latest

# Build from source
git clone https://github.com/google/mangle
cd mangle
go get -t ./...
go build ./...
go test ./...
```

### Rust Implementation

```shell
git clone https://github.com/google/mangle
cd mangle/rust
cargo build
cargo test
```

### Generate Parser Sources (ANTLR)

```shell
wget http://www.antlr.org/download/antlr-4.13.2-complete.jar
alias antlr='java -jar $PWD/antlr-4.13.2-complete.jar'
antlr -Dlanguage=Go -package gen -o ./ parse/gen/Mangle.g4 -visitor
```

---

## Interpreter Usage

### Start REPL

```bash
# Basic start
go run interpreter/main/main.go

# With custom root directory
go run interpreter/main/main.go --root=$PWD/examples

# Preload sources
go run interpreter/main/main.go --load=<file1>,<file2>

# Execute query and exit
go run interpreter/main/main.go --exec=<query>
```

### REPL Commands

```plaintext
<decl>.            adds declaration to interactive buffer
<useDecl>.         adds package-use declaration to interactive buffer
<clause>.          adds clause to interactive buffer, evaluates
?<predicate>       looks up predicate name and queries all facts
?<goal>            queries all facts that match goal
::load <path>      pops interactive buffer and loads source file
::help             display help text
::pop              reset state to before interactive defs or last load
::show <predicate> shows information about predicate
::show all         shows information about all available predicates
<Ctrl-D>           quit
```

---

## Basic Types

### Named Constants (Atoms)

```Mangle
/a
/test12
/antigone
/crates.io/fnv
/home.cern/news/news/computing/30-years-free-and-open-web
```

### Numbers

```Mangle
# Integers (64-bit signed)
0
1
128
-10000

# Floating-point (64-bit)
3.141592
-10.5
```

### Strings

```Mangle
"foo"
'foo'
"something 'quoted'"
'something "quoted"'
"A newline \n"
"A tab \t"
"Java class files start with \xca\xfe\xba\xbe"
"The \u{01f624} emoji"

# Multi-line with backticks
`
I write, erase, rewrite
Erase again, and then
A poppy blooms.
`
```

### Byte Strings

```Mangle
b"A \x80 byte carries special meaning in UTF8 encoded strings"
b"\x80\x81\x82\n"
```

---

## Structured Data Types

### Lists

```Mangle
[]              # Empty list
[0]             # Single element
[/a, /b, /c]    # Multiple elements
[/a, /b, /c,]   # Trailing comma allowed

# Constructors
fn:list(value1, ..., valueN)
fn:cons(value, listValue)
```

### Maps

```Mangle
[/a: /foo, /b: /bar]
[0: "zero", 1: "one",]

# Constructor
fn:map(key1, value1, ... keyN, valueN)
```

### Structs

```Mangle
{}
{/a: /foo, /b: [/bar, /baz]}

# Constructor
fn:struct(key1, value1, ... keyN, valueN)
```

### Pairs and Tuples

```Mangle
# Pairs
fn:pair("web", 2.0)
fn:pair("hello", fn:pair("world", "!"))

# Tuples
fn:tuple("hello", "world", "!")
fn:tuple(1, 2, "three", "4")
```

### Complex Example

```Mangle
triangle_2d([
   { /x: 1,  /y:  2 },
   { /x: 5, /y: 10 },
   { /x: 12,  /y:  5 }
])
```

---

## Type Expressions

### Basic Types

```Mangle
/any                          # Universal type
fn:Singleton(/foo)            # Singleton type for a name
```

### Type Variables

```Mangle
X
fn:Pair(Y, /string)
```

### Union Types

```Mangle
fn:Union()                              # Empty union
fn:Union(/name, /string)                # Union of name and string
fn:Union(fn:Singleton(/foo), fn:Singleton(/bar))  # Union of singletons
```

---

## Defining Facts

```Mangle
volunteer(1, "Aisha Salehi", /teaching).
volunteer(1, "Aisha Salehi", /workshop_facilitation).
volunteer(2, "Xin Watson", /workshop_facilitation).
volunteer(3, "Alyssa P. Hacker", /software_development).

# Facts with relationships
parent(/oedipus, /antigone).
parent(/oedipus, /ismene).
parent(/oedipus, /eteocles).
parent(/oedipus, /polynices).

# Knowledge relationships
knows("Aisha", "Xin").
knows("Xin", "Alyssa").
knows("Alyssa", "Selin").
```

---

## Defining Rules

### Basic Rules

```Mangle
# Identify teachers from volunteers
teacher(ID, Name) <- volunteer(ID, Name, /teaching).

# Equivalent SQL:
# CREATE TABLE teacher AS
# SELECT ID, Name FROM volunteer WHERE Role = '/teaching';
```

### Sibling Rule

```Mangle
sibling(Person1, Person2) <-
   parent(P, Person1), parent(P, Person2), Person1 != Person2.
```

### Join Operations

```Mangle
# Find volunteers with both skills
teacher_and_coder(ID, Name) <-
    volunteer(ID, Name, /teaching),
    volunteer(ID, Name, /software_development).

# SQL equivalent:
# SELECT DISTINCT v1.ID, v1.Name
# FROM volunteer AS v1
# JOIN volunteer AS v2 ON v1.ID = v2.ID AND v1.Name = v2.Name
# WHERE v1.Role = '/teaching' AND v2.Role = '/software_development';
```

### Union via Multiple Rules

```Mangle
coding_workshop_candidate(ID, Name, /teaching) <-
    volunteer(ID, Name, /teaching).
coding_workshop_candidate(ID, Name, /software_development) <-
    volunteer(ID, Name, /software_development).
```

---

## Recursion

### Transitive Closure (Reachability)

```Mangle
# Base case: direct connection
reachable(X, Y) <- knows(X, Y).

# Recursive case: indirect connection
reachable(X, Z) <- knows(X, Y), reachable(Y, Z).
```

**SQL Equivalent** (recursive CTE):

```SQL
WITH RECURSIVE reachable_cte (StartPerson, EndPerson) AS (
    -- Base Case
    SELECT Person1, Person2 FROM knows
    UNION ALL
    -- Recursive Step
    SELECT r.StartPerson, k.Person2
    FROM reachable_cte AS r
    JOIN knows AS k ON r.EndPerson = k.Person1
)
SELECT DISTINCT StartPerson, EndPerson FROM reachable_cte;
```

### Dependency Tracking

```Mangle
# Direct containment
contains_jar(P, Name, Version) :-
  contains_jar_directly(P, Name, Version).

# Transitive through dependencies
contains_jar(P, Name, Version) :-
  project_depends(P, Q),
  contains_jar(Q, Name, Version).
```

### Graph Reachability

```Mangle
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
```

---

## Aggregation

### Basic Aggregation with Transforms

```Mangle
project_dev_energy(ProjectID, NumDevelopers, TotalHours) <-
  project_assignment(ProjectID, VolunteerID, /software_development, Hours)
  |> do fn:group_by(ProjectID),
     NumDevelopers = fn:count(),
     TotalHours = fn:sum(TotalHours).
```

**SQL Equivalent**:

```sql
SELECT ProjectID, COUNT(VolunteerID) as NumDevelopers, SUM(Hours) as TotalHours
FROM project_assignment
WHERE Role = '/software_developer'
GROUP BY ProjectID
```

### Counting with Aggregation

```Mangle
count_projects_with_vulnerable_log4j(Num) :-
  projects_with_vulnerable_log4j(P)
  |> do fn:group_by(),
     let Num = fn:Count().
```

### Handling Empty Groups (Negation Pattern)

```Mangle
# Find projects WITH developers
project_with_developers(ProjectID) <-
  project_assignment(ProjectID, _, /software_development, _).

# Find projects WITHOUT developers
project_without_developers(ProjectID) <-
  project_name(ProjectID, _).
  !project_with_developers(ProjectID)

# Include zero-developer projects
project_dev_energy(ProjectID, NumDevelopers, TotalHours) <-
  project_without_developers(ProjectID),
  NumDevelopers = 0,
  TotalHours = 0.
```

---

## Security Analysis Example

### Vulnerability Detection

```Mangle
# Find projects with vulnerable log4j versions
projects_with_vulnerable_log4j(P) :-
  projects(P),
  contains_jar(P, "log4j", Version),
  Version != "2.17.1",
  Version != "2.12.4",
  Version != "2.3.2".
```

**SQL Equivalent**:

```SQL
SELECT projects.id as P
FROM projects JOIN contains_jar ON projects.id = contains_jar.project_id
WHERE contains_jar.version NOT IN ("2.17.1", "2.12.4", "2.3.2")
```

---

## Trip Pricing Example

```Mangle
# One-leg trip pricing
one_or_two_leg_trip(Codes, Start, Destination, Price) :-
  direct_conn(Code, Start, Destination, Price)
  |> let Codes = [Code].

# Two-leg trip pricing
one_or_two_leg_trip(Codes, Start, Destination, Price) :-
  direct_conn(FirstCode, Start, Connecting, FirstLegPrice).
  direct_conn(SecondCode, Connecting, Destination, SecondLegPrice)
  |> let Code = [FirstCode, SecondCode],
     let Price = fn:plus(FirstLegPrice, SecondLegPrice).
```

---

## Built-in Operations

### Comparisons

```Mangle
Left = Right      # Equality
Left != Right     # Inequality
Left < Right      # Less than
Left <= Right     # Less than or equal
```

### List Operations

```Mangle
fn:list:get(ListValue, Index)   # Get element by index (0-based)
:match_cons(List, Head, Tail)   # Destructure list into head and tail
:match_nil(List)                # Check if list is empty
:list:member(Element, List)     # Check membership
```

### Map Operations

```Mangle
:match_entry(Map, Key, Value)   # Extract key-value pair
:match_field(Record, /field, Value)  # Match field in struct
```

---

## Volunteer Database Example

### Schema Definition

```Mangle
# Define known skills (vocabulary)
skill(/skill/admin).
skill(/skill/facilitate).
skill(/skill/frontline).
skill(/skill/speaking).
skill(/skill/teaching).
skill(/skill/recruiting).
skill(/skill/workshop_facilitation).

# Volunteer facts
volunteer(/v/1).
volunteer_name(/v/1, "Aisha Salehi").
volunteer_time_available(/v/1, /monday, /afternoon).
volunteer_time_available(/v/1, /monday, /morning).
volunteer_interest(/v/1, /skill/frontline).
volunteer_skill(/v/1, /skill/admin).
volunteer_skill(/v/1, /skill/facilitate).
volunteer_skill(/v/1, /skill/teaching).
```

### Vocabulary Constraints

```mangle
Decl volunteer_interest(Volunteer, Skill)
  bound [ /v, /skill ].

Decl volunteer_skill(Volunteer, Skill)
  bound [ /v, /skill ].
```

### Query Patterns

```Mangle
# Find volunteers available Tuesday afternoon
?volunteer_time_available(VolunteerID, /tuesday, /afternoon)

# Find volunteers with specific skill
?volunteer_skill(VolunteerID, /skill/workshop_facilitation)
```

### Teacher-Learner Matching

```Mangle
# Find teacher-learner pairs by skill
teacher_learner_match(Teacher, Learner, Skill) :-
  volunteer_skill(Teacher, Skill),
  volunteer_interest(Learner, Skill).

# Include availability matching
teacher_learner_match_session(Teacher, Learner, Skill, PreferredDay, Slot) :-
  teacher_learner_match(Teacher, Learner, Skill),
  volunteer_time_available(Teacher, PreferredDay, Slot),
  volunteer_time_available(Learner, PreferredDay, Slot).
```

### Value Table Format

```Mangle
volunteer_record({
   /id:             /v/1,
   /name:           "Aisha Salehi",
   /time_available: [ fn:pair(/monday, /morning), fn:pair(/monday, /afternoon) ],
   /interest:       [ /skill/frontline ],
   /skill:          [ /skill/admin, /skill/facilitate, /skill/teaching ]
}).

# Convert back to predicates
volunteer(Id) :-
  volunteer_record(R), :match_field(R, /id, Id).

volunteer_name(Id, Name) :-
  volunteer_record(R), :match_field(R, /id, Id), :match_field(R, /name, Name).
```

---

## Declarations

### Package Declaration

```Mangle
Package <pkg>!
```

### Uses Declaration

```Mangle
Uses <pkg>!
```

### Predicate Declaration

```Mangle
Decl <predicate name>(<Arg1>, ... <ArgN>)
  descr [<descriptor items>]
  bounds [<bound items>]
.
```

### Descriptor Items

```Mangle
doc(<string>, ... <string>)    # Documentation
mode()                          # Argument modes (+, -, ?)
arg(<Arg>, <string>)            # Argument description
deferred()                      # Deferred predicate
merge(<ArgList>, <pred>)        # Merge predicate for lattice
```

### Predicate Modes

```Mangle
# GoodTimeSlots must be provided at request time
Decl matching_availability(VolunteerID, Name, GoodTimeSlots)
  descr [mode("+", "+", "-"), public()].
```

---

## API Integration

### Simple Query API

```json
{
  "input": "?volunteer_time_available(VolunteerID, /tuesday, /afternoon)"
}
```

### Flexible Query API with Program

```json
{
  "program": "good_time(/tuesday, /afternoon).\ngood_time(/wednesday, /morning).\nmatching_availability(VolunteerID, Name, Weekday, Timeslot) :-\n    volunteer_time_available(VolunteerID, Weekday, Timeslot),\n    good_time(Weekday, Timeslot),\n    volunteer_name(VolunteerID, Name).",
  "input": "matching_availability(VolunteerID, Name, Weekday, Timeslot)"
}
```

### gRPC Service Definition

```protobuf
message ByAvRequest { repeated Availability availability = 1; }
message ByAvReply { repeated Volunteer reply = 1; }
service VolunteerQuery {
  rpc GetByMatchingAvailability (ByAvRequest) returns (ByAvReply) {}
}
```

---

## Rule Grammar

```mangle
rule ::= atom '.'
       | atom  ':-' rhs

rhs ::= litOrFml (',' litOrFml)* '.'
      | litOrFml (',' litOrFml)* '|>' transforms

transforms ::= stmts '.'
             | 'do' apply-expr ',' stmts '.'

stmts ::= stmt (',' stmt)*
stmt ::= 'let' var '=' expr

litOrFml ::= atom | '!' atom | cmp

atom ::= pred-ident '(' exprs? ')'

expr ::= apply-expr | constant | var

exprs ::= expr (',' expr)*

apply-expr ::= fn-ident '(' exprs? ')' | [ exprs? ]

cmp ::= expr op expr

op ::= '=' | '!=' | '<' | '>'
```

---

## Relational Algebra Mappings

### Projection

```Datalog
p(X...) = q(X..., _...).
```

### Selection

```Datalog
p(X...) = q(X...), F*
```

### Cartesian Product

```Datalog
p(X..., Y...) :- q1(X...), q2(Y...).
```

### Union

```Datalog
p(X...) :- q1(X...).
p(X...) :- q2(X...).
```

### Set Difference

```Datalog
p(X...) := q1(X...), !q2(X...).
```

### Variable Rectification

```Datalog
# Original
knows(X, X) :- person(X).

# Rectified
knows(X, Y) :- person(X), Y = X.
```

---

## Resources

- **GitHub**: https://github.com/google/mangle
- **ReadTheDocs**: https://mangle.readthedocs.io
- **Go Packages**: https://pkg.go.dev/github.com/google/mangle
- **Demo Service**: https://github.com/burakemir/mangle-service
