package core

import (
	"testing"

	"codenerd/internal/tools"
)

func TestRegisterAllCore(t *testing.T) {
	r := tools.NewRegistry()
	if err := RegisterAll(r); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if len(r.Names()) == 0 {
		t.Fatal("expected core tools to be registered")
	}
	// Idempotent: already-registered tools are skipped, not re-erroring.
	if err := RegisterAll(r); err != nil {
		t.Errorf("second RegisterAll should be a no-op, got: %v", err)
	}
}
