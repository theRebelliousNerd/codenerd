# eval/ - LLM-as-Judge Evaluation Atoms

Prompts for evaluating LLM outputs.

## Files

| File | Purpose |
|------|---------|
| `judge_explanation.yaml` | LLM-as-Judge evaluation criteria and format |

## Evaluation Categories

| Category | Description |
|----------|-------------|
| `CORRECT` | Task completed successfully |
| `LOGIC_ERROR` | Wrong approach or algorithm |
| `SYNTAX_ERROR` | Code syntax issues |
| `API_MISUSE` | Wrong API or library usage |
| `EDGE_CASE` | Missing edge case handling |
| `CONTEXT_MISS` | Missed relevant codebase context |
| `INSTRUCTION_MISS` | Didn't follow instructions |
| `HALLUCINATION` | Made up information |

## Usage

Used by `internal/autopoiesis/prompt_evolution/judge.go` to evaluate execution outcomes and drive prompt evolution.
