package core

import (
	"codenerd/internal/logging"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// TOOL REGISTRY - Integration with Kernel and Shards
// =============================================================================
// This registry bridges the gap between generated tools (Ouroboros),
// the Mangle kernel (facts), and shards (execution).

// Tool represents a registered tool with metadata
type Tool struct {
	Name          string    `json:"name"`
	Command       string    `json:"command"`        // Path to binary or command to execute
	ShardAffinity string    `json:"shard_affinity"` // /coder, /tester, /reviewer, /researcher, /generalist, /all
	Description   string    `json:"description"`
	Capabilities  []string  `json:"capabilities"`
	Hash          string    `json:"hash"` // Binary hash for change detection
	RegisteredAt  time.Time `json:"registered_at"`
	ExecuteCount  int64     `json:"execute_count"`
}

// ToolRegistry manages registered tools and their integration with the kernel
type ToolRegistry struct {
	mu      sync.RWMutex
	tools   map[string]*Tool
	kernel  Kernel
	workDir string
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(workDir string) *ToolRegistry {
	return &ToolRegistry{
		tools:   make(map[string]*Tool),
		workDir: workDir,
	}
}

// SetKernel sets the kernel for fact injection
func (tr *ToolRegistry) SetKernel(k Kernel) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.kernel = k
}

// RegisterTool registers a tool and injects facts into the kernel
func (tr *ToolRegistry) RegisterTool(name, command, shardAffinity string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	logging.ToolsDebug("RegisterTool: registering tool name=%s command=%s affinity=%s", name, command, shardAffinity)

	if name == "" {
		logging.ToolsError("RegisterTool: tool name cannot be empty")
		return fmt.Errorf("tool name cannot be empty")
	}
	if command == "" {
		logging.ToolsError("RegisterTool: tool command cannot be empty for %s", name)
		return fmt.Errorf("tool command cannot be empty")
	}

	// Verify command exists if it's a file path
	if !isCommandName(command) {
		absPath := command
		if !filepath.IsAbs(command) {
			absPath = filepath.Join(tr.workDir, command)
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			logging.ToolsError("RegisterTool: tool binary not found: %s", absPath)
			return fmt.Errorf("tool binary not found: %s", absPath)
		}
		command = absPath
	}

	// Create tool
	tool := &Tool{
		Name:          name,
		Command:       command,
		ShardAffinity: shardAffinity,
		RegisteredAt:  time.Now(),
		Capabilities:  vocabularyCapabilities(name, command, "", nil),
	}

	tr.tools[name] = tool

	// Inject facts into kernel (single tool, immediate evaluate)
	if err := tr.injectToolFacts(tool); err != nil {
		logging.ToolsError("RegisterTool: failed to inject facts for %s: %v", name, err)
		return err
	}

	logging.Tools("RegisterTool: successfully registered tool %s (affinity=%s)", name, shardAffinity)
	return nil
}

// RegisterToolWithInfo registers a tool with full metadata
func (tr *ToolRegistry) RegisterToolWithInfo(tool *Tool) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if tool.RegisteredAt.IsZero() {
		tool.RegisteredAt = time.Now()
	}

	tr.tools[tool.Name] = tool

	// Inject facts into kernel
	return tr.injectToolFacts(tool)
}

// GetTool retrieves a registered tool by name
func (tr *ToolRegistry) GetTool(name string) (*Tool, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	tool, exists := tr.tools[name]
	return tool, exists
}

// GetToolsForShard returns all tools that can be used by a specific shard type
func (tr *ToolRegistry) GetToolsForShard(shardType string) []*Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tools := make([]*Tool, 0)
	for _, tool := range tr.tools {
		if tool.ShardAffinity == "/all" || tool.ShardAffinity == shardType {
			tools = append(tools, tool)
		}
	}
	return tools
}

// ListTools returns all registered tools
func (tr *ToolRegistry) ListTools() []*Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tools := make([]*Tool, 0, len(tr.tools))
	for _, tool := range tr.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ExecuteTool runs a registered tool with raw input (ToolExecutor interface).
func (tr *ToolRegistry) ExecuteTool(ctx context.Context, toolName string, input string) (string, error) {
	args := parseToolInput(input)
	return tr.ExecuteRegisteredTool(ctx, toolName, args)
}

