// Package chat provides the interactive TUI chat interface for codeNERD.
// This file implements the /northstar command - an interactive wizard for defining
// the project's grand vision, north star, and specification.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/prompt"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// NORTHSTAR WIZARD - Project Vision & Specification
// =============================================================================
// The Northstar Wizard guides users through a structured process to define:
// 1. Project vision and mission
// 2. Target users and their needs
// 3. Future capabilities (roadmap)
// 4. Risks and mitigations (red teaming)
// 5. Concrete requirements
//
// Results are stored in:
// - .nerd/northstar.mg (Mangle facts for kernel reasoning)
// - knowledgebase.db (persistent storage for retrieval)

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
// BUBBLETEA MESSAGES - Async operation results
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
		// Parse lines like "- [MUST] User can login"
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			line = strings.TrimPrefix(line, "-")
			line = strings.TrimPrefix(line, "*")
			line = strings.TrimSpace(line)

			req := NorthstarRequirement{
				ID:          fmt.Sprintf("REQ-%03d", idx+1),
				Type:        "functional",
				Description: line,
				Priority:    "should-have",
				Source:      "llm",
			}

			// Parse priority tags
			if strings.Contains(strings.ToUpper(line), "[MUST]") {
				req.Priority = "must-have"
				req.Description = strings.Replace(req.Description, "[MUST]", "", 1)
				req.Description = strings.Replace(req.Description, "[must]", "", 1)
			} else if strings.Contains(strings.ToUpper(line), "[SHOULD]") {
				req.Priority = "should-have"
				req.Description = strings.Replace(req.Description, "[SHOULD]", "", 1)
				req.Description = strings.Replace(req.Description, "[should]", "", 1)
			} else if strings.Contains(strings.ToUpper(line), "[NICE]") {
				req.Priority = "nice-to-have"
				req.Description = strings.Replace(req.Description, "[NICE]", "", 1)
				req.Description = strings.Replace(req.Description, "[nice]", "", 1)
			}
			req.Description = strings.TrimSpace(req.Description)

			if req.Description != "" {
				requirements = append(requirements, req)
				idx++
			}
		}
	}
	return requirements
}

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

// handleNorthstarWizardInput processes user input during the northstar wizard
func (m Model) handleNorthstarWizardInput(input string) (tea.Model, tea.Cmd) {
	if m.northstarWizard == nil {
		m.northstarWizard = NewNorthstarWizard()
	}

	w := m.northstarWizard
	w.LastUpdated = time.Now()
	input = strings.TrimSpace(input)

	// Handle skip/back commands
	lower := strings.ToLower(input)
	if lower == "skip" || lower == "/skip" {
		return m.advanceNorthstarPhase()
	}
	if lower == "back" || lower == "/back" {
		return m.previousNorthstarPhase()
	}
	if lower == "quit" || lower == "/quit" || lower == "exit" {
		m.awaitingNorthstar = false
		m.northstarWizard = nil
		m.textarea.Placeholder = "Ask me anything... (Enter to send, Alt+Enter for newline, Ctrl+C to exit)"
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Northstar wizard cancelled. Your progress was not saved.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}

	switch w.Phase {
	case NorthstarWelcome:
		return m.handleNorthstarWelcome(input)
	case NorthstarDocIngestion:
		return m.handleNorthstarDocIngestion(input)
	case NorthstarProblemStatement:
		return m.handleNorthstarProblem(input)
	case NorthstarVisionStatement:
		return m.handleNorthstarVision(input)
	case NorthstarTargetUsers:
		return m.handleNorthstarUsers(input)
	case NorthstarCapabilities:
		return m.handleNorthstarCapabilities(input)
	case NorthstarRedTeaming:
		return m.handleNorthstarRedTeam(input)
	case NorthstarRequirements:
		return m.handleNorthstarRequirements(input)
	case NorthstarConstraints:
		return m.handleNorthstarConstraints(input)
	case NorthstarSummary:
		return m.handleNorthstarSummary(input)
	}

	return m, nil
}

// =============================================================================
// PHASE HANDLERS
// =============================================================================

