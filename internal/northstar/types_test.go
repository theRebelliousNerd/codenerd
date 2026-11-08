package northstar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// VISION TYPE TESTS
// =============================================================================

func TestVision_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	vision := Vision{
		Mission:    "Test mission",
		Problem:    "Test problem",
		VisionStmt: "Test vision statement",
		Personas: []Persona{
			{Name: "Developer", PainPoints: []string{"complexity"}, Needs: []string{"simplicity"}},
		},
		Capabilities: []Capability{
			{ID: "cap-1", Description: "Core feature", Timeline: "now", Priority: "critical"},
		},
		Risks: []Risk{
			{ID: "risk-1", Description: "Technical debt", Likelihood: "medium", Impact: "high", Mitigation: "Regular refactoring"},
		},
		Requirements: []Requirement{
			{ID: "req-1", Type: "functional", Description: "Must do X", Priority: "must_have"},
		},
		Constraints: []string{"No external dependencies"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(vision)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Vision
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Mission != vision.Mission {
		t.Errorf("Mission mismatch: got %q, want %q", decoded.Mission, vision.Mission)
	}
	if decoded.Problem != vision.Problem {
		t.Errorf("Problem mismatch: got %q, want %q", decoded.Problem, vision.Problem)
	}
	if len(decoded.Personas) != len(vision.Personas) {
		t.Errorf("Personas length mismatch: got %d, want %d", len(decoded.Personas), len(vision.Personas))
	}
	if len(decoded.Capabilities) != len(vision.Capabilities) {
		t.Errorf("Capabilities length mismatch: got %d, want %d", len(decoded.Capabilities), len(vision.Capabilities))
	}
	if len(decoded.Risks) != len(vision.Risks) {
		t.Errorf("Risks length mismatch: got %d, want %d", len(decoded.Risks), len(vision.Risks))
	}
	if len(decoded.Requirements) != len(vision.Requirements) {
		t.Errorf("Requirements length mismatch: got %d, want %d", len(decoded.Requirements), len(vision.Requirements))
	}
}

func TestVision_EmptyFields(t *testing.T) {
	t.Parallel()

	vision := Vision{}

	data, err := json.Marshal(vision)
	if err != nil {
		t.Fatalf("Marshal empty vision error: %v", err)
	}

	var decoded Vision
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal empty vision error: %v", err)
	}

	if decoded.Mission != "" {
		t.Errorf("expected empty Mission, got %q", decoded.Mission)
	}
	if len(decoded.Personas) > 0 {
		t.Errorf("expected empty Personas, got %v", decoded.Personas)
	}
}

// =============================================================================
// PERSONA TYPE TESTS
// =============================================================================

func TestPersona_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		persona Persona
	}{
		{
			name: "full_persona",
			persona: Persona{
				Name:       "Power User",
				PainPoints: []string{"slow performance", "complex UI"},
				Needs:      []string{"speed", "simplicity"},
			},
		},
		{
			name:    "empty_persona",
			persona: Persona{},
		},
		{
			name: "name_only",
			persona: Persona{
				Name: "Basic User",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tc.persona)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var decoded Persona
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			if decoded.Name != tc.persona.Name {
				t.Errorf("Name mismatch: got %q, want %q", decoded.Name, tc.persona.Name)
			}
		})
	}
}

// =============================================================================
// CAPABILITY TYPE TESTS
// =============================================================================

func TestCapability_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	cap := Capability{
		ID:          "cap-test-1",
		Description: "Test capability",
		Timeline:    "now",
		Priority:    "high",
	}

	data, err := json.Marshal(cap)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Capability
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != cap.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, cap.ID)
	}
	if decoded.Timeline != cap.Timeline {
		t.Errorf("Timeline mismatch: got %q, want %q", decoded.Timeline, cap.Timeline)
	}
}

// =============================================================================
// RISK TYPE TESTS
// =============================================================================

func TestRisk_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	risk := Risk{
		ID:          "risk-test-1",
		Description: "Test risk",
		Likelihood:  "high",
		Impact:      "critical",
		Mitigation:  "Test mitigation",
	}

	data, err := json.Marshal(risk)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Risk
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != risk.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, risk.ID)
	}
	if decoded.Mitigation != risk.Mitigation {
		t.Errorf("Mitigation mismatch: got %q, want %q", decoded.Mitigation, risk.Mitigation)
	}
}

