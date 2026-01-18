package shards

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// MOCK SPAWNER
// =============================================================================

type mockObserverSpawner struct {
	lastObserver string
	lastTask     string
	result       string
	err          error
}

func (m *mockObserverSpawner) SpawnObserver(ctx context.Context, observerName, task string) (string, error) {
	m.lastObserver = observerName
	m.lastTask = task
	return m.result, m.err
}

// =============================================================================
// BACKGROUND OBSERVER MANAGER TESTS
// =============================================================================

func TestNewBackgroundObserverManager(t *testing.T) {
	t.Parallel()

	spawner := &mockObserverSpawner{}
	mgr := NewBackgroundObserverManager(spawner)

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.enabled {
		t.Error("manager should not be enabled by default")
	}
	if mgr.checkInterval != 5*time.Minute {
		t.Errorf("unexpected default checkInterval: %v", mgr.checkInterval)
	}
}

func TestBackgroundObserverManager_StartStop(t *testing.T) {
	spawner := &mockObserverSpawner{}
	mgr := NewBackgroundObserverManager(spawner)

	// Start
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !mgr.enabled {
		t.Error("manager should be enabled after Start")
	}

	// Double start should error
	if err := mgr.Start(); err == nil {
		t.Error("expected error on double start")
	}

	// Stop
	mgr.Stop()
	if mgr.enabled {
		t.Error("manager should be disabled after Stop")
	}
}

func TestBackgroundObserverManager_RegisterObserver(t *testing.T) {
	spawner := &mockObserverSpawner{}
	mgr := NewBackgroundObserverManager(spawner)

	// Valid observer
	if err := mgr.RegisterObserver("northstar"); err != nil {
		t.Errorf("RegisterObserver(northstar) failed: %v", err)
	}

	// Check it's active
	active := mgr.GetActiveObservers()
	if len(active) != 1 || active[0] != "northstar" {
		t.Errorf("expected [northstar], got %v", active)
	}

	// Unknown observer
	if err := mgr.RegisterObserver("unknown"); err == nil {
		t.Error("expected error for unknown observer")
	}

	// Non-observer specialist
	if err := mgr.RegisterObserver("goexpert"); err == nil {
		t.Error("expected error for non-observer specialist")
	}
}

func TestBackgroundObserverManager_UnregisterObserver(t *testing.T) {
	spawner := &mockObserverSpawner{}
	mgr := NewBackgroundObserverManager(spawner)

	_ = mgr.RegisterObserver("northstar")
	mgr.UnregisterObserver("northstar")

	active := mgr.GetActiveObservers()
	if len(active) != 0 {
		t.Errorf("expected empty, got %v", active)
	}
}

func TestBackgroundObserverManager_SendEvent(t *testing.T) {
	spawner := &mockObserverSpawner{result: "SCORE: 85\nVISION: Aligned\nDEVIATIONS: none\nRECOMMENDATIONS: none"}
	mgr := NewBackgroundObserverManager(spawner)

	// Event before start should be ignored
	mgr.SendEvent(ObserverEvent{Type: EventTaskStarted})

	// Start and register
	_ = mgr.Start()
	defer mgr.Stop()
	_ = mgr.RegisterObserver("northstar")

	// Send event
	mgr.SendEvent(ObserverEvent{
		Type:   EventTaskStarted,
		Source: "test",
		Target: "task1",
	})

	// Give goroutines time to process
	time.Sleep(100 * time.Millisecond)
}

func TestGetAssessmentLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		score int
		want  AssessmentLevel
	}{
		{100, LevelProceed},
		{80, LevelProceed},
		{79, LevelNote},
		{60, LevelNote},
		{59, LevelClarify},
		{40, LevelClarify},
		{39, LevelBlock},
		{0, LevelBlock},
	}

	for _, tc := range cases {
		got := GetAssessmentLevel(tc.score)
		if got != tc.want {
			t.Errorf("GetAssessmentLevel(%d) = %v, want %v", tc.score, got, tc.want)
		}
	}
}

func TestFormatAssessment(t *testing.T) {
	t.Parallel()

	assessment := ObserverAssessment{
		ObserverName: "TestObserver",
		Score:        85,
		Level:        LevelProceed,
		VisionMatch:  "Aligned with goals",
		Deviations:   []string{"minor scope"},
		Suggestions:  []string{"add tests"},
	}

	output := FormatAssessment(assessment)

	if output == "" {
		t.Error("expected non-empty output")
	}
	if !contains(output, "TestObserver") {
		t.Error("expected observer name in output")
	}
	if !contains(output, "85") {
		t.Error("expected score in output")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
