package northstar

import (
	"path"
	"path/filepath"
	"strings"

	"codenerd/internal/types"
)

// FactQuerier is the slice of a kernel this package needs. Declaring it here
// rather than importing internal/core keeps northstar a leaf package, and keeps
// the module helpers usable from both the guardian and tests.
type FactQuerier interface {
	Query(predicate string) ([]types.Fact, error)
}

// ModuleRequirement is a single requirement declared by a module.
type ModuleRequirement struct {
	ID        string
	Statement string
	Severity  string
}

// EffectiveModulePurpose returns the purpose in force for modulePath, or "" when none.
//
// The kernel is the authority on purpose: module_northstar and
// effective_module_purpose facts are asserted at boot like any other EDB, so
// policy, `nerd query`, and this helper all see exactly the same effective
// purpose. A parallel in-memory copy is one refactor away from disagreeing
// with what the kernel holds. Query failures are returned so callers can fail
// closed rather than silently treating an unavailable authority as "no purpose".
func EffectiveModulePurpose(q FactQuerier, modulePath string) (string, error) {
	if q == nil {
		return "", nil
	}
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return "", nil
	}
	modulePath = path.Clean(filepath.ToSlash(modulePath))
	facts, err := q.Query("effective_module_purpose")
	if err != nil {
		return "", err
	}
	for _, fact := range facts {
		if len(fact.Args) < 2 {
			continue
		}
		mp := path.Clean(filepath.ToSlash(strings.TrimSpace(types.ExtractString(fact.Args[0]))))
		if mp == modulePath {
			return types.ExtractString(fact.Args[1]), nil
		}
	}
	// Fallback to module_northstar directly for environments where the derived
	// predicate has not yet been materialized (tests with a fake querier that
	// only provides EDB facts). This keeps the helper usable against both the
	// derived and the base fact.
	if fallback, err := q.Query("module_northstar"); err == nil {
		for _, fact := range fallback {
			if len(fact.Args) < 2 {
				continue
			}
			mp := path.Clean(filepath.ToSlash(strings.TrimSpace(types.ExtractString(fact.Args[0]))))
			if mp == modulePath {
				return types.ExtractString(fact.Args[1]), nil
			}
		}
	}
	return "", nil
}

// ModuleRequirementsFor returns the requirements declared for modulePath.
//
// Like EffectiveModulePurpose it asks the kernel rather than a cached Go
// struct, so every caller sees the same module_requirement facts that policy
// does.
func ModuleRequirementsFor(q FactQuerier, modulePath string) ([]ModuleRequirement, error) {
	if q == nil {
		return nil, nil
	}
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return nil, nil
	}
	modulePath = path.Clean(filepath.ToSlash(modulePath))
	facts, err := q.Query("module_requirement")
	if err != nil {
		return nil, err
	}
	var out []ModuleRequirement
	for _, fact := range facts {
		if len(fact.Args) < 4 {
			continue
		}
		mp := path.Clean(filepath.ToSlash(strings.TrimSpace(types.ExtractString(fact.Args[0]))))
		if mp != modulePath {
			continue
		}
		id := types.ExtractString(fact.Args[1])
		stmt := types.ExtractString(fact.Args[2])
		sevRaw := types.ExtractString(fact.Args[3])
		sev := strings.TrimPrefix(sevRaw, "/")
		out = append(out, ModuleRequirement{
			ID:        id,
			Statement: stmt,
			Severity:  sev,
		})
	}
	return out, nil
}

// ModuleForPath returns the most specific module path governing filePath.
//
// Semantics: a file is governed by the LONGEST module path that is a directory
// prefix of it. For internal/session/executor.go, if both "internal" and
// "internal/session" declare northstars, "internal/session" wins. That is the
// refine half of inherit-and-refine -- the most specific declaration governs.
// Comparison is on path SEGMENT boundaries, not raw string prefixes, so
// "internal/sessionx" is never treated as being inside "internal/session".
// Return "" when no module governs the file.
// ModuleForPath returns the most specific module path governing filePath.
//
// Semantics: a file is governed by the LONGEST module path that is a directory
// prefix of it. For internal/session/executor.go, if both "internal" and
// "internal/session" declare northstars, "internal/session" wins. That is the
// refine half of inherit-and-refine -- the most specific declaration governs.
// Comparison is on path SEGMENT boundaries, not raw string prefixes, so
// "internal/sessionx" is never treated as being inside "internal/session".
// Return "" when no module governs the file.
func ModuleForPath(q FactQuerier, filePath string) (string, error) {
	if q == nil {
		return "", nil
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", nil
	}
	filePath = path.Clean(filepath.ToSlash(filePath))
	// Remove leading "./" that path.Clean keeps for "."-relative inputs
	if strings.HasPrefix(filePath, "./") {
		filePath = strings.TrimPrefix(filePath, "./")
	}
	// Collect candidates from effective_module_purpose (derived) and also from
	// the base predicates so a fake querier that only provides EDB facts still
	// works. Deduplicate by module path.
	candidates := make(map[string]struct{})
	addFacts := func(pred string, minArgs int) error {
		facts, err := q.Query(pred)
		if err != nil {
			return err
		}
		for _, fact := range facts {
			if len(fact.Args) < minArgs {
				continue
			}
			modRaw := strings.TrimSpace(types.ExtractString(fact.Args[0]))
			if modRaw == "" {
				continue
			}
			mod := path.Clean(filepath.ToSlash(modRaw))
			if mod == "." {
				continue
			}
			candidates[mod] = struct{}{}
		}
		return nil
	}
	if err := addFacts("effective_module_purpose", 2); err != nil {
		return "", err
	}
	if err := addFacts("module_northstar", 2); err != nil {
		return "", err
	}
	if err := addFacts("module_requirement", 2); err != nil {
		return "", err
	}
	best := ""
	bestLen := -1
	for mod := range candidates {
		if isModulePrefix(mod, filePath) {
			if len(mod) > bestLen {
				best = mod
				bestLen = len(mod)
			}
		}
	}
	if bestLen < 0 {
		return "", nil
	}
	return best, nil
}

func isModulePrefix(module, file string) bool {
	if module == file {
		return true
	}
	if strings.HasPrefix(file, module+"/") {
		return true
	}
	return false
}
