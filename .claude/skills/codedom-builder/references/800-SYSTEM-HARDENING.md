# 800: System Hardening

## Overview

This document covers four critical lenses for building a bulletproof Code DOM system: Temporal (code evolution), Runtime (value binding), Adversarial (logic injection defense), and Resilience (broken state handling).

## Lens 1: Temporal (Code Evolution)

### The Problem

The current Code DOM is a snapshot of "Now." It lacks memory of "Then."

**Failure Scenario:**
```text
Turn 1: Agent refactors auth.User.login()
Turn 5: Regression test fails
Turn 6: Agent sees broken code but has no context of what it looked like before
        or why it made the change
```

### Ref Permanence & Git Binding

Extend the `Ref` system to include temporal markers and bind to git history.

#### New Mangle Facts

```mangle
# =============================================================================
# TEMPORAL FACTS (Git Binding)
# =============================================================================

# Who changed this Ref last?
Decl last_modified(Ref.Type<string>, CommitHash.Type<string>, Author.Type<string>, Timestamp.Type<int>).

# What did it look like before?
Decl previous_body(Ref.Type<string>, CommitHash.Type<string>, SourceCode.Type<string>).

# Agent's own changes (this session)
Decl agent_modified(Ref.Type<string>, TurnNumber.Type<int>, OldBody.Type<string>, NewBody.Type<string>).

# Revert capability
Decl revertable(Ref.Type<string>, ToCommit.Type<string>).

revertable(Ref, CommitHash) :-
    agent_modified(Ref, _, _, _),
    previous_body(Ref, CommitHash, _).
```

#### Git Integration

```go
// internal/world/git_temporal.go

type TemporalBinding struct {
    repo     *git.Repository
    parser   *CodeElementParser
}

// GetElementHistory returns the git history for a specific Ref
func (tb *TemporalBinding) GetElementHistory(ref string, limit int) ([]ElementVersion, error) {
    elem, err := tb.parser.GetElement(ref)
    if err != nil {
        return nil, err
    }

    // Git log for the specific lines
    blameResult, err := tb.repo.Blame(&git.BlameOptions{
        Path: elem.File,
    })
    if err != nil {
        return nil, err
    }

    var versions []ElementVersion
    seen := make(map[string]bool)

    for line := elem.StartLine - 1; line < elem.EndLine; line++ {
        lineBlame, _ := blameResult.Line(line)
        if lineBlame != nil && !seen[lineBlame.Hash.String()] {
            seen[lineBlame.Hash.String()] = true
            versions = append(versions, ElementVersion{
                Ref:        ref,
                CommitHash: lineBlame.Hash.String(),
                Author:     lineBlame.Author,
                Timestamp:  lineBlame.When,
                Message:    lineBlame.Text,
            })
        }
    }

    if len(versions) > limit {
        versions = versions[:limit]
    }
    return versions, nil
}

// GetPreviousBody retrieves the element body at a specific commit
func (tb *TemporalBinding) GetPreviousBody(ref string, commitHash string) (string, error) {
    elem, err := tb.parser.GetElement(ref)
    if err != nil {
        return "", err
    }

    commit, err := tb.repo.CommitObject(plumbing.NewHash(commitHash))
    if err != nil {
        return "", err
    }

    file, err := commit.File(elem.File)
    if err != nil {
        return "", err
    }

    content, err := file.Contents()
    if err != nil {
        return "", err
    }

    // Re-parse at that commit to extract element
    elements, err := tb.parser.ParseContent(elem.File, []byte(content))
    if err != nil {
        return "", err
    }

    for _, e := range elements {
        if e.Ref == ref {
            return e.Body, nil
        }
    }

    return "", fmt.Errorf("element %s not found at commit %s", ref, commitHash)
}
```

#### Session-Level Change Tracking

