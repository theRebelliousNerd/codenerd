package learnjudge

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// Request delivery: did the steps a chat request ran deliver every part of
// it? A shard can say "done" and still miss half the request, and the /finish
// round inside one turn reads only the shard's own admission. Here the
// request is split into candidate fragments (RequestFragments), and Jev
// judges, per fragment, whether the person asked for it and whether what the
// steps delivered does it (the coverage questions), plus whether the steps
// name an obstacle codeNERD cannot clear by itself. The kernel decides
// (policy/request_delivery.mg): a missing part is handed back as another
// step until none is missing, a blocker is named, or a round resolves
// nothing.

// DeliveryInput is the person's request, the reports of the steps that ran
// for it in order, and the files those steps changed.
type DeliveryInput struct {
	Request   string
	Reports   []string
	Changed   []string
	Fragments []string
}

// DeliveryReading is Jev's reading of one round: a coverage verdict per
// fragment, the probability that the reports name a blocker, and whether
// the reports were cut to fit.
type DeliveryReading struct {
	Verdicts  []CoverageVerdict
	Blocked   float64
	ResultCut bool
}

// DeliveryNamesBlocker is the blocker question's name.
const DeliveryNamesBlocker = "names_blocker"

// CheckDelivery asks the coverage questions for every fragment and the
// blocker question, over one state; the blocker question rides the first
// request. A failed call is an error, never a reading.
func CheckDelivery(ctx context.Context, a Asker, in DeliveryInput) (DeliveryReading, error) {
	if a == nil {
		return DeliveryReading{}, fmt.Errorf("learnjudge: no Jev client")
	}
	if len(in.Fragments) == 0 {
		return DeliveryReading{}, fmt.Errorf("learnjudge: no request fragments to judge")
	}
	state := deliveryState(in)
	_, cut := state["result_omitted_bytes"]
	reading := DeliveryReading{ResultCut: cut, Verdicts: make([]CoverageVerdict, 0, len(in.Fragments))}
	per := coverageQuestionsPerRequest / 2
	for start := 0; start < len(in.Fragments); start += per {
		end := min(start+per, len(in.Fragments))
		questions := make(map[string]typesafe.Question, 2*(end-start)+1)
		for i := start; i < end; i++ {
			questions[fmt.Sprintf("asked_%d", i)] = askedQuestion(in.Fragments[i])
			questions[fmt.Sprintf("delivered_%d", i)] = deliveredQuestion(in.Fragments[i])
		}
		if start == 0 {
			questions[DeliveryNamesBlocker] = blockerQuestion("result")
		}
		resp, err := a.Ask(typesafe.WithOperation(ctx, OpRequestDelivery), state, questions)
		if err != nil {
			return DeliveryReading{}, err
		}
		if start == 0 {
			if reading.Blocked, err = resp.Noul(DeliveryNamesBlocker); err != nil {
				return DeliveryReading{}, err
			}
		}
		for i := start; i < end; i++ {
			asked, err := resp.Noul(fmt.Sprintf("asked_%d", i))
			if err != nil {
				return DeliveryReading{}, err
			}
			delivered, err := resp.Noul(fmt.Sprintf("delivered_%d", i))
			if err != nil {
				return DeliveryReading{}, err
			}
			reading.Verdicts = append(reading.Verdicts, CoverageVerdict{Fragment: in.Fragments[i], Asked: asked, Delivered: delivered})
		}
	}
	return reading, nil
}

// deliveryState is what a person would look at: the request, what the steps
// reported (one block per step, in order), and the files they changed. An
// empty change list is left out rather than sent as a signal.
func deliveryState(in DeliveryInput) map[string]any {
	state := map[string]any{
		"assistant": "codeNERD, a coding agent working in the person's repository",
		"request":   strings.TrimSpace(in.Request),
	}
	var b strings.Builder
	for i, r := range in.Reports {
		if len(in.Reports) > 1 {
			fmt.Fprintf(&b, "Step %d report:\n", i+1)
		}
		b.WriteString(strings.TrimSpace(r))
		b.WriteString("\n\n")
	}
	putBounded(state, "result", strings.TrimSpace(b.String()), coverageResultLimit)
	if len(in.Changed) > 0 {
		state["files_changed"] = in.Changed
	}
	return state
}

// Kernel projections (policy/request_delivery.mg). Percents are whole
// numbers, as config_param(/jev_yes_percent) and /jev_no_percent are.
const (
	DeliveryReadingPredicate = "delivery_reading"
	DeliveryBlockerPredicate = "delivery_blocker_reading"
	DeliveryCutPredicate     = "delivery_result_cut"
)

// DeliveryFacts projects one round's reading for one request.
func DeliveryFacts(request any, round int, r DeliveryReading) []types.Fact {
	facts := make([]types.Fact, 0, len(r.Verdicts)+2)
	for _, v := range r.Verdicts {
		facts = append(facts, types.Fact{Predicate: DeliveryReadingPredicate, Args: []any{
			request, int64(round), v.Fragment, types.PercentFromRatio(v.Asked), types.PercentFromRatio(v.Delivered),
		}})
	}
	facts = append(facts, types.Fact{Predicate: DeliveryBlockerPredicate, Args: []any{request, int64(round), types.PercentFromRatio(r.Blocked)}})
	if r.ResultCut {
		facts = append(facts, types.Fact{Predicate: DeliveryCutPredicate, Args: []any{request, int64(round)}})
	}
	return facts
}

// fragmentSplitRe marks where one ask can end and the next begin: line
// breaks, semicolons, sentence ends, and the conjunctions that join asks.
// The split only proposes candidates; Jev's asked question drops the ones
// that are context, examples or rules, so over-splitting costs a question,
// never a false gap.
var fragmentSplitRe = regexp.MustCompile(`(?i)\s*(?:[;\n]+|[.!?]\s+)\s*|\s*,?\s+(?:and\s+then|and\s+also|and|then|also|plus)\s+`)

var listMarkerRe = regexp.MustCompile(`(?m)^\s*(?:\d+[.):]|[-*])\s+`)

// leadingJoinRe is a joining word a separator left at the head of a piece
// ("...; then run the tests").
var leadingJoinRe = regexp.MustCompile(`(?i)^(?:and\s+then|and\s+also|and|then|also|plus)\s+`)

var wordRe = regexp.MustCompile(`[\pL\pN_]+`)

// RequestFragments proposes the parts of a request that could each be an
// ask. The wording is the person's own, so a gap can be named back to them.
// A one-word piece is a modifier of the clause before it ("cleanly and
// consistently") and is folded back into it. A request that does not split
// is one fragment.
func RequestFragments(request string) []string {
	var out []string
	seen := map[string]bool{}
	unlisted := listMarkerRe.ReplaceAllString(strings.TrimSpace(request), "\n")
	for _, p := range fragmentSplitRe.Split(unlisted, -1) {
		p = leadingJoinRe.ReplaceAllString(strings.Trim(p, " \t.,;:"), "")
		p = strings.Trim(p, " .,;:")
		if p == "" {
			continue
		}
		if len(wordRe.FindAllString(p, -1)) < 2 && len(out) > 0 {
			out[len(out)-1] += " and " + p
			continue
		}
		key := strings.ToLower(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}
