package broker

import "context"

// Counter turns an assembled request into a token count.
//
// Implementations must be safe for concurrent use and must never return a count
// with a Confidence stronger than the evidence supports. Returning
// ConfidenceExact for a guess is the one unforgivable bug in this package: every
// admission decision downstream trusts that label.
type Counter interface {
	Count(ctx context.Context, req *Request) (Count, error)
}

// CalibratingCounter is the optional half of Counter: a counter that learns
// from provider actuals. The broker feeds every observed response back through
// this when the counter implements it.
type CalibratingCounter interface {
	Counter
	Observe(obs Observation)
}

// EstimatingCounter counts by measuring the request and dividing by a learned
// chars-per-token ratio. It is the counter for every provider that does not
// expose a counting endpoint.
//
// It is never "exact", and it says so. What makes it materially better than the
// constant it replaces is that its error is measured against the provider on
// every single response and fed back, so a session's later admissions are made
// on a ratio fitted to the actual model, language, and content mix in play.
type EstimatingCounter struct {
	cal *Calibrator
}

// NewEstimatingCounter returns a counter backed by cal. A nil calibrator gets a
// fresh one rather than panicking on the hot path.
func NewEstimatingCounter(cal *Calibrator) *EstimatingCounter {
	if cal == nil {
		cal = NewCalibrator()
	}
	return &EstimatingCounter{cal: cal}
}

// Count implements Counter.
func (e *EstimatingCounter) Count(_ context.Context, req *Request) (Count, error) {
	if req == nil {
		return Count{}, errNilRequest
	}

	chars := measure(req)
	total, conf, source := e.cal.Estimate(req.Model, chars.Total())

	return Count{
		Tokens:     total,
		Segments:   splitProportional(total, chars),
		Confidence: conf,
		Source:     source,
		Model:      req.Model,
	}, nil
}

// Observe implements CalibratingCounter.
func (e *EstimatingCounter) Observe(obs Observation) { e.cal.Observe(obs) }

// Calibrator exposes the underlying calibrator for observability.
func (e *EstimatingCounter) Calibrator() *Calibrator { return e.cal }

var _ CalibratingCounter = (*EstimatingCounter)(nil)
