package session

import (
	"context"

	"codenerd/internal/types"
)

// GeneratedToolReceipts returns copies of the bridge's actual returned values,
// in call order. A receipt may be partial, rejected, replayed or malformed;
// callers must inspect its outcomes beside the tool/turn errors. These values
// never establish permission, recognized write effects or task acceptance.
// Raw payloads remain private to this explicit accessor, not automatic result
// serialization or model context. Like the result's other measurements, read
// them after the owning call has completed.
func (r *ExecutionResult) GeneratedToolReceipts() []types.GeneratedToolReceipt {
	if r == nil {
		return nil
	}
	return append([]types.GeneratedToolReceipt(nil), r.generatedToolReceipts...)
}

type generatedReceiptLogKey struct{}

// The accounting wrapper owns this call-local log. Dispatch is synchronous;
// recording happens only after the generated bridge actually returns.
func withGeneratedReceiptLog(ctx context.Context) (context.Context, func() []types.GeneratedToolReceipt) {
	log := new([]types.GeneratedToolReceipt)
	return context.WithValue(ctx, generatedReceiptLogKey{}, log), func() []types.GeneratedToolReceipt {
		return append([]types.GeneratedToolReceipt(nil), (*log)...)
	}
}

func recordGeneratedReceipt(ctx context.Context, receipt types.GeneratedToolReceipt) {
	if log, ok := ctx.Value(generatedReceiptLogKey{}).(*[]types.GeneratedToolReceipt); ok && log != nil {
		*log = append(*log, receipt)
	}
}
