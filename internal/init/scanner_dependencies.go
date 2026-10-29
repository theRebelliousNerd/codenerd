package init

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	wscope "codenerd/internal/workspace"
)

// maxManifestDepth bounds how deep below the workspace root a module manifest
// is still considered part of this project. Real monorepo layouts put modules
// at services/<name>/, packages/<scope>/<name>/ or apps/<team>/<name>/; past
// four levels the hits are almost always fixtures or example projects.
const maxManifestDepth = 4

// maxManifestFiles caps discovery so a pathological tree cannot stall init.
const maxManifestFiles = 256

// findManifestFiles walks the workspace for module manifests with the given
// base names.
//
// This replaces two hardcoded glob pairs (`*/go.mod` and `*/*/go.mod`, likewise
// for package.json) that could only see modules exactly one or two directories
// below the root. The common monorepo shapes — `services/api/go.mod`,
// `packages/@scope/ui/package.json`, `apps/web/frontend/package.json` — sit at
// depth three or four and were invisible, so a monorepo profiled as if it had
// no dependencies at all: no framework, no framework agents, no
// framework-scoped prompt atoms.
//
// Results are sorted so the dependency set (and therefore profile.mg and the
// chosen framework) is identical across runs regardless of directory order.
func findManifestFiles(workspace string, names []string, maxDepth int) []string {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}

	var found []string
	root := filepath.Clean(workspace)
	// Membership, not a private directory list: gitignore and
	// world.ignore_patterns are what keep node_modules out of a monorepo's
	// own dependency set. A failure returns nothing rather than scanning
	// the ignored trees the list used to exist to skip.
	mem, memErr := wscope.For(root)
	if memErr != nil {
		return nil
	}
	if memErr = mem.Refresh(); memErr != nil {
		return nil
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable subtree: skip, never abort discovery
		}
		if len(found) >= maxManifestFiles {
			return filepath.SkipAll
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			member, admErr := mem.Admit(path, true)
			if admErr != nil {
				return admErr
			}
			if !member {
				return filepath.SkipDir
			}
			// Manifests *inside* a directory at exactly maxDepth still count;
			// only its subdirectories are out of range.
			if manifestDepth(root, path) > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		member, _ := mem.Admit(path, false)
		if !member {
			return nil
		}
		if wanted[entry.Name()] {
			found = append(found, path)
		}
		return nil
	})

	sort.Strings(found)
	return found
}

// manifestDepth reports how many directory levels below root path sits.
func manifestDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(filepath.ToSlash(rel), "/"))
}

// frameworkCandidate maps a canonical dependency name produced by
// detectDependencies / detectTransitiveDependencies onto the framework label
// stored in ProjectProfile.Framework.
type frameworkCandidate struct {
	dep       string
	framework string
	// rank breaks ties between co-installed dependencies. A meta-framework
	// outranks the library it wraps (Next.js ships React, Nuxt ships Vue), and
	// an application framework outranks a CLI/TUI or ORM library, because the
	// framework field answers "what shape is this project?" not "what is in
	// go.mod?".
	rank int
}

// frameworkCandidates is ordered for determinism: equal scores resolve to the
// earlier entry, so the same dependency set always yields the same framework.
var frameworkCandidates = []frameworkCandidate{
	// Meta-frameworks — they imply their underlying view library.
	{dep: "nextjs", framework: "nextjs", rank: 100},
	{dep: "nuxt", framework: "nuxt", rank: 100},
	{dep: "gatsby", framework: "gatsby", rank: 95},
	{dep: "nestjs", framework: "nestjs", rank: 95},

	// Server / application frameworks.
	{dep: "django", framework: "django", rank: 90},
	{dep: "fastapi", framework: "fastapi", rank: 90},
	{dep: "flask", framework: "flask", rank: 88},
	{dep: "gin", framework: "gin", rank: 88},
	{dep: "echo", framework: "echo", rank: 87},
	{dep: "fiber", framework: "fiber", rank: 87},
	{dep: "actix-web", framework: "actix-web", rank: 87},
	{dep: "axum", framework: "axum", rank: 87},
	{dep: "rocket", framework: "rocket", rank: 86},
	{dep: "warp", framework: "warp", rank: 85},
	{dep: "express", framework: "express", rank: 85},
	{dep: "fastify", framework: "fastify", rank: 85},
	{dep: "koa", framework: "koa", rank: 84},
	{dep: "gorilla", framework: "gorilla", rank: 80},

	// View libraries — only decisive when no meta-framework is present.
	{dep: "react", framework: "react", rank: 70},
	{dep: "vue", framework: "vue", rank: 70},
	{dep: "angular", framework: "angular", rank: 70},
	{dep: "svelte", framework: "svelte", rank: 68},
	{dep: "solid", framework: "solid", rank: 68},
	{dep: "htmx", framework: "htmx", rank: 60},

	// Terminal UI and CLI frameworks — a TUI binary has no web framework, so
	// these are the honest answer for that shape of project.
	{dep: "bubbletea", framework: "bubbletea", rank: 55},
	{dep: "cobra", framework: "cobra", rank: 40},
}

