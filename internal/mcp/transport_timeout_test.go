package mcp

import (
	"testing"
	"time"
)

// The transport fallback is the installed value from
// integrations.default_timeout: a usable per-server timeout wins, anything
// else falls back to the install, never to a literal in Connect.
func TestResolveTransportTimeout(t *testing.T) {
	prev := DefaultTransportTimeout()
	t.Cleanup(func() { SetTransportTimeoutFallback(prev) })

	SetTransportTimeoutFallback(0) // the 30s last resort
	if got := resolveTransportTimeout("2m"); got != 2*time.Minute {
		t.Errorf("usable timeout = %v, want 2m", got)
	}
	for _, raw := range []string{"", "0s", "-1s", "not-a-duration", "1000000000000h"} {
		if got := resolveTransportTimeout(raw); got != 30*time.Second {
			t.Errorf("timeout %q = %v, want the 30s fallback", raw, got)
		}
	}

	SetTransportTimeoutFallback(45 * time.Second)
	if got := resolveTransportTimeout("bogus"); got != 45*time.Second {
		t.Errorf("unusable timeout = %v, want the installed 45s", got)
	}
	if got := resolveTransportTimeout("2m"); got != 2*time.Minute {
		t.Errorf("usable timeout = %v after install, want 2m", got)
	}
}
