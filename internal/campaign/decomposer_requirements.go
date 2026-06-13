package campaign

import (
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// extractRequirements uses LLM to extract requirements from source content.
func (d *Decomposer) extractRequirements(ctx context.Context, campaignID string, content map[string]string) ([]Requirement, error) {
	if len(content) == 0 {
		return nil, nil
	}

	if d.llmClient == nil {
		return nil, nil
	}

	reqs := make([]Requirement, 0)
	seen := make(map[string]bool)
	reqCounter := 0

	for path, text := range content {
		chunks := chunkText(text, 6000)
		if len(chunks) == 0 {
			continue
		}

		for idx, chunk := range chunks {
			prompt := fmt.Sprintf(`%s

Document: %s
Chunk: %d of %d
Content:
%s
`, ExtractorLogic, path, idx+1, len(chunks), chunk)

			resp, err := d.llmClient.Complete(ctx, prompt)
			if err != nil {
				continue
			}

			resp = cleanJSONResponse(resp)
			var parsed struct {
				Requirements []struct {
					ID          string `json:"id"`
					Description string `json:"description"`
					Priority    string `json:"priority"`
					Source      string `json:"source"`
				} `json:"requirements"`
			}

			if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
				continue
			}

			for _, r := range parsed.Requirements {
				reqCounter++
				id := fmt.Sprintf("/req_%s_%04d", sanitizeCampaignID(campaignID), reqCounter)
				key := fmt.Sprintf("%s|%s", path, r.Description)
				if seen[key] {
					continue
				}
				seen[key] = true
				reqs = append(reqs, Requirement{
					ID:          id,
					CampaignID:  campaignID,
					Description: r.Description,
					Priority:    defaultPriority(r.Priority),
					Source:      path,
				})
			}
		}
	}

	return reqs, nil
}

// extractRequirementsSmart performs retrieval-augmented requirement extraction using the vector store.
func (d *Decomposer) extractRequirementsSmart(ctx context.Context, campaignID, goal, kbPath string, files []FileMetadata) ([]Requirement, error) {
	if d.llmClient == nil {
		logging.CampaignDebug("No LLM client, skipping requirement extraction")
		return nil, nil
	}

	questions := d.generateDiscoveryQuestions(goal)
	if len(questions) == 0 {
		logging.CampaignDebug("No discovery questions generated")
		return nil, nil
	}
	logging.CampaignDebug("Generated %d discovery questions", len(questions))

	kbStore, err := store.NewLocalStore(kbPath)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to open knowledge store: %v", err)
		return nil, fmt.Errorf("failed to open knowledge store: %w", err)
	}
	defer kbStore.Close()

	reqs := make([]Requirement, 0)
	seen := make(map[string]bool)
	reqCounter := 0
	allowedPaths := d.relevantPathsFromKernel()
	if len(allowedPaths) == 0 {
		allowedPaths = pathsForGoal(goal, files)
	}
	logging.CampaignDebug("Using %d allowed paths for vector recall", len(allowedPaths))

	for i, q := range questions {
		logging.CampaignDebug("Processing question %d/%d: %s", i+1, len(questions), q[:min(80, len(q))])

		var entries []store.VectorEntry
		var err error
		if len(allowedPaths) > 0 {
			entries, err = kbStore.VectorRecallSemanticByPaths(ctx, q, 6, allowedPaths)
		} else {
			entries, err = kbStore.VectorRecallSemanticFiltered(ctx, q, 6, "campaign_id", campaignID)
		}
		if err != nil {
			logging.CampaignDebug("Vector recall failed: %v", err)
			continue
		}
		if len(entries) == 0 {
			logging.CampaignDebug("No vector entries found for question")
			continue
		}
		logging.CampaignDebug("Retrieved %d vector entries", len(entries))

		var sb strings.Builder
		for _, e := range entries {
			path := ""
			if p, ok := e.Metadata["path"].(string); ok {
				path = p
			}
			sb.WriteString(fmt.Sprintf("PATH: %s\n", path))
			sb.WriteString(e.Content)
			sb.WriteString("\n---\n")
		}

		prompt := fmt.Sprintf(`Goal: %s
Question: %s
Given the retrieved snippets, extract discrete requirements as JSON:
{
  "requirements": [
    {"description": "...", "priority": "/critical|/high|/normal|/low", "source": "path"}
  ]
}

Snippets:
%s
Return JSON only.`, goal, q, sb.String())

		// Use grounding for research-intensive requirement extraction
		resp, err := d.completeWithGrounding(ctx, prompt)
		if err != nil {
			logging.CampaignDebug("LLM extraction failed: %v", err)
			continue
		}

		resp = cleanJSONResponse(resp)
		var parsed struct {
			Requirements []struct {
				Description string `json:"description"`
				Priority    string `json:"priority"`
				Source      string `json:"source"`
			} `json:"requirements"`
		}
		if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
			logging.CampaignDebug("Failed to parse requirements JSON: %v", err)
			continue
		}

		extractedCount := 0
		for _, r := range parsed.Requirements {
			key := fmt.Sprintf("%s|%s", r.Source, r.Description)
			if seen[key] {
				continue
			}
			reqCounter++
			id := fmt.Sprintf("/req_%s_%04d", sanitizeCampaignID(campaignID), reqCounter)
			seen[key] = true
			reqs = append(reqs, Requirement{
				ID:          id,
				CampaignID:  campaignID,
				Description: r.Description,
				Priority:    defaultPriority(r.Priority),
				Source:      r.Source,
			})
			extractedCount++
		}
		logging.CampaignDebug("Extracted %d new requirements from question %d", extractedCount, i+1)
	}

	logging.Campaign("Total requirements extracted: %d", len(reqs))
	return reqs, nil
}

