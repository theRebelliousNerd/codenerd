// Package researcher - Codebase analysis and dependency scanning.
// This file contains project analysis, dependency extraction, and architectural pattern detection.
package researcher

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ProjectType represents detected project characteristics.
type ProjectType struct {
	Language     string
	Framework    string
	BuildSystem  string
	Architecture string
	Confidence   float64
}

// Dependency represents a project dependency.
type Dependency struct {
	Name    string
	Version string
	Type    string // "direct" or "indirect"
}

// analyzeCodebase performs deep analysis of the local codebase.
func (r *ResearcherShard) analyzeCodebase(ctx context.Context, workspace string) (*ResearchResult, error) {
	// Extract workspace path from task string if it looks like a command
	// e.g., "review file:internal\autopoiesis" -> extract just "internal\autopoiesis" or use cwd
	workspace = r.extractWorkspacePath(workspace)

	if workspace == "" || workspace == "." {
		workspace, _ = os.Getwd()
	}

	// Validate workspace is an actual directory
	info, err := os.Stat(workspace)
	if err != nil || !info.IsDir() {
		// Not a valid directory - fall back to current working directory
		logging.Researcher("Invalid workspace '%s', using current directory", workspace)
		workspace, _ = os.Getwd()
	}

	logging.Researcher("Analyzing codebase at: %s", workspace)

	result := &ResearchResult{
		Query:    "codebase_analysis:" + workspace,
		Keywords: []string{"codebase", "structure", "dependencies"},
		Atoms:    make([]KnowledgeAtom, 0),
	}

	// 1. Scan file topology (incremental + cache-aware)
	if r.scanner != nil {
		_, _ = r.scanner.ScanWorkspaceIncremental(ctx, workspace, r.localDB, world.IncrementalOptions{SkipWhenUnchanged: false})
	}

	var fileFacts []core.Fact
	if r.localDB != nil {
		if cached, err := r.localDB.LoadAllWorldFacts("fast"); err == nil && len(cached) > 0 {
			fileFacts = make([]core.Fact, 0, len(cached))
			for _, cf := range cached {
				fileFacts = append(fileFacts, core.Fact{Predicate: cf.Predicate, Args: cf.Args})
			}
		}
	}
	if len(fileFacts) == 0 && r.scanner != nil {
		fresh, err := r.scanner.ScanWorkspace(workspace)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workspace: %w", err)
		}
		fileFacts = fresh
		// Persist snapshot for future boots.
		if r.localDB != nil {
			_ = world.PersistFastSnapshotToDB(r.localDB, fileFacts)
		}
	}
	result.PagesScraped = len(fileFacts)

	// 2. Detect project type
	projectType := r.detectProjectType(workspace)
	result.Atoms = append(result.Atoms, KnowledgeAtom{
		SourceURL:   workspace,
		Title:       "Project Type Detection",
		Content:     fmt.Sprintf("Detected project type: %s", projectType.Language),
		Concept:     "project_identity",
		Confidence:  projectType.Confidence,
		ExtractedAt: time.Now(),
		Metadata: map[string]interface{}{
			"language":     projectType.Language,
			"framework":    projectType.Framework,
			"build_system": projectType.BuildSystem,
			"architecture": projectType.Architecture,
		},
	})

	// 3. Analyze dependencies
	deps := r.analyzeDependencies(workspace, projectType)
	for _, dep := range deps {
		result.Atoms = append(result.Atoms, KnowledgeAtom{
			SourceURL:   workspace,
			Title:       "Dependency: " + dep.Name,
			Content:     fmt.Sprintf("Project depends on %s (version: %s)", dep.Name, dep.Version),
			Concept:     "dependency",
			Confidence:  0.95,
			ExtractedAt: time.Now(),
			Metadata: map[string]interface{}{
				"name":    dep.Name,
				"version": dep.Version,
				"type":    dep.Type,
			},
		})
	}

	// 4. Extract architectural patterns
	patterns := r.detectArchitecturalPatterns(workspace, fileFacts)
	for _, pattern := range patterns {
		result.Atoms = append(result.Atoms, KnowledgeAtom{
			SourceURL:   workspace,
			Title:       "Architectural Pattern: " + pattern,
			Content:     fmt.Sprintf("Detected architectural pattern: %s", pattern),
			Concept:     "architecture",
			Confidence:  0.8,
			ExtractedAt: time.Now(),
		})
	}

	// 5. Find important files (README, config, entry points)
	importantFiles := r.findImportantFiles(workspace)
	for _, file := range importantFiles {
		content, _ := os.ReadFile(file)
		summary := r.summarizeFile(file, string(content))
		result.Atoms = append(result.Atoms, KnowledgeAtom{
			SourceURL:   file,
			Title:       "Important File: " + filepath.Base(file),
			Content:     summary,
			Concept:     "key_file",
			Confidence:  0.9,
			ExtractedAt: time.Now(),
		})
	}

	// 5a. Robust Documentation Ingestion (Docs/, specs, etc.)
	docAtoms, err := r.IngestDocumentation(ctx, workspace)
	if err == nil && len(docAtoms) > 0 {
		logging.Researcher("Ingested %d documentation atoms", len(docAtoms))
		result.Atoms = append(result.Atoms, docAtoms...)
	} else if err != nil {
		logging.Researcher("Warning: Documentation ingestion failed: %v", err)
	}

	// 6. Generate summary using LLM if available
	if r.llmClient != nil {
		summary, err := r.generateCodebaseSummary(ctx, result)
		if err == nil {
			result.Summary = summary
		}
	} else {
		result.Summary = fmt.Sprintf("Analyzed %d files. Project: %s (%s). Dependencies: %d. Patterns: %v",
			len(fileFacts), projectType.Language, projectType.Framework, len(deps), patterns)
	}

	result.Duration = time.Since(r.startTime)
	result.FactsGenerated = len(result.Atoms)

	return result, nil
}

