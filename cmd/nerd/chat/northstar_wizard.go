// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains the main input router and phase handlers for the Northstar wizard.
//
// File Index (modularized):
//
//	northstar_wizard.go      - Input router and phase handlers (this file)
//	northstar_types.go       - Types, constants, NorthstarWizardState, constructors
//	northstar_utils.go       - Parsing utility functions
//	northstar_persistence.go - Save, Mangle generation, KB storage
//	northstar_navigation.go  - Phase navigation, prompts, auto-generate
//	northstar_llm.go         - LLM integration (doc analysis, requirement generation)
//
// Wizard Phases:
//
//  1. Welcome          - Initial greeting, research doc question
//  2. DocIngestion     - Optional research document analysis
//  3. ProblemStatement - Define the problem being solved
//  4. VisionStatement  - Vision + one-line mission
//  5. TargetUsers      - User personas with pain points/needs
//  6. Capabilities     - Future capabilities with timeline/priority
//  7. RedTeaming       - Risks with likelihood/impact/mitigation
//  8. Requirements     - Crystallized requirements (manual or auto-generated)
//  9. Constraints      - Hard constraints
//  10. Summary         - Review and save
package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// INPUT ROUTER
// =============================================================================

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
