package chat

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
)

// Learning from a correction runs beside the turn the correction asks for, not
// in its place. When the kernel derives learning_correction (jev_correction.go),
// Jev picks the verb the request codeNERD misread asks for, from the verb
// corpus (learnjudge.ChooseVerb), and the mapping is learned when Jev is
// confident that a verb fits and which one it is. Nothing asks the user. Until
// 2026-10-28 a correction replaced the turn with an LLM critic, asked "Should I
// learn it?", and then told the user to retry the request.

// learnedMapping is the outcome of learning from one correction: the fact
// learned, or why nothing was.
type learnedMapping struct {
	fact   string
	reason string
}

// learnFromCorrection learns from a correction in the background and reports
// the outcome on the status line and in the perception log. previousRequest is
// the request the corrected reply answered.
func (m Model) learnFromCorrection(previousRequest, source string) {
	ctx := m.learningContext()
	go func() {
		out := m.chooseLearnedMapping(ctx, previousRequest)
		line := out.reason
		if out.fact != "" {
			line = m.persistLearnedMapping(ctx, out)
		}
		logging.Get(logging.CategoryPerception).Info("learning (/%s): %s", source, line)
		m.ReportStatus("Learning: " + line)
	}()
}

// chooseLearnedMapping asks Jev for the previous request's verb and decides,
// on the configured yes threshold, whether it is a mapping to learn.
func (m Model) chooseLearnedMapping(ctx context.Context, previousRequest string) learnedMapping {
	request := strings.TrimSpace(previousRequest)
	switch {
	case request == "":
		return learnedMapping{reason: "nothing learned: no earlier request to learn from"}
	case m.jev == nil:
		return learnedMapping{reason: "nothing learned: Jev is not configured to pick the verb"}
	}
	var options []learnjudge.VerbOption
	for _, v := range perception.GetVerbCorpus() {
		options = append(options, learnjudge.VerbOption{Verb: v.Verb, Means: v.Synonyms})
	}
	pick, err := learnjudge.ChooseVerb(m.jevContext(ctx), m.jev, request, options)
	if err != nil {
		return learnedMapping{reason: fmt.Sprintf("nothing learned: Jev's verb choice failed: %v", err)}
	}
	yes, _ := m.Config.TypeSafe.Thresholds()
	threshold := float64(yes) / 100
	switch {
	case pick.Fits < threshold:
		return learnedMapping{reason: fmt.Sprintf("nothing learned: no verb fits %q well (fit %.2f, closest %s)", request, pick.Fits, pick.Verb)}
	case pick.Confidence < threshold:
		return learnedMapping{reason: fmt.Sprintf("nothing learned: Jev is unsure which verb %q asks for (%s at %.2f)", request, pick.Verb, pick.Confidence)}
	}
	return learnedMapping{
		fact:   fmt.Sprintf(`learned_exemplar("%s", %s, "", "", %.2f).`, escapeMangleString(request), pick.Verb, pick.Confidence),
		reason: fmt.Sprintf("%q -> %s (Jev %.2f, fit %.2f)", request, pick.Verb, pick.Confidence, pick.Fits),
	}
}

// persistLearnedMapping stores a chosen mapping and says what was stored.
func (m Model) persistLearnedMapping(ctx context.Context, out learnedMapping) string {
	if perception.SharedTaxonomy == nil {
		return "not stored: the taxonomy engine is not available; would have learned " + out.reason
	}
	outcome, err := perception.SharedTaxonomy.PersistLearnedFact(ctx, out.fact)
	if err != nil {
		return fmt.Sprintf("not stored: %v; would have learned %s", err, out.reason)
	}
	return "learned " + out.reason + "." + partialLearningNote(outcome)
}
