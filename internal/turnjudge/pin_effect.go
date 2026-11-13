package turnjudge

import (
	"context"
	"fmt"
	"math"
	"strings"

	"codenerd/internal/typesafe"
)

// Pin effect: whether taking one change back out (a function put back as it
// was, an added one removed, a condition held at a constant) would change
// behaviour a test could observe and force. The pinning gate asks it only of
// a change the turn's tests do not notice; the kernel decides
// (policy/turn_rounds.mg turn_pin_not_pinnable) together with the measurement
// that the package's whole suite does not notice either. The switch is
// typesafe.pin_equivalence.

// OpPinEffect is the usage tracker's operation for this use.
const OpPinEffect = "jev_pin_effect"

// PinEffectRequest builds the state and the question.
func PinEffectRequest(label, withChange, takenOut string) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{
		"change":                 label,
		"code_with_the_change":   withChange,
		"code_with_it_taken_out": takenOut,
	}
	qs := map[string]typesafe.Question{
		"observable": typesafe.Noul{
			Instructions: "Would replacing `code_with_the_change` by `code_with_it_taken_out` change anything a test can observe and force: a returned value, an error, a write, a call made or not made, or an ordering a test can schedule?",
			Criteria: &typesafe.NoulCriteria{
				True:  "Some input or call sequence a test can set up gives a different result, error or side effect with the change taken out.",
				False: "Every result, error and side effect is the same either way; the difference is only speed, memory, or a concurrent interleaving (a lost race, a LoadOrStore that finds the value already stored) that no deterministic test can bring about.",
			},
		},
	}
	return state, qs
}

// JudgePinEffect returns the probability (percent) that the change is
// observable. A failed call is an error, never a "no".
func JudgePinEffect(ctx context.Context, a Asker, label, withChange, takenOut string) (int64, error) {
	if a == nil {
		return 0, fmt.Errorf("turnjudge: no Jev client")
	}
	if strings.TrimSpace(withChange) == "" && strings.TrimSpace(takenOut) == "" {
		return 0, fmt.Errorf("turnjudge: a pin effect reading needs the code on either side")
	}
	state, qs := PinEffectRequest(label, withChange, takenOut)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpPinEffect), state, qs)
	if err != nil {
		return 0, err
	}
	p, err := resp.Noul("observable")
	if err != nil {
		return 0, err
	}
	return int64(math.Round(p * 100)), nil
}

// PinEffectReader is the session's PinEffectJudge over Jev.
type PinEffectReader struct {
	Asker Asker
}

// ReadPinEffect implements session.PinEffectJudge.
func (r PinEffectReader) ReadPinEffect(ctx context.Context, label, withChange, takenOut string) (int64, error) {
	return JudgePinEffect(ctx, r.Asker, label, withChange, takenOut)
}
