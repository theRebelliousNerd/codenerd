package init

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/prompt"
	"codenerd/internal/regression"
	"codenerd/internal/store"
	"codenerd/internal/world"
)

// initPhases is every progress phase in run order. The ETA tracker, the
// progress events and the phase numbers all key on these IDs.
var initPhases = []string{
	"migration", "directory", "scanning", "orientation", "profile",
	"facts", "prompt_atoms", "prompt_db", "agents", "kb_creation",
	"project_tools", "preferences", "session", "tools", "registry",
	"prompt_sync", "complete",
}

type phaseRunner struct {
	i               *Initializer
	remainingPhases []string
	phaseNum        int
}

func newPhaseRunner(i *Initializer) *phaseRunner {
	return &phaseRunner{
		i:               i,
		remainingPhases: append([]string(nil), initPhases...),
		phaseNum:        1,
	}
}

func (p *phaseRunner) advance() {
	if len(p.remainingPhases) > 0 {
		p.remainingPhases = p.remainingPhases[1:]
	}
	p.phaseNum++
}

func (p *phaseRunner) start(id, msg string) {
	p.i.startPhase(p.phaseNum, id, msg)
}

func (p *phaseRunner) complete(string) {
	p.advance()
}

func (i *Initializer) runPhase0Migrations(runner *phaseRunner, result *InitResult) {
	existingNerdDir := filepath.Join(i.config.Workspace, ".nerd")
	if _, statErr := os.Stat(existingNerdDir); statErr == nil {
		runner.start("migration", "Checking database schemas...")
		migrationResults, migErr := store.MigrateAllAgentDBs(existingNerdDir)
		if migErr != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("Migration check failed: %v", migErr))
		} else if len(migrationResults) > 0 {
			for agentName, migResult := range migrationResults {
				if migResult.MigrationsRun > 0 {
					logging.Boot("Migrated %s: v%d → v%d (%d migrations, %d hashes backfilled)",
						agentName, migResult.FromVersion, migResult.ToVersion,
						migResult.MigrationsRun, migResult.HashesComputed)
					fmt.Fprintf(i.output(), "   ✓ Migrated %s database schema (v%d → v%d)\n",
						agentName, migResult.FromVersion, migResult.ToVersion)
				}
			}
		}
		runner.complete("migration")
	} else {
		runner.advance()
	}
}

func (i *Initializer) runPhase1DirectorySetup(runner *phaseRunner, result *InitResult) (string, error) {
	runner.start("directory", "Creating directory structure...")

	nerdDir, err := i.createDirectoryStructure()
	if err != nil {
		return "", fmt.Errorf("failed to create directory structure: %w", err)
	}

	if err := i.createMangleTemplates(nerdDir); err != nil {
		return "", fmt.Errorf("create mangle templates: %w", err)
	}

	fmt.Fprintln(i.output(), "✓ Created .nerd/ directory structure")

	// Seed a starter regression battery. Seed never overwrites, so this is safe
	// under --force too — the same merge-don't-clobber rule the preferences
	// write now follows. A failure here is not worth failing init over: the
	// workspace is usable without a battery, and `nerd regression init` writes
	// the same file on demand.
	if path, created, err := regression.Seed(i.config.Workspace); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to seed regression battery: %v", err))
	} else if created {
		result.FilesCreated = append(result.FilesCreated, path)
		fmt.Fprintln(i.output(), "✓ Seeded regression battery")
	}

	dbPath := filepath.Join(nerdDir, "knowledge.db")
	i.localDB, err = store.NewLocalStore(dbPath)
	if err != nil {
		return "", fmt.Errorf("initialize knowledge database: %w", err)
	}
	if err := i.ensureEmbeddingEngine(); err != nil {
		return "", err
	}
	i.localDB.SetEmbeddingEngine(i.embedEngine)
	result.FilesCreated = append(result.FilesCreated, dbPath)
	fmt.Fprintln(i.output(), "✓ Initialized knowledge database")

	northstarStore, err := northstar.NewStore(nerdDir)
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to initialize Northstar store: %v", err))
	} else if err := northstarStore.Close(); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to close Northstar store: %v", err))
	} else {
		northstarDBPath := filepath.Join(nerdDir, "northstar_knowledge.db")
		result.FilesCreated = append(result.FilesCreated, northstarDBPath)
		fmt.Fprintln(i.output(), "✓ Initialized Northstar vision guardian store")
	}

	runner.complete("directory")
	return nerdDir, nil
}

