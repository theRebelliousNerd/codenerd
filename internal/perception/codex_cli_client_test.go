package perception

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

func TestNewCodexCLIClient(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.CodexCLIConfig
		wantModel   string
		wantSandbox string
		wantTimeout time.Duration
	}{
		{
			name:        "nil config uses defaults",
			cfg:         nil,
			wantModel:   "gpt-5.3-codex",
			wantSandbox: "read-only",
			wantTimeout: 300 * time.Second,
		},
		{
			name: "custom model + sandbox + timeout",
			cfg: &config.CodexCLIConfig{
				Model:   "o4-mini",
				Sandbox: "workspace-write",
				Timeout: 600,
			},
			wantModel:   "o4-mini",
			wantSandbox: "workspace-write",
			wantTimeout: 600 * time.Second,
		},
		{
			name: "empty model uses default",
			cfg: &config.CodexCLIConfig{
				Model:   "",
				Sandbox: "read-only",
				Timeout: 120,
			},
			wantModel:   "gpt-5.3-codex",
			wantSandbox: "read-only",
			wantTimeout: 120 * time.Second,
		},
		{
			name: "zero timeout uses default",
			cfg: &config.CodexCLIConfig{
				Model:   "o3",
				Sandbox: "read-only",
				Timeout: 0,
			},
			wantModel:   "o3",
			wantSandbox: "read-only",
			wantTimeout: 300 * time.Second,
		},
		{
			name: "fallback model configured",
			cfg: &config.CodexCLIConfig{
				Model:         "gpt-5.3-codex",
				FallbackModel: "o4-mini",
				Timeout:       300,
			},
			wantModel:   "gpt-5.3-codex",
			wantSandbox: "read-only",
			wantTimeout: 300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewCodexCLIClient(tt.cfg)

			if client.GetModel() != tt.wantModel {
				t.Errorf("GetModel() = %q, want %q", client.GetModel(), tt.wantModel)
			}

			if client.GetSandbox() != tt.wantSandbox {
				t.Errorf("GetSandbox() = %q, want %q", client.GetSandbox(), tt.wantSandbox)
			}

			if client.GetTimeout() != tt.wantTimeout {
				t.Errorf("GetTimeout() = %v, want %v", client.GetTimeout(), tt.wantTimeout)
			}
		})
	}
}

func TestCodexCLIClient_SettersGetters(t *testing.T) {
	client := NewCodexCLIClient(nil)

	t.Run("SetModel and GetModel", func(t *testing.T) {
		client.SetModel("o4-mini")
		if got := client.GetModel(); got != "o4-mini" {
			t.Errorf("GetModel() after SetModel(o4-mini) = %q, want o4-mini", got)
		}
	})

	t.Run("SetFallbackModel and GetFallbackModel", func(t *testing.T) {
		client.SetFallbackModel("o3-mini")
		if got := client.GetFallbackModel(); got != "o3-mini" {
			t.Errorf("GetFallbackModel() after SetFallbackModel(o3-mini) = %q, want o3-mini", got)
		}
	})

	t.Run("SetSandbox and GetSandbox", func(t *testing.T) {
		client.SetSandbox("workspace-write")
		if got := client.GetSandbox(); got != "workspace-write" {
			t.Errorf("GetSandbox() after SetSandbox(workspace-write) = %q, want workspace-write", got)
		}
	})

	t.Run("SetTimeout and GetTimeout", func(t *testing.T) {
		client.SetTimeout(60 * time.Second)
		if got := client.GetTimeout(); got != 60*time.Second {
			t.Errorf("GetTimeout() after SetTimeout(60s) = %v, want 60s", got)
		}
	})

	t.Run("SetStreaming and GetStreaming", func(t *testing.T) {
		client.SetStreaming(true)
		if got := client.GetStreaming(); got != true {
			t.Errorf("GetStreaming() after SetStreaming(true) = %v, want true", got)
		}
	})
}

func TestCodexCLIClient_buildPrompt(t *testing.T) {
	client := NewCodexCLIClient(nil)

	tests := []struct {
		name         string
		systemPrompt string
		userPrompt   string
		want         string
	}{
		{
			name:         "no system prompt",
			systemPrompt: "",
			userPrompt:   "Hello",
			want:         "Hello",
		},
		{
			name:         "whitespace only system prompt",
			systemPrompt: "   ",
			userPrompt:   "Hello",
			want:         "Hello",
		},
		{
			name:         "with system prompt",
			systemPrompt: "You are helpful.",
			userPrompt:   "Hello",
			want:         "<system_instructions>\nYou are helpful.\n</system_instructions>\n\nHello",
		},
		{
			name:         "system prompt with newlines",
			systemPrompt: "Line 1\nLine 2",
			userPrompt:   "Question",
			want:         "<system_instructions>\nLine 1\nLine 2\n</system_instructions>\n\nQuestion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.buildPrompt(tt.systemPrompt, tt.userPrompt)
			if got != tt.want {
				t.Errorf("buildPrompt(%q, %q) = %q, want %q", tt.systemPrompt, tt.userPrompt, got, tt.want)
			}
		})
	}
}