// =============================================================================
// OBSERVATION TYPE TESTS
// =============================================================================

func TestObservationType_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		obsType  ObservationType
		expected string
	}{
		{ObsTaskCompleted, "task_completed"},
		{ObsFileChanged, "file_changed"},
		{ObsDecisionMade, "decision_made"},
		{ObsPatternDetected, "pattern_detected"},
		{ObsDriftWarning, "drift_warning"},
		{ObsAlignmentSuccess, "alignment_success"},
		{ObsRiskTriggered, "risk_triggered"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			if string(tc.obsType) != tc.expected {
				t.Errorf("ObservationType value mismatch: got %q, want %q", tc.obsType, tc.expected)
			}
		})
	}
}

func TestObservation_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	obs := Observation{
		ID:        "obs-1",
		SessionID: "session-123",
		Timestamp: now,
		Type:      ObsTaskCompleted,
		Subject:   "Test subject",
		Content:   "Test content",
		Relevance: 0.85,
		Tags:      []string{"test", "example"},
		Metadata:  map[string]string{"key": "value"},
	}

	data, err := json.Marshal(obs)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Observation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != obs.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, obs.ID)
	}
	if decoded.Type != obs.Type {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, obs.Type)
	}
	if decoded.Relevance != obs.Relevance {
		t.Errorf("Relevance mismatch: got %f, want %f", decoded.Relevance, obs.Relevance)
	}
}

// =============================================================================
// ALIGNMENT CHECK TESTS
// =============================================================================

func TestAlignmentTrigger_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		trigger  AlignmentTrigger
		expected string
	}{
		{TriggerManual, "manual"},
		{TriggerPhaseGate, "phase_gate"},
		{TriggerPeriodic, "periodic"},
		{TriggerHighImpact, "high_impact"},
		{TriggerTaskComplete, "task_complete"},
		{TriggerSessionStart, "session_start"},
		{TriggerCampaignStart, "campaign_start"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			if string(tc.trigger) != tc.expected {
				t.Errorf("AlignmentTrigger value mismatch: got %q, want %q", tc.trigger, tc.expected)
			}
		})
	}
}

func TestAlignmentResult_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		result   AlignmentResult
		expected string
	}{
		{AlignmentPassed, "passed"},
		{AlignmentWarning, "warning"},
		{AlignmentFailed, "failed"},
		{AlignmentBlocked, "blocked"},
		{AlignmentSkipped, "skipped"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			if string(tc.result) != tc.expected {
				t.Errorf("AlignmentResult value mismatch: got %q, want %q", tc.result, tc.expected)
			}
		})
	}
}

func TestAlignmentCheck_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	check := AlignmentCheck{
		ID:          "check-1",
		Timestamp:   now,
		Trigger:     TriggerManual,
		Subject:     "Test subject",
		Context:     "Test context",
		Result:      AlignmentPassed,
		Score:       0.95,
		Explanation: "All aligned",
		Suggestions: []string{"Continue as is"},
		Duration:    5 * time.Second,
	}

	data, err := json.Marshal(check)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded AlignmentCheck
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != check.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, check.ID)
	}
	if decoded.Score != check.Score {
		t.Errorf("Score mismatch: got %f, want %f", decoded.Score, check.Score)
	}
	if decoded.Result != check.Result {
		t.Errorf("Result mismatch: got %q, want %q", decoded.Result, check.Result)
	}
}

// =============================================================================
// DRIFT EVENT TESTS
// =============================================================================

func TestDriftSeverity_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		severity DriftSeverity
		expected string
	}{
		{DriftMinor, "minor"},
		{DriftModerate, "moderate"},
		{DriftMajor, "major"},
		{DriftCritical, "critical"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			if string(tc.severity) != tc.expected {
				t.Errorf("DriftSeverity value mismatch: got %q, want %q", tc.severity, tc.expected)
			}
		})
	}
}

