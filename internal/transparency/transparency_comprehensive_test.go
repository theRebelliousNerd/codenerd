package transparency

import (
	"errors"
	"strings"
	"testing"

	"codenerd/internal/config"
)

// =============================================================================
// ErrorCategory
// =============================================================================

func TestErrorCategory_Prefix_WhenAllCategories_ShouldReturnCorrectPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cat     ErrorCategory
		wantPfx string
	}{
		{"Safety", ErrorCategorySafety, "[SAFETY]"},
		{"Config", ErrorCategoryConfig, "[CONFIG]"},
		{"API", ErrorCategoryAPI, "[API]"},
		{"Kernel", ErrorCategoryKernel, "[KERNEL]"},
		{"Shard", ErrorCategoryShard, "[SHARD]"},
		{"Filesystem", ErrorCategoryFilesystem, "[FS]"},
		{"Network", ErrorCategoryNetwork, "[NET]"},
		{"Timeout", ErrorCategoryTimeout, "[TIMEOUT]"},
		{"Unknown", ErrorCategoryUnknown, "[ERROR]"},
		{"OutOfRange", ErrorCategory(999), "[ERROR]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.cat.Prefix(); got != tt.wantPfx {
				t.Errorf("Prefix() = %q, want %q", got, tt.wantPfx)
			}
		})
	}
}

func TestErrorCategory_String_WhenAllCategories_ShouldReturnName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cat  ErrorCategory
		want string
	}{
		{ErrorCategorySafety, "safety"},
		{ErrorCategoryConfig, "config"},
		{ErrorCategoryAPI, "api"},
		{ErrorCategoryKernel, "kernel"},
		{ErrorCategoryShard, "shard"},
		{ErrorCategoryFilesystem, "filesystem"},
		{ErrorCategoryNetwork, "network"},
		{ErrorCategoryTimeout, "timeout"},
		{ErrorCategoryUnknown, "unknown"},
		{ErrorCategory(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.cat.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// ClassifyError
// =============================================================================

func TestClassifyError_WhenNil_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	if result := ClassifyError(nil); result != nil {
		t.Errorf("expected nil for nil error, got %v", result)
	}
}

func TestClassifyError_WhenVariousPatterns_ShouldClassifyCorrectly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		errMsg  string
		wantCat ErrorCategory
	}{
		{"Permission", "permission denied", ErrorCategorySafety},
		{"Blocked", "action blocked by constitutional gate", ErrorCategorySafety},
		{"Denied", "access denied", ErrorCategorySafety},
		{"ConfigNotFound", "config not found", ErrorCategoryConfig},
		{"ConfigJson", "configuration error in .json", ErrorCategoryConfig},
		{"RateLimit", "rate limit exceeded", ErrorCategoryAPI},
		{"Unauthorized", "unauthorized access 401", ErrorCategoryAPI},
		{"Quota", "quota exceeded", ErrorCategoryAPI},
		{"Kernel", "kernel initialization failed", ErrorCategoryKernel},
		{"Mangle", "mangle syntax error", ErrorCategoryKernel},
		{"Predicate", "predicate not found", ErrorCategoryKernel},
		{"Shard", "shard execution failed", ErrorCategoryShard},
		{"Executor", "executor not available", ErrorCategoryShard},
		{"FileNotExist", "file does not exist", ErrorCategoryFilesystem},
		{"NoSuchDir", "no such directory", ErrorCategoryFilesystem},
		{"Connection", "connection refused", ErrorCategoryNetwork},
		{"DNS", "dns lookup failed", ErrorCategoryNetwork},
		{"Timeout", "context deadline exceeded timeout", ErrorCategoryTimeout},
		{"DeadlineExceeded", "deadline exceeded", ErrorCategoryTimeout},
		{"Unknown", "some random error", ErrorCategoryUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			classified := ClassifyError(errors.New(tt.errMsg))
			if classified == nil {
				t.Fatal("ClassifyError returned nil")
			}
			if classified.Category != tt.wantCat {
				t.Errorf("category = %s, want %s", classified.Category.String(), tt.wantCat.String())
			}
			if classified.Original == nil {
				t.Error("Original error should be preserved")
			}
			if classified.Summary == "" {
				t.Error("Summary should not be empty")
			}
		})
	}
}