func (m Model) handleNorthstarWelcome(input string) (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	lower := strings.ToLower(input)

	if lower == "yes" || lower == "y" || lower == "1" {
		w.Phase = NorthstarDocIngestion
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: `## Phase 1: Research Document Ingestion

I can analyze existing research documents, specs, or design docs to extract key insights.

**Enter file paths** (one per line, or comma-separated), or type **"done"** to skip:

Examples:
- ` + "`Docs/research/spec.md`" + `
- ` + "`./requirements.txt`" + `
- ` + "`~/notes/project-vision.md`" + `

_Type "done" if you have no documents to ingest._`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Enter document paths or 'done'..."
	} else {
		w.Phase = NorthstarProblemStatement
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: `## Phase 2: Problem Statement

Let's start by defining the problem you're solving.

**What problem does this project solve?**

Be specific. Think about:
- What pain exists today?
- What's broken, slow, or missing?
- Why does this problem matter?

_Write freely - we'll refine it together._`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Describe the problem..."
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) handleNorthstarDocIngestion(input string) (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	lower := strings.ToLower(input)

	if lower == "done" || lower == "" {
		// If we have documents, analyze them with LLM
		if len(w.ResearchDocs) > 0 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Analyzing **%d document(s)**... This may take a moment.", len(w.ResearchDocs)),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true

			// Start async document analysis
			return m, tea.Batch(m.spinner.Tick, m.analyzeNorthstarDocs(w.ResearchDocs))
		}

		// No documents - move directly to next phase
		w.Phase = NorthstarProblemStatement
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: `## Phase 2: Problem Statement

**What problem does this project solve?**

Think about:
- What pain exists today?
- What's broken, slow, or missing?
- Why does this problem matter?

_Write freely - we'll refine it together._`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Describe the problem..."
	} else {
		// Parse file paths
		paths := parseFilePaths(input)
		validPaths := []string{}
		invalidPaths := []string{}

		for _, p := range paths {
			expandedPath := expandPath(m.workspace, p)
			if _, err := os.Stat(expandedPath); err == nil {
				validPaths = append(validPaths, expandedPath)
			} else {
				invalidPaths = append(invalidPaths, p)
			}
		}

		w.ResearchDocs = append(w.ResearchDocs, validPaths...)

		var msg strings.Builder
		if len(validPaths) > 0 {
			msg.WriteString(fmt.Sprintf("Added **%d** document(s):\n", len(validPaths)))
			for _, p := range validPaths {
				msg.WriteString(fmt.Sprintf("- `%s`\n", filepath.Base(p)))
			}
		}
		if len(invalidPaths) > 0 {
			msg.WriteString(fmt.Sprintf("\n⚠️ Could not find **%d** path(s):\n", len(invalidPaths)))
			for _, p := range invalidPaths {
				msg.WriteString(fmt.Sprintf("- `%s`\n", p))
			}
		}
		msg.WriteString(fmt.Sprintf("\n**Total queued:** %d\n\n_Add more paths or type \"done\" to continue._", len(w.ResearchDocs)))

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: msg.String(),
			Time:    time.Now(),
		})
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) handleNorthstarProblem(input string) (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	// Validate: require meaningful problem statement (at least 10 chars)
	if len(strings.TrimSpace(input)) < 10 {
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Please provide a more detailed problem statement (at least a sentence describing the pain point).",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}

	w.Problem = input
	w.Phase = NorthstarVisionStatement

	m.history = append(m.history, Message{
		Role: "assistant",
		Content: fmt.Sprintf(`## Problem Captured

> %s

---

## Phase 3: Vision Statement

Now let's articulate the **grand vision**.

**If this project succeeds wildly, what does the world look like?**

Think big:
- What's the ideal end state?
- How do people's lives improve?
- What becomes possible that wasn't before?

_Paint a picture of success._`, truncateWithEllipsis(input, 200)),
		Time: time.Now(),
	})
	m.textarea.Placeholder = "Describe your vision..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) handleNorthstarVision(input string) (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	if w.SubStep == 0 {
		// Validate: require meaningful vision statement (at least 20 chars)
		if len(strings.TrimSpace(input)) < 20 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Please describe a more detailed vision - what does success look like? (at least a couple sentences)",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		w.Vision = input
		w.SubStep = 1
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: `## Vision Captured

Now distill it into a **mission statement** - one sentence that captures the essence.

**Complete this sentence:**
_"We exist to..."_ or _"Our mission is to..."_

Examples:
- "Make coding accessible to everyone"
- "Eliminate build failures before they happen"
- "Give developers superpowers through AI"`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "One sentence mission..."
	} else {
		// Validate: require meaningful mission statement (at least 10 chars)
		if len(strings.TrimSpace(input)) < 10 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Please provide a mission statement - a single sentence that captures the essence of your project.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		w.Mission = input
		w.SubStep = 0
		w.Phase = NorthstarTargetUsers
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`## Mission Locked

> **%s**

---

## Phase 4: Target Users

Who are you building this for? Let's define **user personas**.

**Describe your primary user:**
- Who are they? (role, background)
- What's their biggest pain point?
- What do they need most?

Example: _"Senior backend developers who waste hours debugging flaky tests"_

_We'll add multiple personas. Type "done" when finished._`, input),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Describe a target user..."
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) handleNorthstarUsers(input string) (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	lower := strings.ToLower(input)

	if lower == "done" {
		if len(w.Personas) == 0 {
			// Add a default persona from the input if we have one
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Please add at least one user persona before continuing.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}

		w.Phase = NorthstarCapabilities
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`## %d Persona(s) Defined

---

## Phase 5: Future Capabilities

What will this project be able to do? Let's build a **capability roadmap**.

For each capability, I'll ask about timeline and priority.

**What's one key capability this project will have?**

Think about:
- Core features (must work day 1)
- Near-term wins (6 months)
- Long-term vision (1-3 years)
- Moonshots (dream features)

_Type "done" when finished adding capabilities._`, len(w.Personas)),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Describe a capability..."
	} else if w.CurrentPersona == nil {
		// Start a new persona
		w.CurrentPersona = &UserPersona{Name: input}
		w.SubStep = 1
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`**Persona: %s**

What are their **top pain points**? (comma-separated or one per line)`, input),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Pain points..."
	} else if w.SubStep == 1 {
		// Pain points
		w.CurrentPersona.PainPoints = splitAndTrim(input)
		w.SubStep = 2
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "What do they **need most** from this solution? (comma-separated)",
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Key needs..."
	} else {
		// Needs - complete persona
		w.CurrentPersona.Needs = splitAndTrim(input)
		w.Personas = append(w.Personas, *w.CurrentPersona)
		w.CurrentPersona = nil
		w.SubStep = 0

		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`✓ **Persona added:** %s

_Add another persona or type "done" to continue._`, w.Personas[len(w.Personas)-1].Name),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Add another persona or 'done'..."
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) handleNorthstarCapabilities(input string) (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	lower := strings.ToLower(input)

	if lower == "done" {
		w.Phase = NorthstarRedTeaming
		w.SubStep = 0
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`## %d Capability(ies) Mapped

---

## Phase 6: Red Teaming 🔴

Time to play **devil's advocate**. Let's stress-test this vision.

I'll ask hard questions. Answer honestly - this makes the vision stronger.

**What could make this project FAIL?**

Think about:
- Technical risks
- Market risks
- Resource constraints
- Competition
- User adoption challenges

_Describe a risk, or type "done" when finished._`, len(w.Capabilities)),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "What could go wrong?"
	} else if w.CurrentCapability == nil {
		w.CurrentCapability = &Capability{Description: input}
		w.SubStep = 1
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: `**Timeline?**

1. Now (core/MVP)
2. 6 months
3. 1 year
4. 3+ years
5. Moonshot (dream)

_Enter number or description:_`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Timeline (1-5)..."
	} else if w.SubStep == 1 {
		w.CurrentCapability.Timeline = parseTimeline(input)
		w.SubStep = 2
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: `**Priority?**

1. Critical (must have)
2. High (should have)
3. Medium (nice to have)
4. Low (someday)`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Priority (1-4)..."
	} else {
		w.CurrentCapability.Priority = parsePriority(input)
		w.Capabilities = append(w.Capabilities, *w.CurrentCapability)
		w.CurrentCapability = nil
		w.SubStep = 0

		cap := w.Capabilities[len(w.Capabilities)-1]
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`✓ **Capability added:** %s
   Timeline: %s | Priority: %s

_Add another or type "done"._`, truncateWithEllipsis(cap.Description, 60), cap.Timeline, cap.Priority),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Add capability or 'done'..."
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) handleNorthstarRedTeam(input string) (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	lower := strings.ToLower(input)

	if lower == "done" {
		w.Phase = NorthstarRequirements
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`## %d Risk(s) Identified

Excellent work thinking critically!

---

## Phase 7: Requirements Crystallization

Let's turn everything into **concrete requirements**.

Based on our discussion, what are the **must-have requirements**?

Format: Just describe the requirement. I'll ask about type and priority.

_Type "done" when finished, or "auto" to let me suggest requirements._`, len(w.Risks)),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Describe a requirement..."
	} else if w.CurrentRisk == nil {
		w.CurrentRisk = &Risk{Description: input}
		w.SubStep = 1
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: `**How likely is this risk?** (high/medium/low)`,
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Likelihood..."
	} else if w.SubStep == 1 {
		w.CurrentRisk.Likelihood = parseLikelihood(input)
		w.SubStep = 2
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: `**If it happens, what's the impact?** (high/medium/low)`,
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Impact..."
	} else if w.SubStep == 2 {
		w.CurrentRisk.Impact = parseLikelihood(input) // Same parser works
		w.SubStep = 3
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: `**How could we mitigate this?** (or "none" if unknown)`,
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Mitigation strategy..."
	} else {
		w.CurrentRisk.Mitigation = input
		w.Risks = append(w.Risks, *w.CurrentRisk)
		w.CurrentRisk = nil
		w.SubStep = 0

		risk := w.Risks[len(w.Risks)-1]
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`✓ **Risk logged:** %s
   Likelihood: %s | Impact: %s

_Add another risk or type "done"._`, truncateWithEllipsis(risk.Description, 50), risk.Likelihood, risk.Impact),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Add risk or 'done'..."
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) handleNorthstarRequirements(input string) (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	lower := strings.ToLower(input)

	if lower == "done" {
		w.Phase = NorthstarConstraints
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`## %d Requirement(s) Captured

---

## Phase 8: Constraints

What are the **hard constraints** on this project?

Examples:
- "Must run on Windows and Linux"
- "Cannot use GPL-licensed code"
- "Must support offline mode"
- "Budget: $X / Timeline: Y months"
- "Team size: N engineers"

_Enter constraints (one per line) or type "done"._`, len(w.Requirements)),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Enter constraints or 'done'..."
	} else if lower == "auto" {
		// Auto-generate requirements from capabilities and risks
		return m.autoGenerateRequirements()
	} else if w.SubStep == 0 {
		// New requirement
		reqID := fmt.Sprintf("REQ-%03d", len(w.Requirements)+1)
		w.Requirements = append(w.Requirements, NorthstarRequirement{
			ID:          reqID,
			Description: input,
			Type:        "functional",
			Priority:    "must-have",
			Source:      "user",
		})
		w.SubStep = 1
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: `**Type?**
1. Functional (what it does)
2. Non-functional (how it does it: performance, security, etc.)
3. Constraint (limitation)`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Type (1-3)..."
	} else if w.SubStep == 1 {
		req := &w.Requirements[len(w.Requirements)-1]
		req.Type = parseReqType(input)
		w.SubStep = 2
		m.history = append(m.history, Message{
			Role: "assistant",
			Content: `**Priority?**
1. Must-have (critical for launch)
2. Should-have (important)
3. Nice-to-have (if time permits)`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Priority (1-3)..."
	} else {
		req := &w.Requirements[len(w.Requirements)-1]
		req.Priority = parseReqPriority(input)
		w.SubStep = 0

		m.history = append(m.history, Message{
			Role: "assistant",
			Content: fmt.Sprintf(`✓ **%s:** %s
   Type: %s | Priority: %s

_Add another or type "done"._`, req.ID, truncateWithEllipsis(req.Description, 50), req.Type, req.Priority),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Add requirement or 'done'..."
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) handleNorthstarConstraints(input string) (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	lower := strings.ToLower(input)

	if lower == "done" || lower == "" {
		w.Phase = NorthstarSummary
		return m.showNorthstarSummary()
	}

	// Parse constraints (can be multiple lines or comma-separated)
	constraints := splitAndTrim(input)
	w.Constraints = append(w.Constraints, constraints...)

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: fmt.Sprintf("✓ Added %d constraint(s). Total: %d\n\n_Add more or type \"done\"._", len(constraints), len(w.Constraints)),
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Add constraints or 'done'..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) showNorthstarSummary() (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	var sb strings.Builder
	sb.WriteString("# 🌟 NORTHSTAR SUMMARY\n\n")

	// Mission
	sb.WriteString("## Mission\n")
	sb.WriteString(fmt.Sprintf("> %s\n\n", w.Mission))

	// Problem
	sb.WriteString("## Problem\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", w.Problem))

	// Vision
	sb.WriteString("## Vision\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", w.Vision))

	// Personas
	if len(w.Personas) > 0 {
		sb.WriteString("## Target Users\n")
		for _, p := range w.Personas {
			sb.WriteString(fmt.Sprintf("### %s\n", p.Name))
			sb.WriteString("**Pain Points:** ")
			sb.WriteString(strings.Join(p.PainPoints, ", "))
			sb.WriteString("\n**Needs:** ")
			sb.WriteString(strings.Join(p.Needs, ", "))
			sb.WriteString("\n\n")
		}
	}

	// Capabilities
	if len(w.Capabilities) > 0 {
		sb.WriteString("## Capabilities Roadmap\n")
		sb.WriteString("| Capability | Timeline | Priority |\n")
		sb.WriteString("|------------|----------|----------|\n")
		for _, c := range w.Capabilities {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
				truncateWithEllipsis(c.Description, 40), c.Timeline, c.Priority))
		}
		sb.WriteString("\n")
	}

	// Risks
	if len(w.Risks) > 0 {
		sb.WriteString("## Identified Risks\n")
		for _, r := range w.Risks {
			sb.WriteString(fmt.Sprintf("- **%s** (L:%s/I:%s)\n", r.Description, r.Likelihood, r.Impact))
			if r.Mitigation != "" && r.Mitigation != "none" {
				sb.WriteString(fmt.Sprintf("  - Mitigation: %s\n", r.Mitigation))
			}
		}
		sb.WriteString("\n")
	}

	// Requirements
	if len(w.Requirements) > 0 {
		sb.WriteString("## Requirements\n")
		sb.WriteString("| ID | Description | Type | Priority |\n")
		sb.WriteString("|----|-------------|------|----------|\n")
		for _, r := range w.Requirements {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				r.ID, truncateWithEllipsis(r.Description, 35), r.Type, r.Priority))
		}
		sb.WriteString("\n")
	}

	// Constraints
	if len(w.Constraints) > 0 {
		sb.WriteString("## Constraints\n")
		for _, c := range w.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")
	sb.WriteString("**Save this Northstar?**\n")
	sb.WriteString("- Type **\"save\"** to store in `.nerd/northstar.mg` and knowledge base\n")
	sb.WriteString("- Type **\"edit\"** to make changes\n")
	sb.WriteString("- Type **\"campaign\"** to save AND start a campaign based on this vision\n")

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: sb.String(),
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "save / edit / campaign..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) handleNorthstarSummary(input string) (tea.Model, tea.Cmd) {
	lower := strings.ToLower(input)

	switch lower {
	case "save", "yes", "y":
		return m.saveNorthstar(false)
	case "campaign":
		return m.saveNorthstar(true)
	case "resume":
		// Show full summary and continue
		return m.showNorthstarSummary()
	case "new":
		// Start fresh
		m.northstarWizard = NewNorthstarWizard()
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: getNorthstarWelcomeMessage(),
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "yes / no..."
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	case "edit", "back":
		m.northstarWizard.Phase = NorthstarProblemStatement
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Returning to edit mode. Starting from Problem Statement.\n\nType \"skip\" to skip any phase.",
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Edit problem or 'skip'..."
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	default:
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Please type **save**, **edit**, or **campaign**.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
}

// =============================================================================
// SAVE & STORAGE
// =============================================================================

func (m Model) saveNorthstar(startCampaign bool) (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	// Track any warnings during save
	var warnings []string

	// Generate Mangle facts
	mangleFacts := generateNorthstarMangle(w)

	// Save to .nerd/northstar.mg
	northstarPath := filepath.Join(m.workspace, ".nerd", "northstar.mg")
	if err := os.WriteFile(northstarPath, []byte(mangleFacts), 0644); err != nil {
		warnings = append(warnings, fmt.Sprintf("Failed to save northstar.mg: %v", err))
	}

	// Save to knowledge database
	if m.localDB != nil {
		if errs := saveNorthstarToKnowledgeBase(m.localDB, w); len(errs) > 0 {
			warnings = append(warnings, fmt.Sprintf("Knowledge base: %d item(s) failed to save", len(errs)))
		}
	} else {
		warnings = append(warnings, "Knowledge database not available")
	}

	// Assert facts into kernel
	if m.kernel != nil {
		if errs := assertNorthstarFacts(m.kernel, w); len(errs) > 0 {
			warnings = append(warnings, fmt.Sprintf("Kernel: %d fact(s) failed to assert", len(errs)))
		}
	} else {
		warnings = append(warnings, "Kernel not available - facts not loaded")
	}

	// Save JSON backup
	jsonPath := filepath.Join(m.workspace, ".nerd", "northstar.json")
	if jsonData, err := json.MarshalIndent(w, "", "  "); err == nil {
		if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to save JSON backup: %v", err))
		}
	}

	// Exit wizard mode
	m.awaitingNorthstar = false
	m.northstarWizard = nil
	m.textarea.Placeholder = "Ask me anything... (Enter to send, Alt+Enter for newline, Ctrl+C to exit)"

	var msg string
	if startCampaign {
		msg = fmt.Sprintf(`## 🌟 Northstar Saved!

Stored to:
- `+"`%s`"+`
- `+"`%s`"+`
- Knowledge database

**Starting campaign based on this vision...**

_The Campaign Planner will decompose your vision into actionable phases._`,
			northstarPath, jsonPath)

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: msg,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()

		// Start campaign
		goal := fmt.Sprintf("Implement the Northstar vision: %s", w.Mission)
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.startCampaign(goal))
	}

	var msgBuilder strings.Builder
	msgBuilder.WriteString("## 🌟 Northstar Saved!\n\n")
	msgBuilder.WriteString("Your vision has been stored to:\n")
	msgBuilder.WriteString(fmt.Sprintf("- `%s` (Mangle facts)\n", northstarPath))
	msgBuilder.WriteString(fmt.Sprintf("- `%s` (JSON backup)\n", jsonPath))
	msgBuilder.WriteString("- Knowledge database (semantic search)\n\n")

	if len(warnings) > 0 {
		msgBuilder.WriteString("**Warnings:**\n")
		for _, w := range warnings {
			msgBuilder.WriteString(fmt.Sprintf("- ⚠️ %s\n", w))
		}
		msgBuilder.WriteString("\n")
	}

	msgBuilder.WriteString("The kernel now has access to your vision for reasoning.\n\n")
	msgBuilder.WriteString("**Next steps:**\n")
	msgBuilder.WriteString("- Run `/campaign start <goal>` to start building\n")
	msgBuilder.WriteString("- Run `/query northstar_mission` to query the kernel\n")
	msgBuilder.WriteString("- The vision will inform all future planning")

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: msgBuilder.String(),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// =============================================================================
// MANGLE GENERATION
// =============================================================================