// ExecuteRegisteredTool executes a registered tool with the given arguments
func (tr *ToolRegistry) ExecuteRegisteredTool(ctx context.Context, toolName string, args []string) (string, error) {
	tool, exists := tr.GetTool(toolName)
	if !exists {
		logging.ToolsError("ExecuteRegisteredTool: tool not registered: %s", toolName)
		return "", fmt.Errorf("tool not registered: %s", toolName)
	}

	// Update execution count
	tr.mu.Lock()
	tool.ExecuteCount++
	execCount := tool.ExecuteCount
	tr.mu.Unlock()

	logging.Tools("ExecuteRegisteredTool: executing tool=%s exec_count=%d args=%v", toolName, execCount, args)

	// Security Validation
	if err := secureValidateCommand(tool.Command); err != nil {
		logging.ToolsError("ExecuteRegisteredTool: security validation failed for command %s: %v", tool.Command, err)
		return "", fmt.Errorf("security validation failed: %w", err)
	}
	if err := secureValidateArgs(args); err != nil {
		logging.ToolsError("ExecuteRegisteredTool: security validation failed for args: %v", err)
		return "", fmt.Errorf("security validation failed: %w", err)
	}

	startTime := time.Now()
	/* #nosec G204 */
	cmd := exec.CommandContext(ctx, tool.Command, args...)
	if tr.workDir != "" {
		cmd.Dir = tr.workDir
	}
	out, err := cmd.CombinedOutput()
	duration := time.Since(startTime)
	output := string(out)

	if err != nil {
		logging.ToolsError("ExecuteRegisteredTool: tool=%s failed after %v: %v (output_len=%d)", toolName, duration, err, len(output))
		return output, fmt.Errorf("tool execution failed: %w", err)
	}

	logging.Tools("ExecuteRegisteredTool: tool=%s completed in %v (output_len=%d)", toolName, duration, len(output))
	return output, nil
}

// UnregisterTool removes a tool from the registry and retracts its facts
func (tr *ToolRegistry) UnregisterTool(name string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	logging.ToolsDebug("UnregisterTool: unregistering tool %s", name)

	if _, exists := tr.tools[name]; !exists {
		logging.ToolsError("UnregisterTool: tool not registered: %s", name)
		return fmt.Errorf("tool not registered: %s", name)
	}

	delete(tr.tools, name)

	// Retract only facts for this specific tool (not all tool facts)
	if tr.kernel != nil {
		var errs []error

		// Retract facts specific to this tool using the tool name as first argument
		if err := tr.kernel.RetractFact(Fact{Predicate: "registered_tool", Args: []any{name}}); err != nil {
			errs = append(errs, fmt.Errorf("failed to retract registered_tool: %w", err))
		}
		if err := tr.kernel.RetractFact(Fact{Predicate: "tool_registered", Args: []any{name}}); err != nil {
			errs = append(errs, fmt.Errorf("failed to retract tool_registered: %w", err))
		}
		if err := tr.kernel.RetractFact(Fact{Predicate: "tool_hash", Args: []any{name}}); err != nil {
			errs = append(errs, fmt.Errorf("failed to retract tool_hash: %w", err))
		}
		if err := tr.kernel.RetractFact(Fact{Predicate: "tool_capability", Args: []any{name}}); err != nil {
			errs = append(errs, fmt.Errorf("failed to retract tool_capability: %w", err))
		}

		if len(errs) > 0 {
			logging.ToolsError("UnregisterTool: errors retracting facts for %s: %v", name, errs)
			return fmt.Errorf("errors retracting tool facts: %v", errs)
		}
	}

	logging.Tools("UnregisterTool: successfully unregistered tool %s", name)
	return nil
}

// toolCapabilityVocabulary is the corpus vocabulary accepted by
// policy/tool_routing.mg (shard_capability_affinity) for tool_capability
// facts. Keep in sync with tool_routing.mg; do not assert raw registration
// categories such as "build", "test" or "lint".
var toolCapabilityVocabulary = map[string]bool{
	"/generation":     true,
	"/debugging":      true,
	"/transformation": true,
	"/inspection":     true,
	"/validation":     true,
	"/execution":      true,
	"/analysis":       true,
	"/knowledge":      true,
}

