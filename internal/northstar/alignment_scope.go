package northstar

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/types"
)

// AlignmentScope is structural evidence, supplied independently of the prose
// subject. It grants no write permission and says nothing about task acceptance.
type AlignmentScope struct {
	Workspace   string           `json:"workspace"`
	Disposition ScopeDisposition `json:"disposition"`
	Paths       []AlignmentPath  `json:"paths,omitempty"`
}

type ScopeDisposition string

const (
	ScopeProjectOnly ScopeDisposition = "project_only"
	ScopePaths       ScopeDisposition = "paths"
	ScopeUnresolved  ScopeDisposition = "unresolved"
)

type AlignmentPathRole string

const (
	PathPlannedWrite   AlignmentPathRole = "planned_write"
	PathObservedEffect AlignmentPathRole = "observed_effect"
	PathArtifact       AlignmentPathRole = "artifact"
	PathContextSource  AlignmentPathRole = "context_source"
	PathManualTarget   AlignmentPathRole = "manual_target"
)

type AlignmentPath struct {
	Path string            `json:"path"`
	Role AlignmentPathRole `json:"role"`
}

// AlignmentCoverage records exactly which context supported a check. A nil
// coverage on historical rows means coverage was not recorded, never complete.
type AlignmentCoverage struct {
	Scope          AlignmentScope `json:"scope"`
	Outcome        ScopeOutcome   `json:"outcome"`
	VisionDigest   string         `json:"vision_digest"`
	ModuleDigest   string         `json:"module_digest,omitempty"`
	Modules        []string       `json:"modules,omitempty"`
	UnmatchedPaths []string       `json:"unmatched_paths,omitempty"`
	Issues         []string       `json:"issues,omitempty"`
	Complete       bool           `json:"complete"`
}

// ScopeOutcome describes module resolution independently of the model verdict.
type ScopeOutcome string

const (
	ScopeResolved ScopeOutcome = "resolved"
	ScopeUnknown  ScopeOutcome = "unknown"
)

