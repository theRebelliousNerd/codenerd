package config

import (
	"fmt"
	"time"
)

// SessionConfig is the `session` section of .nerd/config.json: the knobs the
// session executor reads for one turn. Steve's rule for it, as for the
// campaign section: "everything is configurable, not hard coded.. all from
// config.json".
//
// Before this section the executor's thresholds were Go constants the file
// could not reach: the step planner's 12-step cap and 2-minute bound, the
// repair episode's 3 attempts (an ExecutorConfig field nothing set), the
// 5-minute tool bound and final-answer reserve, the 6-message and
// 24,000-character history window. The session
// executor is built from this section on every boot (internal/system), and
// the thresholds a rule decides with reach the kernel as config_param rows
// (Params).
//
// The post-edit gates (build, tests, critic) are deliberately not here: they
// are completion gates, and a switch in the file that turns one off widens
// what a turn may claim.
//
// Counts left at 0 and durations left empty take the default below. The
// history window is a pointer because 0 is meaningful for it: no prior turn
// reaches the model.
type SessionConfig struct {
	// --- dividing a change task into steps ---

	// StepPlanMinSites is how many distinct edit sites (a file the brief
	// names, or a file:line) a write-oriented brief must name before the
	// executive spends a planning call on it (policy:
	// turn_needs_step_plan). Measured 2026-09-21: 13 planning calls in one
	// campaign run, every one on a one-site brief, every one answered "one
	// step".
	StepPlanMinSites int `json:"step_plan_min_sites,omitempty"`
	// StepPlanMaxSteps bounds a plan: a task that divides into more is not
	// one turn's work, and the executive runs the first steps and reports.
	StepPlanMaxSteps int `json:"step_plan_max_steps,omitempty"`
	// StepPlanTimeout bounds one planning request.
	StepPlanTimeout string `json:"step_plan_timeout,omitempty"`

	// --- repairing a red build or test run ---

	// RepairMaxAttempts is how many repair attempts one build or test repair
	// episode may make before it gives up.
	RepairMaxAttempts int `json:"repair_max_attempts,omitempty"`

	// --- the turn's tools and its answer ---

	// ToolTimeout bounds one tool execution.
	ToolTimeout string `json:"tool_timeout,omitempty"`
	// FinalAnswerReserve keeps the tail of a turn with a deadline for one
	// tool-free completion, so a turn that explored to its deadline still
	// answers.
	FinalAnswerReserve string `json:"final_answer_reserve,omitempty"`

	// --- conversation memory ---

	// HistoryTurnWindow is how many prior conversation messages reach the
	// generating model (6 = three exchanges). 0 sends none.
	HistoryTurnWindow *int `json:"history_turn_window,omitempty"`
	// HistoryCharBudget bounds the characters of prior-turn text sent to the
	// generating model; the oldest exchanges go first, never half of one.
	HistoryCharBudget int `json:"history_char_budget,omitempty"`
}

// DefaultSessionConfig is the session section with every field written down.
func DefaultSessionConfig() SessionConfig {
	window := 6
	return SessionConfig{
		StepPlanMinSites:   2,
		StepPlanMaxSteps:   12,
		StepPlanTimeout:    "2m",
		RepairMaxAttempts:  3,
		ToolTimeout:        "5m",
		FinalAnswerReserve: "5m",
		HistoryTurnWindow:  &window,
		HistoryCharBudget:  24000,
	}
}

// GetSessionConfig returns the session section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetSessionConfig() SessionConfig {
	if c == nil || c.Session == nil {
		return DefaultSessionConfig()
	}
	return c.Session.WithDefaults()
}

// WithDefaults fills every absent field from DefaultSessionConfig.
func (c SessionConfig) WithDefaults() SessionConfig {
	d := DefaultSessionConfig()
	intOr := func(v *int, def int) {
		if *v == 0 {
			*v = def
		}
	}
	strOr := func(v *string, def string) {
		if *v == "" {
			*v = def
		}
	}
	intOr(&c.StepPlanMinSites, d.StepPlanMinSites)
	intOr(&c.StepPlanMaxSteps, d.StepPlanMaxSteps)
	strOr(&c.StepPlanTimeout, d.StepPlanTimeout)
	intOr(&c.RepairMaxAttempts, d.RepairMaxAttempts)
	strOr(&c.ToolTimeout, d.ToolTimeout)
	strOr(&c.FinalAnswerReserve, d.FinalAnswerReserve)
	if c.HistoryTurnWindow == nil {
		c.HistoryTurnWindow = d.HistoryTurnWindow
	}
	intOr(&c.HistoryCharBudget, d.HistoryCharBudget)
	return c
}

