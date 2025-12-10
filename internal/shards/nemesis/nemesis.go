// Package nemesis implements the Nemesis shard - a system-level adversarial agent
// that analyzes patches and generates targeted chaos tools to expose weaknesses.
package nemesis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
)

// nemesisIdentityAtomPath is the path to the Nemesis identity atom in the JIT prompt system.
// The actual identity is loaded via PromptAssembler from internal/prompt/atoms/identity/nemesis.yaml
const nemesisIdentityAtomPath = "identity/nemesis"

// NemesisShard is a persistent specialist (Type B) that performs system-level
// adversarial analysis. It opposes the Coder by actively trying to break
// patches before they are merged.
type NemesisShard struct {
	*core.BaseShardAgent

	// Dependencies
	llmClient     perception.LLMClient
	kernel        core.Kernel
	virtualStore  *core.VirtualStore
	learningStore core.LearningStore
	assembler     *articulation.PromptAssembler

	// State
	vulnerabilityDB *VulnerabilityDB
	armory          *Armory
	mu              sync.RWMutex
}

// VulnerabilityDB tracks successful past attacks and lazy patterns.
type VulnerabilityDB struct {
	SuccessfulAttacks []AttackRecord    `json:"successful_attacks"`
	FailedAttacks     []AttackRecord    `json:"failed_attacks"`
	LazyPatterns      map[string]int    `json:"lazy_patterns"`   // pattern -> count
	HardenedAreas     map[string]int    `json:"hardened_areas"`  // path -> survival count
	LastUpdated       time.Time         `json:"last_updated"`
}

// AttackRecord stores information about a past attack.
type AttackRecord struct {
	PatchID     string    `json:"patch_id"`
	AttackTool  string    `json:"attack_tool"`
	Category    string    `json:"category"`
	Hypothesis  string    `json:"hypothesis"`
	Success     bool      `json:"success"`
	Timestamp   time.Time `json:"timestamp"`
}

// NemesisAnalysis is the output of analyzing a patch.
type NemesisAnalysis struct {
	TargetPatch string        `json:"target_patch"`
	Analysis    ChangeAnalysis `json:"analysis"`
	AttackTools []AttackSpec   `json:"attack_tools"`
}

// ChangeAnalysis describes what changed and its risk level.
type ChangeAnalysis struct {
	ChangeType     string   `json:"change_type"`     // bugfix, feature, refactor
	RiskAssessment string   `json:"risk_assessment"` // low, medium, high, critical
	AttackSurface  []string `json:"attack_surface"`  // functions that can be attacked
	Assumptions    []string `json:"assumptions"`     // assumptions the code makes
}

// AttackSpec describes an attack tool to generate via Ouroboros.
type AttackSpec struct {
	Name          string `json:"name"`
	Category      string `json:"type"`       // concurrency, resource, logic, integration
	Hypothesis    string `json:"hypothesis"` // what we expect to break
	Specification string `json:"specification"` // tool generation prompt
}

// GauntletResult represents the outcome of running The Gauntlet.
type GauntletResult struct {
	PatchID       string         `json:"patch_id"`
	Phase         string         `json:"phase"`
	Verdict       string         `json:"verdict"` // passed, failed
	Timestamp     time.Time      `json:"timestamp"`
	AttacksRun    int            `json:"attacks_run"`
	AttacksFailed int            `json:"attacks_failed"`
	Details       []AttackResult `json:"details"`
}

// AttackResult represents the outcome of a single attack.
type AttackResult struct {
	AttackTool string `json:"attack_tool"`
	Success    bool   `json:"success"` // true = attack succeeded (system broke)
	Failure    string `json:"failure"` // what invariant was violated
	Duration   int64  `json:"duration_ms"`
}

// NewNemesisShard creates a new Nemesis shard instance.
func NewNemesisShard() *NemesisShard {
	config := core.DefaultSpecialistConfig("nemesis", "")
	config.Type = core.ShardTypePersistent
	config.Permissions = []core.ShardPermission{
		core.PermissionReadFile,
		core.PermissionExecCmd,
		core.PermissionCodeGraph,
	}

	logging.Shards("Creating NemesisShard (Type B: Persistent Adversarial Specialist)")

	return &NemesisShard{
		BaseShardAgent: core.NewBaseShardAgent("nemesis", config),
		vulnerabilityDB: &VulnerabilityDB{
			LazyPatterns:  make(map[string]int),
			HardenedAreas: make(map[string]int),
		},
	}
}

