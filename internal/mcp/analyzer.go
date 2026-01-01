package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// LLMClient interface for LLM completions.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// ToolAnalyzer uses LLM to extract semantic metadata from MCP tools.
type ToolAnalyzer struct {
	llm      LLMClient
	embedder embedding.EmbeddingEngine
}

// NewToolAnalyzer creates a new tool analyzer.
func NewToolAnalyzer(llm LLMClient, embedder embedding.EmbeddingEngine) *ToolAnalyzer {
	return &ToolAnalyzer{
		llm:      llm,
		embedder: embedder,
	}
}

// Analyze extracts semantic metadata from an MCP tool schema.
func (a *ToolAnalyzer) Analyze(ctx context.Context, schema MCPToolSchema) (*ToolAnalysis, error) {
	if a.llm == nil {
		return a.analyzeWithoutLLM(schema)
	}

	// Build prompt
	prompt, err := a.buildAnalysisPrompt(schema)
	if err != nil {
		logging.Get(logging.CategoryTools).Warn("Failed to build analysis prompt: %v", err)
		return a.analyzeWithoutLLM(schema)
	}

	// Call LLM
	response, err := a.llm.Complete(ctx, prompt)
	if err != nil {
		logging.Get(logging.CategoryTools).Warn("LLM analysis failed: %v", err)
		return a.analyzeWithoutLLM(schema)
	}

	// Parse response
	analysis, err := a.parseAnalysisResponse(response, schema)
	if err != nil {
		logging.Get(logging.CategoryTools).Warn("Failed to parse analysis response: %v", err)
		return a.analyzeWithoutLLM(schema)
	}

	// Generate embedding for the tool
	if a.embedder != nil {
		embeddingText := a.buildEmbeddingText(schema, analysis)
		taskType := embedding.SelectTaskType(embedding.ContentTypeDocumentation, false)
		var embeddingVec []float32
		var err error
		if taskAware, ok := a.embedder.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
			embeddingVec, err = taskAware.EmbedWithTask(ctx, embeddingText, taskType)
		} else {
			embeddingVec, err = a.embedder.Embed(ctx, embeddingText)
		}
		if err != nil {
			logging.Get(logging.CategoryTools).Debug("Failed to generate embedding: %v", err)
		} else {
			analysis.Embedding = embeddingVec
		}
	}

	return analysis, nil
}

// analyzeWithoutLLM provides basic analysis without LLM.
func (a *ToolAnalyzer) analyzeWithoutLLM(schema MCPToolSchema) (*ToolAnalysis, error) {
	analysis := &ToolAnalysis{
		ToolID:          schema.Name,
		Categories:      inferCategories(schema),
		Capabilities:    inferCapabilities(schema),
		Domain:          "/general",
		ShardAffinities: defaultShardAffinities(),
		UseCases:        []string{schema.Description},
		Condensed:       truncateDescription(schema.Description, 80),
	}

	// Generate embedding if available
	if a.embedder != nil {
		ctx := context.Background()
		embeddingText := a.buildEmbeddingText(schema, analysis)
		taskType := embedding.SelectTaskType(embedding.ContentTypeDocumentation, false)
		var embeddingVec []float32
		var err error
		if taskAware, ok := a.embedder.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
			embeddingVec, err = taskAware.EmbedWithTask(ctx, embeddingText, taskType)
		} else {
			embeddingVec, err = a.embedder.Embed(ctx, embeddingText)
		}
		if err == nil {
			analysis.Embedding = embeddingVec
		}
	}

	return analysis, nil
}

const analysisPromptTemplate = `Analyze this MCP tool and extract structured metadata for tool selection.

## Tool Information
Name: {{.Name}}
Description: {{.Description}}
Input Schema:
{{.InputSchema}}
{{if .OutputSchema}}
Output Schema:
{{.OutputSchema}}
{{end}}

## Required Output
Extract the following as a JSON object:

{
  "categories": ["category1", "category2"],
  "capabilities": ["/capability1", "/capability2"],
  "domain": "/domain",
  "shard_affinities": {"coder": 0-100, "tester": 0-100, "reviewer": 0-100, "researcher": 0-100},
  "use_cases": ["use case 1", "use case 2"],
  "condensed": "One-line description (max 80 chars)"
}

### Categories (select applicable)
- filesystem: File operations (read, write, list, search)
- code_analysis: Code inspection, AST operations, semantic analysis
- code_generation: Creating or modifying code
- shell: Command execution, process management
- git: Version control operations
- web: HTTP requests, web scraping, browser automation
- database: Database queries, data manipulation
- api: External API calls
- testing: Test execution, coverage analysis
- documentation: Doc generation, README operations
- search: Content search, semantic search

### Capabilities (select applicable)
- /read: Reads data without modification
- /write: Creates or modifies data
- /delete: Removes data
- /search: Finds or queries data
- /transform: Converts data formats
- /execute: Runs code or commands
- /analyze: Examines and reports on data
- /validate: Checks correctness

### Domains
- /go, /python, /typescript, /rust, /java (language-specific)
- /general (language-agnostic)

### Shard Affinities (0-100 scores)
- coder: How useful for code generation/modification?
- tester: How useful for test execution/analysis?
- reviewer: How useful for code review/inspection?
- researcher: How useful for information gathering?

Respond with ONLY the JSON object, no other text.`

