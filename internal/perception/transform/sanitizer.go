package transform

// Cross-Model Metadata Sanitization
//
// Fixes: "Invalid `signature` in `thinking` block" error when switching models mid-session.
//
// Root cause: Gemini stores thoughtSignature in metadata.google, Claude stores signature
// in top-level thinking blocks. Foreign signatures fail validation on the target model.

// MinSignatureLength is the minimum length for a valid thinking signature
const MinSignatureLength = 50

// GeminiSignatureFields are fields to strip when sending to Claude
var GeminiSignatureFields = []string{"thoughtSignature", "thinkingMetadata"}

// ClaudeSignatureFields are fields to strip when sending to Gemini
var ClaudeSignatureFields = []string{"signature"}

// SanitizerOptions configures cross-model sanitization
type SanitizerOptions struct {
	TargetModel                  string
	SourceModel                  string
	PreserveNonSignatureMetadata bool
}

// SanitizationResult holds the result of sanitization
type SanitizationResult struct {
	Modified           bool
	SignaturesStripped int
}

// SanitizeCrossModelPayload strips foreign thinking signatures from conversation history.
// Call this before sending to a different model family than the conversation was started with.
func SanitizeCrossModelPayload(contents []map[string]interface{}, targetModel string) *SanitizationResult {
	result := &SanitizationResult{}
	targetFamily := GetModelFamily(targetModel)

	if targetFamily == ModelFamilyUnknown {
		return result
	}

	for _, content := range contents {
		parts, ok := content["parts"].([]interface{})
		if !ok {
			continue
		}

		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if !ok {
				continue
			}

			stripped := sanitizePart(part, targetFamily)
			result.SignaturesStripped += stripped
		}
	}

	result.Modified = result.SignaturesStripped > 0
	return result
}

// sanitizePart sanitizes a single message part based on target model family
func sanitizePart(part map[string]interface{}, targetFamily ModelFamily) int {
	stripped := 0

	if targetFamily == ModelFamilyClaude {
		// Strip Gemini thinking metadata when sending to Claude
		stripped += stripGeminiThinkingMetadata(part)
	} else if targetFamily == ModelFamilyGemini {
		// Strip Claude thinking fields when sending to Gemini
		stripped += stripClaudeThinkingFields(part)
	}

	return stripped
}

// stripGeminiThinkingMetadata removes Gemini-specific thinking metadata
func stripGeminiThinkingMetadata(part map[string]interface{}) int {
	stripped := 0

	// Remove top-level thoughtSignature
	if _, ok := part["thoughtSignature"]; ok {
		delete(part, "thoughtSignature")
		stripped++
	}

	// Remove top-level thinkingMetadata
	if _, ok := part["thinkingMetadata"]; ok {
		delete(part, "thinkingMetadata")
		stripped++
	}

	// Remove from nested metadata.google
	if metadata, ok := part["metadata"].(map[string]interface{}); ok {
		if google, ok := metadata["google"].(map[string]interface{}); ok {
			for _, field := range GeminiSignatureFields {
				if _, exists := google[field]; exists {
					delete(google, field)
					stripped++
				}
			}

			// Clean up empty google object
			if len(google) == 0 {
				delete(metadata, "google")
			}
		}

		// Clean up empty metadata object
		if len(metadata) == 0 {
			delete(part, "metadata")
		}
	}

	return stripped
}

// stripClaudeThinkingFields removes Claude-specific thinking signatures
func stripClaudeThinkingFields(part map[string]interface{}) int {
	stripped := 0

	// Check if this is a thinking block
	partType, _ := part["type"].(string)
	if partType == "thinking" || partType == "redacted_thinking" {
		// Remove signature from thinking blocks
		if sig, ok := part["signature"].(string); ok && len(sig) >= MinSignatureLength {
			delete(part, "signature")
			stripped++
		}
	}

	// Also check for loose signature field (defensive)
	if sig, ok := part["signature"].(string); ok && len(sig) >= MinSignatureLength {
		delete(part, "signature")
		stripped++
	}

	return stripped
}

// IsThinkingPart checks if a message part is a thinking/reasoning block
func IsThinkingPart(part map[string]interface{}) bool {
	if part == nil {
		return false
	}

	// Check thought flag (Gemini format)
	if thought, ok := part["thought"].(bool); ok && thought {
		return true
	}

	// Check type field (Claude format)
	if partType, ok := part["type"].(string); ok {
		return partType == "thinking" || partType == "redacted_thinking" || partType == "reasoning"
	}

	return false
}

// HasValidSignature checks if a thinking part has a valid signature
func HasValidSignature(part map[string]interface{}) bool {
	// Check Gemini-style thoughtSignature
	if sig, ok := part["thoughtSignature"].(string); ok && len(sig) >= MinSignatureLength {
		return true
	}

	// Check Claude-style signature
	if sig, ok := part["signature"].(string); ok && len(sig) >= MinSignatureLength {
		return true
	}

	return false
}

// StripAllThinkingBlocks removes all thinking blocks from contents
// Used before injecting synthetic messages to avoid invalid thinking patterns
func StripAllThinkingBlocks(contents []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(contents))

	for _, content := range contents {
		parts, ok := content["parts"].([]interface{})
		if !ok {
			result = append(result, content)
			continue
		}

		filteredParts := make([]interface{}, 0, len(parts))
		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if !ok {
				filteredParts = append(filteredParts, p)
				continue
			}

			if !IsThinkingPart(part) {
				filteredParts = append(filteredParts, part)
			}
		}

		// Keep at least one part to avoid empty messages
		if len(filteredParts) == 0 && len(parts) > 0 {
			result = append(result, content)
			continue
		}

		newContent := make(map[string]interface{})
		for k, v := range content {
			if k == "parts" {
				newContent[k] = filteredParts
			} else {
				newContent[k] = v
			}
		}
		result = append(result, newContent)
	}

	return result
}

// FilterUnsignedThinkingBlocks removes thinking blocks without valid signatures
func FilterUnsignedThinkingBlocks(contents []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(contents))

	for _, content := range contents {
		parts, ok := content["parts"].([]interface{})
		if !ok {
			result = append(result, content)
			continue
		}

		filteredParts := make([]interface{}, 0, len(parts))
		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if !ok {
				filteredParts = append(filteredParts, p)
				continue
			}

			// Keep non-thinking parts
			if !IsThinkingPart(part) {
				filteredParts = append(filteredParts, part)
				continue
			}

			// Keep thinking parts with valid signatures
			if HasValidSignature(part) {
				filteredParts = append(filteredParts, part)
			}
			// Otherwise drop the unsigned thinking part
		}

		// Keep at least one part
		if len(filteredParts) == 0 && len(parts) > 0 {
			result = append(result, content)
			continue
		}

		newContent := make(map[string]interface{})
		for k, v := range content {
			if k == "parts" {
				newContent[k] = filteredParts
			} else {
				newContent[k] = v
			}
		}
		result = append(result, newContent)
	}

	return result
}
