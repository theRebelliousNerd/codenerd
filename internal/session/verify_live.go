package session

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"codenerd/internal/sessionlog"
)

// verifyLiveSeq numbers the verification runs a live view sees, so a run's
// result pairs with its start even when two overlap.
var verifyLiveSeq atomic.Uint64

// liveVerification shows a harness verification run (the turn's build and
// test gates, the importer expansion, coverage) in the delegated agent's live
// progress as it starts, and returns the call that reports how it ended. The
// gate runs after the model's last answer and can take minutes over large
// packages; without this the view still read "waiting for model" throughout.
func liveVerification(ctx context.Context, command []string) func(VerifyOutcome) {
	callID := fmt.Sprintf("verify-%d", verifyLiveSeq.Add(1))
	subject := strings.Join(command, " ")
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "tool_started", Tool: "verify", CallID: callID, Text: subject})
	return func(outcome VerifyOutcome) {
		status := strings.ToUpper(string(outcome))
		switch outcome {
		case VerifyPassed:
			status = "PASS"
		case VerifyFailed:
			status = "FAIL"
		}
		sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{
			Kind: sessionlog.EventToolResult, Tool: "verify", CallID: callID, Text: subject,
			Status: status, Failed: outcome == VerifyFailed,
		})
	}
}
