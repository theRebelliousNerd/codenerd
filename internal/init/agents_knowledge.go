package init

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/knowledge"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/tools"
	"codenerd/internal/tools/research"
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
	topics := agent.Topics
	// A derived research list replaces the topics this KB fetches. The
	// RecommendedAgent.Topics field stays the catalog: an empty derived list
	// means the winning sources already cover every topic, so nothing is fetched.
	if researched, ok := ecosystemResearchTopics(i.config.Workspace, agent.Name); ok {
		topics = researched
	}
	if upgradeMode {
		// Placeholders name every topic; left in place they make the
		// coverage check below read unresearched topics as covered.
		if n, err := knowledge.PrunePlaceholders(ctx, agentDB.GetDB()); err != nil {
			return stats, fmt.Errorf("prune placeholder atoms: %w", err)
		} else if n > 0 {
			logging.Boot("Pruned %d placeholder atom(s) from %s", n, agent.Name)
		}
		existingAtoms, err := agentDB.GetAllKnowledgeAtoms()
		if err != nil {
			return stats, fmt.Errorf("failed to get existing atoms: %w", err)
		}
		existingHashes = buildAtomHashSet(existingAtoms)
		stats.ExistingAtoms = len(existingAtoms)
		logging.Boot("Upgrade mode: found %d existing atoms in %s", stats.ExistingAtoms, agent.Name)
		// An upgrade researches only what the KB does not already cover.
		// filterTopicsNeedingResearch was written and tested for exactly this
		// and never called, so every `init --force` re-fetched every topic of
		// every agent -- the atoms deduplicated on hash, the Context7 calls
		// and the two-minute budget did not.
		topics = filterTopicsNeedingResearch(existingAtoms, topics, minAtomsPerCoveredTopic)
		stats.SkippedTopics = len(agent.Topics) - len(topics)
	} else {
		existingHashes = make(map[string]bool)
	}

	// Winning corpus sources are knowledge atoms. Each paragraph (or rune
	// page, when the orient threshold says so) is stored whole. The vector
	// row carries the source; a vector failure does not drop the atom.
	for _, chunk := range ecosystemKnowledgeChunks(i.config.Workspace, agent.Name) {
		added, err := appendKnowledgeAtom(ctx, agentDB, chunk.Concept, chunk.Content, 0.95, "init/ecosystem", existingHashes)
		if err != nil {
			logging.Boot("Warning: failed to store ecosystem atom for %s: %v", agent.Name, err)
			continue
		}
		if !added {
			stats.SkippedAtoms++
			continue
		}
		stats.NewAtoms++
	}

	// Research topics using modular tools
	// =========================================================================
	// Research uses the modular tool registry (internal/tools/research/)
	// Context7 provides LLM-optimized documentation for libraries/frameworks
	// =========================================================================
	// A vision expert's own questions are answered by grounded web search.
	if questions, staleDays := ecosystemDomainResearch(i.config.Workspace, agent.Name); len(questions) > 0 && !i.config.SkipResearch {
		added, notes := i.researchDomainQuestions(ctx, agentDB, agent, questions, staleDays, existingHashes)
		stats.NewAtoms += added
		stats.Notes = append(stats.Notes, notes...)
	}

	if stats.SkippedTopics > 0 {
		fmt.Fprintf(i.output(), "     %d of %d topics already covered in %s's KB; not researched again\n", stats.SkippedTopics, len(agent.Topics), agent.Name)
	}
	if !i.config.SkipResearch {
		subjects := KnowledgeSubjects(i.config.Workspace, strings.ToLower(agent.Name), agent.Domains, topics)
		if len(subjects) > 0 {
			fmt.Fprintf(i.output(), "     Acquiring knowledge on %d subject(s) for %s...\n", len(subjects), agent.Name)
			added, outcomes, err := knowledge.Fill(ctx, agentDB, i.knowledgeAcquirer(), subjects)
			stats.NewAtoms += added
			stats.Outcomes = append(stats.Outcomes, outcomes...)
			if err != nil {
				stats.Notes = append(stats.Notes, fmt.Sprintf("%s: knowledge acquisition stopped: %v", agent.Name, err))
			}
			for _, out := range outcomes {
				if out.Status == knowledge.StatusFailed {
					stats.Notes = append(stats.Notes, fmt.Sprintf("%s: %s", agent.Name, out))
				}
			}
			fmt.Fprintf(i.output(), "     %s: %s\n", agent.Name, knowledge.SummarizeOutcomes(outcomes))
		}
	} else {
		fmt.Fprintf(i.output(), "     Skipping research for %s (--skip-research)\n", agent.Name)
	}

	// No placeholders: an agent no source taught has no knowledge, and the
	// init result says so instead of filling the KB with its own name.
	if interim, _ := agentDB.GetAllKnowledgeAtoms(); len(interim) == 0 {
		stats.Notes = append(stats.Notes, fmt.Sprintf("%s has no knowledge: no source taught it; see `nerd check-kb`", agent.Name))
	}
	// Vector backfill: atoms written before embedding was wired (or while the
	// engine was missing) get vectors now so semantic search finds them.
	if n, err := backfillKnowledgeVectors(ctx, agentDB); err != nil {
		logging.Boot("Warning: vector backfill for %s: %v", agent.Name, err)
	} else if n > 0 {
		logging.Boot("Backfilled %d vectors for %s", n, agent.Name)
	}

	// Calculate total atoms
	finalAtoms, _ := agentDB.GetAllKnowledgeAtoms()
	stats.TotalAtoms = len(finalAtoms)

	return stats, nil
}

