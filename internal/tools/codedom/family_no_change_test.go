package codedom

import (
	"context"
	"testing"

	"codenerd/internal/tools"
)

func TestFamilyNoChange_RecordsAReasonedDecision(t *testing.T) {
	ctx, decisions := tools.WithFamilyDecisionLog(context.Background())
	if _, err := executeFamilyNoChange(ctx, map[string]any{"family": "retry", "member": "fn:b.Retry", "reason": "ok"}); err == nil {
		t.Fatal("a decision without a real reason was recorded")
	}
	if _, err := executeFamilyNoChange(ctx, map[string]any{"family": "retry", "member": "fn:b.Retry", "reason": "b already returns as soon as ctx.Done closes"}); err != nil {
		t.Fatal(err)
	}
	got := decisions()
	if len(got) != 1 || got[0].Member != "fn:b.Retry" {
		t.Fatalf("decisions = %v", got)
	}
	if _, err := executeFamilyNoChange(context.Background(), map[string]any{"family": "retry", "member": "fn:b.Retry", "reason": "b already returns as soon as ctx.Done closes"}); err == nil {
		t.Fatal("recorded with no turn open")
	}
}