```go
// Track agent's own changes for surgical revert
type AgentChangeLog struct {
    changes []AgentChange
    mu      sync.RWMutex
}

type AgentChange struct {
    TurnNumber int
    Ref        string
    OldBody    string
    NewBody    string
    Timestamp  time.Time
}

func (log *AgentChangeLog) Record(turn int, ref, oldBody, newBody string) {
    log.mu.Lock()
    defer log.mu.Unlock()
    log.changes = append(log.changes, AgentChange{
        TurnNumber: turn,
        Ref:        ref,
        OldBody:    oldBody,
        NewBody:    newBody,
        Timestamp:  time.Now(),
    })
}

// Revert to a specific turn
func (log *AgentChangeLog) RevertTo(ref string, turn int) (string, bool) {
    log.mu.RLock()
    defer log.mu.RUnlock()

    for i := len(log.changes) - 1; i >= 0; i-- {
        if log.changes[i].Ref == ref && log.changes[i].TurnNumber == turn {
            return log.changes[i].OldBody, true
        }
    }
    return "", false
}
```

## Lens 2: Runtime (Value Binding)

### The Problem

The Code DOM is purely static. The agent sees *structures* but not *values*.

**Failure Scenario:**
```python
def divide(x, y):
    return x / y  # Agent sees this

# At runtime: ZeroDivisionError because y=0
# Agent assumes y is non-zero based on type hint
```

### Runtime Overlay (The "HUD" Concept)

Capture runtime values and map them back to CodeElement Refs.

#### New Mangle Facts

```mangle
# =============================================================================
# RUNTIME VALUE OVERLAY
# =============================================================================

Decl runtime_value(Ref.Type<string>, VarName.Type<string>, Value.Type<string>, TestRunID.Type<string>).
Decl runtime_exception(Ref.Type<string>, ExceptionType.Type<string>, Message.Type<string>, TestRunID.Type<string>).
Decl runtime_assertion_failed(Ref.Type<string>, AssertExpr.Type<string>, TestRunID.Type<string>).

# Annotate elements with runtime knowledge
has_runtime_context(Ref) :- runtime_value(Ref, _, _, _).
has_runtime_failure(Ref) :- runtime_exception(Ref, _, _, _).

# Correlate static analysis with runtime data
static_dynamic_mismatch(Ref, Var, StaticType, RuntimeValue) :-
    element_type_annotation(Ref, Var, StaticType),
    runtime_value(Ref, Var, RuntimeValue, _),
    type_mismatch(StaticType, RuntimeValue).

type_mismatch("int", Value) :- fn:match("^0$", Value, _).  # Zero might indicate issue
type_mismatch("Optional", "None") :- true.  # None passed to non-optional
```

#### Trace Parser

