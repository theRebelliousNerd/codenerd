# build_layer/ - Layered Architecture Atoms

Guidance for building code in architectural layers.

## Files

| File | Layer | Purpose |
|------|-------|---------|
| `scaffold.yaml` | Scaffold | Project structure, boilerplate |
| `data_layer.yaml` | Data | Database, models, repositories |
| `domain_core.yaml` | Domain | Business logic, entities |
| `service.yaml` | Service | Application services, use cases |
| `transport.yaml` | Transport | HTTP, gRPC, CLI handlers |
| `integration.yaml` | Integration | External services, APIs |

## Build Layer Progression

```
Scaffold -> Data -> Domain -> Service -> Transport -> Integration
```

## Selection

Build layer atoms are selected via `build_layer` in CompilationContext:

```yaml
build_layers: ["/data", "/repository"]
```

Each layer provides:
- Patterns for that layer
- Dependencies on lower layers
- Interface contracts
