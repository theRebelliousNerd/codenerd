package mcp

import (
	"testing"
)

// --- DefaultToolSelectionConfig ---

func TestDefaultToolSelectionConfig_ShouldReturnDefaults(t *testing.T) {
	cfg := DefaultToolSelectionConfig()

	if cfg.FullThreshold != 70 {
		t.Errorf("FullThreshold = %d, want 70", cfg.FullThreshold)
	}
	if cfg.CondensedThreshold != 40 {
		t.Errorf("CondensedThreshold = %d, want 40", cfg.CondensedThreshold)
	}
	if cfg.MinimalThreshold != 20 {
		t.Errorf("MinimalThreshold = %d, want 20", cfg.MinimalThreshold)
	}
	if cfg.MaxFullTools != 10 {
		t.Errorf("MaxFullTools = %d, want 10", cfg.MaxFullTools)
	}
	if cfg.MaxCondensedTools != 20 {
		t.Errorf("MaxCondensedTools = %d, want 20", cfg.MaxCondensedTools)
	}
	if cfg.TokenBudget != 4000 {
		t.Errorf("TokenBudget = %d, want 4000", cfg.TokenBudget)
	}
}

// --- ServerStatus constants ---

func TestServerStatus_Values(t *testing.T) {
	tests := []struct {
		status ServerStatus
		want   string
	}{
		{ServerStatusUnknown, "unknown"},
		{ServerStatusConnecting, "connecting"},
		{ServerStatusConnected, "connected"},
		{ServerStatusDisconnected, "disconnected"},
		{ServerStatusError, "error"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("ServerStatus = %q, want %q", tt.status, tt.want)
			}
		})
	}
}

// --- Protocol constants ---

func TestProtocol_Values(t *testing.T) {
	if string(ProtocolHTTP) != "http" {
		t.Errorf("ProtocolHTTP = %q", ProtocolHTTP)
	}
	if string(ProtocolStdio) != "stdio" {
		t.Errorf("ProtocolStdio = %q", ProtocolStdio)
	}
	if string(ProtocolSSE) != "sse" {
		t.Errorf("ProtocolSSE = %q", ProtocolSSE)
	}
}

// --- RenderMode constants ---

func TestRenderMode_Values(t *testing.T) {
	tests := []struct {
		mode RenderMode
		want string
	}{
		{RenderModeFull, "full"},
		{RenderModeCondensed, "condensed"},
		{RenderModeMinimal, "minimal"},
		{RenderModeExcluded, "excluded"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.mode) != tt.want {
				t.Errorf("RenderMode = %q, want %q", tt.mode, tt.want)
			}
		})
	}
}
