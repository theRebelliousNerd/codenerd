package broker

import (
	"math"
	"sort"
	"sync"

	"codenerd/internal/logging"
)

// Reconciliation thresholds.
const (
	// reconcileMinSamples is how many completed calls a model needs before its
	// drift is worth reporting. Early calls on a fresh model are served by the
	// seeded ratio and are expected to be wrong; alarming on them would train
	// operators to ignore the alarm.
	reconcileMinSamples = 20

	// reconcileDriftThreshold is the absolute mean error, as a fraction, past
	// which the meter is reported as untrustworthy for that model.
	//
	// 0.10 is deliberately tighter than the ~20% error of the heuristic this
	// package replaced. A calibrated estimator that cannot beat 10% mean error
	// after twenty observations is not calibrating — measure() has drifted from
	// what the client actually sends, and the ratio is absorbing the difference
	// instead of exposing it.
	reconcileDriftThreshold = 0.10
)

// Reconciler accumulates estimated-versus-actual input tokens per model.
//
// It exists to close a specific blind spot. On Anthropic the counting endpoint
// gives an exact number, so a receipt's EstimateErrorPct proves whether
// measure() is sound. Every other provider has no independent check: the
// estimator predicts, the calibrator corrects itself toward the provider's
// report, and a systematically wrong measure() would produce a plausible ratio
// and correlated wrong counts with nothing to contradict them.
//
// Comparing accumulated totals catches that. Calibration removes *random*
// error; it cannot remove a bias that both sides of the comparison share, so a
// persistent gap between what was predicted and what was billed means the
// prediction model itself is wrong.
//
// Safe for concurrent use.
type Reconciler struct {
	mu     sync.Mutex
	models map[string]*reconcileState
	// alarmed remembers which models have already logged, so a drifting model
	// warns once rather than on every call for the rest of the session.
	alarmed map[string]bool
}

type reconcileState struct {
	samples        int64
	estimatedTotal int64
	actualTotal    int64
	// absErrorSum accumulates per-call absolute relative error, so a model
	// that overshoots as often as it undershoots is not scored as accurate.
	// Totals alone would let +30% and -30% cancel to zero.
	absErrorSum float64
}

// NewReconciler returns an empty reconciler.
func NewReconciler() *Reconciler {
	return &Reconciler{
		models:  make(map[string]*reconcileState),
		alarmed: make(map[string]bool),
	}
}

// Observe records one completed call's predicted and billed input tokens.
// Calls with no reported actual are ignored: they carry no evidence either way.
func (r *Reconciler) Observe(model string, estimated int, actual int64) {
	if model == "" || estimated <= 0 || actual <= 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	st, ok := r.models[model]
	if !ok {
		st = &reconcileState{}
		r.models[model] = st
	}
	st.samples++
	st.estimatedTotal += int64(estimated)
	st.actualTotal += actual
	st.absErrorSum += math.Abs(float64(estimated)-float64(actual)) / float64(actual)

	if st.samples < reconcileMinSamples || r.alarmed[model] {
		return
	}
	meanAbsError := st.absErrorSum / float64(st.samples)
	if meanAbsError <= reconcileDriftThreshold {
		return
	}

	r.alarmed[model] = true
	logging.Get(logging.CategoryAPI).Warn(
		"broker: token counting for %q is drifting: mean absolute error %.1f%% over %d calls "+
			"(predicted %d, billed %d). Calibration removes random error but not a shared bias, so a "+
			"persistent gap means measure() no longer matches what the client sends.",
		model, meanAbsError*100, st.samples, st.estimatedTotal, st.actualTotal)
}

// ModelDrift is one model's reconciliation summary.
type ModelDrift struct {
	Model          string `json:"model"`
	Samples        int64  `json:"samples"`
	EstimatedTotal int64  `json:"estimated_total"`
	ActualTotal    int64  `json:"actual_total"`
	// MeanAbsErrorPct is the average per-call absolute error. This is the
	// number to read: it does not let overshoot and undershoot cancel.
	MeanAbsErrorPct float64 `json:"mean_abs_error_pct"`
	// NetBiasPct is (estimated-actual)/actual over the totals. A large net bias
	// alongside a small mean absolute error means the estimator is
	// consistently off in one direction, which is the correctable kind.
	NetBiasPct float64 `json:"net_bias_pct"`
	// Trustworthy is false once a model has enough samples and still drifts.
	Trustworthy bool `json:"trustworthy"`
}

// Drift returns per-model reconciliation, worst first.
func (r *Reconciler) Drift() []ModelDrift {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]ModelDrift, 0, len(r.models))
	for model, st := range r.models {
		meanAbs := 0.0
		if st.samples > 0 {
			meanAbs = st.absErrorSum / float64(st.samples)
		}
		net := 0.0
		if st.actualTotal > 0 {
			net = float64(st.estimatedTotal-st.actualTotal) / float64(st.actualTotal)
		}
		out = append(out, ModelDrift{
			Model:           model,
			Samples:         st.samples,
			EstimatedTotal:  st.estimatedTotal,
			ActualTotal:     st.actualTotal,
			MeanAbsErrorPct: meanAbs * 100,
			NetBiasPct:      net * 100,
			// A model below the sample floor is reported as trustworthy: it has
			// not failed, it has not been tested. The Samples field is what
			// distinguishes the two.
			Trustworthy: st.samples < reconcileMinSamples || meanAbs <= reconcileDriftThreshold,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MeanAbsErrorPct > out[j].MeanAbsErrorPct })
	return out
}
