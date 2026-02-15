package chat

import "codenerd/internal/config"

// CommandCategory represents a logical grouping of commands.
type CommandCategory int

const (
	CategoryCore     CommandCategory = iota // Essential commands everyone should know
	CategoryBasic                           // Common operations for regular users
	CategoryAdvanced                        // Powerful features for experienced users
	CategoryExpert                          // Internals and debugging commands
	CategorySystem                          // System administration and config
)

// String returns the category name.
func (c CommandCategory) String() string {
	names := []string{"Core", "Basic", "Advanced", "Expert", "System"}
	if int(c) < len(names) {
		return names[c]
	}
	return "Unknown"
}

// CommandInfo holds metadata about a command.
type CommandInfo struct {
	Name        string          // Primary command name (e.g., "/help")
	Aliases     []string        // Alternative names (e.g., ["/h", "/?"])
	Description string          // Short description
	Usage       string          // Example usage
	Category    CommandCategory // Which category this belongs to
	ShowInHelp  bool            // Whether to show in /help output
}

// CommandRegistry holds all registered commands with their metadata.
var CommandRegistry = []CommandInfo{
	// ==========================================================================
	// CORE COMMANDS - Everyone should know these
	// ==========================================================================
	{
		Name:        "/help",
		Aliases:     []string{"/h", "/?"},
		Description: "Show help and command reference",
		Usage:       "/help [all|core|basic|advanced|expert|<command>]",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/status",
		Description: "Show system status",
		Usage:       "/status",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/scan",
		Description: "Index the codebase",
		Usage:       "/scan [--deep|-d]",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/review",
		Description: "Run code review (optionally multi-shard if specialists exist)",
		Usage:       "/review [path] [--andEnhance] [-- <passthrough flags>]",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/test",
		Description: "Run tests or test tasks (specialist-aware)",
		Usage:       "/test [args...]",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/fix",
		Description: "Fix an issue (specialist-aware)",
		Usage:       "/fix <issue description>",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/continue",
		Aliases:     []string{"/resume"},
		Description: "Resume a paused multi-step task",
		Usage:       "/continue",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/clear",
		Description: "Clear chat history (current session)",
		Usage:       "/clear",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/quit",
		Aliases:     []string{"/exit", "/q"},
		Description: "Exit codeNERD",
		Usage:       "/quit",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},

	// ==========================================================================
	// BASIC COMMANDS - Common operations
	// ==========================================================================
	{
		Name:        "/new-session",
		Description: "Start a fresh session (preserves old)",
		Usage:       "/new-session",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/sessions",
		Description: "List and select saved sessions",
		Usage:       "/sessions",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/load-session",
		Description: "Load a specific session by ID",
		Usage:       "/load-session <session-id>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/read",
		Description: "Read a file",
		Usage:       "/read <path> [start:end]",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/mkdir",
		Description: "Create a directory",
		Usage:       "/mkdir <path>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/write",
		Description: "Write a file",
		Usage:       "/write <path> <content>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/edit",
		Description: "Show a file for inline editing",
		Usage:       "/edit <path>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/append",
		Description: "Append to a file",
		Usage:       "/append <path> <content>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/search",
		Description: "Search the codebase",
		Usage:       "/search <pattern>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/patch",
		Description: "Enter patch ingestion mode",
		Usage:       "/patch",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/pick",
		Description: "Open file picker (inserts /read ...)",
		Usage:       "/pick",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/knowledge",
		Description: "View recent knowledge pulls or search persisted knowledge",
		Usage:       "/knowledge [<n>|search <query>]",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/reflection",
		Description: "Show System 2 reflection recall status",
		Usage:       "/reflection",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/usage",
		Description: "Show token usage dashboard",
		Usage:       "/usage",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/agents",
		Description: "List defined agents",
		Usage:       "/agents",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/define-agent",
		Aliases:     []string{"/agent"},
		Description: "Define a new specialist agent (wizard)",
		Usage:       "/define-agent",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/spawn",
		Description: "Spawn a shard or defined agent",
		Usage:       "/spawn <type|agent> <task>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/ingest",
		Description: "Ingest docs into an agent knowledge base",
		Usage:       "/ingest <agent> <path>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},

	// ==========================================================================
	// ADVANCED COMMANDS - Power features
	// ==========================================================================
	{
		Name:        "/refactor",
		Description: "Refactor code (specialist-aware)",
		Usage:       "/refactor <target>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/security",
		Description: "Security-focused analysis",
		Usage:       "/security [path]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/analyze",
		Description: "Complexity/static analysis",
		Usage:       "/analyze [path]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/northstar",
		Aliases:     []string{"/vision", "/spec"},
		Description: "Project vision/spec wizard (Northstar)",
		Usage:       "/northstar",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/alignment",
		Aliases:     []string{"/align"},
		Description: "Run on-demand Northstar alignment check",
		Usage:       "/alignment [subject]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/campaign",
		Description: "Manage campaigns",
		Usage:       "/campaign <start|assault|status|pause|resume|list> [args]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/launchcampaign",
		Description: "Clarify and auto-start a hands-free campaign",
		Usage:       "/launchcampaign <goal>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/clarify",
		Description: "Socratic requirements interrogation",
		Usage:       "/clarify <goal>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/legislate",
		Description: "Synthesize and ratify a safety rule",
		Usage:       "/legislate <constraint>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/query",
		Description: "Query Mangle facts",
		Usage:       "/query <predicate>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/why",
		Description: "Explain why a fact was derived",
		Usage:       "/why <fact>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/shadow",
		Description: "Simulate action without executing",
		Usage:       "/shadow <action>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/whatif",
		Description: "Counterfactual analysis",
		Usage:       "/whatif <change>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/transparency",
		Description: "Toggle transparency mode (phases, safety explanations, verbose errors)",
		Usage:       "/transparency [on|off]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/approve",
		Description: "Approve pending changes",
		Usage:       "/approve",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/reject-finding",
		Description: "Mark a reviewer finding as false positive",
		Usage:       "/reject-finding <file>:<line> <reason>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/accept-finding",
		Description: "Confirm a reviewer finding is valid",
		Usage:       "/accept-finding <file>:<line>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/review-accuracy",
		Description: "Show reviewer accuracy report",
		Usage:       "/review-accuracy",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},

	// ==========================================================================
	// EXPERT COMMANDS - Internals and debugging
	// ==========================================================================
	{
		Name:        "/logic",
		Description: "Show recent kernel facts (logic snapshot)",
		Usage:       "/logic",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/glassbox",
		Description: "Toggle Glass Box debug mode (inline system visibility)",
		Usage:       "/glassbox [status|verbose|<category>]",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/tool",
		Description: "Manage generated tools",
		Usage:       "/tool <list|run|info|generate> [args]",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/jit",
		Description: "Show JIT prompt compiler status",
		Usage:       "/jit",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/cleanup-tools",
		Description: "Clean up tool execution artifacts",
		Usage:       "/cleanup-tools [args...]",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/learn",
		Description: "Run meta-learning on recent turns (taxonomy critic)",
		Usage:       "/learn",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/evolve",
		Description: "Trigger manual prompt evolution cycle",
		Usage:       "/evolve",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/evolution-stats",
		Description: "Show prompt evolution statistics",
		Usage:       "/evolution-stats",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/evolved-atoms",
		Description: "List evolved prompt atoms",
		Usage:       "/evolved-atoms",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/strategies",
		Description: "Show strategy database",
		Usage:       "/strategies",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/promote-atom",
		Description: "Promote a pending evolved atom to corpus",
		Usage:       "/promote-atom <atom-id>",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/reject-atom",
		Description: "Reject a pending evolved atom",
		Usage:       "/reject-atom <atom-id>",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},

	// ==========================================================================
	// SYSTEM COMMANDS - Configuration and administration
	// ==========================================================================
	{
		Name:        "/config",
		Description: "Configure codeNERD",
		Usage:       "/config [wizard|show|set-key|set-theme|engine]",
		Category:    CategorySystem,
		ShowInHelp:  true,
	},
	{
		Name:        "/init",
		Description: "Initialize workspace",
		Usage:       "/init [--force]",
		Category:    CategorySystem,
		ShowInHelp:  true,
	},
	{
		Name:        "/embedding",
		Description: "Embedding commands",
		Usage:       "/embedding <set|stats|reembed> [args]",
		Category:    CategorySystem,
		ShowInHelp:  true,
	},
	{
		Name:        "/refresh-docs",
		Aliases:     []string{"/scan-docs"},
		Description: "Refresh strategic knowledge from docs",
		Usage:       "/refresh-docs [--force|-f]",
		Category:    CategorySystem,
		ShowInHelp:  true,
	},
	{
		Name:        "/scan-path",
		Description: "Scan specific file paths and update kernel facts",
		Usage:       "/scan-path <file1>[,<file2>...]",
		Category:    CategorySystem,
		ShowInHelp:  true,
	},
	{
		Name:        "/scan-dir",
		Description: "Scan a directory and update kernel facts",
		Usage:       "/scan-dir <directory>",
		Category:    CategorySystem,
		ShowInHelp:  true,
	},
	{
		Name:        "/reset",
		Description: "Reset kernel facts (policy and schemas retained)",
		Usage:       "/reset",
		Category:    CategorySystem,
		ShowInHelp:  true,
	},
}

// GetCommandsByCategory returns commands filtered by category.
func GetCommandsByCategory(category CommandCategory) []CommandInfo {
	var result []CommandInfo
	for _, cmd := range CommandRegistry {
		if cmd.Category == category && cmd.ShowInHelp {
			result = append(result, cmd)
		}
	}
	return result
}

// GetCommandsForLevel returns commands appropriate for the user's experience level.
func GetCommandsForLevel(level config.ExperienceLevel) []CommandInfo {
	var result []CommandInfo

	// Always include Core commands
	result = append(result, GetCommandsByCategory(CategoryCore)...)

	// Add more categories based on level
	switch level {
	case config.ExperienceExpert:
		result = append(result, GetCommandsByCategory(CategoryExpert)...)
		result = append(result, GetCommandsByCategory(CategorySystem)...)
		fallthrough
	case config.ExperienceAdvanced:
		result = append(result, GetCommandsByCategory(CategoryAdvanced)...)
		fallthrough
	case config.ExperienceIntermediate:
		result = append(result, GetCommandsByCategory(CategoryBasic)...)
	case config.ExperienceBeginner:
		// Only Core commands for beginners
	}

	return result
}

// FindCommand looks up a command by name or alias.
func FindCommand(name string) *CommandInfo {
	for i := range CommandRegistry {
		cmd := &CommandRegistry[i]
		if cmd.Name == name {
			return cmd
		}
		for _, alias := range cmd.Aliases {
			if alias == name {
				return cmd
			}
		}
	}
	return nil
}

// GetAllCategories returns all command categories with their commands.
func GetAllCategories() map[CommandCategory][]CommandInfo {
	result := make(map[CommandCategory][]CommandInfo)
	for _, cmd := range CommandRegistry {
		if cmd.ShowInHelp {
			result[cmd.Category] = append(result[cmd.Category], cmd)
		}
	}
	return result
}