func (i *Initializer) runPhase2Scanning(ctx context.Context, runner *phaseRunner, result *InitResult) *world.ScanResult {
	runner.start("scanning", "Scanning codebase...")
	fmt.Fprintln(i.output(), "\n📊 Phase 2: Deep Codebase Scan")

	scanResult, err := i.scanner.ScanDirectory(ctx, i.config.Workspace)
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Codebase scan failed: %v", err))
		runner.complete("scanning")
		return nil
	}

	fmt.Fprintf(i.output(), "   Scanned %d files in %d directories\n", scanResult.FileCount, scanResult.DirectoryCount)

	facts := scanResult.ToFacts()
	if len(facts) > 0 {
		if err := i.kernel.LoadFacts(facts); err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("Failed to load scan facts: %v", err))
		}
	}
	runner.complete("scanning")
	return scanResult
}

// runPhase3Orientation measures and orients the repository and drafts the
// north star. A failure here is required: profile, agents and knowledge all
// consume what orientation derives.
func (i *Initializer) runPhase3Orientation(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string) error {
	runner.start("orientation", "Orienting repository history and knowledge...")
	fmt.Fprintln(i.output(), "\n🧭 Phase 3: Orientation")
	if err := i.runOrientation(ctx, nerdDir, result); err != nil {
		result.Failures = append(result.Failures, "Orientation failed: "+err.Error())
		result.LLMMetrics = i.snapshotLLMMetrics()
		return err
	}
	runner.complete("orientation")
	return nil
}

func (i *Initializer) runPhase4Profile(runner *phaseRunner, result *InitResult, nerdDir string, scanResult *world.ScanResult) ProjectProfile {
	runner.start("profile", "Building project profile...")
	fmt.Fprintln(i.output(), "\n📋 Phase 4: Building Project Profile")

	profile := i.buildProjectProfile()
	if scanResult != nil {
		profile.FileCount = scanResult.FileCount
		profile.DirectoryCount = scanResult.DirectoryCount
		profile.TestFileCount = scanResult.TestFileCount
	}
	result.Profile = profile

	profilePath := filepath.Join(nerdDir, "profile.json")
	if err := i.saveProfile(profilePath, profile); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to save profile: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, profilePath)
		fmt.Fprintln(i.output(), "✓ Saved project profile")
	}
	runner.complete("profile")
	return profile
}

func (i *Initializer) runPhase5Facts(runner *phaseRunner, result *InitResult, nerdDir string, profile ProjectProfile) {
	runner.start("facts", "Generating Mangle facts...")
	fmt.Fprintln(i.output(), "\n🧠 Phase 5: Generating Mangle Facts")

	factsPath := filepath.Join(nerdDir, "profile.mg")
	factsCount, err := i.generateFactsFile(factsPath, profile)
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to generate facts: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, factsPath)
		result.FactsGenerated = factsCount
		fmt.Fprintf(i.output(), "✓ Generated %d Mangle facts\n", factsCount)
	}
	runner.complete("facts")
}

func (i *Initializer) runPhase5bPromptAtoms(runner *phaseRunner, result *InitResult, profile ProjectProfile) {
	runner.start("prompt_atoms", "Populating project-specific prompt atoms...")
	fmt.Fprintln(i.output(), "\n📝 Phase 5b: Populating Project-Specific Prompt Atoms")

	if err := i.populateProjectAtoms(profile); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to populate prompt atoms: %v", err))
	}
	runner.complete("prompt_atoms")
}