// detectFrameworkFromDependencies picks the project's primary framework.
//
// ProjectProfile.Framework was declared, persisted to profile.json, emitted as
// the project_framework/1 Mangle fact, used to build the /framework JIT
// selector, used by GenerateToolsForProject and used by categorizeAgent to mark
// framework specialists as "recommended" — but nothing ever assigned it. Every
// one of those consumers saw the empty string, so framework-scoped prompt atoms
// could never be selected and framework agents were never recommended.
//
// Direct dependencies outscore transitive ones by a margin smaller than the gap
// between ranks, so a directly required view library still loses to a
// transitively detected meta-framework that wraps it.
func detectFrameworkFromDependencies(deps []DependencyInfo) string {
	if len(deps) == 0 {
		return ""
	}

	directness := make(map[string]int, len(deps))
	for _, dep := range deps {
		name := strings.ToLower(strings.TrimSpace(dep.Name))
		if name == "" {
			continue
		}
		bonus := 0
		if dep.Type == "direct" {
			bonus = 2
		} else if dep.Type == "dev" {
			bonus = 1
		}
		if existing, seen := directness[name]; !seen || bonus > existing {
			directness[name] = bonus
		}
	}

	best := ""
	bestScore := -1
	for _, candidate := range frameworkCandidates {
		bonus, present := directness[candidate.dep]
		if !present {
			continue
		}
		if score := candidate.rank + bonus; score > bestScore {
			best = candidate.framework
			bestScore = score
		}
	}
	return best
}

// extractGoModVersion extracts the version of a dependency from go.mod content.
func (i *Initializer) extractGoModVersion(content, pkg string) string {
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, pkg) {
			// Format: "pkg version" or "pkg version // indirect"
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// parsePackageJSONDependencies parses package.json and extracts the
// vocabulary's packages (technology_vocabulary.go) with versions: dependencies
// as direct, devDependencies as dev.
func (i *Initializer) parsePackageJSONDependencies(data []byte) []DependencyInfo {
	deps := []DependencyInfo{}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return deps
	}

	add := func(declared map[string]string, kind string) {
		names := make([]string, 0, len(declared))
		for name := range declared {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if canonicalName, ok := technologyFor(EcosystemNPM, name); ok {
				version := declared[name]
				deps = append(deps, DependencyInfo{
					Name:         canonicalName,
					Version:      version,
					MajorVersion: extractMajorVersion(version),
					Type:         kind,
				})
			}
		}
	}
	add(pkg.Dependencies, "direct")
	add(pkg.DevDependencies, "dev")
	return deps
}

// extractMajorVersion extracts the major version number from a version string.
// Handles various formats: "1.2.3", "^1.2.3", "~1.2.3", ">=1.2.3", "v1.2.3"
func extractMajorVersion(version string) string {
	if version == "" {
		return ""
	}

	// Remove common prefixes
	version = strings.TrimPrefix(version, "^")
	version = strings.TrimPrefix(version, "~")
	version = strings.TrimPrefix(version, ">=")
	version = strings.TrimPrefix(version, ">")
	version = strings.TrimPrefix(version, "<=")
	version = strings.TrimPrefix(version, "<")
	version = strings.TrimPrefix(version, "=")
	version = strings.TrimPrefix(version, "v")

	// Split by dot and return first part
	parts := strings.Split(version, ".")
	if len(parts) > 0 {
		// Handle "0" as a special case for 0.x versions
		major := parts[0]
		// Clean any non-numeric characters
		var numStr strings.Builder
		for _, c := range major {
			if c >= '0' && c <= '9' {
				numStr.WriteRune(c)
			} else {
				break
			}
		}
		return numStr.String()
	}

	return ""
}

