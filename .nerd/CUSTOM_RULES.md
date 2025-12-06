# Custom Review Rules

ReviewerShard supports loading custom review rules from JSON files. This allows you to enforce project-specific code standards, security policies, and best practices.

## Quick Start

1. Create a custom rules file at `.nerd/review-rules.json`
2. Define your rules using the JSON format below
3. ReviewerShard will automatically load them on initialization

## Rule Format

```json
{
  "version": "1.0",
  "rules": [
    {
      "id": "CUSTOM001",
      "category": "security",
      "severity": "critical",
      "pattern": "regex pattern here",
      "message": "Description of the issue",
      "suggestion": "How to fix it (optional)",
      "languages": ["go", "python"],
      "description": "Human-readable description (optional)",
      "enabled": true
    }
  ]
}
```

## Fields

### Required Fields

- **id**: Unique identifier for the rule (e.g., "CUSTOM001")
- **category**: One of: `security`, `style`, `bug`, `performance`, `maintainability`
- **severity**: One of: `critical`, `error`, `warning`, `info`
- **pattern**: Regular expression pattern to match against code lines
- **message**: Error message shown when the rule triggers
- **enabled**: Boolean to enable/disable the rule

### Optional Fields

- **suggestion**: Suggested fix or alternative approach
- **languages**: Array of language filters (e.g., `["go", "python"]`). Empty = all languages
- **description**: Human-readable description of what the rule checks

## Supported Languages

Language filters use these identifiers:
- `go` - Go files (.go)
- `python` - Python files (.py)
- `javascript` - JavaScript files (.js, .jsx)
- `typescript` - TypeScript files (.ts, .tsx)
- `rust` - Rust files (.rs)
- `java` - Java files (.java)
- `c` - C files (.c, .h)
- `cpp` - C++ files (.cpp, .cc, .hpp)
- `csharp` - C# files (.cs)
- `ruby` - Ruby files (.rb)
- `php` - PHP files (.php)

## Example Rules

### Security: Prohibit eval()

```json
{
  "id": "CUSTOM001",
  "category": "security",
  "severity": "critical",
  "pattern": "(?i)eval\\s*\\(",
  "message": "Use of eval() is prohibited - major security risk",
  "suggestion": "Use safer alternatives like JSON.parse() or explicit function calls",
  "languages": ["javascript", "typescript"],
  "enabled": true
}
```

### Style: Enforce Structured Logging

```json
{
  "id": "CUSTOM002",
  "category": "bug",
  "severity": "error",
  "pattern": "fmt\\.Print[^l]",
  "message": "Use of fmt.Print/Printf detected - prefer structured logging",
  "suggestion": "Use log.Printf() or a structured logger like slog",
  "languages": ["go"],
  "enabled": true
}
```

### Performance: Inefficient List Building

```json
{
  "id": "CUSTOM005",
  "category": "performance",
  "severity": "info",
  "pattern": "\\+\\=.*\\[\\]",
  "message": "Inefficient list concatenation in loop",
  "suggestion": "Consider using list comprehension or append to pre-allocated list",
  "languages": ["python"],
  "enabled": true
}
```

### Maintainability: Python None Checks

```json
{
  "id": "CUSTOM006",
  "category": "maintainability",
  "severity": "warning",
  "pattern": "if\\s+\\w+\\s*==\\s*None:",
  "message": "Use 'if x is None:' instead of 'if x == None:'",
  "suggestion": "Replace == None with 'is None' for identity check",
  "languages": ["python"],
  "enabled": true
}
```

## Regex Pattern Tips

1. **Case Insensitive**: Use `(?i)` prefix for case-insensitive matching
   ```
   "pattern": "(?i)todo"  // matches TODO, todo, ToDo, etc.
   ```

2. **Word Boundaries**: Use `\\b` to match whole words only
   ```
   "pattern": "\\beval\\b"  // matches eval() but not evaluate()
   ```

3. **Escape Special Characters**: Regex metacharacters need escaping
   - `.` → `\\.`
   - `(` → `\\(`
   - `[` → `\\[`
   - `\\` → `\\\\`

4. **Character Classes**: Use `\\s` for whitespace, `\\w` for word chars
   ```
   "pattern": "if\\s+\\w+\\s*==\\s*None"
   ```

5. **Quantifiers**: Use `*` (0+), `+` (1+), `?` (0-1), `{n}` (exactly n)
   ```
   "pattern": "password\\s*=\\s*['\"][^'\"]{8,}['\"]"
   ```

## Programmatic Usage

### Load Rules from File

```go
reviewer := NewReviewerShard()
if err := reviewer.LoadCustomRules(".nerd/review-rules.json"); err != nil {
    log.Printf("Failed to load custom rules: %v", err)
}
```

### Add Individual Rules

```go
rule := CustomRule{
    ID:         "CUSTOM999",
    Category:   "security",
    Severity:   "critical",
    Pattern:    "(?i)password\\s*=\\s*['\"]",
    Message:    "Hardcoded password detected",
    Suggestion: "Use environment variables or secret management",
    Enabled:    true,
}

if err := reviewer.AddCustomRule(rule); err != nil {
    log.Printf("Failed to add custom rule: %v", err)
}
```

### Get All Custom Rules

```go
rules := reviewer.GetCustomRules()
for _, rule := range rules {
    fmt.Printf("%s: %s\n", rule.ID, rule.Message)
}
```

### Clear All Custom Rules

```go
reviewer.ClearCustomRules()
```

## File Locations

ReviewerShard looks for custom rules in these locations (in order):

1. Path specified in `ReviewerConfig.CustomRulesPath`
2. `.nerd/review-rules.json` (default)
3. Project root `review-rules.json`

## Validation

Rules are validated when loaded:
- ✅ Required fields must be present
- ✅ Severity must be valid (`critical`, `error`, `warning`, `info`)
- ✅ Category must be valid (`security`, `style`, `bug`, `performance`, `maintainability`)
- ✅ Pattern must be a valid regular expression
- ✅ No duplicate rule IDs

Invalid rules are skipped with a warning.

## Tips

1. **Start Conservative**: Begin with `warning` or `info` severity, promote to `error` after validation
2. **Test Patterns**: Use https://regex101.com/ to test regex patterns
3. **Use Descriptive IDs**: Prefix with category (SEC001, STYLE001, BUG001)
4. **Document Rules**: Use the `description` field for maintenance
5. **Version Control**: Commit `.nerd/review-rules.json` to share team standards
6. **Disable Experimental**: Set `enabled: false` for rules being tested

## Example: Full Custom Rules File

See `.nerd/review-rules.example.json` for a complete working example with 7 different rule types.

## Integration with Built-in Rules

Custom rules run alongside built-in ReviewerShard rules:
1. Code DOM safety checks
2. Built-in security patterns
3. Built-in style patterns
4. Built-in bug patterns
5. **Custom rules** ← Your rules run here
6. LLM-powered semantic analysis
7. Learned anti-patterns

Custom rules are checked for every file, respecting language filters.
