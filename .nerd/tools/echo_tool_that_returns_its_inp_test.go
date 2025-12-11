package tools

import (
	"context"
	"testing"
	"time"
)

func TestEchoToolThatReturnsItsInp(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "Happy path with simple string",
			ctx:     context.Background(),
			input:   "hello world",
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "Happy path with empty string",
			ctx:     context.Background(),
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "Happy path with special characters",
			ctx:     context.Background(),
			input:   "123!@#$%^&*()_+-=[]{}|;':\",./<>?",
			want:    "123!@#$%^&*()_+-=[]{}|;':\",./<>?",
			wantErr: false,
		},
		{
			name:    "Happy path with unicode characters",
			ctx:     context.Background(),
			input:   "こんにちは世界",
			want:    "こんにちは世界",
			wantErr: false,
		},
		{
			name:    "Happy path with long string",
			ctx:     context.Background(),
			input:   string(make([]byte, 10000)), // 10KB string
			want:    string(make([]byte, 10000)),
			wantErr: false,
		},
		{
			name:    "Error case with cancelled context",
			ctx:     func() context.Context { ctx, _ := context.WithCancel(context.Background()); ctx.Cancel(); return ctx }(),
			input:   "should not be echoed",
			want:    "",
			wantErr: true,
		},
		{
			name:    "Error case with deadline exceeded",
			ctx:     func() context.Context { ctx, _ := context.WithTimeout(context.Background(), 1*time.Nanosecond); time.Sleep(1 * time.Millisecond); return ctx }(),
			input:   "should not be echoed",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := echoToolThatReturnsItsInp(tt.ctx, tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("echoToolThatReturnsItsInp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("echoToolThatReturnsItsInp() = %v, want %v", got, tt.want)
			}
		})
	}
}