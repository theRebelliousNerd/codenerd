package init

import (
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/tools"
	"codenerd/internal/tools/research"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// getExistingAtomCount returns the number of atoms in an existing KB.
func (i *Initializer) getExistingAtomCount(kbPath string) int {
	db, err := store.NewLocalStore(kbPath)
	if err != nil {
		return 0
	}
	defer db.Close()

	atoms, err := db.GetAllKnowledgeAtoms()
	if err != nil {
		return 0
	}
	return len(atoms)
}

// createAgentKnowledgeBase creates or upgrades the SQLite knowledge base for an agent.
// If upgradeMode is true, it appends new knowledge atoms without reinitializing the schema.
func (i *Initializer) createAgentKnowledgeBase(ctx context.Context, kbPath string, agent RecommendedAgent, upgradeMode bool) (KnowledgeBaseStats, error) {
	stats := KnowledgeBaseStats{}

	// Open the database (NewLocalStore handles schema creation idempotently)
	agentDB, err := store.NewLocalStore(kbPath)
	if err != nil {
		return stats, fmt.Errorf("failed to open agent DB: %w", err)
	}
	if err := i.ensureEmbeddingEngine(); err != nil {
		return stats, err
	}
	agentDB.SetEmbeddingEngine(i.embedEngine)
	defer agentDB.Close()

	// In upgrade mode, get existing atoms for deduplication
	var existingHashes map[string]bool
	if upgradeMode {
		existingAtoms, err := agentDB.GetAllKnowledgeAtoms()
		if err != nil {
			return stats, fmt.Errorf("failed to get existing atoms: %w", err)
		}
		existingHashes = buildAtomHashSet(existingAtoms)
		stats.ExistingAtoms = len(existingAtoms)
		logging.Boot("Upgrade mode: found %d existing atoms in %s", stats.ExistingAtoms, agent.Name)
	} else {
		existingHashes = make(map[string]bool)

		// Inherit shared knowledge pool for new agents (not in upgrade mode)
		sharedKBPath := GetSharedKnowledgePath(i.config.Workspace)
		if SharedKnowledgePoolExists(i.config.Workspace) {
			if inheritErr := InheritSharedKnowledge(agentDB, sharedKBPath); inheritErr != nil {
				logging.Boot("Warning: failed to inherit shared knowledge for %s: %v", agent.Name, inheritErr)
			} else {
				// Re-fetch existing hashes after inheritance
				inheritedAtoms, _ := agentDB.GetAllKnowledgeAtoms()
				existingHashes = buildAtomHashSet(inheritedAtoms)
				stats.NewAtoms += len(inheritedAtoms)
				logging.Boot("Inherited %d shared atoms for %s", len(inheritedAtoms), agent.Name)
			}
		}
	}

	// Add base knowledge atoms for the agent
	baseAtoms := i.generateBaseKnowledgeAtoms(agent)
	for _, atom := range baseAtoms {
		added, err := appendKnowledgeAtom(agentDB, atom.Concept, atom.Content, atom.Confidence, existingHashes)
		if err != nil {
			logging.Boot("Warning: failed to store base atom for %s: %v", agent.Name, err)
			continue
		}
		if added {
			stats.NewAtoms++
		} else {
			stats.SkippedAtoms++
		}
	}

	// Research topics using modular tools
	// =========================================================================
	// Research uses the modular tool registry (internal/tools/research/)
	// Context7 provides LLM-optimized documentation for libraries/frameworks
	// =========================================================================
	if !i.config.SkipResearch && len(agent.Topics) > 0 {
		fmt.Printf("     Researching %d topics for %s...\n", len(agent.Topics), agent.Name)

		// Create a temporary tool registry for research
		registry := tools.NewRegistry()
		if err := research.RegisterAll(registry); err != nil {
			logging.Boot("Warning: failed to register research tools: %v", err)
		} else {
			researchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			for _, topic := range agent.Topics {
				// Try Context7 for documentation
				result, err := registry.Execute(researchCtx, "context7_fetch", map[string]any{"topic": topic})
				if err != nil {
					logging.Boot("Research failed for topic %s: %v", topic, err)
					continue
				}

				if result.Result != "" && len(result.Result) > 100 {
					// Parse research result into knowledge atoms
					atoms := i.parseResearchResult(topic, result.Result)
					for _, atom := range atoms {
						added, err := appendKnowledgeAtom(agentDB, atom.Concept, atom.Content, atom.Confidence, existingHashes)
						if err != nil {
							logging.Boot("Warning: failed to store research atom: %v", err)
							continue
						}
						if added {
							stats.NewAtoms++
						}
					}
					logging.Boot("Added %d atoms from research on %s", len(atoms), topic)
				}
			}
		}

		// Calculate the legacy population proxy from newly added atom count.
		if stats.NewAtoms > 10 {
			stats.QualityScore = 80.0
			stats.QualityRating = "Well populated"
		} else if stats.NewAtoms > 5 {
			stats.QualityScore = 65.0
			stats.QualityRating = "Moderately populated"
		} else {
			stats.QualityScore = 50.0
			stats.QualityRating = "Basic population"
		}
	} else if i.config.SkipResearch {
		fmt.Printf("     Skipping research for %s (--skip-research)\n", agent.Name)
		stats.QualityScore = 50.0
		stats.QualityRating = "Basic population"
	}

	// Calculate total atoms
	finalAtoms, _ := agentDB.GetAllKnowledgeAtoms()
	stats.TotalAtoms = len(finalAtoms)

	return stats, nil
}

// buildAtomHashSet creates a set of content hashes for existing atoms.
func buildAtomHashSet(atoms []store.KnowledgeAtom) map[string]bool {
	hashes := make(map[string]bool)
	for _, atom := range atoms {
		hash := computeAtomHash(atom.Concept, atom.Content)
		hashes[hash] = true
	}
	return hashes
}

// parseResearchResult converts research output into knowledge atoms.
// It splits the content into meaningful chunks for storage.
func (i *Initializer) parseResearchResult(topic, content string) []initKnowledgeAtom {
	var atoms []initKnowledgeAtom

	// Split content into paragraphs/sections
	sections := strings.Split(content, "\n\n")

	for idx, section := range sections {
		section = strings.TrimSpace(section)
		if len(section) < 50 {
			continue // Skip very short sections
		}

		// Truncate very long sections
		if len(section) > 2000 {
			section = section[:2000] + "..."
		}

		// Create atom with topic-based concept
		concept := fmt.Sprintf("%s:section_%d", topic, idx)
		atoms = append(atoms, initKnowledgeAtom{
			Concept:    concept,
			Content:    section,
			Confidence: 0.8, // Research-derived atoms have good confidence
		})
	}

	// Also create a summary atom if we have content
	if len(content) > 100 {
		summary := content
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		atoms = append(atoms, initKnowledgeAtom{
			Concept:    topic + ":summary",
			Content:    summary,
			Confidence: 0.9,
		})
	}

	return atoms
}

// computeAtomHash generates a unique hash for a knowledge atom based on concept and content.
func computeAtomHash(concept, content string) string {
	combined := concept + "::" + content
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes for brevity
}

// appendKnowledgeAtom adds a knowledge atom if it doesn't already exist.
// Returns true if the atom was added, false if it was skipped (duplicate).
func appendKnowledgeAtom(db *store.LocalStore, concept, content string, confidence float64, existingHashes map[string]bool) (bool, error) {
	hash := computeAtomHash(concept, content)

	// Check if this atom already exists
	if existingHashes[hash] {
		return false, nil
	}

	// Store the new atom
	if err := db.StoreKnowledgeAtom(concept, content, confidence); err != nil {
		return false, err
	}

	// Add to hash set to prevent duplicates within this session
	existingHashes[hash] = true
	return true, nil
}

// filterTopicsNeedingResearch checks existing atoms and returns only topics that lack coverage.
// A topic is considered "covered" if there are >= minAtomsPerTopic atoms with matching concepts.
// This prevents redundant Context7 API calls during /init --force upgrades.
func filterTopicsNeedingResearch(existingAtoms []store.KnowledgeAtom, topics []string, minAtomsPerTopic int) []string {
	if len(existingAtoms) == 0 {
		return topics // No existing atoms, research all topics
	}

	// Build a map of topic -> atom count by checking if atom concepts contain topic keywords
	topicCoverage := make(map[string]int)
	for _, topic := range topics {
		topicCoverage[topic] = 0
	}

	// Normalize topic keywords for matching
	topicKeywords := make(map[string][]string)
	for _, topic := range topics {
		// Split topic into keywords (e.g., "go concurrency" -> ["go", "concurrency"])
		keywords := strings.Fields(strings.ToLower(topic))
		topicKeywords[topic] = keywords
	}

	// Count atoms that match each topic
	// Skip inherited atoms from shared pool as they inflate coverage falsely
	for _, atom := range existingAtoms {
		// Skip inherited atoms - they don't represent genuine topic research
		if strings.HasPrefix(atom.Concept, "inherited:") {
			continue
		}
		// Skip base identity/mission atoms - they're boilerplate
		if atom.Concept == "agent_identity" || atom.Concept == "agent_mission" {
			continue
		}

		conceptLower := strings.ToLower(atom.Concept)
		contentLower := strings.ToLower(atom.Content)

		for topic, keywords := range topicKeywords {
			matchCount := 0
			for _, kw := range keywords {
				if strings.Contains(conceptLower, kw) || strings.Contains(contentLower, kw) {
					matchCount++
				}
			}
			// Require at least 2/3 of keywords to match for topic relevance (stricter than 50%)
			// This prevents broad matches like "go" matching everything
			requiredMatches := (len(keywords)*2 + 2) / 3 // ~67% threshold
			if matchCount >= requiredMatches {
				topicCoverage[topic]++
			}
		}
	}

	// Filter to topics needing more research
	needsResearch := make([]string, 0)
	for _, topic := range topics {
		coverage := topicCoverage[topic]
		if coverage < minAtomsPerTopic {
			needsResearch = append(needsResearch, topic)
			logging.Boot("Topic '%s' needs research (current coverage: %d atoms, min: %d)", topic, coverage, minAtomsPerTopic)
		} else {
			logging.Boot("Topic '%s' has sufficient coverage (%d atoms), skipping", topic, coverage)
		}
	}

	return needsResearch
}

// convertStoreAtomsToInitAtoms converts store.KnowledgeAtom to initKnowledgeAtom.
// STUB: Research functionality removed as part of JIT refactor.
func convertStoreAtomsToInitAtoms(storeAtoms []store.KnowledgeAtom) []initKnowledgeAtom {
	atoms := make([]initKnowledgeAtom, 0, len(storeAtoms))
	for _, sa := range storeAtoms {
		atoms = append(atoms, initKnowledgeAtom{
			Concept:    sa.Concept,
			Content:    sa.Content,
			Title:      sa.Concept,
			Confidence: sa.Confidence,
			SourceURL:  "",
		})
	}
	return atoms
}

// generateBaseKnowledgeAtoms generates foundational knowledge for an agent.
func (i *Initializer) generateBaseKnowledgeAtoms(agent RecommendedAgent) []struct {
	Concept    string
	Content    string
	Confidence float64
} {
	atoms := make([]struct {
		Concept    string
		Content    string
		Confidence float64
	}, 0)

	// Add agent identity
	atoms = append(atoms, struct {
		Concept    string
		Content    string
		Confidence float64
	}{
		Concept:    "agent_identity",
		Content:    fmt.Sprintf("I am %s, a specialist agent. %s", agent.Name, agent.Description),
		Confidence: 1.0,
	})

	// Add mission statement
	atoms = append(atoms, struct {
		Concept    string
		Content    string
		Confidence float64
	}{
		Concept:    "agent_mission",
		Content:    fmt.Sprintf("My primary mission is: %s", agent.Reason),
		Confidence: 1.0,
	})

	// Add expertise areas
	for _, topic := range agent.Topics {
		atoms = append(atoms, struct {
			Concept    string
			Content    string
			Confidence float64
		}{
			Concept:    "expertise_area",
			Content:    topic,
			Confidence: 0.9,
		})
	}

	return atoms
}
