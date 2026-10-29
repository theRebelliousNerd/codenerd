package mcp

// Standard-shaped wire fixtures. They are written from the MCP specification
// (2024-11-05 lifecycle and tools/call result shapes), not from this package's
// DTOs, so a decoder that agrees only with its own types cannot pass them.

// standardInitializeResult is a conforming initialize result: the negotiated
// version and an object-valued capability for each name given.
func standardInitializeResult(capabilities ...string) map[string]any {
	declared := map[string]any{}
	for _, name := range capabilities {
		declared[name] = map[string]any{}
	}
	return map[string]any{
		"protocolVersion": protocolVersionLegacyInit,
		"capabilities":    declared,
		"serverInfo":      map[string]any{"name": "fixture", "version": "1.0"},
	}
}

// standardToolResult is a conforming tools/call result carrying one text block.
func standardToolResult(text string, isError bool) map[string]any {
	result := map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}
	if isError {
		result["isError"] = true
	}
	return result
}
