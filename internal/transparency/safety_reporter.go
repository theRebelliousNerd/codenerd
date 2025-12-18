package transparency

import (
	"fmt"
	"strings"
	"time"
)

// SafetyViolationType categorizes why an action was blocked.
type SafetyViolationType int

const (
	ViolationDestructiveAction SafetyViolationType = iota
	ViolationProtectedPath
	ViolationSecretExposure
	ViolationResourceLimit
	ViolationPolicyRule
	ViolationUnauthorized
	ViolationUnknown
)

// String returns a human-readable name for the violation type.
func (v SafetyViolationType) String() string {
	names := []string{
		"Destructive Action",
		"Protected Path",
		"Secret Exposure",
		"Resource Limit",
		"Policy Rule",
		"Unauthorized",
		"Unknown",
	}
	if int(v) < len(names) {
		return names[v]
	}
	return "Unknown"
}

// SafetyViolation represents a blocked action with context.
type SafetyViolation struct {
	ID            string
	Timestamp     time.Time
	Action        string              // The action that was attempted
	ViolationType SafetyViolationType // Why it was blocked
	Rule          string              // The policy rule that triggered the block
	Target        string              // File/path/resource that was the target
	Summary       string              // Human-readable summary
	Explanation   string              // Detailed explanation
	Remediation   []string            // Steps to proceed safely
}

// SafetyReporter tracks and explains safety gate blocks.
type SafetyReporter struct {
	violations []SafetyViolation
	maxHistory int
	enabled    bool
}

// NewSafetyReporter creates a new safety reporter.
func NewSafetyReporter() *SafetyReporter {
	return &SafetyReporter{
		maxHistory: 50,
		enabled:    true,
	}
}

// Enable enables safety reporting.
func (r *SafetyReporter) Enable() {
	r.enabled = true
}

// Disable disables safety reporting.
func (r *SafetyReporter) Disable() {
	r.enabled = false
}

// ReportViolation records a safety violation.
func (r *SafetyReporter) ReportViolation(action, target, rule string) *SafetyViolation {
	if !r.enabled {
		return nil
	}

	violation := r.classifyViolation(action, target, rule)
	violation.ID = fmt.Sprintf("sv_%d", time.Now().UnixNano())
	violation.Timestamp = time.Now()
	violation.Action = action
	violation.Target = target
	violation.Rule = rule

	r.violations = append(r.violations, violation)
	if len(r.violations) > r.maxHistory {
		r.violations = r.violations[1:]
	}

	return &violation
}

// classifyViolation determines the violation type and generates explanations.
func (r *SafetyReporter) classifyViolation(action, target, rule string) SafetyViolation {
	v := SafetyViolation{}

	actionLower := strings.ToLower(action)
	targetLower := strings.ToLower(target)

	// Classify based on action and target patterns
	switch {
	case strings.Contains(actionLower, "rm") || strings.Contains(actionLower, "delete"):
		v.ViolationType = ViolationDestructiveAction
		v.Summary = "Destructive operation blocked"
		v.Explanation = fmt.Sprintf("The action '%s' was blocked because it could cause irreversible data loss.", action)
		v.Remediation = []string{
			"Use /shadow to simulate the action first",
			"If you're sure, explicitly request override with full confirmation",
			"Consider using git to track changes before destructive operations",
		}

	case containsAnyWord(targetLower, ".env", "credential", "secret", "password", "key", "token"):
		v.ViolationType = ViolationSecretExposure
		v.Summary = "Sensitive file protection"
		v.Explanation = fmt.Sprintf("Access to '%s' was blocked to prevent accidental secret exposure.", target)
		v.Remediation = []string{
			"Ensure secrets are never committed to version control",
			"Use environment variables or secret managers instead",
			"Add sensitive files to .gitignore",
		}

	case containsAnyWord(targetLower, ".git", "node_modules", "__pycache__", "vendor"):
		v.ViolationType = ViolationProtectedPath
		v.Summary = "Protected directory"
		v.Explanation = fmt.Sprintf("Operations on '%s' are restricted to prevent system corruption.", target)
		v.Remediation = []string{
			"Work with source files directly instead of managed directories",
			"Use appropriate package managers for dependencies",
		}

	case strings.Contains(rule, "resource_limit") || strings.Contains(rule, "max_"):
		v.ViolationType = ViolationResourceLimit
		v.Summary = "Resource limit exceeded"
		v.Explanation = "The operation would exceed configured resource limits."
		v.Remediation = []string{
			"Break the task into smaller chunks",
			"Adjust limits in /config if appropriate",
			"Check /status for current resource usage",
		}

	case strings.Contains(rule, "permitted") || strings.Contains(rule, "policy"):
		v.ViolationType = ViolationPolicyRule
		v.Summary = "Policy rule violation"
		v.Explanation = fmt.Sprintf("Action blocked by policy rule: %s", rule)
		v.Remediation = []string{
			"Use /query permitted to see what actions are allowed",
			"Use /why to understand the policy reasoning",
			"Consider /legislate to add custom rules if needed",
		}

	default:
		v.ViolationType = ViolationUnknown
		v.Summary = "Action not permitted"
		v.Explanation = fmt.Sprintf("The action '%s' was blocked by the constitutional gate.", action)
		v.Remediation = []string{
			"Use /why to understand why the action was blocked",
			"Try /shadow to simulate the action safely",
			"Check /status for system state",
		}
	}

	return v
}

