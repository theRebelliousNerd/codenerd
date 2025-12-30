// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains decomposition strategies for breaking multi-step patterns
// into discrete, executable TaskSteps.
package chat

import (
	"fmt"
	"regexp"
	"strings"

	"codenerd/internal/perception"
)

// =============================================================================
// DECOMPOSITION STRATEGIES
// =============================================================================
// Each strategy corresponds to a Decomposer type in MultiStepPattern.
// They take captured groups and convert them to TaskSteps.

// Decomposer is a function that breaks captured pattern groups into steps
type Decomposer func(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep

// DecomposerRegistry maps decomposer names to their implementations
var DecomposerRegistry = map[string]Decomposer{
	"explicit_sequence":     decomposeExplicitSequence,
	"numbered_steps":        decomposeNumberedSteps,
	"verb_pair_chain":       decomposeVerbPairChain,
	"conditional_chain":     decomposeConditionalChain,
	"fallback_chain":        decomposeFallbackChain,
	"parallel_split":        decomposeParallelSplit,
	"pronoun_resolution":    decomposePronounResolution,
	"iterative_expansion":   decomposeIterativeExpansion,
	"batch_expansion":       decomposeBatchExpansion,
	"pipeline_chain":        decomposePipelineChain,
	"comparison_chain":      decomposeComparisonChain,
	"constrained_operation": decomposeConstrainedOperation,
	"git_workflow":          decomposeGitWorkflow,
}

// DecomposeWithStrategy uses the registered strategy to decompose input
func DecomposeWithStrategy(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	if pattern == nil {
		return nil
	}

	decomposer, ok := DecomposerRegistry[pattern.Decomposer]
	if !ok {
		// Fallback to generic decomposition
		return decomposeGeneric(input, captures, pattern, workspace)
	}

	steps := decomposer(input, captures, pattern, workspace)

	// Apply dependency inference based on pattern relation
	return InferDependencies(pattern, steps)
}

// =============================================================================
// EXPLICIT SEQUENCE DECOMPOSER
// =============================================================================
// Handles: "first X, then Y, finally Z"

func decomposeExplicitSequence(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	for i, capture := range captures {
		if capture == "" {
			continue
		}

		// Parse each capture as an independent task
		verb, target := parseVerbAndTarget(strings.TrimSpace(capture))

		step := TaskStep{
			Verb:      verb,
			Target:    target,
			ShardType: perception.GetShardTypeForVerb(verb),
		}
		step.Task = formatShardTask(step.Verb, step.Target, "none", workspace)

		// Each step depends on the previous
		if i > 0 {
			step.DependsOn = []int{i - 1}
		}

		steps = append(steps, step)
	}

	return steps
}

// =============================================================================
// NUMBERED STEPS DECOMPOSER
// =============================================================================
// Handles: "1. do X 2. do Y 3. do Z"

func decomposeNumberedSteps(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	// If captures are empty, try to parse numbered steps from input directly
	if len(captures) == 0 || (len(captures) == 1 && captures[0] == "") {
		// Parse numbered list format: "1. foo 2. bar 3. baz"
		stepRegex := regexp.MustCompile(`(?i)(?:step\s*)?(\d+)[.):]\s*([^0-9]+?)(?=(?:step\s*)?\d+[.):]|$)`)
		matches := stepRegex.FindAllStringSubmatch(input, -1)

		for i, match := range matches {
			if len(match) >= 3 {
				capture := strings.TrimSpace(match[2])
				verb, target := parseVerbAndTarget(capture)

				step := TaskStep{
					Verb:      verb,
					Target:    target,
					ShardType: perception.GetShardTypeForVerb(verb),
				}
				step.Task = formatShardTask(step.Verb, step.Target, "none", workspace)

				if i > 0 {
					step.DependsOn = []int{i - 1}
				}
				steps = append(steps, step)
			}
		}
		return steps
	}

	// Use captures if available
	return decomposeExplicitSequence(input, captures, pattern, workspace)
}

// =============================================================================
// VERB PAIR CHAIN DECOMPOSER
// =============================================================================
// Handles: "review X and fix", "create Y and test"

func decomposeVerbPairChain(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	// Get verb pairs for this pattern
	verbPairs := pattern.VerbPairs
	if len(verbPairs) == 0 {
		// Fallback to inferring from input
		return decomposeGeneric(input, captures, pattern, workspace)
	}

	// Determine target from captures
	target := "codebase"
	if len(captures) > 0 && captures[0] != "" {
		target = strings.TrimSpace(captures[0])
	}

	// Create steps for the first matching verb pair
	pair := verbPairs[0]

	// Step 1: First verb
	step1 := TaskStep{
		Verb:      pair[0],
		Target:    target,
		ShardType: perception.GetShardTypeForVerb(pair[0]),
	}
	step1.Task = formatShardTask(step1.Verb, step1.Target, "none", workspace)
	steps = append(steps, step1)

	// Step 2: Second verb (depends on step 1)
	step2 := TaskStep{
		Verb:      pair[1],
		Target:    target,
		ShardType: perception.GetShardTypeForVerb(pair[1]),
		DependsOn: []int{0},
	}
	step2.Task = formatShardTaskWithContext(step2.Verb, step2.Target, "none", workspace, nil)
	steps = append(steps, step2)

	return steps
}

// =============================================================================
// CONDITIONAL CHAIN DECOMPOSER
// =============================================================================
// Handles: "X, if successful, Y"

func decomposeConditionalChain(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	if len(captures) < 2 {
		return decomposeGeneric(input, captures, pattern, workspace)
	}

	// Step 1: The condition action
	verb1, target1 := parseVerbAndTarget(strings.TrimSpace(captures[0]))
	step1 := TaskStep{
		Verb:      verb1,
		Target:    target1,
		ShardType: perception.GetShardTypeForVerb(verb1),
	}
	step1.Task = formatShardTask(step1.Verb, step1.Target, "none", workspace)
	steps = append(steps, step1)

	// Step 2: The action on success (marked as conditional)
	verb2, target2 := parseVerbAndTarget(strings.TrimSpace(captures[1]))
	if target2 == "" || target2 == "none" {
		target2 = target1 // Inherit target from first step
	}
	step2 := TaskStep{
		Verb:      verb2,
		Target:    target2,
		ShardType: perception.GetShardTypeForVerb(verb2),
		DependsOn: []int{0},
	}
	step2.Task = formatShardTask(step2.Verb, step2.Target, "none", workspace)
	steps = append(steps, step2)

	return steps
}

// =============================================================================
// FALLBACK CHAIN DECOMPOSER
// =============================================================================
// Handles: "try X, if fails, Y"

func decomposeFallbackChain(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	// Structure is similar to conditional, but execution logic differs
	// The second step runs only on failure (handled in executeMultiStepTask)
	return decomposeConditionalChain(input, captures, pattern, workspace)
}

// =============================================================================
// PARALLEL SPLIT DECOMPOSER
// =============================================================================
// Handles: "review X and review Y", "do A also do B"

func decomposeParallelSplit(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	// For parallel patterns, create independent steps (no DependsOn)
	for _, capture := range captures {
		if capture == "" {
			continue
		}

		verb, target := parseVerbAndTarget(strings.TrimSpace(capture))

		step := TaskStep{
			Verb:      verb,
			Target:    target,
			ShardType: perception.GetShardTypeForVerb(verb),
			DependsOn: nil, // Explicitly parallel - no dependencies
		}
		step.Task = formatShardTask(step.Verb, step.Target, "none", workspace)
		steps = append(steps, step)
	}

	// If we only got one step from captures, try to split the input differently
	if len(steps) < 2 {
		// Try splitting on common parallel connectors
		parts := splitOnParallelConnectors(input)
		steps = nil
		for _, part := range parts {
			verb, target := parseVerbAndTarget(strings.TrimSpace(part))
			if verb == "" {
				continue
			}
			step := TaskStep{
				Verb:      verb,
				Target:    target,
				ShardType: perception.GetShardTypeForVerb(verb),
				DependsOn: nil,
			}
			step.Task = formatShardTask(step.Verb, step.Target, "none", workspace)
			steps = append(steps, step)
		}
	}

	return steps
}

// =============================================================================
// PRONOUN RESOLUTION DECOMPOSER
// =============================================================================
// Handles: "create X and test it", "fix Y and commit it"

func decomposePronounResolution(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	if len(captures) < 3 {
		return decomposeGeneric(input, captures, pattern, workspace)
	}

	// captures[0] = first verb (e.g., "create")
	// captures[1] = target (e.g., "the handler")
	// captures[2] = second verb (e.g., "test")

	verb1 := "/" + strings.ToLower(captures[0])
	target := captures[1]
	verb2 := "/" + strings.ToLower(captures[2])

	// Normalize verbs
	verb1 = normalizeVerb(verb1)
	verb2 = normalizeVerb(verb2)

	// Step 1: First action
	step1 := TaskStep{
		Verb:      verb1,
		Target:    target,
		ShardType: perception.GetShardTypeForVerb(verb1),
	}
	step1.Task = formatShardTask(step1.Verb, step1.Target, "none", workspace)
	steps = append(steps, step1)

	// Step 2: Second action on same target ("it" resolved to target)
	step2 := TaskStep{
		Verb:      verb2,
		Target:    target, // "it" resolved
		ShardType: perception.GetShardTypeForVerb(verb2),
		DependsOn: []int{0},
	}
	step2.Task = formatShardTask(step2.Verb, step2.Target, "none", workspace)
	steps = append(steps, step2)

	return steps
}

// =============================================================================
// ITERATIVE EXPANSION DECOMPOSER
// =============================================================================
// Handles: "review each handler", "fix every failing test"

func decomposeIterativeExpansion(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	// For iterative patterns, we create a single step with iteration metadata
	// The execution layer will expand this into multiple actual executions

	verb := "/review" // Default
	target := "codebase"
	scope := ""

	if len(captures) >= 1 && captures[0] != "" {
		verb = "/" + strings.ToLower(captures[0])
		verb = normalizeVerb(verb)
	}
	if len(captures) >= 2 && captures[1] != "" {
		target = captures[1]
	}
	if len(captures) >= 3 && captures[2] != "" {
		scope = captures[2]
	}

	// Create a single step that will be expanded during execution
	step := TaskStep{
		Verb:      verb,
		Target:    fmt.Sprintf("each %s", target),
		ShardType: perception.GetShardTypeForVerb(verb),
	}

	if scope != "" {
		step.Task = fmt.Sprintf("%s each %s in %s", verb, target, scope)
	} else {
		step.Task = fmt.Sprintf("%s each %s", verb, target)
	}

	steps = append(steps, step)

	return steps
}

// =============================================================================
// BATCH EXPANSION DECOMPOSER
// =============================================================================
// Handles: "format all go files", "lint entire codebase"

func decomposeBatchExpansion(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	verb := "/review"
	if len(captures) >= 1 && captures[0] != "" {
		verb = "/" + strings.ToLower(captures[0])
		verb = normalizeVerb(verb)
	}

	// Single step with "all" target - file discovery happens at execution time
	step := TaskStep{
		Verb:      verb,
		Target:    "codebase",
		ShardType: perception.GetShardTypeForVerb(verb),
	}
	step.Task = formatShardTask(step.Verb, step.Target, "none", workspace)
	steps = append(steps, step)

	return steps
}

// =============================================================================
// PIPELINE CHAIN DECOMPOSER
// =============================================================================
// Handles: "analyze and pass results to optimizer"

func decomposePipelineChain(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	// Pipeline is a sequential chain with explicit output passing
	// The execution layer will inject prior results into subsequent steps
	return decomposeExplicitSequence(input, captures, pattern, workspace)
}

// =============================================================================
// COMPARISON CHAIN DECOMPOSER
// =============================================================================
// Handles: "compare X and Y, pick best"

func decomposeComparisonChain(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	if len(captures) < 2 {
		return decomposeGeneric(input, captures, pattern, workspace)
	}

	// Step 1: Analyze first option
	step1 := TaskStep{
		Verb:      "/analyze",
		Target:    strings.TrimSpace(captures[0]),
		ShardType: perception.GetShardTypeForVerb("/analyze"),
	}
	step1.Task = formatShardTask(step1.Verb, step1.Target, "none", workspace)
	steps = append(steps, step1)

	// Step 2: Analyze second option (parallel with step 1)
	step2 := TaskStep{
		Verb:      "/analyze",
		Target:    strings.TrimSpace(captures[1]),
		ShardType: perception.GetShardTypeForVerb("/analyze"),
		DependsOn: nil, // Can run in parallel with step 1
	}
	step2.Task = formatShardTask(step2.Verb, step2.Target, "none", workspace)
	steps = append(steps, step2)

	// Step 3: Compare and recommend (depends on both analyses)
	step3 := TaskStep{
		Verb:      "/explain",
		Target:    fmt.Sprintf("comparison of %s vs %s", captures[0], captures[1]),
		ShardType: "",
		DependsOn: []int{0, 1}, // Depends on both analyses
	}
	step3.Task = fmt.Sprintf("compare and recommend: %s vs %s", captures[0], captures[1])
	steps = append(steps, step3)

	return steps
}

// =============================================================================
// CONSTRAINED OPERATION DECOMPOSER
// =============================================================================
// Handles: "refactor X but not Y", "review X while keeping Y"

func decomposeConstrainedOperation(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	if len(captures) < 3 {
		return decomposeGeneric(input, captures, pattern, workspace)
	}

	verb := "/" + strings.ToLower(captures[0])
	verb = normalizeVerb(verb)
	target := strings.TrimSpace(captures[1])
	constraint := strings.TrimSpace(captures[2])

	step := TaskStep{
		Verb:      verb,
		Target:    target,
		ShardType: perception.GetShardTypeForVerb(verb),
	}
	step.Task = formatShardTask(step.Verb, step.Target, "exclude:"+constraint, workspace)
	steps = append(steps, step)

	return steps
}

// =============================================================================
// GIT WORKFLOW DECOMPOSER
// =============================================================================
// Handles: "commit and push", "add, commit, and push"

func decomposeGitWorkflow(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep
	lower := strings.ToLower(input)

	// Detect which git operations are requested
	hasAdd := strings.Contains(lower, "add") || strings.Contains(lower, "stage")
	hasCommit := strings.Contains(lower, "commit") || strings.Contains(lower, "save")
	hasPush := strings.Contains(lower, "push")
	hasPull := strings.Contains(lower, "pull")
	hasCheckout := strings.Contains(lower, "checkout") || strings.Contains(lower, "switch")

	idx := 0

	if hasCheckout {
		step := TaskStep{
			Verb:      "/git",
			Target:    "checkout",
			ShardType: "",
		}
		step.Task = "git checkout"
		steps = append(steps, step)
		idx++
	}

	if hasPull {
		step := TaskStep{
			Verb:      "/git",
			Target:    "pull",
			ShardType: "",
		}
		if idx > 0 {
			step.DependsOn = []int{idx - 1}
		}
		step.Task = "git pull"
		steps = append(steps, step)
		idx++
	}

	if hasAdd {
		step := TaskStep{
			Verb:      "/git",
			Target:    "add",
			ShardType: "",
		}
		if idx > 0 {
			step.DependsOn = []int{idx - 1}
		}
		step.Task = "git add"
		steps = append(steps, step)
		idx++
	}

	if hasCommit {
		step := TaskStep{
			Verb:      "/git",
			Target:    "commit",
			ShardType: "",
		}
		if idx > 0 {
			step.DependsOn = []int{idx - 1}
		}
		step.Task = "git commit"
		steps = append(steps, step)
		idx++
	}

	if hasPush {
		step := TaskStep{
			Verb:      "/git",
			Target:    "push",
			ShardType: "",
		}
		if idx > 0 {
			step.DependsOn = []int{idx - 1}
		}
		step.Task = "git push"
		steps = append(steps, step)
	}

	return steps
}

// =============================================================================
// GENERIC DECOMPOSER (FALLBACK)
// =============================================================================

func decomposeGeneric(input string, captures []string, pattern *MultiStepPattern, workspace string) []TaskStep {
	var steps []TaskStep

	// Try to extract verb-target pairs from the input
	parts := splitOnSequentialConnectors(input)

	for i, part := range parts {
		verb, target := parseVerbAndTarget(strings.TrimSpace(part))
		if verb == "" {
			continue
		}

		step := TaskStep{
			Verb:      verb,
			Target:    target,
			ShardType: perception.GetShardTypeForVerb(verb),
		}
		step.Task = formatShardTask(step.Verb, step.Target, "none", workspace)

		if i > 0 {
			step.DependsOn = []int{i - 1}
		}
		steps = append(steps, step)
	}

	return steps
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// parseVerbAndTarget extracts verb and target from a task description
func parseVerbAndTarget(text string) (verb string, target string) {
	lower := strings.ToLower(text)

	// Try to match against known verbs from corpus
	for _, entry := range perception.VerbCorpus {
		for _, synonym := range entry.Synonyms {
			if strings.HasPrefix(lower, synonym+" ") || strings.HasPrefix(lower, synonym+":") {
				verb = entry.Verb
				target = strings.TrimSpace(text[len(synonym):])
				target = strings.TrimPrefix(target, ":")
				target = strings.TrimSpace(target)
				if target == "" {
					target = "codebase"
				}
				return
			}
			// Check for synonym anywhere in the text
			if strings.Contains(lower, synonym) {
				verb = entry.Verb
				// Try to extract target after the verb
				idx := strings.Index(lower, synonym)
				if idx >= 0 {
					remaining := text[idx+len(synonym):]
					target = strings.TrimSpace(remaining)
					// Clean up target
					target = strings.TrimPrefix(target, "the ")
					target = strings.TrimPrefix(target, "a ")
					target = strings.TrimPrefix(target, "an ")
					if target == "" {
						target = "codebase"
					}
				}
				return
			}
		}
	}

	// Fallback: try to parse as "verb target" directly
	words := strings.Fields(text)
	if len(words) >= 1 {
		potentialVerb := "/" + strings.ToLower(words[0])
		potentialVerb = normalizeVerb(potentialVerb)
		if potentialVerb != "" && potentialVerb != "/" {
			verb = potentialVerb
			if len(words) > 1 {
				target = strings.Join(words[1:], " ")
			} else {
				target = "codebase"
			}
			return
		}
	}

	// Ultimate fallback
	verb = "/explain"
	target = text
	return
}

// normalizeVerb converts common verb forms to canonical verbs
func normalizeVerb(verb string) string {
	normalization := map[string]string{
		"/reviewing":    "/review",
		"/reviewed":     "/review",
		"/fixing":       "/fix",
		"/fixed":        "/fix",
		"/testing":      "/test",
		"/tested":       "/test",
		"/creating":     "/create",
		"/created":      "/create",
		"/refactoring":  "/refactor",
		"/refactored":   "/refactor",
		"/analyzing":    "/analyze",
		"/analyzed":     "/analyze",
		"/committing":   "/git",
		"/committed":    "/git",
		"/pushing":      "/git",
		"/pushed":       "/git",
		"/implement":    "/create",
		"/implementing": "/create",
		"/implemented":  "/create",
		"/add":          "/create",
		"/adding":       "/create",
		"/added":        "/create",
		"/check":        "/review",
		"/checking":     "/review",
		"/checked":      "/review",
		"/scan":         "/security",
		"/scanning":     "/security",
		"/scanned":      "/security",
		"/audit":        "/security",
		"/auditing":     "/security",
		"/audited":      "/security",
		"/write":        "/create",
		"/writing":      "/create",
		"/build":        "/create",
		"/building":     "/create",
		"/run":          "/test",
		"/running":      "/test",
	}

	if normalized, ok := normalization[verb]; ok {
		return normalized
	}

	// Check if it's a known verb
	for _, entry := range perception.VerbCorpus {
		if entry.Verb == verb {
			return verb
		}
	}

	// Return as-is if not found (might still be valid)
	return verb
}

// splitOnSequentialConnectors splits input on sequential connectors
func splitOnSequentialConnectors(input string) []string {
	// Order matters - check longer patterns first
	connectors := []string{
		" and then ",
		" after that ",
		" afterwards ",
		" then ",
		" next ",
		" finally ",
		" subsequently ",
		", then ",
		", and ",
		" and ",
	}

	for _, conn := range connectors {
		if strings.Contains(strings.ToLower(input), conn) {
			parts := strings.Split(strings.ToLower(input), conn)
			// Return non-empty parts
			var result []string
			for _, p := range parts {
				if strings.TrimSpace(p) != "" {
					result = append(result, strings.TrimSpace(p))
				}
			}
			if len(result) > 1 {
				return result
			}
		}
	}

	return []string{input}
}

// splitOnParallelConnectors splits input on parallel connectors
func splitOnParallelConnectors(input string) []string {
	connectors := []string{
		" and also ",
		" also ",
		" additionally ",
		" at the same time ",
		" simultaneously ",
		" in parallel ",
		" as well as ",
	}

	lower := strings.ToLower(input)
	for _, conn := range connectors {
		if strings.Contains(lower, conn) {
			parts := strings.Split(lower, conn)
			var result []string
			for _, p := range parts {
				if strings.TrimSpace(p) != "" {
					result = append(result, strings.TrimSpace(p))
				}
			}
			if len(result) > 1 {
				return result
			}
		}
	}

	// Fallback to splitting on " and " only if no sequential keywords present
	if !strings.Contains(lower, " then ") && !strings.Contains(lower, "after") {
		parts := strings.Split(lower, " and ")
		var result []string
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				result = append(result, strings.TrimSpace(p))
			}
		}
		if len(result) > 1 {
			return result
		}
	}

	return []string{input}
}
