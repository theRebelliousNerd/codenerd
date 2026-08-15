package init

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// manifestSkipDirs are directories that never hold a first-party module
// manifest. Descending into node_modules in particular turns manifest discovery
// into a multi-minute walk over tens of thousands of third-party package.json
// files, and every one of them would be scanned for the project's own
// dependencies.
var manifestSkipDirs = map[string]bool{
	"node_modules": true, "vendor": true, ".git": true, ".nerd": true,
	"dist": true, "build": true, "target": true, "out": true,
	"__pycache__": true, ".next": true, ".nuxt": true, "coverage": true,
	"testdata": true, "third_party": true, ".venv": true, "venv": true,
	".tox": true, ".gradle": true, ".idea": true, ".vscode": true,
}

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
			name := entry.Name()
			if manifestSkipDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			// Manifests *inside* a directory at exactly maxDepth still count;
			// only its subdirectories are out of range.
			if manifestDepth(root, path) > maxDepth {
				return filepath.SkipDir
			}
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

// parsePackageJSONDependencies parses package.json and extracts dependencies with versions.
func (i *Initializer) parsePackageJSONDependencies(data []byte) []DependencyInfo {
	deps := []DependencyInfo{}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return deps
	}

	// Key dependencies to detect with their canonical names
	keyDeps := map[string]string{
		"puppeteer":         "puppeteer",
		"playwright":        "playwright",
		"openai":            "openai",
		"@anthropic-ai/sdk": "anthropic",
		"react":             "react",
		"vue":               "vue",
		"next":              "nextjs",
		"express":           "express",
		"fastify":           "fastify",
		"prisma":            "prisma",
		"@prisma/client":    "prisma",
		"typeorm":           "typeorm",
		"angular":           "angular",
		"@angular/core":     "angular",
		"svelte":            "svelte",
		"solid-js":          "solid",
		"@nestjs/core":      "nestjs",
		"koa":               "koa",
	}

	// Process main dependencies
	for depName, version := range pkg.Dependencies {
		if canonicalName, ok := keyDeps[depName]; ok {
			majorVersion := extractMajorVersion(version)
			deps = append(deps, DependencyInfo{
				Name:         canonicalName,
				Version:      version,
				MajorVersion: majorVersion,
				Type:         "direct",
			})
		}
	}

	// Process dev dependencies (for build tools)
	devKeyDeps := map[string]string{
		"typescript": "typescript",
		"webpack":    "webpack",
		"vite":       "vite",
		"jest":       "jest",
		"vitest":     "vitest",
		"mocha":      "mocha",
		"cypress":    "cypress",
		"eslint":     "eslint",
	}

	for depName, version := range pkg.DevDependencies {
		if canonicalName, ok := devKeyDeps[depName]; ok {
			majorVersion := extractMajorVersion(version)
			deps = append(deps, DependencyInfo{
				Name:         canonicalName,
				Version:      version,
				MajorVersion: majorVersion,
				Type:         "dev",
			})
		}
	}

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

// parseGoSum extracts notable transitive dependencies from go.sum.
func (i *Initializer) parseGoSum(content string) []DependencyInfo {
	deps := []DependencyInfo{}
	seen := make(map[string]bool)

	// Notable transitive dependencies to detect
	notableDeps := map[string]string{
		"github.com/stretchr/testify":         "testify",
		"github.com/google/uuid":              "uuid",
		"go.uber.org/zap":                     "zap",
		"github.com/sirupsen/logrus":          "logrus",
		"github.com/pkg/errors":               "errors",
		"golang.org/x/sync":                   "sync",
		"golang.org/x/crypto":                 "crypto",
		"github.com/prometheus/client_golang": "prometheus",
		"github.com/go-playground/validator":  "validator",
		"github.com/dgrijalva/jwt-go":         "jwt",
		"github.com/golang-jwt/jwt":           "jwt",
		"github.com/redis/go-redis":           "redis",
		"github.com/go-redis/redis":           "redis",
		"go.mongodb.org/mongo-driver":         "mongodb",
		"github.com/aws/aws-sdk-go":           "aws-sdk",
		"cloud.google.com/go":                 "gcp-sdk",
		"github.com/Azure/azure-sdk-for-go":   "azure-sdk",
		"k8s.io/client-go":                    "kubernetes",
		"github.com/hashicorp/consul":         "consul",
		"github.com/hashicorp/vault":          "vault",
		"github.com/nats-io/nats.go":          "nats",
		"github.com/segmentio/kafka-go":       "kafka",
		"github.com/streadway/amqp":           "rabbitmq",
		"github.com/graphql-go/graphql":       "graphql",
		"github.com/99designs/gqlgen":         "gqlgen",
		"google.golang.org/grpc":              "grpc",
		"github.com/grpc-ecosystem":           "grpc-ecosystem",
	}

	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		for pkg, name := range notableDeps {
			if strings.HasPrefix(line, pkg) && !seen[name] {
				deps = append(deps, DependencyInfo{
					Name: name,
					Type: "transitive",
				})
				seen[name] = true
			}
		}
	}

	return deps
}