// buildAnalysisPrompt builds the LLM prompt for tool analysis.
func (a *ToolAnalyzer) buildAnalysisPrompt(schema MCPToolSchema) (string, error) {
	tmpl, err := template.New("analysis").Parse(analysisPromptTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		Name         string
		Description  string
		InputSchema  string
		OutputSchema string
	}{
		Name:         schema.Name,
		Description:  schema.Description,
		InputSchema:  formatJSON(schema.InputSchema),
		OutputSchema: formatJSON(schema.OutputSchema),
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", err
	}

	return sb.String(), nil
}

// parseAnalysisResponse parses the LLM response into ToolAnalysis.
func (a *ToolAnalyzer) parseAnalysisResponse(response string, schema MCPToolSchema) (*ToolAnalysis, error) {
	// Extract JSON from response (handle markdown code blocks)
	jsonStr := extractJSON(response)

	var result struct {
		Categories      []string       `json:"categories"`
		Capabilities    []string       `json:"capabilities"`
		Domain          string         `json:"domain"`
		ShardAffinities map[string]int `json:"shard_affinities"`
		UseCases        []string       `json:"use_cases"`
		Condensed       string         `json:"condensed"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate and normalize
	analysis := &ToolAnalysis{
		ToolID:       schema.Name,
		Categories:   normalizeCategories(result.Categories),
		Capabilities: normalizeCapabilities(result.Capabilities),
		Domain:       normalizeDomain(result.Domain),
		ShardAffinities: normalizeAffinities(result.ShardAffinities),
		UseCases:     result.UseCases,
		Condensed:    truncateDescription(result.Condensed, 80),
	}

	// Fallback for empty condensed
	if analysis.Condensed == "" {
		analysis.Condensed = truncateDescription(schema.Description, 80)
	}

	return analysis, nil
}

// buildEmbeddingText builds text for embedding generation.
func (a *ToolAnalyzer) buildEmbeddingText(schema MCPToolSchema, analysis *ToolAnalysis) string {
	var parts []string

	parts = append(parts, "Tool: "+schema.Name)
	if schema.Description != "" {
		parts = append(parts, "Description: "+schema.Description)
	}
	if len(analysis.Categories) > 0 {
		parts = append(parts, "Categories: "+strings.Join(analysis.Categories, ", "))
	}
	if len(analysis.Capabilities) > 0 {
		parts = append(parts, "Capabilities: "+strings.Join(analysis.Capabilities, ", "))
	}
	if len(analysis.UseCases) > 0 {
		parts = append(parts, "Use cases: "+strings.Join(analysis.UseCases, "; "))
	}

	return strings.Join(parts, "\n")
}

// Helper functions

func formatJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	formatted, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(formatted)
}

func extractJSON(response string) string {
	// Try to find JSON in markdown code block
	if idx := strings.Index(response, "```json"); idx != -1 {
		start := idx + 7
		if endIdx := strings.Index(response[start:], "```"); endIdx != -1 {
			return strings.TrimSpace(response[start : start+endIdx])
		}
	}
	if idx := strings.Index(response, "```"); idx != -1 {
		start := idx + 3
		// Skip optional language identifier
		if nlIdx := strings.Index(response[start:], "\n"); nlIdx != -1 {
			start += nlIdx + 1
		}
		if endIdx := strings.Index(response[start:], "```"); endIdx != -1 {
			return strings.TrimSpace(response[start : start+endIdx])
		}
	}

	// Try to find raw JSON object
	if start := strings.Index(response, "{"); start != -1 {
		depth := 0
		for i := start; i < len(response); i++ {
			switch response[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return response[start : i+1]
				}
			}
		}
	}

	return response
}

