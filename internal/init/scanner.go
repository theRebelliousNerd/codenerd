// Package init implements the "nerd init" cold-start initialization system.
package init

import (
	"codenerd/internal/config"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// detectLanguageFromFiles detects the primary language by looking for config files.
// FIX(BUG-006): Searches subdirectories (2 levels deep) for monorepo support.
func (i *Initializer) detectLanguageFromFiles() string {
	workspace := i.config.Workspace

	// Check for language-specific config files
	checks := []struct {
		file     string
		language string
	}{
		{"go.mod", "go"},
		{"Cargo.toml", "rust"},
		{"package.json", "typescript"}, // Could be JS, but TS is more common now
		{"pyproject.toml", "python"},
		{"requirements.txt", "python"},
		{"setup.py", "python"},
		{"pom.xml", "java"},
		{"build.gradle", "java"},
		{"build.gradle.kts", "kotlin"}, // FIX(BUG-006): Kotlin/Android detection
		{"settings.gradle.kts", "kotlin"},
		{"*.csproj", "csharp"},
		{"mix.exs", "elixir"},
		{"Gemfile", "ruby"},
	}

	// First check root directory
	for _, check := range checks {
		pattern := filepath.Join(workspace, check.file)
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return check.language
		}
	}

	// FIX(BUG-006): For monorepos, check subdirectories (2 levels deep)
	// Count occurrences of each language
	langCounts := make(map[string]int)
	for _, check := range checks {
		// Check 1 level deep: */go.mod
		pattern1 := filepath.Join(workspace, "*", check.file)
		if matches, err := filepath.Glob(pattern1); err == nil {
			langCounts[check.language] += len(matches)
		}
		// Check 2 levels deep: */*/go.mod
		pattern2 := filepath.Join(workspace, "*", "*", check.file)
		if matches, err := filepath.Glob(pattern2); err == nil {
			langCounts[check.language] += len(matches)
		}
	}

	// Return the language with the most config files
	var maxLang string
	var maxCount int
	for lang, count := range langCounts {
		if count > maxCount {
			maxCount = count
			maxLang = lang
		}
	}

	if maxLang != "" {
		return maxLang
	}

	return "unknown"
}