// legacyToolCapabilityMap maps raw registration categories to the corpus
// vocabulary. It handles the exact category strings production registrations
// have historically carried.
var legacyToolCapabilityMap = map[string][]string{
	"build":     {"/generation", "/execution"},
	"compile":   {"/generation", "/execution"},
	"test":      {"/validation", "/execution"},
	"testing":   {"/validation", "/execution"},
	"lint":      {"/inspection", "/validation"},
	"vet":       {"/inspection", "/validation"},
	"check":     {"/validation"},
	"verify":    {"/validation"},
	"run":       {"/execution"},
	"exec":      {"/execution"},
	"execution": {"/execution"},
	"review":    {"/inspection", "/analysis"},
	"search":    {"/inspection"},
	"read":      {"/inspection"},
	"write":     {"/generation"},
	"edit":      {"/transformation"},
	"generate":  {"/generation"},
	"refactor":  {"/transformation"},
	"debug":     {"/debugging"},
	"fix":       {"/debugging"},
	"analyze":   {"/analysis"},
	"analysis":  {"/analysis"},
	"research":  {"/knowledge"},
	"knowledge": {"/knowledge"},
}

// vocabularyCapabilities maps a tool's effect (command) and purpose
// (name, description, declared categories) to the corpus vocabulary. It is
// the single place where that mapping lives; every registration path funnels
// through it via collectToolFacts (and struct construction), so every
// registered tool carries at least one tool_capability fact in vocabulary.
func vocabularyCapabilities(name, command, description string, declared []string) []string {
	seen := make(map[string]bool)
	add := func(caps ...string) {
		for _, c := range caps {
			if toolCapabilityVocabulary[c] {
				seen[c] = true
			}
		}
	}
	for _, d := range declared {
		norm := strings.ToLower(strings.TrimSpace(d))
		noSlash := strings.TrimPrefix(norm, "/")
		if toolCapabilityVocabulary["/"+noSlash] {
			add("/" + noSlash)
			continue
		}
		if mapped, ok := legacyToolCapabilityMap[noSlash]; ok {
			add(mapped...)
			continue
		}
		inferToolCapabilitiesFromText(d, add)
	}
	combined := strings.ToLower(name + " " + command + " " + description + " " + strings.Join(declared, " "))
	inferToolCapabilitiesFromText(combined, add)
	tokens := strings.FieldsFunc(combined, func(r rune) bool { return r < 'a' || r > 'z' })
	tokenSet := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		tokenSet[t] = true
	}
	hasToken := func(words ...string) bool {
		for _, w := range words {
			if tokenSet[w] {
				return true
			}
		}
		return false
	}
	if hasToken("write", "writes", "writing", "implement", "implements", "implementation") {
		add("/generation")
	}
	if hasToken("edit", "edits", "editing", "refactor", "refactors", "refactoring") {
		add("/transformation")
	}
	if hasToken("fix", "fixes", "fixed", "fixing", "bugfix", "hotfix") {
		add("/debugging")
	}
	if hasToken("read", "reads", "reading", "list", "lists", "listing", "glob", "globs", "grep", "search", "searches", "review", "reviews", "lint", "lints", "linting", "linter", "vet", "inspect", "inspects", "inspection", "snapshot", "snapshots") {
		add("/inspection")
	}
	if hasToken("test", "tests", "testing", "tested", "check", "checks", "checking", "lint", "lints", "linting", "linter", "vet", "validate", "validates", "verify", "verifies") {
		add("/validation")
	}
	if hasToken("run", "runs", "running", "runner", "exec", "executes", "command", "commands") {
		add("/execution")
	}
	if hasToken("web", "query", "queries", "docs") {
		add("/knowledge")
	}
	if len(seen) == 0 {
		add("/inspection")
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// inferToolCapabilitiesFromText adds vocabulary capabilities based on
// distinctive substrings in free text (effect and purpose). Short generic
// words are handled via token matching in vocabularyCapabilities to avoid
// substring false positives; this helper only uses long distinctive stems.
func inferToolCapabilitiesFromText(text string, add func(...string)) {
	lower := strings.ToLower(text)
	hasSub := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(lower, s) {
				return true
			}
		}
		return false
	}
	if hasSub("generat", "scaffold", "creat", "compil", "build") {
		add("/generation")
	}
	if hasSub("refactor", "transform", "migrat", "rewrit") {
		add("/transformation")
	}
	if hasSub("debug", "diagnos", "troubleshoot", "breakpoint", "stack trace") {
		add("/debugging")
	}
	if hasSub("inspect", "snapshot", "explor") {
		add("/inspection")
	}
	if hasSub("validat", "verif", "assert") {
		add("/validation")
	}
	if hasSub("execut", "launch", "deploy", "shell", "compil", "build") {
		add("/execution")
	}
	if hasSub("analy", "explain", "profil", "metric", "complex", "summar") {
		add("/analysis")
	}
	if hasSub("research", "knowledge", "document", "fetch", "lookup") {
		add("/knowledge")
	}
	// Lint/vet imply both inspection and validation; keep the pair together
	// so historically categorized tools stay truthful to what they do.
	if hasSub("lint") {
		add("/inspection", "/validation")
	}
}

