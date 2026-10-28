package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// LLMClient is the model the analyzer asks: a system prompt (the analyzer's
// role prompt) and a user message (the tool).
type LLMClient interface {
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// toolAnalyzerRole is the role the analyzer's system prompt compiles under:
// its atoms (atoms/system/mcp_tool_analyzer.yaml) declare shard_types
// ["/mcp_tool_analyzer"]. The judge intent keeps worker intent atoms out of a
// compile whose reply Go parses.
const (
	toolAnalyzerRole   = "mcp_tool_analyzer"
	toolAnalyzerIntent = "judge"
)

// ToolAnalyzer uses LLM to extract semantic metadata from MCP tools.
type ToolAnalyzer struct {
	llm      LLMClient
	prompts  types.RolePromptCompiler
	embedder embedding.EmbeddingEngine
}

// NewToolAnalyzer creates a new tool analyzer. prompts compiles the analyzer's
// system prompt (its instructions and JSON reply contract); with an llm and no
// prompts, Analyze reports types.ErrRolePromptUnavailable.
func NewToolAnalyzer(llm LLMClient, prompts types.RolePromptCompiler, embedder embedding.EmbeddingEngine) *ToolAnalyzer {
	return &ToolAnalyzer{
		llm:      llm,
		prompts:  prompts,
		embedder: embedder,
	}
}

// Analyze extracts semantic metadata from an MCP tool schema. With no model it
// infers the metadata from the tool's name and description. With a model, the
// analyzer's role prompt must compile: a missing or failed compile is an error
// the caller reports, never a different prompt.
func (a *ToolAnalyzer) Analyze(ctx context.Context, schema MCPToolSchema) (*ToolAnalysis, error) {
	if a.llm == nil {
		return a.analyzeWithoutLLM(schema)
	}

	system, err := a.systemPrompt(ctx)
	if err != nil {
		return nil, fmt.Errorf("analyze MCP tool %s: %w", schema.Name, err)
	}

	// Call LLM
	response, err := a.llm.CompleteWithSystem(ctx, system, buildAnalysisPrompt(schema))
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
		// Whole, not cut at 80 runes: a severed summary reads as a complete
		// one and the rest is unreachable. The compiler already budgets
		// secondary tools by tier and count; the text itself stays intact.
		Condensed: strings.TrimSpace(schema.Description),
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

// systemPrompt compiles the analyzer's role prompt.
func (a *ToolAnalyzer) systemPrompt(ctx context.Context) (string, error) {
	if a.prompts == nil {
		return "", fmt.Errorf("%w: no role prompt compiler for the %s", types.ErrRolePromptUnavailable, toolAnalyzerRole)
	}
	return a.prompts.CompileRolePrompt(ctx, types.RolePromptRequest{Role: toolAnalyzerRole, Intent: toolAnalyzerIntent})
}

// buildAnalysisPrompt builds the analyzer's user message: the tool's name,
// description and schemas, and nothing else. What to extract from them and
// the JSON reply contract are the role prompt
// (atoms/system/mcp_tool_analyzer.yaml).
func buildAnalysisPrompt(schema MCPToolSchema) string {
	var sb strings.Builder
	sb.WriteString("## Tool Information\nName: ")
	sb.WriteString(schema.Name)
	sb.WriteString("\nDescription: ")
	sb.WriteString(schema.Description)
	sb.WriteString("\nInput Schema:\n")
	sb.WriteString(formatJSON(schema.InputSchema))
	sb.WriteString("\n")
	if len(schema.OutputSchema) > 0 {
		sb.WriteString("Output Schema:\n")
		sb.WriteString(formatJSON(schema.OutputSchema))
		sb.WriteString("\n")
	}
	return sb.String()
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
		ToolID:          schema.Name,
		Categories:      normalizeCategories(result.Categories),
		Capabilities:    normalizeCapabilities(result.Capabilities),
		Domain:          normalizeDomain(result.Domain),
		ShardAffinities: normalizeAffinities(result.ShardAffinities),
		UseCases:        result.UseCases,
		// The LLM's own summary, kept whole: cutting a semantic
		// condensation at 80 runes severs it mid-thought.
		Condensed: strings.TrimSpace(result.Condensed),
	}

	// Fallback for empty condensed
	if analysis.Condensed == "" {
		analysis.Condensed = strings.TrimSpace(schema.Description)
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
	var v any
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

// Ensure ToolAnalyzer implements ToolAnalyzerInterface.
var _ ToolAnalyzerInterface = (*ToolAnalyzer)(nil)