// detectDependencies scans project files for key dependencies with version information.
// D4 enhancement: Extracts versions to enable version-specific agent recommendations.
// FIX(BUG-006): Searches subdirectories (2 levels deep) for monorepo support.
func (i *Initializer) detectDependencies() []DependencyInfo {
	deps := []DependencyInfo{}
	workspace := i.config.Workspace
	seen := make(map[string]bool) // Dedupe dependencies

	// Key Go dependencies to detect
	goDeps := map[string]string{
		"github.com/go-rod/rod":              "rod",
		"github.com/chromedp/chromedp":       "chromedp",
		"github.com/playwright-community":    "playwright",
		"google/mangle":                      "mangle",
		"github.com/google/mangle":           "mangle",
		"github.com/sashabaranov/go-openai":  "openai",
		"github.com/anthropics/anthropic":    "anthropic",
		"github.com/charmbracelet/bubbletea": "bubbletea",
		"github.com/spf13/cobra":             "cobra",
		"github.com/gin-gonic/gin":           "gin",
		"github.com/labstack/echo":           "echo",
		"github.com/gofiber/fiber":           "fiber",
		"gorm.io/gorm":                       "gorm",
		"github.com/jmoiron/sqlx":            "sqlx",
		"database/sql":                       "sql",
		"github.com/gorilla/mux":             "gorilla",
		"net/http":                           "http",
		"github.com/arangodb/go-driver":      "arangodb",
		"google.golang.org/adk":              "adk",
		"github.com/a2aserver/a2a-go":        "a2a",
	}

	// Helper to scan a go.mod file
	scanGoMod := func(path string) {
		if data, err := os.ReadFile(path); err == nil {
			content := string(data)
			for pkg, name := range goDeps {
				if strings.Contains(content, pkg) && !seen[name] {
					version := i.extractGoModVersion(content, pkg)
					majorVersion := extractMajorVersion(version)
					deps = append(deps, DependencyInfo{
						Name:         name,
						Version:      version,
						MajorVersion: majorVersion,
						Type:         "direct",
					})
					seen[name] = true
				}
			}
		}
	}

	// Helper to scan a package.json file
	scanPackageJSON := func(path string) {
		if data, err := os.ReadFile(path); err == nil {
			nodeDeps := i.parsePackageJSONDependencies(data)
			for _, dep := range nodeDeps {
				if !seen[dep.Name] {
					deps = append(deps, dep)
					seen[dep.Name] = true
				}
			}
		}
	}

	// Check root directory first
	scanGoMod(filepath.Join(workspace, "go.mod"))
	scanPackageJSON(filepath.Join(workspace, "package.json"))

	// FIX(BUG-006): Check subdirectories for monorepo support
	// Check 1 level deep
	if goMods, err := filepath.Glob(filepath.Join(workspace, "*", "go.mod")); err == nil {
		for _, goMod := range goMods {
			scanGoMod(goMod)
		}
	}
	if pkgJSONs, err := filepath.Glob(filepath.Join(workspace, "*", "package.json")); err == nil {
		for _, pkg := range pkgJSONs {
			scanPackageJSON(pkg)
		}
	}

	// Check 2 levels deep
	if goMods, err := filepath.Glob(filepath.Join(workspace, "*", "*", "go.mod")); err == nil {
		for _, goMod := range goMods {
			scanGoMod(goMod)
		}
	}
	if pkgJSONs, err := filepath.Glob(filepath.Join(workspace, "*", "*", "package.json")); err == nil {
		for _, pkg := range pkgJSONs {
			scanPackageJSON(pkg)
		}
	}

	// Parse transitive dependencies from lock files
	transitiveDeps := i.detectTransitiveDependencies()
	deps = append(deps, transitiveDeps...)

	return deps
}

// detectEntryPoints identifies the application entry points based on language patterns.
func (i *Initializer) detectEntryPoints() []string {
	workspace := i.config.Workspace
	entryPoints := []string{}

	// Helper to check for file existence
	exists := func(path string) bool {
		_, err := os.Stat(filepath.Join(workspace, path))
		return err == nil
	}

	// Helper to check file content pattern
	hasContent := func(path, pattern string) bool {
		content, err := os.ReadFile(filepath.Join(workspace, path))
		if err != nil {
			return false
		}
		return strings.Contains(string(content), pattern)
	}

	// 1. Go Detection
	// Standard main.go
	if exists("main.go") {
		entryPoints = append(entryPoints, "main.go")
	}
	// cmd/ directory pattern
	if info, err := os.Stat(filepath.Join(workspace, "cmd")); err == nil && info.IsDir() {
		_ = filepath.Walk(filepath.Join(workspace, "cmd"), func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(path, ".go") {
				// Calculate relative path first
				rel, err := filepath.Rel(workspace, path)
				if err != nil {
					return nil
				}

				// Check for package main
				if hasContent(rel, "package main") && hasContent(rel, "func main()") {
					entryPoints = append(entryPoints, rel)
				}
			}
			return nil
		})
	}

	// 2. Python Detection
	pythonCandidates := []string{"main.py", "app.py", "manage.py", "wsgi.py", "asgi.py", "__main__.py"}
	for _, f := range pythonCandidates {
		if exists(f) {
			entryPoints = append(entryPoints, f)
		}
	}
	// Check for files with if __name__ == "__main__":
	// Limit scan to root and src directories to avoid scanning venv
	scanDirs := []string{".", "src"}
	for _, dir := range scanDirs {
		dirPath := filepath.Join(workspace, dir)
		if _, err := os.Stat(dirPath); err != nil {
			continue
		}

		entries, _ := os.ReadDir(dirPath)
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".py") {
				relPath := filepath.Join(dir, entry.Name())
				if dir == "." {
					relPath = entry.Name()
				}

				// Avoid duplicates if already added by candidate list
				alreadyAdded := false
				for _, ep := range entryPoints {
					if ep == relPath {
						alreadyAdded = true
						break
					}
				}

				if !alreadyAdded && hasContent(relPath, `if __name__ == "__main__":`) {
					entryPoints = append(entryPoints, relPath)
				}
			}
		}
	}

	// 3. Node/TypeScript Detection
	if exists("package.json") {
		data, err := os.ReadFile(filepath.Join(workspace, "package.json"))
		if err == nil {
			var pkg struct {
				Main    string            `json:"main"`
				Bin     interface{}       `json:"bin"` // Can be string or map
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(data, &pkg) == nil {
				if pkg.Main != "" {
					entryPoints = append(entryPoints, pkg.Main)
				}
				// Handle 'bin' field
				switch v := pkg.Bin.(type) {
				case string:
					entryPoints = append(entryPoints, v)
				case map[string]interface{}:
					for _, val := range v {
						if strVal, ok := val.(string); ok {
							entryPoints = append(entryPoints, strVal)
						}
					}
				}
				// Heuristic: check start script
				if start, ok := pkg.Scripts["start"]; ok {
					// Extract filename from "node dist/index.js" or "ts-node src/index.ts"
					parts := strings.Fields(start)
					for _, part := range parts {
						if strings.HasSuffix(part, ".js") || strings.HasSuffix(part, ".ts") {
							entryPoints = append(entryPoints, part)
							break
						}
					}
				}
			}
		}
	}
	// Common Node files if not in package.json
	nodeCandidates := []string{"index.js", "index.ts", "server.js", "server.ts", "app.js", "app.ts"}
	for _, f := range nodeCandidates {
		if exists(f) {
			// Only add if not already covered (avoid duplicates)
			found := false
			for _, ep := range entryPoints {
				if ep == f {
					found = true
					break
				}
			}
			if !found {
				entryPoints = append(entryPoints, f)
			}
		}
	}

	return entryPoints
}

