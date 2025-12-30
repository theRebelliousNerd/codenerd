# hallucination/ - Anti-Hallucination Guards

Shard-specific guidance to prevent common hallucination patterns.

## Files

| File | Shard | Common Hallucinations Prevented |
|------|-------|--------------------------------|
| `coder_hallucinations.yaml` | Coder | Inventing APIs, wrong imports, phantom files |
| `tester_hallucinations.yaml` | Tester | Fake test frameworks, wrong assertions |
| `reviewer_hallucinations.yaml` | Reviewer | Imagined vulnerabilities, false positives |
| `researcher_hallucinations.yaml` | Researcher | Made-up documentation, wrong URLs |
| `nemesis_hallucinations.yaml` | Nemesis | Impossible attack vectors |
| `librarian_hallucinations.yaml` | Librarian | Phantom knowledge atoms |

## Key Pattern

Each file lists:
- Common hallucination types for that shard
- Verification steps before outputting
- Grounding requirements (must cite source)

## Usage

Hallucination atoms are selected alongside identity atoms via `shard_types`.
