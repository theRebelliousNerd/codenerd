package broker

import (
	"fmt"
	"sync"
)

// Calibration bounds and tuning. A learned ratio is clamped into
// [minRatio, maxRatio] so that one malformed observation cannot poison the
// estimate for the rest of the session.
const (
	// DefaultSeedRatio is the chars-per-token starting point for a model with
	// no observations yet.
	//
	// Its precise value matters much less than it looks. The first provider
	// response replaces it outright (alpha is 1.0 at n=0), so it governs only
	// the very first request against a new model. It is set slightly below the
	// familiar 4.0 because measure() includes framing overhead in its character
	// count, and because source code — the dominant content here — tokenizes
	// more densely than prose.
	DefaultSeedRatio = 3.5

	minRatio = 1.0
	maxRatio = 20.0

	// minObservationChars ignores samples too small to carry signal. A 40-char
	// request is dominated by fixed framing and would drag the ratio around
	// without saying anything about how content tokenizes.
	minObservationChars = 256

	// minAlpha floors the exponential weight so the ratio keeps tracking a
	// model whose content mix shifts mid-session (prose turn, then a 3000-line
	// diff) instead of freezing on early history.
	minAlpha = 0.15
)

// Observation is one datum relating measured request size to the provider's
// reported token count for that same request.
type Observation struct {
	Model string
	// Chars is the value measure() produced for the request.
	Chars int
	// ActualInputTokens is the provider's reported input token count, with any
	// cached-content tokens added back in. Without that addition a cache hit
	// looks like a request that tokenized ten times more densely than it did,
	// and the ratio is corrupted by exactly the optimization that was working.
	ActualInputTokens int
}

// Calibrator learns a chars-per-token ratio per model from observed provider
// actuals.
//
// This is what makes the estimator different in kind from the constant it
// replaces. The old counter divided by 4.0 forever and had no path to learn,
// because no component in the codebase ever compared its estimate against the
// provider's reported truth. Here every completed response is a labelled
// training example, and the error shrinks as a session runs.
//
// Safe for concurrent use.
type Calibrator struct {
	mu     sync.RWMutex
	seed   float64
	states map[string]*ratioState
}

type ratioState struct {
	ratio float64
	n     int
}

// NewCalibrator returns a calibrator seeded at DefaultSeedRatio.
func NewCalibrator() *Calibrator { return NewCalibratorWithSeed(DefaultSeedRatio) }

// NewCalibratorWithSeed returns a calibrator with an explicit seed ratio.
// A non-positive or out-of-range seed falls back to DefaultSeedRatio rather
// than producing a counter that divides by zero.
func NewCalibratorWithSeed(seed float64) *Calibrator {
	if seed < minRatio || seed > maxRatio {
		seed = DefaultSeedRatio
	}
	return &Calibrator{seed: seed, states: make(map[string]*ratioState)}
}

// Ratio returns the current chars-per-token ratio for model and the confidence
// that ratio carries.
func (c *Calibrator) Ratio(model string) (float64, Confidence) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	st, ok := c.states[model]
	if !ok || st.n == 0 {
		return c.seed, ConfidenceSeeded
	}
	return st.ratio, ConfidenceCalibrated
}

// Observations returns how many provider actuals have informed model's ratio.
func (c *Calibrator) Observations(model string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if st, ok := c.states[model]; ok {
		return st.n
	}
	return 0
}

// Observe folds one provider actual into the model's ratio.
//
// Rejected silently (the caller is on a response path and has nothing useful to
// do with an error): samples below minObservationChars, non-positive token
// counts, and observed ratios outside the sane band. The last of these is the
// important guard — a provider that reports usage in some other unit, or a
// response whose usage block is partially populated, would otherwise drag the
// ratio somewhere that makes every subsequent admission decision wrong.
func (c *Calibrator) Observe(obs Observation) {
	if obs.Model == "" || obs.Chars < minObservationChars || obs.ActualInputTokens <= 0 {
		return
	}

	observed := float64(obs.Chars) / float64(obs.ActualInputTokens)
	if observed < minRatio || observed > maxRatio {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	st, ok := c.states[obs.Model]
	if !ok {
		st = &ratioState{ratio: c.seed}
		c.states[obs.Model] = st
	}

	// Adaptive weight: the first observation replaces the seed outright, and
	// influence decays toward minAlpha as evidence accumulates. This converges
	// far faster than a fixed alpha while still resisting a single outlier once
	// there is history to compare against.
	alpha := 1.0 / float64(st.n+1)
	if alpha < minAlpha {
		alpha = minAlpha
	}

	st.ratio = st.ratio*(1-alpha) + observed*alpha
	if st.ratio < minRatio {
		st.ratio = minRatio
	}
	if st.ratio > maxRatio {
		st.ratio = maxRatio
	}
	st.n++
}

// Estimate converts a measured character count into a token estimate for model.
func (c *Calibrator) Estimate(model string, chars int) (int, Confidence, string) {
	ratio, conf := c.Ratio(model)
	if chars <= 0 {
		return 0, conf, fmt.Sprintf("estimator:%s(%.2f)", conf, ratio)
	}
	tokens := int(float64(chars)/ratio + 0.5)
	if tokens < 1 {
		tokens = 1
	}
	return tokens, conf, fmt.Sprintf("estimator:%s(%.2f)", conf, ratio)
}

// Snapshot returns a copy of the learned ratios, for observability.
func (c *Calibrator) Snapshot() map[string]RatioSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make(map[string]RatioSnapshot, len(c.states))
	for model, st := range c.states {
		out[model] = RatioSnapshot{Ratio: st.ratio, Observations: st.n}
	}
	return out
}

// RatioSnapshot is one model's learned calibration state.
type RatioSnapshot struct {
	Ratio        float64 `json:"ratio"`
	Observations int     `json:"observations"`
}