func TestClassifiedError_Error_ShouldContainPrefixAndDetails(t *testing.T) {
	t.Parallel()
	classified := ClassifyError(errors.New("permission denied"))
	errStr := classified.Error()

	if !strings.Contains(errStr, "[SAFETY]") {
		t.Error("expected Error() to contain prefix")
	}
	if !strings.Contains(errStr, "permission denied") {
		t.Error("expected Error() to contain original error message")
	}
}

func TestClassifiedError_Unwrap_ShouldReturnOriginal(t *testing.T) {
	t.Parallel()
	orig := errors.New("permission denied")
	classified := ClassifyError(orig)
	if classified.Unwrap() != orig {
		t.Error("Unwrap should return original error")
	}
}

func TestClassifiedError_Format_WhenHasRemediation_ShouldIncludeSteps(t *testing.T) {
	t.Parallel()
	classified := ClassifyError(errors.New("permission denied"))
	formatted := classified.Format()

	if !strings.Contains(formatted, "Suggested fixes") {
		t.Error("expected remediation steps in format output")
	}
}

// =============================================================================
// GetRecoveryGuide
// =============================================================================

func TestGetRecoveryGuide_WhenKnownCategories_ShouldReturnSteps(t *testing.T) {
	t.Parallel()
	categories := []ErrorCategory{
		ErrorCategorySafety,
		ErrorCategoryConfig,
		ErrorCategoryAPI,
		ErrorCategoryKernel,
		ErrorCategoryShard,
		ErrorCategoryFilesystem,
		ErrorCategoryNetwork,
		ErrorCategoryTimeout,
	}

	for _, cat := range categories {
		t.Run(cat.String(), func(t *testing.T) {
			t.Parallel()
			steps := GetRecoveryGuide(cat)
			if len(steps) == 0 {
				t.Errorf("expected recovery steps for %s", cat.String())
			}
		})
	}
}

func TestGetRecoveryGuide_WhenUnknownCategory_ShouldReturnDefault(t *testing.T) {
	t.Parallel()
	steps := GetRecoveryGuide(ErrorCategory(999))
	if len(steps) == 0 {
		t.Error("expected default recovery steps")
	}
}

// =============================================================================
// ShardPhase
// =============================================================================