// collectToolFacts returns the kernel facts for a tool WITHOUT asserting them.
// Use this when batching facts across multiple tools to avoid per-tool evaluation.
func collectToolFacts(tool *Tool) []Fact {
	facts := []Fact{
		{
			Predicate: "registered_tool",
			Args:      []any{tool.Name, tool.Command, tool.ShardAffinity},
		},
		{
			// tool_registered Decl (schemas_tools.mg:14) binds RegisteredAt as
			// /number: epoch seconds, not a formatted string.
			Predicate: "tool_registered",
			Args:      []any{tool.Name, tool.RegisteredAt.Unix()},
		},
	}

	if tool.Hash != "" {
		facts = append(facts, Fact{
			Predicate: "tool_hash",
			Args:      []any{tool.Name, tool.Hash},
		})
	}

	caps := vocabularyCapabilities(tool.Name, tool.Command, tool.Description, tool.Capabilities)
	for _, cap := range caps {
		facts = append(facts, Fact{
			Predicate: "tool_capability",
			Args:      []any{tool.Name, cap},
		})
	}

	return facts
}

// injectToolFacts injects tool registration facts into the kernel.
// Each Assert triggers a Mangle fixpoint evaluation — use collectToolFacts + AssertBatch
// for bulk registration to avoid O(N) evaluations.
func (tr *ToolRegistry) injectToolFacts(tool *Tool) error {
	if tr.kernel == nil {
		return nil // No kernel configured, skip fact injection
	}

	facts := collectToolFacts(tool)
	return tr.kernel.AssertBatch(facts)
}

// SyncFromOuroboros synchronizes tools from an Ouroboros registry.
// Collects all tool facts into a single batch and evaluates once.
func (tr *ToolRegistry) SyncFromOuroboros(toolExecutor ToolExecutor) error {
	if toolExecutor == nil {
		logging.ToolsDebug("SyncFromOuroboros: no tool executor provided, skipping")
		return nil
	}

	tools := toolExecutor.ListTools()
	logging.ToolsDebug("SyncFromOuroboros: syncing %d tools from Ouroboros", len(tools))

	tr.mu.Lock()
	defer tr.mu.Unlock()

	var allFacts []Fact
	syncedCount := 0

	for _, toolInfo := range tools {
		tool := &Tool{
			Name:          toolInfo.Name,
			Command:       toolInfo.BinaryPath,
			ShardAffinity: "/all",
			Description:   toolInfo.Description,
			Hash:          toolInfo.Hash,
			RegisteredAt:  toolInfo.RegisteredAt,
			ExecuteCount:  toolInfo.ExecuteCount,
		}

		if tool.Name == "" {
			continue
		}
		if tool.RegisteredAt.IsZero() {
			tool.RegisteredAt = time.Now()
		}

		tr.tools[tool.Name] = tool
		allFacts = append(allFacts, collectToolFacts(tool)...)
		syncedCount++
	}

	// Single batch assert — one Mangle evaluation for all tools
	if tr.kernel != nil && len(allFacts) > 0 {
		if err := tr.kernel.AssertBatch(allFacts); err != nil {
			logging.ToolsError("SyncFromOuroboros: batch assert failed for %d facts: %v", len(allFacts), err)
			return fmt.Errorf("batch assert failed: %w", err)
		}
	}

	logging.Tools("SyncFromOuroboros: synced %d tools (%d facts, 1 evaluation)", syncedCount, len(allFacts))
	return nil
}

