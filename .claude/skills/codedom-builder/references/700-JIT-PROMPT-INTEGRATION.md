# 700: JIT Prompt Integration

## The Prompt Blindness Problem

Building a "Ferrari engine" (Polyglot DOM + Mangle Logic) is useless if we hand the LLM an instruction manual for a "Corolla" (basic text editor). The LLM must undergo a **Paradigm Shift**:

| Old Mental Model | New Mental Model |
|-----------------|------------------|
| "I am a Text Editor" | "I am a Graph Architect" |
| "I read files and change lines" | "I manipulate abstract symbols spanning files/languages" |
| "Refs are just file paths" | "Refs are stable symbol identifiers" |
| "grep -r to find things" | "Query the graph for impact" |

## Required Prompt Atoms

The following atoms must be created in `internal/prompt/atoms/` to enable the paradigm shift.

### 1. Core Concept: The Semantic Graph

**File:** `internal/prompt/atoms/methodology/semantic_graph.yaml`

This injects the fundamental "physics" of the new world - Refs are the primary interface, not filenames.

```yaml
# Methodology: The Semantic Code Graph
# Explains the "X-Ray Vision" capability provided by the Code DOM architecture.

- id: "methodology/semantic_graph"
  category: "methodology"
  subcategory: "core"
  priority: 90
  is_mandatory: true
  shard_types: ["/coder", "/planner"]
  intent_verbs: ["/fix", "/refactor", "/implement", "/rename"]
  content: |
    ## OPERATIONAL PARADIGM: THE SEMANTIC CODE GRAPH

    You are not just editing text files; you are manipulating a **Unified Abstract
    Semantic Graph (UASG)**. The system perceives code as interconnected "Atoms"
    (Symbols) rather than isolated strings.

    ### THE "REF" IDENTIFIER (YOUR PRIMARY HANDLE)

    Every function, class, and method has a stable **Uniform Resource Identifier (Ref)**.
    You MUST use these Refs to locate and manipulate code surgically.

    **Ref Formats:**
    - **Python:** `py:path/to/file.py:ClassName.method_name`
    - **Go:** `go:path/to/file.go:Package.Struct.Method`
    - **TypeScript:** `ts:path/to/file.ts:Interface.Property`
    - **Rust:** `rs:path/to/file.rs:Struct::method`
    - **Kotlin:** `kt:path/to/file.kt:Class.method`

    ### CROSS-LANGUAGE ENTANGLEMENT

    The Graph detects invisible links between languages via "Wire Names":
    - Go struct field `json:"user_id"` links to TypeScript interface `userId`
    - Python Pydantic `alias="user_id"` links to the same wire name
    - Kotlin `@SerializedName("user_id")` links to the same wire name

    **Instruction:** Do not perform partial refactors. Query the `api_dependency`
    facts to find the *entire* blast radius of a change before applying it.

    ### AVAILABLE QUERIES

    Before editing, query the graph:
    ```
    # Find all elements with a specific wire name
    wire_name(Ref, "user_id")

    # Find API dependencies (what breaks if I change this?)
    api_dependency(BackendRef, FrontendRef)

    # Check if an element is a data contract
    is_data_contract(Ref)

    # Check for safety violations before edit
    deny_edit(Ref, Reason)
    ```
```

### 2. Impact Simulation Protocol

**File:** `internal/prompt/atoms/shards/coder/cognitive_protocol_codedom.yaml`

Patches the cognitive protocol to use Mangle for impact prediction.

```yaml
# Coder Shard: Semantic Impact Simulation
# Replaces vague "mentally simulate" with specific Mangle queries

- id: "shards/coder/cognitive_protocol_codedom"
  category: "shards"
  subcategory: "coder"
  priority: 85
  is_mandatory: false
  shard_types: ["/coder"]
  intent_verbs: ["/fix", "/refactor", "/rename"]
  content: |
    ## PHASE 3: SEMANTIC IMPACT SIMULATION (MANGLE-POWERED)

    Before writing code, query the Logic Kernel to predict the future:

    ### 1. Check Regressions
    "If I remove this decorator, will the `deny_edit(/security_regression)` rule fire?"
    ```
    Query: deny_edit("py:auth.py:login", Reason)
    ```

    ### 2. Check Dependencies
    "Which dependent_refs rely on this Ref?" (Do NOT guess; ASK the graph)
    ```
    Query: impacted_by_change("go:user.go:User", Impacted)
    ```

    ### 3. Check Contracts
    "Am I breaking a 'Data Contract' shared between Python and Go?"
    ```
    Query: api_dependency("go:user.go:User.ID", FrontendRef)
    ```

    ### 4. Check Test Impact
    "Which tests need to run after this change?"
    ```
    Query: impacted_test_file(TestFile)
    ```

    ### RULE: Never Proceed Without Graph Confirmation

    If the graph query reveals:
    - More than 5 impacted files → Request explicit user confirmation
    - Any `deny_edit` rule firing → Stop and explain the block
    - Breaking API changes → Plan atomic cross-file update
```