func (i *Initializer) runPhase5cPromptDB(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("prompt_db", "Initializing prompt corpus database...")
	fmt.Fprintln(i.output(), "\n📦 Phase 5c: Initializing Prompt Corpus Database")

	if err := i.initializePromptDatabase(ctx, nerdDir); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to initialize prompt database: %v", err))
	} else {
		corpusDBPath := filepath.Join(nerdDir, "prompts", "corpus.db")
		result.FilesCreated = append(result.FilesCreated, corpusDBPath)
	}
	runner.complete("prompt_db")
}

func (i *Initializer) runPhase6AnalyzeAgents(ctx context.Context, runner *phaseRunner, result *InitResult, profile ProjectProfile) []RecommendedAgent {
	runner.start("agents", "Analyzing required agents...")
	fmt.Fprintln(i.output(), "\n🤖 Phase 6: Determining Required Type 3 Agents")

	recommendedAgents := i.integrateEcosystem(ctx, result, profile, nil)
	fmt.Fprintf(i.output(), "   Recommended %d Type 3 agents for this project\n", len(recommendedAgents))

	for _, agent := range recommendedAgents {
		fmt.Fprintf(i.output(), "   • %s: %s\n", agent.Name, agent.Reason)
	}

	// --define-agent definitions and interactive curation both change which
	// agents get knowledge bases, so they have to land before phase 7a and
	// before the result records what was recommended.
	recommendedAgents = i.mergeTypeUAgents(recommendedAgents)
	recommendedAgents = i.curateAgents(ctx, recommendedAgents, profile, result)
	result.RecommendedAgents = recommendedAgents

	runner.complete("agents")
	return recommendedAgents
}

func (i *Initializer) runPhase7aCreateAgentKBs(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string, recommendedAgents []RecommendedAgent) {
	if i.config.SkipAgentCreate || len(recommendedAgents) == 0 {
		runner.advance() // Skip kb_creation
		return
	}

	runner.start("kb_creation", "Creating agent knowledge bases...")
	fmt.Fprintln(i.output(), "\n📚 Phase 7a: Creating Agent Knowledge Bases")

	createdAgents, agentKBs := i.createType3Agents(ctx, nerdDir, recommendedAgents, result)
	result.CreatedAgents = createdAgents
	result.AgentKBs = agentKBs

	fmt.Fprintf(i.output(), "   Created %d Type 3 agents with knowledge bases\n", len(createdAgents))
	runner.complete("kb_creation")
}

// runPhase7eProjectTools chooses the project's own commands (Makefile
// targets, package.json scripts) worth exposing to the agent as command tools.
// The planner chooses among commands the repository declares; it cannot invent
// one. Phase 10 writes the choice into available_tools.json.
func (i *Initializer) runPhase7eProjectTools(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string, profile ProjectProfile) {
	runner.start("project_tools", "Choosing project command tools...")
	fmt.Fprintln(i.output(), "\n🛠️  Phase 7e: Choosing Project Command Tools")

	selection, err := i.selectProjectTools(ctx, nerdDir, profile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Project command tools were not chosen: %v", err))
	}
	for _, note := range selection.notes {
		result.Warnings = append(result.Warnings, note)
	}
	i.mu.Lock()
	i.projectTools = selection.tools
	i.mu.Unlock()
	for _, tool := range selection.tools {
		result.ProjectTools = append(result.ProjectTools, tool.Name)
	}
	switch {
	case len(selection.tools) > 0:
		fmt.Fprintf(i.output(), "   ✓ %d project command tools (%s)\n", len(selection.tools), selection.source)
	case len(selection.candidates) == 0:
		fmt.Fprintln(i.output(), "   ⓘ The repository declares no Makefile targets or package scripts")
	default:
		fmt.Fprintf(i.output(), "   ⓘ No project command tools chosen from %d declared commands\n", len(selection.candidates))
	}
	runner.complete("project_tools")
}

