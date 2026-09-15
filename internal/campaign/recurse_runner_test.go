package campaign

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func recurseTestWave(id string, completed, failed int) *Campaign {
	tasks := make([]Task, 0, completed+failed)
	for i := 0; i < completed; i++ {
		tasks = append(tasks, Task{ID: fmt.Sprintf("%s-ok-%d", id, i), Status: TaskCompleted})
	}
	for i := 0; i < failed; i++ {
		tasks = append(tasks, Task{ID: fmt.Sprintf("%s-fail-%d", id, i), Status: TaskFailed})
	}
	return &Campaign{ID: id, Phases: []Phase{{ID: id + "-p", Name: "recurse:mangle:Mangle", Tasks: tasks}}}
}

func TestRecurseRunner_CompletesMaxWaves(t *testing.T) {
	var waves []int
	r := &RecurseRunner{
		MaxWaves: 3,
		NewWave: func(ctx context.Context, wave int, prev *Campaign) (*Campaign, error) {
			waves = append(waves, wave)
			if wave > 0 && prev == nil {
				return nil, errors.New("wave func must receive the previous wave")
			}
			return recurseTestWave(fmt.Sprintf("wave-%d", wave), 2, 1), nil
		},
	}
	result, err := r.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.WavesCompleted != 3 || result.CompletedTasks != 6 || result.FailedTasks != 3 {
		t.Fatalf("result = %+v, want 3 waves / 6 completed / 3 failed", result)
	}
	if result.Stalled || len(result.WaveErrors) != 0 {
		t.Fatalf("clean run must not stall or record errors: %+v", result)
	}
	if result.LastCampaignID != "wave-2" {
		t.Fatalf("last ID = %q, want wave-2", result.LastCampaignID)
	}
}

func TestRecurseRunner_StallFuseTrips(t *testing.T) {
	r := &RecurseRunner{
		MaxWaves:       10,
		StallWaveLimit: 2,
		NewWave: func(ctx context.Context, wave int, prev *Campaign) (*Campaign, error) {
			return recurseTestWave(fmt.Sprintf("wave-%d", wave), 0, 2), nil
		},
	}
	result, err := r.Run(context.Background())
	if err == nil || !errors.Is(err, ErrRecurseStalled) {
		t.Fatalf("err = %v, want ErrRecurseStalled", err)
	}
	if !result.Stalled || result.WavesCompleted != 2 {
		t.Fatalf("result = %+v, want stalled after 2 waves", result)
	}
}

func TestRecurseRunner_ProgressResetsFuse(t *testing.T) {
	// Fail, succeed, fail, fail: the success resets the fuse, so the run
	// survives wave 2 and trips after waves 3+4 complete nothing.
	script := []int{0, 1, 0, 0, 1}
	r := &RecurseRunner{
		MaxWaves:       5,
		StallWaveLimit: 2,
		NewWave: func(ctx context.Context, wave int, prev *Campaign) (*Campaign, error) {
			return recurseTestWave(fmt.Sprintf("wave-%d", wave), script[wave], 0), nil
		},
	}
	result, err := r.Run(context.Background())
	if err == nil || !errors.Is(err, ErrRecurseStalled) {
		t.Fatalf("err = %v, want stall after waves 3-4", err)
	}
	if result.WavesCompleted != 4 || result.CompletedTasks != 1 {
		t.Fatalf("result = %+v, want 4 waves / 1 completed", result)
	}
}

func TestRecurseRunner_WaveErrorRecordedAndContinues(t *testing.T) {
	r := &RecurseRunner{
		MaxWaves: 2,
		NewWave: func(ctx context.Context, wave int, prev *Campaign) (*Campaign, error) {
			c := recurseTestWave(fmt.Sprintf("wave-%d", wave), 1, 0)
			if wave == 0 {
				return c, errors.New("checkpoint exploded")
			}
			return c, nil
		},
	}
	result, err := r.Run(context.Background())
	if err != nil {
		t.Fatalf("a wave error with progress must not stop the run: %v", err)
	}
	if len(result.WaveErrors) != 1 || !strings.Contains(result.WaveErrors[0], "checkpoint exploded") {
		t.Fatalf("WaveErrors = %v, want the wave 0 error recorded", result.WaveErrors)
	}
	if result.WavesCompleted != 2 || result.CompletedTasks != 2 {
		t.Fatalf("result = %+v, want both waves counted", result)
	}
}

