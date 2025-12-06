// Package shards implements specialized ShardAgent types for the Cortex 1.5.0 architecture.
// This file implements mock generation automation for TesterShard.
package shards

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// =============================================================================
// MOCK GENERATION
// =============================================================================

// MockInfo represents information about a mock file.
type MockInfo struct {
	MockFile      string    // Path to the mock file (e.g., mock_interface.go)
	InterfaceFile string    // Path to the interface definition file
	InterfaceName string    // Name of the interface
	PackageName   string    // Package name
	IsStale       bool      // Whether the mock is outdated
	LastModified  time.Time // Last modification time of mock file
}

// detectStaleMocks identifies outdated mocks in a test file.
// It scans for mock imports and checks if interface signatures have changed.
func (t *TesterShard) detectStaleMocks(ctx context.Context, testFile string) ([]string, error) {
	if t.virtualStore == nil {
		return nil, fmt.Errorf("virtualStore required for file operations")
	}

	// Read the test file
	readAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/read_file", testFile},
	}
	content, err := t.virtualStore.RouteAction(ctx, readAction)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	staleMocks := make([]string, 0)

	// Detect mock imports
	mockImports := t.extractMockImports(content, testFile)

	for _, mockInfo := range mockImports {
		isStale, err := t.isMockStale(ctx, mockInfo)
		if err != nil {
			fmt.Printf("[TesterShard] Warning: failed to check staleness for %s: %v\n", mockInfo.MockFile, err)
			continue
		}

		if isStale {
			staleMocks = append(staleMocks, mockInfo.InterfaceFile)
			fmt.Printf("[TesterShard] Detected stale mock: %s (interface: %s)\n",
				mockInfo.MockFile, mockInfo.InterfaceFile)
		}
	}

	return staleMocks, nil
}

// extractMockImports extracts mock file references from test file content.
func (t *TesterShard) extractMockImports(content, testFile string) []MockInfo {
	mocks := make([]MockInfo, 0)
	dir := filepath.Dir(testFile)

	// Pattern 1: Go files with mock_ prefix in same directory
	mockFilePattern := regexp.MustCompile(`mock_(\w+)\.go`)
	matches := mockFilePattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			mockFile := filepath.Join(dir, "mock_"+match[1]+".go")
			interfaceFile := filepath.Join(dir, match[1]+".go")
			mocks = append(mocks, MockInfo{
				MockFile:      mockFile,
				InterfaceFile: interfaceFile,
				InterfaceName: capitalize(match[1]),
			})
		}
	}

	// Pattern 2: Import statements for mock packages
	importPattern := regexp.MustCompile(`import\s+(?:.*?)\s+"([^"]*mocks?[^"]*)"`)
	importMatches := importPattern.FindAllStringSubmatch(content, -1)
	for _, match := range importMatches {
		if len(match) > 1 {
			// Extract package path from import
			importPath := match[1]
			mocks = append(mocks, MockInfo{
				MockFile:      importPath,
				InterfaceFile: strings.TrimSuffix(importPath, "/mocks"),
			})
		}
	}

	// Pattern 3: Check for gomock usage
	if strings.Contains(content, "gomock.NewController") || strings.Contains(content, "gomock.Controller") {
		// This test uses gomock - scan for mock types
		mockTypePattern := regexp.MustCompile(`\*Mock(\w+)`)
		typeMatches := mockTypePattern.FindAllStringSubmatch(content, -1)
		for _, match := range typeMatches {
			if len(match) > 1 {
				interfaceName := match[1]
				mockFile := filepath.Join(dir, "mock_"+strings.ToLower(interfaceName)+".go")
				mocks = append(mocks, MockInfo{
					MockFile:      mockFile,
					InterfaceName: interfaceName,
				})
			}
		}
	}

	return mocks
}

