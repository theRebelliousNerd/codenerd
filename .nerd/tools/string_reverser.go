package tools

import (
	"context"
	"errors"
)

// ToolDescriptionStringReverser describes the string_reverser tool functionality.
const ToolDescriptionStringReverser = "Reverses the input string character by character."

// stringReverser reverses the input string character by character.
// It returns an error if the input string is empty.
func stringReverser(ctx context.Context, input string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Validate input
	if input == "" {
		return "", errors.New("input string cannot be empty")
	}

	// Convert string to rune slice to handle multi-byte characters correctly
	runes := []rune(input)

	// Reverse the rune slice
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	// Convert back to string and return
	return string(runes), nil
}