// SetLLMClient injects the LLM client.
func (n *NemesisShard) SetLLMClient(client perception.LLMClient) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.llmClient = client
}

// SetParentKernel injects the kernel reference.
func (n *NemesisShard) SetParentKernel(k core.Kernel) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.kernel = k
	n.BaseShardAgent.SetParentKernel(k)
}

// SetVirtualStore injects the virtual store.
func (n *NemesisShard) SetVirtualStore(vs *core.VirtualStore) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.virtualStore = vs
}

// SetLearningStore injects the learning store for vulnerability persistence.
func (n *NemesisShard) SetLearningStore(store core.LearningStore) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.learningStore = store
}

// SetArmory injects the Armory for persisting successful attack tools.
func (n *NemesisShard) SetArmory(armory *Armory) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.armory = armory
}

// SetPromptAssembler injects the prompt assembler.
func (n *NemesisShard) SetPromptAssembler(assembler *articulation.PromptAssembler) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.assembler = assembler
}

// Execute runs the Nemesis analysis on a patch, diff, or file target.
// Task formats:
//   - "analyze:<patch_id>" - Analyze a patch for weaknesses
//   - "gauntlet:<patch_id>" - Run full adversarial testing
//   - "review:<file_or_dir>" - Review files and generate/run attack scripts (for /review command)
//   - "anti_autopoiesis:<patch_id>" - Detect lazy fix patterns
func (n *NemesisShard) Execute(ctx context.Context, task string) (string, error) {
	timer := logging.StartTimer(logging.CategoryShards, "NemesisShard.Execute")
	defer timer.Stop()

	logging.Shards("NemesisShard executing: %s", task)
	n.SetState(core.ShardStateRunning)
	defer n.SetState(core.ShardStateCompleted)

	// Parse task type
	parts := strings.SplitN(task, ":", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid task format, expected 'analyze:<patch_id>', 'gauntlet:<patch_id>', or 'review:<target>'")
	}

	action := parts[0]
	target := parts[1]

	switch action {
	case "analyze":
		return n.analyzePatch(ctx, target)
	case "gauntlet":
		return n.runGauntlet(ctx, target)
	case "review":
		return n.reviewTarget(ctx, target)
	case "anti_autopoiesis":
		return n.detectLazyPatterns(ctx, target)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// analyzePatch performs adversarial analysis on a patch.
func (n *NemesisShard) analyzePatch(ctx context.Context, patchID string) (string, error) {
	logging.Shards("Nemesis analyzing patch: %s", patchID)

	// Get the diff from the kernel or virtual store
	diff, err := n.getPatchDiff(ctx, patchID)
	if err != nil {
		return "", fmt.Errorf("failed to get patch diff: %w", err)
	}

	// Build analysis prompt
	prompt := n.buildAnalysisPrompt(diff)

	// Query LLM for analysis
	n.mu.RLock()
	client := n.llmClient
	n.mu.RUnlock()

	if client == nil {
		return "", fmt.Errorf("LLM client not available")
	}

	response, err := client.Complete(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Parse the analysis
	analysis, err := n.parseAnalysisResponse(response)
	if err != nil {
		logging.Get(logging.CategoryShards).Warn("Failed to parse Nemesis analysis: %v", err)
		// Return raw response as fallback
		return response, nil
	}

	analysis.TargetPatch = patchID

	// Record analysis in kernel
	n.recordAnalysis(analysis)

	// Format output
	output, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return response, nil
	}

	return string(output), nil
}

// runGauntlet executes the full adversarial testing gauntlet on a patch.
func (n *NemesisShard) runGauntlet(ctx context.Context, patchID string) (string, error) {
	logging.Shards("Nemesis running The Gauntlet for patch: %s", patchID)

	result := &GauntletResult{
		PatchID:   patchID,
		Phase:     "nemesis",
		Verdict:   "passed",
		Timestamp: time.Now(),
	}

	// First, analyze the patch
	analysisJSON, err := n.analyzePatch(ctx, patchID)
	if err != nil {
		return "", fmt.Errorf("analysis phase failed: %w", err)
	}

	var analysis NemesisAnalysis
	if err := json.Unmarshal([]byte(analysisJSON), &analysis); err != nil {
		logging.Get(logging.CategoryShards).Warn("Failed to parse analysis for Gauntlet: %v", err)
		// Continue with empty attack list
	}

	// Run each attack tool
	for _, attackSpec := range analysis.AttackTools {
		logging.Shards("Gauntlet attack: %s (%s)", attackSpec.Name, attackSpec.Category)
		result.AttacksRun++

		attackResult := n.runAttack(ctx, patchID, attackSpec)
		result.Details = append(result.Details, attackResult)

		if attackResult.Success {
			// Attack succeeded = system broke = Nemesis wins
			result.AttacksFailed++
			result.Verdict = "failed"

			// Record successful attack
			n.recordAttackSuccess(patchID, attackSpec, attackResult.Failure)

			logging.Shards("Nemesis VICTORY: %s broke the system", attackSpec.Name)
			break // Fail fast
		}
	}

	// Run regression attacks from Armory
	if n.armory != nil {
		armoryAttacks := n.armory.GetRegressionAttacks()
		for _, attack := range armoryAttacks {
			logging.Shards("Armory regression: %s", attack.Name)
			result.AttacksRun++

			// Convert armory attack to spec
			spec := AttackSpec{
				Name:       attack.Name,
				Category:   attack.Category,
				Hypothesis: attack.Vulnerability,
			}

			attackResult := n.runAttack(ctx, patchID, spec)
			result.Details = append(result.Details, attackResult)

			if attackResult.Success {
				result.AttacksFailed++
				result.Verdict = "failed"
				logging.Shards("Armory regression VICTORY: %s broke the system", attack.Name)
				break
			}
		}
	}

	// Record Gauntlet result in kernel
	n.recordGauntletResult(result)

	// If passed, record hardened areas
	if result.Verdict == "passed" {
		for _, surface := range analysis.Analysis.AttackSurface {
			n.vulnerabilityDB.HardenedAreas[surface]++
		}
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Gauntlet %s (attacks: %d/%d)", result.Verdict, result.AttacksFailed, result.AttacksRun), nil
	}

	return string(output), nil
}

// detectLazyPatterns implements anti-autopoiesis by detecting recurring weak fixes.
func (n *NemesisShard) detectLazyPatterns(ctx context.Context, patchID string) (string, error) {
	logging.Shards("Nemesis detecting lazy patterns in patch: %s", patchID)

	diff, err := n.getPatchDiff(ctx, patchID)
	if err != nil {
		return "", fmt.Errorf("failed to get patch diff: %w", err)
	}

	// Look for common lazy fix patterns
	patterns := []struct {
		Name    string
		Pattern string
		Counter string
	}{
		{"timeout_increase", "Timeout", "timeout_increase"},
		{"retry_addition", "retry", "retry_addition"},
		{"error_swallow", "_ = err", "error_swallow"},
		{"mutex_wrap", "sync.Mutex", "mutex_wrap"},
	}

	detectedPatterns := make([]string, 0)
	for _, p := range patterns {
		if strings.Contains(diff, p.Pattern) {
			n.vulnerabilityDB.LazyPatterns[p.Counter]++
			count := n.vulnerabilityDB.LazyPatterns[p.Counter]

			if count >= 3 {
				detectedPatterns = append(detectedPatterns, fmt.Sprintf(
					"LAZY PATTERN DETECTED: %s (seen %d times). Consider fixing root cause.",
					p.Name, count,
				))
			}
		}
	}

	if len(detectedPatterns) == 0 {
		return "No lazy patterns detected.", nil
	}

	return strings.Join(detectedPatterns, "\n"), nil
}

// ReviewResult holds the output of a Nemesis review for integration with /review command.
type ReviewResult struct {
	Target           string             `json:"target"`
	FilesAnalyzed    []string           `json:"files_analyzed"`
	AttacksGenerated int                `json:"attacks_generated"`
	AttacksExecuted  int                `json:"attacks_executed"`
	VulnsFound       int                `json:"vulns_found"`
	Findings         []ReviewFinding    `json:"findings"`
	AttackResults    []*AttackExecution `json:"attack_results,omitempty"`
	Duration         time.Duration      `json:"duration"`
}

// ReviewFinding represents a vulnerability found during Nemesis review.
type ReviewFinding struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Function    string `json:"function"`
	Severity    string `json:"severity"` // critical, error, warning, info
	Category    string `json:"category"` // nil_pointer, boundary, concurrency, resource, format
	Description string `json:"description"`
	Evidence    string `json:"evidence"` // Output from attack or code pattern
}

// reviewTarget performs adversarial review of files for the /review command.
// This generates and executes attack scripts to find where code breaks.
func (n *NemesisShard) reviewTarget(ctx context.Context, target string) (string, error) {
	logging.Shards("Nemesis reviewing target: %s", target)
	startTime := time.Now()

	result := &ReviewResult{
		Target:   target,
		Findings: make([]ReviewFinding, 0),
	}

	// Resolve target to list of Go files
	files, err := n.resolveTargetFiles(target)
	if err != nil {
		return "", fmt.Errorf("failed to resolve target files: %w", err)
	}
	result.FilesAnalyzed = files

	if len(files) == 0 {
		return n.formatReviewResult(result), nil
	}

	logging.Shards("Nemesis found %d Go files to analyze", len(files))

	// Create attack runner
	runner, err := NewAttackRunner(30*time.Second, 512)
	if err != nil {
		logging.Shards("Failed to create attack runner: %v", err)
		// Continue without execution - just do static analysis
	}
	if runner != nil {
		defer runner.Cleanup()
	}

	// Analyze each file and generate attacks
	n.mu.RLock()
	client := n.llmClient
	n.mu.RUnlock()

	for _, file := range files {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Read source code
		sourceCode, err := n.readSourceFile(file)
		if err != nil {
			logging.Shards("Failed to read %s: %v", file, err)
			continue
		}

		// Extract function names from source
		functions := n.extractFunctionNames(sourceCode)
		if len(functions) == 0 {
			continue
		}

		// Generate attack scripts via LLM
		if client != nil && runner != nil {
			scripts, err := runner.GenerateAttackScripts(
				ctx,
				func(ctx context.Context, prompt string) (string, error) {
					return client.Complete(ctx, prompt)
				},
				file,
				functions,
				sourceCode,
			)
			if err != nil {
				logging.Shards("Failed to generate attacks for %s: %v", file, err)
			} else {
				result.AttacksGenerated += len(scripts)

				// Execute the attacks
				executions, err := runner.RunAttackBattery(ctx, scripts)
				if err != nil {
					logging.Shards("Attack execution error for %s: %v", file, err)
				}

				result.AttacksExecuted += len(executions)
				result.AttackResults = append(result.AttackResults, executions...)

				// Convert successful attacks to findings
				for _, exec := range executions {
					if exec.Success {
						result.VulnsFound++
						result.Findings = append(result.Findings, ReviewFinding{
							File:        file,
							Function:    exec.Script.TargetFunction,
							Severity:    n.breakageToSeverity(exec.BreakageType),
							Category:    exec.Script.Category,
							Description: fmt.Sprintf("%s: %s", exec.Script.Name, exec.Script.Hypothesis),
							Evidence:    n.truncateOutput(exec.Output, 500),
						})
					}
				}
			}
		}

		// Also do static pattern analysis
		staticFindings := n.analyzeStaticPatterns(file, sourceCode)
		result.Findings = append(result.Findings, staticFindings...)
	}

	result.Duration = time.Since(startTime)

	// Record findings in kernel
	n.recordReviewFindings(result)

	return n.formatReviewResult(result), nil
}

// resolveTargetFiles converts a target path to a list of Go files.
func (n *NemesisShard) resolveTargetFiles(target string) ([]string, error) {
	var files []string

	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// Single file
		if strings.HasSuffix(target, ".go") && !strings.HasSuffix(target, "_test.go") {
			files = append(files, target)
		}
		return files, nil
	}

	// Walk directory
	err = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Skip vendor, node_modules, hidden dirs
			name := info.Name()
			if name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Include Go files but skip tests
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// readSourceFile reads a Go source file.
func (n *NemesisShard) readSourceFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Limit to first 10KB for analysis
	if len(data) > 10*1024 {
		data = data[:10*1024]
	}

	return string(data), nil
}

