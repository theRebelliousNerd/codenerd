package transparency

import (
	"fmt"
	"strings"
)

// ErrorCategory classifies errors for user guidance.
type ErrorCategory int

const (
	// ErrorCategorySafety indicates a constitutional/safety rule violation.
	ErrorCategorySafety ErrorCategory = iota

	// ErrorCategoryConfig indicates a configuration issue.
	ErrorCategoryConfig

	// ErrorCategoryAPI indicates an LLM API error.
	ErrorCategoryAPI

	// ErrorCategoryKernel indicates a Mangle kernel issue.
	ErrorCategoryKernel

	// ErrorCategoryShard indicates a shard execution issue.
	ErrorCategoryShard

	// ErrorCategoryFilesystem indicates a file/directory issue.
	ErrorCategoryFilesystem

	// ErrorCategoryNetwork indicates a network connectivity issue.
	ErrorCategoryNetwork

	// ErrorCategoryTimeout indicates an operation timeout.
	ErrorCategoryTimeout

	// ErrorCategoryUnknown is the fallback for unclassified errors.
	ErrorCategoryUnknown
)

// Prefix returns the display prefix for this error category.
func (c ErrorCategory) Prefix() string {
	prefixes := []string{
		"[SAFETY]",
		"[CONFIG]",
		"[API]",
		"[KERNEL]",
		"[SHARD]",
		"[FS]",
		"[NET]",
		"[TIMEOUT]",
		"[ERROR]",
	}
	if int(c) < len(prefixes) {
		return prefixes[c]
	}
	return "[ERROR]"
}

// String returns the category name.
func (c ErrorCategory) String() string {
	names := []string{
		"safety",
		"config",
		"api",
		"kernel",
		"shard",
		"filesystem",
		"network",
		"timeout",
		"unknown",
	}
	if int(c) < len(names) {
		return names[c]
	}
	return "unknown"
}

// ClassifiedError wraps an error with classification and remediation.
type ClassifiedError struct {
	Original    error
	Category    ErrorCategory
	Summary     string
	Remediation []string
}

// Error implements the error interface.
func (ce *ClassifiedError) Error() string {
	return ce.Format()
}

// Unwrap returns the original error for errors.Is/As compatibility.
func (ce *ClassifiedError) Unwrap() error {
	return ce.Original
}

// Format returns a user-friendly error message with remediation.
func (ce *ClassifiedError) Format() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s %s\n\n", ce.Category.Prefix(), ce.Summary))
	sb.WriteString(fmt.Sprintf("Details: %s\n", ce.Original.Error()))

	if len(ce.Remediation) > 0 {
		sb.WriteString("\nSuggested fixes:\n")
		for _, r := range ce.Remediation {
			sb.WriteString(fmt.Sprintf("  - %s\n", r))
		}
	}

	return sb.String()
}

