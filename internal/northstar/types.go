// Package northstar implements the permanent Northstar Guardian system agent.
// Northstar is the project vision guardian - it holds the vision definition,
// monitors all activity for alignment, and ensures work stays on track.
//
// Unlike user-defined specialists in .nerd/agents/, Northstar is a core system
// component with its prompt atoms in internal/prompt/atoms/northstar/ and its
// project-specific knowledge in .nerd/northstar_knowledge.db.
package northstar

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
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

// PersonaFactID returns the Mangle ID used for a persona in northstar_persona
// and every predicate that references a persona. Link fields accept either the
// human-readable persona name or this ID, so a hand-edited northstar.json does
// not have to know the encoding.
func PersonaFactID(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "persona_") {
		return name
	}
	return "persona_" + name
}

// ToFacts converts the Vision into a slice of Mangle facts.
func (v *Vision) ToFacts() []types.Fact {
	var facts []types.Fact

	if v.Mission != "" {
		facts = append(facts, types.Fact{Predicate: "northstar_mission", Args: []any{"global", v.Mission}})
	}
	if v.Problem != "" {
		facts = append(facts, types.Fact{Predicate: "northstar_problem", Args: []any{"global", v.Problem}})
	}
	if v.VisionStmt != "" {
		facts = append(facts, types.Fact{Predicate: "northstar_vision", Args: []any{"global", v.VisionStmt}})
	}

	// Referential integrity sets. Link facts (serves/supports/addresses) are
	// only emitted for targets that actually exist in this vision: a dangling
	// northstar_serves("cap_1", "persona_ghost") would make unserved_persona
	// and orphan_capability silently wrong, which is worse than no link at all.
	personaIDs := make(map[string]struct{}, len(v.Personas))
	capIDs := make(map[string]struct{}, len(v.Capabilities))
	riskIDs := make(map[string]struct{}, len(v.Risks))

	for _, p := range v.Personas {
		id := PersonaFactID(p.Name)
		personaIDs[id] = struct{}{}
		facts = append(facts, types.Fact{Predicate: "northstar_persona", Args: []any{id, p.Name}})
		for _, pp := range p.PainPoints {
			facts = append(facts, types.Fact{Predicate: "northstar_pain_point", Args: []any{id, pp}})
		}
		for _, need := range p.Needs {
			facts = append(facts, types.Fact{Predicate: "northstar_need", Args: []any{id, need}})
		}
	}

	for _, c := range v.Capabilities {
		capIDs[c.ID] = struct{}{}
		facts = append(facts, types.Fact{Predicate: "northstar_capability", Args: []any{c.ID, c.Description, "/" + c.Timeline, parsePriority(c.Priority)}})
	}

	// northstar_serves(CapID, PersonaID). Declared in schemas_misc.mg and read by
	// unserved_persona / orphan_capability / capability_addresses_need, none of
	// which could ever fire while nothing emitted this predicate.
	for _, c := range v.Capabilities {
		for _, target := range c.Serves {
			pid := PersonaFactID(target)
			if _, ok := personaIDs[pid]; !ok {
				continue
			}
			facts = append(facts, types.Fact{Predicate: "northstar_serves", Args: []any{c.ID, pid}})
		}
	}

	for _, r := range v.Risks {
		riskIDs[r.ID] = struct{}{}
		facts = append(facts, types.Fact{Predicate: "northstar_risk", Args: []any{r.ID, r.Description, "/" + r.Likelihood, parseRiskImpact(r.Impact)}})
		if r.Mitigation != "" {
			// The strategy slot is Decl'd /name, so the free text cannot go in
			// it directly. Emitting the same constant /mitigation for every risk
			// made has_mitigation/1 work but made every mitigation
			// indistinguishable: two risks with opposite strategies unified.
			// Emit a text-derived name here and the free text alongside it.
			facts = append(facts, types.Fact{Predicate: "northstar_mitigation", Args: []any{r.ID, MitigationStrategyAtom(r.Mitigation)}})
			facts = append(facts, types.Fact{Predicate: "northstar_mitigation_text", Args: []any{r.ID, types.MangleString(r.Mitigation)}})
		}
	}

	for _, req := range v.Requirements {
		facts = append(facts, types.Fact{Predicate: "northstar_requirement", Args: []any{req.ID, "/" + req.Type, req.Description, parsePriority(req.Priority)}})
	}

	// northstar_supports(ReqID, CapID) / northstar_addresses(ReqID, RiskID).
	// orphan_requirement, risk_addressing_requirement, unaddressed_high_risk and
	// strategic_warning(/critical_unmitigated_risk, ...) all depend on these.
	for _, req := range v.Requirements {
		for _, capID := range req.Supports {
			if _, ok := capIDs[strings.TrimSpace(capID)]; !ok {
				continue
			}
			facts = append(facts, types.Fact{Predicate: "northstar_supports", Args: []any{req.ID, strings.TrimSpace(capID)}})
		}
		for _, riskID := range req.Addresses {
			if _, ok := riskIDs[strings.TrimSpace(riskID)]; !ok {
				continue
			}
			facts = append(facts, types.Fact{Predicate: "northstar_addresses", Args: []any{req.ID, strings.TrimSpace(riskID)}})
		}
	}

	for i, c := range v.Constraints {
		facts = append(facts, types.Fact{Predicate: "northstar_constraint", Args: []any{fmt.Sprintf("constraint_%d", i), c}})
	}

	facts = append(facts, types.Fact{Predicate: "northstar_defined", Args: []any{}})

	return facts
}

// MitigationStrategyAtom encodes mitigation free text as a stable Mangle name
// constant of the form /mit_<slug>_<hash8>.
//
// The slug keeps the atom readable in `nerd query` output; the hash suffix keeps
// it injective, so two mitigations that slug identically (or slug to nothing,
// e.g. non-latin text) still produce distinct atoms. Mangle name constants
// reject whitespace, more than two slashes and anything ending in a known file
// extension, so the slug is restricted to [a-z0-9_].
func MitigationStrategyAtom(text string) types.MangleAtom {
	sum := sha256.Sum256([]byte(text))
	digest := hex.EncodeToString(sum[:])[:8]

	var sb strings.Builder
	lastUnderscore := true // suppress a leading underscore
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore && sb.Len() < mitigationSlugMax {
				sb.WriteByte('_')
				lastUnderscore = true
			}
		}
		if sb.Len() >= mitigationSlugMax {
			break
		}
	}
	slug := strings.Trim(sb.String(), "_")
	if slug == "" {
		return types.MangleAtom("/mit_" + digest)
	}
	return types.MangleAtom("/mit_" + slug + "_" + digest)
}

const mitigationSlugMax = 40

func parsePriority(p string) int {
	switch p {
	case "critical", "must_have":
		return 100
	case "high", "should_have":
		return 80
	case "medium":
		return 50
	case "low", "nice_to_have":
		return 20
	default:
		return 50
	}
}

func parseRiskImpact(i string) int {
	switch i {
	case "high":
		return 100
	case "medium":
		return 50
	case "low":
		return 20
	default:
		return 50
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
	// Serves lists the personas this capability exists for, by persona name or
	// by persona_<Name> ID. Projected as northstar_serves(CapID, PersonaID).
	Serves []string `json:"serves,omitempty"`
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
	// Supports lists capability IDs this requirement realizes.
	// Projected as northstar_supports(ReqID, CapID).
	Supports []string `json:"supports,omitempty"`
	// Addresses lists risk IDs this requirement mitigates.
	// Projected as northstar_addresses(ReqID, RiskID).
	Addresses []string `json:"addresses,omitempty"`
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