```go
// internal/world/runtime_overlay.go

type RuntimeOverlay struct {
    values     map[string][]RuntimeValue  // ref -> runtime values
    exceptions map[string][]RuntimeException
}

type RuntimeValue struct {
    Ref       string
    VarName   string
    Value     string
    Type      string
    TestRunID string
    Line      int
}

// ParsePythonTrace extracts runtime info from a Python traceback
func (ro *RuntimeOverlay) ParsePythonTrace(trace string, testRunID string) error {
    // Parse traceback format:
    //   File "path.py", line 42, in function_name
    //     code_line
    //   ExceptionType: message

    lines := strings.Split(trace, "\n")
    var currentFile, currentFunc string
    var currentLine int

    for i, line := range lines {
        if strings.Contains(line, "File \"") {
            // Extract file, line, function
            matches := traceFileRegex.FindStringSubmatch(line)
            if len(matches) >= 4 {
                currentFile = matches[1]
                currentLine, _ = strconv.Atoi(matches[2])
                currentFunc = matches[3]
            }
        } else if strings.Contains(line, "Error:") || strings.Contains(line, "Exception:") {
            // Extract exception
            parts := strings.SplitN(line, ":", 2)
            if len(parts) == 2 {
                ref := ro.buildRef(currentFile, currentFunc)
                ro.exceptions[ref] = append(ro.exceptions[ref], RuntimeException{
                    Ref:       ref,
                    Type:      strings.TrimSpace(parts[0]),
                    Message:   strings.TrimSpace(parts[1]),
                    Line:      currentLine,
                    TestRunID: testRunID,
                })
            }
        }
    }

    return nil
}

// ParseGoTrace extracts runtime info from Go panic traces
func (ro *RuntimeOverlay) ParseGoTrace(trace string, testRunID string) error {
    // Parse Go panic format:
    //   panic: runtime error: ...
    //   goroutine 1 [running]:
    //   package.function(args)
    //       /path/file.go:42 +0x...

    lines := strings.Split(trace, "\n")
    var panicMsg string

    for i, line := range lines {
        if strings.HasPrefix(line, "panic:") {
            panicMsg = strings.TrimPrefix(line, "panic: ")
        } else if strings.Contains(line, ".go:") {
            // Extract file and line
            matches := goTraceRegex.FindStringSubmatch(line)
            if len(matches) >= 3 {
                file := matches[1]
                lineNum, _ := strconv.Atoi(matches[2])

                // Get function from previous line
                funcLine := lines[i-1]
                funcName := extractGoFuncName(funcLine)

                ref := ro.buildGoRef(file, funcName)
                ro.exceptions[ref] = append(ro.exceptions[ref], RuntimeException{
                    Ref:       ref,
                    Type:      "panic",
                    Message:   panicMsg,
                    Line:      lineNum,
                    TestRunID: testRunID,
                })
            }
        }
    }

    return nil
}

// EmitFacts generates Mangle facts from runtime data
func (ro *RuntimeOverlay) EmitFacts() []MangleFact {
    var facts []MangleFact

    for ref, values := range ro.values {
        for _, v := range values {
            facts = append(facts, MangleFact{
                Predicate: "runtime_value",
                Args:      []interface{}{ref, v.VarName, v.Value, v.TestRunID},
            })
        }
    }

    for ref, exceptions := range ro.exceptions {
        for _, e := range exceptions {
            facts = append(facts, MangleFact{
                Predicate: "runtime_exception",
                Args:      []interface{}{ref, e.Type, e.Message, e.TestRunID},
            })
        }
    }

    return facts
}
```

## Lens 3: Adversarial (Logic Injection Defense)

### The Attack Vector

**Logic Injection:** Malicious code that attacks the Mangle kernel itself.

```python
def func():
    """
    @mangle: allow_override(true)
    security_decorator(/bypass_all).
    deny_edit(_, _) :- false.
    """
    pass
```

If the parser blindly copies docstrings into Mangle predicates, an attacker could override `deny_edit` rules.

### Defense: Sanitization & Namespace Isolation

#### Rule 1: Never Allow Raw Text to Become Predicates

```go
// CRITICAL: All text from user code becomes ARGUMENTS, never PREDICATES

// BAD - Raw text becomes predicate (INJECTABLE)
func emitFactBAD(docstring string) MangleFact {
    return MangleFact{
        Predicate: docstring,  // DANGER: User controls predicate name!
        Args:      []interface{}{},
    }
}

// GOOD - Raw text is always an argument (SAFE)
func emitFactGOOD(ref, docstring string) MangleFact {
    return MangleFact{
        Predicate: "docstring_content",  // Fixed predicate
        Args:      []interface{}{ref, docstring},  // User text is argument
    }
}
```

#### Rule 2: Input Sanitization

