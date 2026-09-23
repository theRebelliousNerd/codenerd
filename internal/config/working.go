package config

import "fmt"

// WorkingConfig is the `working` section of .nerd/config.json: the spans the
// working policy (internal/context/working_set.mg) decides a tool loop's
// regime, steering, stop and finalize with, and the bounds of the context it
// selects. Steve's rule, as for every section: "everything is configurable,
// not hard coded.. all from config.json" (sweep finding F8).
//
// They are stall thresholds, not run ceilings: a loop that makes progress runs
// as long as it needs (feedback: no run-level limits), and none of these
// counts tool calls. The repeat threshold was a config key once
// (core_limits.tool_loop_repeat_threshold) and was moved into the policy on
// 2026-09-18 so nobody could tune it into a count ceiling; it is here again
// with a floor that keeps it a repeat detector (at least 2 identical cycles).
type WorkingConfig struct {
	// TranscriptRounds is how many of the last native call/result rounds a
	// request keeps in the provider transcript (working_transcript_rounds).
	TranscriptRounds int `json:"transcript_rounds,omitempty"`
	// TranscriptSlack is how many rounds past TranscriptRounds the kept span
	// may grow before it is cut back, so the cached prefix holds between cuts
	// (working_transcript_slack). Zero is a value -- the every-round slide --
	// so absence is nil, not 0.
	TranscriptSlack *int `json:"transcript_slack,omitempty"`
	// SectionCeilingBytes bounds the selected-context section of one request
	// (working_section_ceiling); a read that falls out is one recall away.
	SectionCeilingBytes int `json:"section_ceiling_bytes,omitempty"`
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

// DefaultWorkingConfig is the working section with every field written down:
// the values the policy carried as literals until 2026-09-23.
func DefaultWorkingConfig() WorkingConfig {
	slack := 3
	return WorkingConfig{
		TranscriptRounds:    3,
		TranscriptSlack:     &slack,
		SectionCeilingBytes: 131072,
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
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetWorkingConfig() WorkingConfig {
	if c == nil || c.Working == nil {
		return DefaultWorkingConfig()
	}
	return c.Working.WithDefaults()
}

// WithDefaults fills every absent field from DefaultWorkingConfig.
func (c WorkingConfig) WithDefaults() WorkingConfig {
	d := DefaultWorkingConfig()
	for _, f := range []struct{ v, def *int }{
		{&c.TranscriptRounds, &d.TranscriptRounds},
		{&c.SectionCeilingBytes, &d.SectionCeilingBytes},
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
	if c.TranscriptSlack == nil {
		c.TranscriptSlack = d.TranscriptSlack
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
	atLeast("transcript_rounds", c.TranscriptRounds, 1, "a request must keep the round it answers")
	atLeast("transcript_slack", *c.TranscriptSlack, 0, "slack is a count of rounds")
	atLeast("section_ceiling_bytes", c.SectionCeilingBytes, 4096, "the working set refuses a smaller section (WorkingSet.SectionCeiling)")
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
		{Key: "/working_transcript_rounds", Value: int64(c.TranscriptRounds)},
		{Key: "/working_transcript_slack", Value: int64(*c.TranscriptSlack)},
		{Key: "/working_section_ceiling", Value: int64(c.SectionCeilingBytes)},
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
