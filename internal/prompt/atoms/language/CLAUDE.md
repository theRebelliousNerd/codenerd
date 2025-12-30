# language/ - Programming Language Atoms

Language-specific guidance, idioms, and encyclopedic references.

## Files

| File | Language | Content |
|------|----------|---------|
| `go.yaml` | Go | Core idioms, error handling, concurrency |
| `python.yaml` | Python | Core patterns |
| `python_advanced.yaml` | Python | Advanced patterns |
| `python_encyclopedia.yaml` | Python | Comprehensive reference |
| `rust.yaml` | Rust | Ownership, lifetimes, patterns |
| `rust_encyclopedia.yaml` | Rust | Comprehensive reference |
| `typescript.yaml` | TypeScript | Core patterns |
| `typescript_encyclopedia.yaml` | TypeScript | Comprehensive reference |
| `javascript.yaml` | JavaScript | Core patterns |
| `javascript_encyclopedia.yaml` | JavaScript | Comprehensive reference |
| `java.yaml` | Java | Core patterns |
| `java_encyclopedia.yaml` | Java | Comprehensive reference |
| `mangle.yaml` | Mangle | Datalog syntax basics |
| `mangle_encyclopedia.yaml` | Mangle | Comprehensive reference |

## Selection

Language atoms are selected via `languages` selector:

```yaml
languages: ["/go", "/golang"]
```

## Encyclopedia Pattern

`*_encyclopedia.yaml` files contain comprehensive references with `content_concise` and `content_min` variants for budget fitting.
