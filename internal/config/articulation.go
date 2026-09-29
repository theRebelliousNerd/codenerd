package config

import (
	"fmt"
	"sync/atomic"
)

// ArticulationConfig is the `articulation` section of .nerd/config.json.
// The legacy session-context block — the blackboard AssembleSystemPrompt
// appends when the JIT compiler is not what served the prompt — takes its
// size from here. The JIT path fits its own atom budget; this share is the
// fallback block's ceiling.
type ArticulationConfig struct {
	// SessionContextSharePercent is the percent of the configured prompt
	// budget (jit.token_budget, the budget the assembler already holds) the
	// session blackboard may occupy. The block is rendered whole — every
	// list, every line, and compressed history — and then clamped head and
	// tail. Safety constraints are written first, so the head keeps them;
	// history is written last, so the tail keeps its end. A percent, not a
	// character count: the same share grows when jit.token_budget is raised
	// toward the context window. 0 means the default.
	SessionContextSharePercent int `json:"session_context_share_percent,omitempty"`
}

// DefaultArticulationConfig is the articulation section with every field
// written down.
//
// The share is 25. The default prompt budget is 200000 tokens
// (DefaultJITConfig.TokenBudget). A quarter of that, at BytesPerToken (4),
// is 200000 characters — 50000 tokens, about six times the old flat 32 KiB
// (~8192 tokens). That 32 KiB was ~4% of the same default budget and stayed
// 32 KiB when the window grew; a share moves with the budget instead.
//
// The legacy prompt also carries the baseline identity, the kernel-injected
// rows and the parsed intent beside this block, and that path does not fit
// those sections inside this ceiling. 25% leaves the other three quarters
// for them. 100% would book the entire prompt budget for the blackboard
// alone and push the baseline past the window on the fallback path. 4%
// would reproduce the 32 KiB cap on the default budget.
func DefaultArticulationConfig() ArticulationConfig {
	return ArticulationConfig{SessionContextSharePercent: 25}
}

// GetArticulationConfig returns the articulation section with every absent
// field defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetArticulationConfig() ArticulationConfig {
	if c == nil || c.Articulation == nil {
		return DefaultArticulationConfig()
	}
	return c.Articulation.WithDefaults()
}

// WithDefaults fills every absent field from DefaultArticulationConfig.
// Zero means the key was absent: a share of 0 would withhold the blackboard
// on every prompt, which is not a setting, so it takes the default.
func (c ArticulationConfig) WithDefaults() ArticulationConfig {
	if c.SessionContextSharePercent == 0 {
		c.SessionContextSharePercent = DefaultArticulationConfig().SessionContextSharePercent
	}
	return c
}

// Check reports a share outside 1-100, addressed under prefix ("articulation").
func (c ArticulationConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	if c.SessionContextSharePercent < 1 || c.SessionContextSharePercent > 100 {
		return []Problem{{
			Severity: SeverityError,
			Path:     prefix + ".session_context_share_percent",
			Message:  fmt.Sprintf("%d is outside 1-100: a share is a percent of the prompt budget", c.SessionContextSharePercent),
			Fix:      "a percent between 1 and 100, or remove the key for the default",
		}}
	}
	return nil
}

// activeArticulation is the process-wide articulation policy, installed by
// LoadUserConfig the same way the observation limits are. The legacy prompt
// assembler reads it when it sizes the session blackboard, so a share in
// config.json applies without each assembler call site threading the struct
// through.
var activeArticulation atomic.Pointer[ArticulationConfig]

func init() {
	SetArticulationConfig(DefaultArticulationConfig())
}

// SetArticulationConfig installs the process-wide articulation policy.
// LoadUserConfig is the production caller; tests install and restore.
func SetArticulationConfig(c ArticulationConfig) {
	c = c.WithDefaults()
	activeArticulation.Store(&c)
}

// ResolvedArticulationConfig is the installed articulation policy. Without
// a load it is the defaults.
func ResolvedArticulationConfig() ArticulationConfig {
	if c := activeArticulation.Load(); c != nil {
		return *c
	}
	return DefaultArticulationConfig()
}
