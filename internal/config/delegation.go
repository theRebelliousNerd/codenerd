package config

import "fmt"

// DelegationConfig is the `delegation` section of .nerd/config.json: the
// thresholds the kernel decides a chat delegation's attempts with
// (policy/delegation.mg, delegation_move), and the bounds on what the judge is
// shown and may answer. The attempt cap is the persona's own,
// shard_profiles.<persona>.max_retries.
//
// Until 2026-09-23 an LLM judge's word decided whether a delegated mutation
// succeeded, the turn's kernel verdict was dropped on the way, and an LLM
// "shard selection advisor" picked who retried (sweep finding F5). The
// kernel's verdict decides now, and the judge can only withhold.
type DelegationConfig struct {
	// JudgeRejectConfidence is the confidence, as a percent, at or above
	// which the LLM judge's grounded defect withholds an attempt the kernel
	// verdict calls done. 0 lets any grounded defect withhold. A pointer
	// because 0 is meaningful.
	JudgeRejectConfidence *int `json:"judge_reject_confidence,omitempty"`
	// JudgeEvidenceRounds is how many further judge rounds may supply the
	// evidence an insufficient-evidence reply requested, over the same
	// frozen packet, before the attempt is unverifiable. 0 asks once. A
	// round re-asks the judge; it never re-runs the worker.
	JudgeEvidenceRounds *int `json:"judge_evidence_rounds,omitempty"`
	// JudgeEvidenceBytes bounds the evidence packet the judge is shown (the
	// diffs and the worker's own account share it), and each expansion.
	JudgeEvidenceBytes *int `json:"judge_evidence_bytes,omitempty"`
	// JudgeReplyBytes bounds a judge reply; a longer one breaks the reply
	// contract and the judgment is unavailable.
	JudgeReplyBytes *int `json:"judge_reply_bytes,omitempty"`
}

// Bounds on the judge's evidence and reply sizes.
const (
	minJudgeEvidenceBytes = 2048
	maxJudgeEvidenceBytes = 1 << 20
	minJudgeReplyBytes    = 512
	maxJudgeReplyBytes    = 1 << 20
	maxJudgeEvidenceRound = 5
)

// DefaultDelegationConfig is the delegation section with every field written
// down.
func DefaultDelegationConfig() DelegationConfig {
	reject, rounds, evidence, reply := 0, 1, 24000, 16384
	return DelegationConfig{
		JudgeRejectConfidence: &reject,
		JudgeEvidenceRounds:   &rounds,
		JudgeEvidenceBytes:    &evidence,
		JudgeReplyBytes:       &reply,
	}
}

// GetDelegationConfig returns the delegation section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetDelegationConfig() DelegationConfig {
	if c == nil || c.Delegation == nil {
		return DefaultDelegationConfig()
	}
	return c.Delegation.WithDefaults()
}

// WithDefaults fills every absent field from DefaultDelegationConfig.
func (c DelegationConfig) WithDefaults() DelegationConfig {
	d := DefaultDelegationConfig()
	if c.JudgeRejectConfidence == nil {
		c.JudgeRejectConfidence = d.JudgeRejectConfidence
	}
	if c.JudgeEvidenceRounds == nil {
		c.JudgeEvidenceRounds = d.JudgeEvidenceRounds
	}
	if c.JudgeEvidenceBytes == nil {
		c.JudgeEvidenceBytes = d.JudgeEvidenceBytes
	}
	if c.JudgeReplyBytes == nil {
		c.JudgeReplyBytes = d.JudgeReplyBytes
	}
	return c
}

// Check reports the contradictions in a delegation section, addressed under
// prefix ("delegation").
func (c DelegationConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	outside := func(key string, v, lo, hi int, fix string) {
		if v < lo || v > hi {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + "." + key,
				Message:  fmt.Sprintf("%d is outside %d-%d", v, lo, hi),
				Fix:      fix,
			})
		}
	}
	outside("judge_reject_confidence", *c.JudgeRejectConfidence, 0, 100, "an integer percent, or remove the key for the default")
	outside("judge_evidence_rounds", *c.JudgeEvidenceRounds, 0, maxJudgeEvidenceRound, "how many further judge rounds may supply requested evidence, or remove the key for the default")
	outside("judge_evidence_bytes", *c.JudgeEvidenceBytes, minJudgeEvidenceBytes, maxJudgeEvidenceBytes, "the evidence packet's byte budget, or remove the key for the default")
	outside("judge_reply_bytes", *c.JudgeReplyBytes, minJudgeReplyBytes, maxJudgeReplyBytes, "the largest judge reply accepted, in bytes, or remove the key for the default")
	return out
}

// Params are the delegation knobs the kernel's rules read, as config_param
// rows. Each key is declared config_param_required(/delegation, Key) beside
// the rule that reads it (policy/delegation.mg). The byte bounds are not
// decisions: the verifier enforces them before anything reaches the kernel.
func (c DelegationConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/delegation_judge_reject_confidence", Value: int64(*c.JudgeRejectConfidence)},
		{Key: "/delegation_judge_evidence_rounds", Value: int64(*c.JudgeEvidenceRounds)},
	}
}