func TestCodexCLIClient_buildCLIArgs_DisableShellToolDefault(t *testing.T) {
	client := NewCodexCLIClient(nil)

	args := client.buildCLIArgs(context.Background(), client.GetModel(), "out.txt", "")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--disable shell_tool") {
		t.Fatalf("expected default args to disable shell_tool, got: %s", joined)
	}
}

func TestCodexCLIClient_buildCLIArgs_DisableShellToolConfig(t *testing.T) {
	disableShell := false
	client := NewCodexCLIClient(&config.CodexCLIConfig{
		Model:            "gpt-5.3-codex",
		DisableShellTool: &disableShell,
	})

	args := client.buildCLIArgs(context.Background(), client.GetModel(), "out.txt", "")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--disable shell_tool") {
		t.Fatalf("did not expect args to disable shell_tool when configured false, got: %s", joined)
	}
}

func TestCodexCLIClient_buildCLIArgs_ReasoningEffortByCapability(t *testing.T) {
	client := NewCodexCLIClient(&config.CodexCLIConfig{
		Model:                        "gpt-5.3-codex",
		ReasoningEffortDefault:       "high",
		ReasoningEffortHighSpeed:     "low",
		ReasoningEffortBalanced:      "medium",
		ReasoningEffortHighReasoning: "xhigh",
	})

	ctx := context.WithValue(context.Background(), types.CtxKeyModelCapability, types.CapabilityHighReasoning)
	args := client.buildCLIArgs(ctx, client.GetModel(), "out.txt", "")
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "model_reasoning_effort=\"xhigh\"") {
		t.Fatalf("expected high_reasoning to select xhigh effort, got: %s", joined)
	}
}

func TestCodexCLIClient_buildCLIArgs_ConfigOverridesWin(t *testing.T) {
	client := NewCodexCLIClient(&config.CodexCLIConfig{
		Model:                  "gpt-5.3-codex",
		ReasoningEffortDefault: "xhigh",
		ConfigOverrides: map[string]string{
			"model_reasoning_effort": "\"low\"",
		},
	})

	ctx := context.WithValue(context.Background(), types.CtxKeyModelCapability, types.CapabilityHighReasoning)
	args := client.buildCLIArgs(ctx, client.GetModel(), "out.txt", "")
	joined := strings.Join(args, " ")

	// Should not add a second override; the explicit override should be used.
	if strings.Count(joined, "model_reasoning_effort=") != 1 {
		t.Fatalf("expected exactly one model_reasoning_effort override, got: %s", joined)
	}
	if !strings.Contains(joined, "model_reasoning_effort=\"low\"") {
		t.Fatalf("expected explicit override to win, got: %s", joined)
	}
}

func TestCodexCLIClient_buildCLIArgs_ConfigOverridesDeterministicOrder(t *testing.T) {
	client := NewCodexCLIClient(&config.CodexCLIConfig{
		Model: "gpt-5.3-codex",
		ConfigOverrides: map[string]string{
			"z_key": "\"z\"",
			"a_key": "\"a\"",
		},
	})

	args := client.buildCLIArgs(context.Background(), client.GetModel(), "out.txt", "")
	joined := strings.Join(args, " ")

	// We sort keys, so a_key should appear before z_key.
	if strings.Index(joined, "a_key=") < 0 || strings.Index(joined, "z_key=") < 0 {
		t.Fatalf("expected both overrides present, got: %s", joined)
	}
	if strings.Index(joined, "a_key=") > strings.Index(joined, "z_key=") {
		t.Fatalf("expected deterministic ordering (a_key before z_key), got: %s", joined)
	}
}

func TestCodexCLIClient_StreamingConfig(t *testing.T) {
	cfg := &config.CodexCLIConfig{
		Model:     "gpt-5.3-codex",
		Streaming: true,
	}

	client := NewCodexCLIClient(cfg)
	if got := client.GetStreaming(); got != true {
		t.Errorf("GetStreaming() = %v, want true", got)
	}
}

// TestCodexCLIClient_LLMClientInterface verifies the client implements LLMClient.
func TestCodexCLIClient_LLMClientInterface(t *testing.T) {
	var _ LLMClient = (*CodexCLIClient)(nil)
}