func TestDriftEvent_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	event := DriftEvent{
		ID:           "drift-1",
		Timestamp:    now,
		Severity:     DriftMajor,
		Category:     "architecture",
		Description:  "Test drift",
		Evidence:     []string{"evidence-1", "evidence-2"},
		RelatedCheck: "check-1",
		Resolved:     false,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded DriftEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != event.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, event.ID)
	}
	if decoded.Severity != event.Severity {
		t.Errorf("Severity mismatch: got %q, want %q", decoded.Severity, event.Severity)
	}
	if len(decoded.Evidence) != len(event.Evidence) {
		t.Errorf("Evidence length mismatch: got %d, want %d", len(decoded.Evidence), len(event.Evidence))
	}
}

func TestDriftEvent_Resolved(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	resolvedAt := now.Add(time.Hour)
	event := DriftEvent{
		ID:         "drift-resolved",
		Timestamp:  now,
		Severity:   DriftMinor,
		Resolved:   true,
		ResolvedAt: &resolvedAt,
		Resolution: "Fixed the issue",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded DriftEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if !decoded.Resolved {
		t.Error("expected Resolved to be true")
	}
	if decoded.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
	if decoded.Resolution != event.Resolution {
		t.Errorf("Resolution mismatch: got %q, want %q", decoded.Resolution, event.Resolution)
	}
}

// =============================================================================
// GUARDIAN CONFIG TESTS
// =============================================================================

func TestDefaultGuardianConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultGuardianConfig()

	if cfg.PeriodicCheckInterval != 5 {
		t.Errorf("PeriodicCheckInterval: got %d, want 5", cfg.PeriodicCheckInterval)
	}
	if !cfg.EnablePhaseGates {
		t.Error("EnablePhaseGates should be true by default")
	}
	if !cfg.EnablePeriodicCheck {
		t.Error("EnablePeriodicCheck should be true by default")
	}
	if !cfg.EnableHighImpact {
		t.Error("EnableHighImpact should be true by default")
	}
	if len(cfg.HighImpactPaths) != 0 {
		t.Errorf("the default watches no paths; a workspace declares its own (GuardianConfigFor): %v", cfg.HighImpactPaths)
	}
	if cfg.WarningThreshold != 0.7 {
		t.Errorf("WarningThreshold: got %f, want 0.7", cfg.WarningThreshold)
	}
	if cfg.FailureThreshold != 0.5 {
		t.Errorf("FailureThreshold: got %f, want 0.5", cfg.FailureThreshold)
	}
	if cfg.BlockThreshold != 0.3 {
		t.Errorf("BlockThreshold: got %f, want 0.3", cfg.BlockThreshold)
	}
}

func TestGuardianConfig_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := GuardianConfig{
		PeriodicCheckInterval: 10,
		EnablePhaseGates:      false,
		EnablePeriodicCheck:   true,
		EnableHighImpact:      false,
		HighImpactPaths:       []string{"custom/path/"},
		WarningThreshold:      0.8,
		FailureThreshold:      0.6,
		BlockThreshold:        0.4,
		AlignmentModel:        "test-model",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded GuardianConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.PeriodicCheckInterval != cfg.PeriodicCheckInterval {
		t.Errorf("PeriodicCheckInterval mismatch: got %d, want %d", decoded.PeriodicCheckInterval, cfg.PeriodicCheckInterval)
	}
	if decoded.EnablePhaseGates != cfg.EnablePhaseGates {
		t.Errorf("EnablePhaseGates mismatch: got %v, want %v", decoded.EnablePhaseGates, cfg.EnablePhaseGates)
	}
	if decoded.AlignmentModel != cfg.AlignmentModel {
		t.Errorf("AlignmentModel mismatch: got %q, want %q", decoded.AlignmentModel, cfg.AlignmentModel)
	}
}

// =============================================================================
// GUARDIAN STATE TESTS
// =============================================================================

func TestGuardianState_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	state := GuardianState{
		VisionDefined:       true,
		LastCheck:           now,
		TasksSinceCheck:     3,
		ActiveDriftCount:    1,
		OverallAlignment:    0.85,
		SessionObservations: 10,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded GuardianState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if !decoded.VisionDefined {
		t.Error("VisionDefined should be true")
	}
	if decoded.TasksSinceCheck != state.TasksSinceCheck {
		t.Errorf("TasksSinceCheck mismatch: got %d, want %d", decoded.TasksSinceCheck, state.TasksSinceCheck)
	}
	if decoded.OverallAlignment != state.OverallAlignment {
		t.Errorf("OverallAlignment mismatch: got %f, want %f", decoded.OverallAlignment, state.OverallAlignment)
	}
}