// extractFunctionNames extracts function names from Go source code.
func (n *NemesisShard) extractFunctionNames(source string) []string {
	var functions []string

	lines := strings.Split(source, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "func ") {
			// Extract function name
			// func Foo(
			// func (r *Receiver) Method(
			rest := strings.TrimPrefix(line, "func ")

			// Handle method receiver
			if strings.HasPrefix(rest, "(") {
				idx := strings.Index(rest, ")")
				if idx > 0 {
					rest = strings.TrimSpace(rest[idx+1:])
				}
			}

			// Get name up to (
			idx := strings.Index(rest, "(")
			if idx > 0 {
				name := strings.TrimSpace(rest[:idx])
				if name != "" && name[0] >= 'A' && name[0] <= 'Z' {
					// Only exported functions
					functions = append(functions, name)
				}
			}
		}
	}

	return functions
}

// analyzeStaticPatterns looks for common vulnerability patterns without execution.
func (n *NemesisShard) analyzeStaticPatterns(file, source string) []ReviewFinding {
	var findings []ReviewFinding

	lines := strings.Split(source, "\n")

	// Pattern matchers
	patterns := []struct {
		Pattern   string
		Category  string
		Severity  string
		Message   string
	}{
		{" = err", "error_handling", "warning", "Error assigned but may not be checked"},
		{"_ = err", "error_handling", "error", "Error explicitly ignored"},
		{"panic(", "stability", "warning", "Explicit panic - may crash in production"},
		{".Lock()", "concurrency", "info", "Mutex usage - verify unlock on all paths"},
		{"go func()", "concurrency", "warning", "Goroutine closure - verify variable capture"},
		{"interface{}", "type_safety", "info", "Empty interface - type assertions may panic"},
		{"unsafe.", "memory", "warning", "Unsafe package usage - potential memory corruption"},
	}

	for lineNum, line := range lines {
		for _, p := range patterns {
			if strings.Contains(line, p.Pattern) {
				findings = append(findings, ReviewFinding{
					File:        file,
					Line:        lineNum + 1,
					Severity:    p.Severity,
					Category:    p.Category,
					Description: p.Message,
					Evidence:    strings.TrimSpace(line),
				})
			}
		}
	}

	return findings
}