// ManualAlignmentScope preserves the manual command's bare-path form. Prose is
// deliberately project-only; incidental path mentions are never parsed as scope.
// Ambiguous prose may be assessed globally without claiming module coverage.
func ManualAlignmentScope(workspace, subject string) AlignmentScope {
	s := strings.TrimSpace(subject)
	scope := AlignmentScope{Workspace: workspace, Disposition: ScopeProjectOnly}
	barePath := len(strings.Fields(s)) == 1 && (strings.ContainsAny(s, `/\`) || path.Ext(s) != "" || s == ".")
	// Existing whole-path subjects also cover single-component directories and
	// paths with spaces. This checks the entire subject, never prose fragments.
	if !barePath && workspace != "" {
		if canonical, err := canonicalAlignmentPath(workspace, s); err == nil {
			_, err := os.Stat(types.ResolveWorkspacePath(workspace, canonical))
			barePath = err == nil
		}
	}
	if barePath {
		scope.Disposition = ScopePaths
		scope.Paths = []AlignmentPath{{Path: s, Role: PathManualTarget}}
	}
	return scope
}

func canonicalAlignmentPath(workspace, raw string) (string, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\x00\r\n") {
		return "", fmt.Errorf("invalid or ambiguous path %q", raw)
	}
	p := types.CanonicalPath(workspace, raw)
	if p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") || strings.Contains(p, ":") {
		return "", fmt.Errorf("path %q is outside workspace or ambiguous", raw)
	}
	// A glob is a set, not a canonical path. Its governing modules cannot be
	// established by pretending that the wildcard is a literal directory.
	if strings.ContainsAny(p, "*?[") {
		return "", fmt.Errorf("path %q has unresolved glob scope", raw)
	}
	return p, nil
}

func normalizeAlignmentScope(scope AlignmentScope) (AlignmentScope, []string) {
	workspace, err := alignmentWorkspaceIdentity(scope.Workspace)
	out := AlignmentScope{Workspace: workspace, Disposition: scope.Disposition}
	var issues []string
	if err != nil {
		issues = append(issues, err.Error())
	}
	if scope.Workspace == "" {
		out.Workspace = ""
		if scope.Disposition != ScopeProjectOnly {
			issues = append(issues, "workspace identity unavailable")
		}
	}
	switch scope.Disposition {
	case ScopeProjectOnly:
		if len(scope.Paths) > 0 {
			issues = append(issues, "project-only request carries scoped paths")
		}
	case ScopePaths:
		if len(scope.Paths) == 0 {
			issues = append(issues, "affected paths unavailable")
		}
	case ScopeUnresolved:
		issues = append(issues, "affected scope unresolved")
	default:
		issues = append(issues, "scope disposition unavailable")
	}
	seen := map[AlignmentPath]bool{}
	for _, supplied := range scope.Paths {
		p, err := canonicalAlignmentPath(out.Workspace, supplied.Path)
		if err != nil {
			issues = append(issues, err.Error())
			// Retain rejected identities for diagnosis; Complete remains false.
			out.Paths = append(out.Paths, supplied)
			continue
		}
		switch supplied.Role {
		case PathPlannedWrite, PathObservedEffect, PathArtifact, PathContextSource, PathManualTarget:
		default:
			issues = append(issues, fmt.Sprintf("path %q has unknown role %q", supplied.Path, supplied.Role))
		}
		entry := AlignmentPath{Path: p, Role: supplied.Role}
		if !seen[entry] {
			seen[entry] = true
			out.Paths = append(out.Paths, entry)
		}
	}
	sort.Slice(out.Paths, func(i, j int) bool {
		if out.Paths[i].Path != out.Paths[j].Path {
			return out.Paths[i].Path < out.Paths[j].Path
		}
		return out.Paths[i].Role < out.Paths[j].Role
	})
	return out, issues
}

func sortedUnique(values []string) []string {
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}

func alignmentScopeSection(coverage *AlignmentCoverage) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Alignment Scope\nWorkspace: %s\nDisposition: %s\n", coverage.Scope.Workspace, coverage.Scope.Disposition)
	fmt.Fprintf(&sb, "Outcome: %s\n", coverage.Outcome)
	for _, p := range coverage.Scope.Paths {
		fmt.Fprintf(&sb, "- [%s] %s\n", p.Role, p.Path)
	}
	for _, p := range coverage.UnmatchedPaths {
		fmt.Fprintf(&sb, "No declared module: %s\n", p)
	}
	for _, issue := range coverage.Issues {
		fmt.Fprintf(&sb, "Scope issue: %s\n", issue)
	}
	sb.WriteString("\n")
	return sb.String()
}

func coverageContainsScope(coverage *AlignmentCoverage, scope AlignmentScope) bool {
	if coverage == nil || !coverage.Complete {
		return false
	}
	normalized, issues := normalizeAlignmentScope(scope)
	if len(issues) > 0 || normalized.Workspace != coverage.Scope.Workspace || normalized.Disposition != coverage.Scope.Disposition {
		return false
	}
	for _, p := range normalized.Paths {
		found := false
		for _, prior := range coverage.Scope.Paths {
			if prior.Role == p.Role && (prior.Path == "." || isModulePrefix(prior.Path, p.Path)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func alignmentWorkspaceIdentity(workspace string) (string, error) {
	if workspace == "" {
		return "", nil
	}
	if strings.TrimSpace(workspace) != workspace || strings.ContainsAny(workspace, "\x00\r\n") {
		return workspace, fmt.Errorf("ambiguous workspace identity %q", workspace)
	}
	clean := types.SlashClean(workspace)
	if strings.HasPrefix(clean, "/") || (len(clean) >= 3 && clean[1] == ':' && clean[2] == '/') {
		return clean, nil
	}
	absolute, err := filepath.Abs(clean)
	if err != nil {
		return clean, fmt.Errorf("workspace identity: %w", err)
	}
	return types.SlashClean(absolute), nil
}

func cloneAlignmentCoverage(c *AlignmentCoverage) *AlignmentCoverage {
	if c == nil {
		return nil
	}
	out := *c
	out.Scope.Paths = append([]AlignmentPath(nil), c.Scope.Paths...)
	out.Modules = append([]string(nil), c.Modules...)
	out.UnmatchedPaths = append([]string(nil), c.UnmatchedPaths...)
	out.Issues = append([]string(nil), c.Issues...)
	return &out
}