// detectProjectType analyzes the workspace to determine project type.
func (r *ResearcherShard) detectProjectType(workspace string) ProjectType {
	pt := ProjectType{
		Language:     "unknown",
		Architecture: "unknown",
		Confidence:   0.5,
	}

	// Check for language markers
	markers := map[string]struct {
		lang      string
		framework string
		build     string
	}{
		"go.mod":           {"go", "", "go"},
		"go.sum":           {"go", "", "go"},
		"Cargo.toml":       {"rust", "", "cargo"},
		"package.json":     {"javascript", "", "npm"},
		"requirements.txt": {"python", "", "pip"},
		"pyproject.toml":   {"python", "", "poetry"},
		"pom.xml":          {"java", "", "maven"},
		"build.gradle":     {"java", "", "gradle"},
		"Gemfile":          {"ruby", "rails", "bundler"},
		"composer.json":    {"php", "", "composer"},
	}

	for file, info := range markers {
		if _, err := os.Stat(filepath.Join(workspace, file)); err == nil {
			pt.Language = info.lang
			pt.BuildSystem = info.build
			pt.Confidence = 0.95
			if info.framework != "" {
				pt.Framework = info.framework
			}
			break
		}
	}

	// Detect frameworks
	if pt.Language == "javascript" {
		if content, err := os.ReadFile(filepath.Join(workspace, "package.json")); err == nil {
			s := string(content)
			switch {
			case strings.Contains(s, `"next"`):
				pt.Framework = "nextjs"
			case strings.Contains(s, `"react"`):
				pt.Framework = "react"
			case strings.Contains(s, `"vue"`):
				pt.Framework = "vue"
			case strings.Contains(s, `"express"`):
				pt.Framework = "express"
			}
		}
	}

	if pt.Language == "go" {
		if content, err := os.ReadFile(filepath.Join(workspace, "go.mod")); err == nil {
			s := string(content)
			switch {
			case strings.Contains(s, "gin-gonic"):
				pt.Framework = "gin"
			case strings.Contains(s, "echo"):
				pt.Framework = "echo"
			case strings.Contains(s, "fiber"):
				pt.Framework = "fiber"
			case strings.Contains(s, "chi"):
				pt.Framework = "chi"
			}
		}
	}

	// Detect architecture
	dirs := []string{"cmd", "internal", "pkg", "api", "web", "services", "handlers"}
	foundDirs := 0
	for _, dir := range dirs {
		if info, err := os.Stat(filepath.Join(workspace, dir)); err == nil && info.IsDir() {
			foundDirs++
		}
	}

	if foundDirs >= 3 {
		pt.Architecture = "clean_architecture"
	} else if _, err := os.Stat(filepath.Join(workspace, "docker-compose.yml")); err == nil {
		pt.Architecture = "microservices"
	} else {
		pt.Architecture = "monolith"
	}

	return pt
}

