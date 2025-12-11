package tools

import (
	"context"
	"testing"
)

func TestASimpleHelloWorldGreeter(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		input       string
		want        string
		expectError bool
		errorMsg    string
	}{
		{
			name:  "Happy Path - Standard Name",
			ctx:   context.Background(),
			input: "Goopher",
			want:  "Hello, Goopher! Welcome to the world.",
		},
		{
			name:  "Happy Path - Name with spaces",
			ctx:   context.Background(),
			input: "Code Nerd",
			want:  "Hello, Code Nerd! Welcome to the world.",
		},
		{
			name:  "Happy Path - Name with special characters",
			ctx:   context.Background(),
			input: "user-123",
			want:  "Hello, user-123! Welcome to the world.",
		},
		{
			name:        "Error Case - Empty Input",
			ctx:         context.Background(),
			input:       "",
			expectError: true,
			errorMsg:    "input name cannot be empty",
		},
		{
			name:        "Error Case - Cancelled Context",
			ctx:         func() context.Context { ctx, _ := context.WithCancel(context.Background()); ctx.Cancel(); return ctx }(),
			input:       "AnyName",
			expectError: true,
			errorMsg:    "context cancelled",
		},
		{
			name:  "Edge Case - Single Character Name",
			ctx:   context.Background(),
			input: "A",
			want:  "Hello, A! Welcome to the world.",
		},
		{
			name:  "Edge Case - Very Long Name",
			ctx:   context.Background(),
			input: "ThisIsAVeryLongNameThatMightBeUsedInSomeSystemToTestTheBehaviorOfTheGreeterFunctionWithAStringThatIsNotTypicalInLength",
			want:  "Hello, ThisIsAVeryLongNameThatMightBeUsedInSomeSystemToTestTheBehaviorOfTheGreeterFunctionWithAStringThatIsNotTypicalInLength! Welcome to the world.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := aSimpleHelloWorldGreeter(tt.ctx, tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("aSimpleHelloWorldGreeter() expected error, got none")
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("aSimpleHelloWorldGreeter() error = %v, wantErrMsg %v", err.Error(), tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("aSimpleHelloWorldGreeter() unexpected error = %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("aSimpleHelloWorldGreeter() = %v, want %v", got, tt.want)
			}
		})
	}
}