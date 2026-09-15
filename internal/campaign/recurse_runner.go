package campaign

import (
	"context"
	"errors"
	"fmt"
)

// The wave loop for /recurse: run wave campaigns back to back, retargeting
// from each wave's findings, until the wave budget runs out, the operator
// stops it, or the stall fuse trips. The fuse is what keeps "forever"
// honest: a run that completes nothing, wave after wave, is not working —
// it is burning budget on a loop that cannot converge, and must say so.

// RecurseResult reports what a run accomplished.
type RecurseResult struct {
	// WavesCompleted counts waves whose orchestrator returned without a
	// context error, including a final stalled wave.
	WavesCompleted int
	// CompletedTasks totals TaskCompleted across all waves.
	CompletedTasks int
	// FailedTasks totals TaskFailed across all waves.
	FailedTasks int
	// Stalled is true when the fuse stopped the run.
	Stalled bool
	// LastCampaignID identifies the final wave campaign for inspection.
	LastCampaignID string
	// WaveErrors records per-wave Run errors that did not stop the run
	// ("wave N: ..."). A wave that errors but completes tasks continues;
	// the error stays visible here instead of vanishing into the log.
	WaveErrors []string
}

// RecurseWaveFunc runs one wave to completion and returns the finished
// campaign plus the run error, if any. The previous finished wave (nil for
// wave zero) carries the findings the wave plan retargets from. Production
// implementations build an orchestrator, run it, and hand back the campaign
// they built (the orchestrator mutates it in place); the runner only judges
// the loop, never the execution.
type RecurseWaveFunc func(ctx context.Context, wave int, prev *Campaign) (finished *Campaign, runErr error)

// RecurseLoop is the shared wave policy: bounds plus the stall fuse. The CLI
// runner and the chat wave-chainer both consult it, so a bound or a stall
// means the same thing on every surface. Observe records one finished wave
// and judges what comes next.
type RecurseLoop struct {
	// MaxWaves bounds the run. Zero with AllowUnbounded runs until stopped
	// or stalled.
	MaxWaves int
	// AllowUnbounded permits MaxWaves == 0. The CLIs set it from yolo mode
	// only; the loop refuses an unbounded run without it.
	AllowUnbounded bool
	// StallWaveLimit stops the run after this many consecutive waves with
	// zero completed tasks. Zero means DefaultRecurseStallWaves.
	StallWaveLimit int

	waves        int
	stalledWaves int
}

// RecurseDecision is what the loop judges after a finished wave.
type RecurseDecision int

const (
	// RecurseProceed plans and runs the next wave.
	RecurseProceed RecurseDecision = iota
	// RecurseDoneBounds ends the run: the wave budget ran out.
	RecurseDoneBounds
	// RecurseDoneStalled ends the run: the fuse tripped.
	RecurseDoneStalled
)

// Validate rejects an unconfigured loop before the first wave.
func (l *RecurseLoop) Validate() error {
	if l.MaxWaves == 0 && !l.AllowUnbounded {
		return fmt.Errorf("recurse: unbounded run requires yolo mode")
	}
	if l.MaxWaves < 0 {
		return fmt.Errorf("recurse: max waves must be >= 0")
	}
	if l.StallWaveLimit < 0 {
		return fmt.Errorf("recurse: stall wave limit must be >= 0")
	}
	return nil
}

// Observe records a finished wave and judges the run. A wave that completes
// nothing advances the fuse; any completed task resets it.
func (l *RecurseLoop) Observe(finished *Campaign) (RecurseDecision, WaveFindings) {
	findings := SummarizeWave(finished)
	l.waves++
	if findings.Completed == 0 {
		l.stalledWaves++
	} else {
		l.stalledWaves = 0
	}
	limit := l.StallWaveLimit
	if limit == 0 {
		limit = DefaultRecurseStallWaves
	}
	if l.stalledWaves >= limit {
		return RecurseDoneStalled, findings
	}
	if l.MaxWaves > 0 && l.waves >= l.MaxWaves {
		return RecurseDoneBounds, findings
	}
	return RecurseProceed, findings
}

// WavesCompleted reports how many waves the loop has recorded.
func (l *RecurseLoop) WavesCompleted() int { return l.waves }

// RecurseRunner runs waves until a bound. It owns no LLM or kernel handles;
// the wave function builds each wave's orchestrator the way any other
// campaign entry point does, so recurse stays an extension over campaigns
// rather than a second orchestrator.
type RecurseRunner struct {
	NewWave RecurseWaveFunc
	// MaxWaves bounds the run. Zero with AllowUnbounded runs until stopped
	// or stalled.
	MaxWaves int
	// AllowUnbounded permits MaxWaves == 0. The CLIs set it from yolo mode
	// only; the library refuses an unbounded run without it.
	AllowUnbounded bool
	// StallWaveLimit stops the run after this many consecutive waves with
	// zero completed tasks. Zero means DefaultRecurseStallWaves.
	StallWaveLimit int
}

// ErrRecurseStalled reports the fuse trip. It is returned together with the
// result so far; callers exit non-zero on it.
var ErrRecurseStalled = errors.New("recurse stalled: consecutive waves completed nothing")

// Run executes waves sequentially. Waves are sequential on purpose: wave N+1
// retargets from wave N's findings, so parallel waves would plan blind.
func (r *RecurseRunner) Run(ctx context.Context) (*RecurseResult, error) {
	if r.NewWave == nil {
		return nil, fmt.Errorf("recurse: no wave function")
	}
	loop := &RecurseLoop{
		MaxWaves:       r.MaxWaves,
		AllowUnbounded: r.AllowUnbounded,
		StallWaveLimit: r.StallWaveLimit,
	}
	if err := loop.Validate(); err != nil {
		return nil, err
	}

	result := &RecurseResult{}
	var prev *Campaign
	for wave := 0; ; wave++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		finished, runErr := r.NewWave(ctx, wave, prev)
		if ctx.Err() != nil {
			// Stopped mid-wave: the wave's own campaign record keeps what
			// it managed, but the run counts only completed waves.
			return result, ctx.Err()
		}
		if finished == nil {
			return result, fmt.Errorf("recurse wave %d: wave function returned nil", wave)
		}
		if runErr != nil {
			// A wave that errors still counts: its finished tasks are real
			// progress and its failures feed the next wave.
			// One bad wave must not end a sweep (the fuse judges progress,
			// not luck), but the error must stay visible in the result.
			result.WaveErrors = append(result.WaveErrors, fmt.Sprintf("wave %d: %v", wave, runErr))
		}
		decision, findings := loop.Observe(finished)
		result.WavesCompleted++
		result.CompletedTasks += findings.Completed
		result.FailedTasks += findings.Failed
		result.LastCampaignID = finished.ID
		prev = finished
		switch decision {
		case RecurseDoneStalled:
			result.Stalled = true
			return result, fmt.Errorf("%w (%d waves, last %s)",
				ErrRecurseStalled, loop.WavesCompleted(), result.LastCampaignID)
		case RecurseDoneBounds:
			return result, nil
		}
	}
}
