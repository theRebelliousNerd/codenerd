package reviewer

import (
	"context"
	"strings"
)

// =============================================================================
// ONE-HOP DEPENDENCY FETCHING
// =============================================================================
// Fetches upstream (imports) and downstream (importers) files for review context.

// DependencyContext holds 1-hop dependency files for a reviewed file.
type DependencyContext struct {
	TargetFile string            // The file being reviewed
	Upstream   []string          // Files this file imports
	Downstream []string          // Files that import this file
	Contents   map[string]string // Content of dependency files (truncated for context)
}

// getOneHopDependencies queries the kernel for dependency_link facts
// to find upstream (what this file imports) and downstream (what imports this file).
func (r *ReviewerShard) getOneHopDependencies(ctx context.Context, filePath string) (*DependencyContext, error) {
	depCtx := &DependencyContext{
		TargetFile: filePath,
		Upstream:   make([]string, 0),
		Downstream: make([]string, 0),
		Contents:   make(map[string]string),
	}

	if r.kernel == nil {
		return depCtx, nil // No kernel, return empty context
	}

	// Query dependency_link facts: dependency_link(CallerID, CalleeID, ImportPath)
	// For upstream: find where this file/package is the Caller
	// For downstream: find where this file/package is the Callee
	depLinks, err := r.kernel.Query("dependency_link")
	if err != nil {
		return depCtx, nil // Query failed, return empty context
	}

	// Normalize the target file path for matching
	normalizedTarget := normalizePath(filePath)

	for _, fact := range depLinks {
		if len(fact.Args) < 3 {
			continue
		}

		caller, _ := fact.Args[0].(string)
		callee, _ := fact.Args[1].(string)

		// Check if this file is the caller (find what it imports - upstream)
		if pathMatches(caller, normalizedTarget) {
			// callee is something this file imports
			if depFile := resolveDepToFile(callee); depFile != "" {
				depCtx.Upstream = append(depCtx.Upstream, depFile)
			}
		}

		// Check if this file is the callee (find what imports it - downstream)
		if pathMatches(callee, normalizedTarget) {
			// caller imports this file
			if depFile := resolveDepToFile(caller); depFile != "" {
				depCtx.Downstream = append(depCtx.Downstream, depFile)
			}
		}
	}

	// Deduplicate
	depCtx.Upstream = deduplicateStrings(depCtx.Upstream)
	depCtx.Downstream = deduplicateStrings(depCtx.Downstream)

	// Load truncated content for each dependency (limit to avoid context explosion)
	maxDeps := 5 // Limit to 5 upstream + 5 downstream
	allDeps := make([]string, 0)
	for i, dep := range depCtx.Upstream {
		if i >= maxDeps {
			break
		}
		allDeps = append(allDeps, dep)
	}
	for i, dep := range depCtx.Downstream {
		if i >= maxDeps {
			break
		}
		allDeps = append(allDeps, dep)
	}

	for _, dep := range allDeps {
		content, err := r.readFile(ctx, dep)
		if err != nil {
			continue
		}
		// Truncate to first 100 lines for context
		lines := strings.Split(content, "\n")
		if len(lines) > 100 {
			content = strings.Join(lines[:100], "\n") + "\n// ... (truncated)"
		}
		depCtx.Contents[dep] = content
	}

	return depCtx, nil
}

// normalizePath normalizes a file path for comparison.
func normalizePath(path string) string {
	// Convert backslashes to forward slashes and lowercase
	path = strings.ReplaceAll(path, "\\", "/")
	return strings.ToLower(path)
}

// pathMatches checks if a dependency reference matches a file path.
func pathMatches(depRef, normalizedPath string) bool {
	// dependency_link uses formats like "pkg:packagename" or file paths
	depRef = strings.ToLower(depRef)

	// Direct file path match
	if strings.Contains(normalizedPath, depRef) || strings.Contains(depRef, normalizedPath) {
		return true
	}

	// Package-based match: extract package from path
	// e.g., "pkg:core" matches "internal/core/foo.go"
	if strings.HasPrefix(depRef, "pkg:") {
		pkgName := strings.TrimPrefix(depRef, "pkg:")
		return strings.Contains(normalizedPath, "/"+pkgName+"/") ||
			strings.HasSuffix(normalizedPath, "/"+pkgName)
	}

	return false
}

// resolveDepToFile attempts to resolve a dependency reference to a file path.
func resolveDepToFile(depRef string) string {
	// If it's already a file path, return it
	if strings.HasSuffix(depRef, ".go") || strings.HasSuffix(depRef, ".py") ||
		strings.HasSuffix(depRef, ".ts") || strings.HasSuffix(depRef, ".js") ||
		strings.HasSuffix(depRef, ".rs") {
		return depRef
	}

	// For pkg: references, we can't directly resolve to files without more context
	// Return empty - the caller will need to use other mechanisms
	return ""
}

// deduplicateStrings removes duplicates from a string slice.
func deduplicateStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