### 3. Tool Steering for Semantic Navigation

**File:** `internal/prompt/atoms/shards/coder/tool_steering_codedom.yaml`

Explicitly forbids "dumb" regex searches in favor of semantic lookups.

```yaml
# Coder Shard: Semantic Tool Steering
# Teaches when to use CodeDOM tools vs raw file access

- id: "shards/coder/tool_steering_codedom"
  category: "shards"
  subcategory: "coder"
  priority: 80
  is_mandatory: false
  shard_types: ["/coder"]
  intent_verbs: ["/fix", "/refactor", "/implement"]
  content: |
    ## SEMANTIC NAVIGATION (PREFERRED)

    **STOP** reading entire files to find one function.
    **USE** semantic tools for surgical precision:

    | Task | Wrong Tool | Right Tool |
    |------|------------|------------|
    | Find a function | `grep -r "def foo"` | `get_element("py:file.py:foo")` |
    | Modify a method | `read_file` + regex | `edit_element(ref, new_body)` |
    | Find callers | `grep` for function name | `query: calls(Caller, ref)` |
    | Rename symbol | Find/replace in files | `rename_symbol(ref, new_name)` |

    ### SEMANTIC TOOLS

    | Tool | Purpose | Example |
    |------|---------|---------|
    | `get_elements(file)` | List all symbols in file | Discover available functions |
    | `get_element(ref)` | View specific symbol | Inspect function body |
    | `edit_element(ref, body)` | Replace symbol body | Modify implementation |
    | `rename_symbol(ref, name)` | Cross-file rename | Propagates via wire_name |
    | `delete_element(ref)` | Remove symbol | Handles dependent cleanup |
    | `insert_element(after, body)` | Add new symbol | Correct indentation |

    ### WHEN TO USE "RAW" FILE ACCESS

    Only use `read_file` or regex-based tools if:
    1. The file type is not supported by the DOM (`.yaml`, `.json`, `.md`, `.toml`)
    2. You are debugging a parser failure
    3. You are creating a completely new file from scratch
    4. The file has syntax errors preventing DOM parsing

    ### ANTI-PATTERNS (WILL BE BLOCKED)

    The Constitutional Gate will block:
    - ❌ `sed` commands that modify code files
    - ❌ Regex replacements without semantic verification
    - ❌ Partial refactors that break API contracts
```

### 4. Exemplar: Polyglot Refactor

**File:** `internal/prompt/atoms/exemplar/polyglot_refactor.yaml`

Concrete example of solving a SWE-bench-style problem using the new system.

