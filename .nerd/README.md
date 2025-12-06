# .nerd Directory

This directory contains codeNERD configuration files and custom review rules.

## Files

### review-rules.json

Custom review rules for the ReviewerShard. Define your own code review patterns here.

**Format:**
```json
{
  "version": "1.0",
  "rules": [
    {
      "id": "CUSTOM001",
      "category": "security",
      "severity": "critical",
      "pattern": "regex pattern",
      "message": "Description",
      "suggestion": "How to fix",
      "languages": ["go", "python"],
      "enabled": true
    }
  ]
}
```

**Documentation:** See `CUSTOM_RULES.md` for full documentation

**Example:** See `review-rules.example.json` for working examples

## Quick Start

1. Copy the example file:
   ```bash
   cp review-rules.example.json review-rules.json
   ```

2. Edit `review-rules.json` to add your custom rules

3. ReviewerShard will automatically load them on initialization

## Rule Categories

- `security` - Security vulnerabilities
- `style` - Code style and formatting
- `bug` - Potential bugs and errors
- `performance` - Performance issues
- `maintainability` - Code maintainability

## Rule Severities

- `critical` - Blocks commit, must be fixed
- `error` - Should be fixed before merge
- `warning` - Should be addressed
- `info` - Informational, FYI

## Testing Rules

Use regex101.com to test your patterns before adding them.

## Version Control

Commit `review-rules.json` to share rules across your team.
