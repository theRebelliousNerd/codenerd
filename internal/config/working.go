package config

import "fmt"

// WorkingConfig is the `working` section of .nerd/config.json: the spans the
// working policy (internal/context/working_set.mg) decides a tool loop's
// regime, steering, stop and finalize with, and the bounds of its context
// ledger. Steve's rule, as for every section: "everything is configurable,
// not hard coded.. all from config.json" (sweep finding F8).
//
// They are stall thresholds, not run ceilings: a loop that makes progress runs
// as long as it needs (feedback: no run-level limits), and none of these
// counts tool calls. The repeat threshold was a config key once
// (core_limits.tool_loop_repeat_threshold) and was moved into the policy on
// 2026-09-18 so nobody could tune it into a count ceiling; it is here again
// with a floor that keeps it a repeat detector (at least 2 identical cycles).
type WorkingConfig struct {
	// LedgerCeilingBytes is the size at which the policy compacts the context
	// ledger -- the tool results a working request carries whole since the
	// last compaction (working_ledger_ceiling). It is a trigger, not a cut:
	// compaction moves stale and superseded results out first, then only
	// enough older live results to fit. Zero means "derive it" (see
	// GetWorkingConfig); an explicit value wins.
	LedgerCeilingBytes int `json:"ledger_ceiling_bytes,omitempty"`
	// LedgerKeepRounds is how many of the latest rounds a compaction leaves
	// in place while they are live (working_ledger_keep_rounds). Stale and
	// superseded results are not protected by the window.
	LedgerKeepRounds int `json:"ledger_keep_rounds,omitempty"`
	// NudgeRounds is the span after which a read task is nudged to conclude,
	// a change task to implement or verify (working_nudge_rounds).
	NudgeRounds int `json:"nudge_rounds,omitempty"`
	// CommitRounds is the span of reading after which a change task that has
	// written nothing is put in the commit regime (working_commit_rounds).
	CommitRounds int `json:"commit_rounds,omitempty"`
	// FinalizeRounds is the span after a write with no write and no
	// verification after which the harness asks for the conclusion and runs
	// the gates (working_finalize_rounds).
	FinalizeRounds int `json:"finalize_rounds,omitempty"`
	// StallRounds is the span of reading after which a change task that has
	// written nothing is stopped as stalled (working_stall_rounds).
	StallRounds int `json:"stall_rounds,omitempty"`
	// RepeatThreshold is how many identical deterministic trace cycles make a
	// loop (working_repeat_threshold). At least 2.
	RepeatThreshold int `json:"repeat_threshold,omitempty"`
	// RepairReadRounds is the reading a repair attempt gets before its reading
	// closes (working_repair_read_rounds).
	RepairReadRounds int `json:"repair_read_rounds,omitempty"`
	// StructuralTrials is how many structural queries a loop makes before raw
	// search opens (working_structural_trials).
	StructuralTrials int `json:"structural_trials,omitempty"`
	// StructuralMissLimit is how many structural queries without an answer
	// open raw search early (working_structural_miss_limit).
	StructuralMissLimit int `json:"structural_miss_limit,omitempty"`
}

// ledgerBytesPerToken converts the context window's token budget into the
// ledger's byte count. prompt.EstimateTokens counts (len+3)/4, and the
// ledger sums bytes, so one token of budget is four bytes of tool result.
// It is that estimator's unit conversion, not a config knob: a second knob
// would let the two disagree about how big a result is.
const ledgerBytesPerToken = 4

// LedgerCeilingBytesFromContext is the working-memory share of the serving
// model's input budget, in bytes:
//
//	context_window.max_tokens * context_window.working_reserve_percent / 100 * 4
//
// working_reserve_percent is the share InputBudget already names for working
// memory (Core + Atom + History + Working; internal/config/memory.go). A
// zero max_tokens or percent takes that field's default, so a partial
// context_window block still derives.
//
// jit.token_budget is not this number. It is the compiled system prompt's
// ceiling, and GetEffectiveJITConfig already clamps it to
// context_window.max_tokens. Using it here would book the same window twice:
// once as the prompt, again as the ledger.
func LedgerCeilingBytesFromContext(window ContextWindowConfig) int {
	def := DefaultContextWindowConfig()
	maxTokens := window.MaxTokens
	if maxTokens <= 0 {
		maxTokens = def.MaxTokens
	}
	percent := window.WorkingReservePercent
	if percent <= 0 {
		percent = def.WorkingReservePercent
	}
	return maxTokens * percent / 100 * ledgerBytesPerToken
}

// DefaultWorkingConfig is the working section with every field written down.
// The ledger ceiling is the working-memory share of the default context
// window (200000 tokens, 50%): 400000 bytes. The flat 65536-byte default
// (the 2026-09-22 16k-token economics point) compacted a 16-read working set
// on the first live nerd fix of main 2dadb513 (2026-09-29, session
// 20260929_052520): 14 live results left the request and the model spent the
// stall span recalling them. An explicit working.ledger_ceiling_bytes still
// wins over this derivation.
func DefaultWorkingConfig() WorkingConfig {
	return WorkingConfig{
		LedgerCeilingBytes:  LedgerCeilingBytesFromContext(DefaultContextWindowConfig()),
		LedgerKeepRounds:    2,
		NudgeRounds:         8,
		CommitRounds:        16,
		FinalizeRounds:      16,
		StallRounds:         24,
		RepeatThreshold:     2,
		RepairReadRounds:    1,
		StructuralTrials:    4,
		StructuralMissLimit: 2,
	}
}

