package chat

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// =============================================================================
// MULTI-STEP TASK HANDLING
// =============================================================================

// TaskStep represents a single step in a multi-step task
type TaskStep struct {
	Verb      string
	Target    string
	ShardType string
	Task      string
	DependsOn []int // Indices of steps that must complete first
}

// stripQuotedSubstrings removes content inside single, double, or
// backtick quotes so verb-counting and regex pattern matching don't
// pick up nouns the user is using as literal content. Failure mode
// this guards against: 'write a file with the word "test" as its
// content' previously matched two verbs (write + test) and triggered
// multi-step decomposition, splitting a single file-write into
// "create" + "test" sub-steps.
func stripQuotedSubstrings(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inSingle, inDouble, inBacktick := false, false, false
	for _, r := range s {
		switch r {
		case '\'':
			if !inDouble && !inBacktick {
				inSingle = !inSingle
				continue
			}
		case '"':
			if !inSingle && !inBacktick {
				inDouble = !inDouble
				continue
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
				continue
			}
		}
		if !inSingle && !inDouble && !inBacktick {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// multiStepSignals performs the multi-step pattern EXTRACTION in Go (regex,
// keyword, and verb-count matching over the quote-stripped input — the kind of
// fuzzy/NL pattern work the repo guardrails keep OUT of Mangle) and returns the
// set of detected signal atoms. It does NOT short-circuit: every applicable
// signal is collected so the kernel can reason over the full set.
//
// Signals (atom form): /campaign_verb, /keyword_match, /verb_count_high,
// /compound_pattern. The combination DECISION (which signals => multi-step) is
// the kernel's job — see is_multi_step in delegation.mg.
func multiStepSignals(input string, intent perception.Intent) []string {
	// Operate on the quote-stripped form so verbs inside literal
	// content (file content, error messages, quoted examples) don't
	// inflate the verb counter or trigger compound-pattern regexes.
	lower := strings.ToLower(stripQuotedSubstrings(input))

	var signals []string

	// Intent-based heuristic
	if intent.Verb == "/campaign" {
		signals = append(signals, "/campaign_verb")
	}

	// Multi-step indicators — only counted when they appear OUTSIDE
	// quoted content (which is why we use the stripped string).
	//
	// Tuned (routing reliability): the old list included conversational filler
	// ("also", "then ", "first", "1.") that fired on ordinary prose and pasted
	// snippets, decomposing single-step requests. Only explicit sequencing
	// phrases remain; /keyword_match is additionally a WEAK signal that the
	// kernel only honors when corroborated by /verb_count_high (delegation.mg).
	multiStepKeywords := []string{
		"and then", "after that", "and after",
		"step 1", "step 2", "step 3",
		"first,", "second,", "third,", "finally,",
	}
	for _, keyword := range multiStepKeywords {
		if strings.Contains(lower, keyword) {
			signals = append(signals, "/keyword_match")
			break
		}
	}

	// Check for multiple verbs in the input. Threshold of 3 — two
	// verb-synonym hits in a short request (e.g. "write a file" matches both
	// 'write' and 'file' if 'file' is a synonym for a verb in the corpus) was
	// producing false positives on trivially single-step tasks.
	verbCount := 0
	verbHigh := false
	for _, entry := range perception.GetVerbCorpus() {
		for _, synonym := range entry.Synonyms {
			if strings.Contains(lower, synonym) {
				verbCount++
				if verbCount >= 3 {
					verbHigh = true
				}
			}
		}
	}
	if verbHigh {
		signals = append(signals, "/verb_count_high")
	}

	// Check for compound tasks (review + test, fix + test, etc.).
	// All patterns use word-boundary anchors so 'create.*test' only matches
	// when 'create' is actually present as a token, not as a substring inside
	// a longer word. `tests?` covers the plural ("fix it and run the tests").
	compoundPatterns := []string{
		`\breview\b.*\btests?\b`, `\bfix\b.*\btests?\b`, `\brefactor\b.*\btests?\b`,
		`\bcreate\b.*\btests?\b`, `\bimplement\b.*\btests?\b`,
	}
	for _, pattern := range compoundPatterns {
		if matched, _ := regexp.MatchString(pattern, lower); matched {
			signals = append(signals, "/compound_pattern")
			break
		}
	}

	return signals
}

// detectMultiStepTask decides whether input requires multiple steps. Step 5:
// the EXTRACTION stays in Go (multiStepSignals); the DECISION moves to policy.
// Go asserts one multi_step_signal fact per detected signal, then queries
// is_multi_step.
//
// Fail-safe: if the kernel is nil, the assert/query errors, or the kernel
// returns nothing, fall back to a legacy Go boolean that mirrors the policy
// combination (strong signal alone, or weak keyword corroborated by verb
// count). A kernel hiccup can never silently disable multi-step detection.
func (m *Model) detectMultiStepTask(input string, intent perception.Intent) bool {
	signals := multiStepSignals(input, intent)
	legacy := legacyMultiStepDecision(signals)

	if m.kernel == nil {
		return legacy
	}

	// Clear the prior turn's signals so stale matches cannot leak.
	_ = m.kernel.Retract("multi_step_signal")
	for _, sig := range signals {
		if err := m.kernel.Assert(core.Fact{
			Predicate: "multi_step_signal",
			Args:      []any{types.MangleAtom(sig)},
		}); err != nil {
			logging.Routing("[detectMultiStepTask] assert multi_step_signal failed, using legacy gate: %v", err)
			return legacy
		}
	}

	facts, err := m.kernel.Query("is_multi_step")
	if err != nil {
		logging.Routing("[detectMultiStepTask] query is_multi_step failed, using legacy gate: %v", err)
		return legacy
	}
	if len(facts) == 0 {
		// No derivation: the combination rule said no (e.g. a weak keyword
		// signal without corroboration). The legacy boolean mirrors the same
		// combination, so falling back keeps behavior identical while still
		// covering the lost-fact edge case.
		return legacy
	}
	return true
}

// legacyMultiStepDecision mirrors policy/delegation.mg's is_multi_step
// combination for the kernel-unavailable fallback: strong signals decide
// alone, the weak keyword signal needs the verb-count corroboration.
func legacyMultiStepDecision(signals []string) bool {
	has := func(want string) bool {
		return slices.Contains(signals, want)
	}
	if has("/campaign_verb") || has("/compound_pattern") {
		return true
	}
	return has("/keyword_match") && has("/verb_count_high")
}

// decomposeTask breaks a complex task into discrete steps using the encyclopedic corpus.
// This function uses the multi-step pattern corpus for comprehensive decomposition.
func decomposeTask(input string, intent perception.Intent, workspace string) []TaskStep {
	// Try to match against the encyclopedic multi-step corpus first
	pattern, captures := MatchMultiStepPattern(input)

	if pattern != nil {
		// Use the corpus-based decomposition strategy
		steps := DecomposeWithStrategy(input, captures, pattern, workspace)
		if len(steps) > 1 {
			return steps
		}
		// If decomposition returned only 1 step, fall through to legacy handling
	}

	// Legacy fallback patterns for backwards compatibility
	var steps []TaskStep
	lower := strings.ToLower(input)

	// Pattern 1: "fix X and test it" or "create X and test"
	if strings.Contains(lower, "test") && (intent.Verb == "/fix" || intent.Verb == "/create" || intent.Verb == "/refactor") {
		// Step 1: Primary action
		step1 := TaskStep{
			Verb:      intent.Verb,
			Target:    intent.Target,
			ShardType: perception.GetShardTypeForVerb(intent.Verb),
		}
		step1.Task = formatShardTask(step1.Verb, step1.Target, intent.Constraint, workspace)
		steps = append(steps, step1)

		// Step 2: Testing
		step2 := TaskStep{
			Verb:      "/test",
			Target:    intent.Target,
			ShardType: "tester",
			DependsOn: []int{0}, // Depends on step 1
		}
		step2.Task = formatShardTask(step2.Verb, step2.Target, "none", workspace)
		steps = append(steps, step2)

		return steps
	}

	// Pattern 2: "review codebase" or "review all files" - already handled by multi-file discovery
	// Single step with multiple files
	if intent.Verb == "/review" || intent.Verb == "/security" || intent.Verb == "/analyze" {
		step := TaskStep{
			Verb:      intent.Verb,
			Target:    intent.Target,
			ShardType: perception.GetShardTypeForVerb(intent.Verb),
		}
		step.Task = formatShardTask(step.Verb, step.Target, intent.Constraint, workspace)
		steps = append(steps, step)
		return steps
	}

	// Default: single step
	if len(steps) == 0 {
		step := TaskStep{
			Verb:      intent.Verb,
			Target:    intent.Target,
			ShardType: perception.GetShardTypeForVerb(intent.Verb),
		}
		step.Task = formatShardTask(step.Verb, step.Target, intent.Constraint, workspace)
		steps = append(steps, step)
	}

	return steps
}

// discoverFiles finds files in the workspace based on constraint filters
func discoverFiles(workspace, constraint string) []string {
	var files []string

	// Determine file patterns based on constraint
	var extensions []string
	constraintLower := strings.ToLower(constraint)

	switch {
	case strings.Contains(constraintLower, "go"):
		extensions = []string{".go"}
	case strings.Contains(constraintLower, "python") || strings.Contains(constraintLower, "py"):
		extensions = []string{".py"}
	case strings.Contains(constraintLower, "javascript") || strings.Contains(constraintLower, "js"):
		extensions = []string{".js", ".jsx", ".ts", ".tsx"}
	case strings.Contains(constraintLower, "rust"):
		extensions = []string{".rs"}
	case strings.Contains(constraintLower, "java"):
		extensions = []string{".java"}
	default:
		// Default: all common code file extensions
		extensions = []string{".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rs", ".java", ".c", ".cpp", ".h"}
	}

	// Walk workspace and collect matching files
	filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip hidden directories and files
		if strings.Contains(path, "/.") || strings.Contains(path, "\\.") {
			return nil
		}

		// Skip vendor, node_modules, etc.
		skipDirs := []string{"vendor", "node_modules", ".git", ".nerd", "dist", "build"}
		for _, skip := range skipDirs {
			if strings.Contains(path, string(filepath.Separator)+skip+string(filepath.Separator)) {
				return nil
			}
		}

		// Check if file matches extension filter
		ext := filepath.Ext(path)
		if slices.Contains(extensions, ext) {
			// Convert to relative path
			if relPath, err := filepath.Rel(workspace, path); err == nil {
				files = append(files, relPath)
			}
		}

		return nil
	})

	// Limit to 50 files for safety (avoid overwhelming the shard)
	if len(files) > 50 {
		files = files[:50]
	}

	return files
}