func TestShardPhase_String_WhenAllPhases_ShouldReturnName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		phase ShardPhase
		want  string
	}{
		{PhaseIdle, "Idle"},
		{PhaseInitializing, "Initializing"},
		{PhaseLoading, "Loading context"},
		{PhaseAnalyzing, "Analyzing"},
		{PhaseGenerating, "Generating"},
		{PhaseExecuting, "Executing"},
		{PhaseComplete, "Complete"},
		{PhaseFailed, "Failed"},
		{ShardPhase(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.phase.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// ShardObserver
// =============================================================================

func TestShardObserver_StartExecution_WhenEnabled_ShouldTrackAndNotify(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.Enable()

	var received []PhaseUpdate
	obs.AddObserver(&mockObserver{fn: func(u PhaseUpdate) {
		received = append(received, u)
	}})

	obs.StartExecution("shard-1", "coder", "write tests")

	exec := obs.GetExecution("shard-1")
	if exec == nil {
		t.Fatal("expected execution to be tracked")
	}
	if exec.ShardType != "coder" {
		t.Errorf("ShardType = %q, want coder", exec.ShardType)
	}
	if exec.Phase != PhaseInitializing {
		t.Errorf("Phase = %v, want PhaseInitializing", exec.Phase)
	}
	if len(received) != 1 {
		t.Errorf("expected 1 notification, got %d", len(received))
	}
}

func TestShardObserver_StartExecution_WhenDisabled_ShouldTrackButNotNotify(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	// default: disabled

	var received []PhaseUpdate
	obs.AddObserver(&mockObserver{fn: func(u PhaseUpdate) {
		received = append(received, u)
	}})

	obs.StartExecution("shard-1", "coder", "write tests")

	exec := obs.GetExecution("shard-1")
	if exec == nil {
		t.Fatal("expected execution to be tracked even when disabled")
	}
	if len(received) != 0 {
		t.Errorf("expected no notifications when disabled, got %d", len(received))
	}
}

func TestShardObserver_UpdatePhase_ShouldTransitionAndRecordHistory(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.Enable()

	obs.StartExecution("shard-1", "coder", "task")
	obs.UpdatePhase("shard-1", PhaseLoading, "loading context files")

	exec := obs.GetExecution("shard-1")
	if exec.Phase != PhaseLoading {
		t.Errorf("Phase = %v, want PhaseLoading", exec.Phase)
	}
	if exec.Message != "loading context files" {
		t.Errorf("Message = %q, want 'loading context files'", exec.Message)
	}

	history := obs.GetPhaseHistory(10)
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestShardObserver_UpdatePhase_WhenUnknownShardID_ShouldBeNoop(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.Enable()
	obs.UpdatePhase("nonexistent", PhaseLoading, "msg")
	// Should not panic
}

func TestShardObserver_EndExecution_WhenSuccess_ShouldSetComplete(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.Enable()
	obs.StartExecution("shard-1", "coder", "task")
	obs.EndExecution("shard-1", false)

	exec := obs.GetExecution("shard-1")
	if exec.Phase != PhaseComplete {
		t.Errorf("Phase = %v, want PhaseComplete", exec.Phase)
	}
}

func TestShardObserver_EndExecution_WhenFailed_ShouldSetFailed(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.Enable()
	obs.StartExecution("shard-1", "coder", "task")
	obs.EndExecution("shard-1", true)

	exec := obs.GetExecution("shard-1")
	if exec.Phase != PhaseFailed {
		t.Errorf("Phase = %v, want PhaseFailed", exec.Phase)
	}
}

func TestShardObserver_EndExecution_WhenUnknown_ShouldBeNoop(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.EndExecution("nonexistent", false) // Should not panic
}

func TestShardObserver_GetActiveExecutions_ShouldExcludeTerminalPhases(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.Enable()
	obs.StartExecution("active", "coder", "task")
	obs.StartExecution("complete", "tester", "task")
	obs.EndExecution("complete", false)
	obs.StartExecution("failed", "reviewer", "task")
	obs.EndExecution("failed", true)

	active := obs.GetActiveExecutions()
	if len(active) != 1 {
		t.Errorf("expected 1 active execution, got %d", len(active))
	}
	if active[0].ShardID != "active" {
		t.Errorf("expected active shard, got %s", active[0].ShardID)
	}
}

func TestShardObserver_GetExecution_WhenNotFound_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	if obs.GetExecution("nonexistent") != nil {
		t.Error("expected nil for nonexistent shard")
	}
}

func TestShardObserver_SetProgress_ShouldUpdateProgress(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.StartExecution("shard-1", "coder", "task")
	obs.SetProgress("shard-1", 0.75)

	exec := obs.GetExecution("shard-1")
	if exec.Progress != 0.75 {
		t.Errorf("Progress = %f, want 0.75", exec.Progress)
	}
}

func TestShardObserver_GetPhaseHistory_WhenLimitLargerThanHistory_ShouldReturnAll(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.Enable()
	obs.StartExecution("s1", "coder", "task")
	obs.UpdatePhase("s1", PhaseLoading, "")

	history := obs.GetPhaseHistory(100)
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestShardObserver_GetPhaseHistory_WhenZeroLimit_ShouldReturnAll(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.Enable()
	obs.StartExecution("s1", "coder", "task")
	obs.UpdatePhase("s1", PhaseLoading, "")

	history := obs.GetPhaseHistory(0)
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestShardObserver_ClearHistory_ShouldEmptyHistory(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.Enable()
	obs.StartExecution("s1", "coder", "task")
	obs.UpdatePhase("s1", PhaseLoading, "")
	obs.ClearHistory()

	history := obs.GetPhaseHistory(10)
	if len(history) != 0 {
		t.Errorf("expected 0 history entries after clear, got %d", len(history))
	}
}

func TestShardObserver_FormatExecutionSummary_WhenNoActive_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	if obs.FormatExecutionSummary() != "" {
		t.Error("expected empty string for no active executions")
	}
}

func TestShardObserver_FormatExecutionSummary_WhenActive_ShouldReturnStatusLines(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.StartExecution("shard-1", "coder", "write tests")

	summary := obs.FormatExecutionSummary()
	if !strings.Contains(summary, "coder") {
		t.Errorf("expected summary to contain shard type, got %q", summary)
	}
}

func TestShardExecution_StatusLine_WhenWithMessage_ShouldIncludeMessage(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.StartExecution("shard-1", "coder", "task")
	obs.UpdatePhase("shard-1", PhaseLoading, "loading context")

	exec := obs.GetExecution("shard-1")
	status := exec.StatusLine()
	if !strings.Contains(status, "loading context") {
		t.Errorf("expected status line to contain message, got %q", status)
	}
}

// =============================================================================
// SafetyReporter
// =============================================================================

func TestSafetyReporter_ReportViolation_WhenDestructive_ShouldClassifyCorrectly(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	v := r.ReportViolation("rm -rf /", "/", "safety_rule")

	if v == nil {
		t.Fatal("expected violation")
	}
	if v.ViolationType != ViolationDestructiveAction {
		t.Errorf("ViolationType = %v, want ViolationDestructiveAction", v.ViolationType)
	}
}

func TestSafetyReporter_ReportViolation_WhenSecret_ShouldClassifyCorrectly(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	v := r.ReportViolation("read", ".env", "safety_rule")

	if v.ViolationType != ViolationSecretExposure {
		t.Errorf("ViolationType = %v, want ViolationSecretExposure", v.ViolationType)
	}
}

func TestSafetyReporter_ReportViolation_WhenProtectedPath_ShouldClassifyCorrectly(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	v := r.ReportViolation("modify", ".git/config", "safety_rule")

	if v.ViolationType != ViolationProtectedPath {
		t.Errorf("ViolationType = %v, want ViolationProtectedPath", v.ViolationType)
	}
}

func TestSafetyReporter_ReportViolation_WhenResourceLimit_ShouldClassifyCorrectly(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	v := r.ReportViolation("allocate", "memory", "resource_limit_exceeded")

	if v.ViolationType != ViolationResourceLimit {
		t.Errorf("ViolationType = %v, want ViolationResourceLimit", v.ViolationType)
	}
}

func TestSafetyReporter_ReportViolation_WhenPolicy_ShouldClassifyCorrectly(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	v := r.ReportViolation("exec", "/bin/bash", "not permitted by policy")

	if v.ViolationType != ViolationPolicyRule {
		t.Errorf("ViolationType = %v, want ViolationPolicyRule", v.ViolationType)
	}
}

func TestSafetyReporter_ReportViolation_WhenDisabled_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	r.Disable()

	v := r.ReportViolation("rm -rf /", "/", "safety")
	if v != nil {
		t.Error("expected nil violation when disabled")
	}
}

func TestSafetyReporter_GetRecentViolations_ShouldReturnMostRecent(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()

	for range 5 {
		r.ReportViolation("action", "target", "rule")
	}

	recent := r.GetRecentViolations(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent violations, got %d", len(recent))
	}
}

func TestSafetyReporter_GetRecentViolations_WhenZeroOrNegativeLimit_ShouldReturnAll(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	r.ReportViolation("action", "target", "rule")
	r.ReportViolation("action2", "target2", "rule2")

	all := r.GetRecentViolations(0)
	if len(all) != 2 {
		t.Errorf("expected 2 violations, got %d", len(all))
	}

	neg := r.GetRecentViolations(-1)
	if len(neg) != 2 {
		t.Errorf("expected 2 violations with negative limit, got %d", len(neg))
	}
}

func TestSafetyReporter_GetViolation_WhenFound_ShouldReturnViolation(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	v := r.ReportViolation("action", "target", "rule")

	found := r.GetViolation(v.ID)
	if found == nil {
		t.Fatal("expected to find violation by ID")
	}
	if found.Action != "action" {
		t.Errorf("Action = %q, want 'action'", found.Action)
	}
}

func TestSafetyReporter_GetViolation_WhenNotFound_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	if r.GetViolation("nonexistent") != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestSafetyReporter_FormatViolation_WhenNil_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	if r.FormatViolation(nil) != "" {
		t.Error("expected empty string for nil violation")
	}
}

func TestSafetyReporter_FormatViolation_WhenValid_ShouldContainSections(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	v := r.ReportViolation("rm -rf /", "/etc", "safety_rule")

	formatted := r.FormatViolation(v)
	if !strings.Contains(formatted, "[SAFETY]") {
		t.Error("expected [SAFETY] header")
	}
	if !strings.Contains(formatted, "How to proceed") {
		t.Error("expected remediation section")
	}
	if !strings.Contains(formatted, "Why was this blocked") {
		t.Error("expected explanation section")
	}
}

func TestSafetyReporter_ClearHistory_ShouldEmptyViolations(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()
	r.ReportViolation("action", "target", "rule")
	r.ClearHistory()

	if len(r.GetRecentViolations(10)) != 0 {
		t.Error("expected empty violations after clear")
	}
}

// =============================================================================
// SafetyViolationType
// =============================================================================

func TestSafetyViolationType_String_WhenAllTypes_ShouldReturnName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		typ  SafetyViolationType
		want string
	}{
		{ViolationDestructiveAction, "Destructive Action"},
		{ViolationProtectedPath, "Protected Path"},
		{ViolationSecretExposure, "Secret Exposure"},
		{ViolationResourceLimit, "Resource Limit"},
		{ViolationPolicyRule, "Policy Rule"},
		{ViolationUnauthorized, "Unauthorized"},
		{ViolationUnknown, "Unknown"},
		{SafetyViolationType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// ExplainSafetyAction
// =============================================================================

func TestExplainSafetyAction_WhenDestructive_ShouldShowRisks(t *testing.T) {
	t.Parallel()
	result := ExplainSafetyAction("rm -rf /tmp")
	if !strings.Contains(result, "Destructive") {
		t.Error("expected destructive risk")
	}
	if !strings.Contains(result, "Potential Risks") {
		t.Error("expected risks section")
	}
}

func TestExplainSafetyAction_WhenForce_ShouldShowForceRisk(t *testing.T) {
	t.Parallel()
	result := ExplainSafetyAction("git push -f")
	if !strings.Contains(result, "Force flag") {
		t.Error("expected force flag risk")
	}
}

func TestExplainSafetyAction_WhenSudo_ShouldShowElevatedRisk(t *testing.T) {
	t.Parallel()
	result := ExplainSafetyAction("sudo apt install")
	if !strings.Contains(result, "Elevated privileges") {
		t.Error("expected elevated privileges risk")
	}
}

func TestExplainSafetyAction_WhenSafe_ShouldShowLowRisk(t *testing.T) {
	t.Parallel()
	result := ExplainSafetyAction("echo hello")
	if !strings.Contains(result, "Low") {
		t.Error("expected low risk level")
	}
	if !strings.Contains(result, "appears safe") {
		t.Error("expected safe assessment")
	}
}

func TestExplainSafetyAction_WhenMultipleRisks_ShouldShowHighRisk(t *testing.T) {
	t.Parallel()
	result := ExplainSafetyAction("sudo rm -rf -f /secret")
	if !strings.Contains(result, "High") {
		t.Error("expected high risk level for multiple risks")
	}
}

// =============================================================================
// TransparencyManager - Extended tests
// =============================================================================

func TestTransparencyManager_WhenNilConfig_ShouldUseDefaults(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil)
	if tm.IsEnabled() {
		t.Error("expected disabled by default")
	}
	cfg := tm.GetConfig()
	if cfg == nil {
		t.Fatal("config should not be nil")
	}
	if !cfg.ShardPhases {
		t.Error("expected ShardPhases=true by default")
	}
}

func TestTransparencyManager_Enable_ShouldCascadeToSubComponents(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil)
	tm.Enable()

	if !tm.IsEnabled() {
		t.Error("expected enabled")
	}
	if !tm.ShardObserver().IsEnabled() {
		t.Error("expected shard observer enabled when ShardPhases=true")
	}
}

func TestTransparencyManager_Disable_ShouldCascadeToSubComponents(t *testing.T) {
	t.Parallel()
	cfg := &config.TransparencyConfig{
		Enabled:            true,
		ShardPhases:        true,
		SafetyExplanations: true,
		VerboseErrors:      true,
	}
	tm := NewTransparencyManager(cfg)
	tm.Disable()

	if tm.IsEnabled() {
		t.Error("expected disabled")
	}
	if tm.ShardObserver().IsEnabled() {
		t.Error("expected shard observer disabled")
	}
}

func TestTransparencyManager_Toggle_ShouldAlternateBetweenStates(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil)

	// First toggle: disabled -> enabled
	result := tm.Toggle()
	if !result {
		t.Error("expected toggle to return true (enabled)")
	}
	if !tm.IsEnabled() {
		t.Error("expected enabled after toggle")
	}

	// Second toggle: enabled -> disabled
	result = tm.Toggle()
	if result {
		t.Error("expected toggle to return false (disabled)")
	}
	if tm.IsEnabled() {
		t.Error("expected disabled after second toggle")
	}
}

