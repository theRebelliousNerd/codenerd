package tools

import (
	"context"
	"testing"
)

func TestASimpleLineCounterForText(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		ctx         context.Context
		input       string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:  "Happy Path - Single Line",
			ctx:   ctx,
			input: "Hello, world!",
			want:  "1",
		},
		{
			name:  "Happy Path - Multiple Lines",
			ctx:   ctx,
			input: "line1\nline2\nline3",
			want:  "3",
		},
		{
			name:  "Happy Path - Lines with trailing newline",
			ctx:   ctx,
			input: "line1\nline2\n",
			want:  "2",
		},
		{
			name:  "Happy Path - Empty lines",
			ctx:   ctx,
			input: "line1\n\nline3",
			want:  "3",
		},
		{
			name:  "Happy Path - Only newlines",
			ctx:   ctx,
			input: "\n\n\n",
			want:  "3",
		},
		{
			name:  "Edge Case - Empty String",
			ctx:   ctx,
			input: "",
			want:  "0",
		},
		{
			name:    "Error Case - Context Canceled",
			ctx:     func() context.Context { c, _ := context.WithCancel(ctx); c.Cancel(); return c }(),
			input:   "some text",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := aSimpleLineCounterForText(tt.ctx, tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("aSimpleLineCounterForText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("aSimpleLineCounterForText() error = %v, expected to contain %s", err, tt.errContains)
				}
				return
			}

			if got != tt.want {
				t.Errorf("aSimpleLineCounterForText() = %v, want %v", got, tt.want)
			}
		})
	}
}