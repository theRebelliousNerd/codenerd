package tools

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

const ToolDescriptionWordCounter = "Counts the number of words in a given text string"

// wordCounter counts the number of words in the input string.
// A word is defined as a sequence of alphanumeric characters separated by whitespace or punctuation.
func wordCounter(ctx context.Context, input string) (string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Handle empty input
	if input == "" {
		return "0", nil
	}

	// Count words using a simple state machine
	count := 0
	inWord := false

	for _, r := range input {
		// Check for context cancellation during processing
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if !inWord {
				count++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	return fmt.Sprintf("%d", count), nil
}