func TestTransparencyManager_StartShard_WhenEnabled_ShouldTrack(t *testing.T) {
	t.Parallel()
	cfg := &config.TransparencyConfig{
		Enabled:     true,
		ShardPhases: true,
	}
	tm := NewTransparencyManager(cfg)

	tm.StartShard("shard-1", "coder", "write tests")
	exec := tm.ShardObserver().GetExecution("shard-1")
	if exec == nil {
		t.Fatal("expected shard to be tracked")
	}
}

// The old form of this test asserted that a disabled manager tracks nothing.
// That assertion encoded the bug: transparency is off by default, so gating the
// feed on the master toggle meant `/transparency on` mid-run showed an empty
// Active Operations list for every shard already in flight. Tracking is now
// gated on ShardPhases alone; the master toggle still gates notifications and
// phase history, which is where the cost and the noise live.
func TestTransparencyManager_StartShard_WhenDisabled_ShouldTrackButNotNotify(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil) // disabled by default, ShardPhases on

	obs := &capturingPhaseObserver{}
	tm.ShardObserver().AddObserver(obs)

	tm.StartShard("shard-1", "coder", "write tests")
	tm.UpdateShardPhase("shard-1", PhaseExecuting, "running")

	exec := tm.ShardObserver().GetExecution("shard-1")
	if exec == nil {
		t.Fatal("expected shard tracked even while transparency is off")
	}
	if exec.Phase != PhaseExecuting {
		t.Errorf("expected phase to advance, got %s", exec.Phase)
	}
	if len(obs.updates) != 0 {
		t.Errorf("expected no notifications while disabled, got %d", len(obs.updates))
	}
	if len(tm.ShardObserver().GetPhaseHistory(0)) != 0 {
		t.Error("expected no phase history while disabled")
	}
}

