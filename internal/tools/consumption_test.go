package tools

import (
	"context"
	"testing"
)

func TestConsumption_ReportsSumAcrossPasses(t *testing.T) {
	ctx, read := WithConsumptionLog(context.Background())
	ReportConsumed(ctx, 100, 2)
	ReportConsumed(ctx, 50, 1)
	got, ok := read()
	if !ok || got.Bytes != 150 || got.Items != 3 {
		t.Errorf("consumption %+v reported=%v, want 150 bytes over 3 items", got, ok)
	}
}

func TestConsumption_AToolThatReportsNothingIsNotReported(t *testing.T) {
	_, read := WithConsumptionLog(context.Background())
	if _, ok := read(); ok {
		t.Error("a log nothing wrote to claims a measurement")
	}
}

func TestConsumption_AMeasuredZeroIsReported(t *testing.T) {
	ctx, read := WithConsumptionLog(context.Background())
	ReportConsumed(ctx, 0, 0)
	if got, ok := read(); !ok || got.Bytes != 0 {
		t.Errorf("a search of an empty tree is a measured zero, got %+v reported=%v", got, ok)
	}
}

func TestConsumption_NoLogRecordsNothing(t *testing.T) {
	ReportConsumed(context.Background(), 10, 1) // must not panic
}

func TestConsumption_ANegativeReportCannotShrinkATotal(t *testing.T) {
	ctx, read := WithConsumptionLog(context.Background())
	ReportConsumed(ctx, 100, 1)
	ReportConsumed(ctx, -500, -3)
	if got, _ := read(); got.Bytes != 100 || got.Items != 1 {
		t.Errorf("negative report changed the total: %+v", got)
	}
}
