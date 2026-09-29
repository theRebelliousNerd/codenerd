package embedding

import (
	"testing"
	"time"
)

func TestNewOllamaEngine_ClientTimeoutFollowsInstalledBound(t *testing.T) {
	prev := EmbedRequestTimeout()
	t.Cleanup(func() {
		if prev > 0 {
			SetEmbedRequestTimeout(prev)
			return
		}
		embedRequestTimeout.Store(0)
	})

	SetEmbedRequestTimeout(90 * time.Second)
	engine, err := NewOllamaEngine("http://127.0.0.1:9", "timeout-probe", 3)
	if err != nil {
		t.Fatal(err)
	}
	if engine.client.Timeout != 90*time.Second {
		t.Fatalf("client timeout = %s, want 90s from the installed embed bound", engine.client.Timeout)
	}
}

func TestNewOllamaEngine_UninstalledBoundLeavesClientUnbounded(t *testing.T) {
	prev := EmbedRequestTimeout()
	embedRequestTimeout.Store(0)
	t.Cleanup(func() {
		if prev > 0 {
			SetEmbedRequestTimeout(prev)
		}
	})
	engine, err := NewOllamaEngine("http://127.0.0.1:9", "timeout-probe", 3)
	if err != nil {
		t.Fatal(err)
	}
	if engine.client.Timeout != 0 {
		t.Fatalf("client timeout = %s, want 0 when no bound is installed", engine.client.Timeout)
	}
}