// RestoreFromDisk restores the registry from a directory of compiled tools.
// Continues restoring even if individual tools fail, collecting all errors.
func (tr *ToolRegistry) RestoreFromDisk(compiledDir string) error {
	logging.ToolsDebug("RestoreFromDisk: restoring tools from %s", compiledDir)
	entries, err := os.ReadDir(compiledDir)
	if err != nil {
		if os.IsNotExist(err) {
			logging.ToolsDebug("RestoreFromDisk: directory %s does not exist, skipping", compiledDir)
			return nil
		}
		logging.ToolsError("RestoreFromDisk: failed to read directory %s: %v", compiledDir, err)
		return fmt.Errorf("failed to read compiled tools directory: %w", err)
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()

	var allFacts []Fact
	restoredCount := 0
	now := time.Now()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if ext := filepath.Ext(name); ext == ".exe" {
			name = name[:len(name)-len(ext)]
		}

		binaryPath := filepath.Join(compiledDir, entry.Name())

		tool := &Tool{
			Name:          name,
			Command:       binaryPath,
			ShardAffinity: "/all",
			Description:   "Restored from disk",
			RegisteredAt:  now,
		}

		tr.tools[tool.Name] = tool
		allFacts = append(allFacts, collectToolFacts(tool)...)
		restoredCount++
	}

	// Single batch assert — one Mangle evaluation for all tools
	if tr.kernel != nil && len(allFacts) > 0 {
		if err := tr.kernel.AssertBatch(allFacts); err != nil {
			logging.ToolsError("RestoreFromDisk: batch assert failed: %v", err)
			return fmt.Errorf("batch assert failed: %w", err)
		}
	}

	logging.Tools("RestoreFromDisk: restored %d tools (%d facts, 1 evaluation) from %s", restoredCount, len(allFacts), compiledDir)
	return nil
}

// StaticToolDef represents a tool definition loaded from available_tools.json.
// This mirrors init.ToolDefinition but avoids import cycles.
type StaticToolDef struct {
	Name          string
	Category      string
	Description   string
	Command       string
	ShardAffinity string
}

// RestoreFromStaticDefs loads tools from a slice of StaticToolDef into the registry.
// This is used to hydrate tools from available_tools.json at session boot.
// Continues restoring even if individual tools fail, collecting all errors.
func (tr *ToolRegistry) RestoreFromStaticDefs(defs []StaticToolDef) error {
	logging.ToolsDebug("RestoreFromStaticDefs: restoring %d static tool definitions", len(defs))

	tr.mu.Lock()
	defer tr.mu.Unlock()

	var allFacts []Fact
	restoredCount := 0
	now := time.Now()

	for _, def := range defs {
		affinity := def.ShardAffinity
		if affinity == "" {
			affinity = "/all"
		} else if !strings.HasPrefix(affinity, "/") {
			affinity = "/" + strings.ToLower(strings.TrimSuffix(affinity, "Shard"))
		}

		tool := &Tool{
			Name:          def.Name,
			Command:       def.Command,
			ShardAffinity: affinity,
			Description:   def.Description,
			Capabilities:  vocabularyCapabilities(def.Name, def.Command, def.Description, []string{def.Category}),
			RegisteredAt:  now,
		}

		if tool.Name == "" {
			continue
		}

		tr.tools[tool.Name] = tool
		allFacts = append(allFacts, collectToolFacts(tool)...)
		restoredCount++
	}

	// Single batch assert — one Mangle evaluation for all tools
	if tr.kernel != nil && len(allFacts) > 0 {
		if err := tr.kernel.AssertBatch(allFacts); err != nil {
			logging.ToolsError("RestoreFromStaticDefs: batch assert failed: %v", err)
			return fmt.Errorf("batch assert failed: %w", err)
		}
	}

	logging.Tools("RestoreFromStaticDefs: restored %d tools (%d facts, 1 evaluation)", restoredCount, len(allFacts))
	return nil
}