func inferCategories(schema MCPToolSchema) []string {
	name := strings.ToLower(schema.Name)
	desc := strings.ToLower(schema.Description)
	combined := name + " " + desc

	var categories []string

	if containsAny(combined, "file", "read", "write", "path", "directory") {
		categories = append(categories, "filesystem")
	}
	if containsAny(combined, "search", "find", "query", "grep") {
		categories = append(categories, "search")
	}
	if containsAny(combined, "git", "commit", "branch", "diff") {
		categories = append(categories, "git")
	}
	if containsAny(combined, "http", "url", "web", "fetch", "request") {
		categories = append(categories, "web")
	}
	if containsAny(combined, "exec", "run", "shell", "command", "process") {
		categories = append(categories, "shell")
	}
	if containsAny(combined, "code", "parse", "ast", "syntax", "analyze") {
		categories = append(categories, "code_analysis")
	}
	if containsAny(combined, "test", "assert", "coverage") {
		categories = append(categories, "testing")
	}
	if containsAny(combined, "database", "sql", "query", "db") {
		categories = append(categories, "database")
	}

	if len(categories) == 0 {
		categories = []string{"general"}
	}

	return categories
}

func inferCapabilities(schema MCPToolSchema) []string {
	name := strings.ToLower(schema.Name)
	desc := strings.ToLower(schema.Description)
	combined := name + " " + desc

	var caps []string

	if containsAny(combined, "read", "get", "list", "show", "view") {
		caps = append(caps, "/read")
	}
	if containsAny(combined, "write", "create", "set", "put", "save", "update") {
		caps = append(caps, "/write")
	}
	if containsAny(combined, "delete", "remove", "clear") {
		caps = append(caps, "/delete")
	}
	if containsAny(combined, "search", "find", "query", "lookup") {
		caps = append(caps, "/search")
	}
	if containsAny(combined, "exec", "run", "execute", "invoke") {
		caps = append(caps, "/execute")
	}
	if containsAny(combined, "analyze", "inspect", "check", "review") {
		caps = append(caps, "/analyze")
	}
	if containsAny(combined, "transform", "convert", "format", "parse") {
		caps = append(caps, "/transform")
	}
	if containsAny(combined, "validate", "verify", "check") {
		caps = append(caps, "/validate")
	}

	if len(caps) == 0 {
		caps = []string{"/read"} // Default assumption
	}

	return caps
}

func defaultShardAffinities() map[string]int {
	return map[string]int{
		"coder":      50,
		"tester":     30,
		"reviewer":   40,
		"researcher": 40,
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func normalizeCategories(cats []string) []string {
	validCategories := map[string]bool{
		"filesystem":      true,
		"code_analysis":   true,
		"code_generation": true,
		"shell":           true,
		"git":             true,
		"web":             true,
		"database":        true,
		"api":             true,
		"testing":         true,
		"documentation":   true,
		"search":          true,
		"general":         true,
	}

	var result []string
	for _, cat := range cats {
		cat = strings.ToLower(strings.TrimSpace(cat))
		if validCategories[cat] {
			result = append(result, cat)
		}
	}

	if len(result) == 0 {
		result = []string{"general"}
	}

	return result
}

func normalizeCapabilities(caps []string) []string {
	validCaps := map[string]bool{
		"/read":      true,
		"/write":     true,
		"/delete":    true,
		"/search":    true,
		"/transform": true,
		"/execute":   true,
		"/analyze":   true,
		"/validate":  true,
	}

	var result []string
	for _, cap := range caps {
		cap = strings.ToLower(strings.TrimSpace(cap))
		if !strings.HasPrefix(cap, "/") {
			cap = "/" + cap
		}
		if validCaps[cap] {
			result = append(result, cap)
		}
	}

	if len(result) == 0 {
		result = []string{"/read"}
	}

	return result
}

func normalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !strings.HasPrefix(domain, "/") {
		domain = "/" + domain
	}

	validDomains := map[string]bool{
		"/go":         true,
		"/python":     true,
		"/typescript": true,
		"/javascript": true,
		"/rust":       true,
		"/java":       true,
		"/general":    true,
	}

	if validDomains[domain] {
		return domain
	}
	return "/general"
}

func normalizeAffinities(affinities map[string]int) map[string]int {
	result := defaultShardAffinities()

	for key, val := range affinities {
		key = strings.ToLower(strings.TrimSpace(key))
		// Clamp to 0-100
		if val < 0 {
			val = 0
		}
		if val > 100 {
			val = 100
		}
		if _, ok := result[key]; ok {
			result[key] = val
		}
	}

	return result
}

func truncateDescription(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Ensure ToolAnalyzer implements ToolAnalyzerInterface.
var _ ToolAnalyzerInterface = (*ToolAnalyzer)(nil)
