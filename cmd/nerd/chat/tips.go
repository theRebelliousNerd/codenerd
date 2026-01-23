package chat

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/ux"
)

// TipContext represents the current context for generating tips.
type TipContext struct {
	LastCommand     string   // Last command executed
	LastShardType   string   // Last shard that ran (coder, tester, reviewer, researcher)
	ErrorOccurred   bool     // Whether an error just occurred
	FilesMentioned  []string // Files mentioned in recent conversation
	TestsFailed     bool     // Whether tests just failed
	ReviewIssues    int      // Number of review issues found
	JourneyState    ux.UserJourneyState
	ExperienceLevel config.ExperienceLevel
}

// ContextualTip represents a tip to show the user.
type ContextualTip struct {
	Text     string // The tip text
	Command  string // Optional command to suggest
	Priority int    // Higher = more relevant (0-100)
}

// TipGenerator generates contextual tips based on user actions.
type TipGenerator struct {
	workspace       string
	experienceLevel config.ExperienceLevel
	journeyState    ux.UserJourneyState
	lastTipTime     time.Time
	shownTips       map[string]int // Track which tips have been shown (tip hash -> count)
}

// NewTipGenerator creates a new tip generator.
func NewTipGenerator(workspace string) *TipGenerator {
	pm := ux.NewPreferencesManager(workspace)
	level := config.ExperienceBeginner
	state := ux.StateNew
	if err := pm.Load(); err == nil {
		state = pm.GetJourneyState()
		switch state {
		case ux.StatePower:
			level = config.ExperienceExpert
		case ux.StateProductive:
			level = config.ExperienceAdvanced
		case ux.StateLearning:
			level = config.ExperienceIntermediate
		}
	}

	return &TipGenerator{
		workspace:       workspace,
		experienceLevel: level,
		journeyState:    state,
		shownTips:       make(map[string]int),
	}
}

// ShouldShowTip returns true if we should show a tip now.
func (g *TipGenerator) ShouldShowTip() bool {
	// Don't show tips to power users unless explicitly requested
	if g.journeyState == ux.StatePower {
		return false
	}

	// Rate limit: don't show tips more than once per minute
	if time.Since(g.lastTipTime) < time.Minute {
		return false
	}

	// Random chance (30% for beginners, 15% for others)
	threshold := 0.15
	if g.journeyState == ux.StateNew || g.experienceLevel == config.ExperienceBeginner {
		threshold = 0.30
	}

	return rand.Float64() < threshold
}

// GenerateTip generates a contextual tip based on the current context.
func (g *TipGenerator) GenerateTip(ctx TipContext) *ContextualTip {
	tips := g.gatherRelevantTips(ctx)
	if len(tips) == 0 {
		return nil
	}

	// Sort by priority and pick the best one (with some randomness)
	best := tips[0]
	for _, tip := range tips[1:] {
		if tip.Priority > best.Priority {
			best = tip
		}
	}

	// Mark as shown
	g.shownTips[best.Text]++
	g.lastTipTime = time.Now()

	return &best
}

// gatherRelevantTips collects tips relevant to the current context.
func (g *TipGenerator) gatherRelevantTips(ctx TipContext) []ContextualTip {
	var tips []ContextualTip

	// Context-specific tips based on last action
	switch {
	case ctx.ErrorOccurred:
		tips = append(tips, g.errorRecoveryTips()...)

	case ctx.TestsFailed:
		tips = append(tips, ContextualTip{
			Text:     "Tests failed? Use `/fix` to automatically diagnose and repair the issue.",
			Command:  "/fix",
			Priority: 80,
		})

	case ctx.ReviewIssues > 0:
		tips = append(tips, ContextualTip{
			Text:     fmt.Sprintf("Found %d review issues. Use `/accept-finding <id>` or `/reject-finding <id>` to process them.", ctx.ReviewIssues),
			Priority: 70,
		})

	case ctx.LastCommand == "/scan":
		tips = append(tips, ContextualTip{
			Text:     "Scan complete! Try `/query file_topology` to see indexed files.",
			Command:  "/query file_topology",
			Priority: 60,
		})

	case ctx.LastShardType == "reviewer":
		tips = append(tips, ContextualTip{
			Text:     "After a review, use `/why` to understand how codeNERD reached its conclusions.",
			Command:  "/why",
			Priority: 50,
		})

	case ctx.LastShardType == "coder":
		tips = append(tips, ContextualTip{
			Text:     "Just wrote code? Run `/test` to verify it works correctly.",
			Command:  "/test",
			Priority: 60,
		})

	case ctx.LastShardType == "researcher":
		tips = append(tips, ContextualTip{
			Text:     "Research complete! The knowledge is now available for future questions.",
			Priority: 40,
		})
	}

	// Experience-level specific tips
	tips = append(tips, g.levelSpecificTips(ctx)...)

	// Filter out tips shown too many times
	var filtered []ContextualTip
	for _, tip := range tips {
		if g.shownTips[tip.Text] < 3 {
			filtered = append(filtered, tip)
		}
	}

	return filtered
}

