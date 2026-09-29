package config

import (
	"fmt"
	"sync/atomic"
)

// ClassificationConfig is the `classification` section of .nerd/config.json:
// how much prior conversation the intent classifier is shown.
//
// It is not session.history_turn_window / session.history_char_budget. Those
// size the generating model's memory (default six messages, 24000 bytes).
// Classification runs on every interactive turn, on classification_model,
// and only has to tell this request from the last few. One knob cannot serve
// both: widening the generator's memory would widen the cheap call, and
// tightening classification would amputate the generator. Reasoning
// summaries are classification-only; the session window never sends them.
//
// Turns are evicted whole, oldest first. A turn is never sliced. The newest
// prior turn is still included when it alone exceeds the budget:
// classification is one shot and has no recall verb, and a follow-up
// ("fix that", "now compile it") refers to that turn. The prompt says so.
//
// history_turn_window is a pointer because 0 is meaningful: send no prior
// turn. A char budget of 0 is the absent key (omitempty), not "unlimited".
type ClassificationConfig struct {
	// HistoryTurnWindow is how many prior turns reach the classifier.
	// 0 sends none. The default is 5.
	HistoryTurnWindow *int `json:"history_turn_window,omitempty"`
	// HistoryCharBudget is the byte budget (len of content plus reasoning
	// summary, the same measure session.history_char_budget uses) for those
	// turns. Older turns leave whole until the rest fit. The default, 14000,
	// is five turns at the old per-turn slices (2000 of content and 800 of
	// reasoning summary): five turns that used to be cut to the cap still
	// fit whole, and a longer turn displaces older ones instead of being cut.
	HistoryCharBudget int `json:"history_char_budget,omitempty"`
}

// DefaultClassificationConfig is the classification section with every field
// written down.
func DefaultClassificationConfig() ClassificationConfig {
	window := 5
	return ClassificationConfig{
		HistoryTurnWindow: &window,
		HistoryCharBudget: 14000,
	}
}

// GetClassificationConfig returns the classification section with every
// absent field defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetClassificationConfig() ClassificationConfig {
	if c == nil || c.Classification == nil {
		return DefaultClassificationConfig()
	}
	return c.Classification.WithDefaults()
}

// WithDefaults fills every absent field from DefaultClassificationConfig.
func (c ClassificationConfig) WithDefaults() ClassificationConfig {
	d := DefaultClassificationConfig()
	if c.HistoryTurnWindow == nil {
		c.HistoryTurnWindow = d.HistoryTurnWindow
	}
	if c.HistoryCharBudget == 0 {
		c.HistoryCharBudget = d.HistoryCharBudget
	}
	return c
}

// ClassificationHistory is a ClassificationConfig resolved for the
// classifier: defaults filled. Resolve is the only way to make one, and
// LoadUserConfig installs it where BuildPrompt reads it.
type ClassificationHistory struct {
	TurnWindow int
	CharBudget int
}

// Resolve defaults the section. Check is what refuses a contradictory file,
// and LoadUserConfig runs it first.
func (c ClassificationConfig) Resolve() ClassificationHistory {
	c = c.WithDefaults()
	return ClassificationHistory{
		TurnWindow: *c.HistoryTurnWindow,
		CharBudget: c.HistoryCharBudget,
	}
}

// Check reports the contradictions in a classification section, addressed
// under prefix ("classification"). Absent fields are defaulted first, so
// only what the file says can be wrong.
func (c ClassificationConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	if *c.HistoryTurnWindow < 0 {
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     prefix + ".history_turn_window",
			Message:  fmt.Sprintf("%d is negative", *c.HistoryTurnWindow),
			Fix:      "0 to send no prior turn, or a turn count",
		})
	}
	if c.HistoryCharBudget < 1 {
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     prefix + ".history_char_budget",
			Message:  fmt.Sprintf("%d is below 1", c.HistoryCharBudget),
			Fix:      "a byte budget of at least 1, or remove the key for the default",
		})
	}
	return out
}

// activeClassificationHistory is the process-wide resolved classification
// history policy, installed by LoadUserConfig the same way the observation
// limits are. The intent classifier reads it when it builds the prompt,
// without opening the config file itself.
var activeClassificationHistory atomic.Pointer[ClassificationHistory]

func init() {
	SetClassificationHistory(DefaultClassificationConfig().Resolve())
}

// SetClassificationHistory installs the process-wide classification history
// policy. LoadUserConfig is the production caller; tests install and restore.
func SetClassificationHistory(h ClassificationHistory) {
	activeClassificationHistory.Store(&h)
}

// ResolvedClassificationHistory is the installed classification history
// policy. Without a load it is the defaults.
func ResolvedClassificationHistory() ClassificationHistory {
	if h := activeClassificationHistory.Load(); h != nil {
		return *h
	}
	return DefaultClassificationConfig().Resolve()
}