// SessionPolicy is a SessionConfig resolved for the executor: defaults
// filled, durations parsed. Resolve is the only way to make one, so a policy
// in hand has passed Check.
type SessionPolicy struct {
	StepPlanMinSites   int
	StepPlanMaxSteps   int
	StepPlanTimeout    time.Duration
	RepairMaxAttempts  int
	ToolTimeout        time.Duration
	FinalAnswerReserve time.Duration
	HistoryTurnWindow  int
	HistoryCharBudget  int
}

// Resolve defaults, checks and parses the section. Every problem Check finds
// is an error here.
func (c SessionConfig) Resolve() (SessionPolicy, error) {
	c = c.WithDefaults()
	if problems := c.Check("session"); len(problems) > 0 {
		msg := fmt.Sprintf("the session config has %d error(s):", len(problems))
		for _, p := range problems {
			msg += "\n  " + p.String()
		}
		return SessionPolicy{}, fmt.Errorf("%s", msg)
	}
	d := func(s string) time.Duration {
		v, _ := time.ParseDuration(s) // Check parsed every one of these
		return v
	}
	return SessionPolicy{
		StepPlanMinSites:   c.StepPlanMinSites,
		StepPlanMaxSteps:   c.StepPlanMaxSteps,
		StepPlanTimeout:    d(c.StepPlanTimeout),
		RepairMaxAttempts:  c.RepairMaxAttempts,
		ToolTimeout:        d(c.ToolTimeout),
		FinalAnswerReserve: d(c.FinalAnswerReserve),
		HistoryTurnWindow:  *c.HistoryTurnWindow,
		HistoryCharBudget:  c.HistoryCharBudget,
	}, nil
}

// Check reports the contradictions in a session section, addressed under
// prefix ("session"). Absent fields are defaulted first, so only what the
// file says can be wrong.
func (c SessionConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	add := func(field, msg, fix string) {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + "." + field, Message: msg, Fix: fix})
	}
	for _, f := range []struct {
		name string
		v    int
	}{
		{"step_plan_min_sites", c.StepPlanMinSites},
		{"step_plan_max_steps", c.StepPlanMaxSteps},
		{"repair_max_attempts", c.RepairMaxAttempts},
		{"history_char_budget", c.HistoryCharBudget},
	} {
		if f.v < 1 {
			add(f.name, fmt.Sprintf("%d is below 1", f.v), "a count of at least 1, or remove the key for the default")
		}
	}
	if c.StepPlanMinSites >= 1 && c.StepPlanMaxSteps >= 1 && c.StepPlanMaxSteps < c.StepPlanMinSites {
		add("step_plan_max_steps", fmt.Sprintf("%d is below step_plan_min_sites (%d): no plan a brief qualifies for could be run", c.StepPlanMaxSteps, c.StepPlanMinSites), "a cap at least step_plan_min_sites")
	}
	if *c.HistoryTurnWindow < 0 {
		add("history_turn_window", fmt.Sprintf("%d is negative", *c.HistoryTurnWindow), "0 to send no prior turn, or a message count")
	}
	for _, f := range []struct {
		name string
		v    string
	}{
		{"step_plan_timeout", c.StepPlanTimeout},
		{"tool_timeout", c.ToolTimeout},
		{"final_answer_reserve", c.FinalAnswerReserve},
	} {
		d, err := time.ParseDuration(f.v)
		switch {
		case err != nil:
			add(f.name, fmt.Sprintf("%q is not a duration: %v", f.v, err), `a Go duration such as "30s", "5m" or "1500ms"`)
		case d <= 0:
			add(f.name, fmt.Sprintf("%q is not positive", f.v), "a positive duration, or remove the key for the default")
		}
	}
	return out
}

// Params are the session knobs the kernel's rules read, as config_param
// rows. Each key is declared config_param_required(/session, Key) next to
// the rules that read it.
func (p SessionPolicy) Params() []Param {
	return []Param{
		{Key: "/session_step_plan_min_sites", Value: int64(p.StepPlanMinSites)},
	}
}
