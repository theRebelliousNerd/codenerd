# Mangle Programming Skill Assets

Ready-to-use templates and examples for Mangle development.

## Templates

### starter-schema.gl

A comprehensive schema template with predicate declarations organized by category:

- Core entity predicates
- Attribute predicates
- Relationship predicates
- Temporal predicates
- Structured data predicates
- Derived predicate declarations
- Aggregation result predicates

**Usage**: Copy and customize for your domain.

### starter-policy.gl

A policy template with common IDB rules:

- Transitive closure patterns
- Path construction
- Negation patterns (set difference)
- Aggregation rules
- Classification rules
- Sibling/peer relationships
- Structured data access
- Temporal rules
- Safety/validation rules

**Usage**: Copy and add domain-specific rules.

### codenerd-schemas.gl

Schema template specifically for codeNERD's neuro-symbolic architecture:

- Intent & Focus (perception transducer output)
- World Model (file system, symbol graph)
- Diagnostics
- TDD loop state
- Shard management
- Research & knowledge
- Observations & memory
- Campaign & goals
- Autopoiesis (tool generation)

**Usage**: Reference for codeNERD development.

## Examples

### examples/vulnerability-scanner.mg

Complete vulnerability analysis program demonstrating:

- Dependency tracking (transitive)
- CVE propagation
- Patched version exclusion
- Vulnerability path construction
- Summary statistics

### examples/access-control.mg

Role-based access control (RBAC) example showing:

- Role hierarchy
- Permission inheritance
- Explicit denials
- Owner override
- Access auditing
- Privilege analysis

### examples/aggregation-patterns.mg

Comprehensive aggregation examples covering:

- Simple count
- Sum aggregation
- Min/Max
- Multi-variable grouping
- Average computation
- Filtering before aggregation
- Aggregation with joins
- Nested aggregation
- Conditional aggregation
- Existence checks

## Go Integration

### go-integration/

Complete Go boilerplate for embedding Mangle:

- `main.go` - Full working example
- `go.mod` - Module definition

Features demonstrated:

- Parsing Mangle source
- Creating evaluation engine
- Adding facts dynamically
- Querying results
- Type conversions (Go ↔ Mangle)

**Usage**:

```bash
cd go-integration
go mod tidy
go run main.go
```

## Quick Start

1. **New project**: Copy `starter-schema.gl` and `starter-policy.gl`
2. **Vulnerability analysis**: Start from `examples/vulnerability-scanner.mg`
3. **Access control**: Start from `examples/access-control.mg`
4. **Learn aggregation**: Study `examples/aggregation-patterns.mg`
5. **Go integration**: Copy `go-integration/` directory
