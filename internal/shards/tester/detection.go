package tester

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// =============================================================================
// FRAMEWORK DETECTION
// =============================================================================

// detectFramework auto-detects the testing framework based on file extension or project files.
func (t *TesterShard) detectFramework(target string) string {
	ext := strings.ToLower(filepath.Ext(target))

	switch ext {
	case ".go":
		return "gotest"
	case ".ts", ".tsx", ".js", ".jsx":
		return "jest"
	case ".py":
		return "pytest"
	case ".rs":
		return "cargo"
	case ".java":
		return "junit"
	case ".cs":
		return "xunit"
	case ".rb":
		return "rspec"
	case ".php":
		return "phpunit"
	case ".swift":
		return "xctest"
	default:
		// Check for project files
		return "gotest" // Default to Go
	}
}

// buildTestCommand builds the test command for the given framework.
func (t *TesterShard) buildTestCommand(framework string, task *TesterTask) string {
	target := task.Target
	if target == "" {
		target = "./..."
	}

	switch framework {
	case "gotest":
		if t.testerConfig.VerboseOutput {
			return fmt.Sprintf("go test -v %s", target)
		}
		return fmt.Sprintf("go test %s", target)
	case "jest":
		return fmt.Sprintf("npx jest %s", target)
	case "pytest":
		return fmt.Sprintf("pytest %s", target)
	case "cargo":
		return "cargo test"
	case "junit":
		return "mvn test"
	case "xunit":
		return "dotnet test"
	case "rspec":
		return fmt.Sprintf("rspec %s", target)
	case "phpunit":
		return "vendor/bin/phpunit"
	default:
		return fmt.Sprintf("go test %s", target)
	}
}

// buildCoverageCommand builds the coverage command for the given framework.
func (t *TesterShard) buildCoverageCommand(framework string, task *TesterTask) string {
	target := task.Target
	if target == "" {
		target = "./..."
	}

	switch framework {
	case "gotest":
		return fmt.Sprintf("go test -cover -coverprofile=coverage.out %s", target)
	case "jest":
		return fmt.Sprintf("npx jest --coverage %s", target)
	case "pytest":
		return fmt.Sprintf("pytest --cov=%s", target)
	case "cargo":
		return "cargo tarpaulin"
	default:
		return fmt.Sprintf("go test -cover %s", target)
	}
}

// buildBuildCommand builds the build command for the given framework.
func (t *TesterShard) buildBuildCommand(framework string) string {
	switch framework {
	case "gotest":
		return "go build ./..."
	case "jest":
		return "npm run build"
	case "pytest":
		return "python -m py_compile"
	case "cargo":
		return "cargo build"
	default:
		return "go build ./..."
	}
}

// =============================================================================
// TEST TYPE DETECTION
// =============================================================================

// detectTestType analyzes a test file and returns its type: "unit", "integration", "e2e", or "unknown".
// It examines build tags, imports, test names, and other patterns to classify the test.
func (t *TesterShard) detectTestType(ctx context.Context, testFile string) string {
	if testFile == "" {
		return "unknown"
	}

	// Read file content
	var content string
	if t.virtualStore != nil {
		action := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", testFile},
		}
		var err error
		content, err = t.virtualStore.RouteAction(ctx, action)
		if err != nil {
			return "unknown"
		}
	} else {
		return "unknown"
	}

	// Detect based on framework
	framework := t.detectFramework(testFile)

	switch framework {
	case "gotest":
		return t.detectGoTestType(content)
	case "pytest":
		return t.detectPytestType(content)
	case "jest":
		return t.detectJestTestType(content)
	case "cargo":
		return t.detectRustTestType(content)
	default:
		return t.detectGenericTestType(content)
	}
}