// analyzeDependencies extracts project dependencies.
func (r *ResearcherShard) analyzeDependencies(workspace string, pt ProjectType) []Dependency {
	var deps []Dependency

	switch pt.Language {
	case "go":
		deps = r.parseGoMod(filepath.Join(workspace, "go.mod"))
	case "javascript":
		deps = r.parsePackageJSON(filepath.Join(workspace, "package.json"))
	case "python":
		deps = r.parseRequirements(filepath.Join(workspace, "requirements.txt"))
	case "rust":
		deps = r.parseCargoToml(filepath.Join(workspace, "Cargo.toml"))
	}

	return deps
}

// parseGoMod extracts dependencies from go.mod.
func (r *ResearcherShard) parseGoMod(path string) []Dependency {
	var deps []Dependency
	content, err := os.ReadFile(path)
	if err != nil {
		return deps
	}

	lines := strings.Split(string(content), "\n")
	inRequire := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			continue
		}
		if inRequire || strings.HasPrefix(line, "require ") {
			parts := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(parts) >= 2 {
				depType := "direct"
				if strings.Contains(line, "// indirect") {
					depType = "indirect"
				}
				deps = append(deps, Dependency{
					Name:    parts[0],
					Version: parts[1],
					Type:    depType,
				})
			}
		}
	}

	return deps
}

// parsePackageJSON extracts dependencies from package.json.
func (r *ResearcherShard) parsePackageJSON(path string) []Dependency {
	var deps []Dependency
	content, err := os.ReadFile(path)
	if err != nil {
		return deps
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return deps
	}

	for name, version := range pkg.Dependencies {
		deps = append(deps, Dependency{Name: name, Version: version, Type: "direct"})
	}
	for name, version := range pkg.DevDependencies {
		deps = append(deps, Dependency{Name: name, Version: version, Type: "dev"})
	}

	return deps
}

// parseRequirements extracts dependencies from requirements.txt.
func (r *ResearcherShard) parseRequirements(path string) []Dependency {
	var deps []Dependency
	content, err := os.ReadFile(path)
	if err != nil {
		return deps
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Handle various formats: pkg==1.0, pkg>=1.0, pkg
		re := regexp.MustCompile(`^([a-zA-Z0-9_-]+)([<>=!]+)?(.*)$`)
		if matches := re.FindStringSubmatch(line); len(matches) > 0 {
			deps = append(deps, Dependency{
				Name:    matches[1],
				Version: matches[2] + matches[3],
				Type:    "direct",
			})
		}
	}

	return deps
}

// parseCargoToml extracts dependencies from Cargo.toml.
func (r *ResearcherShard) parseCargoToml(path string) []Dependency {
	var deps []Dependency
	content, err := os.ReadFile(path)
	if err != nil {
		return deps
	}

	lines := strings.Split(string(content), "\n")
	inDeps := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[dependencies]") {
			inDeps = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inDeps = false
			continue
		}
		if inDeps && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				version := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				deps = append(deps, Dependency{Name: name, Version: version, Type: "direct"})
			}
		}
	}

	return deps
}

