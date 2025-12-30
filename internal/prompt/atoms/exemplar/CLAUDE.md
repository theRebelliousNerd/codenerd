# exemplar/ - Few-Shot Example Atoms

Concrete examples demonstrating correct behavior patterns.

## Files

| File | Examples For |
|------|--------------|
| `go_exemplars.yaml` | Go code patterns and idioms |
| `piggyback_exemplars.yaml` | Piggyback Protocol response format |
| `refactor_exemplars.yaml` | Code refactoring patterns |
| `review_exemplars.yaml` | Code review output format |
| `test_exemplars.yaml` | Test generation patterns |
| `codedom_refactor_exemplars.yaml` | CodeDOM-based refactoring |

## Purpose

Few-shot examples help the LLM understand:
- Expected output format
- Quality standards
- Edge case handling
- Domain-specific patterns

## Selection

Exemplars are **flesh** (optional) atoms selected via vector similarity to the current task.
