package chat

import (
	"fmt"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/ux"
)

// HelpRenderer generates help text based on user experience level.
type HelpRenderer struct {
	experienceLevel config.ExperienceLevel
	workspace       string
}

// NewHelpRenderer creates a new help renderer for the given workspace.
func NewHelpRenderer(workspace string) *HelpRenderer {
	// Load experience level from preferences
	pm := ux.NewPreferencesManager(workspace)
	level := config.ExperienceBeginner
	if err := pm.Load(); err == nil {
		journeyState := pm.GetJourneyState()
		switch journeyState {
		case ux.StatePower:
			level = config.ExperienceExpert
		case ux.StateProductive:
			level = config.ExperienceAdvanced
		case ux.StateLearning:
			level = config.ExperienceIntermediate
		default:
			level = config.ExperienceBeginner
		}
	}

	return &HelpRenderer{
		experienceLevel: level,
		workspace:       workspace,
	}
}

// RenderHelp generates help text appropriate for the user's level.
// If arg is empty, shows default help. Otherwise can be: all, basic, advanced, expert, or a command name.
func (r *HelpRenderer) RenderHelp(arg string) string {
	arg = strings.TrimSpace(strings.ToLower(arg))

	// Handle specific command lookup
	if arg != "" && arg != "all" && arg != "basic" && arg != "advanced" && arg != "expert" && arg != "core" {
		if cmd := FindCommand("/" + strings.TrimPrefix(arg, "/")); cmd != nil {
			return r.renderCommandDetails(cmd)
		}
	}

	// Handle category-specific help
	switch arg {
	case "all":
		return r.renderAllCommands()
	case "core":
		return r.renderCategory(CategoryCore)
	case "basic":
		return r.renderCategory(CategoryBasic)
	case "advanced":
		return r.renderCategory(CategoryAdvanced)
	case "expert":
		return r.renderCategories(CategoryExpert, CategorySystem)
	default:
		return r.renderProgressiveHelp()
	}
}

// renderProgressiveHelp shows help based on experience level.
func (r *HelpRenderer) renderProgressiveHelp() string {
	var sb strings.Builder

	switch r.experienceLevel {
	case config.ExperienceBeginner:
		sb.WriteString("## Getting Started\n\n")
		sb.WriteString("codeNERD understands natural language - just type what you want!\n\n")
		sb.WriteString("### Essential Commands\n\n")
		sb.WriteString(r.renderCommandTable(GetCommandsByCategory(CategoryCore)))
		sb.WriteString("\n### Tips\n\n")
		sb.WriteString("- Type naturally: \"Fix the bug in main.go\"\n")
		sb.WriteString("- Ask questions: \"What does this function do?\"\n")
		sb.WriteString("- Get more help: `/help all` to see all commands\n")

	case config.ExperienceIntermediate:
		sb.WriteString("## Command Reference\n\n")
		sb.WriteString("### Core Commands\n")
		sb.WriteString(r.renderCommandTable(GetCommandsByCategory(CategoryCore)))
		sb.WriteString("\n### Basic Commands\n")
		sb.WriteString(r.renderCommandTable(GetCommandsByCategory(CategoryBasic)))
		sb.WriteString("\n*Type `/help advanced` for power features*\n")

	case config.ExperienceAdvanced:
		sb.WriteString("## Command Reference\n\n")
		sb.WriteString("### Core\n")
		sb.WriteString(r.renderCommandTable(GetCommandsByCategory(CategoryCore)))
		sb.WriteString("\n### Basic\n")
		sb.WriteString(r.renderCommandTable(GetCommandsByCategory(CategoryBasic)))
		sb.WriteString("\n### Advanced\n")
		sb.WriteString(r.renderCommandTable(GetCommandsByCategory(CategoryAdvanced)))
		sb.WriteString("\n*Type `/help expert` for internals*\n")

	case config.ExperienceExpert:
		sb.WriteString("## Full Command Reference\n\n")
		categories := GetAllCategories()
		for _, cat := range []CommandCategory{CategoryCore, CategoryBasic, CategoryAdvanced, CategoryExpert, CategorySystem} {
			if cmds, ok := categories[cat]; ok && len(cmds) > 0 {
				sb.WriteString(fmt.Sprintf("### %s\n", cat.String()))
				sb.WriteString(r.renderCommandTable(cmds))
				sb.WriteString("\n")
			}
		}

	default:
		// Fallback to basic help
		sb.WriteString("## Commands\n\n")
		sb.WriteString(r.renderCommandTable(GetCommandsByCategory(CategoryCore)))
		sb.WriteString("\n*Type `/help all` for all commands*\n")
	}

	// Add keyboard shortcuts for all levels
	sb.WriteString("\n### Keyboard Shortcuts\n\n")
	sb.WriteString("| Key | Action |\n|-----|--------|\n")
	sb.WriteString("| `Ctrl+C` | Exit |\n")
	sb.WriteString("| `Ctrl+X` | Stop current operation |\n")
	sb.WriteString("| `Shift+Tab` | Cycle continuation mode |\n")
	sb.WriteString("| `Alt+L` | Toggle logic pane |\n")
	sb.WriteString("| `Alt+M` | Toggle mouse capture |\n")

	return sb.String()
}