func generateNorthstarMangle(w *NorthstarWizardState) string {
	var sb strings.Builder

	sb.WriteString("# Northstar Vision Facts\n")
	sb.WriteString(fmt.Sprintf("# Generated: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("# This file defines the project's north star and informs kernel reasoning.\n")
	sb.WriteString("# Schema declarations are in internal/core/defaults/schemas.mg\n\n")

	// Core Vision Facts
	sb.WriteString("# Core Vision Facts\n")
	sb.WriteString(fmt.Sprintf("northstar_mission(/ns_mission, %q).\n", w.Mission))
	sb.WriteString(fmt.Sprintf("northstar_problem(/ns_problem, %q).\n", w.Problem))
	sb.WriteString(fmt.Sprintf("northstar_vision(/ns_vision, %q).\n", w.Vision))
	sb.WriteString("\n")

	// Personas
	if len(w.Personas) > 0 {
		sb.WriteString("# User Personas\n")
		for i, p := range w.Personas {
			personaID := fmt.Sprintf("/persona_%d", i+1)
			sb.WriteString(fmt.Sprintf("northstar_persona(%s, %q).\n", personaID, p.Name))
			for _, pain := range p.PainPoints {
				sb.WriteString(fmt.Sprintf("northstar_pain_point(%s, %q).\n", personaID, pain))
			}
			for _, need := range p.Needs {
				sb.WriteString(fmt.Sprintf("northstar_need(%s, %q).\n", personaID, need))
			}
		}
		sb.WriteString("\n")
	}

	// Capabilities
	if len(w.Capabilities) > 0 {
		sb.WriteString("# Capabilities\n")
		for i, c := range w.Capabilities {
			capID := fmt.Sprintf("/cap_%d", i+1)
			sb.WriteString(fmt.Sprintf("northstar_capability(%s, %q, /%s, /%s).\n",
				capID, c.Description, strings.ReplaceAll(c.Timeline, " ", "_"), c.Priority))
		}
		sb.WriteString("\n")
	}

	// Risks
	if len(w.Risks) > 0 {
		sb.WriteString("# Risks\n")
		for i, r := range w.Risks {
			riskID := fmt.Sprintf("/risk_%d", i+1)
			sb.WriteString(fmt.Sprintf("northstar_risk(%s, %q, /%s, /%s).\n",
				riskID, r.Description, r.Likelihood, r.Impact))
			if r.Mitigation != "" && r.Mitigation != "none" {
				sb.WriteString(fmt.Sprintf("northstar_mitigation(%s, %q).\n", riskID, r.Mitigation))
			}
		}
		sb.WriteString("\n")
	}

	// Requirements
	if len(w.Requirements) > 0 {
		sb.WriteString("# Requirements\n")
		for _, r := range w.Requirements {
			reqID := fmt.Sprintf("/%s", strings.ToLower(r.ID))
			sb.WriteString(fmt.Sprintf("northstar_requirement(%s, /%s, %q, /%s).\n",
				reqID, r.Type, r.Description, strings.ReplaceAll(r.Priority, "-", "_")))
		}
		sb.WriteString("\n")
	}

	// Constraints
	if len(w.Constraints) > 0 {
		sb.WriteString("# Constraints\n")
		for i, c := range w.Constraints {
			constraintID := fmt.Sprintf("/constraint_%d", i+1)
			sb.WriteString(fmt.Sprintf("northstar_constraint(%s, %q).\n", constraintID, c))
		}
	}

	return sb.String()
}

// =============================================================================
// HELPERS
// =============================================================================

func (m Model) advanceNorthstarPhase() (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	w.SubStep = 0

	switch w.Phase {
	case NorthstarWelcome:
		w.Phase = NorthstarProblemStatement
	case NorthstarDocIngestion:
		w.Phase = NorthstarProblemStatement
	case NorthstarProblemStatement:
		w.Phase = NorthstarVisionStatement
	case NorthstarVisionStatement:
		w.Phase = NorthstarTargetUsers
	case NorthstarTargetUsers:
		w.Phase = NorthstarCapabilities
	case NorthstarCapabilities:
		w.Phase = NorthstarRedTeaming
	case NorthstarRedTeaming:
		w.Phase = NorthstarRequirements
	case NorthstarRequirements:
		w.Phase = NorthstarConstraints
	case NorthstarConstraints:
		w.Phase = NorthstarSummary
		return m.showNorthstarSummary()
	}

	// Recursively handle the new phase with empty input to show prompt
	return m.showNorthstarPhasePrompt()
}

func (m Model) previousNorthstarPhase() (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	w.SubStep = 0

	switch w.Phase {
	case NorthstarProblemStatement:
		w.Phase = NorthstarWelcome
	case NorthstarVisionStatement:
		w.Phase = NorthstarProblemStatement
	case NorthstarTargetUsers:
		w.Phase = NorthstarVisionStatement
	case NorthstarCapabilities:
		w.Phase = NorthstarTargetUsers
	case NorthstarRedTeaming:
		w.Phase = NorthstarCapabilities
	case NorthstarRequirements:
		w.Phase = NorthstarRedTeaming
	case NorthstarConstraints:
		w.Phase = NorthstarRequirements
	case NorthstarSummary:
		w.Phase = NorthstarConstraints
	}

	return m.showNorthstarPhasePrompt()
}

func (m Model) showNorthstarPhasePrompt() (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	var prompt string
	var placeholder string

	switch w.Phase {
	case NorthstarWelcome:
		prompt = getNorthstarWelcomeMessage()
		placeholder = "yes / no..."
	case NorthstarDocIngestion:
		prompt = "Enter document paths or type \"done\":"
		placeholder = "Document paths..."
	case NorthstarProblemStatement:
		prompt = "## Problem Statement\n\n**What problem does this project solve?**"
		placeholder = "Describe the problem..."
	case NorthstarVisionStatement:
		if w.SubStep == 0 {
			prompt = "## Vision\n\n**If this succeeds wildly, what does the world look like?**"
			placeholder = "Describe the vision..."
		} else {
			prompt = "**Distill into a one-sentence mission:**"
			placeholder = "Mission statement..."
		}
	case NorthstarTargetUsers:
		prompt = "## Target Users\n\n**Describe a target user persona** (or \"done\"):"
		placeholder = "User persona..."
	case NorthstarCapabilities:
		prompt = "## Capabilities\n\n**What's one capability this project will have?** (or \"done\"):"
		placeholder = "Capability..."
	case NorthstarRedTeaming:
		prompt = "## Red Teaming 🔴\n\n**What could make this FAIL?** (or \"done\"):"
		placeholder = "Risk..."
	case NorthstarRequirements:
		prompt = "## Requirements\n\n**What's a must-have requirement?** (or \"done\", or \"auto\"):"
		placeholder = "Requirement..."
	case NorthstarConstraints:
		prompt = "## Constraints\n\n**What are the hard constraints?** (or \"done\"):"
		placeholder = "Constraints..."
	}

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: prompt,
		Time:    time.Now(),
	})
	m.textarea.Placeholder = placeholder
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func (m Model) autoGenerateRequirements() (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	// If LLM is available, use it for intelligent requirement generation
	if m.client != nil && (len(w.Capabilities) > 0 || len(w.Risks) > 0 || len(w.ExtractedFacts) > 0) {
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Analyzing vision, capabilities, and risks to generate requirements... This may take a moment.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true

		// Start async requirement generation
		return m, tea.Batch(m.spinner.Tick, m.generateRequirementsWithLLM())
	}

	// Fallback: Simple heuristic-based generation
	return m.autoGenerateRequirementsHeuristic()
}

// autoGenerateRequirementsHeuristic uses simple heuristics to generate requirements.
// This is the fallback when LLM is not available.
func (m Model) autoGenerateRequirementsHeuristic() (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	initialCount := len(w.Requirements)

	// Generate requirements from capabilities
	for _, cap := range w.Capabilities {
		if cap.Priority == "critical" || cap.Priority == "high" {
			w.Requirements = append(w.Requirements, NorthstarRequirement{
				ID:          fmt.Sprintf("REQ-%03d", len(w.Requirements)+1),
				Type:        "functional",
				Description: fmt.Sprintf("Implement: %s", cap.Description),
				Priority:    "must-have",
				Source:      "capability",
			})
		}
	}

	// Generate requirements from risks (mitigations)
	for _, risk := range w.Risks {
		if risk.Impact == "high" && risk.Mitigation != "" && risk.Mitigation != "none" {
			w.Requirements = append(w.Requirements, NorthstarRequirement{
				ID:          fmt.Sprintf("REQ-%03d", len(w.Requirements)+1),
				Type:        "non-functional",
				Description: fmt.Sprintf("Risk mitigation: %s", risk.Mitigation),
				Priority:    "should-have",
				Source:      "risk-mitigation",
			})
		}
	}

	// Generate requirements from research-extracted facts
	for _, fact := range w.ExtractedFacts {
		// Classify the fact type based on keywords
		factLower := strings.ToLower(fact)
		var reqType, priority string

		if strings.Contains(factLower, "must") || strings.Contains(factLower, "require") ||
			strings.Contains(factLower, "need") || strings.Contains(factLower, "critical") {
			reqType = "functional"
			priority = "must-have"
		} else if strings.Contains(factLower, "performance") || strings.Contains(factLower, "security") ||
			strings.Contains(factLower, "scalab") || strings.Contains(factLower, "reliab") {
			reqType = "non-functional"
			priority = "should-have"
		} else if strings.Contains(factLower, "constraint") || strings.Contains(factLower, "limit") ||
			strings.Contains(factLower, "cannot") || strings.Contains(factLower, "must not") {
			reqType = "constraint"
			priority = "must-have"
		} else {
			// Skip facts that don't clearly map to requirements
			continue
		}

		w.Requirements = append(w.Requirements, NorthstarRequirement{
			ID:          fmt.Sprintf("REQ-%03d", len(w.Requirements)+1),
			Type:        reqType,
			Description: fact,
			Priority:    priority,
			Source:      "research",
		})
	}

	generated := len(w.Requirements) - initialCount
	var sources []string
	if len(w.Capabilities) > 0 {
		sources = append(sources, "capabilities")
	}
	if len(w.Risks) > 0 {
		sources = append(sources, "risks")
	}
	if len(w.ExtractedFacts) > 0 {
		sources = append(sources, "research documents")
	}

	sourceStr := "capabilities and risks"
	if len(sources) > 0 {
		sourceStr = strings.Join(sources, ", ")
	}

	m.history = append(m.history, Message{
		Role: "assistant",
		Content: fmt.Sprintf(`✓ Auto-generated **%d requirements** from %s.

_Add more manually or type "done" to continue._`, generated, sourceStr),
		Time: time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// generateRequirementsWithLLM uses the LLM to intelligently generate requirements.
func (m Model) generateRequirementsWithLLM() tea.Cmd {
	return func() tea.Msg {
		w := m.northstarWizard

		// Build context for LLM
		var contextBuilder strings.Builder
		contextBuilder.WriteString("## Project Vision\n")
		contextBuilder.WriteString(fmt.Sprintf("Mission: %s\n\n", w.Mission))
		contextBuilder.WriteString(fmt.Sprintf("Problem: %s\n\n", w.Problem))
		contextBuilder.WriteString(fmt.Sprintf("Vision: %s\n\n", w.Vision))

		if len(w.Capabilities) > 0 {
			contextBuilder.WriteString("## Capabilities\n")
			for _, cap := range w.Capabilities {
				contextBuilder.WriteString(fmt.Sprintf("- [%s/%s] %s\n", cap.Timeline, cap.Priority, cap.Description))
			}
			contextBuilder.WriteString("\n")
		}

		if len(w.Risks) > 0 {
			contextBuilder.WriteString("## Risks\n")
			for _, risk := range w.Risks {
				contextBuilder.WriteString(fmt.Sprintf("- [%s/%s] %s\n", risk.Likelihood, risk.Impact, risk.Description))
				if risk.Mitigation != "" && risk.Mitigation != "none" {
					contextBuilder.WriteString(fmt.Sprintf("  Mitigation: %s\n", risk.Mitigation))
				}
			}
			contextBuilder.WriteString("\n")
		}

		if len(w.Personas) > 0 {
			contextBuilder.WriteString("## User Personas\n")
			for _, p := range w.Personas {
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", p.Name))
				contextBuilder.WriteString(fmt.Sprintf("  Needs: %s\n", strings.Join(p.Needs, ", ")))
			}
			contextBuilder.WriteString("\n")
		}

		if len(w.ExtractedFacts) > 0 {
			contextBuilder.WriteString("## Research Insights\n")
			for _, fact := range w.ExtractedFacts {
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", fact))
			}
			contextBuilder.WriteString("\n")
		}

		contextBuilder.WriteString(`
Based on the above context, generate concrete, actionable requirements.

For each requirement, provide:
1. A clear description
2. Type: functional, non-functional, or constraint
3. Priority: must-have, should-have, or nice-to-have
4. Rationale: why this requirement exists

Format each requirement as:
REQ-NNN | TYPE | PRIORITY | Description | Rationale

Generate between 5-15 requirements focusing on:
- Core functionality from capabilities
- Risk mitigations
- User needs from personas
- Constraints and non-functional requirements (performance, security, usability)`)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Build prompt using helper (supports JIT if available)
		systemPrompt, userPrompt := m.buildNorthstarPrompt(ctx, "requirements", contextBuilder.String())

		response, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			return requirementsGeneratedMsg{err: err}
		}

		// Parse LLM response into requirements
		requirements := parseGeneratedRequirements(response, len(w.Requirements))
		return requirementsGeneratedMsg{requirements: requirements}
	}
}

func getNorthstarWelcomeMessage() string {
	return `# 🌟 NORTHSTAR WIZARD

Welcome to the **Northstar Definition Process**.

This wizard will guide you through defining your project's:
1. **Problem Statement** - What pain are you solving?
2. **Vision** - What does success look like?
3. **Target Users** - Who are you building for?
4. **Capabilities** - What will it do?
5. **Red Teaming** - What could go wrong?
6. **Requirements** - What must be built?
7. **Constraints** - What are the limits?

Your answers will be stored in:
- ` + "`.nerd/northstar.mg`" + ` (Mangle facts for reasoning)
- ` + "`.nerd/northstar.json`" + ` (JSON backup)
- Knowledge database (for semantic search)

---

**Do you have research documents to ingest first?** (yes/no)

_Examples: spec files, design docs, market research, competitor analysis_`
}

// Helper functions

func parseFilePaths(input string) []string {
	input = strings.ReplaceAll(input, "\n", ",")
	parts := strings.Split(input, ",")
	var paths []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func expandPath(workspace, path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workspace, path)
}

func splitAndTrim(input string) []string {
	// Handle both newlines and commas
	input = strings.ReplaceAll(input, "\n", ",")
	parts := strings.Split(input, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parseTimeline(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "now", "core", "mvp":
		return "now"
	case "2", "6", "6mo", "6 months":
		return "6mo"
	case "3", "1yr", "1 year", "year":
		return "1yr"
	case "4", "3yr", "3 years", "3+":
		return "3yr"
	case "5", "moonshot", "dream":
		return "moonshot"
	default:
		return lower
	}
}

func parsePriority(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "critical", "must":
		return "critical"
	case "2", "high", "should":
		return "high"
	case "3", "medium", "nice":
		return "medium"
	case "4", "low", "someday":
		return "low"
	default:
		return "medium"
	}
}

func parseLikelihood(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "high", "h", "likely":
		return "high"
	case "2", "medium", "m", "moderate":
		return "medium"
	case "3", "low", "l", "unlikely":
		return "low"
	default:
		return "medium"
	}
}

func parseReqType(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "functional", "func", "f":
		return "functional"
	case "2", "non-functional", "nonfunctional", "nf", "quality":
		return "non-functional"
	case "3", "constraint", "c", "limit":
		return "constraint"
	default:
		return "functional"
	}
}

func parseReqPriority(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "must", "must-have", "critical":
		return "must-have"
	case "2", "should", "should-have", "important":
		return "should-have"
	case "3", "nice", "nice-to-have", "optional":
		return "nice-to-have"
	default:
		return "should-have"
	}
}

func truncateWithEllipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// saveNorthstarToKnowledgeBase stores northstar data in the knowledge database
// Returns a slice of errors encountered during storage (empty if all succeeded)
func saveNorthstarToKnowledgeBase(db interface {
	StoreKnowledgeAtom(concept, content string, confidence float64) error
}, w *NorthstarWizardState) []error {
	var errs []error

	// Store mission
	if err := db.StoreKnowledgeAtom("northstar:mission", w.Mission, 1.0); err != nil {
		errs = append(errs, err)
	}
	if err := db.StoreKnowledgeAtom("northstar:problem", w.Problem, 1.0); err != nil {
		errs = append(errs, err)
	}
	if err := db.StoreKnowledgeAtom("northstar:vision", w.Vision, 1.0); err != nil {
		errs = append(errs, err)
	}

	// Store personas
	for _, p := range w.Personas {
		if err := db.StoreKnowledgeAtom("northstar:persona", fmt.Sprintf("%s: %s", p.Name, strings.Join(p.Needs, ", ")), 0.9); err != nil {
			errs = append(errs, err)
		}
	}

	// Store capabilities
	for _, c := range w.Capabilities {
		if err := db.StoreKnowledgeAtom("northstar:capability", fmt.Sprintf("[%s/%s] %s", c.Timeline, c.Priority, c.Description), 0.9); err != nil {
			errs = append(errs, err)
		}
	}

	// Store risks
	for _, r := range w.Risks {
		if err := db.StoreKnowledgeAtom("northstar:risk", fmt.Sprintf("[%s/%s] %s - Mitigation: %s", r.Likelihood, r.Impact, r.Description, r.Mitigation), 0.85); err != nil {
			errs = append(errs, err)
		}
	}

	// Store requirements
	for _, r := range w.Requirements {
		if err := db.StoreKnowledgeAtom("northstar:requirement", fmt.Sprintf("[%s] %s: %s (%s)", r.ID, r.Type, r.Description, r.Priority), 0.95); err != nil {
			errs = append(errs, err)
		}
	}

	// Store constraints
	for _, c := range w.Constraints {
		if err := db.StoreKnowledgeAtom("northstar:constraint", c, 0.9); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// assertNorthstarFacts injects ALL northstar facts into the kernel for reasoning
// Returns a slice of errors encountered during assertion (empty if all succeeded)
func assertNorthstarFacts(kernel interface{ AssertString(fact string) error }, w *NorthstarWizardState) []error {
	var errs []error

	// Helper to assert and collect errors
	assert := func(fact string) {
		if err := kernel.AssertString(fact); err != nil {
			errs = append(errs, err)
		}
	}

	// Mark that northstar is defined
	assert("northstar_defined().")

	// Core vision facts
	assert(fmt.Sprintf("northstar_mission(/ns_mission, %q).", w.Mission))
	assert(fmt.Sprintf("northstar_problem(/ns_problem, %q).", w.Problem))
	assert(fmt.Sprintf("northstar_vision(/ns_vision, %q).", w.Vision))

	// Personas with pain points and needs
	for i, p := range w.Personas {
		personaID := fmt.Sprintf("/persona_%d", i+1)
		assert(fmt.Sprintf("northstar_persona(%s, %q).", personaID, p.Name))
		for _, pain := range p.PainPoints {
			assert(fmt.Sprintf("northstar_pain_point(%s, %q).", personaID, pain))
		}
		for _, need := range p.Needs {
			assert(fmt.Sprintf("northstar_need(%s, %q).", personaID, need))
		}
	}

	// Capabilities
	for i, c := range w.Capabilities {
		capID := fmt.Sprintf("/cap_%d", i+1)
		timeline := strings.ReplaceAll(c.Timeline, " ", "_")
		assert(fmt.Sprintf("northstar_capability(%s, %q, /%s, /%s).", capID, c.Description, timeline, c.Priority))
	}

	// Risks and mitigations
	for i, r := range w.Risks {
		riskID := fmt.Sprintf("/risk_%d", i+1)
		assert(fmt.Sprintf("northstar_risk(%s, %q, /%s, /%s).", riskID, r.Description, r.Likelihood, r.Impact))
		if r.Mitigation != "" && r.Mitigation != "none" {
			assert(fmt.Sprintf("northstar_mitigation(%s, %q).", riskID, r.Mitigation))
		}
	}

	// Requirements
	for _, r := range w.Requirements {
		reqID := fmt.Sprintf("/%s", strings.ToLower(r.ID))
		priority := strings.ReplaceAll(r.Priority, "-", "_")
		assert(fmt.Sprintf("northstar_requirement(%s, /%s, %q, /%s).", reqID, r.Type, r.Description, priority))
	}

	// Constraints
	for i, c := range w.Constraints {
		constraintID := fmt.Sprintf("/constraint_%d", i+1)
		assert(fmt.Sprintf("northstar_constraint(%s, %q).", constraintID, c))
	}

	return errs
}

// buildNorthstarPrompt constructs system and user prompts for northstar LLM operations.
// It attempts to use the JIT compiler if available, otherwise falls back to static prompts.
func (m Model) buildNorthstarPrompt(ctx context.Context, phase, content string) (systemPrompt, userPrompt string) {
	// Prefer JIT compilation if available (northstar atoms are phase-tagged)
	if m.jitCompiler != nil {
		cc := m.buildNorthstarCompilationContext(phase)
		if res, err := m.jitCompiler.Compile(ctx, cc); err == nil && res != nil && strings.TrimSpace(res.Prompt) != "" {
			return res.Prompt, content
		}
	}

	// Fallback: static prompts based on phase
	return getNorthstarStaticPrompt(phase), content
}

// buildNorthstarCompilationContext creates a CompilationContext for the northstar phase.
func (m Model) buildNorthstarCompilationContext(phase string) *prompt.CompilationContext {
	cc := prompt.NewCompilationContext()

	// Set northstar phase
	cc.NorthstarPhase = "/" + phase

	// Set operational mode to planning
	cc.OperationalMode = "/planning"

	// Set shard type to researcher (for document analysis)
	cc.ShardType = "/researcher"
	cc.ShardID = "northstar_wizard"
	cc.ShardName = "Northstar Wizard"

	// Set intent verb
	cc.IntentVerb = "/research"

	// Token budget (80k for prompt, 20k for response)
	cc.TokenBudget = 100000
	cc.ReservedTokens = 20000

	return cc
}

// getNorthstarStaticPrompt returns a static prompt for the given northstar phase.
// This is used as a fallback when JIT compilation is not available.
func getNorthstarStaticPrompt(phase string) string {
	switch phase {
	case "doc_ingestion":
		return `You are analyzing research documents to extract key insights for defining a project's vision and requirements.

Analyze the provided documents and extract key insights in the following categories:
- Problem statements or pain points mentioned
- Target users or personas described
- Desired capabilities or features
- Risks or concerns raised
- Constraints or limitations noted
- Success criteria or goals

Return ONLY the extracted insights, one per line. Be concise but specific. Maximum 15 insights.`

	case "requirements":
		return `You are helping generate concrete requirements from a project's vision, capabilities, and risks.

Based on the provided context, generate specific, actionable requirements that:
- Are testable and measurable
- Derive from stated capabilities and risk mitigations
- Address user needs and pain points
- Are categorized as functional, non-functional, or constraints

Return requirements in a structured format with clear prioritization.`

	default:
		return `You are assisting with project vision and planning. Provide clear, actionable insights based on the context provided.`
	}
}

// analyzeNorthstarDocs reads and analyzes research documents using the LLM
// to extract key insights for the northstar definition process.
func (m Model) analyzeNorthstarDocs(docPaths []string) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return northstarDocsAnalyzedMsg{err: fmt.Errorf("LLM client not available")}
		}

		// Read all documents
		var docContents strings.Builder
		for _, path := range docPaths {
			content, err := os.ReadFile(path)
			if err != nil {
				continue // Skip unreadable files
			}
			docContents.WriteString(fmt.Sprintf("\n--- Document: %s ---\n", filepath.Base(path)))
			// Truncate very large files
			text := string(content)
			if len(text) > 10000 {
				text = text[:10000] + "\n... [truncated]"
			}
			docContents.WriteString(text)
			docContents.WriteString("\n")
		}

		if docContents.Len() == 0 {
			return northstarDocsAnalyzedMsg{facts: []string{}}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Build prompt using helper (supports JIT if available)
		systemPrompt, userPrompt := m.buildNorthstarPrompt(ctx, "doc_ingestion", docContents.String())

		response, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			return northstarDocsAnalyzedMsg{err: fmt.Errorf("analysis failed: %w", err)}
		}

		// Parse response into facts
		lines := strings.Split(response, "\n")
		var facts []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Remove leading bullets, dashes, numbers
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			line = strings.TrimPrefix(line, "• ")
			if len(line) > 10 { // Skip very short lines
				facts = append(facts, line)
			}
		}

		return northstarDocsAnalyzedMsg{facts: facts}
	}
}