// detectArchitecturalPatterns identifies common patterns in the codebase.
func (r *ResearcherShard) detectArchitecturalPatterns(workspace string, facts []core.Fact) []string {
	var patterns []string

	// Check directory structure from facts
	dirs := make(map[string]bool)
	for _, fact := range facts {
		if fact.Predicate == "directory" && len(fact.Args) > 0 {
			if path, ok := fact.Args[0].(string); ok {
				rel, _ := filepath.Rel(workspace, path)
				dirs[rel] = true
			}
		}
	}

	// Detect patterns based on structure
	if dirs["cmd"] && dirs["internal"] && dirs["pkg"] {
		patterns = append(patterns, "standard_go_layout")
	}
	if dirs["src"] && dirs["tests"] {
		patterns = append(patterns, "src_tests_separation")
	}
	if dirs["api"] || dirs["handlers"] || dirs["routes"] {
		patterns = append(patterns, "api_layer")
	}
	if dirs["services"] || dirs["usecases"] {
		patterns = append(patterns, "service_layer")
	}
	if dirs["repository"] || dirs["store"] || dirs["db"] {
		patterns = append(patterns, "repository_pattern")
	}
	if dirs["domain"] || dirs["entities"] || dirs["models"] {
		patterns = append(patterns, "domain_driven")
	}

	// Check for specific files
	if _, err := os.Stat(filepath.Join(workspace, "Dockerfile")); err == nil {
		patterns = append(patterns, "containerized")
	}
	if _, err := os.Stat(filepath.Join(workspace, ".github/workflows")); err == nil {
		patterns = append(patterns, "ci_cd_github_actions")
	}

	// Extended architecture pattern detection

	// Serverless patterns
	serverlessPatterns := r.detectServerlessPatterns(workspace, dirs)
	patterns = append(patterns, serverlessPatterns...)

	// Event-driven patterns
	eventDrivenPatterns := r.detectEventDrivenPatterns(workspace)
	patterns = append(patterns, eventDrivenPatterns...)

	// Monorepo patterns
	monorepoPatterns := r.detectMonorepoPatterns(workspace)
	patterns = append(patterns, monorepoPatterns...)

	// GraphQL patterns
	if r.detectGraphQLPatterns(workspace) {
		patterns = append(patterns, "graphql_api")
	}

	// gRPC patterns
	if r.detectGRPCPatterns(workspace) {
		patterns = append(patterns, "grpc_services")
	}

	// Plugin-based architecture
	if r.detectPluginArchitecture(workspace, dirs) {
		patterns = append(patterns, "plugin_based")
	}

	// Hexagonal/Ports-Adapters architecture
	if dirs["ports"] || dirs["adapters"] || (dirs["domain"] && dirs["infrastructure"]) {
		patterns = append(patterns, "hexagonal_architecture")
	}

	// CQRS pattern
	if dirs["commands"] && dirs["queries"] {
		patterns = append(patterns, "cqrs")
	}

	return patterns
}

// detectServerlessPatterns identifies serverless architecture patterns.
func (r *ResearcherShard) detectServerlessPatterns(workspace string, dirs map[string]bool) []string {
	var patterns []string

	// AWS Lambda detection
	lambdaIndicators := []string{
		"serverless.yml", "serverless.yaml", // Serverless Framework
		"sam.yaml", "template.yaml", // AWS SAM
		"cdk.json", // AWS CDK
	}
	for _, indicator := range lambdaIndicators {
		if _, err := os.Stat(filepath.Join(workspace, indicator)); err == nil {
			patterns = append(patterns, "serverless_aws")
			break
		}
	}

	// Check for Lambda handler patterns in code
	if dirs["functions"] || dirs["lambdas"] || dirs["handlers"] {
		if _, err := os.Stat(filepath.Join(workspace, "serverless.yml")); err == nil {
			patterns = append(patterns, "serverless_framework")
		}
	}

	// Google Cloud Functions
	if _, err := os.Stat(filepath.Join(workspace, "cloudbuild.yaml")); err == nil {
		patterns = append(patterns, "serverless_gcp")
	}

	// Azure Functions
	if _, err := os.Stat(filepath.Join(workspace, "host.json")); err == nil {
		if _, err := os.Stat(filepath.Join(workspace, "function.json")); err == nil {
			patterns = append(patterns, "serverless_azure")
		}
	}

	// Vercel/Netlify Functions
	if dirs["api"] {
		if _, err := os.Stat(filepath.Join(workspace, "vercel.json")); err == nil {
			patterns = append(patterns, "serverless_vercel")
		}
		if _, err := os.Stat(filepath.Join(workspace, "netlify.toml")); err == nil {
			patterns = append(patterns, "serverless_netlify")
		}
	}

	return patterns
}

