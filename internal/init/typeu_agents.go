// Package init implements the "nerd init" cold-start initialization system.
// This file contains Type U agent support for user-defined agents at init time.
package init

import (
	"fmt"
	"strings"
)

// TypeUAgentDefinition represents a user-defined agent from --define-agent flag.
// Format: "Name:role:topic1,topic2,topic3"
type TypeUAgentDefinition struct {
	Name   string   // Agent name (alphanumeric, no spaces)
	Role   string   // Role description (max 100 chars)
	Topics []string // Research topics (1-10 topics)
}

// TypeUAgentError represents a validation error for a Type U agent definition.
type TypeUAgentError struct {
	Flag    string // Original flag value
	Field   string // Field that failed validation
	Message string // Error message
}

func (e *TypeUAgentError) Error() string {
	return fmt.Sprintf("invalid --define-agent %q: %s - %s", e.Flag, e.Field, e.Message)
}

// ParseTypeUAgentFlag parses a --define-agent flag value into a TypeUAgentDefinition.
// Expected format: "Name:role:topic1,topic2,topic3"
// Example: "K8sExpert:Kubernetes deployment specialist:kubernetes,helm,kubectl"
func ParseTypeUAgentFlag(flagValue string) (*TypeUAgentDefinition, error) {
	if flagValue == "" {
		return nil, &TypeUAgentError{
			Flag:    flagValue,
			Field:   "flag",
			Message: "empty value",
		}
	}

	// Split by colon - expect exactly 3 parts
	parts := strings.SplitN(flagValue, ":", 3)
	if len(parts) != 3 {
		return nil, &TypeUAgentError{
			Flag:    flagValue,
			Field:   "format",
			Message: "expected format 'Name:role:topic1,topic2,...'",
		}
	}

	name := strings.TrimSpace(parts[0])
	role := strings.TrimSpace(parts[1])
	topicsRaw := strings.TrimSpace(parts[2])

	// Parse topics (comma-separated)
	var topics []string
	for _, t := range strings.Split(topicsRaw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			topics = append(topics, t)
		}
	}

	def := &TypeUAgentDefinition{
		Name:   name,
		Role:   role,
		Topics: topics,
	}

	// Validate the definition
	if err := ValidateTypeUAgentDefinition(def, flagValue); err != nil {
		return nil, err
	}

	return def, nil
}

// ParseTypeUAgentFlags parses multiple --define-agent flag values.
// Returns all valid definitions and any errors encountered.
func ParseTypeUAgentFlags(flagValues []string) ([]TypeUAgentDefinition, []error) {
	var defs []TypeUAgentDefinition
	var errs []error

	for _, flagValue := range flagValues {
		def, err := ParseTypeUAgentFlag(flagValue)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		defs = append(defs, *def)
	}

	return defs, errs
}

// ValidateTypeUAgentDefinition validates a Type U agent definition.
// Returns an error if validation fails.
func ValidateTypeUAgentDefinition(def *TypeUAgentDefinition, originalFlag string) error {
	// Name validation: must be alphanumeric (no spaces)
	if def.Name == "" {
		return &TypeUAgentError{
			Flag:    originalFlag,
			Field:   "name",
			Message: "name cannot be empty",
		}
	}
	for _, r := range def.Name {
		if !isAlphanumericRune(r) {
			return &TypeUAgentError{
				Flag:    originalFlag,
				Field:   "name",
				Message: "name must be alphanumeric (no spaces or special characters)",
			}
		}
	}

	// Role validation: max 100 chars
	if def.Role == "" {
		return &TypeUAgentError{
			Flag:    originalFlag,
			Field:   "role",
			Message: "role description cannot be empty",
		}
	}
	if len(def.Role) > 100 {
		return &TypeUAgentError{
			Flag:    originalFlag,
			Field:   "role",
			Message: fmt.Sprintf("role description exceeds 100 characters (got %d)", len(def.Role)),
		}
	}

	// Topics validation: at least 1, max 10
	if len(def.Topics) == 0 {
		return &TypeUAgentError{
			Flag:    originalFlag,
			Field:   "topics",
			Message: "at least 1 topic is required",
		}
	}
	if len(def.Topics) > 10 {
		return &TypeUAgentError{
			Flag:    originalFlag,
			Field:   "topics",
			Message: fmt.Sprintf("maximum 10 topics allowed (got %d)", len(def.Topics)),
		}
	}

	return nil
}

// isAlphanumericRune checks if a rune is alphanumeric (a-z, A-Z, 0-9).
func isAlphanumericRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// ToRecommendedAgent converts a TypeUAgentDefinition to a RecommendedAgent.
// Type U agents are marked as "user" type and given default permissions.
func (def *TypeUAgentDefinition) ToRecommendedAgent() RecommendedAgent {
	return RecommendedAgent{
		Name:        def.Name,
		Type:        "user", // Type U agents are marked as "user" type
		Description: def.Role,
		Topics:      def.Topics,
		Permissions: []string{"read_file", "code_graph", "exec_cmd"}, // Default permissions
		Priority:    50,                                              // Lower than auto-detected agents
		Reason:      "User-defined agent created via --define-agent",
	}
}

// TypeUAgentsToRecommended converts a slice of TypeUAgentDefinitions to RecommendedAgents.
func TypeUAgentsToRecommended(defs []TypeUAgentDefinition) []RecommendedAgent {
	agents := make([]RecommendedAgent, 0, len(defs))
	for _, def := range defs {
		agents = append(agents, def.ToRecommendedAgent())
	}
	return agents
}
