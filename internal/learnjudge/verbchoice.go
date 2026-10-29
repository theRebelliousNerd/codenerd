package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Verb choice: which of codeNERD's verbs does a request ask for? Jev picks
// from the verb corpus (a Choice: select, not generate) and says, in a
// companion Noul, whether any verb fits at all, since a Choice always picks
// one. The learning loop uses it on the request a correction says codeNERD
// misread: the mapping is learned only when Jev is confident on both, so
// learning needs no LLM critic and no "Should I learn it?" from the user.
// Live on Jev 1.14 over all 43 verbs, 13 requests with a clear verb were
// picked right, 11 at 0.96 or more; over 70 real dogfood messages, the
// companion read 50 as fitting no verb well (answers like "yes" or "go",
// questions about codeNERD's own state, feedback on its behaviour).

// VerbOption is one verb and its own words (its synonyms in the corpus).
type VerbOption struct {
	Verb  string
	Means []string
}

// VerbChoice is Jev's pick: the verb, the probability on it, and the
// probability that any verb fits the request.
type VerbChoice struct {
	Verb       string
	Confidence float64
	Fits       float64
}

// Question names.
const (
	ChosenVerb = "chosen_verb"
	VerbFits   = "verb_fits"
)

// verbMeansLimit bounds each verb's description: its first synonyms are the
// ones that name it; a long tail of phrasings makes options overlap.
const verbMeansLimit = 8

// VerbChoiceRequest builds the state and both questions. Option names are the
// verbs without their slash: a name that describes the action helps Jev.
func VerbChoiceRequest(request string, options []VerbOption) (map[string]any, map[string]typesafe.Question) {
	criteria := make(map[string]any, len(options))
	names := make([]string, 0, len(options))
	for _, o := range options {
		name := strings.TrimPrefix(strings.TrimSpace(o.Verb), "/")
		if name == "" {
			continue
		}
		means := o.Means
		if len(means) > verbMeansLimit {
			means = means[:verbMeansLimit]
		}
		desc := "The user asks codeNERD to " + name + "."
		if len(means) > 0 {
			desc = "The user asks codeNERD to " + strings.Join(means, ", ") + "."
		}
		criteria[name] = desc
		names = append(names, name)
	}
	state := map[string]any{
		"assistant": "codeNERD, a coding agent working in the user's repository",
		"request":   request,
		"actions":   names,
	}
	return state, map[string]typesafe.Question{
		ChosenVerb: typesafe.Choice{
			Instructions: "Which action does the user ask codeNERD for in `request`?",
			Criteria:     criteria,
		},
		VerbFits: typesafe.Noul{
			Instructions: "Is one of `actions` a good name for what the user asks for in `request`?",
			Criteria: &typesafe.NoulCriteria{
				True:  "One of the actions describes the request well.",
				False: "The request asks for something none of the actions describes, or only loosely.",
			},
		},
	}
}

// ChooseVerb asks Jev for the request's verb. A failed call is an error,
// never a pick.
func ChooseVerb(ctx context.Context, a Asker, request string, options []VerbOption) (VerbChoice, error) {
	if a == nil {
		return VerbChoice{}, fmt.Errorf("learnjudge: no Jev client")
	}
	if strings.TrimSpace(request) == "" || len(options) == 0 {
		return VerbChoice{}, fmt.Errorf("learnjudge: a verb choice needs a request and verbs")
	}
	state, questions := VerbChoiceRequest(request, options)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpVerbChoice), state, questions)
	if err != nil {
		return VerbChoice{}, err
	}
	pick, err := resp.Choice(ChosenVerb)
	if err != nil {
		return VerbChoice{}, err
	}
	fits, err := resp.Noul(VerbFits)
	if err != nil {
		return VerbChoice{}, err
	}
	return VerbChoice{Verb: "/" + pick.Choice, Confidence: pick.Confidence, Fits: fits}, nil
}
