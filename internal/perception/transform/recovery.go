package transform

// Thinking Recovery Module
//
// Minimal implementation for recovering from corrupted thinking state.
// When Claude's conversation history gets corrupted (thinking blocks stripped/malformed),
// this module provides a "last resort" recovery by closing the current turn and starting fresh.
//
// Philosophy: "Let it crash and start again" - Instead of trying to fix corrupted state,
// we abandon the corrupted turn and let Claude generate fresh thinking.

// ConversationState holds analyzed conversation state for thinking mode
type ConversationState struct {
	// InToolLoop is true if we're in an incomplete tool use loop (ends with functionResponse)
	InToolLoop bool

	// TurnStartIdx is the index of first model message in current turn
	TurnStartIdx int

	// TurnHasThinking indicates whether the TURN started with thinking
	TurnHasThinking bool

	// LastModelIdx is the index of last model message
	LastModelIdx int

	// LastModelHasThinking indicates whether last model msg has thinking
	LastModelHasThinking bool

	// LastModelHasToolCalls indicates whether last model msg has tool calls
	LastModelHasToolCalls bool
}

// isFunctionResponsePart checks if a message part is a function response (tool result)
func isFunctionResponsePart(part map[string]interface{}) bool {
	_, ok := part["functionResponse"]
	return ok
}

// isFunctionCallPart checks if a message part is a function call
func isFunctionCallPart(part map[string]interface{}) bool {
	_, ok := part["functionCall"]
	return ok
}

// isToolResultMessage checks if a message is a tool result container
func isToolResultMessage(msg map[string]interface{}) bool {
	role, _ := msg["role"].(string)
	if role != "user" {
		return false
	}

	parts, ok := msg["parts"].([]interface{})
	if !ok {
		return false
	}

	for _, p := range parts {
		part, ok := p.(map[string]interface{})
		if ok && isFunctionResponsePart(part) {
			return true
		}
	}
	return false
}

// messageHasThinking checks if a message contains thinking/reasoning content
func messageHasThinking(msg map[string]interface{}) bool {
	// Check Gemini format: parts array
	if parts, ok := msg["parts"].([]interface{}); ok {
		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if ok && IsThinkingPart(part) {
				return true
			}
		}
	}

	// Check Anthropic format: content array
	if content, ok := msg["content"].([]interface{}); ok {
		for _, c := range content {
			block, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			if blockType == "thinking" || blockType == "redacted_thinking" {
				return true
			}
		}
	}

	return false
}

// messageHasToolCalls checks if a message contains tool calls
func messageHasToolCalls(msg map[string]interface{}) bool {
	// Check Gemini format: parts array with functionCall
	if parts, ok := msg["parts"].([]interface{}); ok {
		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if ok && isFunctionCallPart(part) {
				return true
			}
		}
	}

	// Check Anthropic format: content array with tool_use
	if content, ok := msg["content"].([]interface{}); ok {
		for _, c := range content {
			block, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			if blockType == "tool_use" {
				return true
			}
		}
	}

	return false
}

// AnalyzeConversationState analyzes conversation state to detect tool use loops and thinking mode issues.
//
// Key insight: A "turn" can span multiple assistant messages in a tool-use loop.
// We need to find the TURN START (first assistant message after last real user message)
// and check if THAT message had thinking, not just the last assistant message.
func AnalyzeConversationState(contents []map[string]interface{}) *ConversationState {
	state := &ConversationState{
		TurnStartIdx: -1,
		LastModelIdx: -1,
	}

	if len(contents) == 0 {
		return state
	}

	// First pass: Find the last "real" user message (not a tool result)
	lastRealUserIdx := -1
	for i, msg := range contents {
		role, _ := msg["role"].(string)
		if role == "user" && !isToolResultMessage(msg) {
			lastRealUserIdx = i
		}
	}

	// Second pass: Analyze conversation and find turn boundaries
	for i, msg := range contents {
		role, _ := msg["role"].(string)

		if role == "model" || role == "assistant" {
			hasThinking := messageHasThinking(msg)
			hasToolCalls := messageHasToolCalls(msg)

			// Track if this is the turn start
			if i > lastRealUserIdx && state.TurnStartIdx == -1 {
				state.TurnStartIdx = i
				state.TurnHasThinking = hasThinking
			}

			state.LastModelIdx = i
			state.LastModelHasToolCalls = hasToolCalls
			state.LastModelHasThinking = hasThinking
		}
	}

	// Determine if we're in a tool loop
	// We're in a tool loop if the conversation ends with a tool result
	if len(contents) > 0 {
		lastMsg := contents[len(contents)-1]
		role, _ := lastMsg["role"].(string)
		if role == "user" && isToolResultMessage(lastMsg) {
			state.InToolLoop = true
		}
	}

	return state
}

// NeedsThinkingRecovery checks if conversation state requires tool loop closure for thinking recovery.
//
// Returns true if:
// - We're in a tool loop (state.InToolLoop)
// - The turn didn't start with thinking (state.TurnHasThinking === false)
//
// This is the trigger for the "let it crash and start again" recovery.
func NeedsThinkingRecovery(state *ConversationState) bool {
	return state.InToolLoop && !state.TurnHasThinking
}

