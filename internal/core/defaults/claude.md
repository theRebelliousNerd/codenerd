## Top 30 Common Errors AI Coding Agents Make When Writing Mangle Code

These are categorized by the layer of the stack where the "Stochastic Gap" occurs: Syntax, Logic/Safety, Data Structures, and Integration.

### I. Syntactic Hallucinations (The "Soufflé/SQL" Bias)

AI models trained on SQL, Prolog, and Soufflé often force those syntaxes into Mangle.

1. **Atom vs. String Confusion**
   - **Error:** Using `"active"` when the schema requires `/active`.
   - **Correction:** Use `/atom` for enums/IDs. Mangle treats these as disjoint types; they will never unify.

2. **Soufflé Declarations**
   - **Error:** `.decl edge(x:number, y:number).`
   - **Correction:** `Decl edge(X.Type<int>, Y.Type<int>).` (Note uppercase `Decl` and type syntax).

3. **Lowercase Variables**
   - **Error:** `ancestor(x, y) :- parent(x, y).` (Prolog style).
   - **Correction:** `ancestor(X, Y) :- parent(X, Y).` Variables must be UPPERCASE.

4. **Inline Aggregation (SQL Style)**
   - **Error:** `total(Sum) :- item(X), Sum = sum(X).`
   - **Correction:** Use the pipe operator: `... |> do fn:group_by(), let Sum = fn:Sum(X).`

5. **Implicit Grouping**
   - **Error:** Assuming variables in the head automatically trigger GROUP BY (like SQL).
   - **Correction:** Grouping is explicit in the `do fn:group_by(...)` transform step.

6. **Missing Periods**
   - **Error:** Ending a rule with a newline instead of `.`
   - **Correction:** Every clause must end with a period `.`

7. **Comment Syntax**
   - **Error:** `// This is a comment` or `/* ... */`
   - **Correction:** Use `# This is a comment.`

8. **Assignment vs. Unification**
   - **Error:** `X := 5` or `let X = 5` inside a rule body (without pipe).
   - **Correction:** Use unification `X = 5` inside the body, or `let` only within a transform block.

### II. Semantic Safety & Logic (The "Datalog" Gap)

Mangle requires strict logical validity that probabilistic models often miss.

9. **Unsafe Head Variables**
   - **Error:** `result(X) :- other(Y).` (X is unbounded).
   - **Correction:** Every variable in the head must appear in a positive atom in the body.

10. **Unsafe Negation**
    - **Error:** `safe(X) :- not distinct(X).`
    - **Correction:** Variables in a negated atom must be bound first: `safe(X) :- candidate(X), not distinct(X).`

11. **Stratification Cycles**
    - **Error:** `p(X) :- not q(X). q(X) :- not p(X).`
    - **Correction:** Ensure no recursion passes through a negation. Restructure logic into strict layers (strata).

12. **Infinite Recursion (Counter Fallacy)**
    - **Error:** `count(N) :- count(M), N = fn:plus(M, 1).` (Unbounded generation).
    - **Correction:** Always bound recursion with a limit or a finite domain (e.g., `N < 100`).

13. **Cartesian Product Explosion**
    - **Error:** Placing large tables before filters: `res(X) :- huge_table(X), X = /specific_id.`
    - **Correction:** Selectivity first: `res(X) :- X = /specific_id, huge_table(X).`

14. **Null Checking (Open World Bias)**
    - **Error:** `check(X) :- data(X), X != null.`
    - **Correction:** Mangle follows the Closed World Assumption. If a fact exists, it is not null. "Missing" facts are simply not there.

15. **Duplicate Rule Definitions**
    - **Error:** Thinking multiple rules overwrite each other.
    - **Correction:** Multiple rules create a UNION. `p(x) :- a(x).` and `p(x) :- b(x).` means `p` is true if `a` OR `b` is true.

16. **Anonymous Variable Misuse**
    - **Error:** Using `_` when the value is actually needed later in the rule.
    - **Correction:** Use `_` only for values you truly don't care about. It never binds.

### III. Data Types & Functions (The "JSON" Bias)

AI agents often hallucinate object-oriented accessors for Mangle's structured data.

17. **Map Dot Notation**
    - **Error:** `Val = Map.key` or `Map['key']`
    - **Correction:** Use `:match_entry(Map, /key, Val)` or `:match_field(Struct, /key, Val)`.

18. **List Indexing**
    - **Error:** `Head = List[0].`
    - **Correction:** Use `:match_cons(List, Head, Tail)` or `fn:list:get(List, 0)`.

19. **Type Mismatch (Int vs Float)**
    - **Error:** `X = 5` when X is declared `Type<float>`.
    - **Correction:** Mangle is strict. Use `5.0` for floats, `5` for ints.

20. **String Interpolation**
    - **Error:** `msg("Error: $Code").`
    - **Correction:** Use `fn:string_concat` or build list structures. Mangle has no string interpolation.

21. **Hallucinated Functions**
    - **Error:** `fn:split`, `fn:date`, `fn:substring` (assuming StdLib parity with Python).
    - **Correction:** Verify function existence in builtin package. Mangle's standard library is minimal.

22. **Aggregation Safety**
    - **Error:** `... |> do fn:group_by(UnboundVar) ...`
    - **Correction:** Grouping variables must be bound in the rule body before the pipe `|>`.

23. **Struct Syntax**
    - **Error:** `{"key": "value"}` (JSON style).
    - **Correction:** `{ /key: "value" }` (Note the atom key and spacing).

### IV. Go Integration & Architecture (The "API" Gap)

When embedding Mangle, AI agents fail to navigate the boundary between Go and Logic.

24. **Fact Store Type Errors**
    - **Error:** `store.Add("pred", "arg").`
    - **Correction:** Must use `engine.Atom`, `engine.Number` types wrapped in `engine.Value`.

25. **Incorrect Engine Entry Point**
    - **Error:** `engine.Run()` (Hallucination).
    - **Correction:** Use `engine.EvalProgram` or `engine.EvalProgramNaive`.

26. **Ignoring Imports**
    - **Error:** Generating Mangle code without necessary package references or failing to import the Go engine package correctly.
    - **Correction:** Explicitly manage `github.com/google/mangle/engine`.

27. **External Predicate Signature**
    - **Error:** Writing a Go function for a predicate that returns `(interface{}, error)`.
    - **Correction:** External predicates require `func(query engine.Query, cb func(engine.Fact)) error`.

28. **Parsing vs. Execution**
    - **Error:** Passing raw strings to `EvalProgram`.
    - **Correction:** Code must be parsed (`parse.Unit`) and analyzed (`analysis.AnalyzeOneUnit`) before evaluation.

29. **Assuming IO Access**
    - **Error:** `read_file(Path, Content).`
    - **Correction:** Mangle is pure. IO must happen in Go before execution (loading facts) or via external predicates.

30. **Package Hallucination (Slopsquatting)**
    - **Error:** Importing non-existent Mangle libraries (`use /std/date`).
    - **Correction:** Verify imports. Mangle has a very small, specific ecosystem.

## How to Avoid These Mistakes (For the Mangle Architect)

1. **Feed the Grammar:** Provide the "Complete Syntax Reference" (File 200) in the prompt context.
2. **Solver-in-the-Loop:** Do not trust "Zero-Shot" code. Run a loop: Generate -> Parse (with `mangle/parse`) -> Feed Errors back to LLM -> Regenerate.
3. **Explicit Typing:** Force the AI to declare types (`Decl`) first. This forces it to decide between `/atoms` and `"strings"` early.
4. **Review for Liveness:** Manually audit recursive rules for termination conditions.