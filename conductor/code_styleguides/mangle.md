# Code Style Guide: Mangle

## Core Philosophy
- **Logic determines Reality:** Rules must describe *what* is true (declarative), not *how* to compute it (imperative).
- **Separation of Concerns:** Keep schemas (EDB) in `schemas_*.mg` and policies (IDB) in `policy/*.mg`.
- **Neuro-Symbolic Bridge:** Rules should be designed to receive "atoms" from the Perception Transducer and emit "actions" via the Virtual Store.

## Naming Conventions
- **Predicates:** Use `snake_case` (e.g., `user_intent`, `next_action`).
- **Atoms (Constants):** Use `/lowercase_with_underscores` (e.g., `/read_file`, `/mutation`).
- **Variables:** Use `UPPERCASE` or `PascalCase` (e.g., `X`, `FileID`, `TaskStatus`).
- **Files:** Use `.mg` extension. Modularize files by domain (e.g., `schemas_world.mg`).

## Formatting & Structure
- **Termination:** Every statement **MUST** end with a period (`.`).
- **Rule Layout:**
  ```mangle
  head_predicate(Arg1, Arg2) :-
      body_atom_1(Arg1, X),
      body_atom_2(X, Arg2),
      X != Arg1.
  ```
- **Modularity:** Aim for files under 600 lines. Use the `modularization index` pattern found in `internal/core/defaults/schemas.mg`.

## Declarations (The Schema Mandate)
Every predicate **MUST** have a `Decl` statement specifying argument names and types. This enables runtime type checking.
```mangle
Decl file_topology(
    Path.Type<string>,
    Hash.Type<string>,
    Language.Type<n>,
    LastModified.Type<int>,
    IsTestFile.Type<bool>
).
```

## Safety & Logic Patterns
- **Stratified Negation:** Negated atoms (`not p(X)`) must follow positive atoms that bind all variables in the negated atom.
- **Constitutional Gates:** Every action-deriving rule must join with `permitted(Action, ...)` to ensure safety.
- **Aggregation Transforms:** Use the `|>` pipeline for all groupings and calculations.
  ```mangle
  risk_summary(Project, VulnCount) :-
      vulnerable_project(Project, _, _) |>
      do fn:group_by(Project),
      let VulnCount = fn:Count().
  ```
- **Recursion Mastery:** For transitive closures (dependencies, call graphs), use the base-case + recursive-step pattern.
  ```mangle
  impacted(X) :- modified(X).
  impacted(X) :- dependency_link(X, Y, _), impacted(Y).
  ```

## Advanced Operational Patterns
- **Spreading Activation:** Use activation scores to manage LLM context injection.
- **Piggyback Control:** Rules should support the dual-channel protocol by emitting `control_packet` atoms.
- **Adversarial Hardening:** Write rules that assume potential failure and derive corrective actions or escalations.


> *[Archived & Reviewed by The Librarian on 2026-01-25]*