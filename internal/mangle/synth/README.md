# MangleSynth Tool Design (Draft)

Goal: Provide a single, schema-driven path for Mangle generation so LLMs never emit raw Mangle.
The tool accepts structured input, compiles to Mangle via native parser/AST, and returns validated output
with diagnostics for repair loops.

## Primary Use Cases

- Legislator and mangle_repair shard rule synthesis
- Autopoiesis rule generation and patching
- Any agent that needs to emit valid Mangle on first attempt

## Input Contract (Structured, No NL)

Use the existing `mangle_synth_v1` JSON schema as the tool input. This keeps the LLM in a
strictly structured mode and maps directly to the synth compiler.

Example (single clause):

```json
{
  "format": "mangle_synth_v1",
  "program": {
    "clauses": [
      {
        "head": { "pred": "next_action", "args": [ { "kind": "name", "value": "/run_tests" } ] },
        "body": [
          { "kind": "atom", "atom": { "pred": "test_state", "args": [ { "kind": "name", "value": "/failing" } ] } }
        ]
      }
    ]
  }
}
```

## Output Contract

Return a tool response with:

- `mangle`: compiled Mangle code string
- `clauses`: count
- `diagnostics`: structured errors/warnings (parse, safety, schema)
- `ok`: boolean success flag

## Pipeline (Native Parser + Validation)

1. **Schema validation**: Validate against `mangle_synth_v1` JSON schema.
2. **Compile**: Convert to AST using existing `internal/mangle/synth` compiler.
3. **Render**: Emit Mangle text.
4. **Native parse check**: `parse.Unit(bytes.NewReader([]byte(mangle)))`.
5. **Schema + safety checks**: Run the same validators used by `mangle_repair`.
6. **Return**: If invalid, return diagnostics for next repair attempt.

## Tool Interface (VirtualStore)

Proposed tool name: `mangle_synth_tool`

Input:
- `mangle_synth_v1` JSON (full ProgramSpec)

Output:
```json
{
  "ok": true,
  "mangle": "next_action(/run_tests).",
  "clauses": 1,
  "diagnostics": []
}
```

## Integration Plan

1. **Replace raw LLM output** in legislator/mangle_repair with tool calls.
2. **Keep schema enforcement** for the tool input (LLM outputs only structured JSON).
3. **Use tool diagnostics** to drive retries with targeted error feedback.

## Notes

- This aligns with the existing `mangle_synth_v1` schema and synth compiler.
- The parser check ensures syntax is correct even if JSON compiles.
- Diagnostics should be consistent with the mangle feedback system for reuse.