// isMockStale checks if a mock file is outdated compared to its interface.
func (t *TesterShard) isMockStale(ctx context.Context, mockInfo MockInfo) (bool, error) {
	if t.virtualStore == nil {
		return false, fmt.Errorf("virtualStore required")
	}

	// Check if mock file exists
	statAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/stat_file", mockInfo.MockFile},
	}
	_, err := t.virtualStore.RouteAction(ctx, statAction)
	if err != nil {
		// Mock file doesn't exist - it's stale by definition
		return true, nil
	}

	// If interface file is specified, check modification times
	if mockInfo.InterfaceFile != "" {
		// Read interface file to detect changes
		readAction := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", mockInfo.InterfaceFile},
		}
		interfaceContent, err := t.virtualStore.RouteAction(ctx, readAction)
		if err != nil {
			return false, fmt.Errorf("failed to read interface file: %w", err)
		}

		// Read mock file
		readMockAction := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", mockInfo.MockFile},
		}
		mockContent, err := t.virtualStore.RouteAction(ctx, readMockAction)
		if err != nil {
			return false, fmt.Errorf("failed to read mock file: %w", err)
		}

		// Check if interface methods are present in mock
		interfaceMethods := t.extractInterfaceMethods(interfaceContent, mockInfo.InterfaceName)
		mockMethods := t.extractMockMethods(mockContent)

		// If interface has methods not in mock, it's stale
		for _, method := range interfaceMethods {
			if !contains(mockMethods, method) {
				fmt.Printf("[TesterShard] Mock missing method: %s\n", method)
				return true, nil
			}
		}
	}

	return false, nil
}

// extractInterfaceMethods extracts method signatures from interface definition.
func (t *TesterShard) extractInterfaceMethods(content, interfaceName string) []string {
	methods := make([]string, 0)

	if interfaceName == "" {
		// Try to find any interface
		interfacePattern := regexp.MustCompile(`type\s+(\w+)\s+interface\s*\{([^}]+)\}`)
		matches := interfacePattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				interfaceBody := match[2]
				methods = append(methods, t.parseInterfaceMethods(interfaceBody)...)
			}
		}
		return methods
	}

	// Find specific interface definition
	interfacePattern := regexp.MustCompile(`type\s+` + interfaceName + `\s+interface\s*\{([^}]+)\}`)
	matches := interfacePattern.FindStringSubmatch(content)
	if len(matches) < 2 {
		return methods
	}

	interfaceBody := matches[1]
	return t.parseInterfaceMethods(interfaceBody)
}

// parseInterfaceMethods parses method names from interface body.
func (t *TesterShard) parseInterfaceMethods(interfaceBody string) []string {
	methods := make([]string, 0)

	// Extract method names
	methodPattern := regexp.MustCompile(`(\w+)\s*\([^)]*\)`)
	methodMatches := methodPattern.FindAllStringSubmatch(interfaceBody, -1)
	for _, match := range methodMatches {
		if len(match) > 1 {
			methods = append(methods, match[1])
		}
	}

	return methods
}

// extractMockMethods extracts implemented methods from mock file.
func (t *TesterShard) extractMockMethods(content string) []string {
	methods := make([]string, 0)

	// Pattern: func (m *MockType) MethodName(...)
	methodPattern := regexp.MustCompile(`func\s+\([^)]+\*Mock\w+\)\s+(\w+)\s*\(`)
	matches := methodPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			methods = append(methods, match[1])
		}
	}

	return methods
}

// regenerateMock regenerates a mock file for the given interface.
func (t *TesterShard) regenerateMock(ctx context.Context, interfacePath string) error {
	if t.virtualStore == nil {
		return fmt.Errorf("virtualStore required for mock regeneration")
	}

	t.mu.RLock()
	framework := t.testerConfig.Framework
	t.mu.RUnlock()

	if framework == "auto" {
		framework = t.detectFramework(interfacePath)
	}

	fmt.Printf("[TesterShard] Regenerating mock for interface: %s\n", interfacePath)

	switch framework {
	case "gotest":
		return t.regenerateGoMock(ctx, interfacePath)
	case "jest":
		return t.regenerateJestMock(ctx, interfacePath)
	case "pytest":
		return t.regeneratePytestMock(ctx, interfacePath)
	default:
		return fmt.Errorf("mock generation not supported for framework: %s", framework)
	}
}