// parsePackageLock extracts notable transitive dependencies from package-lock.json.
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

	// Notable transitive dependencies
	notableDeps := map[string]string{
		"@babel/core":           "babel",
		"webpack":               "webpack",
		"vite":                  "vite",
		"esbuild":               "esbuild",
		"rollup":                "rollup",
		"jest":                  "jest",
		"mocha":                 "mocha",
		"cypress":               "cypress",
		"eslint":                "eslint",
		"prettier":              "prettier",
		"typescript":            "typescript",
		"axios":                 "axios",
		"lodash":                "lodash",
		"moment":                "moment",
		"dayjs":                 "dayjs",
		"rxjs":                  "rxjs",
		"socket.io":             "socket.io",
		"mongoose":              "mongoose",
		"sequelize":             "sequelize",
		"@prisma/client":        "prisma",
		"redis":                 "redis",
		"@aws-sdk/client-s3":    "aws-sdk",
		"@google-cloud/storage": "gcp-sdk",
		"nuxt":                  "nuxt",
		"@nuxt/kit":             "nuxt",
		"gatsby":                "gatsby",
		"svelte":                "svelte",
		"solid-js":              "solid",
		"htmx.org":              "htmx",
		"tailwindcss":           "tailwind",
		"@emotion/react":        "emotion",
		"styled-components":     "styled-components",
	}

	seen := make(map[string]bool)

	// Check packages (npm v7+ format)
	for pkgPath := range lockFile.Packages {
		for pkg, name := range notableDeps {
			if strings.Contains(pkgPath, "/"+pkg) && !seen[name] {
				deps = append(deps, DependencyInfo{
					Name: name,
					Type: "transitive",
				})
				seen[name] = true
			}
		}
	}

	// Check dependencies (npm v6 format)
	for pkg := range lockFile.Dependencies {
		if name, ok := notableDeps[pkg]; ok && !seen[name] {
			deps = append(deps, DependencyInfo{
				Name: name,
				Type: "transitive",
			})
			seen[name] = true
		}
	}

	return deps
}

// parseYarnLock extracts notable transitive dependencies from yarn.lock.
func (i *Initializer) parseYarnLock(content string) []DependencyInfo {
	deps := []DependencyInfo{}
	seen := make(map[string]bool)

	notableDeps := []string{
		"nuxt", "gatsby", "svelte", "solid-js", "htmx.org",
		"tailwindcss", "webpack", "vite", "rollup", "esbuild",
		"jest", "cypress", "playwright", "puppeteer",
	}

	for _, pkg := range notableDeps {
		// yarn.lock format: "package@version":
		if strings.Contains(content, fmt.Sprintf("\"%s@", pkg)) && !seen[pkg] {
			deps = append(deps, DependencyInfo{
				Name: pkg,
				Type: "transitive",
			})
			seen[pkg] = true
		}
	}

	return deps
}

