# 600: Type System - Declarations, Bounds, and What Is Checked

**Purpose**: write `Decl` statements the pinned engine accepts, choose bound types for every
column, and know which checks happen where. Every form below was loaded and evaluated with
`nerd check-mangle --standalone --eval` on the pinned engine (2026-09-19); the engine's own spec is
`docs/spec_decls.md` in the module cache, and where the two disagree the engine wins (the spec
spells the keyword `bounds`; the parser only accepts `bound`).

## Declarations

Every predicate needs one `Decl` before use, and only one per loaded unit. Argument names in a
`Decl` are variables (uppercase); they document the columns and bind nothing.

```mangle
Decl employee(ID, Name, Dept) bound [/number, /string, /name].
Decl salary(EmpID, Amount) bound [/number, /float64].
```

A `bound [...]` block lists one type per argument, in order, and **the count must match the
arity exactly**: `Decl pair(X, Y) bound [/number].` fails with `expected 2 bounds, got 1`. Omit
the block entirely (`Decl pair(X, Y).`) and every column accepts anything.

## Bound types

Type constants:

| Bound | Holds |
|---|---|
| `/number` | 64-bit signed integer |
| `/float64` | 64-bit float |
| `/string` | UTF-8 string, `"quoted"` |
| `/name` | name constant, `/active` |
| `/bytes` | byte string, `b"ab"` |
| `/any` | anything (use it for a column you do not want to constrain) |

Type functions are **capitalised**; the lowercase spelling (`fn:list(/string)`) is an analysis
error in a bound, because lowercase `fn:list` is the value-level list constructor:

| Bound | Holds | Literal |
|---|---|---|
| `fn:List(/string)` | list | `["critical", "urgent"]` |
| `fn:Map(/string, /number)` | map | `["a": 1, "b": 2]` |
| `fn:Pair(/string, /number)` | pair | `fn:pair("a", 1)` |
| `fn:Struct(/name, /string, /age, /number)` | struct with those fields | `{/name: "ann", /age: 3}` |
| `fn:Union(/string, /number)` | either type | `"s"` or `4` |
| `fn:Singleton(/only)` | exactly that name | `/only` |

```mangle
Decl tags(ID, Tags) bound [/number, fn:List(/string)].
Decl person(ID, Info) bound [/number, fn:Struct(/name, /string, /age, /number)].
Decl setting(Key, Value) bound [/name, fn:Union(/string, /number)].

tags(1, ["urgent", "critical"]).
person(1, {/name: "Alice", /age: 30}).
setting(/port, 8080).
setting(/host, "localhost").
```

Several `bound` blocks are alternatives: `Decl t(X) bound [/number] bound [/string].` accepts
either. `Decl t(X) temporal bound [...]` declares a temporal predicate (see
[960-FORK_FEATURES](960-FORK_FEATURES_v0.5.1.md); codeNERD's kernel cannot evaluate temporal
literals).

### Tagged unions (the canonical line's type expression form)

A discriminated union over structs, written in the `.Type<...>` expression form inside a bound
(from the engine's `examples/tagged_union.mg`):

```mangle
Decl api_message(M)
  bound[
    .TaggedUnion</type,
      /create : .Struct</name : /string, /count : /number>,
      /delete : .Struct</id : /number>,
      /ping   : .Struct<>
    >
  ].

api_message({/type: /create, /name: "widget", /count: 5}).
api_message({/type: /ping}).
```

## Reading structured values

Destructure with builtins; there is no `[Head | Tail]` pattern and no subscript (`L[0]`), and both
are parse errors.

```mangle
Decl person(ID, Info) bound [/number, fn:Struct(/name, /string, /age, /number)].
Decl person_name(ID, Name) bound [/number, /string].
Decl tags(ID, Tags) bound [/number, fn:List(/string)].
Decl tagged(ID, Tag) bound [/number, /string].

person(1, {/name: "Alice", /age: 30}).
tags(1, ["urgent", "critical"]).

person_name(ID, Name) :- person(ID, Info), :match_field(Info, /name, Name).
tagged(ID, Tag) :- tags(ID, Tags), :list:member(Tag, Tags).
```

`:match_field(Struct, Field, Out)` needs `Struct` bound by an earlier premise and `Out` free: a
constant in the third position is an analysis error (`expected /CallExpr (arg 2) to be a free
variable`). Bind it, then compare: `:match_field(N, /type, T), T = /call_expr .`

Lists are built with `fn:list(...)`, `fn:list:cons(Head, Tail)` and `fn:list:append(...)`, read
with `fn:list:get(L, I)`, `fn:list:len(L)` and `fn:list:contains(L, X)` (which returns `/true` or
`/false`), and enumerated with `:list:member(X, L)`.

## What is checked, and where

- **The engine does not type-check facts against bounds.** `Decl t(X) bound [/number].` followed
  by `t(1.5).` loads and stores `1.5`. A value of the wrong shape simply never joins with a rule
  that expects the right one: the classic silent empty result. Keep the producer (a Go assert or a
  fact), the `Decl` and the consumer (a rule premise) identical.
- **Comparisons are integer-only.** `<`, `<=`, `>`, `>=` work on `/number`; on a `/float64` column
  evaluation aborts (`value 0.9 (4) is not a number`), and the engine has no float comparison.
  codeNERD keeps scores as integers 0..100 for this reason.
- **codeNERD's kernel adds its own check for facts Go asserts.** `convertValueToTypedTerm`
  (`internal/mangle/engine.go`) consults the bound to turn Go strings into `/name` or `/string`
  constants (explicit `types.MangleAtom` / `types.MangleString` win), and a float asserted where a
  number is declared aborts the fixpoint (`kernel_fact_decl.go`). Pass typed values from Go.
- **No inference.** Nothing infers a column's type from how rules use it.

## Wrong forms models produce

```mangle
# WRONG: annotated variables are the Google 0.4.0 form; the pinned parser rejects them
Decl employee(ID.Type<int>, Name.Type<string>).
# WRONG: bounds are type expressions, not literals or ad-hoc syntax
Decl tags(ID, Tags) bound [/number, [string]].
Decl flexible(Value) bound [int | string].
# WRONG: the keyword is bound, not bounds (even though the engine's spec says bounds)
Decl t(X) bounds [/number].
```

Right forms for the same intent: `bound [/number, fn:List(/string)]`,
`bound [fn:Union(/number, /string)]`, `bound [/number]`.

---

**See also**: [200-SYNTAX_REFERENCE](200-SYNTAX_REFERENCE.md) section 6,
[020-ENGINE_TRUTHS](020-ENGINE_TRUTHS_v0.5.1.md).