// breakageToSeverity converts a breakage type to severity level.
func (n *NemesisShard) breakageToSeverity(breakageType string) string {
	switch breakageType {
	case "panic":
		return "critical"
	case "race":
		return "critical"
	case "timeout":
		return "error"
	case "assertion":
		return "error"
	default:
		return "warning"
	}
}

// truncateOutput limits output length for display.
func (n *NemesisShard) truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "...[truncated]"
}

// recordReviewFindings records findings in the kernel.
func (n *NemesisShard) recordReviewFindings(result *ReviewResult) {
	if n.kernel == nil {
		return
	}

	// Record summary fact
	_ = n.kernel.Assert(core.Fact{
		Predicate: "nemesis_review",
		Args: []interface{}{
			result.Target,
			len(result.FilesAnalyzed),
			result.AttacksGenerated,
			result.VulnsFound,
			time.Now().Unix(),
		},
	})

	// Record each finding
	for _, f := range result.Findings {
		_ = n.kernel.Assert(core.Fact{
			Predicate: "nemesis_finding",
			Args: []interface{}{
				f.File,
				f.Line,
				f.Severity,
				f.Category,
				f.Description,
			},
		})
	}
}

// formatReviewResult formats the review result for output.
func (n *NemesisShard) formatReviewResult(result *ReviewResult) string {
	var sb strings.Builder

	sb.WriteString("## Nemesis Adversarial Review\n\n")

	// Summary stats
	sb.WriteString(fmt.Sprintf("**Target:** %s\n", result.Target))
	sb.WriteString(fmt.Sprintf("**Files Analyzed:** %d\n", len(result.FilesAnalyzed)))
	sb.WriteString(fmt.Sprintf("**Attacks Generated:** %d\n", result.AttacksGenerated))
	sb.WriteString(fmt.Sprintf("**Attacks Executed:** %d\n", result.AttacksExecuted))
	sb.WriteString(fmt.Sprintf("**Vulnerabilities Found:** %d\n", result.VulnsFound))
	sb.WriteString(fmt.Sprintf("**Duration:** %v\n\n", result.Duration.Round(time.Millisecond)))

	if result.VulnsFound > 0 {
		sb.WriteString("**STATUS: VULNERABILITIES DETECTED**\n\n")
	} else if result.AttacksExecuted > 0 {
		sb.WriteString("**STATUS: Code survived adversarial testing**\n\n")
	}

	// Group findings by severity
	critical := make([]ReviewFinding, 0)
	errors := make([]ReviewFinding, 0)
	warnings := make([]ReviewFinding, 0)
	info := make([]ReviewFinding, 0)

	for _, f := range result.Findings {
		switch f.Severity {
		case "critical":
			critical = append(critical, f)
		case "error":
			errors = append(errors, f)
		case "warning":
			warnings = append(warnings, f)
		default:
			info = append(info, f)
		}
	}

	// Output by severity
	if len(critical) > 0 {
		sb.WriteString("### Critical Issues\n\n")
		for _, f := range critical {
			sb.WriteString(n.formatFinding(f))
		}
	}

	if len(errors) > 0 {
		sb.WriteString("### Errors\n\n")
		for _, f := range errors {
			sb.WriteString(n.formatFinding(f))
		}
	}

	if len(warnings) > 0 {
		sb.WriteString("### Warnings\n\n")
		for _, f := range warnings {
			sb.WriteString(n.formatFinding(f))
		}
	}

	if len(info) > 0 && len(result.Findings) < 20 {
		sb.WriteString("### Info\n\n")
		for _, f := range info {
			sb.WriteString(n.formatFinding(f))
		}
	}

	// Attack execution details if any succeeded
	if result.AttackResults != nil {
		sb.WriteString(FormatAttackReport(result.AttackResults))
	}

	return sb.String()
}