func TestTransparencyManager_StartShard_WhenShardPhasesOff_ShouldNotTrack(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(&config.TransparencyConfig{Enabled: true, ShardPhases: false})

	tm.StartShard("shard-1", "coder", "write tests")
	if tm.ShardObserver().GetExecution("shard-1") != nil {
		t.Error("expected no tracking when ShardPhases is off")
	}
}

type capturingPhaseObserver struct {
	updates []PhaseUpdate
}

func (c *capturingPhaseObserver) OnPhaseChange(update PhaseUpdate) {
	c.updates = append(c.updates, update)
}

func TestTransparencyManager_UpdateShardPhase_WhenDisabled_ShouldBeNoop(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil)
	tm.UpdateShardPhase("shard-1", PhaseLoading, "msg")
	// Should not panic
}

func TestTransparencyManager_EndShard_WhenDisabled_ShouldBeNoop(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil)
	tm.EndShard("shard-1", false)
	// Should not panic
}

func TestTransparencyManager_ReportSafetyViolation_WhenEnabled_ShouldReturnViolation(t *testing.T) {
	t.Parallel()
	cfg := &config.TransparencyConfig{
		Enabled:            true,
		SafetyExplanations: true,
	}
	tm := NewTransparencyManager(cfg)
	v := tm.ReportSafetyViolation("rm -rf /", "/", "safety_gate")

	if v == nil {
		t.Fatal("expected violation returned")
	}
}

