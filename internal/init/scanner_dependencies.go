package init

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