// detectGoTestType detects test type for Go tests.
func (t *TesterShard) detectGoTestType(content string) string {
	lines := strings.Split(content, "\n")

	// Check for build tags (must be at top of file)
	for i := 0; i < 10 && i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		// Look for build constraint comments
		if strings.HasPrefix(line, "// +build") || strings.HasPrefix(line, "//go:build") {
			if strings.Contains(line, "integration") {
				return "integration"
			}
			if strings.Contains(line, "e2e") {
				return "e2e"
			}
		}
	}

	// Check filename patterns
	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, "_integration_test.go") {
		return "integration"
	}
	if strings.Contains(lowerContent, "_e2e_test.go") {
		return "e2e"
	}

	// Check imports for integration test indicators
	integrationImports := []string{
		"database/sql",
		"github.com/docker/",
		"github.com/testcontainers/",
		"net/http/httptest",
		"testing/fstest",
		"io/ioutil",
	}

	e2eImports := []string{
		"github.com/chromedp/",
		"github.com/playwright-community/",
		"github.com/tebeka/selenium",
	}

	for _, importPattern := range integrationImports {
		if strings.Contains(content, importPattern) {
			return "integration"
		}
	}

	for _, importPattern := range e2eImports {
		if strings.Contains(content, importPattern) {
			return "e2e"
		}
	}

	// Check for database-related test patterns
	dbPatterns := []string{
		"testDB", "testDatabase", "setupDB", "setupDatabase",
		"db.Exec", "db.Query", "db.Prepare",
		".Begin()", ".Commit()", ".Rollback()",
		"sql.Open",
	}

	for _, pattern := range dbPatterns {
		if strings.Contains(content, pattern) {
			return "integration"
		}
	}

	// Check for HTTP client patterns (integration)
	httpPatterns := []string{
		"http.NewRequest", "http.Client{", "httptest.NewServer",
		"ListenAndServe", "http.Get(", "http.Post(",
	}

	for _, pattern := range httpPatterns {
		if strings.Contains(content, pattern) {
			return "integration"
		}
	}

	// Check for file system operations (often integration)
	fsPatterns := []string{
		"os.Create", "os.Open", "ioutil.ReadFile", "ioutil.WriteFile",
		"os.MkdirAll", "os.RemoveAll",
	}

	fsCount := 0
	for _, pattern := range fsPatterns {
		if strings.Contains(content, pattern) {
			fsCount++
		}
	}
	if fsCount >= 2 {
		return "integration"
	}

	// Default to unit test if no integration patterns found
	return "unit"
}

// detectPytestType detects test type for Python pytest tests.
func (t *TesterShard) detectPytestType(content string) string {
	lines := strings.Split(content, "\n")

	// Check for pytest markers
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "@pytest.mark.integration") {
			return "integration"
		}
		if strings.Contains(trimmed, "@pytest.mark.e2e") {
			return "e2e"
		}
		if strings.Contains(trimmed, "@pytest.mark.unit") {
			return "unit"
		}
	}

	// Check filename patterns
	if strings.Contains(content, "test_integration") || strings.Contains(content, "integration_test") {
		return "integration"
	}
	if strings.Contains(content, "test_e2e") || strings.Contains(content, "e2e_test") {
		return "e2e"
	}

	// Check imports for integration indicators
	integrationImports := []string{
		"import requests",
		"from requests import",
		"import psycopg2",
		"import pymongo",
		"import redis",
		"from sqlalchemy import",
		"import docker",
		"from testcontainers import",
	}

	e2eImports := []string{
		"from selenium import",
		"from playwright import",
		"import playwright",
	}

	lowerContent := strings.ToLower(content)
	for _, importPattern := range integrationImports {
		if strings.Contains(lowerContent, strings.ToLower(importPattern)) {
			return "integration"
		}
	}

	for _, importPattern := range e2eImports {
		if strings.Contains(lowerContent, strings.ToLower(importPattern)) {
			return "e2e"
		}
	}

	// Check for database/network patterns
	if strings.Contains(content, "db.session") || strings.Contains(content, "Session()") {
		return "integration"
	}

	return "unit"
}

