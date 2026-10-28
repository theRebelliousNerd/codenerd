// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains type definitions for the Northstar wizard.
package chat

import (
	"time"
)

// =============================================================================
// NORTHSTAR TYPES AND CONSTANTS
// =============================================================================

// NorthstarPhase represents the current wizard phase
type NorthstarPhase int

const (
	NorthstarWelcome NorthstarPhase = iota
	NorthstarDocIngestion
	NorthstarProblemStatement
	NorthstarVisionStatement
	NorthstarTargetUsers
	NorthstarCapabilities
	NorthstarRedTeaming
	NorthstarRequirements
	NorthstarConstraints
	NorthstarSummary
	NorthstarComplete
)

// UserPersona represents a target user archetype
type UserPersona struct {
	Name       string   `json:"name"`
	PainPoints []string `json:"pain_points"`
	Needs      []string `json:"needs"`
}

// Capability represents a future project capability
type Capability struct {
	// ID is the capability's stable identity in the canonical vision. The
	// wizard mints it when the capability is created (ensureIDs); position is
	// never identity.
	ID          string `json:"id,omitempty"`
	Description string `json:"description"`
	Timeline    string `json:"timeline"` // "now", "6mo", "1yr", "3yr", "moonshot"
	Priority    string `json:"priority"` // "critical", "high", "medium", "low"
}

// Risk represents an identified project risk
type Risk struct {
	// ID is the risk's stable identity in the canonical vision (see Capability.ID).
	ID          string `json:"id,omitempty"`
	Description string `json:"description"`
	Likelihood  string `json:"likelihood"` // "high", "medium", "low"
	Impact      string `json:"impact"`     // "high", "medium", "low"
	Mitigation  string `json:"mitigation"`
}

// NorthstarRequirement represents a crystallized requirement
type NorthstarRequirement struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // "functional", "non-functional", "constraint"
	Description string `json:"description"`
	Priority    string `json:"priority"` // "must-have", "should-have", "nice-to-have"
	Source      string `json:"source"`   // Origin: "user", "research", "red-team"
	// Supports and Addresses are the capability and risk IDs this requirement
	// realizes and mitigates. The wizard does not ask for them; derivation and
	// the heuristic generator fill them, and they travel to the canonical vision.
	Supports  []string `json:"supports,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
}

// =============================================================================
// BUBBLETEA MESSAGES
// =============================================================================

// requirementsGeneratedMsg is sent when LLM generates requirements
type requirementsGeneratedMsg struct {
	requirements []NorthstarRequirement
	err          error
}

// NOTE: northstarDocsAnalyzedMsg is defined in model.go

// =============================================================================
// WIZARD STATE
// =============================================================================

// NorthstarWizardState tracks the state of the northstar definition wizard
type NorthstarWizardState struct {
	Phase   NorthstarPhase
	SubStep int // For multi-part phases

	// Document ingestion
	ResearchDocs   []string
	ExtractedFacts []string

	// Vision
	Problem string
	Vision  string
	Mission string // One-liner

	// Target users
	CurrentPersona *UserPersona
	Personas       []UserPersona

	// Capabilities
	Capabilities      []Capability
	CurrentCapability *Capability

	// Red teaming
	Risks           []Risk
	CurrentRisk     *Risk
	RedTeamInsights []string

	// Requirements
	Requirements []NorthstarRequirement

	// Constraints
	Constraints []string

	// Processing state
	IsProcessing  bool
	ProcessingMsg string

	// Metadata
	CreatedAt   time.Time
	LastUpdated time.Time

	// edit is the canonical vision this wizard is an editable view of, and what
	// it was based on. Unexported on purpose: it is not part of the view and
	// must never be serialized as one.
	edit northstarEdit
}

// NewNorthstarWizard creates a new wizard state
func NewNorthstarWizard() *NorthstarWizardState {
	return &NorthstarWizardState{
		Phase:        NorthstarWelcome,
		CreatedAt:    time.Now(),
		LastUpdated:  time.Now(),
		Personas:     []UserPersona{},
		Capabilities: []Capability{},
		Risks:        []Risk{},
		Requirements: []NorthstarRequirement{},
		Constraints:  []string{},
	}
}