// extractGoModVersion extracts the version of a dependency from go.mod content.
func (i *Initializer) extractGoModVersion(content, pkg string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
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

	lines := strings.Split(content, "\n")
	for _, line := range lines {
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

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "name = ") {
			name := strings.Trim(strings.TrimPrefix(line, "name = "), "\"")
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
		Default map[string]interface{} `json:"default"`
		Develop map[string]interface{} `json:"develop"`
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

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "name = ") {
			name := strings.Trim(strings.TrimPrefix(line, "name = "), "\"")
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

// detectBuildSystem identifies the primary build system used by the project.
// Returns the build system name and associated metadata.
func (i *Initializer) detectBuildSystem() (string, map[string]string) {
	workspace := i.config.Workspace
	metadata := make(map[string]string)

	// Build systems in priority order (more specific first)
	buildSystems := []struct {
		Name       string
		Indicators []string
		FileCheck  func(string) bool
	}{
		// Monorepo build tools (check first as they often coexist with language-specific tools)
		{
			Name:       "turborepo",
			Indicators: []string{"turbo.json"},
			FileCheck:  nil,
		},
		{
			Name:       "nx",
			Indicators: []string{"nx.json", "workspace.json"},
			FileCheck:  nil,
		},
		{
			Name:       "rush",
			Indicators: []string{"rush.json"},
			FileCheck:  nil,
		},
		{
			Name:       "lerna",
			Indicators: []string{"lerna.json"},
			FileCheck:  nil,
		},
		// Language-agnostic build systems
		{
			Name:       "bazel",
			Indicators: []string{"WORKSPACE", "WORKSPACE.bazel", "BUILD.bazel", "MODULE.bazel"},
			FileCheck:  nil,
		},
		{
			Name:       "buck",
			Indicators: []string{"BUCK", ".buckconfig"},
			FileCheck:  nil,
		},
		{
			Name:       "buck2",
			Indicators: []string{".buckroot", "BUCK.v2"},
			FileCheck:  nil,
		},
		{
			Name:       "pants",
			Indicators: []string{"pants.toml", "BUILD"},
			FileCheck:  nil,
		},
		{
			Name:       "please",
			Indicators: []string{".plzconfig"},
			FileCheck:  nil,
		},
		// C/C++ build systems
		{
			Name:       "cmake",
			Indicators: []string{"CMakeLists.txt"},
			FileCheck:  nil,
		},
		{
			Name:       "meson",
			Indicators: []string{"meson.build"},
			FileCheck:  nil,
		},
		{
			Name:       "ninja",
			Indicators: []string{"build.ninja"},
			FileCheck:  nil,
		},
		{
			Name:       "autotools",
			Indicators: []string{"configure.ac", "Makefile.am"},
			FileCheck:  nil,
		},
		// JVM build systems
		{
			Name:       "gradle",
			Indicators: []string{"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"},
			FileCheck:  nil,
		},
		{
			Name:       "maven",
			Indicators: []string{"pom.xml"},
			FileCheck:  nil,
		},
		{
			Name:       "sbt",
			Indicators: []string{"build.sbt"},
			FileCheck:  nil,
		},
		{
			Name:       "ant",
			Indicators: []string{"build.xml"},
			FileCheck:  nil,
		},
		// .NET build systems
		{
			Name:       "msbuild",
			Indicators: []string{"*.csproj", "*.fsproj", "*.vbproj", "*.sln"},
			FileCheck:  nil,
		},
		{
			Name:       "dotnet",
			Indicators: []string{"global.json"},
			FileCheck:  nil,
		},
		// Go build system
		{
			Name:       "go",
			Indicators: []string{"go.mod"},
			FileCheck:  nil,
		},
		// Rust build system
		{
			Name:       "cargo",
			Indicators: []string{"Cargo.toml"},
			FileCheck:  nil,
		},
		// Node.js/JavaScript build systems
		{
			Name:       "npm",
			Indicators: []string{"package.json", "package-lock.json"},
			FileCheck:  nil,
		},
		{
			Name:       "yarn",
			Indicators: []string{"yarn.lock"},
			FileCheck:  nil,
		},
		{
			Name:       "pnpm",
			Indicators: []string{"pnpm-lock.yaml"},
			FileCheck:  nil,
		},
		{
			Name:       "bun",
			Indicators: []string{"bun.lockb"},
			FileCheck:  nil,
		},
		// Python build systems
		{
			Name:       "poetry",
			Indicators: []string{"pyproject.toml", "poetry.lock"},
			FileCheck:  nil,
		},
		{
			Name:       "pipenv",
			Indicators: []string{"Pipfile", "Pipfile.lock"},
			FileCheck:  nil,
		},
		{
			Name:       "pip",
			Indicators: []string{"requirements.txt", "setup.py"},
			FileCheck:  nil,
		},
		{
			Name:       "hatch",
			Indicators: []string{"hatch.toml"},
			FileCheck:  nil,
		},
		{
			Name:       "pdm",
			Indicators: []string{"pdm.lock"},
			FileCheck:  nil,
		},
		// Ruby build systems
		{
			Name:       "bundler",
			Indicators: []string{"Gemfile", "Gemfile.lock"},
			FileCheck:  nil,
		},
		{
			Name:       "rake",
			Indicators: []string{"Rakefile"},
			FileCheck:  nil,
		},
		// PHP build systems
		{
			Name:       "composer",
			Indicators: []string{"composer.json", "composer.lock"},
			FileCheck:  nil,
		},
		// Elixir build systems
		{
			Name:       "mix",
			Indicators: []string{"mix.exs"},
			FileCheck:  nil,
		},
		// Generic make
		{
			Name:       "make",
			Indicators: []string{"Makefile", "makefile", "GNUmakefile"},
			FileCheck:  nil,
		},
		// Task runners
		{
			Name:       "just",
			Indicators: []string{"justfile", "Justfile"},
			FileCheck:  nil,
		},
		{
			Name:       "task",
			Indicators: []string{"Taskfile.yml", "Taskfile.yaml"},
			FileCheck:  nil,
		},
	}

	// Check for each build system
	for _, bs := range buildSystems {
		for _, indicator := range bs.Indicators {
			// Handle glob patterns
			if strings.Contains(indicator, "*") {
				pattern := filepath.Join(workspace, indicator)
				if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
					metadata["detected_file"] = filepath.Base(matches[0])
					return bs.Name, metadata
				}
			} else {
				// Direct file check
				if _, err := os.Stat(filepath.Join(workspace, indicator)); err == nil {
					metadata["detected_file"] = indicator
					return bs.Name, metadata
				}
			}
		}
	}

	return "", metadata
}

// detectBuildSystemDetails returns extended information about the build system.
func (i *Initializer) detectBuildSystemDetails() BuildSystemInfo {
	workspace := i.config.Workspace
	info := BuildSystemInfo{}

	buildSystem, metadata := i.detectBuildSystem()
	info.Name = buildSystem
	info.Metadata = metadata

	// Get additional details based on build system type
	switch buildSystem {
	case "gradle":
		// Check for Kotlin DSL vs Groovy DSL
		if _, err := os.Stat(filepath.Join(workspace, "build.gradle.kts")); err == nil {
			info.Variant = "kotlin-dsl"
		} else {
			info.Variant = "groovy-dsl"
		}
		// Check for wrapper
		if _, err := os.Stat(filepath.Join(workspace, "gradlew")); err == nil {
			info.HasWrapper = true
		}
	case "maven":
		// Check for wrapper
		if _, err := os.Stat(filepath.Join(workspace, "mvnw")); err == nil {
			info.HasWrapper = true
		}
	case "bazel":
		// Check for Bzlmod (MODULE.bazel) vs WORKSPACE
		if _, err := os.Stat(filepath.Join(workspace, "MODULE.bazel")); err == nil {
			info.Variant = "bzlmod"
		} else {
			info.Variant = "workspace"
		}
	case "cmake":
		// Check for presets
		if _, err := os.Stat(filepath.Join(workspace, "CMakePresets.json")); err == nil {
			info.Features = append(info.Features, "presets")
		}
	case "npm", "yarn", "pnpm":
		// Check for scripts in package.json
		if data, err := os.ReadFile(filepath.Join(workspace, "package.json")); err == nil {
			var pkg struct {
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(data, &pkg) == nil {
				// Common script patterns
				if _, ok := pkg.Scripts["build"]; ok {
					info.Features = append(info.Features, "build-script")
				}
				if _, ok := pkg.Scripts["test"]; ok {
					info.Features = append(info.Features, "test-script")
				}
				if _, ok := pkg.Scripts["lint"]; ok {
					info.Features = append(info.Features, "lint-script")
				}
			}
		}
	}

	return info
}

// BuildSystemInfo holds detailed information about the detected build system.
type BuildSystemInfo struct {
	Name       string            // Primary build system name
	Variant    string            // Variant or flavor (e.g., "kotlin-dsl" for gradle)
	HasWrapper bool              // Whether a wrapper script exists
	Features   []string          // Detected features/capabilities
	Metadata   map[string]string // Additional metadata
}

// detectProjectType identifies if the project is an application, library, or hybrid.
// This helps determine which agents to create (e.g., skip SecurityAuditor for pure libraries).
func (i *Initializer) detectProjectType() string {
	workspace := i.config.Workspace

	// Indicators for application (has runnable entry points)
	appIndicators := 0
	// Indicators for library (exports packages/modules for others to use)
	libIndicators := 0

	// Go project detection
	if _, err := os.Stat(filepath.Join(workspace, "go.mod")); err == nil {
		// Check for cmd/ directory (strong app indicator)
		if _, err := os.Stat(filepath.Join(workspace, "cmd")); err == nil {
			appIndicators += 3
		}
		// Check for main.go in root
		if _, err := os.Stat(filepath.Join(workspace, "main.go")); err == nil {
			appIndicators += 2
		}
		// Check for internal/ directory (app or library with private code)
		if _, err := os.Stat(filepath.Join(workspace, "internal")); err == nil {
			appIndicators += 1
		}
		// Check for pkg/ directory (library exporting public packages)
		if _, err := os.Stat(filepath.Join(workspace, "pkg")); err == nil {
			libIndicators += 2
		}
		// Scan for main function in any .go file
		if i.hasMainFunction(workspace) {
			appIndicators += 2
		}
	}

	// Node.js project detection
	if data, err := os.ReadFile(filepath.Join(workspace, "package.json")); err == nil {
		var pkg struct {
			Main    string            `json:"main"`
			Bin     interface{}       `json:"bin"`
			Scripts map[string]string `json:"scripts"`
			Exports interface{}       `json:"exports"`
			Types   string            `json:"types"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			// CLI tool (bin field present)
			if pkg.Bin != nil {
				appIndicators += 3
			}
			// Has start/serve script (application)
			if _, ok := pkg.Scripts["start"]; ok {
				appIndicators += 2
			}
			if _, ok := pkg.Scripts["serve"]; ok {
				appIndicators += 1
			}
			// Has exports or types field (library)
			if pkg.Exports != nil {
				libIndicators += 2
			}
			if pkg.Types != "" {
				libIndicators += 2
			}
			if pkg.Main != "" && !strings.Contains(pkg.Main, "bin") {
				libIndicators += 1
			}
		}
	}

	// Python project detection
	if data, err := os.ReadFile(filepath.Join(workspace, "pyproject.toml")); err == nil {
		content := string(data)
		// Check for scripts/entry points (application)
		if strings.Contains(content, "[tool.poetry.scripts]") ||
			strings.Contains(content, "[project.scripts]") {
			appIndicators += 3
		}
		// Check for classifiers indicating library
		if strings.Contains(content, "Framework ::") ||
			strings.Contains(content, "Library") {
			libIndicators += 2
		}
	}
	// Check for __main__.py (application entry point)
	if _, err := os.Stat(filepath.Join(workspace, "__main__.py")); err == nil {
		appIndicators += 2
	}
	// Check for setup.py (library distribution)
	if _, err := os.Stat(filepath.Join(workspace, "setup.py")); err == nil {
		libIndicators += 1
	}

	// Rust project detection
	if data, err := os.ReadFile(filepath.Join(workspace, "Cargo.toml")); err == nil {
		content := string(data)
		// Check for [[bin]] sections (application)
		if strings.Contains(content, "[[bin]]") {
			appIndicators += 3
		}
		// Check for src/main.rs (application)
		if _, err := os.Stat(filepath.Join(workspace, "src", "main.rs")); err == nil {
			appIndicators += 2
		}
		// Check for [lib] section (library)
		if strings.Contains(content, "[lib]") {
			libIndicators += 2
		}
		// Check for src/lib.rs (library)
		if _, err := os.Stat(filepath.Join(workspace, "src", "lib.rs")); err == nil {
			libIndicators += 2
		}
	}

	// Java/JVM project detection
	gradleFiles := []string{"build.gradle", "build.gradle.kts"}
	for _, gf := range gradleFiles {
		if data, err := os.ReadFile(filepath.Join(workspace, gf)); err == nil {
			content := string(data)
			// Application plugins
			if strings.Contains(content, "application") {
				appIndicators += 3
			}
			// Library plugins
			if strings.Contains(content, "java-library") ||
				strings.Contains(content, "'maven-publish'") {
				libIndicators += 3
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "pom.xml")); err == nil {
		content := string(data)
		// Executable JAR packaging
		if strings.Contains(content, "<packaging>jar</packaging>") &&
			strings.Contains(content, "<mainClass>") {
			appIndicators += 2
		}
		// POM packaging (library parent)
		if strings.Contains(content, "<packaging>pom</packaging>") {
			libIndicators += 2
		}
	}

	// Container/deployment indicators (strong app signal)
	if _, err := os.Stat(filepath.Join(workspace, "Dockerfile")); err == nil {
		appIndicators += 2
	}
	if _, err := os.Stat(filepath.Join(workspace, "docker-compose.yml")); err == nil {
		appIndicators += 2
	}
	if _, err := os.Stat(filepath.Join(workspace, "docker-compose.yaml")); err == nil {
		appIndicators += 2
	}
	// Kubernetes manifests
	if _, err := os.Stat(filepath.Join(workspace, "k8s")); err == nil {
		appIndicators += 2
	}
	if _, err := os.Stat(filepath.Join(workspace, "kubernetes")); err == nil {
		appIndicators += 2
	}

	// Determine project type based on indicator balance
	if appIndicators > 0 && libIndicators > 0 {
		// Both indicators present
		if appIndicators > libIndicators*2 {
			return "application"
		} else if libIndicators > appIndicators*2 {
			return "library"
		}
		return "hybrid" // Both app and library functionality
	} else if appIndicators > 0 {
		return "application"
	} else if libIndicators > 0 {
		return "library"
	}

	return "unknown"
}

// hasMainFunction checks if any Go file in the workspace has a main function.
func (i *Initializer) hasMainFunction(workspace string) bool {
	// Check common locations
	locations := []string{
		filepath.Join(workspace, "main.go"),
		filepath.Join(workspace, "cmd"),
	}

	for _, loc := range locations {
		info, err := os.Stat(loc)
		if err != nil {
			continue
		}

		if info.IsDir() {
			// Scan directory for main.go files
			err := filepath.Walk(loc, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() && strings.HasSuffix(path, ".go") {
					if content, err := os.ReadFile(path); err == nil {
						if strings.Contains(string(content), "func main()") {
							return filepath.SkipAll // Found main, stop walking
						}
					}
				}
				return nil
			})
			if err == filepath.SkipAll {
				return true
			}
		} else if strings.HasSuffix(loc, ".go") {
			// Check single file
			if content, err := os.ReadFile(loc); err == nil {
				if strings.Contains(string(content), "func main()") {
					return true
				}
			}
		}
	}

	return false
}

// createDirectoryStructure creates the .nerd/ directory and subdirectories.
func (i *Initializer) createDirectoryStructure() (string, error) {
	nerdDir := filepath.Join(i.config.Workspace, ".nerd")
	toolsDir := filepath.Join(nerdDir, "tools")

	dirs := []string{
		nerdDir,
		filepath.Join(nerdDir, "shards"),      // Knowledge shards for specialists
		filepath.Join(nerdDir, "sessions"),    // Session history
		filepath.Join(nerdDir, "cache"),       // Temporary cache
		filepath.Join(nerdDir, "mangle"),      // Mangle logic overlay (User Extensions)
		filepath.Join(nerdDir, "prompts"),     // Prompt atoms (YAML → SQLite)
		toolsDir,                              // Autopoiesis generated tools
		filepath.Join(toolsDir, ".compiled"),  // Compiled tool binaries
		filepath.Join(toolsDir, ".learnings"), // Tool execution learnings
		filepath.Join(toolsDir, ".profiles"),  // Tool quality profiles
		filepath.Join(toolsDir, ".traces"),    // Reasoning traces for tool generation
		filepath.Join(nerdDir, "agents"),      // Persistent agent definitions
		filepath.Join(nerdDir, "campaigns"),   // Campaign checkpoints
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Create .gitignore for .nerd/
	gitignorePath := filepath.Join(nerdDir, ".gitignore")
	gitignoreContent := `# codeNERD local files
knowledge.db
knowledge.db-journal
sessions/
cache/
*.log

# Autopoiesis internal directories (always ignore)
tools/.compiled/
tools/.learnings/
tools/.profiles/
tools/.traces/

# Keep tools/ source and agents/ tracked (user may want to commit generated tools)
# Uncomment below to ignore tool source code:
# tools/*.go
# agents/
`
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return "", fmt.Errorf("failed to create .gitignore: %w", err)
	}

	// Create config.json with defaults if it doesn't exist
	configPath := filepath.Join(nerdDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := i.createDefaultConfig(configPath); err != nil {
			return "", fmt.Errorf("failed to create config.json: %w", err)
		}
	}

	return nerdDir, nil
}

// createDefaultConfig creates a config.json with sensible defaults.
func (i *Initializer) createDefaultConfig(path string) error {
	cfg := &config.UserConfig{
		Provider: "zai",
		Model:    "glm-4.7",
		Theme:    "light",
		ContextWindow: &config.ContextWindowConfig{
			MaxTokens:              128000,
			CoreReservePercent:     5,
			AtomReservePercent:     30,
			HistoryReservePercent:  15,
			WorkingReservePercent:  50,
			RecentTurnWindow:       5,
			CompressionThreshold:   0.80,
			TargetCompressionRatio: 100.0,
			ActivationThreshold:    30.0,
		},
		Embedding: &config.EmbeddingConfig{
			Provider:       "ollama",
			OllamaEndpoint: "http://localhost:11434",
			OllamaModel:    "embeddinggemma",
			GenAIModel:     "gemini-embedding-001",
			TaskType:       "SEMANTIC_SIMILARITY",
		},
		ShardProfiles: map[string]config.ShardProfile{
			"coder": {
				Model:                 "glm-4.7",
				Temperature:           0.7,
				TopP:                  0.9,
				MaxContextTokens:      30000,
				MaxOutputTokens:       6000,
				MaxExecutionTimeSec:   600,
				MaxRetries:            3,
				MaxFactsInShardKernel: 30000,
				EnableLearning:        true,
			},
			"tester": {
				Model:                 "glm-4.7",
				Temperature:           0.5,
				TopP:                  0.9,
				MaxContextTokens:      20000,
				MaxOutputTokens:       4000,
				MaxExecutionTimeSec:   300,
				MaxRetries:            3,
				MaxFactsInShardKernel: 20000,
				EnableLearning:        true,
			},
			"reviewer": {
				Model:                 "glm-4.7",
				Temperature:           0.3,
				TopP:                  0.9,
				MaxContextTokens:      40000,
				MaxOutputTokens:       8000,
				MaxExecutionTimeSec:   900,
				MaxRetries:            2,
				MaxFactsInShardKernel: 30000,
				EnableLearning:        false,
			},
			"researcher": {
				Model:                 "glm-4.7",
				Temperature:           0.6,
				TopP:                  0.95,
				MaxContextTokens:      25000,
				MaxOutputTokens:       5000,
				MaxExecutionTimeSec:   600,
				MaxRetries:            3,
				MaxFactsInShardKernel: 25000,
				EnableLearning:        true,
			},
		},
		DefaultShard: &config.ShardProfile{
			Model:                 "glm-4.7",
			Temperature:           0.7,
			TopP:                  0.9,
			MaxContextTokens:      20000,
			MaxOutputTokens:       4000,
			MaxExecutionTimeSec:   300,
			MaxRetries:            3,
			MaxFactsInShardKernel: 20000,
			EnableLearning:        true,
		},
		CoreLimits: &config.CoreLimits{
			MaxTotalMemoryMB:      12288,
			MaxConcurrentShards:   4,
			MaxSessionDurationMin: 120,
			MaxFactsInKernel:      250000,
			MaxDerivedFactsLimit:  100000,
		},
		// No default MCP servers - internal capabilities use internal packages directly
		Integrations: &config.IntegrationsConfig{
			Servers: make(map[string]config.MCPServerIntegration),
		},
		Execution: &config.ExecutionConfig{
			AllowedBinaries: []string{
				"go", "git", "grep", "ls", "mkdir", "cp", "mv",
				"npm", "npx", "node", "python", "python3", "pip",
				"cargo", "rustc", "make", "cmake",
			},
			DefaultTimeout:   "30s",
			WorkingDirectory: ".",
			AllowedEnvVars:   []string{"PATH", "HOME", "GOPATH", "GOROOT"},
		},
		Logging: &config.LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   "codenerd.log",
		},
		ToolGeneration: &config.ToolGenerationConfig{
			TargetOS:   "windows",
			TargetArch: "amd64",
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}
