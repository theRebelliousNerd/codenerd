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
		Description: "Show available commands",
		Usage:       "/help [command|all|basic|advanced|expert]",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/scan",
		Description: "Index the codebase",
		Usage:       "/scan [--deep]",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/review",
		Description: "Code review + security scan",
		Usage:       "/review [file]",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/test",
		Description: "Run and analyze tests",
		Usage:       "/test [pattern]",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/fix",
		Description: "Fix issues in code",
		Usage:       "/fix <description>",
		Category:    CategoryCore,
		ShowInHelp:  true,
	},
	{
		Name:        "/clear",
		Description: "Clear chat history",
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
		Name:        "/read",
		Description: "Read file contents",
		Usage:       "/read <file> [start:end]",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/write",
		Description: "Write content to file",
		Usage:       "/write <file>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/edit",
		Description: "Edit a file",
		Usage:       "/edit <file> [line]",
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
		Name:        "/refactor",
		Description: "Refactor code",
		Usage:       "/refactor <description>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/status",
		Description: "Show system status",
		Usage:       "/status",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/agents",
		Description: "List available agents",
		Usage:       "/agents",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/spawn",
		Description: "Spawn an agent",
		Usage:       "/spawn <agent> <task>",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/usage",
		Description: "Show token usage",
		Usage:       "/usage",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},
	{
		Name:        "/sessions",
		Description: "Manage sessions",
		Usage:       "/sessions [list|restore <id>]",
		Category:    CategoryBasic,
		ShowInHelp:  true,
	},

	// ==========================================================================
	// ADVANCED COMMANDS - Power features
	// ==========================================================================
	{
		Name:        "/query",
		Description: "Query Mangle facts",
		Usage:       "/query <predicate> [args...]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/why",
		Description: "Explain reasoning chain",
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
		Description: "Explore hypothetical scenarios",
		Usage:       "/whatif <scenario>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/campaign",
		Description: "Manage multi-phase goals",
		Usage:       "/campaign [status|pause|resume]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/launchcampaign",
		Description: "Start a new campaign",
		Usage:       "/launchcampaign <goal>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/research",
		Aliases:     []string{"/ingest"},
		Description: "Deep research on a topic",
		Usage:       "/research <topic>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/define-agent",
		Aliases:     []string{"/agent"},
		Description: "Define a custom agent",
		Usage:       "/define-agent <name>",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/learn",
		Description: "Teach a preference",
		Usage:       "/learn \"<pattern>\"",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/security",
		Description: "Security-focused analysis",
		Usage:       "/security [file]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/analyze",
		Description: "Static analysis",
		Usage:       "/analyze [file]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},

	// ==========================================================================
	// EXPERT COMMANDS - Internals and debugging
	// ==========================================================================
	{
		Name:        "/logic",
		Description: "Toggle logic pane (Glass Box)",
		Usage:       "/logic [on|off]",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/jit",
		Description: "Inspect JIT prompt compiler",
		Usage:       "/jit [stats|atoms|last]",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/tool",
		Description: "Manage generated tools",
		Usage:       "/tool [list|generate|info <name>]",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/legislate",
		Description: "Add custom Mangle rules",
		Usage:       "/legislate <rule>",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/northstar",
		Aliases:     []string{"/vision", "/spec"},
		Description: "Set project north star",
		Usage:       "/northstar <doc_path>",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/alignment",
		Aliases:     []string{"/align"},
		Description: "Check alignment with project vision",
		Usage:       "/alignment [subject]",
		Category:    CategoryAdvanced,
		ShowInHelp:  true,
	},
	{
		Name:        "/approve",
		Description: "Approve a pending action",
		Usage:       "/approve",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/accept-finding",
		Description: "Accept a reviewer finding",
		Usage:       "/accept-finding <id>",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},
	{
		Name:        "/reject-finding",
		Description: "Reject a reviewer finding",
		Usage:       "/reject-finding <id>",
		Category:    CategoryExpert,
		ShowInHelp:  true,
	},

	// ==========================================================================
	// SYSTEM COMMANDS - Configuration and administration
	// ==========================================================================
	{
		Name:        "/config",
		Description: "Configure codeNERD",
		Usage:       "/config [wizard|show|set-model]",
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
		Description: "Embedding configuration",
		Usage:       "/embedding [status|test|reembed]",
		Category:    CategorySystem,
		ShowInHelp:  true,
	},
	{
		Name:        "/new-session",
		Description: "Start fresh session",
		Usage:       "/new-session",
		Category:    CategorySystem,
		ShowInHelp:  true,
	},
	{
		Name:        "/transparency",
		Description: "Toggle transparency mode",
		Usage:       "/transparency [on|off]",
		Category:    CategoryAdvanced,
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