// detectTransitiveDependencies parses lock files to detect hidden/transitive dependencies.
// This helps detect frameworks that are used indirectly (e.g., Vue via Nuxt, React via Next.js).
func (i *Initializer) detectTransitiveDependencies() []DependencyInfo {
	deps := []DependencyInfo{}
	workspace := i.config.Workspace

	// Parse go.sum for Go transitive dependencies
	goSumPath := filepath.Join(workspace, "go.sum")
	if data, err := os.ReadFile(goSumPath); err == nil {
		goSumDeps := i.parseGoSum(string(data))
		deps = append(deps, goSumDeps...)
	}

	// Parse package-lock.json for Node.js transitive dependencies
	pkgLockPath := filepath.Join(workspace, "package-lock.json")
	if data, err := os.ReadFile(pkgLockPath); err == nil {
		pkgLockDeps := i.parsePackageLock(data)
		deps = append(deps, pkgLockDeps...)
	}

	// Parse yarn.lock for Yarn projects
	yarnLockPath := filepath.Join(workspace, "yarn.lock")
	if data, err := os.ReadFile(yarnLockPath); err == nil {
		yarnDeps := i.parseYarnLock(string(data))
		deps = append(deps, yarnDeps...)
	}

	// Parse pnpm-lock.yaml for pnpm projects
	pnpmLockPath := filepath.Join(workspace, "pnpm-lock.yaml")
	if data, err := os.ReadFile(pnpmLockPath); err == nil {
		pnpmDeps := i.parsePnpmLock(string(data))
		deps = append(deps, pnpmDeps...)
	}

	// Parse Cargo.lock for Rust transitive dependencies
	cargoLockPath := filepath.Join(workspace, "Cargo.lock")
	if data, err := os.ReadFile(cargoLockPath); err == nil {
		cargoDeps := i.parseCargoLock(string(data))
		deps = append(deps, cargoDeps...)
	}

	// Parse Pipfile.lock or poetry.lock for Python transitive dependencies
	pipfileLockPath := filepath.Join(workspace, "Pipfile.lock")
	if data, err := os.ReadFile(pipfileLockPath); err == nil {
		pipDeps := i.parsePipfileLock(data)
		deps = append(deps, pipDeps...)
	}

	poetryLockPath := filepath.Join(workspace, "poetry.lock")
	if data, err := os.ReadFile(poetryLockPath); err == nil {
		poetryDeps := i.parsePoetryLock(string(data))
		deps = append(deps, poetryDeps...)
	}

	return deps
}

// parseGoSum extracts the vocabulary's Go modules (technology_vocabulary.go)
// from go.sum as transitive dependencies.
func (i *Initializer) parseGoSum(content string) []DependencyInfo {
	deps := []DependencyInfo{}
	seen := make(map[string]bool)
	pkgs := vocabularyPackages(EcosystemGo)
	for line := range strings.SplitSeq(content, "\n") {
		for _, pkg := range pkgs {
			name, _ := technologyFor(EcosystemGo, pkg)
			if strings.HasPrefix(line, pkg) && !seen[name] {
				deps = append(deps, DependencyInfo{Name: name, Type: "transitive"})
				seen[name] = true
			}
		}
	}
	return deps
}