```yaml
# Exemplar: Handling a Multi-Language Refactor
# Demonstrates how to use Mangle to safely rename a field across the stack.

- id: "exemplar/polyglot_refactor"
  category: "exemplar"
  subcategory: "refactor"
  priority: 50
  is_mandatory: false
  shard_types: ["/coder"]
  intent_verbs: ["/refactor", "/rename"]
  content: |
    ## EXEMPLAR: CROSS-LANGUAGE FIELD RENAME

    **User Intent:** "Rename the 'email' field to 'primary_email' in the User model."

    ### BAD Approach (Text Editor Mental Model)

    1. `grep -r "email" .` → Returns 5000 hits, noise
    2. Manually open 10 files
    3. String replace "email" → "primary_email"
    4. **Result:** Accidentally renames `send_email` function. Broken build.

    ### GOOD Approach (Graph Architect Mental Model)

    **Step 1: Identify Source**
    ```
    get_element("py:backend/models.py:User")
    → Returns User class with field "email"
    ```

    **Step 2: Query Wire Name Dependencies**
    ```
    Query: wire_name(Ref, "email")
    → Returns:
      - py:backend/models.py:User.email (Pydantic alias)
      - go:api/types.go:UserStruct.Email (json tag)
      - ts:frontend/types.ts:IUser.email (interface prop)
    ```

    **Step 3: Query Impact**
    ```
    Query: api_dependency("py:backend/models.py:User.email", Frontend)
    → Returns: [ts:frontend/types.ts:IUser.email]
    ```

    **Step 4: Check Safety**
    ```
    Query: deny_edit("py:backend/models.py:User.email", Reason)
    → Returns: [] (no blocks)
    ```

    **Step 5: Plan Atomic Change**
    ```json
    {
      "transaction": "rename_wire_name",
      "old_name": "email",
      "new_name": "primary_email",
      "targets": [
        {"ref": "py:backend/models.py:User.email", "action": "update_alias"},
        {"ref": "go:api/types.go:UserStruct.Email", "action": "update_json_tag"},
        {"ref": "ts:frontend/types.ts:IUser.email", "action": "rename_property"}
      ]
    }
    ```

    **Step 6: Execute in Transaction**
    - Begin 2PC transaction
    - Apply all 3 edits to shadow filesystem
    - Validate with Mangle (no deny_edit fires)
    - Commit atomically

    **Step 7: Run Impacted Tests**
    ```
    Query: impacted_test_file(TestFile)
    → Returns: [test_user.py, user_test.go, user.spec.ts]
    ```
    Run only these 3 test files, not the full suite.

    ### KEY INSIGHT

    The "Graph Architect" approach:
    - Finds exactly 3 relevant locations (not 5000 grep hits)
    - Verifies safety before editing
    - Applies changes atomically (no split brain)
    - Runs minimal test set (not full suite)
```

### 5. Safety Awareness Atom

**File:** `internal/prompt/atoms/safety/codedom_guardrails.yaml`

Teaches the LLM about the safety rules that will block its edits.

```yaml
# Safety: CodeDOM Guardrails Awareness
# Teaches the LLM what deny_edit rules exist so it can avoid violations

- id: "safety/codedom_guardrails"
  category: "safety"
  subcategory: "codedom"
  priority: 95
  is_mandatory: true
  shard_types: ["/coder"]
  intent_verbs: ["/fix", "/refactor", "/implement"]
  content: |
    ## CODEDOM SAFETY GUARDRAILS

    The Constitutional Gate will BLOCK edits that trigger these rules.
    You MUST check for violations BEFORE attempting an edit.

    ### Python Violations

    | Rule | Trigger | How to Avoid |
    |------|---------|--------------|
    | `/security_regression` | Removing @login_required | Keep security decorators |
    | `/type_hint_regression` | Removing return type hints | Preserve type annotations |
    | `/unprotected_route` | Route without auth decorator | Add authentication |

    ### Go Violations

    | Rule | Trigger | How to Avoid |
    |------|---------|--------------|
    | `/goroutine_leak` | go func() without sync | Add WaitGroup or channel |
    | `/goroutine_no_context` | Goroutine without ctx | Pass context.Context |
    | `/error_handling_regression` | Reducing error checks | Keep all error handling |
    | `/ignored_error` | `_, _ := fn()` | Handle errors: `if err != nil` |
    | `/lock_without_defer` | Lock() without defer Unlock() | Always `defer mu.Unlock()` |

    ### TypeScript/React Violations

    | Rule | Trigger | How to Avoid |
    |------|---------|--------------|
    | `/react_stale_closure` | useEffect missing deps | Add to dependency array |
    | `/type_weakened_to_any` | Changing type to `any` | Use specific types |
    | `/missing_error_handling` | fetch without catch | Add error handling |

    ### Universal Violations

    | Rule | Trigger | How to Avoid |
    |------|---------|--------------|
    | `/hardcoded_credential` | API keys in code | Use environment variables |
    | `/breaking_api_change` | Changing wire name | Update all API consumers |
    | `/test_coverage_regression` | Removing tests | Keep or replace tests |

    ### Pre-Edit Safety Check

    Before every edit, run:
    ```
    Query: deny_edit("your:target:ref", Reason)
    ```

    If any Reason is returned, you MUST:
    1. Explain the violation to the user
    2. Propose an alternative approach
    3. Do NOT attempt to bypass the safety rule
```

## Integration with JIT Compiler

These atoms must be registered in the JIT Prompt Compiler:

