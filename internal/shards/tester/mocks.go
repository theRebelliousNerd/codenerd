package tester

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
)

// =============================================================================
// MOCK TYPES
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

// =============================================================================
// MOCK DETECTION
// =============================================================================

// detectStaleMocks identifies outdated mocks in a test file.
func (t *TesterShard) detectStaleMocks(ctx context.Context, testFile string) ([]string, error) {
	if t.virtualStore == nil {
		return nil, fmt.Errorf("virtualStore required for file operations")
	}

	readAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/read_file", testFile},
	}
	content, err := t.virtualStore.RouteAction(ctx, readAction)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	staleMocks := make([]string, 0)
	mockImports := t.extractMockImports(content, testFile)

	for _, mockInfo := range mockImports {
		isStale, err := t.isMockStale(ctx, mockInfo)
		if err != nil {
			fmt.Printf("[TesterShard] Warning: failed to check staleness for %s: %v\n", mockInfo.MockFile, err)
			continue
		}
		if isStale {
			staleMocks = append(staleMocks, mockInfo.InterfaceFile)
		}
	}

	return staleMocks, nil
}

// extractMockImports extracts mock file references from test file content.
func (t *TesterShard) extractMockImports(content, testFile string) []MockInfo {
	mocks := make([]MockInfo, 0)
	dir := filepath.Dir(testFile)

	// Pattern 1: Go files with mock_ prefix
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
			importPath := match[1]
			mocks = append(mocks, MockInfo{
				MockFile:      importPath,
				InterfaceFile: strings.TrimSuffix(importPath, "/mocks"),
			})
		}
	}

	// Pattern 3: gomock usage
	if strings.Contains(content, "gomock.NewController") || strings.Contains(content, "gomock.Controller") {
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

	statAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/stat_file", mockInfo.MockFile},
	}
	_, err := t.virtualStore.RouteAction(ctx, statAction)
	if err != nil {
		return true, nil // Mock doesn't exist
	}

	if mockInfo.InterfaceFile != "" {
		readAction := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", mockInfo.InterfaceFile},
		}
		interfaceContent, err := t.virtualStore.RouteAction(ctx, readAction)
		if err != nil {
			return false, fmt.Errorf("failed to read interface file: %w", err)
		}

		readMockAction := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", mockInfo.MockFile},
		}
		mockContent, err := t.virtualStore.RouteAction(ctx, readMockAction)
		if err != nil {
			return false, fmt.Errorf("failed to read mock file: %w", err)
		}

		interfaceMethods := t.extractInterfaceMethods(interfaceContent, mockInfo.InterfaceName)
		mockMethods := t.extractMockMethods(mockContent)

		for _, method := range interfaceMethods {
			if !contains(mockMethods, method) {
				return true, nil
			}
		}
	}

	return false, nil
}

// isMockError checks if test output indicates a mock-related error.
func (t *TesterShard) isMockError(output string) bool {
	lowerOutput := strings.ToLower(output)
	mockErrorIndicators := []string{
		"mock", "gomock", "mockgen",
		"unexpected call", "missing call",
		"wrong number of calls", "mock expectations",
		"stub", "spy", "double",
		"interface not implemented",
		"undefined: mock",
	}

	for _, indicator := range mockErrorIndicators {
		if strings.Contains(lowerOutput, indicator) {
			return true
		}
	}
	return false
}

// =============================================================================
// MOCK REGENERATION
// =============================================================================

// regenerateMock regenerates a mock file for the given interface.
func (t *TesterShard) regenerateMock(ctx context.Context, interfacePath string) error {
	if t.virtualStore == nil {
		return fmt.Errorf("virtualStore required for mock regeneration")
	}

	framework := t.testerConfig.Framework
	if framework == "auto" {
		framework = t.detectFramework(interfacePath)
	}

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
	readAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/read_file", interfacePath},
	}
	content, err := t.virtualStore.RouteAction(ctx, readAction)
	if err != nil {
		return fmt.Errorf("failed to read interface file: %w", err)
	}

	packageName := t.extractPackageName(content)
	interfaceNames := t.extractInterfaceNames(content)

	if len(interfaceNames) == 0 {
		return fmt.Errorf("no interfaces found in %s", interfacePath)
	}

	dir := filepath.Dir(interfacePath)
	baseFilename := strings.TrimSuffix(filepath.Base(interfacePath), filepath.Ext(interfacePath))
	mockFilePath := filepath.Join(dir, "mock_"+baseFilename+".go")

	// Try mockgen first
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
		return t.generateMockViaLLM(ctx, interfacePath, mockFilePath, packageName, interfaceNames)
	}

	mockgenCmd := fmt.Sprintf("mockgen -source=%s -destination=%s -package=%s",
		interfacePath, mockFilePath, packageName)

	execAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/run_command", mockgenCmd},
	}
	_, err = t.virtualStore.RouteAction(ctx, execAction)
	if err != nil {
		return t.generateMockViaLLM(ctx, interfacePath, mockFilePath, packageName, interfaceNames)
	}

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
	dir := filepath.Dir(interfacePath)
	mockDir := filepath.Join(dir, "__mocks__")

	mkdirAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/create_dir", mockDir},
	}
	_, _ = t.virtualStore.RouteAction(ctx, mkdirAction)

	mockFilePath := filepath.Join(mockDir, filepath.Base(interfacePath))

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
	dir := filepath.Dir(interfacePath)
	baseFilename := strings.TrimSuffix(filepath.Base(interfacePath), ".py")
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