// detectEventDrivenPatterns identifies event-driven architecture patterns.
func (r *ResearcherShard) detectEventDrivenPatterns(workspace string) []string {
	var patterns []string

	// Check go.mod or package.json for event-driven dependencies
	if content, err := os.ReadFile(filepath.Join(workspace, "go.mod")); err == nil {
		contentStr := string(content)

		// Kafka
		if strings.Contains(contentStr, "segmentio/kafka-go") ||
			strings.Contains(contentStr, "confluentinc/confluent-kafka-go") ||
			strings.Contains(contentStr, "Shopify/sarama") {
			patterns = append(patterns, "event_driven_kafka")
		}

		// RabbitMQ
		if strings.Contains(contentStr, "streadway/amqp") ||
			strings.Contains(contentStr, "rabbitmq/amqp091-go") {
			patterns = append(patterns, "event_driven_rabbitmq")
		}

		// NATS
		if strings.Contains(contentStr, "nats-io/nats.go") {
			patterns = append(patterns, "event_driven_nats")
		}

		// Redis Pub/Sub or Streams
		if strings.Contains(contentStr, "go-redis/redis") ||
			strings.Contains(contentStr, "redis/go-redis") {
			patterns = append(patterns, "event_driven_redis")
		}
	}

	// Check package.json for Node.js projects
	if content, err := os.ReadFile(filepath.Join(workspace, "package.json")); err == nil {
		contentStr := string(content)

		if strings.Contains(contentStr, `"kafkajs"`) {
			patterns = append(patterns, "event_driven_kafka")
		}
		if strings.Contains(contentStr, `"amqplib"`) {
			patterns = append(patterns, "event_driven_rabbitmq")
		}
		if strings.Contains(contentStr, `"nats"`) {
			patterns = append(patterns, "event_driven_nats")
		}
	}

	// Check for event/message directories
	eventDirs := []string{"events", "messages", "subscribers", "publishers", "consumers", "producers"}
	for _, dir := range eventDirs {
		if _, err := os.Stat(filepath.Join(workspace, dir)); err == nil {
			if len(patterns) == 0 {
				patterns = append(patterns, "event_driven")
			}
			break
		}
	}

	return patterns
}

// detectMonorepoPatterns identifies monorepo architecture patterns.
func (r *ResearcherShard) detectMonorepoPatterns(workspace string) []string {
	var patterns []string

	// pnpm workspaces
	if _, err := os.Stat(filepath.Join(workspace, "pnpm-workspace.yaml")); err == nil {
		patterns = append(patterns, "monorepo_pnpm")
	}

	// Nx
	if _, err := os.Stat(filepath.Join(workspace, "nx.json")); err == nil {
		patterns = append(patterns, "monorepo_nx")
	}

	// Turborepo
	if _, err := os.Stat(filepath.Join(workspace, "turbo.json")); err == nil {
		patterns = append(patterns, "monorepo_turborepo")
	}

	// Lerna
	if _, err := os.Stat(filepath.Join(workspace, "lerna.json")); err == nil {
		patterns = append(patterns, "monorepo_lerna")
	}

	// Rush
	if _, err := os.Stat(filepath.Join(workspace, "rush.json")); err == nil {
		patterns = append(patterns, "monorepo_rush")
	}

	// Yarn workspaces (check package.json for workspaces field)
	if content, err := os.ReadFile(filepath.Join(workspace, "package.json")); err == nil {
		if strings.Contains(string(content), `"workspaces"`) {
			patterns = append(patterns, "monorepo_yarn_workspaces")
		}
	}

	// Go workspace
	if _, err := os.Stat(filepath.Join(workspace, "go.work")); err == nil {
		patterns = append(patterns, "monorepo_go_workspace")
	}

	// Generic packages directory
	if _, err := os.Stat(filepath.Join(workspace, "packages")); err == nil {
		if len(patterns) == 0 {
			patterns = append(patterns, "monorepo")
		}
	}

	return patterns
}

