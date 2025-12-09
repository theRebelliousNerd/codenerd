// Package init implements the "nerd init" cold-start initialization system.
// This file contains interactive mode for agent curation during initialization.
package init

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/shards/researcher"
)

// DetectedAgent represents an agent detected during project analysis.
// It contains metadata for interactive selection.
type DetectedAgent struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Description     string            `json:"description"`
	Topics          []string          `json:"topics"`
	Permissions     []string          `json:"permissions"`
	Priority        int               `json:"priority"`
	Reason          string            `json:"reason"`
	Tools           []string          `json:"tools,omitempty"`
	ToolPreferences map[string]string `json:"tool_preferences,omitempty"`

	// Selection metadata
	Recommended  bool   `json:"recommended"`   // true = recommended for this project
	Selected     bool   `json:"selected"`      // true = user selected this agent
	Category     string `json:"category"`      // "core", "language", "framework", "dependency", "optional"
	DetectedBy   string `json:"detected_by"`   // what triggered detection
	Context7Hint string `json:"context7_hint"` // additional context from Context7 research
}

// AgentSelectionPreferences stores user's agent selection preferences.
type AgentSelectionPreferences struct {
	AcceptedAgents        []string  `json:"accepted_agents,omitempty"`
	RejectedAgents        []string  `json:"rejected_agents,omitempty"`
	LastInteractive       time.Time `json:"last_interactive,omitempty"`
	AutoAcceptRecommended bool      `json:"auto_accept_recommended"`
}

// InteractiveConfig holds configuration for interactive agent selection.
type InteractiveConfig struct {
	Reader           *bufio.Reader
	Writer           *os.File
	SkipConfirmation bool
	PreviousPrefs    *AgentSelectionPreferences
}

// DefaultInteractiveConfig returns a default interactive configuration.
func DefaultInteractiveConfig() InteractiveConfig {
	return InteractiveConfig{
		Reader:           bufio.NewReader(os.Stdin),
		Writer:           os.Stdout,
		SkipConfirmation: false,
	}
}

