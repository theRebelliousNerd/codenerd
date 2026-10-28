package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Instruction coverage: a compound `nerd run` instruction is split into
// fragments, and the result must deliver every fragment the person asked for.
// The split is a guess, so Jev judges two things per fragment instead of a
// word-overlap check: whether the fragment is something the person asked to
// be done or answered (not a description, an example or a place to look), and
// whether the result does or answers it. Code decides (CoverageGaps).

// coverageResultLimit bounds the result in the state, to fit Jev's
// per-question context; the state says how much was left out, and a cut
// result never yields a gap (CoverageGaps).
const coverageResultLimit = 60000

// coverageQuestionsPerRequest keeps each request's questions (two per
// fragment) within one call.
const coverageQuestionsPerRequest = 40

// CoverageVerdict is Jev's reading of one fragment: the probability that it
// is a deliverable the person asked for, and that the result delivers it.
type CoverageVerdict struct {
	Fragment  string
	Asked     float64
	Delivered float64
}

// CheckCoverage judges every fragment against the person's request and the
// result. A failed call is an error, never a "no": the caller falls back.
// It reports whether the result was cut to fit.
func CheckCoverage(ctx context.Context, a Asker, request, result string, fragments []string) ([]CoverageVerdict, bool, error) {
	if a == nil {
		return nil, false, fmt.Errorf("learnjudge: no Jev client")
	}
	state := map[string]any{
		"assistant": "codeNERD, a coding agent working in the person's repository",
		"request":   strings.TrimSpace(request),
	}
	putBounded(state, "result", strings.TrimSpace(result), coverageResultLimit)
	_, cut := state["result_omitted_bytes"]

	verdicts := make([]CoverageVerdict, 0, len(fragments))
	per := coverageQuestionsPerRequest / 2
	for start := 0; start < len(fragments); start += per {
		end := min(start+per, len(fragments))
		questions := make(map[string]typesafe.Question, 2*(end-start))
		for i := start; i < end; i++ {
			questions[fmt.Sprintf("asked_%d", i)] = askedQuestion(fragments[i])
			questions[fmt.Sprintf("delivered_%d", i)] = deliveredQuestion(fragments[i])
		}
		resp, err := a.Ask(typesafe.WithOperation(ctx, OpCoverage), state, questions)
		if err != nil {
			return nil, cut, err
		}
		for i := start; i < end; i++ {
			asked, err := resp.Noul(fmt.Sprintf("asked_%d", i))
			if err != nil {
				return nil, cut, err
			}
			delivered, err := resp.Noul(fmt.Sprintf("delivered_%d", i))
			if err != nil {
				return nil, cut, err
			}
			verdicts = append(verdicts, CoverageVerdict{Fragment: fragments[i], Asked: asked, Delivered: delivered})
		}
	}
	return verdicts, cut, nil
}

func askedQuestion(fragment string) typesafe.Question {
	return typesafe.Noul{
		Instructions: fmt.Sprintf("In `request`, is %q something the person asks codeNERD to do or to answer?", fragment),
		Criteria: &typesafe.NoulCriteria{
			True:  "It names an action to take, an artifact to produce, or a question to answer, such as \"add a test\" or \"explain why it fails\".",
			False: "It describes the situation, gives an example, names a place to look, states a rule for how to work (\"do not change files\"), or is a piece of another item's wording.",
		},
	}
}

func deliveredQuestion(fragment string) typesafe.Question {
	return typesafe.Noul{
		Instructions: fmt.Sprintf("Does `result` do or answer %q, as `request` asks?", fragment),
		Criteria: &typesafe.NoulCriteria{
			True:  "`result` shows that part done, or gives its answer.",
			False: "`result` does not mention that part, or says it was not done.",
		},
	}
}

// CoverageGaps is the decision, a truth table over the verdicts with the
// configured yes and no thresholds (probabilities in [0,1]):
//
//	asked <= no                        -> not a deliverable; ignored
//	asked > no and delivered >= yes    -> delivered
//	asked > no and delivered <= no     -> missing, unless the result was cut
//	                                      to fit, when it is unsettled
//	otherwise                          -> unsettled: reported, not a failure
func CoverageGaps(verdicts []CoverageVerdict, resultCut bool, yes, no float64) (missing, unsettled []string) {
	for _, v := range verdicts {
		switch {
		case v.Asked <= no:
		case v.Delivered >= yes:
		case v.Delivered <= no && !resultCut:
			missing = append(missing, v.Fragment)
		default:
			unsettled = append(unsettled, v.Fragment)
		}
	}
	return missing, unsettled
}