// errorRecoveryTips returns tips for recovering from errors.
func (g *TipGenerator) errorRecoveryTips() []ContextualTip {
	return []ContextualTip{
		{
			Text:     "Encountered an error? Check `/status` for system health.",
			Command:  "/status",
			Priority: 70,
		},
		{
			Text:     "Use `/shadow` to simulate an action before executing it.",
			Command:  "/shadow",
			Priority: 60,
		},
		{
			Text:     "Error messages include remediation steps - check the suggestions!",
			Priority: 50,
		},
	}
}

// levelSpecificTips returns tips appropriate for the user's experience level.
func (g *TipGenerator) levelSpecificTips(ctx TipContext) []ContextualTip {
	// Use ctx for future-proofing
	_ = ctx.LastCommand
	var tips []ContextualTip

	switch g.experienceLevel {
	case config.ExperienceBeginner:
		tips = append(tips, []ContextualTip{
			{
				Text:     "Type naturally! codeNERD understands phrases like \"find all TODO comments\".",
				Priority: 40,
			},
			{
				Text:     "Use `/help` anytime to see available commands.",
				Command:  "/help",
				Priority: 30,
			},
			{
				Text:     "Press `Ctrl+X` to stop any running operation.",
				Priority: 30,
			},
		}...)

	case config.ExperienceIntermediate:
		tips = append(tips, []ContextualTip{
			{
				Text:     "Use `/spawn` to run specialized agents for specific tasks.",
				Command:  "/spawn",
				Priority: 40,
			},
			{
				Text:     "Multi-step tasks? Use `Shift+Tab` to change continuation mode.",
				Priority: 35,
			},
			{
				Text:     "Create custom agents with `/define-agent` for repeated tasks.",
				Command:  "/define-agent",
				Priority: 35,
			},
		}...)

	case config.ExperienceAdvanced:
		tips = append(tips, []ContextualTip{
			{
				Text:     "Toggle `/logic` to see the Glass Box view of Mangle reasoning.",
				Command:  "/logic",
				Priority: 40,
			},
			{
				Text:     "Use `/campaign` for complex, multi-phase goals.",
				Command:  "/campaign",
				Priority: 35,
			},
			{
				Text:     "Inspect prompt compilation with `/jit stats`.",
				Command:  "/jit stats",
				Priority: 30,
			},
		}...)

	case config.ExperienceExpert:
		tips = append(tips, []ContextualTip{
			{
				Text:     "Add custom Mangle rules with `/legislate`.",
				Command:  "/legislate",
				Priority: 30,
			},
			{
				Text:     "Generate custom tools with `/tool generate <description>`.",
				Command:  "/tool generate",
				Priority: 30,
			},
		}...)
	}

	return tips
}

// FormatTip formats a tip for display.
func FormatTip(tip *ContextualTip) string {
	if tip == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("**Tip**: ")
	sb.WriteString(tip.Text)

	if tip.Command != "" {
		sb.WriteString(fmt.Sprintf(" Try: `%s`", tip.Command))
	}

	return sb.String()
}

// GetRandomGenericTip returns a random generic tip (for when there's no specific context).
func GetRandomGenericTip(level config.ExperienceLevel) string {
	var tips []string

	switch level {
	case config.ExperienceBeginner:
		tips = []string{
			"**Tip**: You can ask questions naturally, like \"What does this function do?\"",
			"**Tip**: Use `/review` to get a code review of your project.",
			"**Tip**: Press `Ctrl+C` to exit codeNERD.",
			"**Tip**: Type `/help` to see all available commands.",
		}
	case config.ExperienceIntermediate:
		tips = []string{
			"**Tip**: Use `/research <topic>` to deep-dive into documentation.",
			"**Tip**: Create reusable agents with `/define-agent`.",
			"**Tip**: The `/shadow` command lets you simulate actions safely.",
		}
	case config.ExperienceAdvanced, config.ExperienceExpert:
		tips = []string{
			"**Tip**: Query the Mangle kernel with `/query <predicate>`.",
			"**Tip**: Use `/why <fact>` to see derivation chains.",
			"**Tip**: Toggle the logic pane with `Alt+L`.",
		}
	default:
		tips = []string{
			"**Tip**: Type `/help` to see available commands.",
		}
	}

	if len(tips) == 0 {
		return ""
	}
	return tips[rand.Intn(len(tips))]
}
