# Prompt Audit Checklist

Use this checklist when creating new shards or reviewing prompt changes.

## 1. Structural Integrity (The Piggyback Check)

- [ ] **Protocol Compliance**: Does the prompt explicitly demand `control_packet` BEFORE `surface_response`?
- [ ] **JSON Robustness**: Is the JSON schema clearly defined? Does it handle potential markdown wrapping?
- [ ] **Artifact Classification**: Does the prompt require `artifact_type` (project_code/self_tool/diagnostic)?
- [ ] **Reasoning Trace**: Is the `ReasoningTraceDirective` included/appended?

## 2. Semantic Effectiveness (The Smart Check)

- [ ] **Context Injection**: Does the prompt call `buildSessionContextPrompt()`?
- [ ] **Context Explanation**: Does the prompt explain *why* context is being provided (e.g., "Active Diagnostics")?
- [ ] **Persona Definition**: Is the role clearly defined (e.g., "You are an Expert Go Reviewer")?
- [ ] **Output Constraints**: Are negative constraints active? (e.g., "Do not use non-standard libs")?

## 3. Tool Steering (The Determinism Check)

- [ ] **Closed Set**: Does the prompt list tools as a "Closed Set" (Selected by Kernel)?
- [ ] **Improvisation Ban**: Is there explicit language forbidding tool invention?
- [ ] **Descriptions**: Are tool descriptions action-oriented? (e.g., "Use this to search...")?

## 4. Safety & Constitution (The Law Check)

- [ ] **Safety Gate**: Does the prompt acknowledge that the Kernel may block actions?
- [ ] **Fallback**: Does it tell the model what to do if an action is blocked? (e.g., "Explain to user").

## 5. Automated Validation

Run the `audit_prompts.py` script to check for these programmatically.

```bash
python .claude/skills/prompt-architect/scripts/audit_prompts.py
```
