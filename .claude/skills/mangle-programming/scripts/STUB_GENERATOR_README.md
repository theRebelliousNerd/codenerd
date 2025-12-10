# Mangle-to-Go Stub Generator

Accelerate VirtualStore development by automatically generating Go stub implementations for Mangle virtual predicates.

## Quick Start

```bash
# List all predicates in schema
python generate_stubs.py schemas.mg --list

# Generate stubs for all virtual predicates (Section 7B)
python generate_stubs.py schemas.mg --virtual-only --output virtual_preds.go

# Generate specific predicates
python generate_stubs.py schemas.mg --predicates "user_intent,file_topology" --output stubs.go

# Generate to stdout for review
python generate_stubs.py schemas.mg --predicates "recall_similar"
```

## Usage

```
python generate_stubs.py <schema_file.mg> [options]

Options:
  --output FILE, -o FILE    Output Go file path (default: stdout)
  --package NAME, -p NAME   Go package name (default: mangle)
  --predicates LIST         Comma-separated list of predicates to generate
  --list, -l                Just list declared predicates with arities
  --interface-only          Generate interface definitions only, no stubs
  --virtual-only            Only generate virtual predicates (Section 7B)
```

## Examples

### 1. Generate Virtual Predicates for VirtualStore

The most common use case - generating stubs for all virtual predicates that need Go implementations:

```bash
python generate_stubs.py internal/core/defaults/schemas.mg \
  --virtual-only \
  --output internal/core/virtual_predicates.go \
  --package core
```

This generates:
- `QueryLearnedPredicate`
- `QuerySessionPredicate`
- `RecallSimilarPredicate`
- `QueryKnowledgeGraphPredicate`
- `QueryActivationsPredicate`
- `HasLearnedPredicate`
- And helper predicates from Section 7D

### 2. Generate Specific Predicates

For targeted development:

```bash
python generate_stubs.py schemas.mg \
  --predicates "user_intent,next_action,permitted" \
  --output kernel_preds.go
```

### 3. List All Available Predicates

Explore what's available in a schema:

```bash
python generate_stubs.py schemas.mg --list
```

Output:
```
Predicates found in schemas.mg:

  user_intent/5
    user_intent(ID, Category, Verb, Target, Constraint)
    Category: /query, /mutation, /instruction
    Section: §1 INTENT SCHEMA

  query_learned/2 [VIRTUAL]
    query_learned(Predicate, Args) - Queries cold_storage for learned facts
    Section: §7B VIRTUAL PREDICATES FOR KNOWLEDGE QUERIES

  ...

Total: 387 predicates
```

### 4. Interface-Only Mode

Generate just the type definitions and methods without stub implementations:

```bash
python generate_stubs.py schemas.mg \
  --predicates "user_intent" \
  --interface-only
```

Useful for:
- Prototyping
- Generating headers for documentation
- Quick reference

## Type Mappings

The generator automatically maps Mangle types to Go engine types:

| Mangle Type | Go Type | Example |
|-------------|---------|---------|
| `Type<n>` | `engine.Atom` | `/foo`, `/bar` |
| `Type<name>` | `engine.Atom` | Name constants |
| `Type<string>` | `engine.String` | `"hello"` |
| `Type<int>` | `engine.Int64` | `42` |
| `Type<float>` | `engine.Float64` | `3.14` |
| `Type<[T]>` | `engine.List` | `[1, 2, 3]` |
| `Type<{/k: v}>` | `engine.Map` | `{/key: "value"}` |
| `Type<Any>` | `engine.Value` | Any value |

## Generated Code Structure

Each predicate gets:

1. **Struct Type** - PascalCase name with `Predicate` suffix
   ```go
   type UserIntentPredicate struct {}
   ```

2. **Name Method** - Returns predicate name
   ```go
   func (p *UserIntentPredicate) Name() string { return "user_intent" }
   ```

3. **Arity Method** - Returns argument count
   ```go
   func (p *UserIntentPredicate) Arity() int { return 5 }
   ```

4. **Query Method** - Stub implementation with helpful comments
   ```go
   func (p *UserIntentPredicate) Query(query engine.Query, callback func(engine.Fact) error) error {
       // Arguments:
       //   query.Args[0] - ID: engine.Value
       //   query.Args[1] - Category: engine.Value
       // ...
       // TODO: Implement query logic
       return nil
   }
   ```

5. **Registration Function** - Registers all predicates
   ```go
   func RegisterPredicates(store *engine.FactStore) {
       store.RegisterVirtual(&UserIntentPredicate{})
       // ...
   }
   ```

## Integration with VirtualStore

After generating stubs:

1. **Review and implement** the `Query` method for each predicate
2. **Add to VirtualStore** initialization:
   ```go
   // In internal/core/virtual_store.go
   import "your/package/path"

   func NewVirtualStore() *VirtualStore {
       store := &VirtualStore{...}
       RegisterPredicates(store.engine)
       return store
   }
   ```

3. **Implement virtual logic** in each predicate's `Query` method:
   ```go
   func (p *QueryLearnedPredicate) Query(query engine.Query, callback func(engine.Fact) error) error {
       // Extract bound arguments
       predName, _ := query.Args[0].(engine.Atom)

       // Query knowledge.db
       rows := db.Query("SELECT args FROM cold_storage WHERE predicate = ?", predName)

       // Return results via callback
       for rows.Next() {
           var args string
           rows.Scan(&args)
           callback(engine.Fact{
               Predicate: "query_learned",
               Args: []engine.Value{predName, engine.String(args)},
           })
       }
       return nil
   }
   ```

## Development Workflow

### Bootstrap New Virtual Predicates

1. Add `Decl` statements to `schemas.mg` in Section 7B
2. Generate stubs:
   ```bash
   python generate_stubs.py schemas.mg --virtual-only --output virtual_preds.go
   ```
3. Implement `Query` methods
4. Register with VirtualStore
5. Test with Mangle queries

### Regenerate After Schema Changes

```bash
# Backup existing implementations
cp virtual_preds.go virtual_preds.go.bak

# Regenerate stubs
python generate_stubs.py schemas.mg --virtual-only --output virtual_preds.go

# Merge implementations from backup
# (Keep your Query implementations, replace boilerplate)
```

## Tips

- **Virtual-only flag** - Use `--virtual-only` to focus on predicates that need Go implementations
- **Incremental generation** - Use `--predicates` to generate one predicate at a time during development
- **Review before commit** - Generated stubs are templates; always review and implement logic
- **Keep comments** - The generated comments include declaration signatures and type info

## Advanced: Custom Type Annotations

The generator parses `Type<...>` annotations from Decl statements. For best results:

```mangle
# Good: Explicit types
Decl user_intent(ID.Type<n>, Category.Type<n>, Verb.Type<n>, Target.Type<string>, Constraint.Type<Any>).

# Works: No annotations (defaults to Type<Any>)
Decl simple_fact(Arg1, Arg2).

# Best: With inline comments
Decl file_topology(Path, Hash, Language, LastModified, IsTestFile).
# Language: /go, /python, /ts, /rust
# IsTestFile: /true, /false
```

## Limitations

- Does not generate actual implementation logic (by design - that's your job!)
- Type inference is basic - complex nested types default to `engine.Value`
- Comments are extracted from preceding lines (up to 10 lines back)
- Section detection relies on standard Mangle schema header format

## See Also

- `diagnose_stratification.py` - Stratification checker for Mangle rules
- `.claude/skills/mangle-programming/SKILL.md` - Complete Mangle language reference
- `internal/core/virtual_store.go` - VirtualStore implementation reference
