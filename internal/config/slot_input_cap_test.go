package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A slot's max_input_tokens is a token count with a floor: negative is not a
// count, and a cap below MinPromptTokens could not be sent a compiled prompt.
// Both refuse the file at the field's path; a cap on a slot with no provider
// is reported as capping nothing.
func TestCheck_SlotMaxInputTokens(t *testing.T) {
	tests := []struct {
		name     string
		worker   *WorkerLLMConfig
		planner  *PlannerLLMConfig
		severity Severity // "" when nothing is reported at path
		path     string
	}{
		{
			name:   "unset is no cap",
			worker: &WorkerLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5"},
			path:   "worker.max_input_tokens",
		},
		{
			name:   "a cap at the floor is accepted",
			worker: &WorkerLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5", MaxInputTokens: MinPromptTokens},
			path:   "worker.max_input_tokens",
		},
		{
			name:   "the deployment's worker cap is accepted",
			worker: &WorkerLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5", MaxInputTokens: 100000},
			path:   "worker.max_input_tokens",
		},
		{
			name:     "negative is refused",
			worker:   &WorkerLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5", MaxInputTokens: -1},
			severity: SeverityError,
			path:     "worker.max_input_tokens",
		},
		{
			name:     "below the floor is refused",
			worker:   &WorkerLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5", MaxInputTokens: MinPromptTokens - 1},
			severity: SeverityError,
			path:     "worker.max_input_tokens",
		},
		{
			name:     "the planner slot is judged the same way",
			planner:  &PlannerLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5", MaxInputTokens: 100},
			severity: SeverityError,
			path:     "planner.max_input_tokens",
		},
		{
			name:     "a cap on a slot that names no provider caps nothing",
			worker:   &WorkerLLMConfig{MaxInputTokens: 100000},
			severity: SeverityWarning,
			path:     "worker.max_input_tokens",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &UserConfig{
				Provider: "anthropic", Model: "claude-sonnet-5-5", AnthropicAPIKey: "k",
				Worker: tt.worker, Planner: tt.planner,
			}
			problems := cfg.Check(nil)
			var got []Problem
			for _, p := range problems {
				if p.Path == tt.path {
					got = append(got, p)
				}
			}
			if tt.severity == "" {
				if len(got) != 0 {
					t.Fatalf("Check reported %v at %s, want nothing", got, tt.path)
				}
				return
			}
			if len(got) != 1 || got[0].Severity != tt.severity {
				t.Fatalf("Check at %s = %v, want one %s", tt.path, got, tt.severity)
			}
			if got[0].Fix == "" {
				t.Errorf("the problem at %s names no fix: %+v", tt.path, got[0])
			}
		})
	}
}

// The field exists in the strict schema (an unknown key refuses the file), it
// loads onto the worker slot, survives slot resolution, and a value below the
// floor refuses the file with the field named.
func TestLoadUserConfig_SlotMaxInputTokens(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    int
		wantErr string
	}{
		{
			name: "loads onto the resolved worker",
			body: `{"provider":"anthropic","model":"claude-sonnet-5-5","anthropic_api_key":"k",
				"worker":{"provider":"anthropic","model":"claude-haiku-5-5","max_input_tokens":100000}}`,
			want: 100000,
		},
		{
			name: "below the floor refuses the file",
			body: `{"provider":"anthropic","model":"claude-sonnet-5-5","anthropic_api_key":"k",
				"worker":{"provider":"anthropic","model":"claude-haiku-5-5","max_input_tokens":512}}`,
			wantErr: "worker.max_input_tokens",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadUserConfig(path)
			if tt.wantErr != "" {
				var cerr *ConfigError
				if !errors.As(err, &cerr) || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadUserConfig error = %v, want a *ConfigError naming %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadUserConfig: %v", err)
			}
			worker := cfg.GetWorkerLLMConfig()
			if worker == nil {
				t.Fatal("the worker slot did not resolve")
			}
			if worker.MaxInputTokens != tt.want {
				t.Errorf("worker.max_input_tokens = %d, want %d", worker.MaxInputTokens, tt.want)
			}
		})
	}
}

// A planner that differs from the worker only by its input cap is a different
// slot: the cap is part of what a slot is.
func TestGetPlannerLLMConfig_ADifferentInputCapIsADifferentSlot(t *testing.T) {
	cfg := &UserConfig{
		Worker:  &WorkerLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5", MaxInputTokens: 100000},
		Planner: &PlannerLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5"},
	}
	if cfg.GetPlannerLLMConfig() == nil {
		t.Fatal("a planner uncapped where the worker is capped resolved to nil")
	}
}