// regenerateGoMock regenerates Go mocks using mockgen or LLM.
func (t *TesterShard) regenerateGoMock(ctx context.Context, interfacePath string) error {
	// Read the interface file to extract interface names
	readAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/read_file", interfacePath},
	}
	content, err := t.virtualStore.RouteAction(ctx, readAction)
	if err != nil {
		return fmt.Errorf("failed to read interface file: %w", err)
	}

	// Extract package and interface names
	packageName := t.extractPackageName(content)
	interfaceNames := t.extractInterfaceNames(content)

	if len(interfaceNames) == 0 {
		return fmt.Errorf("no interfaces found in %s", interfacePath)
	}

	dir := filepath.Dir(interfacePath)
	baseFilename := filepath.Base(interfacePath)
	baseFilename = strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename))

	// Generate mock file path
	mockFilePath := filepath.Join(dir, "mock_"+baseFilename+".go")

	// Check if mockgen is available
	checkCmd := "which mockgen"
	if strings.Contains(strings.ToLower(t.testerConfig.WorkingDir), "windows") {
		checkCmd = "where mockgen"
	}

	checkAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/run_command", checkCmd},
	}
	_, err = t.virtualStore.RouteAction(ctx, checkAction)

	if err != nil {
		// mockgen not available, generate mock manually via LLM
		fmt.Printf("[TesterShard] mockgen not found, using LLM for mock generation\n")
		return t.generateMockViaLLM(ctx, interfacePath, mockFilePath, packageName, interfaceNames)
	}

	// Build mockgen command
	mockgenCmd := fmt.Sprintf("mockgen -source=%s -destination=%s -package=%s",
		interfacePath, mockFilePath, packageName)

	// Execute mockgen
	execAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/run_command", mockgenCmd},
	}
	_, err = t.virtualStore.RouteAction(ctx, execAction)
	if err != nil {
		fmt.Printf("[TesterShard] mockgen failed, falling back to LLM: %v\n", err)
		return t.generateMockViaLLM(ctx, interfacePath, mockFilePath, packageName, interfaceNames)
	}

	fmt.Printf("[TesterShard] Generated mock: %s\n", mockFilePath)

	// Assert facts about mock generation
	if t.kernel != nil {
		_ = t.kernel.Assert(core.Fact{
			Predicate: "mock_generated",
			Args:      []interface{}{mockFilePath, interfacePath, time.Now().Unix()},
		})
	}

	return nil
}

// regenerateJestMock regenerates Jest/TypeScript mocks.
func (t *TesterShard) regenerateJestMock(ctx context.Context, interfacePath string) error {
	// Jest typically uses __mocks__ directory or inline mocks
	dir := filepath.Dir(interfacePath)
	mockDir := filepath.Join(dir, "__mocks__")

	// Create mock directory if it doesn't exist
	mkdirAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/create_dir", mockDir},
	}
	_, _ = t.virtualStore.RouteAction(ctx, mkdirAction)

	baseFilename := filepath.Base(interfacePath)
	mockFilePath := filepath.Join(mockDir, baseFilename)

	// Read interface to generate mock
	readAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/read_file", interfacePath},
	}
	content, err := t.virtualStore.RouteAction(ctx, readAction)
	if err != nil {
		return fmt.Errorf("failed to read interface file: %w", err)
	}

	interfaceNames := t.extractTypeScriptInterfaces(content)
	return t.generateMockViaLLM(ctx, interfacePath, mockFilePath, "", interfaceNames)
}