// relevantPathsFromKernel reads Mangle-derived relevance decisions.
func (d *Decomposer) relevantPathsFromKernel() []string {
	if d.kernel == nil {
		return nil
	}

	facts, err := d.kernel.Query("is_relevant")
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	paths := make([]string, 0, len(facts))
	for _, fact := range facts {
		if len(fact.Args) == 0 {
			continue
		}
		path, ok := fact.Args[0].(string)
		if !ok {
			path = types.ExtractString(fact.Args[0])
		}
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	return paths
}

// pathsForGoal derives candidate file paths whose tags align with the goal keywords.
func pathsForGoal(goal string, files []FileMetadata) []string {
	if len(files) == 0 {
		return nil
	}
	goal = strings.ToLower(goal)
	tokens := strings.FieldsFunc(goal, func(r rune) bool {
		return r == ' ' || r == '/' || r == '-' || r == '_'
	})
	tokenSet := make(map[string]struct{})
	for _, t := range tokens {
		if len(t) < 3 {
			continue
		}
		tokenSet[t] = struct{}{}
	}

	paths := make([]string, 0)
	for _, f := range files {
		match := false
		for _, tag := range f.Tags {
			if len(tag) < 3 {
				continue
			}
			if _, ok := tokenSet[tag]; ok {
				match = true
				break
			}
		}
		if match {
			paths = append(paths, f.Path)
		}
	}
	return paths
}

// topologyContextSummary builds a concise summary of the planner's doc-driven topology hints.
func (d *Decomposer) topologyContextSummary() string {
	if d.kernel == nil {
		return ""
	}

	var sb strings.Builder

	// Proposed phases (active layers)
	phaseSet := make(map[string]struct{})
	if facts, err := d.kernel.Query("proposed_phase"); err == nil {
		for _, fact := range facts {
			if len(fact.Args) == 0 {
				continue
			}
			phase := types.ExtractString(fact.Args[0])
			if phase != "" {
				phaseSet[phase] = struct{}{}
			}
		}
	}
	if len(phaseSet) > 0 {
		phases := make([]string, 0, len(phaseSet))
		for p := range phaseSet {
			phases = append(phases, p)
		}
		sort.Strings(phases)
		sb.WriteString("- Proposed phases: ")
		sb.WriteString(strings.Join(phases, ", "))
		sb.WriteString("\n")
	}

	// Dependencies between layers
	deps := make([]string, 0)
	if facts, err := d.kernel.Query("phase_dependency_generated"); err == nil {
		for _, fact := range facts {
			if len(fact.Args) < 2 {
				continue
			}
			deps = append(deps, fmt.Sprintf("%v -> %v", fact.Args[0], fact.Args[1]))
		}
	}
	if len(deps) > 0 {
		sort.Strings(deps)
		sb.WriteString("- Generated ordering:\n")
		for i, dep := range deps {
			if i >= 6 {
				break
			}
			sb.WriteString("  * ")
			sb.WriteString(dep)
			sb.WriteString("\n")
		}
	}

	// Context scope per phase (sample)
	scope := make(map[string][]string)
	if facts, err := d.kernel.Query("phase_context_scope"); err == nil {
		for _, fact := range facts {
			if len(fact.Args) < 2 {
				continue
			}
			phase := types.ExtractString(fact.Args[0])
			doc := types.ExtractString(fact.Args[1])
			if phase == "" || doc == "" {
				continue
			}
			if len(scope[phase]) < 3 {
				scope[phase] = append(scope[phase], doc)
			}
		}
	}
	if len(scope) > 0 {
		keys := make([]string, 0, len(scope))
		for k := range scope {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) > 3 {
			keys = keys[:3]
		}
		sb.WriteString("- Context scope (sample):\n")
		for _, phase := range keys {
			sb.WriteString("  * ")
			sb.WriteString(phase)
			sb.WriteString(": ")
			sb.WriteString(strings.Join(scope[phase], ", "))
			sb.WriteString("\n")
		}
	}

	// Conflicts (if any)
	conflicts := make([]string, 0, 3)
	if facts, err := d.kernel.Query("doc_conflict"); err == nil {
		for _, fact := range facts {
			if len(conflicts) >= 3 {
				break
			}
			if len(fact.Args) < 3 {
				continue
			}
			conflicts = append(conflicts, fmt.Sprintf("%v crosses %v vs %v", fact.Args[0], fact.Args[1], fact.Args[2]))
		}
	}
	if len(conflicts) > 0 {
		sb.WriteString("- Potentially broad docs:\n")
		for _, c := range conflicts {
			sb.WriteString("  * ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

// generateDiscoveryQuestions creates targeted retrieval questions from the goal.
func (d *Decomposer) generateDiscoveryQuestions(goal string) []string {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return nil
	}

	base := []string{
		"What are the functional requirements?",
		"What are the security and compliance requirements?",
		"What integration or API contracts are required?",
		"What UI/UX or branding constraints exist?",
	}

	questions := make([]string, 0, len(base)+2)
	for _, q := range base {
		questions = append(questions, fmt.Sprintf("%s (Goal: %s)", q, goal))
	}

	// Add a targeted ask using the goal keyword directly.
	questions = append(questions,
		fmt.Sprintf("Key specifications related to: %s", goal),
		fmt.Sprintf("Edge cases and non-functional requirements for: %s", goal),
	)

	return questions
}

var goalTopicRegex = regexp.MustCompile(`[a-z0-9]+`)

// extractTopicsFromGoal tokenizes a goal into lowercase topics for Mangle selection.
func extractTopicsFromGoal(goal string) []string {
	goal = strings.ToLower(goal)
	if goal == "" {
		return nil
	}

	matches := goalTopicRegex.FindAllString(goal, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	topics := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		topics = append(topics, m)
	}

	return topics
}

var rePrefix = regexp.MustCompile(`^\d+[-_]?`)

// deriveTagsFromPath converts structured folder/file names into tag tokens.
func deriveTagsFromPath(path string) []string {
	clean := strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(clean, "/")
	tags := make(map[string]struct{})

	for _, p := range parts {
		base := strings.ToLower(strings.TrimSuffix(p, filepath.Ext(p)))
		base = rePrefix.ReplaceAllString(base, "")
		if base == "" {
			continue
		}
		tags[base] = struct{}{}
		for seg := range strings.SplitSeq(base, "-") {
			if seg != "" {
				tags[seg] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(tags))
	for t := range tags {
		out = append(out, t)
	}
	return out
}

// RawPlan represents the LLM's proposed plan structure.
type RawPlan struct {
	Title      string     `json:"title"`
	Confidence float64    `json:"confidence"`
	Phases     []RawPhase `json:"phases"`
}

// RawPhase represents a proposed phase.
type RawPhase struct {
	Name               string    `json:"name"`
	Order              int       `json:"order"`
	Description        string    `json:"description"`
	Category           string    `json:"category"`
	ObjectiveType      string    `json:"objective_type"`
	VerificationMethod string    `json:"verification_method"`
	Complexity         string    `json:"complexity"`
	DependsOn          []int     `json:"depends_on"` // Indices of dependent phases
	Tasks              []RawTask `json:"tasks"`
	FocusPatterns      []string  `json:"focus_patterns"`
	RequiredTools      []string  `json:"required_tools"`
}

// RawTask represents a proposed task.
type RawTask struct {
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	Order       int      `json:"order,omitzero"`
	DependsOn   []int    `json:"depends_on"` // Indices of dependent tasks in same phase
	Artifacts   []string `json:"artifacts"`
	WriteSet    []string `json:"write_set,omitzero"`

	// Shard routing (optional - enables explicit shard selection)
	Shard       string `json:"shard,omitzero"`        // Which shard to use (e.g., "coder", "researcher")
	ShardInput  string `json:"shard_input,omitzero"`  // Full input to pass to shard
	ContextFrom []int  `json:"context_from,omitzero"` // Task indices to pull results from for context
}

// planResponseSchema enforces RawPlan structure for schema-capable LLM clients.
// Keep this aligned with RawPlan/RawPhase/RawTask fields.
const planResponseSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": true,
  "required": ["title", "confidence", "phases"],
  "properties": {
    "title": { "type": "string", "minLength": 1 },
    "confidence": { "type": "number", "minimum": 0, "maximum": 1 },
    "phases": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": true,
        "required": [
          "name",
          "order",
          "category",
          "description",
          "objective_type",
          "verification_method",
          "complexity",
          "depends_on",
          "focus_patterns",
          "required_tools",
          "tasks"
        ],
        "properties": {
          "name": { "type": "string", "minLength": 1 },
          "order": { "type": "integer", "minimum": 0 },
          "category": {
            "type": "string",
            "enum": ["/scaffold", "/domain_core", "/data_layer", "/service", "/transport", "/integration", "/research", "/test", "/ops"]
          },
          "description": { "type": "string", "minLength": 1 },
          "objective_type": {
            "type": "string",
            "enum": ["/create", "/modify", "/test", "/research", "/validate", "/integrate", "/review"]
          },
          "verification_method": {
            "type": "string",
            "enum": ["/tests_pass", "/builds", "/manual_review", "/shard_validation", "/none", "/nemesis_gauntlet"]
          },
          "complexity": { "type": "string", "enum": ["/low", "/medium", "/high", "/critical"] },
          "depends_on": { "type": "array", "items": { "type": "integer", "minimum": 0 } },
          "focus_patterns": { "type": "array", "items": { "type": "string" } },
          "required_tools": { "type": "array", "items": { "type": "string" } },
          "tasks": {
            "type": "array",
            "items": {
              "type": "object",
              "additionalProperties": true,
              "required": ["description", "type", "priority", "depends_on", "artifacts"],
              "properties": {
                "description": { "type": "string", "minLength": 1 },
                "type": {
                  "type": "string",
                  "enum": ["/file_create", "/file_modify", "/test_write", "/test_run", "/research", "/verify", "/document", "/campaign_ref", "/tool_create"]
                },
                "priority": { "type": "string", "enum": ["/critical", "/high", "/normal", "/low"] },
                "order": { "type": "integer", "minimum": 0 },
                "depends_on": { "type": "array", "items": { "type": "integer", "minimum": 0 } },
                "artifacts": { "type": "array", "items": { "type": "string" } },
                "write_set": { "type": "array", "items": { "type": "string" } },
                "shard": { "type": "string" },
                "shard_input": { "type": "string" },
                "context_from": { "type": "array", "items": { "type": "integer", "minimum": 0 } }
              }
            }
          }
        }
      }
    }
  }
}`
