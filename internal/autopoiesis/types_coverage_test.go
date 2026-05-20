package autopoiesis

import (
	"testing"
)

// --- LoopStage.String ---

func TestLoopStage_String_AllStages(t *testing.T) {
	tests := []struct {
		stage LoopStage
		want  string
	}{
		{StageDetection, "detection"},
		{StageSpecification, "specification"},
		{StageSafetyCheck, "safety_check"},
		{StageThunderdome, "thunderdome"},
		{StageCompilation, "compilation"},
		{StageRegistration, "registration"},
		{StageExecution, "execution"},
		{StageComplete, "complete"},
		{StageSimulation, "simulation"},
		{StagePanic, "panic"},
		{LoopStage(999), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.stage.String()
			if got != tt.want {
				t.Errorf("LoopStage(%d).String() = %q, want %q", tt.stage, got, tt.want)
			}
		})
	}
}

// --- ActionType.String ---

func TestActionType_String_AllTypes(t *testing.T) {
	tests := []struct {
		at   ActionType
		want string
	}{
		{ActionNone, "none"},
		{ActionStartCampaign, "start_campaign"},
		{ActionGenerateTool, "generate_tool"},
		{ActionCreateAgent, "create_agent"},
		{ActionDelegateToShard, "delegate_to_shard"},
		{ActionType(999), "none"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.at.String()
			if got != tt.want {
				t.Errorf("ActionType(%d).String() = %q, want %q", tt.at, got, tt.want)
			}
		})
	}
}

// --- Config defaults ---

func TestConfig_ZeroValue_ShouldHaveSensibleDefaults(t *testing.T) {
	cfg := Config{}
	if cfg.MinConfidence != 0 {
		t.Errorf("zero Config.MinConfidence = %v, want 0", cfg.MinConfidence)
	}
	if cfg.EnableToolGeneration {
		t.Error("zero Config.EnableToolGeneration = true, want false")
	}
}

// --- QuickResult ---

func TestQuickResult_ZeroValue_ShouldBeInert(t *testing.T) {
	qr := QuickResult{}
	if qr.NeedsCampaign || qr.NeedsPersistent || qr.NeedsTool {
		t.Error("zero QuickResult should not need anything")
	}
	if qr.TopAction != nil {
		t.Error("zero QuickResult.TopAction should be nil")
	}
}

// --- CampaignPayload ---

func TestCampaignPayload_ShouldStoreFields(t *testing.T) {
	cp := CampaignPayload{
		Phases:         []string{"plan", "implement", "verify"},
		EstimatedFiles: 42,
		Reasons:        []string{"complex"},
	}
	if len(cp.Phases) != 3 {
		t.Errorf("expected 3 phases, got %d", len(cp.Phases))
	}
	if cp.EstimatedFiles != 42 {
		t.Errorf("expected 42 files, got %d", cp.EstimatedFiles)
	}
}

// --- AgentMemory ---

func TestAgentMemory_ZeroValue_ShouldHaveEmptyFields(t *testing.T) {
	mem := AgentMemory{}
	if mem.AgentName != "" {
		t.Errorf("expected empty AgentName, got %q", mem.AgentName)
	}
	if len(mem.Learnings) != 0 {
		t.Errorf("expected empty Learnings, got %d", len(mem.Learnings))
	}
	if mem.Preferences != nil {
		t.Error("expected nil Preferences")
	}
}

// --- Learning ---

func TestLearning_ShouldStoreFields(t *testing.T) {
	l := Learning{
		ID:         "test-1",
		Type:       "preference",
		Content:    "use tabs",
		Source:     "user",
		Confidence: 0.9,
	}
	if l.ID != "test-1" {
		t.Errorf("expected ID test-1, got %s", l.ID)
	}
	if l.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %v", l.Confidence)
	}
}
