package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCampaignOutcome(t *testing.T) {
	boom := errors.New("boom")
	deadlineErr := context.DeadlineExceeded

	tests := []struct {
		name         string
		runErr       error
		ctxErr       error
		timeout      time.Duration
		wantNil      bool
		wantContains []string
		wantErrIs    error
	}{
		{
			name:    "completed run returns nil",
			runErr:  nil,
			ctxErr:  nil,
			timeout: 25 * time.Minute,
			wantNil: true,
		},
		{
			name:    "completed run with context error still nil when runErr nil",
			runErr:  nil,
			ctxErr:  context.DeadlineExceeded,
			timeout: 10 * time.Second,
			wantNil: true,
		},
		{
			name:         "timeout returns error naming timeout and mentioning resume",
			runErr:       deadlineErr,
			ctxErr:       context.DeadlineExceeded,
			timeout:      25 * time.Minute,
			wantContains: []string{"25m0s", "resume", "--timeout", "nerd campaign resume"},
			wantErrIs:    deadlineErr,
		},
		{
			name:         "timeout with different duration names timeout",
			runErr:       errors.New("context deadline exceeded"),
			ctxErr:       context.Canceled,
			timeout:      5 * time.Minute,
			wantContains: []string{"5m0s", "resume", "--timeout"},
		},
		{
			name:         "genuine failure with no context error returns wrapped failure",
			runErr:       boom,
			ctxErr:       nil,
			timeout:      25 * time.Minute,
			wantContains: []string{"campaign failed", "boom"},
			wantErrIs:    boom,
		},
		{
			name:         "genuine failure preserves timeout independence",
			runErr:       errors.New("something else failed"),
			ctxErr:       nil,
			timeout:      1 * time.Minute,
			wantContains: []string{"campaign failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := campaignOutcome(tt.runErr, tt.ctxErr, tt.timeout)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("campaignOutcome() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("campaignOutcome() = nil, want error")
			}
			msg := got.Error()
			for _, substr := range tt.wantContains {
				if !strings.Contains(msg, substr) {
					t.Errorf("campaignOutcome() error = %q, want to contain %q", msg, substr)
				}
			}
			if tt.wantErrIs != nil && !errors.Is(got, tt.wantErrIs) {
				t.Errorf("campaignOutcome() error = %v, want errors.Is %v", got, tt.wantErrIs)
			}
		})
	}
}
