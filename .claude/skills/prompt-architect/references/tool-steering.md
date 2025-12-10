# Tool Steering & Deterministic Selection

In codeNERD, tool use is **Deterministic**, not improvised. The LLM does *not* decide which tool to call; it *describes an intent*, and the Kernel *grants* the most appropriate tool based on Mangle predicates.

This document guides you in designing tools that the Kernel can steer effectively.

## 1. The Philosophy of Determinism

Standard Agents: "I think I'll write a Python script to do this." (Hallucination risk: High)
codeNERD Agents: "My intent is `/mutation` on `target`. Constraint: `safety_checked`." -> Kernel resolves to `tool:refactor_safe`.

We achieve this via **Semantic Binding**: Tools are bound to logic predicates, not just fuzzy descriptions.

## 2. Tool Definition Anatomy

Tools are defined in `internal/core/tool_registry.go`. A "God Tier" tool definition requires precision in four fields:

```go
type Tool struct {
    Name          string    // Unique ID (e.g., "grep_search")
    Command       string    // Execution path
    ShardAffinity string    // Who can use this? (e.g., "/researcher", "/all")
    Description   string    // The Semantic Anchor
    Capabilities  []string  // Logic Tags (e.g., "/search", "/file_io")
}
```

### 2.1. The Name (The Handle)

- **Bad**: `do_search`, `fix_it`, `script_1`
- **Good**: `grep_search`, `ast_rewrite`, `dependency_graph`
- **Rule**: Use `noun_verb` or `system_action` format.

### 2.2. The Description (The Anchor)

This is the most critical field for the Hybrid Search (Vector + Keyword) often used in Spreading Activation.

- **Pattern**: `[Action] [Target] using [Mechanism]. Use when [Condition].`
- **Example**: "Search for string patterns in file content using ripgrep. Use when you need to find specific code snippets or TODOs across the codebase."

### 2.3. Shard Affinity (The RBAC)

Limit tools to the specialists that understand them.

- `/coder`: Can use formatting, refactoring, code-mod tools.
- `/researcher`: Can use web search, `llms.txt` parsers.
- `/system`: Can use `legislator`, `process_manager`.
- `/all`: Generic utilities (`cat`, `ls`).

### 2.4. Capabilities (The Logic Tags)

These map directly to Mangle atoms `tool_capability(Tool, Cap)`.

- **Common Tags**: `/search`, `/io`, `/network`, `/ast`, `/debug`.
- **Logic**: The Kernel uses these to prune the search space. If Intent is `/mutation`, tools with only `/query` capability are hard-filtered.

## 3. Mangle Integration

When a tool is registered, it projects these facts into the Kernel:

```mangle
registered_tool("grep_search", "/usr/bin/rg", "/all").
tool_capability("grep_search", "/search").
tool_capability("grep_search", "/read").
```

### Deterministic Routing Logic

The `tactile_router` uses these rules to bind Intent to Tool:

```mangle
# Select tools that match the Intent Verb's required capability
candidate_tool(Tool) :-
    user_intent(_, Verb, _, _),
    verb_requires(Verb, Cap),
    tool_capability(Tool, Cap).
```

## 4. Writing Effective Tool Explanations in Prompts

Do not dump a raw list of tools. Use the **Action-Enablement** pattern.

**Weak Pattern (Context Starvation)**:

```text
Tools: grep, ls, cat, sed.
```

**God Tier Pattern (Steering)**:

```text
AVAILABLE TOOLS (Selected by Kernel):
- grep_search: Locates code patterns. Use for 'Find X'.
- file_read: Reads file content. Use for 'Show me X'.

You MUST use one of these tools. Do not invent tools.
```

## 5. Autopoiesis: Creating New Tools

When codeNERD creates a tool for itself (Type: `self_tool`), it must auto-generate these definitions.

**The Ouroboros Mandate**:

1. **Generate Code**: Create the Go binary.
2. **Generate Definition**: Create the `Tool` struct with inferred capabilities.
3. **Register**: Project `registered_tool` fact.
4. **Use**: The new tool is now available for selection.
