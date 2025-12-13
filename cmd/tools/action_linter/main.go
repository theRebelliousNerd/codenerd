// Package main implements an action drift linter for codeNERD.
//
// It cross-checks:
// - Mangle policy-emitted actions (next_action/*_next_action)
// - Router default route patterns
// - VirtualStore supported ActionType values
//
// Usage:
//
//	go run ./cmd/tools/action_linter -mg-root internal/core/defaults -virtual-store internal/core/virtual_store.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/shards/system"
)

type issueSeverity string

const (
	severityError   issueSeverity = "error"
	severityWarning issueSeverity = "warning"
)

type issue struct {
	Severity issueSeverity
	Action   string
	Message  string
	Sources  []string
}

func main() {
	mgRoot := flag.String("mg-root", "internal/core/defaults", "Root directory to scan for .mg policy files")
	virtualStoreFile := flag.String("virtual-store", "internal/core/virtual_store.go", "Path to internal/core/virtual_store.go")
	failOnWarn := flag.Bool("fail-on-warn", false, "Exit non-zero if warnings are present")
	warnUnusedExecutors := flag.Bool("warn-unused-executors", true, "Warn when VirtualStore action types are never emitted by policy")
	flag.Parse()

	routerRoutes := system.DefaultRouterConfig().DefaultRoutes

	policyActions, err := extractPolicyActions(*mgRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "action_linter: failed to scan mg files: %v\n", err)
		os.Exit(2)
	}

	virtualActions, err := extractVirtualStoreActionTypes(*virtualStoreFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "action_linter: failed to parse virtual store actions: %v\n", err)
		os.Exit(2)
	}

	issues := lint(policyActions, routerRoutes, virtualActions, *warnUnusedExecutors)

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Severity != issues[j].Severity {
			return issues[i].Severity < issues[j].Severity
		}
		if issues[i].Action != issues[j].Action {
			return issues[i].Action < issues[j].Action
		}
		return issues[i].Message < issues[j].Message
	})

	var errCount, warnCount int
	for _, it := range issues {
		switch it.Severity {
		case severityError:
			errCount++
		case severityWarning:
			warnCount++
		}
	}

	fmt.Printf("Actions: policy=%d, virtual_store=%d, router_routes=%d\n", len(policyActions), len(virtualActions), len(routerRoutes))
	if errCount == 0 && warnCount == 0 {
		fmt.Println("OK: no issues found")
		return
	}

	fmt.Printf("Issues: %d errors, %d warnings\n", errCount, warnCount)
	for _, it := range issues {
		loc := it.Action
		if len(it.Sources) > 0 {
			loc = fmt.Sprintf("%s (%s)", it.Action, strings.Join(it.Sources, ", "))
		}
		fmt.Printf("- %s: %s: %s\n", it.Severity, loc, it.Message)
	}

	if errCount > 0 || (*failOnWarn && warnCount > 0) {
		os.Exit(1)
	}
}

type actionSources struct {
	Action     string
	Sources    []string
	Predicates map[string]struct{}
}

func extractPolicyActions(root string) (map[string]actionSources, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("mg-root is not a directory: %s", root)
	}

	// Executive consumes these predicates in queryNextActions().
	actionPredicates := map[string]struct{}{
		"next_action":          {},
		"tdd_next_action":      {},
		"campaign_next_action": {},
		"repair_next_action":   {},
	}

	// Capture predicate + leading name constant.
	re := regexp.MustCompile(`(?m)\b(next_action|tdd_next_action|campaign_next_action|repair_next_action)\(\s*(/[^,\s\)]+)`)

	out := make(map[string]actionSources)
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".mg" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		matches := re.FindAllStringSubmatch(string(data), -1)
		if len(matches) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(root, path)

		for _, m := range matches {
			if len(m) < 3 {
				continue
			}
			pred := m[1]
			if _, ok := actionPredicates[pred]; !ok {
				continue
			}
			raw := strings.TrimSpace(m[2])
			normalized := strings.TrimPrefix(raw, "/")
			if normalized == "" {
				continue
			}
			rec, ok := out[normalized]
			if !ok {
				rec = actionSources{
					Action:     "/" + normalized,
					Sources:    nil,
					Predicates: make(map[string]struct{}),
				}
			}
			rec.Predicates[pred] = struct{}{}
			rec.Sources = append(rec.Sources, fmt.Sprintf("%s:%s", filepath.ToSlash(rel), pred))
			out[normalized] = rec
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	for k, rec := range out {
		rec.Sources = uniqueSorted(rec.Sources)
		out[k] = rec
	}

	return out, nil
}