// detectJestTestType detects test type for JavaScript/TypeScript Jest tests.
func (t *TesterShard) detectJestTestType(content string) string {
	lines := strings.Split(content, "\n")

	// Check for describe blocks with integration/e2e keywords
	describeRegex := regexp.MustCompile(`describe\(['"]([^'"]+)['"]`)
	for _, line := range lines {
		matches := describeRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			testName := strings.ToLower(matches[1])
			if strings.Contains(testName, "integration") {
				return "integration"
			}
			if strings.Contains(testName, "e2e") || strings.Contains(testName, "end-to-end") {
				return "e2e"
			}
		}
	}

	// Check filename patterns
	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, ".integration.test.") || strings.Contains(lowerContent, ".integration.spec.") {
		return "integration"
	}
	if strings.Contains(lowerContent, ".e2e.test.") || strings.Contains(lowerContent, ".e2e.spec.") {
		return "e2e"
	}

	// Check imports for integration indicators
	integrationImports := []string{
		"import axios",
		"from 'axios'",
		"import fetch",
		"import supertest",
		"from 'supertest'",
		"import mongodb",
		"import pg",
		"from 'pg'",
		"import redis",
		"@testcontainers/",
	}

	e2eImports := []string{
		"import puppeteer",
		"from 'puppeteer'",
		"import playwright",
		"from 'playwright'",
		"@playwright/test",
	}

	for _, importPattern := range integrationImports {
		if strings.Contains(lowerContent, strings.ToLower(importPattern)) {
			return "integration"
		}
	}

	for _, importPattern := range e2eImports {
		if strings.Contains(lowerContent, strings.ToLower(importPattern)) {
			return "e2e"
		}
	}

	// Check for API call patterns
	apiPatterns := []string{
		".get(", ".post(", ".put(", ".delete(",
		"axios.", "fetch(",
	}

	apiCount := 0
	for _, pattern := range apiPatterns {
		if strings.Contains(content, pattern) {
			apiCount++
		}
	}
	if apiCount >= 2 {
		return "integration"
	}

	return "unit"
}

// detectRustTestType detects test type for Rust tests.
func (t *TesterShard) detectRustTestType(content string) string {
	lines := strings.Split(content, "\n")

	// Check for test attributes
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "#[test]") {
			continue
		}
		if strings.Contains(trimmed, "#[ignore") && strings.Contains(trimmed, "integration") {
			return "integration"
		}
		if strings.Contains(trimmed, "#[ignore") && strings.Contains(trimmed, "e2e") {
			return "e2e"
		}
	}

	// Check for integration test directories (tests/ vs src/)
	if strings.Contains(content, "tests/integration") {
		return "integration"
	}
	if strings.Contains(content, "tests/e2e") {
		return "e2e"
	}

	// Check imports
	integrationImports := []string{
		"use sqlx::",
		"use tokio_postgres::",
		"use reqwest::",
		"use testcontainers::",
	}

	for _, importPattern := range integrationImports {
		if strings.Contains(content, importPattern) {
			return "integration"
		}
	}

	return "unit"
}

// detectGenericTestType provides fallback detection for other frameworks.
func (t *TesterShard) detectGenericTestType(content string) string {
	lowerContent := strings.ToLower(content)

	// Check for common integration/e2e keywords
	if strings.Contains(lowerContent, "integration") {
		return "integration"
	}
	if strings.Contains(lowerContent, "e2e") || strings.Contains(lowerContent, "end-to-end") {
		return "e2e"
	}

	// Check for common integration patterns
	integrationPatterns := []string{
		"database", "http", "api", "network", "docker", "container",
	}

	patternCount := 0
	for _, pattern := range integrationPatterns {
		if strings.Contains(lowerContent, pattern) {
			patternCount++
		}
	}

	if patternCount >= 2 {
		return "integration"
	}

	return "unit"
}
