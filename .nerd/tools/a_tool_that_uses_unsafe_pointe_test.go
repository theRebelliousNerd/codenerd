package tools

import (
	"context"
	"testing"
	"time"
)

func TestAToolThatUsesUnsafePointe(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		input   string
		want    string
		wantErr bool
		err     error
	}{
		{
			name:    "Happy Path - Simple ASCII",
			ctx:     context.Background(),
			input:   "hello",
			want:    "olleh",
			wantErr: false,
		},
		{
			name:    "Happy Path - Unicode Characters",
			ctx:     context.Background(),
			input:   "hello, 世界",
			want:    "界世 ,olleh",
			wantErr: false,
		},
		{
			name:    "Happy Path - Palindrome",
			ctx:     context.Background(),
			input:   "level",
			want:    "level",
			wantErr: false,
		},
		{
			name:    "Happy Path - Single Character",
			ctx:     context.Background(),
			input:   "a",
			want:    "a",
			wantErr: false,
		},
		{
			name:    "Edge Case - Empty String",
			ctx:     context.Background(),
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "Edge Case - String with Spaces",
			ctx:     context.Background(),
			input:   " a b c ",
			want:    " c b a ",
			wantErr: false,
		},
		{
			name:    "Error Case - Context Canceled",
			ctx:     func() context.Context { ctx, _ := context.WithCancel(context.Background()); ctx.Cancel(); return ctx }(),
			input:   "test",
			want:    "",
			wantErr: true,
			err:     context.Canceled,
		},
		{
			name:    "Error Case - Context Deadline Exceeded",
			ctx:     func() context.Context { ctx, _ := context.WithTimeout(context.Background(), 1*time.Nanosecond); time.Sleep(1 * time.Millisecond); return ctx }(),
			input:   "test",
			want:    "",
			wantErr: true,
			err:     context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := aToolThatUsesUnsafePointe(tt.ctx, tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("aToolThatUsesUnsafePointe() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if !errors.Is(err, tt.err) {
					t.Errorf("aToolThatUsesUnsafePointe() error = %v, expected error to contain %v", err, tt.err)
				}
				return
			}

			if got != tt.want {
				t.Errorf("aToolThatUsesUnsafePointe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegisterAToolThatUsesUnsafePointe(t *testing.T) {
	registry := make(map[string]interface{})
	RegisterAToolThatUsesUnsafePointe(registry)

	toolFunc, ok := registry["a_tool_that_uses_unsafe_pointe"]
	if !ok {
		t.Fatalf("Expected tool 'a_tool_that_uses_unsafe_pointe' to be registered")
	}

	// Type assert to check if it's the correct function signature
	_, ok = toolFunc.(func(context.Context, string) (string, error))
	if !ok {
		t.Fatalf("Registered tool does not have the expected function signature")
	}
}