// GetRecentViolations returns recent violations.
func (r *SafetyReporter) GetRecentViolations(limit int) []SafetyViolation {
	if limit <= 0 || limit > len(r.violations) {
		limit = len(r.violations)
	}

	start := len(r.violations) - limit
	result := make([]SafetyViolation, limit)
	copy(result, r.violations[start:])
	return result
}

// GetViolation returns a specific violation by ID.
func (r *SafetyReporter) GetViolation(id string) *SafetyViolation {
	for i := range r.violations {
		if r.violations[i].ID == id {
			return &r.violations[i]
		}
	}
	return nil
}

// FormatViolation returns a formatted explanation of a violation.
func (r *SafetyReporter) FormatViolation(v *SafetyViolation) string {
	if v == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## [SAFETY] %s\n\n", v.Summary))
	sb.WriteString(fmt.Sprintf("**Action**: `%s`\n", v.Action))
	if v.Target != "" {
		sb.WriteString(fmt.Sprintf("**Target**: `%s`\n", v.Target))
	}
	sb.WriteString(fmt.Sprintf("**Type**: %s\n\n", v.ViolationType.String()))

	sb.WriteString("### Why was this blocked?\n\n")
	sb.WriteString(v.Explanation)
	sb.WriteString("\n\n")

	if len(v.Remediation) > 0 {
		sb.WriteString("### How to proceed\n\n")
		for _, step := range v.Remediation {
			sb.WriteString(fmt.Sprintf("- %s\n", step))
		}
	}

	if v.Rule != "" {
		sb.WriteString(fmt.Sprintf("\n*Policy rule: `%s`*\n", v.Rule))
	}

	return sb.String()
}

// ClearHistory clears violation history.
func (r *SafetyReporter) ClearHistory() {
	r.violations = nil
}

// ExplainSafetyAction generates a safety explanation for a hypothetical action.
// This is useful for /safety <action> command.
func ExplainSafetyAction(action string) string {
	actionLower := strings.ToLower(action)

	var sb strings.Builder
	sb.WriteString("## Safety Analysis\n\n")
	sb.WriteString(fmt.Sprintf("**Action**: `%s`\n\n", action))

	// Analyze potential risks
	risks := []string{}
	warnings := []string{}

	if strings.Contains(actionLower, "rm") || strings.Contains(actionLower, "delete") {
		risks = append(risks, "**Destructive**: This action may permanently delete files")
		warnings = append(warnings, "Consider using `git status` before deletion to ensure nothing important is lost")
	}

	if strings.Contains(actionLower, "force") || strings.Contains(actionLower, "-f") {
		risks = append(risks, "**Force flag**: Bypasses safety confirmations")
		warnings = append(warnings, "Force flags can lead to unintended consequences")
	}

	if strings.Contains(actionLower, "sudo") || strings.Contains(actionLower, "admin") {
		risks = append(risks, "**Elevated privileges**: Requires administrative access")
		warnings = append(warnings, "Elevated privileges should be used sparingly")
	}

	if containsAnyWord(actionLower, ".env", "secret", "password", "credential") {
		risks = append(risks, "**Sensitive data**: May involve credentials or secrets")
		warnings = append(warnings, "Never commit sensitive data to version control")
	}

	if len(risks) == 0 {
		sb.WriteString("**Risk Level**: Low\n\n")
		sb.WriteString("This action appears safe to execute.\n")
	} else {
		sb.WriteString("**Risk Level**: ")
		if len(risks) >= 3 {
			sb.WriteString("High")
		} else if len(risks) >= 2 {
			sb.WriteString("Medium")
		} else {
			sb.WriteString("Low-Medium")
		}
		sb.WriteString("\n\n")

		sb.WriteString("### Potential Risks\n\n")
		for _, risk := range risks {
			sb.WriteString(fmt.Sprintf("- %s\n", risk))
		}

		if len(warnings) > 0 {
			sb.WriteString("\n### Recommendations\n\n")
			for _, warning := range warnings {
				sb.WriteString(fmt.Sprintf("- %s\n", warning))
			}
		}
	}

	sb.WriteString("\n### Safe Alternatives\n\n")
	sb.WriteString("- Use `/shadow` to simulate the action first\n")
	sb.WriteString("- Use `/whatif` to explore consequences\n")
	sb.WriteString("- Check `/query permitted` to see allowed actions\n")

	return sb.String()
}

// containsAnyWord checks if s contains any of the given words.
func containsAnyWord(s string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}
