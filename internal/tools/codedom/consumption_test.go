package codedom

import (
	"context"
	"testing"

	"codenerd/internal/tools"
)

// A structural answer computed over the whole index consumed the index; one
// the provider scoped to a path or a ref says nothing about how much it read.
func TestRender_ReportsTheIndexAWholeIndexAnswerWasComputedOver(t *testing.T) {
	ctx, consumed := tools.WithConsumptionLog(context.Background())
	page := pageRequest{tool: "find_symbol", query: "q"}
	obs := Observation{Generation: 1, ScannedFiles: 120, ScannedBytes: 900_000}
	if _, err := page.render(ctx, []string{"row"}, "h", obs, "none"); err != nil {
		t.Fatal(err)
	}
	if got, ok := consumed(); !ok || got.Bytes != 900_000 || got.Items != 120 {
		t.Errorf("consumed %+v reported=%v, want 900000 bytes over 120 files", got, ok)
	}
}

func TestRender_AScopedAnswerReportsNoScan(t *testing.T) {
	ctx, consumed := tools.WithConsumptionLog(context.Background())
	page := pageRequest{tool: "package_outline", query: "q"}
	if _, err := page.render(ctx, []string{"row"}, "h", Observation{Generation: 1}, "none"); err != nil {
		t.Fatal(err)
	}
	if got, ok := consumed(); ok {
		t.Errorf("a scoped answer reported %+v; its scan is unknown", got)
	}
}
