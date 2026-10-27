// Package campaign provides multi-phase campaign orchestration.
// This file implements specialist knowledge injection for campaign tasks.
package campaign

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// SpecialistKnowledgeProvider retrieves relevant knowledge from specialist databases.
type SpecialistKnowledgeProvider interface {
	// GetRelevantKnowledge returns knowledge atoms relevant to a task from a specialist's DB.
	GetRelevantKnowledge(ctx context.Context, specialistName, task string, limit int) ([]KnowledgeAtom, error)

	// GetSpecialistPath returns the knowledge DB path for a specialist.
	GetSpecialistPath(specialistName string) string
}

// KnowledgeAtom represents a piece of specialist knowledge.
type KnowledgeAtom struct {
	Content    string  // The knowledge content
	Source     string  // Where it came from (e.g., file path)
	Relevance  float64 // 0-1 relevance score
	Specialist string  // Which specialist provided it
}

// LocalSpecialistKnowledgeProvider implements SpecialistKnowledgeProvider using
// the specialists' local stores. Semantic search embeds with each store's own
// configured engine; the provider names no model of its own.
type LocalSpecialistKnowledgeProvider struct {
	workspace string
}

// NewLocalSpecialistKnowledgeProvider creates a provider for specialist knowledge.
func NewLocalSpecialistKnowledgeProvider(workspace string) *LocalSpecialistKnowledgeProvider {
	return &LocalSpecialistKnowledgeProvider{workspace: workspace}
}

// GetSpecialistPath returns the knowledge DB path for a specialist.
func (p *LocalSpecialistKnowledgeProvider) GetSpecialistPath(specialistName string) string {
	return types.AgentKnowledgeDBPath(filepath.Join(p.workspace, ".nerd"), specialistName)
}

// GetRelevantKnowledge retrieves knowledge atoms from a specialist's database.
func (p *LocalSpecialistKnowledgeProvider) GetRelevantKnowledge(ctx context.Context, specialistName, task string, limit int) ([]KnowledgeAtom, error) {
	if limit <= 0 {
		limit = 5
	}

	dbPath := p.GetSpecialistPath(specialistName)

	// A specialist without a knowledge DB has no knowledge. Opening the path
	// anyway would create an empty store under .nerd/agents for every
	// specialist a task names (store.NewLocalStore creates what it opens).
	if _, err := os.Stat(dbPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("specialist %s knowledge DB: %w", specialistName, err)
		}
		logging.CampaignDebug("No knowledge DB for specialist %s", specialistName)
		return nil, nil
	}

	localStore, err := store.NewLocalStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open specialist %s knowledge DB: %w", specialistName, err)
	}
	defer localStore.Close()

	// Query for relevant knowledge atoms using semantic search
	atoms := make([]KnowledgeAtom, 0, limit)

	// First try semantic search if embeddings are available
	knowledgeAtoms, err := localStore.SearchKnowledgeAtomsSemantic(ctx, task, limit)
	if err == nil && len(knowledgeAtoms) > 0 {
		for _, ka := range knowledgeAtoms {
			atoms = append(atoms, KnowledgeAtom{
				Content:    ka.Content,
				Source:     ka.Source,
				Relevance:  ka.Confidence,
				Specialist: specialistName,
			})
		}
		logging.CampaignDebug("Retrieved %d knowledge atoms from %s via semantic search", len(atoms), specialistName)
		return atoms, nil
	}

	// Fallback: get knowledge atoms by concept (using task as concept query)
	// Extract a key concept from the task description
	concept := extractKeyConcept(task)
	knowledgeAtoms, err = localStore.GetKnowledgeAtoms(concept)
	if err != nil {
		logging.CampaignDebug("Failed to get knowledge atoms from %s: %v", specialistName, err)
		return nil, nil
	}

	for _, ka := range knowledgeAtoms {
		atoms = append(atoms, KnowledgeAtom{
			Content:    ka.Content,
			Source:     ka.Source,
			Relevance:  ka.Confidence,
			Specialist: specialistName,
		})
	}

	// Limit the results
	if len(atoms) > limit {
		atoms = atoms[:limit]
	}

	logging.CampaignDebug("Retrieved %d knowledge atoms from %s (concept fallback)", len(atoms), specialistName)
	return atoms, nil
}

// extractKeyConcept extracts a key concept from a task description.
func extractKeyConcept(task string) string {
	// Simple extraction: use first 50 chars or first sentence
	if len(task) > 50 {
		task = task[:50]
	}
	// Remove newlines
	task = strings.ReplaceAll(task, "\n", " ")
	return task
}

// FormatSpecialistKnowledge formats knowledge atoms for injection into task context.
func FormatSpecialistKnowledge(atoms []KnowledgeAtom) string {
	if len(atoms) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n=== SPECIALIST KNOWLEDGE ===\n")

	// Group by specialist
	bySpecialist := make(map[string][]KnowledgeAtom)
	for _, atom := range atoms {
		bySpecialist[atom.Specialist] = append(bySpecialist[atom.Specialist], atom)
	}

	for specialist, specialistAtoms := range bySpecialist {
		sb.WriteString(fmt.Sprintf("\nFrom %s:\n", specialist))
		for i, atom := range specialistAtoms {
			sb.WriteString(fmt.Sprintf("%d. ", i+1))
			// Truncate long content
			content := atom.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			sb.WriteString(content)
			if atom.Source != "" {
				sb.WriteString(fmt.Sprintf(" (source: %s)", atom.Source))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// buildTaskInputWithSpecialistKnowledge extends buildTaskInput with specialist knowledge injection.
func (o *Orchestrator) buildTaskInputWithSpecialistKnowledge(ctx context.Context, task *Task, specialist string) (string, error) {
	// Start with base input
	input, err := o.buildTaskInput(ctx, task)
	if err != nil {
		return "", err
	}

	// If we have a knowledge provider and a specialist, inject their knowledge
	if o.specialistKnowledgeProvider != nil && specialist != "" {
		atoms, err := o.specialistKnowledgeProvider.GetRelevantKnowledge(ctx, specialist, task.Description, 5)
		if err != nil {
			// Enrichment, not the task: the task runs without it, and says why.
			logging.Get(logging.CategoryCampaign).Warn("Specialist %s knowledge unavailable for task %s: %v", specialist, task.ID, err)
		} else if len(atoms) > 0 {
			input += FormatSpecialistKnowledge(atoms)
			logging.Campaign("Injected %d knowledge atoms from specialist %s into task %s", len(atoms), specialist, task.ID)
		}
	}

	return input, nil
}