func TestGuardianState_DefaultValues(t *testing.T) {
	t.Parallel()

	state := GuardianState{}

	if state.VisionDefined {
		t.Error("VisionDefined should default to false")
	}
	if state.TasksSinceCheck != 0 {
		t.Errorf("TasksSinceCheck should default to 0, got %d", state.TasksSinceCheck)
	}
	if state.OverallAlignment != 0 {
		t.Errorf("OverallAlignment should default to 0, got %f", state.OverallAlignment)
	}
}

// =============================================================================
// REQUIREMENT TYPE TESTS
// =============================================================================

func TestRequirement_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	req := Requirement{
		ID:          "req-test-1",
		Type:        "functional",
		Description: "Test requirement",
		Priority:    "must_have",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Requirement
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != req.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, req.ID)
	}
	if decoded.Type != req.Type {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, req.Type)
	}
	if decoded.Priority != req.Priority {
		t.Errorf("Priority mismatch: got %q, want %q", decoded.Priority, req.Priority)
	}
}

// High-impact paths are the workspace's nerd.md critical: list, matched on
// whole path segments.
func TestGuardianConfigFor_WatchesTheWorkspacesCriticalPaths(t *testing.T) {
	ws := t.TempDir()
	if got := GuardianConfigFor(ws).HighImpactPaths; len(got) != 0 {
		t.Fatalf("no nerd.md, no high-impact paths: %v", got)
	}
	body := "---\nschema: nerd/v1\ncritical:\n  - internal/core\n  - \"*.mg\"\n---\n"
	if err := os.WriteFile(filepath.Join(ws, "nerd.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := GuardianConfigFor(ws)
	if len(cfg.HighImpactPaths) != 2 {
		t.Fatalf("HighImpactPaths = %v", cfg.HighImpactPaths)
	}
	for p, want := range map[string]bool{
		"internal/core/kernel.go": true,
		"policy/rules.mg":         true,
		"internal/corex/a.go":     false,
		"internal/session/s.go":   false,
	} {
		got := false
		for _, pattern := range cfg.HighImpactPaths {
			got = got || matchesHighImpactPath(pattern, p)
		}
		if got != want {
			t.Errorf("%s high-impact = %v, want %v", p, got, want)
		}
	}
}

func TestStabilize_PersonaFactID_TrimsPrefixesAndEmpty(t *testing.T) {
	t.Parallel()
	if got := PersonaFactID(""); got != "" {
		t.Fatalf("empty should stay empty, got %q", got)
	}
	if got := PersonaFactID("  "); got != "" {
		t.Fatalf("blank should stay empty, got %q", got)
	}
	if got := PersonaFactID("  ada "); got != "persona_ada" {
		t.Fatalf("should trim and prefix, got %q", got)
	}
	if got := PersonaFactID("persona_ada"); got != "persona_ada" {
		t.Fatalf("should not double-prefix, got %q", got)
	}
}

func TestStabilize_AlignmentResultAssessed_SeparatesVerdicts(t *testing.T) {
	t.Parallel()
	for _, r := range []AlignmentResult{AlignmentPassed, AlignmentWarning, AlignmentFailed, AlignmentBlocked} {
		if !r.Assessed() {
			t.Fatalf("%q should count as assessed", r)
		}
	}
	for _, r := range []AlignmentResult{AlignmentSkipped, AlignmentUnknown, AlignmentResult("other")} {
		if r.Assessed() {
			t.Fatalf("%q should not count as assessed", r)
		}
	}
}

func TestStabilize_MitigationStrategyAtom_StableDistinctAndSafe(t *testing.T) {
	t.Parallel()
	a1 := string(MitigationStrategyAtom("retry with backoff"))
	a2 := string(MitigationStrategyAtom("retry with backoff"))
	if a1 != a2 {
		t.Fatalf("same text should give same atom: %q vs %q", a1, a2)
	}
	a3 := string(MitigationStrategyAtom("retry with backoff!"))
	if a1 == a3 {
		t.Fatalf("different text should give different atoms: %q", a1)
	}
	for _, a := range []string{a1, a3, string(MitigationStrategyAtom("  "))} {
		if len(a) < 5 || a[:5] != "/mit_" {
			t.Fatalf("atom should have /mit_ prefix, got %q", a)
		}
		for _, r := range a {
			isLower := r >= 'a' && r <= 'z'
			isDigit := r >= '0' && r <= '9'
			if r != '/' && r != '_' && !isLower && !isDigit {
				t.Fatalf("atom %q has unsafe rune %q", a, r)
			}
		}
	}
}

func TestStabilize_IsNil_CoversTypedNils(t *testing.T) {
	t.Parallel()
	var p *Guardian
	var m map[string]struct{}
	var s []string
	var ch chan int
	var fn func()
	if !IsNil(nil) {
		t.Fatal("nil interface should be nil")
	}
	if !IsNil(p) || !IsNil(m) || !IsNil(s) || !IsNil(ch) || !IsNil(fn) {
		t.Fatal("typed nils should report nil")
	}
	if IsNil(0) || IsNil("") || IsNil(struct{}{}) {
		t.Fatal("non-nil values should not report nil")
	}
	if IsNil(&Guardian{}) {
		t.Fatal("non-nil pointer should not report nil")
	}
}

func TestStabilize_SortedResults_StableOrder(t *testing.T) {
	t.Parallel()
	m := &AlignmentMetrics{ChecksByResult: map[AlignmentResult]int{AlignmentFailed: 1, AlignmentPassed: 2, AlignmentBlocked: 3}}
	got := m.SortedResults()
	want := []AlignmentResult{AlignmentBlocked, AlignmentFailed, AlignmentPassed}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	var nilMapMetrics *AlignmentMetrics
	_ = nilMapMetrics
	empty := &AlignmentMetrics{}
	if out := empty.SortedResults(); len(out) != 0 {
		t.Fatalf("empty metrics should sort to empty, got %v", out)
	}
}

func TestStabilize_Clamp01AndTermOverlap_Boundaries(t *testing.T) {
	t.Parallel()
	if clamp01(-0.5) != 0 || clamp01(1.5) != 1 || clamp01(0.25) != 0.25 {
		t.Fatalf("clamp01 should bound to [0,1]")
	}
	if termOverlap("anything", "") != 0 {
		t.Fatal("empty source should overlap 0")
	}
	if termOverlap("anything", "a an of") != 0 {
		t.Fatal("short-only source should overlap 0")
	}
	full := termOverlap("the guardian watches critical paths", "guardian critical paths")
	if full != 1 {
		t.Fatalf("full overlap should be 1, got %v", full)
	}
	partial := termOverlap("guardian only", "guardian critical paths")
	if partial <= 0 || partial >= 1 {
		t.Fatalf("partial overlap should be in (0,1), got %v", partial)
	}
	dup := termOverlap("guardian", "guardian guardian paths paths")
	if dup != 0.5 {
		t.Fatalf("duplicate source terms should dedupe, got %v", dup)
	}
}

func TestStabilize_MintID_LowestFreeCaseInsensitive(t *testing.T) {
	t.Parallel()
	if got := MintID("cap_", nil); got != "cap_1" {
		t.Fatalf("empty pool should mint cap_1, got %q", got)
	}
	if got := MintID("cap_", []string{"cap_1", "CAP_2", "cap_4"}); got != "cap_3" {
		t.Fatalf("should fill lowest free gap case-insensitively, got %q", got)
	}
}

func TestStabilize_CosineSimilarity_IdentityAndOrthogonal(t *testing.T) {
	t.Parallel()
	if got := CosineSimilarity([]float32{1, 0}, []float32{0, 1}); got != 0 {
		t.Fatalf("orthogonal should be 0, got %v", got)
	}
	if got := CosineSimilarity([]float32{1, 2}, []float32{1, 2}); got < 0.999999 || got > 1.000001 {
		t.Fatalf("identical should be ~1, got %v", got)
	}
	if got := CosineSimilarity(nil, []float32{1}); got != 0 {
		t.Fatalf("empty should be 0, got %v", got)
	}
	if got := CosineSimilarity([]float32{1}, []float32{1, 2}); got != 0 {
		t.Fatalf("mismatched lengths should be 0, got %v", got)
	}
}