// topicCache shares one run's topic fetches across agents: a topic two
// specialists both research is fetched once, and a fetch already in flight is
// awaited rather than repeated.
type topicCache struct {
	mu      sync.Mutex
	entries map[string]*topicFetch
}

type topicFetch struct {
	done chan struct{}
	body string
	err  error
}

func (c *topicCache) fetch(ctx context.Context, topic string, fetch func(ctx context.Context, topic string) (string, error)) (string, error) {
	key := strings.ToLower(strings.TrimSpace(topic))
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[string]*topicFetch)
	}
	if entry, ok := c.entries[key]; ok {
		c.mu.Unlock()
		select {
		case <-entry.done:
			return entry.body, entry.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	entry := &topicFetch{done: make(chan struct{})}
	c.entries[key] = entry
	c.mu.Unlock()

	entry.body, entry.err = fetch(ctx, topic)
	close(entry.done)
	return entry.body, entry.err
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

// parseResearchResult converts research output into knowledge atoms, one per
// paragraph. Each paragraph is stored whole: a section cut at 2000
// characters ended mid-sentence and the specialist reasoned from the half it
// was given, and the ":summary" atom this used to add was not a summary but
// the first 500 characters of the text under a name that claimed otherwise.
func (i *Initializer) parseResearchResult(topic, content string) []initKnowledgeAtom {
	var atoms []initKnowledgeAtom

	// Split content into paragraphs/sections
	sections := strings.Split(content, "\n\n")

	for idx, section := range sections {
		section = strings.TrimSpace(section)
		if len(section) < 50 {
			continue // Skip very short sections
		}

		// Create atom with topic-based concept
		concept := fmt.Sprintf("%s:section_%d", topic, idx)
		atoms = append(atoms, initKnowledgeAtom{
			Concept:    concept,
			Content:    section,
			Confidence: 0.8, // Research-derived atoms have good confidence
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
func appendKnowledgeAtom(ctx context.Context, db *store.LocalStore, concept, content string, confidence float64, source string, existingHashes map[string]bool) (bool, error) {
	hash := computeAtomHash(concept, content)

	// Check if this atom already exists
	if existingHashes[hash] {
		return false, nil
	}

	// Store the new atom with its embedding so semantic search can find it.
	// StoreKnowledgeAtom alone leaves the vectors table empty, which is how
	// agents ended up with knowledge_atoms rows but zero searchable vectors.
	if err := db.StoreKnowledgeAtomWithEmbedding(ctx, concept, content, confidence, source); err != nil {
		return false, err
	}

	// Add to hash set to prevent duplicates within this session
	existingHashes[hash] = true
	return true, nil
}

// minAtomsPerCoveredTopic is how many genuine atoms (not inherited, not the
// identity boilerplate) a topic needs in an agent's KB before an upgrade stops
// researching it again. Two sections of one fetched page are already more
// than a hash-duplicate refetch would add.
const minAtomsPerCoveredTopic = 2

// topicFetcher returns the function one topic's documentation is fetched
// with: Initializer.fetchTopic when set (tests), otherwise Context7 through a
// fresh modular research registry.
func (i *Initializer) topicFetcher() (func(ctx context.Context, topic string) (string, error), error) {
	if i.fetchTopic != nil {
		return i.fetchTopic, nil
	}
	registry := tools.NewRegistry()
	if err := research.RegisterAll(registry); err != nil {
		return nil, err
	}
	return func(ctx context.Context, topic string) (string, error) {
		result, err := registry.Execute(ctx, "context7_fetch", map[string]any{"topic": topic})
		if err != nil {
			return "", err
		}
		return result.Result, nil
	}, nil
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
		// Placeholder rows name every topic and teach nothing.
		if knowledge.IsLegacyPlaceholder(atom.Concept, atom.Content) {
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
