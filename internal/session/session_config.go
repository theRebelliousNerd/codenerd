package session

import (
	"fmt"

	"codenerd/internal/config"
	"codenerd/internal/logging"
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
	return config.SessionPolicy{StepPlanMinSites: minSites, RepairMaxAttempts: c.sessionRepairMaxAttempts()}.Params()
}

// sessionRepairMaxAttempts is the attempt cap repair_exhausted reads
// (session.repair_max_attempts). Repair is always bounded: a zero field takes
// the section's default, never "unbounded".
func (c ExecutorConfig) sessionRepairMaxAttempts() int {
	if c.RepairMaxAttempts > 0 {
		return c.RepairMaxAttempts
	}
	return defaultSessionPolicy.RepairMaxAttempts
}

// ensureSessionParams puts the executor's thresholds into the kernel as
// config_param rows before a rule that reads them is asked
// (config.EnsureParams).
func (e *Executor) ensureSessionParams() {
	if e.kernel == nil {
		return
	}
	if err := config.EnsureParams(e.kernel, e.configSnapshot().sessionParams()); err != nil {
		logging.Get(logging.CategorySession).Error("session policy params were not asserted: %v", err)
	}
}
