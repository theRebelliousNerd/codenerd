package tools

import (
	"context"
	"testing"
)

func TestCreateAToolThatValidatesJ(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		ctx         context.Context
		input       string
		expected    string
		expectError bool
	}{
		// Happy path tests
		{
			name:     "Valid JSON object",
			ctx:      ctx,
			input:    `{"key": "value", "number": 123}`,
			expected: "Valid JSON",
		},
		{
			name:     "Valid JSON array",
			ctx:      ctx,
			input:    `[1, "two", {"three": 3}]`,
			expected: "Valid JSON",
		},
		{
			name:     "Valid JSON with nested structures",
			ctx:      ctx,
			input:    `{"user": {"id": 1, "name": "test"}, "items": [1, 2, 3]}`,
			expected: "Valid JSON",
		},
		{
			name:     "Valid JSON with different data types",
			ctx:      ctx,
			input:    `{"string": "hello", "int": 42, "float": 3.14, "bool": true, "null": null}`,
			expected: "Valid JSON",
		},

		// Error case tests
		{
			name:     "Invalid JSON - missing closing brace",
			ctx:      ctx,
			input:    `{"key": "value"`,
			expected: "Invalid JSON: unexpected end of JSON input",
		},
		{
			name:     "Invalid JSON - trailing comma",
			ctx:      ctx,
			input:    `{"key": "value",}`,
			expected: "Invalid JSON: invalid character '}' looking for beginning of object key string",
		},
		{
			name:     "Invalid JSON - unquoted key",
			ctx:      ctx,
			input:    `{key: "value"}`,
			expected: "Invalid JSON: invalid character 'k' looking for beginning of object key string",
		},
		{
			name:     "Invalid JSON - single quotes",
			ctx:      ctx,
			input:    `{'key': 'value'}`,
			expected: "Invalid JSON: invalid character '\\' looking for beginning of value",
		},

		// Edge case tests
		{
			name:     "Empty input string",
			ctx:      ctx,
			input:    "",
			expected: "Error: empty input provided",
		},
		{
			name:     "Input with only whitespace",
			ctx:      ctx,
			input:    "   \n\t  ",
			expected: "Invalid JSON: unexpected end of JSON input",
		},
		{
			name:     "Simple valid JSON string",
			ctx:      ctx,
			input:    `"just a string"`,
			expected: "Valid JSON",
		},
		{
			name:     "Simple valid JSON number",
			ctx:      ctx,
			input:    `123`,
			expected: "Valid JSON",
		},
		{
			name:     "Valid JSON boolean true",
			ctx:      ctx,
			input:    `true`,
			expected: "Valid JSON",
		},
		{
			name:     "Valid JSON boolean false",
			ctx:      ctx,
			input:    `false`,
			expected: "Valid JSON",
		},
		{
			name:     "Valid JSON null",
			ctx:      ctx,
			input:    `null`,
			expected: "Valid JSON",
		},
		{
			name:        "Context is cancelled",
			ctx:         func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			input:       `{"key": "value"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createAToolThatValidatesJ(tt.ctx, tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("createAToolThatValidatesJ() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("createAToolThatValidatesJ() unexpected error = %v", err)
				return
			}

			if got != tt.expected {
				t.Errorf("createAToolThatValidatesJ() = %q, want %q", got, tt.expected)
			}
		})
	}
}