package logging

import (
	"strings"
	"testing"
)

func BenchmarkEscapeString(b *testing.B) {
	// Create a string that requires escaping
	input := "Hello \"World\"\nThis is a backslash: \\ \tAnd a tab."
	// Make it long enough to matter
	input = strings.Repeat(input, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = escapeString(input)
	}
}

func BenchmarkEscapeStringNoEscapes(b *testing.B) {
	// Create a string that requires NO escaping
	input := "Hello World This is a normal string without special chars."
	// Make it long
	input = strings.Repeat(input, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = escapeString(input)
	}
}
