package northstar

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/types"
)

// alignmentFactSnapshot requires one coherent view rather than repeated scans
// per path. Both production kernels expose QueryAll. A Query-only dependency
// remains useful for relevance helpers, but cannot establish scoped coverage.
type alignmentFactSnapshot interface {
	QueryAll() (map[string][]types.Fact, error)
}

func (g *Guardian) resolveAlignmentScope(scope AlignmentScope, vision *Vision) (*AlignmentCoverage, string) {
	normalized, issues := normalizeAlignmentScope(scope)
	coverage := &AlignmentCoverage{Scope: normalized, Outcome: ScopeUnknown, VisionDigest: VisionDigest(vision), Issues: issues}
	if normalized.Disposition != ScopeProjectOnly && g.store != nil {
		workspace, err := alignmentWorkspaceIdentity(filepath.Dir(filepath.Dir(g.store.Path())))
		if err != nil {
			coverage.Issues = append(coverage.Issues, err.Error())
		}
		if normalized.Workspace != workspace {
			coverage.Issues = append(coverage.Issues, "scope workspace differs from guardian workspace")
		}
	}
	if len(coverage.Issues) > 0 {
		return coverage, ""
	}
	if normalized.Disposition == ScopeProjectOnly {
		coverage.Outcome = ScopeResolved
		coverage.Complete = true
		return coverage, ""
	}
	g.mu.RLock()
	q := g.querier
	g.mu.RUnlock()
	snapshot, ok := q.(alignmentFactSnapshot)
	if !ok || IsNil(snapshot) {
		coverage.Issues = append(coverage.Issues, "module fact snapshot unavailable")
		return coverage, ""
	}
	facts, err := snapshot.QueryAll()
	if err != nil {
		coverage.Issues = append(coverage.Issues, "module fact snapshot: "+err.Error())
		return coverage, ""
	}
	view, err := alignmentModules(normalized.Workspace, facts)
	if err != nil {
		coverage.Issues = append(coverage.Issues, err.Error())
		return coverage, ""
	}
	selected := map[string]bool{}
	for _, p := range normalized.Paths {
		best := ""
		matched := false
		for mod := range view {
			if isModulePrefix(mod, p.Path) && len(mod) > len(best) {
				best = mod
			}
			// Directory write contracts (including '.') cover nested modules
			// as well. Selecting only the parent would lose their refinements.
			if p.Path == "." || isModulePrefix(p.Path, mod) {
				selected[mod] = true
				matched = true
			}
		}
		if best != "" {
			selected[best] = true
			matched = true
		}
		if !matched {
			coverage.UnmatchedPaths = append(coverage.UnmatchedPaths, p.Path)
		}
	}
	for mod := range selected {
		coverage.Modules = append(coverage.Modules, mod)
	}
	sort.Strings(coverage.Modules)
	coverage.UnmatchedPaths = sortedUnique(coverage.UnmatchedPaths)
	if len(coverage.Modules) == 0 {
		coverage.Issues = append(coverage.Issues, "no declared module for affected paths")
		return coverage, ""
	}
	var section strings.Builder
	for _, mod := range coverage.Modules {
		m := view[mod]
		if strings.TrimSpace(m.purpose) == "" {
			coverage.Issues = append(coverage.Issues, fmt.Sprintf("effective purpose unavailable for module %s", mod))
		}
		fmt.Fprintf(&section, "## Module Northstar (%s)\n**Purpose:** %s\n\n", mod, m.purpose)
		if len(m.requirements) > 0 {
			section.WriteString("### Module Requirements\n")
		}
		for _, req := range m.requirements {
			fmt.Fprintf(&section, "- [%s/%s/%s] %s\n", mod, req.ID, req.Severity, req.Statement)
		}
		section.WriteString("\n")
	}
	if len(coverage.Issues) > 0 {
		// No partial module context: assess the project while retaining why
		// module coverage could not be established.
		coverage.Modules = nil
		return coverage, ""
	}
	coverage.ModuleDigest = fmt.Sprintf("%x", sha256.Sum256([]byte(section.String())))
	coverage.Outcome = ScopeResolved
	coverage.Complete = true
	return coverage, section.String()
}

type alignmentModule struct {
	purpose      string
	requirements []ModuleRequirement
}

// alignmentModules serializes only kernel-derived effective purpose; inheritance
// stays in prompt_northstar.mg. Missing or contradictory facts are input errors.
func alignmentModules(workspace string, facts map[string][]types.Fact) (map[string]*alignmentModule, error) {
	view := map[string]*alignmentModule{}
	for _, pred := range []string{"module_northstar", "effective_module_purpose", "module_requirement"} {
		if _, present := facts[pred]; !present {
			return nil, fmt.Errorf("module predicate %s unavailable in snapshot", pred)
		}
		for _, fact := range facts[pred] {
			arity := 2
			if pred == "module_requirement" {
				arity = 4
			}
			if len(fact.Args) != arity {
				return nil, fmt.Errorf("malformed %s fact", pred)
			}
			mod, err := canonicalAlignmentPath(workspace, types.ExtractString(fact.Args[0]))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", pred, err)
			}
			if mod == "." {
				continue
			} // preserve ModuleForPath's project-root exclusion
			m := view[mod]
			if m == nil {
				m = &alignmentModule{}
				view[mod] = m
			}
			switch pred {
			case "effective_module_purpose":
				purpose := types.ExtractString(fact.Args[1])
				if m.purpose != "" && m.purpose != purpose {
					return nil, fmt.Errorf("conflicting effective purposes for %s", mod)
				}
				m.purpose = purpose
			case "module_requirement":
				req := ModuleRequirement{ID: types.ExtractString(fact.Args[1]), Statement: types.ExtractString(fact.Args[2]), Severity: strings.TrimPrefix(types.ExtractString(fact.Args[3]), "/")}
				if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Statement) == "" {
					return nil, fmt.Errorf("incomplete module requirement for %s", mod)
				}
				duplicate := false
				for _, existing := range m.requirements {
					if existing.ID != req.ID {
						continue
					}
					if existing != req {
						return nil, fmt.Errorf("conflicting requirement %s/%s", mod, req.ID)
					}
					duplicate = true
				}
				if !duplicate {
					m.requirements = append(m.requirements, req)
				}
			}
		}
	}
	for _, m := range view {
		sort.Slice(m.requirements, func(i, j int) bool { return m.requirements[i].ID < m.requirements[j].ID })
	}
	return view, nil
}
