package tools

import (
	"context"
	"errors"
)

const ToolDescriptionAToolThatUsesUnsafePointe = "A tool that reverses a string using safe operations."

// aToolThatUsesUnsafePointe reverses the input string using safe operations.
// It creates a new string with the characters in reverse order.
func aToolThatUsesUnsafePointe(ctx context.Context, input string) (string, error) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Handle empty string
	if input == "" {
		return "", nil
	}

	// Convert string to rune slice to properly handle Unicode characters
	runes := []rune(input)
	
	// Reverse the runes
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	// Convert back to string
	return string(runes), nil
}

// Register registers the tool with the tool registry
func RegisterAToolThatUsesUnsafePointe(registry map[string]interface{}) {
	registry["a_tool_that_uses_unsafe_pointe"] = aToolThatUsesUnsafePointe
}