// ClassifyError analyzes an error and returns a classified version.
func ClassifyError(err error) *ClassifiedError {
	if err == nil {
		return nil
	}

	classified := &ClassifiedError{
		Original: err,
		Category: ErrorCategoryUnknown,
		Summary:  "An unexpected error occurred",
	}

	errStr := strings.ToLower(err.Error())

	// Pattern matching for classification
	switch {
	case containsAny(errStr, "permission", "blocked", "constitutional", "not permitted", "denied"):
		classified.Category = ErrorCategorySafety
		classified.Summary = "A safety rule prevented this action"
		classified.Remediation = []string{
			"Use /shadow to simulate the action first",
			"Check if the target is in a protected directory",
			"Verify you have the necessary permissions",
		}

	case containsAny(errStr, "config", "configuration") ||
		(containsAny(errStr, "not found") && containsAny(errStr, ".json", "config")):
		classified.Category = ErrorCategoryConfig
		classified.Summary = "Configuration issue detected"
		classified.Remediation = []string{
			"Run /config wizard to set up configuration",
			"Check .nerd/config.json exists and is valid JSON",
			"Run /config show to view current configuration",
		}

	case containsAny(errStr, "api", "rate limit", "quota", "unauthorized", "401", "403", "429"):
		classified.Category = ErrorCategoryAPI
		classified.Summary = "LLM API issue"
		classified.Remediation = []string{
			"Check your API key is valid",
			"Verify you haven't exceeded rate limits",
			"Try a different model with /config set-model",
			"Check your account balance/quota",
		}

	case containsAny(errStr, "kernel", "mangle", "derivation", "stratification", "predicate"):
		classified.Category = ErrorCategoryKernel
		classified.Summary = "Logic kernel issue"
		classified.Remediation = []string{
			"Run /query * to check kernel state",
			"Check .nerd/mangle/*.mg files for syntax errors",
			"Try /scan to rebuild the kernel",
		}

	case containsAny(errStr, "shard", "spawn", "executor"):
		classified.Category = ErrorCategoryShard
		classified.Summary = "Shard execution issue"
		classified.Remediation = []string{
			"Check shard type is valid (coder, tester, reviewer, researcher)",
			"Verify memory limits haven't been exceeded",
			"Try running a simpler task first",
		}

	case containsAny(errStr, "file", "directory", "path", "no such", "cannot open", "permission denied"):
		classified.Category = ErrorCategoryFilesystem
		classified.Summary = "Filesystem issue"
		classified.Remediation = []string{
			"Check the file/directory exists",
			"Verify you have read/write permissions",
			"Run /scan to refresh the file index",
		}

	case containsAny(errStr, "connection", "network", "dial", "dns", "host", "unreachable"):
		classified.Category = ErrorCategoryNetwork
		classified.Summary = "Network connectivity issue"
		classified.Remediation = []string{
			"Check your internet connection",
			"Verify the service endpoint is reachable",
			"Try again in a few moments",
		}

	case containsAny(errStr, "timeout", "deadline", "context deadline", "timed out"):
		classified.Category = ErrorCategoryTimeout
		classified.Summary = "Operation timed out"
		classified.Remediation = []string{
			"Try a smaller scope or simpler task",
			"Check network connectivity",
			"Increase timeout in configuration",
		}
	}

	return classified
}

// containsAny returns true if s contains any of the patterns.
func containsAny(s string, patterns ...string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// GetRecoveryGuide returns remediation steps for an error category.
func GetRecoveryGuide(category ErrorCategory) []string {
	guides := map[ErrorCategory][]string{
		ErrorCategorySafety: {
			"Use /shadow to simulate the action without executing",
			"Check /query permission_denied for blocked rules",
			"Request explicit override if you understand the risks",
		},
		ErrorCategoryConfig: {
			"Run /config wizard for guided setup",
			"Run /config show to view current settings",
			"Check .nerd/config.json for syntax errors",
		},
		ErrorCategoryAPI: {
			"Verify your API key with /config show",
			"Check provider status page for outages",
			"Try a fallback model with /config set-model",
		},
		ErrorCategoryKernel: {
			"Run /query * to see all facts",
			"Check /why for derivation traces",
			"Try /scan to refresh the kernel state",
		},
		ErrorCategoryShard: {
			"Check /status for system state",
			"Try /spawn <type> with a simple task",
			"Review logs with /logs",
		},
		ErrorCategoryFilesystem: {
			"Run /scan to refresh file index",
			"Check file permissions",
			"Verify path exists",
		},
		ErrorCategoryNetwork: {
			"Check internet connectivity",
			"Verify endpoint URLs in config",
			"Try again after a brief wait",
		},
		ErrorCategoryTimeout: {
			"Reduce scope of the operation",
			"Check for resource constraints",
			"Increase timeout in /config",
		},
	}

	if steps, ok := guides[category]; ok {
		return steps
	}
	return []string{"Check logs for more details", "Try /help for available commands"}
}