// countTrailingToolResults counts tool results at the end of the conversation
func countTrailingToolResults(contents []map[string]interface{}) int {
	count := 0

	for i := len(contents) - 1; i >= 0; i-- {
		msg := contents[i]
		role, _ := msg["role"].(string)

		if role == "user" {
			parts, ok := msg["parts"].([]interface{})
			if !ok {
				break
			}

			hasResponse := false
			for _, p := range parts {
				part, ok := p.(map[string]interface{})
				if ok && isFunctionResponsePart(part) {
					count++
					hasResponse = true
				}
			}

			if !hasResponse {
				break // Real user message, stop counting
			}
		} else if role == "model" || role == "assistant" {
			break // Stop at the model that made the tool calls
		}
	}

	return count
}

// CloseToolLoopForThinking closes an incomplete tool loop by injecting synthetic messages to start a new turn.
//
// This is the "let it crash and start again" recovery mechanism.
//
// When we detect:
// - We're in a tool loop (conversation ends with functionResponse)
// - The tool call was made WITHOUT thinking (thinking was stripped/corrupted)
// - We NOW want to enable thinking
//
// Instead of trying to fix the corrupted state, we:
// 1. Strip ALL thinking blocks (removes any corrupted ones)
// 2. Add synthetic MODEL message to complete the non-thinking turn
// 3. Add synthetic USER message to start a NEW turn
//
// This allows Claude to generate fresh thinking for the new turn.
func CloseToolLoopForThinking(contents []map[string]interface{}) []map[string]interface{} {
	// Strip any old/corrupted thinking first
	strippedContents := StripAllThinkingBlocks(contents)

	// Count tool results from the end of the conversation
	toolResultCount := countTrailingToolResults(strippedContents)

	// Build synthetic model message content based on tool count
	var syntheticModelContent string
	switch {
	case toolResultCount == 0:
		syntheticModelContent = "[Processing previous context.]"
	case toolResultCount == 1:
		syntheticModelContent = "[Tool execution completed.]"
	default:
		syntheticModelContent = "[" + string(rune('0'+toolResultCount)) + " tool executions completed.]"
	}

	// Step 1: Inject synthetic MODEL message to complete the non-thinking turn
	syntheticModel := map[string]interface{}{
		"role": "model",
		"parts": []interface{}{
			map[string]interface{}{"text": syntheticModelContent},
		},
	}

	// Step 2: Inject synthetic USER message to start a NEW turn
	syntheticUser := map[string]interface{}{
		"role": "user",
		"parts": []interface{}{
			map[string]interface{}{"text": "[Continue]"},
		},
	}

	return append(strippedContents, syntheticModel, syntheticUser)
}

// LooksLikeCompactedThinkingTurn detects if a message looks like it was compacted from a thinking-enabled turn.
//
// This is a heuristic to distinguish between:
// - "Never had thinking" (model didn't use thinking mode)
// - "Thinking was stripped" (context compaction removed thinking blocks)
//
// Heuristics:
// 1. Has functionCall parts (typical thinking flow produces tool calls)
// 2. No thinking parts (thought: true)
// 3. No text content before functionCall (thinking responses usually have text)
func LooksLikeCompactedThinkingTurn(msg map[string]interface{}) bool {
	parts, ok := msg["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return false
	}

	// Check if message has function calls
	hasFunctionCall := false
	firstFuncIdx := -1
	for i, p := range parts {
		part, ok := p.(map[string]interface{})
		if ok && isFunctionCallPart(part) {
			hasFunctionCall = true
			if firstFuncIdx == -1 {
				firstFuncIdx = i
			}
		}
	}

	if !hasFunctionCall {
		return false
	}

	// Check for thinking blocks
	hasThinking := false
	for _, p := range parts {
		part, ok := p.(map[string]interface{})
		if ok && IsThinkingPart(part) {
			hasThinking = true
			break
		}
	}

	if hasThinking {
		return false
	}

	// Check for text content before the first functionCall
	hasTextBeforeFunctionCall := false
	for i, p := range parts {
		if i >= firstFuncIdx {
			break
		}
		part, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		text, _ := part["text"].(string)
		thought, _ := part["thought"].(bool)
		if text != "" && !thought {
			hasTextBeforeFunctionCall = true
			break
		}
	}

	// If we have functionCall but no text before it, likely compacted
	return !hasTextBeforeFunctionCall
}

// HasPossibleCompactedThinking checks if any message in the current turn looks like it was compacted.
func HasPossibleCompactedThinking(contents []map[string]interface{}, turnStartIdx int) bool {
	if turnStartIdx < 0 || turnStartIdx >= len(contents) {
		return false
	}

	for i := turnStartIdx; i < len(contents); i++ {
		msg := contents[i]
		role, _ := msg["role"].(string)
		if role == "model" && LooksLikeCompactedThinkingTurn(msg) {
			return true
		}
	}

	return false
}
