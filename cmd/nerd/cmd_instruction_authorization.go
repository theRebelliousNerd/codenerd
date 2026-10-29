package main

import (
	"context"
	"path/filepath"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/turnjudge"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"

	"go.uber.org/zap"
)

// `nerd run "fix X, then commit it"` names the commit just as a chat turn
// does, and the delegated coder must be able to finish it. The chat reads the
// message with Jev beside perception (cmd/nerd/chat/jev_turn.go); the one-shot
// command asks the same question about the instruction and carries the answer
// the same way, on the context its delegation runs under
// (perception.Authorization). It asks only the judgments, and takes back every
// fact it asserted before routing reads the kernel: this command's routing is
// perception's, as it was before.

// instructionAuthorization is what the instruction allowed beyond editing;
// zero when Jev is not configured or its call fails (a missing judgment is no
// evidence, never a yes).
func instructionAuthorization(ctx context.Context, kernel types.Kernel, input string) perception.Authorization {
	cfg, err := config.LoadUserConfig(filepath.Join(workspace, ".nerd", "config.json"))
	if err != nil || cfg == nil || cfg.TypeSafe == nil || !cfg.TypeSafe.Uses().TurnJudgments {
		return perception.Authorization{}
	}
	client, err := typesafe.FromConfig(cfg.TypeSafe)
	if err != nil {
		logger.Warn("Request authorization: Jev client unavailable; nothing beyond the edits is authorized", zap.Error(err))
		return perception.Authorization{}
	}
	if client == nil {
		return perception.Authorization{}
	}
	return authorizationFromJev(ctx, client, cfg.TypeSafe, kernel, input)
}

// authorizationFromJev asks Jev about the instruction, lets the kernel derive
// the authorization from the judgments, reads it back, and retracts the
// judgments.
func authorizationFromJev(ctx context.Context, asker turnjudge.Asker, ts *config.TypeSafeConfig, kernel types.Kernel, input string) perception.Authorization {
	if kernel == nil {
		return perception.Authorization{}
	}
	jctx, cancel := context.WithTimeout(ctx, ts.Uses().TurnJudgmentTimeout)
	defer cancel()
	res, err := turnjudge.Judge(jctx, asker, turnjudge.Input{Message: input, RecentTurns: ts.Uses().RecentTurns})
	if err != nil {
		logger.Warn("Request authorization: no evidence from Jev; nothing beyond the edits is authorized", zap.Error(err))
		return perception.Authorization{}
	}
	if err := config.EnsureParams(kernel, ts.Params()); err != nil {
		logger.Warn("Request authorization: Jev thresholds not asserted", zap.Error(err))
		return perception.Authorization{}
	}
	defer func() {
		for _, pred := range turnjudge.Predicates {
			_ = kernel.Retract(pred)
		}
	}()
	if err := kernel.AssertBatch(res.Facts()); err != nil {
		logger.Warn("Request authorization: judgments not asserted", zap.Error(err))
		return perception.Authorization{}
	}
	return turnjudge.AuthorizationFrom(kernel)
}