func TestTransparencyManager_ReportSafetyViolation_WhenDisabled_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil)
	v := tm.ReportSafetyViolation("rm -rf /", "/", "safety_gate")

	if v != nil {
		t.Error("expected nil violation when disabled")
	}
}

func TestTransparencyManager_FormatError_WhenNilError_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil)
	if tm.FormatError(nil) != "" {
		t.Error("expected empty string for nil error")
	}
}

func TestTransparencyManager_FormatError_WhenEnabled_ShouldIncludeSuggestions(t *testing.T) {
	t.Parallel()
	cfg := &config.TransparencyConfig{
		Enabled:       true,
		VerboseErrors: true,
	}
	tm := NewTransparencyManager(cfg)
	formatted := tm.FormatError(errors.New("permission denied"))

	if !strings.Contains(formatted, "Suggested fixes") {
		t.Error("expected verbose error with suggested fixes")
	}
}

func TestTransparencyManager_FormatError_WhenDisabled_ShouldReturnSimpleFormat(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil)
	formatted := tm.FormatError(errors.New("permission denied"))

	if strings.Contains(formatted, "Suggested fixes") {
		t.Error("expected simple format without suggested fixes when disabled")
	}
	if !strings.Contains(formatted, "permission denied") {
		t.Error("expected error message in output")
	}
}

func TestTransparencyManager_GetStatus_ShouldContainFeatureTable(t *testing.T) {
	t.Parallel()
	cfg := &config.TransparencyConfig{
		Enabled:     true,
		ShardPhases: true,
	}
	tm := NewTransparencyManager(cfg)

	status := tm.GetStatus()
	if !strings.Contains(status, "Transparency Status") {
		t.Error("expected status header")
	}
	if !strings.Contains(status, "Feature Flags") {
		t.Error("expected feature flags section")
	}
}

func TestTransparencyManager_SafetyReporter_ShouldNotBeNil(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(nil)
	if tm.SafetyReporter() == nil {
		t.Error("expected non-nil SafetyReporter")
	}
}

// =============================================================================
// Test helpers
// =============================================================================

type mockObserver struct {
	fn func(PhaseUpdate)
}

func (m *mockObserver) OnPhaseChange(update PhaseUpdate) {
	m.fn(update)
}
