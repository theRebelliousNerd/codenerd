package perception

import (
	"testing"

	"codenerd/internal/types"
)

var _ types.ModelIdentifier = (*CodexCLIClient)(nil)

var _ types.ModelIdentifier = (*ClaudeCodeCLIClient)(nil)

func TestCLIClientsReportAModelIdentity(t *testing.T) {
	c := &CodexCLIClient{}
	c.SetModel("gpt-5.4")
	provider, model := c.ModelIdentity()
	if provider != "codex-cli" {
		t.Fatalf("CodexCLIClient provider = %q, want %q", provider, "codex-cli")
	}
	if model != "gpt-5.4" {
		t.Fatalf("CodexCLIClient model = %q, want %q", model, "gpt-5.4")
	}

	k := &ClaudeCodeCLIClient{}
	k.SetModel("claude-opus-5")
	provider, model = k.ModelIdentity()
	if provider != "claude-cli" {
		t.Fatalf("ClaudeCodeCLIClient provider = %q, want %q", provider, "claude-cli")
	}
	if model != "claude-opus-5" {
		t.Fatalf("ClaudeCodeCLIClient model = %q, want %q", model, "claude-opus-5")
	}
}
