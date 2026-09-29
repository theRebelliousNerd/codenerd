// Package chat provides the interactive TUI chat interface for codeNERD.
package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/northstar"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) generateRequirementsWithLLM() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil || m.northstarWizard == nil {
			return requirementsGeneratedMsg{err: fmt.Errorf("northstar wizard or LLM client not available")}
		}
		budget, err := northstar.ResolveDeriveBudget(m.workspace)
		if err != nil {
			return requirementsGeneratedMsg{err: err}
		}
		reqs, err := northstar.DeriveRequirements(m.northstarContext(), m.client, nil, budget, northstarRequirementsInput(m.northstarWizard))
		if err != nil {
			return requirementsGeneratedMsg{err: err}
		}
		out := make([]NorthstarRequirement, 0, len(reqs))
		for _, req := range reqs {
			if len(req.Supports) > 0 || len(req.Addresses) > 0 {
				return requirementsGeneratedMsg{err: fmt.Errorf("automatic requirements include capability or risk links that this wizard cannot preserve")}
			}
			out = append(out, NorthstarRequirement{ID: req.ID, Type: req.Type, Description: req.Description, Priority: req.Priority, Source: req.Source})
		}
		return requirementsGeneratedMsg{requirements: out}
	}
}

func (m Model) analyzeNorthstarDocs(docPaths []string) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return northstarDocsAnalyzedMsg{err: fmt.Errorf("LLM client not available")}
		}
		if len(docPaths) == 0 {
			return northstarDocsAnalyzedMsg{err: fmt.Errorf("no documents supplied")}
		}
		docs := make([]northstar.SourceDocument, 0, len(docPaths))
		for _, path := range docPaths {
			readPath := path
			if m.workspace != "" && !filepath.IsAbs(path) {
				readPath = filepath.Join(m.workspace, path)
			}
			content, err := os.ReadFile(readPath)
			if err != nil {
				return northstarDocsAnalyzedMsg{err: fmt.Errorf("read northstar document %s: %w", path, err)}
			}
			docs = append(docs, northstar.SourceDocument{Path: path, Body: string(content)})
		}
		budget, err := northstar.ResolveDeriveBudget(m.workspace)
		if err != nil {
			return northstarDocsAnalyzedMsg{err: err}
		}
		class, err := northstar.ClassifyDocuments(m.northstarContext(), m.client, nil, budget, docs)
		if err != nil {
			return northstarDocsAnalyzedMsg{err: err}
		}
		return northstarDocsAnalyzedMsg{facts: class.Lines()}
	}
}

func (m Model) northstarContext() context.Context {
	if m.shutdownCtx != nil {
		return m.shutdownCtx
	}
	return context.Background()
}

func northstarRequirementsInput(w *NorthstarWizardState) northstar.WizardRequirementsInput {
	state := northstar.WizardRequirementsInput{
		Mission: w.Mission, Problem: w.Problem, Vision: w.Vision,
		Constraints: w.Constraints, ExtractedFacts: w.ExtractedFacts,
	}
	for _, p := range w.Personas {
		state.Personas = append(state.Personas, northstar.WizardPersona{Name: p.Name, PainPoints: p.PainPoints, Needs: p.Needs})
	}
	for i, c := range w.Capabilities {
		state.Capabilities = append(state.Capabilities, northstar.WizardCapability{ID: fmt.Sprintf("cap_%d", i+1), Description: c.Description, Timeline: c.Timeline, Priority: c.Priority})
	}
	for i, r := range w.Risks {
		state.Risks = append(state.Risks, northstar.WizardRisk{ID: fmt.Sprintf("risk_%d", i+1), Description: r.Description, Likelihood: r.Likelihood, Impact: r.Impact, Mitigation: r.Mitigation})
	}
	for _, r := range w.Requirements {
		state.ExistingRequirements = append(state.ExistingRequirements, northstar.WizardRequirement{ID: r.ID, Type: r.Type, Description: r.Description, Priority: r.Priority, Source: r.Source})
	}
	return state
}
