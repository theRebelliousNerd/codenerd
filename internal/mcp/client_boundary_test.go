package mcp

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestConnectAll_EmptyConfig verifies that ConnectAll with an entirely empty
// config map returns nil (no servers to connect, no error) and does not panic.
//
// QA boundary item: TestConnectAll_EmptyConfig.
func TestConnectAll_EmptyConfig(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, map[string]MCPServerConfig{})
	if err := mgr.ConnectAll(context.Background()); err != nil {
		t.Errorf("expected nil error for empty config, got %v", err)
	}

	// Same with nil config — must not panic.
	mgr2 := NewMCPClientManager(nil, nil, nil)
	if err := mgr2.ConnectAll(context.Background()); err != nil {
		t.Errorf("expected nil error for nil config, got %v", err)
	}
}

// TestDisconnect_EmptyServerID verifies Disconnect("") returns a clear error
// rather than no-op or panic.
//
// QA boundary item: TestDisconnect_EmptyServerID.
func TestDisconnect_EmptyServerID(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	err := mgr.Disconnect("")
	if err == nil {
		t.Fatal("expected error for empty server ID")
	}
	if err.Error() != "server ID cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestConnect_InvalidProtocol verifies Connect rejects clearly invalid
// protocol strings.
//
// QA boundary item: TestConnect_InvalidProtocol (e.g., "ftp://", "").
func TestConnect_InvalidProtocol(t *testing.T) {
	cases := []struct {
		name     string
		protocol string
		wantErr  string // substring of expected error
	}{
		{"empty_protocol", "", "protocol cannot be empty"},
		{"ftp_url", "ftp://", "unsupported protocol"},
		{"random_string", "not_a_protocol", "unsupported protocol"},
		{"unicode_garbage", "日本語", "unsupported protocol"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := map[string]MCPServerConfig{
				"srv": {ID: "srv", Enabled: true, Protocol: tt.protocol},
			}
			mgr := NewMCPClientManager(nil, nil, cfg)
			err := mgr.Connect(context.Background(), "srv")
			if err == nil {
				t.Fatalf("expected error for protocol %q", tt.protocol)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestParseToolID_Malformed verifies parseToolID handles malformed IDs without
// panic and returns sensible (empty) parts.
//
// QA boundary item: TestParseToolID_Malformed (just "/", "server/", "/tool").
func TestParseToolID_Malformed(t *testing.T) {
	cases := []struct {
		input      string
		wantServer string
		wantTool   string
	}{
		{"/", "", ""},
		{"server/", "server", ""},
		{"/tool", "", "tool"},
		{"", "", ""},
		{"//double//slash", "//double/", "slash"},
	}

	for _, tt := range cases {
		t.Run(tt.input, func(t *testing.T) {
			server, tool := parseToolID(tt.input)
			if server != tt.wantServer || tool != tt.wantTool {
				t.Errorf("parseToolID(%q) = (%q, %q), want (%q, %q)",
					tt.input, server, tool, tt.wantServer, tt.wantTool)
			}
		})
	}
}

// TestParseDuration_ExtremeValues verifies that the Connect timeout-parsing
// logic guards against extreme duration strings — "0" (zero), "-1s" (negative),
// massive overflow values, and unparseable junk — falling back to the 30s
// default rather than passing a useless or panicking duration to NewHTTPTransport.
//
// We do this by driving Connect with each timeout value and asserting it does
// not panic. We expect a non-nil error in every case because the connection
// itself will fail (no listening server), but the failure must come from the
// transport layer, never from a panic in duration handling.
//
// QA boundary item: TestParseDuration_ExtremeValues.
func TestParseDuration_ExtremeValues(t *testing.T) {
	values := []string{
		"",
		"0",
		"0s",
		"-1s",
		"1000000000000h", // overflow — time.ParseDuration returns an error
		"not-a-duration",
		"1.5h",  // valid — should also work
		"100ms", // valid small positive
	}

	for _, tv := range values {
		t.Run(tv, func(t *testing.T) {
			// Use an unreachable address so Connect fails quickly without
			// blocking on real network. We just want to exercise the
			// timeout-parsing branch and confirm no panic.
			cfg := map[string]MCPServerConfig{
				"srv": {
					ID:       "srv",
					Enabled:  true,
					Protocol: "http",
					BaseURL:  "http://127.0.0.1:1", // unlikely to listen
					Timeout:  tv,
				},
			}
			mgr := NewMCPClientManager(nil, nil, cfg)
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			// Must NOT panic. Returned error is fine (network unreachable).
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Connect panicked on timeout %q: %v", tv, r)
				}
			}()
			_ = mgr.Connect(ctx, "srv")
		})
	}
}