// formatFinding formats a single finding for display.
func (n *NemesisShard) formatFinding(f ReviewFinding) string {
	var sb strings.Builder

	location := f.File
	if f.Line > 0 {
		location = fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	if f.Function != "" {
		location = fmt.Sprintf("%s (%s)", location, f.Function)
	}

	sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", f.Category, f.Description))
	sb.WriteString(fmt.Sprintf("  - Location: `%s`\n", location))
	if f.Evidence != "" {
		sb.WriteString(fmt.Sprintf("  - Evidence: `%s`\n", f.Evidence))
	}
	sb.WriteString("\n")

	return sb.String()
}

// runAttack executes a single attack against the patch.
func (n *NemesisShard) runAttack(ctx context.Context, patchID string, spec AttackSpec) AttackResult {
	result := AttackResult{
		AttackTool: spec.Name,
		Success:    false,
	}

	startTime := time.Now()

	// For now, this is a simulation - in a full implementation,
	// this would trigger Ouroboros to generate the attack tool and run it
	// against the staged environment.

	// Check if kernel has system invariant violations
	if n.kernel != nil {
		facts, err := n.kernel.Query("system_invariant_violated")
		if err == nil && len(facts) > 0 {
			// System is already in a bad state
			result.Success = true
			if len(facts[0].Args) > 0 {
				result.Failure = fmt.Sprintf("%v", facts[0].Args[0])
			}
		}
	}

	result.Duration = time.Since(startTime).Milliseconds()
	return result
}

// getPatchDiff retrieves the diff for a patch from the kernel or virtual store.
func (n *NemesisShard) getPatchDiff(ctx context.Context, patchID string) (string, error) {
	// Try kernel first
	if n.kernel != nil {
		facts, err := n.kernel.Query("patch_diff")
		if err == nil {
			for _, fact := range facts {
				if len(fact.Args) >= 2 && fmt.Sprintf("%v", fact.Args[0]) == patchID {
					if diff, ok := fact.Args[1].(string); ok {
						return diff, nil
					}
				}
			}
		}
	}

	// Try virtual store
	if n.virtualStore != nil {
		result, err := n.virtualStore.RouteAction(ctx, core.Fact{
			Predicate: "action",
			Args:      []interface{}{"/get_patch_diff", patchID},
		})
		if err == nil && result != "" {
			return result, nil
		}
	}

	// Return placeholder if not found
	return "// Patch diff not available - analyzing patch ID: " + patchID, nil
}

// buildAnalysisPrompt constructs the prompt for patch analysis using JIT prompt system.
func (n *NemesisShard) buildAnalysisPrompt(diff string) string {
	n.mu.RLock()
	assembler := n.assembler
	n.mu.RUnlock()

	// Use JIT prompt assembler if available
	if assembler != nil {
		// Build context for JIT compilation
		pc := &articulation.PromptContext{
			ShardID:   n.GetID(),
			ShardType: "nemesis",
		}

		// Compile prompt from atoms via JIT
		compiled, err := assembler.AssembleSystemPrompt(context.Background(), pc)
		if err == nil && compiled != "" {
			// Append the dynamic diff content
			return compiled + "\n\n## PATCH DIFF\n```\n" + diff + "\n```\n"
		}
		logging.ShardsDebug("JIT prompt compilation failed, using fallback: %v", err)
	}

	// Fallback: minimal inline prompt (identity loaded from YAML atoms in production)
	var sb strings.Builder
	sb.WriteString("# Nemesis Analysis Request\n\n")
	sb.WriteString("You are the Nemesis - a Systemic Chaos Architect. Analyze this patch and find weaknesses.\n\n")
	sb.WriteString("## PATCH DIFF\n```\n")
	sb.WriteString(diff)
	sb.WriteString("\n```\n\n")
	sb.WriteString("Output JSON with 'analysis' and 'attack_tools' arrays.\n")
	return sb.String()
}

// parseAnalysisResponse parses the LLM response into a NemesisAnalysis.
func (n *NemesisShard) parseAnalysisResponse(response string) (*NemesisAnalysis, error) {
	// Clean up response
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Find JSON object bounds
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	response = response[start : end+1]

	var analysis NemesisAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse analysis JSON: %w", err)
	}

	return &analysis, nil
}