// detectGraphQLPatterns checks for GraphQL API patterns.
func (r *ResearcherShard) detectGraphQLPatterns(workspace string) bool {
	// Check for GraphQL schema files
	schemaFiles := []string{
		"schema.graphql", "schema.gql",
		"*.graphql", "graphql/schema.graphql",
	}

	for _, pattern := range schemaFiles {
		if matches, _ := filepath.Glob(filepath.Join(workspace, pattern)); len(matches) > 0 {
			return true
		}
	}

	// Check for gqlgen (Go GraphQL)
	if _, err := os.Stat(filepath.Join(workspace, "gqlgen.yml")); err == nil {
		return true
	}

	// Check dependencies
	if content, err := os.ReadFile(filepath.Join(workspace, "go.mod")); err == nil {
		if strings.Contains(string(content), "99designs/gqlgen") ||
			strings.Contains(string(content), "graphql-go/graphql") {
			return true
		}
	}

	if content, err := os.ReadFile(filepath.Join(workspace, "package.json")); err == nil {
		if strings.Contains(string(content), `"graphql"`) ||
			strings.Contains(string(content), `"apollo-server"`) ||
			strings.Contains(string(content), `"@graphql"`) {
			return true
		}
	}

	return false
}

// detectGRPCPatterns checks for gRPC service patterns.
func (r *ResearcherShard) detectGRPCPatterns(workspace string) bool {
	// Check for proto files
	if matches, _ := filepath.Glob(filepath.Join(workspace, "**/*.proto")); len(matches) > 0 {
		return true
	}

	// Check proto directory
	if _, err := os.Stat(filepath.Join(workspace, "proto")); err == nil {
		return true
	}

	// Check for buf configuration (modern proto management)
	if _, err := os.Stat(filepath.Join(workspace, "buf.yaml")); err == nil {
		return true
	}

	// Check dependencies
	if content, err := os.ReadFile(filepath.Join(workspace, "go.mod")); err == nil {
		if strings.Contains(string(content), "google.golang.org/grpc") ||
			strings.Contains(string(content), "google.golang.org/protobuf") {
			return true
		}
	}

	if content, err := os.ReadFile(filepath.Join(workspace, "package.json")); err == nil {
		if strings.Contains(string(content), `"@grpc/grpc-js"`) ||
			strings.Contains(string(content), `"grpc"`) {
			return true
		}
	}

	return false
}

// detectPluginArchitecture checks for plugin-based architecture patterns.
func (r *ResearcherShard) detectPluginArchitecture(workspace string, dirs map[string]bool) bool {
	// Check for plugin directories
	pluginDirs := []string{"plugins", "extensions", "addons", "modules"}
	for _, dir := range pluginDirs {
		if dirs[dir] {
			return true
		}
	}

	// Check for Go plugin patterns
	if content, err := os.ReadFile(filepath.Join(workspace, "go.mod")); err == nil {
		if strings.Contains(string(content), "plugin") {
			// Look for plugin.Open usage in main files
			if matches, _ := filepath.Glob(filepath.Join(workspace, "cmd/**/main.go")); len(matches) > 0 {
				for _, match := range matches {
					if fileContent, err := os.ReadFile(match); err == nil {
						if strings.Contains(string(fileContent), "plugin.Open") {
							return true
						}
					}
				}
			}
		}
	}

	// Check for Hashicorp go-plugin pattern
	if content, err := os.ReadFile(filepath.Join(workspace, "go.mod")); err == nil {
		if strings.Contains(string(content), "hashicorp/go-plugin") {
			return true
		}
	}

	return false
}