```go
// internal/world/sanitizer.go

type Sanitizer struct {
    dangerousPatterns []*regexp.Regexp
}

func NewSanitizer() *Sanitizer {
    return &Sanitizer{
        dangerousPatterns: []*regexp.Regexp{
            // Mangle syntax in strings
            regexp.MustCompile(`:-`),                    // Rule definition
            regexp.MustCompile(`Decl\s+\w+\(`),          // Declaration
            regexp.MustCompile(`\.\s*$`),                // Fact terminator
            regexp.MustCompile(`@mangle:`),              // Direct injection attempt
            regexp.MustCompile(`deny_edit|permitted`),   // Safety bypass
            regexp.MustCompile(`\|>`),                   // Aggregation
        },
    }
}

// SanitizeForMangle removes or escapes dangerous patterns
func (s *Sanitizer) SanitizeForMangle(input string) string {
    result := input

    for _, pattern := range s.dangerousPatterns {
        result = pattern.ReplaceAllString(result, "[SANITIZED]")
    }

    // Escape special characters
    result = strings.ReplaceAll(result, "\"", "\\\"")
    result = strings.ReplaceAll(result, "\n", "\\n")

    return result
}

// ValidatePredicate ensures predicate names are from allowed set
func (s *Sanitizer) ValidatePredicate(pred string) error {
    allowedPredicates := map[string]bool{
        "code_element":       true,
        "element_signature":  true,
        "element_body":       true,
        "element_parent":     true,
        "docstring_content":  true,
        "py_class":           true,
        "py_decorator":       true,
        // ... all allowed predicates
    }

    if !allowedPredicates[pred] {
        return fmt.Errorf("disallowed predicate: %s", pred)
    }
    return nil
}
```

#### Rule 3: Namespace Isolation

```mangle
# =============================================================================
# NAMESPACE ISOLATION
# User-derived facts cannot affect safety rules
# =============================================================================

# User content is quarantined in user_* namespace
Decl user_docstring(Ref.Type<string>, Content.Type<string>).
Decl user_comment(Ref.Type<string>, Content.Type<string>).

# Safety rules NEVER reference user_* predicates
# This is enforced by schema validation
deny_edit(Ref, Reason) :-
    # ONLY references system predicates, never user_*
    snapshot:py_decorator(Ref, Dec),
    security_decorator(Dec),
    not candidate:py_decorator(Ref, Dec),
    Reason = /security_regression.

# Nemesis validation: Ensure no rule references user_* in body
invalid_rule(RuleName) :-
    rule_definition(RuleName, Body),
    fn:contains(Body, "user_").
```

#### Nemesis Module: DOM Breaker

```go
// internal/shards/nemesis/dom_breaker.go

type DOMBreaker struct {
    parser    *CodeElementParser
    kernel    *Kernel
    sanitizer *Sanitizer
}

func NewDOMBreaker(parser *CodeElementParser, kernel *Kernel) *DOMBreaker {
    return &DOMBreaker{
        parser:    parser,
        kernel:    kernel,
        sanitizer: NewSanitizer(),
    }
}

// GenerateMaliciousFiles creates test files with attack vectors
func (db *DOMBreaker) GenerateMaliciousFiles() []MaliciousFile {
    return []MaliciousFile{
        // Mangle syntax in docstrings
        {
            Name: "mangle_injection.py",
            Content: `
def evil():
    """
    deny_edit(_, _) :- false.
    permitted(Action) :- true.
    """
    pass
`,
            ExpectedBehavior: "Docstring treated as string, not executed",
        },

        // Mixed indentation
        {
            Name: "mixed_indent.py",
            Content: "def foo():\n\treturn 1\n    return 2\n",
            ExpectedBehavior: "Parser error detected, element marked tainted",
        },

        // Decorator on non-existent
        {
            Name: "orphan_decorator.py",
            Content: "@login_required\n# no function here\nx = 1\n",
            ExpectedBehavior: "Decorator not associated with any element",
        },

        // Recursive class definition
        {
            Name: "recursive_class.py",
            Content: `
class A(B):
    pass

class B(A):
    pass
`,
            ExpectedBehavior: "Circular inheritance detected",
        },

        // Unicode attacks
        {
            Name: "unicode_attack.py",
            Content: "def foo\u200b():\n    pass\n",  // Zero-width space in name
            ExpectedBehavior: "Unicode normalized or rejected",
        },

        // Null byte injection
        {
            Name: "null_byte.py",
            Content: "def foo():\n    x = 'a\x00b'\n    pass\n",
            ExpectedBehavior: "Null bytes sanitized",
        },
    }
}

// RunBreakingTests attempts to break the parser with malicious inputs
func (db *DOMBreaker) RunBreakingTests() []BreakingTestResult {
    var results []BreakingTestResult

    for _, mf := range db.GenerateMaliciousFiles() {
        result := BreakingTestResult{
            File:     mf.Name,
            Expected: mf.ExpectedBehavior,
        }

        // Create temp file
        tmpFile := filepath.Join(os.TempDir(), mf.Name)
        os.WriteFile(tmpFile, []byte(mf.Content), 0644)
        defer os.Remove(tmpFile)

        // Attempt to parse
        elements, err := db.parser.ParseFile(tmpFile, []byte(mf.Content))
        if err != nil {
            result.Outcome = "Parse error (expected)"
            result.Passed = true
        } else {
            // Check if any dangerous facts were created
            facts := db.parser.EmitFacts(elements)
            for _, fact := range facts {
                if err := db.sanitizer.ValidatePredicate(fact.Predicate); err != nil {
                    result.Outcome = fmt.Sprintf("SECURITY VIOLATION: %v", err)
                    result.Passed = false
                    break
                }
            }
            if result.Outcome == "" {
                result.Outcome = "Safely handled"
                result.Passed = true
            }
        }

        results = append(results, result)
    }

    return results
}
```

## Lens 4: Resilience (Broken State)

### The Problem

Tree-sitter is tolerant, but Mangle is strict. Partial facts from broken code can lead to incorrect deductions.

**Failure Scenario:**
```python
# User writes broken code
class User:
def login(self):  # IndentationError - not inside class
    pass
```

Tree-sitter returns partial AST. Mangle receives "login is a global function" (wrong!) and allows dangerous global-scope refactoring.

### The "Tainted" Flag

Tag elements from broken parses as tainted. Block all edits to tainted scopes.

#### Mangle Facts

```mangle
# =============================================================================
# TAINT TRACKING FOR PARSE ERRORS
# =============================================================================

Decl parser_error(File.Type<string>, Line.Type<int>, Message.Type<string>).
Decl element_tainted(Ref.Type<string>, Reason.Type<string>).

# Elements in files with parse errors are tainted
element_tainted(Ref, "file_has_syntax_errors") :-
    code_element(Ref, _, File, _, _),
    parser_error(File, _, _).

# Elements from ERROR nodes are directly tainted
element_tainted(Ref, "parsed_from_error_node") :-
    parser_error_node(Ref).

# Taint propagates through parent relationships
element_tainted(ChildRef, "parent_tainted") :-
    element_parent(ChildRef, ParentRef),
    element_tainted(ParentRef, _).

# Block ALL edits to tainted elements
deny_edit(Ref, /syntax_error_in_scope) :-
    element_tainted(Ref, _).

# Provide remediation
remediation(/syntax_error_in_scope, "Fix the syntax error first, then retry the edit.").
```

#### Parser Integration

```go
// internal/world/python_parser.go (extended)

func (p *PythonParser) Parse(path string, content []byte) ([]CodeElement, []ParseError, error) {
    tree, err := p.parser.ParseCtx(context.Background(), nil, content)
    if err != nil {
        return nil, nil, err
    }
    defer tree.Close()

    var elements []CodeElement
    var errors []ParseError
    taintedLines := make(map[int]bool)

    root := tree.RootNode()

    // First pass: collect all ERROR nodes
    p.collectErrorNodes(root, path, content, &errors, taintedLines)

    // Second pass: extract elements, marking tainted ones
    p.walkNode(root, path, "", &elements, content, taintedLines)

    return elements, errors, nil
}

func (p *PythonParser) collectErrorNodes(node *sitter.Node, path string, content []byte, errors *[]ParseError, tainted map[int]bool) {
    if node.Type() == "ERROR" || node.IsMissing() {
        startLine := int(node.StartPoint().Row) + 1
        endLine := int(node.EndPoint().Row) + 1

        *errors = append(*errors, ParseError{
            File:    path,
            Line:    startLine,
            Column:  int(node.StartPoint().Column) + 1,
            Message: p.errorMessage(node, content),
        })

        // Mark all lines in error range as tainted
        for line := startLine; line <= endLine; line++ {
            tainted[line] = true
        }
    }

    for i := 0; i < int(node.ChildCount()); i++ {
        p.collectErrorNodes(node.Child(i), path, content, errors, tainted)
    }
}

func (p *PythonParser) walkNode(node *sitter.Node, path, parentRef string, elements *[]CodeElement, content []byte, taintedLines map[int]bool) {
    // Skip ERROR nodes for element extraction
    if node.Type() == "ERROR" || node.IsMissing() {
        return
    }

    elem := p.nodeToElement(node, path, parentRef, content)
    if elem != nil {
        // Check if any line in element range is tainted
        for line := elem.StartLine; line <= elem.EndLine; line++ {
            if taintedLines[line] {
                elem.Metadata["tainted"] = true
                elem.Metadata["taint_reason"] = "overlaps_with_error_node"
                break
            }
        }

        *elements = append(*elements, *elem)

        if elem.Type == ElementStruct {
            p.walkNode(node, path, elem.Ref, elements, content, taintedLines)
        }
    }

    // Recurse into children
    for i := 0; i < int(node.ChildCount()); i++ {
        p.walkNode(node.Child(i), path, parentRef, elements, content, taintedLines)
    }
}

// EmitLanguageFacts includes taint facts
func (p *PythonParser) EmitLanguageFacts(elements []CodeElement) []MangleFact {
    var facts []MangleFact

    for _, elem := range elements {
        // ... normal facts ...

        // Emit taint facts
        if tainted, ok := elem.Metadata["tainted"].(bool); ok && tainted {
            reason := elem.Metadata["taint_reason"].(string)
            facts = append(facts, MangleFact{
                Predicate: "element_tainted",
                Args:      []interface{}{elem.Ref, reason},
            })
        }
    }

    return facts
}
```

#### Workflow: Fix Before Refactor

```text
1. Agent attempts: edit_element("py:auth.py:login", new_body)

2. Constitutional Gate queries: deny_edit("py:auth.py:login", Reason)
   → Returns: /syntax_error_in_scope

3. Agent receives block with remediation:
   "Cannot edit py:auth.py:login - syntax error in scope.
    Fix the syntax error first, then retry the edit.
    Errors:
    - Line 3: Unexpected indent (missing colon after class definition)"

4. Agent fixes syntax error first:
   edit_lines("py:auth.py", 1, "class User:\n")

5. Parser re-parses, taint cleared
6. Original edit now permitted
```

## Summary: Hardening Checklist

| Lens | Enhancement | Implementation |
|------|-------------|----------------|
| Temporal | Git binding | `last_modified`, `previous_body` facts |
| Temporal | Session change log | `agent_modified` for surgical revert |
| Runtime | Value overlay | `runtime_value`, `runtime_exception` facts |
| Runtime | Trace parser | Parse Python/Go tracebacks to Refs |
| Adversarial | Input sanitization | Remove Mangle syntax from user strings |
| Adversarial | Namespace isolation | `user_*` namespace quarantine |
| Adversarial | Nemesis DOM Breaker | Fuzz parser with malicious inputs |
| Resilience | Taint flag | `element_tainted` for ERROR nodes |
| Resilience | Edit blocking | `deny_edit(/syntax_error_in_scope)` |
| Resilience | Fix-first workflow | Remediation guidance |

## Mangle Schema Additions

```mangle
# =============================================================================
# HARDENING SCHEMA DECLARATIONS
# =============================================================================

# Temporal
Decl last_modified(Ref.Type<string>, CommitHash.Type<string>, Author.Type<string>, Timestamp.Type<int>).
Decl previous_body(Ref.Type<string>, CommitHash.Type<string>, SourceCode.Type<string>).
Decl agent_modified(Ref.Type<string>, TurnNumber.Type<int>, OldBody.Type<string>, NewBody.Type<string>).

# Runtime
Decl runtime_value(Ref.Type<string>, VarName.Type<string>, Value.Type<string>, TestRunID.Type<string>).
Decl runtime_exception(Ref.Type<string>, ExceptionType.Type<string>, Message.Type<string>, TestRunID.Type<string>).

# Adversarial
Decl user_docstring(Ref.Type<string>, Content.Type<string>).
Decl user_comment(Ref.Type<string>, Content.Type<string>).

# Resilience
Decl parser_error(File.Type<string>, Line.Type<int>, Message.Type<string>).
Decl element_tainted(Ref.Type<string>, Reason.Type<string>).
Decl parser_error_node(Ref.Type<string>).
```