func TestRecurseRunner_UnboundedRequiresYolo(t *testing.T) {
	r := &RecurseRunner{
		NewWave: func(ctx context.Context, wave int, prev *Campaign) (*Campaign, error) {
			return recurseTestWave("x", 1, 0), nil
		},
	}
	if _, err := r.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "yolo") {
		t.Fatalf("unbounded without AllowUnbounded must refuse: %v", err)
	}
}

func TestRecurseRunner_UnboundedRunsUntilStopped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &RecurseRunner{
		AllowUnbounded: true,
		NewWave: func(ctx context.Context, wave int, prev *Campaign) (*Campaign, error) {
			if wave >= 3 {
				cancel()
			}
			return recurseTestWave(fmt.Sprintf("wave-%d", wave), 1, 0), nil
		},
	}
	result, err := r.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if result.WavesCompleted != 3 {
		t.Fatalf("waves = %d, want 3 before the stop", result.WavesCompleted)
	}
}

func TestRecurseLoop_Validate(t *testing.T) {
	if err := (&RecurseLoop{}).Validate(); err == nil {
		t.Fatal("unbounded without yolo must fail")
	}
	if err := (&RecurseLoop{MaxWaves: -1}).Validate(); err == nil {
		t.Fatal("negative waves must fail")
	}
	if err := (&RecurseLoop{StallWaveLimit: -1}).Validate(); err == nil {
		t.Fatal("negative stall limit must fail")
	}
	if err := (&RecurseLoop{MaxWaves: 1}).Validate(); err != nil {
		t.Fatalf("bounded must pass: %v", err)
	}
	if err := (&RecurseLoop{AllowUnbounded: true}).Validate(); err != nil {
		t.Fatalf("unbounded with yolo must pass: %v", err)
	}
}

func TestRecurseLoop_ObserveDecides(t *testing.T) {
	loop := &RecurseLoop{MaxWaves: 3, StallWaveLimit: 2}
	done := recurseTestWave("a", 1, 0)
	empty := recurseTestWave("b", 0, 1)
	if d, _ := loop.Observe(done); d != RecurseProceed {
		t.Fatalf("wave with progress must proceed: %v", d)
	}
	if d, _ := loop.Observe(empty); d != RecurseProceed {
		t.Fatalf("first empty wave must proceed: %v", d)
	}
	if d, _ := loop.Observe(empty); d != RecurseDoneStalled {
		t.Fatalf("second empty wave must stall: %v", d)
	}
	if loop.WavesCompleted() != 3 {
		t.Fatalf("waves = %d, want 3", loop.WavesCompleted())
	}

	bounded := &RecurseLoop{MaxWaves: 1}
	if d, _ := bounded.Observe(done); d != RecurseDoneBounds {
		t.Fatalf("budget spent must end: %v", d)
	}
}

func TestRecurseRunner_NilGuards(t *testing.T) {
	if _, err := (&RecurseRunner{}).Run(context.Background()); err == nil {
		t.Fatal("nil wave func must fail")
	}
	r := &RecurseRunner{
		MaxWaves: 1,
		NewWave:  func(ctx context.Context, wave int, prev *Campaign) (*Campaign, error) { return nil, nil },
	}
	if _, err := r.Run(context.Background()); err == nil {
		t.Fatal("nil finished wave must fail")
	}
	r = &RecurseRunner{
		MaxWaves: -1,
		NewWave:  func(ctx context.Context, wave int, prev *Campaign) (*Campaign, error) { return recurseTestWave("x", 1, 0), nil },
	}
	if _, err := r.Run(context.Background()); err == nil {
		t.Fatal("negative waves must fail")
	}
}