func (i *Initializer) runPhase8Preferences(runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("preferences", "Initializing preferences...")
	fmt.Fprintln(i.output(), "\n⚙️ Phase 8: Initializing Preferences")

	preferences := i.initPreferences()
	result.Preferences = preferences

	prefsPath := filepath.Join(nerdDir, "preferences.json")
	effective, err := i.savePreferences(prefsPath, preferences)
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to save preferences: %v", err))
	} else {
		result.Preferences = effective
		result.FilesCreated = append(result.FilesCreated, prefsPath)
		fmt.Fprintln(i.output(), "✓ Initialized preferences")
	}
	runner.complete("preferences")
}

// runPhase9Session seeds session.json for a workspace that has none. An
// existing session belongs to the user's running or resumable session: a
// re-init used to replace it with a fresh ID and zero turns, so the next chat
// could not resume.
func (i *Initializer) runPhase9Session(runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("session", "Creating session state...")

	sessionPath := filepath.Join(nerdDir, "session.json")
	created, err := i.seedSessionState(sessionPath)
	switch {
	case err != nil:
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to init session: %v", err))
	case created:
		result.FilesCreated = append(result.FilesCreated, sessionPath)
	}
	runner.complete("session")
}

// runPhase10Tools writes available_tools.json, the static command tools boot
// registers: the tools of every language the profile measured (the primary
// language and each module's), the framework's, and the project command tools
// phase 7e chose.
func (i *Initializer) runPhase10Tools(runner *phaseRunner, result *InitResult, nerdDir string, profile ProjectProfile) {
	runner.start("tools", "Generating tool definitions...")
	fmt.Fprintln(i.output(), "\n🔧 Phase 10: Generating Tool Definitions")

	tools := GenerateToolsForProject(projectTechnologies(profile))
	i.mu.Lock()
	tools = mergeProjectTools(tools, i.projectTools)
	i.registeredTools = tools
	i.mu.Unlock()
	if err := SaveToolsToFile(nerdDir, tools); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to save tools: %v", err))
	} else {
		toolsFile := filepath.Join(nerdDir, "tools", "available_tools.json")
		result.FilesCreated = append(result.FilesCreated, toolsFile)
		fmt.Fprintf(i.output(), "   ✓ Generated %d tool definitions\n", len(tools))

		categories := make(map[string]int)
		for _, tool := range tools {
			categories[tool.Category]++
		}
		for _, cat := range sortedKeys(categories) {
			fmt.Fprintf(i.output(), "      - %s: %d\n", cat, categories[cat])
		}
	}
	runner.complete("tools")
}

// projectTechnologies are the languages and framework whose tools a project
// needs: a Go service with a package.json front end needs both toolchains.
func projectTechnologies(profile ProjectProfile) []string {
	var tech []string
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || value == "unknown" || seen[value] {
			return
		}
		seen[value] = true
		tech = append(tech, value)
	}
	add(profile.Language)
	for _, module := range profile.Modules {
		add(module.Language)
	}
	add(profile.Framework)
	return tech
}

