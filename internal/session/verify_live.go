package session

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"codenerd/internal/sessionlog"
	"codenerd/internal/turntime"
)

// verifyLiveSeq numbers the verification runs a live view sees, so a run's
// result pairs with its start even when two overlap.
var verifyLiveSeq atomic.Uint64

// liveVerification shows a harness verification run (the turn's build and
// test gates, the importer expansion, coverage) in the delegated agent's live
// progress as it starts, and returns the call that reports how it ended. The
// gate runs after the model's last answer and can take minutes over large
// packages; without this the view still read "waiting for model" throughout.
//
// note says what the gate chose to run, when it chose (the importer gate's
// test selection: how many packages and tests, or why it fell back to all);
// it leads the subject, so the view names the choice before the command.
func liveVerification(ctx context.Context, command []string, note string) func(VerifyOutcome) {
	callID := fmt.Sprintf("verify-%d", verifyLiveSeq.Add(1))
	subject := strings.Join(command, " ")
	if note = strings.TrimSpace(note); note != "" {
		subject = note + ": " + subject
	}
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

// verificationKind names what a verification subprocess is for on the turn's
// time ledger. A caller that knows (the importer expansion, coverage, pinning,
// the final suite) names it on the context; otherwise the command says: go
// build, go vet, go test, and go test with a cover profile.
func verificationKind(ctx context.Context, name string, args []string) string {
	if kind := turntime.VerifyKindFrom(ctx); kind != "" {
		return kind
	}
	if name != "go" || len(args) == 0 {
		return turntime.KindOther
	}
	switch args[0] {
	case "build":
		return turntime.KindBuild
	case "vet":
		return turntime.KindVet
	case "test":
		for _, a := range args[1:] {
			if strings.HasPrefix(a, "-coverprofile") {
				return turntime.KindCoverage
			}
		}
		return turntime.KindTests
	}
	return turntime.KindOther
}

// verificationSpan opens one verification run on the turn's time ledger and
// returns the call that closes it. Every harness verification subprocess goes
// through runVerificationCommand, which opens it, so a run is timed once
// whichever gate started it and wherever it sits (inside a repair round too:
// the ledger charges it to verification, see package turntime).
func verificationSpan(ctx context.Context, name string, args []string) func() {
	return turntime.From(ctx).Verify(verificationKind(ctx, name, args))
}
