// Package northstar implements the permanent Northstar Guardian system agent.
// Northstar is the project vision guardian - it holds the vision definition,
// monitors all activity for alignment, and ensures work stays on track.
//
// Unlike user-defined specialists in .nerd/agents/, Northstar is a core system
// component with its prompt atoms in internal/prompt/atoms/northstar/ and its
// project-specific knowledge in .nerd/northstar_knowledge.db.
package northstar

import (
	"fmt"
	"time"

	"codenerd/internal/types"
)

// =============================================================================
// VISION DEFINITION TYPES
// =============================================================================

// Vision represents the complete project vision definition.
type Vision struct {
	Mission      string        `json:"mission"`
	Problem      string        `json:"problem"`
	VisionStmt   string        `json:"vision_statement"`
	Personas     []Persona     `json:"personas"`
	Capabilities []Capability  `json:"capabilities"`
	Risks        []Risk        `json:"risks"`
	Requirements []Requirement `json:"requirements"`
	Constraints  []string      `json:"constraints"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// ToFacts converts the Vision into a slice of Mangle facts.
func (v *Vision) ToFacts() []types.Fact {
	var facts []types.Fact

	if v.Mission != "" {
		facts = append(facts, types.Fact{Predicate: "northstar_mission", Args: []interface{}{"global", v.Mission}})
	}
	if v.Problem != "" {
		facts = append(facts, types.Fact{Predicate: "northstar_problem", Args: []interface{}{"global", v.Problem}})
	}
	if v.VisionStmt != "" {
		facts = append(facts, types.Fact{Predicate: "northstar_vision", Args: []interface{}{"global", v.VisionStmt}})
	}

	for _, p := range v.Personas {
		id := fmt.Sprintf("persona_%s", p.Name)
		facts = append(facts, types.Fact{Predicate: "northstar_persona", Args: []interface{}{id, p.Name}})
		for _, pp := range p.PainPoints {
			facts = append(facts, types.Fact{Predicate: "northstar_pain_point", Args: []interface{}{id, pp}})
		}
		for _, need := range p.Needs {
			facts = append(facts, types.Fact{Predicate: "northstar_need", Args: []interface{}{id, need}})
		}
	}

	for _, c := range v.Capabilities {
		facts = append(facts, types.Fact{Predicate: "northstar_capability", Args: []interface{}{c.ID, c.Description, "/" + c.Timeline, parsePriority(c.Priority)}})
	}

	for _, r := range v.Risks {
		facts = append(facts, types.Fact{Predicate: "northstar_risk", Args: []interface{}{r.ID, r.Description, "/" + r.Likelihood, parseRiskImpact(r.Impact)}})
		if r.Mitigation != "" {
			// Convert strategy to a valid Mangle /name atom
			strategyName := fmt.Sprintf("/%s", "mitigation") 
			facts = append(facts, types.Fact{Predicate: "northstar_mitigation", Args: []interface{}{r.ID, strategyName}})
		}
	}

	for _, req := range v.Requirements {
		facts = append(facts, types.Fact{Predicate: "northstar_requirement", Args: []interface{}{req.ID, "/" + req.Type, req.Description, parsePriority(req.Priority)}})
	}

	for i, c := range v.Constraints {
		facts = append(facts, types.Fact{Predicate: "northstar_constraint", Args: []interface{}{fmt.Sprintf("constraint_%d", i), c}})
	}

	facts = append(facts, types.Fact{Predicate: "northstar_defined", Args: []interface{}{}})

	return facts
}

func parsePriority(p string) int {
	switch p {
	case "critical", "must_have": return 100
	case "high", "should_have": return 80
	case "medium": return 50
	case "low", "nice_to_have": return 20
	default: return 50
	}
}

func parseRiskImpact(i string) int {
	switch i {
	case "high": return 100
	case "medium": return 50
	case "low": return 20
	default: return 50
	}
}

// Persona represents a user persona with their pain points and needs.
type Persona struct {
	Name       string   `json:"name"`
	PainPoints []string `json:"pain_points"`
	Needs      []string `json:"needs"`
}

// Capability represents a planned capability with timeline and priority.
type Capability struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Timeline    string `json:"timeline"` // "now", "next", "later"
	Priority    string `json:"priority"` // "critical", "high", "medium", "low"
}

// Risk represents an identified risk with likelihood, impact, and mitigation.
type Risk struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Likelihood  string `json:"likelihood"` // "high", "medium", "low"
	Impact      string `json:"impact"`     // "high", "medium", "low"
	Mitigation  string `json:"mitigation"`
}

// Requirement represents a formal requirement.
type Requirement struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // "functional", "non_functional", "constraint"
	Description string `json:"description"`
	Priority    string `json:"priority"` // "must_have", "should_have", "nice_to_have"
}

// =============================================================================
// OBSERVATION TYPES
// =============================================================================

// Observation represents something Northstar noticed during a session.
type Observation struct {
	ID        string            `json:"id"`
	SessionID string            `json:"session_id"`
	Timestamp time.Time         `json:"timestamp"`
	Type      ObservationType   `json:"type"`
	Subject   string            `json:"subject"`   // What was observed (file, task, decision)
	Content   string            `json:"content"`   // The observation itself
	Relevance float64           `json:"relevance"` // 0.0-1.0 relevance to vision
	Tags      []string          `json:"tags"`
	Metadata  map[string]string `json:"metadata"`
}

// ObservationType categorizes observations.
type ObservationType string

const (
	ObsTaskCompleted    ObservationType = "task_completed"
	ObsFileChanged      ObservationType = "file_changed"
	ObsDecisionMade     ObservationType = "decision_made"
	ObsPatternDetected  ObservationType = "pattern_detected"
	ObsDriftWarning     ObservationType = "drift_warning"
	ObsAlignmentSuccess ObservationType = "alignment_success"
	ObsRiskTriggered    ObservationType = "risk_triggered"
)

// =============================================================================
// ALIGNMENT CHECK TYPES
// =============================================================================

// AlignmentCheck represents a formal alignment validation.
type AlignmentCheck struct {
	ID          string           `json:"id"`
	Timestamp   time.Time        `json:"timestamp"`
	Trigger     AlignmentTrigger `json:"trigger"`
	Subject     string           `json:"subject"` // What was checked
	Context     string           `json:"context"` // Additional context
	Result      AlignmentResult  `json:"result"`
	Score       float64          `json:"score"`       // 0.0-1.0 alignment score
	Explanation string           `json:"explanation"` // LLM explanation
	Suggestions []string         `json:"suggestions"` // Improvement suggestions
	Duration    time.Duration    `json:"duration"`
}

// AlignmentTrigger indicates what triggered the alignment check.
type AlignmentTrigger string

const (
	TriggerManual        AlignmentTrigger = "manual"         // User ran /alignment
	TriggerPhaseGate     AlignmentTrigger = "phase_gate"     // Campaign phase transition
	TriggerPeriodic      AlignmentTrigger = "periodic"       // Every N tasks
	TriggerHighImpact    AlignmentTrigger = "high_impact"    // High-impact change detected
	TriggerTaskComplete  AlignmentTrigger = "task_complete"  // After significant task
	TriggerSessionStart  AlignmentTrigger = "session_start"  // New session started
	TriggerCampaignStart AlignmentTrigger = "campaign_start" // New campaign started
)

// AlignmentResult indicates the outcome of an alignment check.
type AlignmentResult string

const (
	AlignmentPassed  AlignmentResult = "passed"  // Fully aligned
	AlignmentWarning AlignmentResult = "warning" // Minor drift detected
	AlignmentFailed  AlignmentResult = "failed"  // Significant drift
	AlignmentBlocked AlignmentResult = "blocked" // Cannot proceed without fix
	AlignmentSkipped AlignmentResult = "skipped" // Check skipped (no vision defined)
)

// =============================================================================
// DRIFT DETECTION TYPES
// =============================================================================

// DriftEvent represents detected drift from the vision.
type DriftEvent struct {
	ID           string        `json:"id"`
	Timestamp    time.Time     `json:"timestamp"`
	Severity     DriftSeverity `json:"severity"`
	Category     string        `json:"category"` // Which aspect drifted
	Description  string        `json:"description"`
	Evidence     []string      `json:"evidence"`      // What indicates drift
	RelatedCheck string        `json:"related_check"` // AlignmentCheck ID
	Resolved     bool          `json:"resolved"`
	ResolvedAt   *time.Time    `json:"resolved_at,omitempty"`
	Resolution   string        `json:"resolution,omitempty"`
}

// DriftSeverity indicates how severe the drift is.
type DriftSeverity string

const (
	DriftMinor    DriftSeverity = "minor"    // Cosmetic, can continue
	DriftModerate DriftSeverity = "moderate" // Should address soon
	DriftMajor    DriftSeverity = "major"    // Needs immediate attention
	DriftCritical DriftSeverity = "critical" // Blocks further work
)

// =============================================================================
// GUARDIAN CONFIGURATION
// =============================================================================

// GuardianConfig configures Northstar Guardian behavior.
type GuardianConfig struct {
	// Periodic check interval (in tasks)
	PeriodicCheckInterval int `json:"periodic_check_interval"` // Default: 5 tasks

	// Enable automatic checks
	EnablePhaseGates    bool `json:"enable_phase_gates"`    // Check at phase transitions
	EnablePeriodicCheck bool `json:"enable_periodic_check"` // Check every N tasks
	EnableHighImpact    bool `json:"enable_high_impact"`    // Check high-impact changes

	// High-impact paths (changes here trigger checks)
	HighImpactPaths []string `json:"high_impact_paths"`

	// Severity thresholds
	WarningThreshold float64 `json:"warning_threshold"` // Below this = warning (default: 0.7)
	FailureThreshold float64 `json:"failure_threshold"` // Below this = failed (default: 0.5)
	BlockThreshold   float64 `json:"block_threshold"`   // Below this = blocked (default: 0.3)

	// Model for alignment checks
	AlignmentModel string `json:"alignment_model"` // LLM model for checks
}

// DefaultGuardianConfig returns sensible defaults.
func DefaultGuardianConfig() GuardianConfig {
	return GuardianConfig{
		PeriodicCheckInterval: 5,
		EnablePhaseGates:      true,
		EnablePeriodicCheck:   true,
		EnableHighImpact:      true,
		HighImpactPaths: []string{
			"internal/core/",
			"internal/session/",
			"internal/perception/",
			"cmd/nerd/",
			"*.mg",
		},
		WarningThreshold: 0.7,
		FailureThreshold: 0.5,
		BlockThreshold:   0.3,
		AlignmentModel:   "", // Use default
	}
}

// =============================================================================
// GUARDIAN STATE
// =============================================================================

// GuardianState tracks the current state of the Northstar Guardian.
type GuardianState struct {
	VisionDefined       bool      `json:"vision_defined"`
	LastCheck           time.Time `json:"last_check"`
	TasksSinceCheck     int       `json:"tasks_since_check"`
	ActiveDriftCount    int       `json:"active_drift_count"`
	OverallAlignment    float64   `json:"overall_alignment"` // Running average
	SessionObservations int       `json:"session_observations"`
}