// parsePnpmLock extracts notable transitive dependencies from pnpm-lock.yaml.
func (i *Initializer) parsePnpmLock(content string) []DependencyInfo {
	deps := []DependencyInfo{}
	seen := make(map[string]bool)

	notableDeps := []string{
		"nuxt", "gatsby", "svelte", "solid-js", "htmx.org",
		"tailwindcss", "webpack", "vite", "rollup", "esbuild",
	}

	for _, pkg := range notableDeps {
		// pnpm-lock.yaml format: /package@version:
		if strings.Contains(content, "/"+pkg+"@") && !seen[pkg] {
			deps = append(deps, DependencyInfo{
				Name: pkg,
				Type: "transitive",
			})
			seen[pkg] = true
		}
	}

	return deps
}

// parseCargoLock extracts notable transitive dependencies from Cargo.lock.
func (i *Initializer) parseCargoLock(content string) []DependencyInfo {
	deps := []DependencyInfo{}
	seen := make(map[string]bool)

	// Notable Rust transitive dependencies
	notableDeps := map[string]string{
		"tokio":      "tokio",
		"async-std":  "async-std",
		"hyper":      "hyper",
		"actix-web":  "actix-web",
		"axum":       "axum",
		"rocket":     "rocket",
		"warp":       "warp",
		"diesel":     "diesel",
		"sqlx":       "sqlx",
		"serde":      "serde",
		"tracing":    "tracing",
		"clap":       "clap",
		"reqwest":    "reqwest",
		"tonic":      "tonic", // gRPC
		"prost":      "prost", // protobuf
		"redis":      "redis",
		"lapin":      "lapin", // RabbitMQ
		"rdkafka":    "kafka",
		"aws-sdk-s3": "aws-sdk",
	}

	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		if after, ok := strings.CutPrefix(line, "name = "); ok {
			name := strings.Trim(after, "\"")
			if mappedName, ok := notableDeps[name]; ok && !seen[mappedName] {
				deps = append(deps, DependencyInfo{
					Name: mappedName,
					Type: "transitive",
				})
				seen[mappedName] = true
			}
		}
	}

	return deps
}

// parsePipfileLock extracts notable transitive dependencies from Pipfile.lock.
func (i *Initializer) parsePipfileLock(data []byte) []DependencyInfo {
	deps := []DependencyInfo{}

	var lockFile struct {
		Default map[string]any `json:"default"`
		Develop map[string]any `json:"develop"`
	}

	if err := json.Unmarshal(data, &lockFile); err != nil {
		return deps
	}

	notableDeps := map[string]string{
		"django":       "django",
		"flask":        "flask",
		"fastapi":      "fastapi",
		"celery":       "celery",
		"redis":        "redis",
		"sqlalchemy":   "sqlalchemy",
		"pytest":       "pytest",
		"numpy":        "numpy",
		"pandas":       "pandas",
		"tensorflow":   "tensorflow",
		"torch":        "pytorch",
		"boto3":        "aws-sdk",
		"google-cloud": "gcp-sdk",
		"azure":        "azure-sdk",
		"pydantic":     "pydantic",
		"httpx":        "httpx",
		"aiohttp":      "aiohttp",
	}

	seen := make(map[string]bool)

	for pkg := range lockFile.Default {
		if name, ok := notableDeps[pkg]; ok && !seen[name] {
			deps = append(deps, DependencyInfo{
				Name: name,
				Type: "transitive",
			})
			seen[name] = true
		}
	}

	return deps
}

// parsePoetryLock extracts notable transitive dependencies from poetry.lock.
func (i *Initializer) parsePoetryLock(content string) []DependencyInfo {
	deps := []DependencyInfo{}
	seen := make(map[string]bool)

	notableDeps := map[string]string{
		"django":     "django",
		"flask":      "flask",
		"fastapi":    "fastapi",
		"celery":     "celery",
		"redis":      "redis",
		"sqlalchemy": "sqlalchemy",
		"pytest":     "pytest",
		"numpy":      "numpy",
		"pandas":     "pandas",
	}

	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		if after, ok := strings.CutPrefix(line, "name = "); ok {
			name := strings.Trim(after, "\"")
			if mappedName, ok := notableDeps[name]; ok && !seen[mappedName] {
				deps = append(deps, DependencyInfo{
					Name: mappedName,
					Type: "transitive",
				})
				seen[mappedName] = true
			}
		}
	}

	return deps
}