// BuildToolCatalog creates a formatted tool catalog string for prompt injection.
// This enables Piggyback++ architecture where tools are requested via structured
// output (control_packet.tool_requests) instead of native LLM function calling.
//
// The catalog is organized by shard affinity and includes:
// - Tool name and description
// - Parameters with types and descriptions (from schema when available)
// - Usage examples
//
// Parameters:
//   - shardType: Filter tools by shard affinity ("/coder", "/tester", "/all", etc.)
//     Pass empty string to include all tools.
func (tr *ToolRegistry) BuildToolCatalog(shardType string) string {
	tools := tr.GetToolsForShard(shardType)
	if len(tools) == 0 {
		// Also check for "/all" if specific shard has no tools
		if shardType != "" && shardType != "/all" {
			tools = tr.GetToolsForShard("/all")
		}
	}

	if len(tools) == 0 {
		return ""
	}

	var catalog strings.Builder
	catalog.WriteString("\n## Available Tools\n\n")
	catalog.WriteString("Request tools via `tool_requests` in control_packet:\n")
	catalog.WriteString("```json\n")
	catalog.WriteString("\"tool_requests\": [{\n")
	catalog.WriteString("  \"id\": \"req_1\",\n")
	catalog.WriteString("  \"tool_name\": \"<tool_name>\",\n")
	catalog.WriteString("  \"tool_args\": { ... },\n")
	catalog.WriteString("  \"purpose\": \"why this tool is needed\"\n")
	catalog.WriteString("}]\n")
	catalog.WriteString("```\n\n")

	// Group tools by affinity for better organization
	byAffinity := make(map[string][]*Tool)
	for _, tool := range tools {
		affinity := tool.ShardAffinity
		if affinity == "" {
			affinity = "/all"
		}
		byAffinity[affinity] = append(byAffinity[affinity], tool)
	}

	// Output tools grouped by affinity, in a stable order.
	//
	// This catalog is part of the system prompt, and by the epoch fingerprint's
	// own design tool definitions are the FIRST thing in a provider's cacheable
	// prefix. Ranging the map directly shuffled the section order on every call,
	// so the prefix bytes differed run to run and a prefix cache could never hit
	// across them — a cost paid on every request, to save a sort of half a dozen
	// keys.
	//
	// It also makes the prompt diffable. Two runs that differ only in map order
	// cannot be compared, which is exactly what someone needs to do when a
	// prompt change makes the agent worse.
	affinities := make([]string, 0, len(byAffinity))
	for affinity := range byAffinity {
		affinities = append(affinities, affinity)
	}
	sort.Strings(affinities)

	for _, affinity := range affinities {
		toolList := byAffinity[affinity]
		// Within a section too. GetToolsForShard builds its slice by ranging
		// the registry map, so the tools arrive in a different order on every
		// call — sorting the sections alone would leave the same instability
		// one level down, which is the sort of half-fix that reads as done.
		sort.Slice(toolList, func(i, j int) bool { return toolList[i].Name < toolList[j].Name })

		catalog.WriteString(fmt.Sprintf("### %s Tools\n\n", strings.TrimPrefix(affinity, "/")))
		for _, tool := range toolList {
			catalog.WriteString(fmt.Sprintf("**%s**\n", tool.Name))
			if tool.Description != "" {
				catalog.WriteString(fmt.Sprintf("%s\n", tool.Description))
			}
			if len(tool.Capabilities) > 0 {
				catalog.WriteString(fmt.Sprintf("Capabilities: %s\n", strings.Join(tool.Capabilities, ", ")))
			}
			catalog.WriteString("\n")
		}
	}

	// Add tool generation encouragement
	catalog.WriteString("### Missing a Tool?\n\n")
	catalog.WriteString("If you need a capability not available above, request tool generation:\n")
	catalog.WriteString("1. Add a mangle_update: `missing_tool_for(\"<capability>\", \"<description>\")`\n")
	catalog.WriteString("2. The Ouroboros system will generate, compile, and register the tool\n")
	catalog.WriteString("3. The tool will be available in subsequent turns\n\n")

	logging.ToolsDebug("BuildToolCatalog: built catalog with %d tools for shard %s (length=%d)", len(tools), shardType, catalog.Len())
	return catalog.String()
}

// isCommandName checks if a string is a command name (not a path)
// secureValidateCommand checks for common command injection and traversal vectors
func secureValidateCommand(command string) error {
	if strings.Contains(command, "..") {
		return fmt.Errorf("security violation: command path contains traversal components")
	}

	normalized := strings.ReplaceAll(command, "\\", "/")
	base := strings.ToLower(filepath.Base(normalized))
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}

	forbiddenShells := []string{"sh", "bash", "zsh", "cmd", "powershell", "pwsh", "csh", "ksh", "dash"}
	for _, shell := range forbiddenShells {
		if base == shell {
			return fmt.Errorf("security violation: direct execution of shell (%s) is forbidden", base)
		}
	}
	return nil
}

// secureValidateArgs checks arguments for dangerous payloads
func secureValidateArgs(args []string) error {
	for _, arg := range args {
		if strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("security violation: null byte in argument")
		}
	}
	return nil
}

func isCommandName(s string) bool {
	return !filepath.IsAbs(s) && !strings.Contains(s, string(filepath.Separator))
}

func parseToolInput(input string) []string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return []string{trimmed}
	}
	return strings.Fields(trimmed)
}
