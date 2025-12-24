// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains type definitions for the Northstar wizard.
package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	Description string `json:"description"`
	Timeline    string `json:"timeline"` // "now", "6mo", "1yr", "3yr", "moonshot"
	Priority    string `json:"priority"` // "critical", "high", "medium", "low"
}

// Risk represents an identified project risk
type Risk struct {
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

// parseGeneratedRequirements parses LLM response into requirements
func parseGeneratedRequirements(response string, startIdx int) []NorthstarRequirement {
	var requirements []NorthstarRequirement
	lines := strings.Split(response, "\n")
	idx := startIdx
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip markdown list markers
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		// Parse type from brackets [F], [NF], [C]
		reqType := "functional"
		if strings.HasPrefix(line, "[") {
			endBracket := strings.Index(line, "]")
			if endBracket > 0 {
				typeMarker := strings.ToLower(line[1:endBracket])
				line = strings.TrimSpace(line[endBracket+1:])
				switch typeMarker {
				case "f", "functional":
					reqType = "functional"
				case "nf", "non-functional", "quality":
					reqType = "non-functional"
				case "c", "constraint":
					reqType = "constraint"
				}
			}
		}
		// Parse priority if present
		priority := "should-have"
		if strings.Contains(strings.ToLower(line), "(must") {
			priority = "must-have"
		} else if strings.Contains(strings.ToLower(line), "(nice") {
			priority = "nice-to-have"
		}
		if line == "" {
			continue
		}
		idx++
		requirements = append(requirements, NorthstarRequirement{
			ID:          "REQ-" + string(rune('A'+idx-1)),
			Type:        reqType,
			Description: line,
			Priority:    priority,
			Source:      "auto-generated",
		})
		if idx-startIdx >= 15 { // Max 15 generated requirements
			break
		}
	}
	return requirements
}

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

// loadExistingNorthstar attempts to load an existing northstar definition from disk
func loadExistingNorthstar(workspace string) (*NorthstarWizardState, bool) {
	jsonPath := filepath.Join(workspace, ".nerd", "northstar.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, false
	}

	var wizard NorthstarWizardState
	if err := json.Unmarshal(data, &wizard); err != nil {
		return nil, false
	}

	// Ensure slices are initialized (JSON may decode nil slices)
	if wizard.Personas == nil {
		wizard.Personas = []UserPersona{}
	}
	if wizard.Capabilities == nil {
		wizard.Capabilities = []Capability{}
	}
	if wizard.Risks == nil {
		wizard.Risks = []Risk{}
	}
	if wizard.Requirements == nil {
		wizard.Requirements = []NorthstarRequirement{}
	}
	if wizard.Constraints == nil {
		wizard.Constraints = []string{}
	}
	if wizard.ResearchDocs == nil {
		wizard.ResearchDocs = []string{}
	}
	if wizard.ExtractedFacts == nil {
		wizard.ExtractedFacts = []string{}
	}

	return &wizard, true
}
