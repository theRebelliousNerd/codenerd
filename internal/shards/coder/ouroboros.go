package coder

import (
	"context"
	"fmt"

	"codenerd/internal/logging"
)

// =============================================================================
// OUROBOROS ROUTING - Self-Tool Generation
// =============================================================================
// Routes self-tool artifacts through the Ouroboros pipeline instead of
// writing them directly to the filesystem. This ensures all tools codeNERD
// creates for itself go through safety checking and proper compilation.

// routeToOuroboros sends a self-tool artifact through the Ouroboros pipeline.
// This is called when artifact_type is "self_tool" or "diagnostic".
func (c *CoderShard) routeToOuroboros(ctx context.Context, result *CoderResult) (string, error) {
	logging.Coder("Routing self-tool to Ouroboros: name=%s, type=%s", result.ToolName, result.ArtifactType)

	// Validate we have code to generate
	if len(result.Edits) == 0 {
		return "", fmt.Errorf("no code generated for self-tool")
	}

	// Get the first edit's content as the tool code
	edit := result.Edits[0]
	if edit.NewContent == "" {
		return "", fmt.Errorf("empty code content for self-tool")
	}

	// Determine tool name
	toolName := result.ToolName
	if toolName == "" {
		toolName = extractToolName(edit.File)
	}
	if toolName == "" {
		return "", fmt.Errorf("could not determine tool name for self-tool")
	}

	// Check if we have a tool generator via VirtualStore
	if c.virtualStore == nil {
		logging.Get(logging.CategoryCoder).Warn("No VirtualStore available, falling back to direct write")
		return c.fallbackDirectWrite(ctx, result)
	}

	toolGenerator := c.virtualStore.GetToolGenerator()
	if toolGenerator == nil {
		logging.Get(logging.CategoryCoder).Warn("No ToolGenerator available, falling back to direct write")
		return c.fallbackDirectWrite(ctx, result)
	}

	// Prepare parameters for tool generation
	purpose := edit.Rationale
	code := edit.NewContent
	confidence := 0.8 // Default confidence for LLM-generated code
	priority := 1.0
	isDiagnostic := result.ArtifactType == ArtifactTypeDiagnostic

	logging.Coder("Submitting to Ouroboros: tool=%s, code_len=%d", toolName, len(code))

	// Submit to Ouroboros
	success, genToolName, binaryPath, errMsg := toolGenerator.GenerateToolFromCode(
		ctx, toolName, purpose, code, confidence, priority, isDiagnostic,
	)

	if !success {
		logging.Get(logging.CategoryCoder).Error("Ouroboros rejected tool: %s", errMsg)
		return "", fmt.Errorf("ouroboros rejected tool: %s", errMsg)
	}

	logging.Coder("Ouroboros generation successful: tool=%s, binary=%s", genToolName, binaryPath)

	// Track acceptance
	c.trackAcceptance("self_tool_generation")

	// Build response
	response := fmt.Sprintf("✓ Self-tool '%s' generated successfully via Ouroboros\n"+
		"  Binary: %s\n"+
		"  The tool is now available for execution.",
		genToolName, binaryPath)

	return response, nil
}

// fallbackDirectWrite writes the tool directly if Ouroboros is not available.
// This is a degraded mode - tools won't go through safety checking.
func (c *CoderShard) fallbackDirectWrite(ctx context.Context, result *CoderResult) (string, error) {
	logging.Get(logging.CategoryCoder).Warn("FALLBACK: Writing self-tool directly without Ouroboros safety checks")

	// Apply edits normally but warn about the safety bypass
	if err := c.applyEdits(ctx, result.Edits); err != nil {
		return "", fmt.Errorf("fallback direct write failed: %w", err)
	}

	response := fmt.Sprintf("⚠ Self-tool written directly (Ouroboros unavailable)\n"+
		"  File: %s\n"+
		"  WARNING: Tool did not go through safety checking or compilation.\n"+
		"  Consider running 'nerd tool compile %s' manually.",
		result.Edits[0].File, result.ToolName)

	return response, nil
}