// regeneratePytestMock regenerates Python mocks.
func (t *TesterShard) regeneratePytestMock(ctx context.Context, interfacePath string) error {
	// Python typically uses unittest.mock or pytest-mock
	// Generate a mock factory or suggest using MagicMock

	dir := filepath.Dir(interfacePath)
	baseFilename := filepath.Base(interfacePath)
	baseFilename = strings.TrimSuffix(baseFilename, ".py")
	mockFilePath := filepath.Join(dir, "mock_"+baseFilename+".py")

	readAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/read_file", interfacePath},
	}
	content, err := t.virtualStore.RouteAction(ctx, readAction)
	if err != nil {
		return fmt.Errorf("failed to read interface file: %w", err)
	}

	classNames := t.extractPythonClasses(content)
	return t.generateMockViaLLM(ctx, interfacePath, mockFilePath, "", classNames)
}

// generateMockViaLLM generates mock code using LLM when mockgen is unavailable.
func (t *TesterShard) generateMockViaLLM(ctx context.Context, interfacePath, mockFilePath, packageName string, interfaceNames []string) error {
	if t.llmClient == nil {
		return fmt.Errorf("LLM client required for mock generation")
	}

	// Read interface file
	readAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/read_file", interfacePath},
	}
	interfaceContent, err := t.virtualStore.RouteAction(ctx, readAction)
	if err != nil {
		return fmt.Errorf("failed to read interface file: %w", err)
	}

	// Build prompt for mock generation
	systemPrompt := t.buildMockGenSystemPrompt(filepath.Ext(interfacePath))
	userPrompt := t.buildMockGenUserPrompt(interfaceContent, packageName, interfaceNames)

	// Call LLM
	response, err := t.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("LLM mock generation failed: %w", err)
	}

	// Extract code from response
	mockCode := t.extractCodeFromResponse(response)

	// Write mock file
	writeAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/write_file", mockFilePath, mockCode},
	}
	_, err = t.virtualStore.RouteAction(ctx, writeAction)
	if err != nil {
		return fmt.Errorf("failed to write mock file: %w", err)
	}

	fmt.Printf("[TesterShard] Generated mock via LLM: %s\n", mockFilePath)

	// Assert facts
	if t.kernel != nil {
		_ = t.kernel.Assert(core.Fact{
			Predicate: "mock_generated",
			Args:      []interface{}{mockFilePath, interfacePath, time.Now().Unix()},
		})
	}

	return nil
}

// buildMockGenSystemPrompt builds system prompt for mock generation.
func (t *TesterShard) buildMockGenSystemPrompt(fileExt string) string {
	var framework string
	switch fileExt {
	case ".go":
		framework = "Go with gomock patterns"
	case ".ts", ".tsx", ".js", ".jsx":
		framework = "TypeScript/JavaScript with Jest"
	case ".py":
		framework = "Python with unittest.mock"
	default:
		framework = "appropriate testing framework"
	}

	return fmt.Sprintf(`You are an expert in generating mock implementations for testing.

Language/Framework: %s

Guidelines:
- Generate complete, production-ready mock implementations
- Implement all methods from the interface with proper signatures
- Use appropriate mock framework patterns (gomock, Jest, unittest.mock)
- Include helpful mock methods for test assertions (e.g., EXPECT(), times(), verify())
- Add comments explaining mock usage
- Follow language-specific best practices

Return ONLY the mock code, no explanations.`, framework)
}

// buildMockGenUserPrompt builds user prompt for mock generation.
func (t *TesterShard) buildMockGenUserPrompt(interfaceContent, packageName string, interfaceNames []string) string {
	var sb strings.Builder
	sb.WriteString("Generate a mock implementation for the following interface(s):\n\n")
	sb.WriteString("```\n")
	sb.WriteString(interfaceContent)
	sb.WriteString("\n```\n\n")

	if packageName != "" {
		sb.WriteString(fmt.Sprintf("Package: %s\n", packageName))
	}

	if len(interfaceNames) > 0 {
		sb.WriteString(fmt.Sprintf("Interfaces to mock: %s\n", strings.Join(interfaceNames, ", ")))
	}

	sb.WriteString("\nGenerate a complete mock implementation that can be used in unit tests.")

	return sb.String()
}