func extractVirtualStoreActionTypes(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Match: ActionFoo ActionType = "foo_bar"
	re := regexp.MustCompile(`(?m)^\s*Action[A-Za-z0-9_]+\s+ActionType\s*=\s*"([a-zA-Z0-9_]+)"`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	out := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		val := strings.TrimSpace(m[1])
		if val == "" {
			continue
		}
		out[val] = struct{}{}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no ActionType constants found in %s", path)
	}
	return out, nil
}

func lint(policyActions map[string]actionSources, routes []system.ToolRoute, virtualActions map[string]struct{}, warnUnusedExecutors bool) []issue {
	issues := make([]issue, 0, 64)

	// Policy -> router -> virtual store
	for normalized, rec := range policyActions {
		route, ok := bestRouteForAction("/"+normalized, routes)
		if !ok {
			issues = append(issues, issue{
				Severity: severityError,
				Action:   rec.Action,
				Message:  "policy emits action but router has no matching route",
				Sources:  rec.Sources,
			})
			continue
		}

		_, hasExecutor := virtualActions[normalized]
		if route.ToolName == "kernel_internal" && hasExecutor {
			issues = append(issues, issue{
				Severity: severityError,
				Action:   rec.Action,
				Message:  "router routes this action to kernel_internal, but VirtualStore has an executor (action will be dropped)",
				Sources:  rec.Sources,
			})
			continue
		}

		if route.ToolName != "kernel_internal" && !hasExecutor {
			issues = append(issues, issue{
				Severity: severityError,
				Action:   rec.Action,
				Message:  fmt.Sprintf("router routes this action to %q, but VirtualStore has no executor", route.ToolName),
				Sources:  rec.Sources,
			})
		}
	}

	if warnUnusedExecutors {
		// VirtualStore action types that policy never emits (potential dead code / drift).
		for action := range virtualActions {
			if _, ok := policyActions[action]; ok {
				continue
			}
			issues = append(issues, issue{
				Severity: severityWarning,
				Action:   "/" + action,
				Message:  "VirtualStore supports action, but policy never emits it (possible dead action or future capability)",
			})
		}
	}

	return issues
}

func bestRouteForAction(actionType string, routes []system.ToolRoute) (system.ToolRoute, bool) {
	normalizedAction := strings.TrimPrefix(actionType, "/")
	if normalizedAction == "" {
		return system.ToolRoute{}, false
	}

	const (
		matchNone     = 0
		matchContains = 1
		matchPrefix   = 2
		matchExact    = 3
	)

	bestScore := matchNone
	bestLen := -1
	bestPattern := ""
	bestRoute := system.ToolRoute{}

	for _, route := range routes {
		pattern := strings.TrimPrefix(route.ActionPattern, "/")
		if pattern == "" {
			continue
		}

		score := matchNone
		switch {
		case normalizedAction == pattern:
			score = matchExact
		case strings.HasPrefix(normalizedAction, pattern):
			score = matchPrefix
		case strings.Contains(normalizedAction, pattern):
			score = matchContains
		default:
			continue
		}

		if score > bestScore ||
			(score == bestScore && len(pattern) > bestLen) ||
			(score == bestScore && len(pattern) == bestLen && pattern < bestPattern) {
			bestScore = score
			bestLen = len(pattern)
			bestPattern = pattern
			bestRoute = route
		}
	}

	if bestScore == matchNone {
		return system.ToolRoute{}, false
	}
	return bestRoute, true
}

func uniqueSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
