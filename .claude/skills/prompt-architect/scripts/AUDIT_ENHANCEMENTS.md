# Prompt Audit Script Enhancements

## Overview

The `audit_prompts.py` script has been enhanced from 141 lines to 779 lines with comprehensive validation capabilities.

## What Was Added

### 1. Severity Levels (4 Levels)

```python
class Severity(Enum):
    ERROR = "ERROR"    # Must fix - blocks deployment
    WARN = "WARN"      # Should fix - quality issue
    INFO = "INFO"      # Suggestion - improvement opportunity
    OK = "OK"          # Passed check
```

### 2. Data Structures

- `Issue` dataclass: Represents a single audit issue
- `PromptStats` dataclass: Statistics for a single prompt
- `AuditReport` dataclass: Complete audit report with aggregated statistics

### 3. Comprehensive Checks

#### Structural Checks (6 checks)
- **schema_completeness**: JSON schema completeness (control_packet, surface_response)
- **piggyback_ordering**: Piggyback field ordering (control_packet BEFORE surface_response)
- **thought_first_ordering**: Thought-first ordering directive presence
- **reasoning_directive**: ReasoningTraceDirective presence
- **artifact_classification**: Artifact classification for mutation prompts
- **context_injection**: SessionContext injection markers

#### Semantic Checks (5 checks)
- **context_categories**: Context category usage (diagnostics, test state, git state, etc.)
- **tool_steering**: Tool steering quality (AVAILABLE TOOLS section)
- **mangle_atoms**: Control packet references to mangle atoms
- **specificity_score**: Specificity score (ratio of specific terms vs generic)
- **hallucination_catalog**: Hallucination catalog presence for shard prompts

#### Specialist Checks (3 checks)
- **specialist_knowledge**: KnowledgeAtom placeholder presence for Type B/U
- **specialist_hints**: SpecialistHints injection
- **domain_constraints**: Domain constraint sections

#### Anti-Pattern Checks (15 checks from anti-patterns.md)
1. **surface_before_control**: Premature Articulation (Bug #14)
2. **raw_text_output**: Raw Text Output (Protocol Bypass)
3. **missing_reasoning**: Missing Reasoning Trace
4. **context_starvation**: Context Starvation
5. **tool_hallucination**: Tool Hallucination
6. **artifact_amnesia**: Artifact Amnesia
7. **intent_misclassification**: Intent Misclassification
8. **constitutional_bypass**: Constitutional Bypass
9. **permission_escalation**: Permission Escalation
10. **feature_creep**: Feature Creep
11. **over_engineering**: Over-Engineering
12. **copy_paste_syndrome**: Copy-Paste Syndrome
13. **context_flooding**: Context Flooding
14. **context_ignorance**: Context Ignorance
15. **injection_vulnerability**: Injection Vulnerability

### 4. Output Formats

#### Console Output (Default)
- Colored output (ERROR=red, WARN=yellow, INFO=blue, OK=green)
- Grouped by file and severity
- Summary statistics
- Verbose mode to show all checks including passed

#### JSON Output (`--json`)
```json
{
  "summary": {
    "files_scanned": 24,
    "prompts_found": 18,
    "average_length": 12450.3,
    "errors": 2,
    "warnings": 3,
    "info": 1,
    "passed": 12
  },
  "prompts": [...]
}
```

#### Markdown Output (`--markdown FILE`)
- Complete report with file grouping
- Statistics tables
- Check type breakdown
- Timestamped

### 5. Statistics Tracking

- Total prompts scanned
- By severity breakdown (ERROR, WARN, INFO, OK)
- By check type breakdown
- Average prompt length
- Coverage percentage
- Files scanned count

### 6. Command Line Options

```bash
--root DIR        # Root directory to scan (default: .)
--verbose         # Show all checks including passed
--json            # Output as JSON
--markdown FILE   # Output markdown report to file
--min-length N    # Minimum prompt length (default: 8000)
--shard-length N  # Minimum shard prompt length (default: 15000)
--fail-on LEVEL   # Exit 1 if issues at this level (ERROR/WARN)
```

### 7. Example Outputs

#### Console (Default)
```
=== Prompt Audit Report ===

Scanning: c:\CodeProjects\codeNERD\internal

[ERROR] internal/shards/coder/generation.go :: buildSystemPrompt
  - Missing THOUGHT-FIRST ordering directive
  - Control packet does not reference mangle atoms

[WARN] internal/shards/tester/generation.go :: testerSystemPrompt
  - Prompt length 12,450 chars (minimum for shard: 15,000)
  - Missing hallucination catalog section

[INFO] internal/perception/transducer.go :: transducerSystemPrompt
  - Consider adding more disambiguation rules

=== Summary ===
Files Scanned: 24
Prompts Found: 18
Errors: 2
Warnings: 3
Info: 1
Passed: 12
```

## Integration with Anti-Patterns

The script now checks for all 15 anti-patterns documented in `anti-patterns.md`:

1. **Premature Articulation** - Checks for THOUGHT-FIRST directive
2. **Raw Text Output** - Checks for JSON enforcement
3. **Missing Reasoning** - Checks for reasoning_trace requirement
4. **Context Starvation** - Checks for dynamic injection markers
5. **Tool Hallucination** - Checks for explicit tool constraints
6. **Artifact Amnesia** - Checks for artifact_type in mutation prompts
7. **Intent Misclassification** - Checks for disambiguation rules
8. **Constitutional Bypass** - Checks for Constitutional awareness
9. **Permission Escalation** - Checks for permission boundaries
10. **Feature Creep** - Checks for scope discipline
11. **Over-Engineering** - Checks for simplicity principle
12. **Copy-Paste Syndrome** - Checks for style matching
13. **Context Flooding** - Checks for priority ordering
14. **Context Ignorance** - Checks for context explanation
15. **Injection Vulnerability** - Checks for anti-injection directives

## Usage Examples

### Basic Scan
```bash
python audit_prompts.py --root ./internal
```

### Verbose Output
```bash
python audit_prompts.py --root ./internal --verbose
```

### JSON for CI/CD
```bash
python audit_prompts.py --root ./internal --json > audit.json
```

### Markdown Report
```bash
python audit_prompts.py --root ./internal --markdown report.md
```

### Fail on Warnings
```bash
python audit_prompts.py --root ./internal --fail-on WARN
```

### Custom Length Thresholds
```bash
python audit_prompts.py --root ./internal --min-length 10000 --shard-length 20000
```

## CI/CD Integration

The script can be integrated into CI/CD pipelines using the `--fail-on` flag:

```yaml
# GitHub Actions example
- name: Audit Prompts
  run: |
    python .claude/skills/prompt-architect/scripts/audit_prompts.py \
      --root ./internal \
      --fail-on ERROR \
      --markdown prompt-audit-report.md
```

If any errors are found, the script will exit with code 1, failing the CI build.

## Statistics

- **Original:** 141 lines
- **Enhanced:** 779 lines
- **Growth:** 5.5x expansion
- **Total Checks:** 29 different validation rules
- **Anti-Patterns:** 15 integrated from documentation
- **Output Formats:** 3 (Console, JSON, Markdown)
- **Severity Levels:** 4 (ERROR, WARN, INFO, OK)