// renderAllCommands shows all commands grouped by category.
func (r *HelpRenderer) renderAllCommands() string {
	var sb strings.Builder
	sb.WriteString("## All Commands\n\n")

	categories := GetAllCategories()
	for _, cat := range []CommandCategory{CategoryCore, CategoryBasic, CategoryAdvanced, CategoryExpert, CategorySystem} {
		if cmds, ok := categories[cat]; ok && len(cmds) > 0 {
			sb.WriteString(fmt.Sprintf("### %s\n\n", cat.String()))
			sb.WriteString(r.renderCommandTable(cmds))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderCategory shows commands for a specific category.
func (r *HelpRenderer) renderCategory(cat CommandCategory) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s Commands\n\n", cat.String()))
	sb.WriteString(r.renderCommandTable(GetCommandsByCategory(cat)))
	return sb.String()
}

// renderCategories shows commands for multiple categories.
func (r *HelpRenderer) renderCategories(cats ...CommandCategory) string {
	var sb strings.Builder
	for _, cat := range cats {
		cmds := GetCommandsByCategory(cat)
		if len(cmds) > 0 {
			sb.WriteString(fmt.Sprintf("### %s\n\n", cat.String()))
			sb.WriteString(r.renderCommandTable(cmds))
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// renderCommandTable renders a markdown table of commands.
func (r *HelpRenderer) renderCommandTable(cmds []CommandInfo) string {
	if len(cmds) == 0 {
		return "_No commands in this category_\n"
	}

	var sb strings.Builder
	sb.WriteString("| Command | Description |\n")
	sb.WriteString("|---------|-------------|\n")

	for _, cmd := range cmds {
		name := cmd.Name
		if len(cmd.Aliases) > 0 {
			name = fmt.Sprintf("%s (%s)", cmd.Name, strings.Join(cmd.Aliases, ", "))
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", name, cmd.Description))
	}

	return sb.String()
}

// renderCommandDetails shows detailed help for a specific command.
func (r *HelpRenderer) renderCommandDetails(cmd *CommandInfo) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## %s\n\n", cmd.Name))
	sb.WriteString(fmt.Sprintf("**%s**\n\n", cmd.Description))

	if len(cmd.Aliases) > 0 {
		sb.WriteString(fmt.Sprintf("**Aliases**: %s\n\n", strings.Join(cmd.Aliases, ", ")))
	}

	sb.WriteString(fmt.Sprintf("**Usage**: `%s`\n\n", cmd.Usage))
	sb.WriteString(fmt.Sprintf("**Category**: %s\n", cmd.Category.String()))

	return sb.String()
}

// GetCurrentLevel returns the current experience level.
func (r *HelpRenderer) GetCurrentLevel() config.ExperienceLevel {
	return r.experienceLevel
}

// SetLevel sets the experience level (useful for testing).
func (r *HelpRenderer) SetLevel(level config.ExperienceLevel) {
	r.experienceLevel = level
}
