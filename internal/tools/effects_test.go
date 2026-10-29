package tools

import (
	"context"
	"strings"
	"testing"
)

func TestUnknownEffectsCannotExecute(t *testing.T) {
	executed := false
	r := NewRegistry()
	tool := &Tool{Name: "unclassified", Execute: func(context.Context, map[string]any) (string, error) { executed = true; return "", nil }}
	if _, err := r.ExecuteTool(t.Context(), tool, nil); err == nil || !strings.Contains(err.Error(), "effect declaration") {
		t.Fatalf("unknown effect: %v", err)
	}
	if executed {
		t.Fatal("unclassified tool ran")
	}
	tool.Effect = EffectRead
	if _, err := r.ExecuteTool(t.Context(), tool, nil); err != nil {
		t.Fatal(err)
	}
	if !executed {
		t.Fatal("explicit effect did not run")
	}
}
