package system

import (
	"context"
	"testing"
	"time"
)

// The consultation routing policy escalates an unanswered question to a
// stronger profile only when the configuration names one: boot installs it
// from shard_profiles.consultation_escalation, and a default_shard is not
// read as one.
func TestBootInstallsTheConsultEscalationProfileOnlyWhenConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	for _, tc := range []struct {
		name   string
		config string
		want   bool
	}{
		{"named", `{"provider":"openai","model":"gpt-5.4","openai_api_key":"test-key","shard_profiles":{"consultation_escalation":{"provider":"openai","model":"gpt-5.4"}}}`, true},
		{"default shard only", `{"provider":"openai","model":"gpt-5.4","openai_api_key":"test-key","default_shard":{"provider":"openai","model":"gpt-5.4"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			writeCortexIdentityConfig(t, ws, tc.config)
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			cortex, err := BootCortex(ctx, ws, "", nil)
			if err != nil {
				t.Fatalf("BootCortex: %v", err)
			}
			defer requireWorkspaceReleased(t, cortex, ws)
			if got := cortex.SessionExecutor.HasConsultEscalation(); got != tc.want {
				t.Fatalf("escalation installed = %v, want %v", got, tc.want)
			}
			if got := cortex.SessionExecutor.CloneForTask().HasConsultEscalation(); got != tc.want {
				t.Fatalf("a task clone's escalation = %v, want %v", got, tc.want)
			}
		})
	}
}
