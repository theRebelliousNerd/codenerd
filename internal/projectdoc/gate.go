package projectdoc

import (
	"strings"

	"codenerd/internal/types"
)

// FactQuerier is the slice of a kernel this gate needs. Declaring it here rather
// than importing internal/core keeps projectdoc a leaf package, and keeps the
// gate usable from both the session executor and the VirtualStore.
type FactQuerier interface {
	Query(predicate string) ([]types.Fact, error)
}

// ForbiddenByKernel reports whether nerd.md protects target, asking the kernel
// rather than a cached Go struct.
//
// The kernel is the authority on purpose: nerd.md rules are asserted as
// project_forbidden_path facts at boot like any other EDB, so policy, `nerd
// query`, and this gate all see exactly the same rules. A parallel in-memory
// copy is one refactor away from disagreeing with what the kernel holds — which
// is precisely the failure mode Document.ForbidsPath has, being a second
// implementation of this same predicate over a struct.
//
// This function exists because there were two independent write gates and only
// one of them was reachable from most write paths. internal/session's Executor
// checked project_forbidden_path before running a write-mutation tool; the
// VirtualStore, which is how shards actually route file writes, checked nothing.
// A shard could write a protected path that the interactive path refused.
//
// Query failures are returned to callers, which fail write gates closed. The
// helper itself cannot construct an enforcement error because callers need
// surface-specific logging, but it never converts an unavailable authority into
// an allow decision.
func ForbiddenByKernel(q FactQuerier, target string) (reason string, forbidden bool, err error) {
	if q == nil || strings.TrimSpace(target) == "" {
		return "", false, nil
	}

	facts, err := q.Query(PredForbiddenPath)
	if err != nil {
		return "", false, err
	}

	normalized := normalizeForMatch(target)
	if normalized == "" {
		return "", false, nil
	}

	for _, fact := range facts {
		if len(fact.Args) < 2 {
			continue
		}
		match := normalizeForMatch(types.ExtractString(fact.Args[0]))
		if match == "" {
			continue
		}
		if strings.Contains(normalized, match) {
			return types.ExtractString(fact.Args[1]), true, nil
		}
	}
	return "", false, nil
}

// normalizeForMatch is the single definition of how a path is compared against a
// forbid rule: slash-normalised and case-folded, then matched as a substring.
//
// Substring, not glob: a rule reading ".nerd/config.json" should protect that
// file whether the tool names it relatively, absolutely, or with Windows
// separators. Both the Go gates and the Mangle path_contains helper have to
// agree on this, so there is exactly one place to change it.
//
// The separator rewrite is deliberately NOT filepath.ToSlash. That function is
// a no-op everywhere except Windows, because it converts os.PathSeparator — so
// on Linux and macOS `.nerd\config.json` reached the comparison with its
// backslashes intact, failed to match the `.nerd/config.json` rule, and walked
// straight through the write gate. The separator a path is written with is an
// attacker-controlled detail of the string, not a property of the host, so it
// is normalized unconditionally.
func normalizeForMatch(p string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(p), `\`, "/"))
}
