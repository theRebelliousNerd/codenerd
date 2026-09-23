package session

import (
	"fmt"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// defaultSessionPolicy is the `session` section's defaults, resolved: what an
// executor built without the user's file runs on (a test, a tool that builds
// its own executor), and what a zero field in a hand-built ExecutorConfig
// falls back to. The values live in config.DefaultSessionConfig and nowhere
// else.
var defaultSessionPolicy = func() config.SessionPolicy {
	p, err := config.DefaultSessionConfig().Resolve()
	if err != nil {
		panic(fmt.Sprintf("the session section's own defaults do not resolve: %v", err))
	}
	return p
}()

// ExecutorConfigFrom is the executor's config for a resolved session section.
// The post-edit gates are on and the safety gate is enabled: those are not
// the file's to switch off (see config.SessionConfig).
func ExecutorConfigFrom(p config.SessionPolicy) ExecutorConfig {
	return ExecutorConfig{
		ToolTimeout:        p.ToolTimeout,
		RepairMaxAttempts:  p.RepairMaxAttempts,
		FinalAnswerReserve: p.FinalAnswerReserve,
		EnableSafetyGate:   true,
		TokenBudget:        DefaultTokenBudget(),
		HistoryTurnWindow:  p.HistoryTurnWindow,
		HistoryCharBudget:  p.HistoryCharBudget,
		StepPlanMinSites:   p.StepPlanMinSites,
		StepPlanMaxSteps:   p.StepPlanMaxSteps,
		StepPlanTimeout:    p.StepPlanTimeout,
		// On by default: the failure this prevents (confident, non-compiling
		// edits reported as complete) is silent, and a default-off guard against
		// a silent failure protects nobody.
		VerifyBuildAfterEdits:  true,
		VerifyTestsAfterEdits:  true,
		CriticReviewAfterEdits: true,
	}
}

// sessionParams are the executor's thresholds the policy decides with, as
// config_param rows. A zero field in a hand-built ExecutorConfig takes the
// section's default, as every other reader of that field does.
func (c ExecutorConfig) sessionParams() []config.Param {
	minSites := c.StepPlanMinSites
	if minSites <= 0 {
		minSites = defaultSessionPolicy.StepPlanMinSites
	}
	return config.SessionPolicy{StepPlanMinSites: minSites}.Params()
}

// ensureSessionParams puts the executor's thresholds into the kernel as
// config_param rows before a rule that reads them is asked. The kernel is
// shared and outlives a config, and a rule over an absent threshold derives
// nothing, so a row that is missing or holds another value is replaced; one
// that already holds this value is left alone, since config_param is
// replicated into every shard and re-asserting it re-evaluates all of them.
func (e *Executor) ensureSessionParams() {
	if e.kernel == nil {
		return
	}
	held := map[string]int64{}
	if rows, err := e.kernel.Query(config.ConfigParamPredicate); err == nil {
		for _, f := range rows {
			if len(f.Args) != 2 {
				continue
			}
			if v, ok := f.Args[1].(int64); ok {
				held[types.ExtractString(f.Args[0])] = v
			}
		}
	}
	for _, f := range config.ParamFacts(e.configSnapshot().sessionParams()) {
		key := types.ExtractString(f.Args[0])
		if v, ok := held[key]; ok && v == f.Args[1].(int64) {
			continue
		}
		_ = e.kernel.RetractFact(types.Fact{Predicate: f.Predicate, Args: []any{f.Args[0]}})
		if err := e.kernel.Assert(f); err != nil {
			logging.Get(logging.CategorySession).Error("session policy param %s was not asserted: %v", key, err)
		}
	}
}