// findImportantFiles locates key files in the workspace.
func (r *ResearcherShard) findImportantFiles(workspace string) []string {
	important := []string{
		"README.md", "README", "readme.md",
		"CLAUDE.md", ".claude",
		"go.mod", "package.json", "Cargo.toml", "requirements.txt",
		"Makefile", "docker-compose.yml", "Dockerfile",
		".env.example", "config.yaml", "config.json",
	}

	var found []string
	for _, file := range important {
		path := filepath.Join(workspace, file)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
		}
	}

	// Also find main entry points
	entryPatterns := []string{
		"main.go", "cmd/*/main.go",
		"index.js", "index.ts", "app.js", "app.ts",
		"main.py", "app.py", "__main__.py",
		"main.rs", "lib.rs",
	}
	for _, pattern := range entryPatterns {
		matches, _ := filepath.Glob(filepath.Join(workspace, pattern))
		found = append(found, matches...)
	}

	return found
}

// summarizeFile creates a brief summary of a file's purpose.
func (r *ResearcherShard) summarizeFile(path string, content string) string {
	base := filepath.Base(path)

	// For known files, provide specific summaries
	switch base {
	case "go.mod":
		lines := strings.Split(content, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "module ") {
			return fmt.Sprintf("Go module: %s", strings.TrimPrefix(lines[0], "module "))
		}
	case "package.json":
		var pkg struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if json.Unmarshal([]byte(content), &pkg) == nil {
			return fmt.Sprintf("NPM package: %s - %s", pkg.Name, pkg.Description)
		}
	case "README.md", "readme.md":
		// Extract first paragraph
		lines := strings.Split(content, "\n")
		var summary strings.Builder
		for _, line := range lines {
			if strings.TrimSpace(line) == "" && summary.Len() > 0 {
				break
			}
			if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
				summary.WriteString(line + " ")
			}
		}
		if summary.Len() > 200 {
			return summary.String()[:200] + "..."
		}
		return summary.String()
	}

	// Generic summary: first non-empty line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#") {
			if len(line) > 100 {
				return line[:100] + "..."
			}
			return line
		}
	}

	return fmt.Sprintf("File: %s (%d bytes)", base, len(content))
}

// generateCodebaseSummary uses LLM to create a comprehensive summary.
func (r *ResearcherShard) generateCodebaseSummary(ctx context.Context, result *ResearchResult) (string, error) {
	// Build context from atoms
	var contextStr strings.Builder
	contextStr.WriteString("Analyzed codebase with the following findings:\n\n")
	for _, atom := range result.Atoms {
		contextStr.WriteString(fmt.Sprintf("- %s: %s\n", atom.Title, atom.Content))
	}

	prompt := fmt.Sprintf(`Based on this codebase analysis, provide a concise 2-3 sentence summary suitable for an AI coding agent to understand the project context:

%s

Summary (2-3 sentences):`, contextStr.String())

	summary, err := r.llmComplete(ctx, prompt)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(summary), nil
}

// extractWorkspacePath extracts a valid workspace path from a task string.
// Handles cases where the researcher is called with command-style tasks like "review file:path"
// that aren't valid directory paths.
func (r *ResearcherShard) extractWorkspacePath(task string) string {
	task = strings.TrimSpace(task)

	// If task is empty or ".", return as-is (will use cwd)
	if task == "" || task == "." {
		return task
	}

	// Check if it looks like a command with file: or files: prefix
	// e.g., "review file:internal/autopoiesis" or "security_scan files:a.go,b.go"
	if strings.Contains(task, "file:") {
		// Extract path after file:
		parts := strings.SplitN(task, "file:", 2)
		if len(parts) > 1 {
			path := strings.TrimSpace(parts[1])
			// Remove any trailing command parts
			if idx := strings.Index(path, " "); idx > 0 {
				path = path[:idx]
			}
			// Check if extracted path is a valid directory
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				return path
			}
			// If not a directory, extract parent directory
			dir := filepath.Dir(path)
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir
			}
		}
	}

	// Check if task is a valid directory path directly
	if info, err := os.Stat(task); err == nil && info.IsDir() {
		return task
	}

	// Check if first word is a valid directory
	parts := strings.Fields(task)
	if len(parts) > 0 {
		if info, err := os.Stat(parts[0]); err == nil && info.IsDir() {
			return parts[0]
		}
	}

	// Not a valid directory - return empty to trigger fallback to cwd
	return ""
}