// InteractiveAgentSelection prompts the user to select agents interactively.
func InteractiveAgentSelection(agents []DetectedAgent, config InteractiveConfig) ([]DetectedAgent, error) {
	if len(agents) == 0 {
		return agents, nil
	}

	// Sort agents: recommended first, then by priority
	sortAgentsForDisplay(agents)

	// Display detected agents
	fmt.Fprintln(config.Writer, "\nDetected specialist agents for your project:\n")
	displayAgentList(agents, config.Writer)

	// Prompt for quick selection
	fmt.Fprintln(config.Writer, "")
	fmt.Fprint(config.Writer, "Keep all recommended? (y/n/c for customize): ")

	choice, err := readInput(config.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	choice = strings.ToLower(strings.TrimSpace(choice))

	switch choice {
	case "y", "yes", "":
		// Accept all recommended
		for i := range agents {
			if agents[i].Recommended {
				agents[i].Selected = true
			}
		}
		fmt.Fprintln(config.Writer, "\nAccepted all recommended agents.")
		return filterSelectedAgents(agents), nil

	case "n", "no":
		// Accept only core agents
		for i := range agents {
			if agents[i].Category == "core" {
				agents[i].Selected = true
			} else {
				agents[i].Selected = false
			}
		}
		fmt.Fprintln(config.Writer, "\nKept only core agents.")
		return filterSelectedAgents(agents), nil

	case "c", "customize":
		return customizeAgentSelection(agents, config)

	default:
		fmt.Fprintf(config.Writer, "\nUnrecognized input '%s'. Accepting recommended agents.\n", choice)
		for i := range agents {
			if agents[i].Recommended {
				agents[i].Selected = true
			}
		}
		return filterSelectedAgents(agents), nil
	}
}

// customizeAgentSelection allows the user to toggle individual agents.
func customizeAgentSelection(agents []DetectedAgent, config InteractiveConfig) ([]DetectedAgent, error) {
	fmt.Fprintln(config.Writer, "\nCustomize agent selection:")
	fmt.Fprintln(config.Writer, "Enter agent numbers to toggle (comma-separated), or 'done' to finish.\n")

	// Pre-select recommended
	for i := range agents {
		if agents[i].Recommended {
			agents[i].Selected = true
		}
	}

	// Display numbered list
	displayNumberedList(agents, config.Writer)

	fmt.Fprint(config.Writer, "\nToggle (e.g., '1,3,5') or 'done': ")

	for {
		input, err := readInput(config.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(strings.ToLower(input))

		if input == "done" || input == "d" || input == "" {
			break
		}

		// Parse numbers
		parts := strings.Split(input, ",")
		toggled := []string{}
		for _, p := range parts {
			p = strings.TrimSpace(p)
			var num int
			if _, err := fmt.Sscanf(p, "%d", &num); err == nil {
				if num >= 1 && num <= len(agents) {
					idx := num - 1
					agents[idx].Selected = !agents[idx].Selected
					toggled = append(toggled, agents[idx].Name)
				}
			}
		}

		if len(toggled) > 0 {
			fmt.Fprintf(config.Writer, "Toggled: %s\n", strings.Join(toggled, ", "))
		}

		displayNumberedList(agents, config.Writer)
		fmt.Fprint(config.Writer, "\nToggle more or 'done': ")
	}

	return filterSelectedAgents(agents), nil
}

func displayNumberedList(agents []DetectedAgent, w *os.File) {
	for i, agent := range agents {
		marker := "[ ]"
		if agent.Selected {
			marker = "[x]"
		}
		fmt.Fprintf(w, "  %2d. %s %s\n", i+1, marker, agent.Name)
	}
}

// displayAgentList displays the agent list with selection markers.
func displayAgentList(agents []DetectedAgent, w *os.File) {
	for _, agent := range agents {
		marker := "[ ]"
		if agent.Recommended {
			marker = "[x]"
		}

		tag := "(optional"
		if agent.Recommended {
			tag = "(recommended"
		}

		if agent.DetectedBy != "" {
			tag += " - " + agent.DetectedBy
		}
		tag += ")"

		fmt.Fprintf(w, "  %s %s %s\n", marker, agent.Name, tag)
		fmt.Fprintf(w, "      %s\n", agent.Reason)
	}
}

// sortAgentsForDisplay sorts agents for optimal display order.
func sortAgentsForDisplay(agents []DetectedAgent) {
	sort.SliceStable(agents, func(i, j int) bool {
		// Core agents first
		if agents[i].Category == "core" && agents[j].Category != "core" {
			return true
		}
		if agents[j].Category == "core" && agents[i].Category != "core" {
			return false
		}

		// Recommended before optional
		if agents[i].Recommended && !agents[j].Recommended {
			return true
		}
		if agents[j].Recommended && !agents[i].Recommended {
			return false
		}

		// Higher priority first
		return agents[i].Priority > agents[j].Priority
	})
}

// filterSelectedAgents returns only the agents that are selected.
func filterSelectedAgents(agents []DetectedAgent) []DetectedAgent {
	selected := make([]DetectedAgent, 0, len(agents))
	for _, agent := range agents {
		if agent.Selected {
			selected = append(selected, agent)
		}
	}
	return selected
}

// readInput reads a line of input from the reader.
func readInput(reader *bufio.Reader) (string, error) {
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// ConvertToDetectedAgents converts RecommendedAgent slice to DetectedAgent slice.
func ConvertToDetectedAgents(recommended []RecommendedAgent, profile ProjectProfile) []DetectedAgent {
	detected := make([]DetectedAgent, 0, len(recommended))

	for _, r := range recommended {
		d := DetectedAgent{
			Name:            r.Name,
			Type:            r.Type,
			Description:     r.Description,
			Topics:          r.Topics,
			Permissions:     r.Permissions,
			Priority:        r.Priority,
			Reason:          r.Reason,
			Tools:           r.Tools,
			ToolPreferences: r.ToolPreferences,
		}

		// Determine category and recommendation status
		d.Category, d.Recommended, d.DetectedBy = categorizeAgent(r, profile)
		d.Selected = d.Recommended

		detected = append(detected, d)
	}

	return detected
}

// categorizeAgent determines the category, recommendation status, and detection source.
func categorizeAgent(agent RecommendedAgent, profile ProjectProfile) (category string, recommended bool, detectedBy string) {
	name := strings.ToLower(agent.Name)

	// Core agents are always recommended
	if name == "securityauditor" || name == "testarchitect" {
		return "core", true, "always included"
	}

	// Language-specific agents
	if strings.Contains(name, "goexpert") && (strings.ToLower(profile.Language) == "go" || strings.ToLower(profile.Language) == "golang") {
		return "language", true, "primary language"
	}

	// Framework-specific
	if strings.Contains(name, "bubbletea") && strings.Contains(strings.ToLower(profile.Framework), "bubbletea") {
		return "framework", true, profile.Framework + " framework"
	}

	// Default: optional
	return "optional", false, "detected"
}

// ConvertToRecommendedAgents converts DetectedAgent slice back to RecommendedAgent slice.
func ConvertToRecommendedAgents(detected []DetectedAgent) []RecommendedAgent {
	recommended := make([]RecommendedAgent, 0, len(detected))

	for _, d := range detected {
		r := RecommendedAgent{
			Name:            d.Name,
			Type:            d.Type,
			Description:     d.Description,
			Topics:          d.Topics,
			Permissions:     d.Permissions,
			Priority:        d.Priority,
			Reason:          d.Reason,
			Tools:           d.Tools,
			ToolPreferences: d.ToolPreferences,
		}
		recommended = append(recommended, r)
	}

	return recommended
}

// AgentSuggestion represents an agent suggestion from Context7 research.
type AgentSuggestion struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Topics      []string `json:"topics"`
	Reason      string   `json:"reason"`
	Confidence  float64  `json:"confidence"`
	SourceTopic string   `json:"source_topic"`
}

// GetContext7AgentSuggestions queries Context7 for agent suggestions based on detected frameworks.
func GetContext7AgentSuggestions(ctx context.Context, researchShard *researcher.ResearcherShard, profile ProjectProfile) ([]AgentSuggestion, error) {
	if researchShard == nil {
		return nil, nil
	}

	suggestions := make([]AgentSuggestion, 0)

	// Collect topics to research based on dependencies
	depNames := make(map[string]bool)
	for _, dep := range profile.Dependencies {
		depNames[strings.ToLower(dep.Name)] = true
	}

	// Check for specialized frameworks
	specializedTopics := map[string]AgentSuggestion{
		"htmx": {
			Name:        "HTMXExpert",
			Description: "Expert in HTMX hypermedia patterns and server-driven UI",
			Topics:      []string{"htmx patterns", "htmx best practices", "htmx forms"},
			Confidence:  0.85,
		},
		"graphql": {
			Name:        "GraphQLExpert",
			Description: "Expert in GraphQL schema design and resolvers",
			Topics:      []string{"graphql schema", "graphql resolvers", "graphql mutations"},
			Confidence:  0.85,
		},
		"redis": {
			Name:        "RedisExpert",
			Description: "Expert in Redis caching and data structures",
			Topics:      []string{"redis patterns", "redis caching", "redis pub/sub"},
			Confidence:  0.80,
		},
		"kubernetes": {
			Name:        "K8sExpert",
			Description: "Expert in Kubernetes deployments and orchestration",
			Topics:      []string{"kubernetes patterns", "helm charts", "k8s operators"},
			Confidence:  0.85,
		},
	}

	for depName, suggestion := range specializedTopics {
		if depNames[depName] {
			suggestion.SourceTopic = depName
			suggestion.Reason = fmt.Sprintf("Detected %s dependency", depName)
			suggestions = append(suggestions, suggestion)
			logging.Boot("Context7: Suggesting %s agent for %s", suggestion.Name, depName)
		}
	}

	return suggestions, nil
}

// LoadAgentPreferences loads agent selection preferences from .nerd/preferences.json.
func LoadAgentPreferences(workspace string) (*AgentSelectionPreferences, error) {
	path := filepath.Join(workspace, ".nerd", "preferences.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var wrapper struct {
		AgentSelection *AgentSelectionPreferences `json:"agent_selection,omitempty"`
	}

	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	return wrapper.AgentSelection, nil
}

// SaveAgentPreferences saves agent selection preferences to .nerd/preferences.json.
func SaveAgentPreferences(workspace string, agentPrefs *AgentSelectionPreferences) error {
	path := filepath.Join(workspace, ".nerd", "preferences.json")

	// Load existing preferences
	existingData, _ := os.ReadFile(path)
	var existing map[string]interface{}
	if len(existingData) > 0 {
		json.Unmarshal(existingData, &existing)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Update agent selection
	existing["agent_selection"] = agentPrefs

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