// GetWorkingConfig returns the working section with every absent field
// defaulted. A nil receiver is the defaults. An absent or zero
// ledger_ceiling_bytes is derived from this config's context window, not
// from the default window baked into DefaultWorkingConfig: a user who set
// context_window.max_tokens and left the ledger key out gets a ceiling that
// matches the model they are serving. DefaultUserConfig stores a filled
// Working struct, so a non-zero ceiling there is the derived default and
// matches its default context window. A loaded file leaves the key at 0
// when it is absent (omitempty), which is the case this re-derives.
func (c *UserConfig) GetWorkingConfig() WorkingConfig {
	if c == nil {
		return DefaultWorkingConfig()
	}
	var raw WorkingConfig
	explicit := 0
	if c.Working != nil {
		raw = *c.Working
		explicit = c.Working.LedgerCeilingBytes
	}
	out := raw.WithDefaults()
	if explicit == 0 {
		out.LedgerCeilingBytes = LedgerCeilingBytesFromContext(c.GetContextWindowConfig())
	}
	return out
}

// WithDefaults fills every absent field from DefaultWorkingConfig.
func (c WorkingConfig) WithDefaults() WorkingConfig {
	d := DefaultWorkingConfig()
	for _, f := range []struct{ v, def *int }{
		{&c.LedgerCeilingBytes, &d.LedgerCeilingBytes},
		{&c.LedgerKeepRounds, &d.LedgerKeepRounds},
		{&c.NudgeRounds, &d.NudgeRounds},
		{&c.CommitRounds, &d.CommitRounds},
		{&c.FinalizeRounds, &d.FinalizeRounds},
		{&c.StallRounds, &d.StallRounds},
		{&c.RepeatThreshold, &d.RepeatThreshold},
		{&c.RepairReadRounds, &d.RepairReadRounds},
		{&c.StructuralTrials, &d.StructuralTrials},
		{&c.StructuralMissLimit, &d.StructuralMissLimit},
	} {
		if *f.v == 0 {
			*f.v = *f.def
		}
	}
	return c
}

// Check reports the contradictions in a working section, addressed under
// prefix ("working").
func (c WorkingConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	atLeast := func(field string, v, floor int, why string) {
		if v < floor {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + "." + field,
				Message:  fmt.Sprintf("%d is below %d: %s", v, floor, why),
				Fix:      fmt.Sprintf("a value of at least %d, or remove the key for the default", floor),
			})
		}
	}
	atLeast("ledger_ceiling_bytes", c.LedgerCeilingBytes, 4096, "below one large read, every round would compact and nothing would stay cached")
	atLeast("ledger_keep_rounds", c.LedgerKeepRounds, 1, "a compaction keeps at least the round the request answers")
	atLeast("nudge_rounds", c.NudgeRounds, 1, "a span is a count of rounds")
	atLeast("commit_rounds", c.CommitRounds, 1, "a span is a count of rounds")
	atLeast("finalize_rounds", c.FinalizeRounds, 1, "a span is a count of rounds")
	atLeast("stall_rounds", c.StallRounds, 1, "a span is a count of rounds")
	atLeast("repeat_threshold", c.RepeatThreshold, 2, "one cycle is not a repeat; below 2 every turn would stop at its first round")
	atLeast("repair_read_rounds", c.RepairReadRounds, 1, "a repair attempt reads its failure at least once")
	atLeast("structural_trials", c.StructuralTrials, 1, "a trial is a count of queries")
	atLeast("structural_miss_limit", c.StructuralMissLimit, 1, "a limit is a count of queries")
	if c.StallRounds < c.CommitRounds {
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     prefix + ".stall_rounds",
			Message:  fmt.Sprintf("%d is below commit_rounds (%d): a change task would be stopped before its reading closes", c.StallRounds, c.CommitRounds),
			Fix:      "stall_rounds of at least commit_rounds",
		})
	}
	return out
}

// Params are the working spans the policy reads, as config_param rows; the
// keys are declared config_param_required(/working, Key) in working_set.mg.
func (c WorkingConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/working_ledger_ceiling", Value: int64(c.LedgerCeilingBytes)},
		{Key: "/working_ledger_keep_rounds", Value: int64(c.LedgerKeepRounds)},
		{Key: "/working_nudge_rounds", Value: int64(c.NudgeRounds)},
		{Key: "/working_commit_rounds", Value: int64(c.CommitRounds)},
		{Key: "/working_finalize_rounds", Value: int64(c.FinalizeRounds)},
		{Key: "/working_stall_rounds", Value: int64(c.StallRounds)},
		{Key: "/working_repeat_threshold", Value: int64(c.RepeatThreshold)},
		{Key: "/working_repair_read_rounds", Value: int64(c.RepairReadRounds)},
		{Key: "/working_structural_trials", Value: int64(c.StructuralTrials)},
		{Key: "/working_structural_miss_limit", Value: int64(c.StructuralMissLimit)},
	}
}
