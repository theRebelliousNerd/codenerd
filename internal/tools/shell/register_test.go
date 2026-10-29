package shell

import (
	"testing"

	"codenerd/internal/tools"
)

func TestRegisterAllShell(t *testing.T) {
	r := tools.NewRegistry()
	if err := RegisterAll(r); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if len(r.Names()) == 0 {
		t.Fatal("expected shell tools to be registered")
	}
	if err := RegisterAll(r); err != nil {
		t.Errorf("second RegisterAll should be a no-op, got: %v", err)
	}
}