// parsePackageLock extracts the vocabulary's npm packages from
// package-lock.json as transitive dependencies.
func (i *Initializer) parsePackageLock(data []byte) []DependencyInfo {
	deps := []DependencyInfo{}

	var lockFile struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}

	if err := json.Unmarshal(data, &lockFile); err != nil {
		return deps
	}

	seen := make(map[string]bool)
	pkgs := vocabularyPackages(EcosystemNPM)
	paths := make([]string, 0, len(lockFile.Packages))
	for pkgPath := range lockFile.Packages {
		paths = append(paths, pkgPath)
	}
	sort.Strings(paths)

	// Check packages (npm v7+ format)
	for _, pkgPath := range paths {
		for _, pkg := range pkgs {
			name, _ := technologyFor(EcosystemNPM, pkg)
			if strings.HasSuffix(pkgPath, "node_modules/"+pkg) && !seen[name] {
				deps = append(deps, DependencyInfo{Name: name, Type: "transitive"})
				seen[name] = true
			}
		}
	}

	// Check dependencies (npm v6 format)
	for _, pkg := range pkgs {
		if _, ok := lockFile.Dependencies[pkg]; !ok {
			continue
		}
		if name, _ := technologyFor(EcosystemNPM, pkg); !seen[name] {
			deps = append(deps, DependencyInfo{Name: name, Type: "transitive"})
			seen[name] = true
		}
	}

	return deps
}

// parseYarnLock extracts the vocabulary's npm packages from yarn.lock.
func (i *Initializer) parseYarnLock(content string) []DependencyInfo {
	return npmLockMentions(content, func(pkg string) string { return fmt.Sprintf("\"%s@", pkg) })
}

// parsePnpmLock extracts the vocabulary's npm packages from pnpm-lock.yaml.
func (i *Initializer) parsePnpmLock(content string) []DependencyInfo {
	return npmLockMentions(content, func(pkg string) string { return "/" + pkg + "@" })
}

// npmLockMentions reports each vocabulary npm package the lock text names in
// its entry form (yarn: "pkg@, pnpm: /pkg@) as a transitive dependency.
func npmLockMentions(content string, entry func(pkg string) string) []DependencyInfo {
	deps := []DependencyInfo{}
	seen := make(map[string]bool)
	for _, pkg := range vocabularyPackages(EcosystemNPM) {
		name, _ := technologyFor(EcosystemNPM, pkg)
		if strings.Contains(content, entry(pkg)) && !seen[name] {
			deps = append(deps, DependencyInfo{Name: name, Type: "transitive"})
			seen[name] = true
		}
	}
	return deps
}

// parseCargoLock extracts the vocabulary's crates from Cargo.lock.
func (i *Initializer) parseCargoLock(content string) []DependencyInfo {
	deps := []DependencyInfo{}
	seen := make(map[string]bool)
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		if after, ok := strings.CutPrefix(line, "name = "); ok {
			name := strings.Trim(after, "\"")
			if mappedName, ok := technologyFor(EcosystemRust, name); ok && !seen[mappedName] {
				deps = append(deps, DependencyInfo{Name: mappedName, Type: "transitive"})
				seen[mappedName] = true
			}
		}
	}
	return deps
}

// parsePipfileLock extracts the vocabulary's Python distributions from
// Pipfile.lock.
func (i *Initializer) parsePipfileLock(data []byte) []DependencyInfo {
	deps := []DependencyInfo{}

	var lockFile struct {
		Default map[string]any `json:"default"`
		Develop map[string]any `json:"develop"`
	}

	if err := json.Unmarshal(data, &lockFile); err != nil {
		return deps
	}

	seen := make(map[string]bool)
	for _, pkg := range vocabularyPackages(EcosystemPython) {
		if _, ok := lockFile.Default[pkg]; !ok {
			continue
		}
		if name, _ := technologyFor(EcosystemPython, pkg); !seen[name] {
			deps = append(deps, DependencyInfo{Name: name, Type: "transitive"})
			seen[name] = true
		}
	}
	return deps
}

// parsePoetryLock extracts the vocabulary's Python distributions from
// poetry.lock.
func (i *Initializer) parsePoetryLock(content string) []DependencyInfo {
	deps := []DependencyInfo{}
	seen := make(map[string]bool)
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		if after, ok := strings.CutPrefix(line, "name = "); ok {
			name := strings.Trim(after, "\"")
			if mappedName, ok := technologyFor(EcosystemPython, name); ok && !seen[mappedName] {
				deps = append(deps, DependencyInfo{Name: mappedName, Type: "transitive"})
				seen[mappedName] = true
			}
		}
	}
	return deps
}