// runPhase11Registry writes .nerd/agents.json and tells a running session's
// shard manager about the agents this run created. Agents an earlier run or
// the user registered stay in the registry unless the user declined them; an
// agent's tool grants are limited to the tools phase 10 registered, because
// boot refuses an agent outright when it names a tool that is not registered.
func (i *Initializer) runPhase11Registry(runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("registry", "Generating agent registry...")

	existing, err := i.loadExistingAgentRegistry(nerdDir)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Existing agent registry unreadable; only this run's agents are registered: %v", err))
		existing = nil
	}
	declined := map[string]bool{}
	if previous, err := LoadAgentPreferences(i.config.Workspace); err == nil && previous != nil {
		for _, name := range previous.RejectedAgents {
			declined[strings.ToLower(name)] = true
		}
	}
	i.mu.RLock()
	registered := i.registeredTools
	i.mu.RUnlock()

	created, notes := restrictAgentTools(result.CreatedAgents, registered)
	result.CreatedAgents = created
	agents := mergeAgentRegistry(created, existing, declined)
	agents, retainedNotes := restrictAgentTools(agents, registered)
	result.Warnings = append(result.Warnings, uniqueSorted(append(notes, retainedNotes...))...)

	registryPath := filepath.Join(nerdDir, "agents.json")
	if err := i.saveAgentRegistry(registryPath, agents); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to save agent registry: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, registryPath)
	}
	i.registerAgentsWithShardManager(created)
	runner.complete("registry")
}

// mergeAgentRegistry keeps the registry entries this run did not recreate:
// agents from earlier runs and agents the user defined elsewhere, unless the
// user declined them or their knowledge base is gone. This run's agents
// replace entries of the same name.
func mergeAgentRegistry(created []CreatedAgent, existing map[string]CreatedAgent, declined map[string]bool) []CreatedAgent {
	merged := append([]CreatedAgent(nil), created...)
	have := map[string]bool{}
	for _, agent := range created {
		have[strings.ToLower(agent.Name)] = true
	}
	names := make([]string, 0, len(existing))
	for name := range existing {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		agent := existing[name]
		key := strings.ToLower(agent.Name)
		if have[key] || declined[key] {
			continue
		}
		if agent.KnowledgePath != "" {
			if _, err := os.Stat(agent.KnowledgePath); err != nil {
				continue
			}
		}
		have[key] = true
		merged = append(merged, agent)
	}
	return merged
}

// restrictAgentTools drops tool grants (and preferences naming them) for
// tools that are not registered, and says which.
func restrictAgentTools(agents []CreatedAgent, registered []ToolDefinition) ([]CreatedAgent, []string) {
	available := make(map[string]bool, len(registered))
	for _, tool := range registered {
		available[tool.Name] = true
	}
	var notes []string
	out := make([]CreatedAgent, len(agents))
	for index, agent := range agents {
		var tools, dropped []string
		for _, name := range agent.Tools {
			if available[name] {
				tools = append(tools, name)
			} else {
				dropped = append(dropped, name)
			}
		}
		agent.Tools = tools
		if len(agent.ToolPreferences) > 0 {
			prefs := make(map[string]string, len(agent.ToolPreferences))
			for key, name := range agent.ToolPreferences {
				if available[name] {
					prefs[key] = name
				}
			}
			agent.ToolPreferences = prefs
			if len(prefs) == 0 {
				agent.ToolPreferences = nil
			}
		}
		if len(dropped) > 0 {
			notes = append(notes, fmt.Sprintf("%s: tool grant(s) %s dropped; this project registers no such tool",
				agent.Name, strings.Join(dropped, ", ")))
		}
		out[index] = agent
	}
	return out, notes
}

func (i *Initializer) runPhase12PromptSync(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("prompt_sync", "Syncing agent prompts to knowledge DBs...")
	fmt.Fprintln(i.output(), "\n📝 Phase 12: Syncing Agent Prompts")
	if err := initializeDiscoveredAgentStores(ctx, i.config.Workspace); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to initialize discovered expert knowledge: %v", err))
		runner.complete("prompt_sync")
		return
	}

	promptCount, syncErr := prompt.ReloadAllPrompts(ctx, nerdDir, i.embedEngine)
	if syncErr != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to sync agent prompts: %v", syncErr))
		fmt.Fprintf(i.output(), "   ✗ Required failure: %v\n", syncErr)
	} else if promptCount > 0 {
		fmt.Fprintf(i.output(), "   ✓ Synced %d prompt atoms to knowledge DBs\n", promptCount)
		logging.Boot("Synced %d prompt atoms from YAML to knowledge DBs", promptCount)
	} else {
		fmt.Fprintln(i.output(), "   ✓ No prompt atoms to sync")
	}
	runner.complete("prompt_sync")
}