```go
// internal/prompt/compiler.go

func (c *Compiler) registerCodeDOMAtoms() {
    // Register methodology atoms
    c.RegisterAtomFile("internal/prompt/atoms/methodology/semantic_graph.yaml")

    // Register coder shard atoms
    c.RegisterAtomFile("internal/prompt/atoms/shards/coder/cognitive_protocol_codedom.yaml")
    c.RegisterAtomFile("internal/prompt/atoms/shards/coder/tool_steering_codedom.yaml")

    // Register exemplars
    c.RegisterAtomFile("internal/prompt/atoms/exemplar/polyglot_refactor.yaml")

    // Register safety awareness
    c.RegisterAtomFile("internal/prompt/atoms/safety/codedom_guardrails.yaml")
}
```

## ConfigFactory Tool Mapping

Update `ConfigFactory` to include CodeDOM tools for relevant intents:

```go
// internal/prompt/config_factory.go

func (p *ConfigAtomProvider) registerCodeDOMTools() {
    codedomTools := []string{
        "get_elements",
        "get_element",
        "edit_element",
        "rename_symbol",
        "delete_element",
        "insert_element",
        "query_wire_names",
        "get_api_dependencies",
        "check_safety",
    }

    // Add to refactor intents
    p.atoms["/refactor"].Tools = append(p.atoms["/refactor"].Tools, codedomTools...)
    p.atoms["/rename"].Tools = append(p.atoms["/rename"].Tools, codedomTools...)

    // Add query-only tools to /fix (can query but maybe not full refactor)
    p.atoms["/fix"].Tools = append(p.atoms["/fix"].Tools,
        "get_elements", "get_element", "check_safety")
}
```

## Validation: Is the LLM Using Semantic Mode?

Add observability to detect if the LLM is still using "text editor" patterns:

```go
// internal/transparency/semantic_mode_detector.go

type SemanticModeDetector struct {
    textEditorPatterns []string
}

func NewSemanticModeDetector() *SemanticModeDetector {
    return &SemanticModeDetector{
        textEditorPatterns: []string{
            `grep -r`,
            `find . -name`,
            `sed -i`,
            `awk '{`,
            `read_file.*\n.*edit_file`,  // Read then edit without get_element
        },
    }
}

func (d *SemanticModeDetector) Analyze(response string) *SemanticModeReport {
    report := &SemanticModeReport{
        UsingSemanticMode: true,
        TextEditorHits:    []string{},
    }

    for _, pattern := range d.textEditorPatterns {
        if regexp.MustCompile(pattern).MatchString(response) {
            report.UsingSemanticMode = false
            report.TextEditorHits = append(report.TextEditorHits, pattern)
        }
    }

    return report
}

// If text editor mode detected, inject reminder atom
func (d *SemanticModeDetector) GetReminder() string {
    return `
REMINDER: You have Semantic Code Graph capabilities.
Instead of grep/sed, use get_element(ref) and edit_element(ref, body).
Query the graph for dependencies before making changes.
`
}
```

## Measuring Success

Track adoption of semantic mode vs text editor mode:

```mangle
# =============================================================================
# SEMANTIC MODE ADOPTION METRICS
# =============================================================================

Decl tool_invocation(SessionID.Type<string>, Tool.Type<string>, Timestamp.Type<int>).

# Semantic tools
is_semantic_tool(/get_element).
is_semantic_tool(/get_elements).
is_semantic_tool(/edit_element).
is_semantic_tool(/rename_symbol).
is_semantic_tool(/query_wire_names).

# Text editor tools
is_text_editor_tool(/read_file).
is_text_editor_tool(/grep).
is_text_editor_tool(/sed).

# Calculate semantic mode ratio
semantic_tool_count(Session, Count) :-
    tool_invocation(Session, Tool, _),
    is_semantic_tool(Tool) |>
    do fn:group_by(Session),
    let Count = fn:count().

text_editor_tool_count(Session, Count) :-
    tool_invocation(Session, Tool, _),
    is_text_editor_tool(Tool) |>
    do fn:group_by(Session),
    let Count = fn:count().

# Sessions using primarily semantic mode (>70% semantic tools)
semantic_mode_session(Session) :-
    semantic_tool_count(Session, Semantic),
    text_editor_tool_count(Session, TextEditor),
    Total = fn:plus(Semantic, TextEditor),
    Ratio = fn:divide(Semantic, Total),
    Ratio > 0.7.
```
