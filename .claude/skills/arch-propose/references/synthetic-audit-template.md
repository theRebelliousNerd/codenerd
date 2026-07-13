# Synthetic Architecture Audit: <feature>

**Mode:** greenfield | expand
**Status:** Pre-Implementation
**Evidence date:** YYYY-MM-DD

## 1. Verified adjacent current state

| Claim | File or symbol evidence | Confidence |
|---|---|---|

## 2. Feature code currently present

State exactly what exists. If none exists, write: "No feature implementation was
found in the inspected paths."

## 3. Reuse opportunities

| Existing mechanism | How the proposal can extend it | Constraints |
|---|---|---|

## 4. Required integration surfaces

- Mangle declarations and policy
- kernel/VirtualStore action path
- JIT prompt atoms and compiler selection
- session and shard lifecycle
- persistence and memory
- CLI/MCP/A2A/tool exposure
- observability and recovery
- tests and validation

Mark non-applicable rows with a reason.

## 5. Candidate contract summary

Summarize the selected data model, predicate contracts, state ownership,
permissions, lifecycle, and failure behavior.

## 6. Assumptions and unknowns

| Item | Type | Resolution gate |
|---|---|---|

## 7. Expand-mode diff

| Existing file | Keep / revise / replace | Reason |
|---|---|---|

## 8. Writer handoff

List exact files, ownership, required citations, and statements that must remain
verbatim for consistency.