// extractCodeFromResponse extracts code block from LLM response.
func (t *TesterShard) extractCodeFromResponse(response string) string {
	// Extract code block if present
	if idx := strings.Index(response, "```"); idx != -1 {
		endIdx := strings.LastIndex(response, "```")
		if endIdx > idx {
			code := response[idx+3 : endIdx]
			// Remove language tag if present
			if newlineIdx := strings.Index(code, "\n"); newlineIdx != -1 {
				firstLine := strings.TrimSpace(code[:newlineIdx])
				if !strings.Contains(firstLine, " ") && len(firstLine) < 20 {
					code = code[newlineIdx+1:]
				}
			}
			return strings.TrimSpace(code)
		}
	}
	return strings.TrimSpace(response)
}

// extractPackageName extracts package name from Go file content.
func (t *TesterShard) extractPackageName(content string) string {
	re := regexp.MustCompile(`package\s+(\w+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return "main"
}

// extractInterfaceNames extracts all interface names from Go file content.
func (t *TesterShard) extractInterfaceNames(content string) []string {
	names := make([]string, 0)
	re := regexp.MustCompile(`type\s+(\w+)\s+interface`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			names = append(names, match[1])
		}
	}
	return names
}

// extractTypeScriptInterfaces extracts interface names from TypeScript content.
func (t *TesterShard) extractTypeScriptInterfaces(content string) []string {
	names := make([]string, 0)
	re := regexp.MustCompile(`interface\s+(\w+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			names = append(names, match[1])
		}
	}
	return names
}

// extractPythonClasses extracts class names from Python content.
func (t *TesterShard) extractPythonClasses(content string) []string {
	names := make([]string, 0)
	re := regexp.MustCompile(`class\s+(\w+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			names = append(names, match[1])
		}
	}
	return names
}

// =============================================================================
// TASK HANDLERS
// =============================================================================

// handleRegenerateMocks handles the regenerate_mocks task.
func (t *TesterShard) handleRegenerateMocks(ctx context.Context, task *TesterTask) (string, error) {
	if task.Target == "" && task.File == "" {
		return "", fmt.Errorf("no target file specified for mock regeneration")
	}

	interfacePath := task.Target
	if task.File != "" {
		interfacePath = task.File
	}

	err := t.regenerateMock(ctx, interfacePath)
	if err != nil {
		return "", fmt.Errorf("failed to regenerate mock: %w", err)
	}

	return fmt.Sprintf("Successfully regenerated mock for interface: %s", interfacePath), nil
}

// handleDetectStaleMocks handles the detect_stale_mocks task.
func (t *TesterShard) handleDetectStaleMocks(ctx context.Context, task *TesterTask) (string, error) {
	if task.Target == "" && task.File == "" {
		return "", fmt.Errorf("no test file specified for stale mock detection")
	}

	testFile := task.Target
	if task.File != "" {
		testFile = task.File
	}

	staleMocks, err := t.detectStaleMocks(ctx, testFile)
	if err != nil {
		return "", fmt.Errorf("failed to detect stale mocks: %w", err)
	}

	if len(staleMocks) == 0 {
		return "No stale mocks detected", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d stale mock(s):\n", len(staleMocks)))
	for i, mock := range staleMocks {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, mock))
	}
	sb.WriteString("\nUse 'regenerate_mocks' to update them.")

	return sb.String(), nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// isMockError checks if test output indicates a mock-related error.
func (t *TesterShard) isMockError(output string) bool {
	lowerOutput := strings.ToLower(output)
	mockErrorIndicators := []string{
		"mock", "gomock", "mockgen",
		"unexpected call", "missing call",
		"wrong number of calls", "mock expectations",
		"stub", "spy", "double",
		"interface not implemented",
		"method has a pointer receiver",
		"undefined: mock",
		"cannot use", // Often appears with type mismatches in mocks
	}

	for _, indicator := range mockErrorIndicators {
		if strings.Contains(lowerOutput, indicator) {
			return true
		}
	}

	return false
}

// capitalize capitalizes the first letter of a string.
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
