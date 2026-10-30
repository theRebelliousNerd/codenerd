package config

import (
	"math"
	"path/filepath"
	"strings"
	"time"
)

// MissionConfig bounds an explicitly started mission invocation. It does not
// change the lifetime of ordinary turns or ordinary recurse runs.
type MissionConfig struct {
	MaxCycles        int      `json:"max_cycles,omitempty"`
	MaxSeconds       int      `json:"max_seconds,omitempty"`
	MaxSpendMicroUSD int64    `json:"max_spend_microusd,omitempty"`
	NoProgressCycles int      `json:"no_progress_cycles,omitempty"`
	Concurrency      int      `json:"concurrency,omitempty"`
	ContextBytes     int      `json:"context_bytes,omitempty"`
	ProtectedPaths   []string `json:"protected_paths,omitempty"`
}

func DefaultMissionConfig() MissionConfig {
	return MissionConfig{MaxCycles: 8, MaxSeconds: 3600, MaxSpendMicroUSD: 5000000,
		NoProgressCycles: 2, Concurrency: 1, ContextBytes: 65536,
		ProtectedPaths: []string{".git", ".gitignore", ".mangleignore", ".github", ".nerd", ".claude", ".codex", ".agents", "agents.md", "AGENTS.md",
			"internal/core/defaults", "internal/core/mangle_updates.go", "internal/core/kernel_validation.go",
			"internal/core/virtual_store_constitution.go", "internal/gates", "internal/evidence",
			"internal/config/mission.go", "internal/campaign/recurse_mission.go", "internal/campaign/recurse_mission_policy.go",
			"internal/campaign/recurse_mission_storage.go", "internal/campaign/recurse_mission_meter.go",
			"internal/campaign/recurse_cycle.go", "internal/campaign/recurse_policy.go", "internal/campaign/recurse_git.go",
			"internal/campaign/orchestrator_attempt_writes.go", "internal/campaign/orchestrator_utils.go",
			"internal/campaign/recurse_mission_scope.go", "internal/session/write_guard.go",
			"internal/session/build_verify.go", "internal/session/test_verify.go", "internal/session/critic.go",
			"internal/campaign/orchestrator_acceptance.go", "internal/campaign/risk_gate_contract.go"}}
}

func (c MissionConfig) WithDefaults() MissionConfig {
	d := DefaultMissionConfig()
	if c.MaxCycles == 0 {
		c.MaxCycles = d.MaxCycles
	}
	if c.MaxSeconds == 0 {
		c.MaxSeconds = d.MaxSeconds
	}
	if c.MaxSpendMicroUSD == 0 {
		c.MaxSpendMicroUSD = d.MaxSpendMicroUSD
	}
	if c.NoProgressCycles == 0 {
		c.NoProgressCycles = d.NoProgressCycles
	}
	if c.Concurrency == 0 {
		c.Concurrency = d.Concurrency
	}
	if c.ContextBytes == 0 {
		c.ContextBytes = d.ContextBytes
	}
	if c.ProtectedPaths == nil {
		c.ProtectedPaths = d.ProtectedPaths
	}
	return c
}

func (c *UserConfig) GetMissionConfig() MissionConfig {
	if c == nil || c.Mission == nil {
		return DefaultMissionConfig()
	}
	return c.Mission.WithDefaults()
}

func (c MissionConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	for _, p := range c.Params() {
		if p.Value <= 0 {
			out = append(out, Problem{Severity: SeverityError, Path: prefix,
				Message: p.Key + " must be positive", Fix: "use a positive value or omit for the default"})
		}
	}
	if c.Concurrency != 1 {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + ".concurrency",
			Message: "mission adoption requires serial execution", Fix: "set concurrency to 1"})
	}
	if int64(c.MaxSeconds) > math.MaxInt64/int64(time.Second) {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + ".max_seconds", Message: "duration cannot be represented", Fix: "use a representable positive duration"})
	}
	for _, path := range c.ProtectedPaths {
		clean := filepath.ToSlash(filepath.Clean(path))
		if strings.TrimSpace(path) == "" || filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, "../") {
			out = append(out, Problem{Severity: SeverityError, Path: prefix + ".protected_paths", Message: "paths must be workspace-relative", Fix: "use workspace-relative paths"})
		}
	}
	return out
}

func (c MissionConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/mission_max_cycles", Value: int64(c.MaxCycles)},
		{Key: "/mission_max_seconds", Value: int64(c.MaxSeconds)},
		{Key: "/mission_max_spend_microusd", Value: c.MaxSpendMicroUSD},
		{Key: "/mission_no_progress_cycles", Value: int64(c.NoProgressCycles)},
		{Key: "/mission_concurrency", Value: int64(c.Concurrency)},
		{Key: "/mission_context_bytes", Value: int64(c.ContextBytes)},
	}
}