func (i *Initializer) finalizeInitialization(runner *phaseRunner, result *InitResult, startTime time.Time, profile ProjectProfile) (*InitResult, error) {
	result.LLMMetrics = i.snapshotLLMMetrics()
	result.Warnings = append(result.Warnings, degradedEnrichmentWarnings(result.LLMMetrics)...)
	i.captureLogProblems(result)
	validation, validationErr := ValidateAllAgentDBs(result.NerdDir)
	result.Validation = validation
	recordValidationOutcome(result, validation, validationErr)
	result.Success = initializationSucceeded(result)
	result.Duration = time.Since(startTime)

	completionMessage := "Initialization complete!"
	if !result.Success {
		completionMessage = "Initialization completed with required failures"
	} else if result.LLMMetrics.Failed > 0 {
		completionMessage = "Initialization complete with degraded LLM enrichment"
	}
	runner.start("complete", completionMessage)
	runner.complete("complete")

	i.printSummary(result, profile)

	return result, nil
}

// recordValidationOutcome turns the structural validation of the shard
// knowledge bases into required failures. A workspace with no specialist
// knowledge base is valid when init created no specialist; it is a failure
// when the agents init just created left no database behind.
func recordValidationOutcome(result *InitResult, summary *ValidationSummary, err error) {
	if result == nil {
		return
	}
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Knowledge-base validation failed: %v", err))
		return
	}
	if summary == nil {
		result.Failures = append(result.Failures, "Knowledge-base validation returned no result")
		return
	}
	if summary.TotalDBs == 0 {
		if len(result.CreatedAgents) > 0 {
			result.Failures = append(result.Failures, fmt.Sprintf(
				"Knowledge-base validation found no shard databases for %d created agents", len(result.CreatedAgents)))
		}
		return
	}
	if !summary.OverallValid {
		result.Failures = append(result.Failures, fmt.Sprintf(
			"Knowledge-base validation found %d/%d invalid shard databases",
			summary.InvalidDBs, summary.TotalDBs,
		))
	}
}

// createMangleTemplates creates placeholder files for user extensions.
func (i *Initializer) createMangleTemplates(nerdDir string) error {
	mangleDir := filepath.Join(nerdDir, "mangle")

	// extensions.mg - For new schema definitions
	extPath := filepath.Join(mangleDir, "extensions.mg")
	extContent := `# User Schema Extensions
# Define project-specific predicates here.
# These will be loaded AFTER the core schemas.

# Example:
# Decl project_metadata(Key, Value).
# Decl deploy_target(Env, URL).
`
	if err := writeFileIfAbsent(extPath, []byte(extContent), 0644); err != nil {
		return fmt.Errorf("create extensions template: %w", err)
	}

	// policy_overrides.mg - For custom rules
	policyPath := filepath.Join(mangleDir, "policy_overrides.mg")
	policyContent := `# User Policy Overrides
# Define project-specific rules here.
# These can extend or override core behavior.

# Example: Allow deleting .tmp files even if modified
# permitted(Action) :- 
#     action_type(Action, /delete_file),
#     target_path(Action, Path),
#     fn:string_suffix(Path, ".tmp").
`
	if err := writeFileIfAbsent(policyPath, []byte(policyContent), 0644); err != nil {
		return fmt.Errorf("create policy overrides template: %w", err)
	}

	return nil
}

// sortedKeys returns m's keys in order, so printed tallies do not shuffle.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func initializationSucceeded(result *InitResult) bool {
	return result != nil && len(result.Failures) == 0
}

// writeFileIfAbsent atomically creates a seed file without replacing user
// content. O_EXCL closes the check-then-write race during concurrent init runs.
func writeFileIfAbsent(path string, content []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}