// generateMockViaLLM generates mock code using LLM.
func (t *TesterShard) generateMockViaLLM(ctx context.Context, interfacePath, mockFilePath, packageName string, interfaceNames []string) error {
	if t.llmClient == nil {
		return fmt.Errorf("LLM client required for mock generation")
	}

	readAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/read_file", interfacePath},
	}
	interfaceContent, err := t.virtualStore.RouteAction(ctx, readAction)
	if err != nil {
		return fmt.Errorf("failed to read interface file: %w", err)
	}

	systemPrompt := t.buildMockGenSystemPrompt(filepath.Ext(interfacePath))
	userPrompt := t.buildMockGenUserPrompt(interfaceContent, packageName, interfaceNames)

	rawResponse, err := t.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("LLM mock generation failed: %w", err)
	}

	// Process through Piggyback Protocol - extract surface, route control to kernel
	processed := articulation.ProcessLLMResponse(rawResponse)
	if processed.Control != nil {
		t.routeControlPacketToKernel(processed.Control)
	}

	mockCode := t.extractCodeFromResponse(processed.Surface)

	writeAction := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"/write_file", mockFilePath, mockCode},
	}
	_, err = t.virtualStore.RouteAction(ctx, writeAction)
	if err != nil {
		return fmt.Errorf("failed to write mock file: %w", err)
	}

	if t.kernel != nil {
		_ = t.kernel.Assert(core.Fact{
			Predicate: "mock_generated",
			Args:      []interface{}{mockFilePath, interfacePath, time.Now().Unix()},
		})
	}

	return nil
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

// extractInterfaceMethods extracts method signatures from interface definition.
func (t *TesterShard) extractInterfaceMethods(content, interfaceName string) []string {
	methods := make([]string, 0)

	if interfaceName == "" {
		interfacePattern := regexp.MustCompile(`type\s+(\w+)\s+interface\s*\{([^}]+)\}`)
		matches := interfacePattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				methods = append(methods, t.parseInterfaceMethods(match[2])...)
			}
		}
		return methods
	}

	interfacePattern := regexp.MustCompile(`type\s+` + interfaceName + `\s+interface\s*\{([^}]+)\}`)
	matches := interfacePattern.FindStringSubmatch(content)
	if len(matches) >= 2 {
		return t.parseInterfaceMethods(matches[1])
	}

	return methods
}

// parseInterfaceMethods parses method names from interface body.
func (t *TesterShard) parseInterfaceMethods(interfaceBody string) []string {
	methods := make([]string, 0)
	methodPattern := regexp.MustCompile(`(\w+)\s*\([^)]*\)`)
	matches := methodPattern.FindAllStringSubmatch(interfaceBody, -1)
	for _, match := range matches {
		if len(match) > 1 {
			methods = append(methods, match[1])
		}
	}
	return methods
}

// extractMockMethods extracts implemented methods from mock file.
func (t *TesterShard) extractMockMethods(content string) []string {
	methods := make([]string, 0)
	methodPattern := regexp.MustCompile(`func\s+\([^)]+\*Mock\w+\)\s+(\w+)\s*\(`)
	matches := methodPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			methods = append(methods, match[1])
		}
	}
	return methods
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
- Include helpful mock methods for test assertions
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
	if idx := strings.Index(response, "```"); idx != -1 {
		endIdx := strings.LastIndex(response, "```")
		if endIdx > idx {
			code := response[idx+3 : endIdx]
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

// capitalize capitalizes the first letter of a string.
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