// recordAnalysis records the analysis in the kernel.
func (n *NemesisShard) recordAnalysis(analysis *NemesisAnalysis) {
	if n.kernel == nil {
		return
	}

	// Record patch analysis
	_ = n.kernel.Assert(core.Fact{
		Predicate: "nemesis_analysis",
		Args: []interface{}{
			analysis.TargetPatch,
			analysis.Analysis.ChangeType,
			analysis.Analysis.RiskAssessment,
			len(analysis.AttackTools),
			time.Now().Unix(),
		},
	})

	// Record attack surface
	for _, surface := range analysis.Analysis.AttackSurface {
		_ = n.kernel.Assert(core.Fact{
			Predicate: "attack_surface",
			Args:      []interface{}{analysis.TargetPatch, surface},
		})
	}

	// Record attack tools
	for _, tool := range analysis.AttackTools {
		_ = n.kernel.Assert(core.Fact{
			Predicate: "nemesis_attack_tool",
			Args: []interface{}{
				tool.Name,
				tool.Name,
				analysis.TargetPatch,
				tool.Category,
			},
		})
	}
}

// recordAttackSuccess records a successful attack in kernel and armory.
func (n *NemesisShard) recordAttackSuccess(patchID string, spec AttackSpec, failure string) {
	if n.kernel != nil {
		_ = n.kernel.Assert(core.Fact{
			Predicate: "nemesis_attack_run",
			Args: []interface{}{
				spec.Name,
				patchID,
				time.Now().Unix(),
				"/success",
			},
		})

		_ = n.kernel.Assert(core.Fact{
			Predicate: "nemesis_victory",
			Args:      []interface{}{patchID},
		})
	}

	// Add to vulnerability DB
	n.vulnerabilityDB.SuccessfulAttacks = append(n.vulnerabilityDB.SuccessfulAttacks, AttackRecord{
		PatchID:    patchID,
		AttackTool: spec.Name,
		Category:   spec.Category,
		Hypothesis: spec.Hypothesis,
		Success:    true,
		Timestamp:  time.Now(),
	})

	// Add to armory for regression testing
	if n.armory != nil {
		n.armory.AddAttack(ArmoryAttack{
			Name:          spec.Name,
			Category:      spec.Category,
			Vulnerability: failure,
			Specification: spec.Specification,
			CreatedAt:     time.Now(),
		})
	}
}

// recordGauntletResult records the Gauntlet result in the kernel.
func (n *NemesisShard) recordGauntletResult(result *GauntletResult) {
	if n.kernel == nil {
		return
	}

	verdict := "/passed"
	if result.Verdict == "failed" {
		verdict = "/failed"
	}

	_ = n.kernel.Assert(core.Fact{
		Predicate: "gauntlet_result",
		Args: []interface{}{
			result.PatchID,
			result.Phase,
			verdict,
			result.Timestamp.Unix(),
		},
	})
}
