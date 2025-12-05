// Package researcher - Codebase analysis and dependency scanning.
// This file contains project analysis, dependency extraction, and architectural pattern detection.
package researcher

import (
	"codenerd/internal/core"
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
	if workspace == "" || workspace == "." {
		workspace, _ = os.Getwd()
	}

	fmt.Printf("[Researcher] Analyzing codebase at: %s\n", workspace)

	result := &ResearchResult{
		Query:    "codebase_analysis:" + workspace,
		Keywords: []string{"codebase", "structure", "dependencies"},
		Atoms:    make([]KnowledgeAtom, 0),
	}

	// 1. Scan file topology
	fileFacts, err := r.scanner.ScanWorkspace(workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workspace: %w", err)
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

	return patterns
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
