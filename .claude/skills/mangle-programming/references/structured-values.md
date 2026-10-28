# Structured values: one fact, one whole thing

Plain Datalog forces every object into flat columns, which spreads one
event across five relations. Mangle lets a fact carry a list, a map, a
struct or a tagged union as a single constant, and rules take it apart
with `:match_*`. Use this when the thing travels together and is
consumed together.

## Where it pays

- **Events arriving from Go.** A tool call, an LLM turn, a file change:
  assert the whole record as one struct, and let rules dispatch on its tag.

  ```mangle
  Decl tool_event(E) bound [.TaggedUnion</kind,
    /read  : .Struct</path : /string>,
    /edit  : .Struct</path : /string, /lines : /number>,
    /shell : .Struct</argv : .List</string>>>].
  edited(P) :- tool_event(E), :match_field(E, /kind, K), K = /edit,
               :match_field(E, /path, P).
  ```

  A new variant means changing the Decl and the rules that care about it,
  not adding a new predicate to the schema and every producer.
- **Payloads handed back to Go.** A `do` transform with `fn:collect` or
  `fn:collect_to_map` gives Go one fact holding the whole answer, such as a
  map from file to risk. Go reads one row instead of joining several.
- **Paths and witnesses.** A list keeps the order and the route
  (`[/a, /b, /d]`), so "why" travels with "what". The engine's
  shortest-path example works this way.
- **Composite identity.** `fn:pair(Path, Digest)` used as a key gives one
  join column instead of two.

## Go side

- Build values with the engine's `ast` constructors at the producer, from
  the Decl. Don't format strings and parse them back.
- Read results by destructuring the constant in Go: `ast.Constant` holds
  pair, list, map and struct shapes. Don't re-query to rebuild them.
- `json2struct` and `proto2struct` in the engine module turn JSON or
  protobuf into structs. They're worth an experiment for ingesting tool
  output whole.

## Where it hurts

- A rule that must join, negate or aggregate one component wants that
  component as its own column. Destructure once into a narrow relation,
  then reason over that.
- Building new structured terms inside a hot recursive rule allocates
  heavily and can grow without bound.
- Deep nesting makes hashing and provenance expensive at scale.

## Experiments worth running

- Replace a family of flat `*_event` predicates with one tagged union.
  Compare rule count, readability, and the Decl churn a new event kind causes.
- Have the kernel emit a ready-to-render struct for the UI, so Go stops
  assembling it from several queries.
- Turn on strict bounds (the lab default) over a tagged-union Decl and see
  what a wrong variant field costs: a compile error, or silence?
