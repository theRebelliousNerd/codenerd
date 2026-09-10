package broker

import "unicode/utf8"

// TextCounter estimates tokens for a bare string using the shared calibrator.
//
// It exists so that the parts of the system that count something other than an
// outbound request — the context compressor counts facts and compressed turns,
// not HTTP bodies — use the same learned ratio as admission does, instead of
// keeping a second ruler that disagrees.
//
// That second ruler is exactly what this replaces. internal/context used to
// carry its own charsPerToken = 4.0 with a TokenEstimator seam that nothing
// ever filled, and every compression threshold in the system was denominated in
// that unit while the provider's exact figure sat unused one package away.
type TextCounter struct {
	cal   *Calibrator
	model string
}

// TextCounter returns a counter for model. An empty model resolves to the
// meter's primary model, so callers that only know "the model we are talking
// to" — which is most of them — still get a ratio trained on real responses
// rather than a private default.
func (m *Meter) TextCounter(model string) *TextCounter {
	m.mu.RLock()
	cal := m.calibrator
	if model == "" {
		model = m.primaryModel
	}
	m.mu.RUnlock()
	return &TextCounter{cal: cal, model: model}
}

// EstimateTokens implements the estimator seam used by internal/context.
func (t *TextCounter) EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	chars := utf8.RuneCountInString(s)

	ratio, _ := t.cal.Ratio(t.model)
	tokens := int(float64(chars)/ratio + 0.5)
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// Confidence reports the regime backing this counter's numbers, so a caller
// displaying a budget can say whether it is a measurement or an estimate.
func (t *TextCounter) Confidence() Confidence {
	_, conf := t.cal.Ratio(t.model)
	return conf
}

// Ratio exposes the current learned chars-per-token ratio, for observability.
func (t *TextCounter) Ratio() float64 {
	ratio, _ := t.cal.Ratio(t.model)
	return ratio
}

// PrimaryModel returns the meter's primary model.
func (m *Meter) PrimaryModel() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.primaryModel
}

// PromptBudget returns the token allowance for assembling a system prompt,
// as a share of the window the ledger is actually enforcing.
//
// Every caller of this used to carry its own literal: 65536 in the session
// executor, 120000 in init's JIT integration, a 60000 clamp in the prompt
// assembler, 200000 in the campaign context pager. None of them read the
// configured window and none of them knew about each other. Deriving from the
// ledger means a workspace that configures a 400k window gets proportionally
// larger prompts everywhere, and one that configures 64k does not silently
// assemble prompts that cannot fit.
//
// share is clamped to (0, 1]. A non-positive or unconfigured window yields
// fallback, so a caller during boot — before Configure has run — still gets a
// usable number rather than zero.
func (m *Meter) PromptBudget(share float64, fallback int) int {
	if share <= 0 || share > 1 {
		share = 1
	}

	available := m.Ledger().Available()
	if available <= 0 {
		return fallback
	}

	budget := int(float64(available) * share)
	if budget < minPromptBudget {
		// A prompt too small to hold the identity and methodology atoms
		// produces a specialist that is neither, which is worse than a prompt
		// that slightly overruns a conservative share.
		return minPromptBudget
	}
	return budget
}

// minPromptBudget is the floor below which a compiled prompt stops being able
// to carry its mandatory atoms.
const minPromptBudget = 8000
