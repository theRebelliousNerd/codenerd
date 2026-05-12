---

remediated: false
subsystem: mcp
---
# QA NEGATIVE TESTING GAPS: MCP Tool Analyzer Subsystem
**Date**: 2026-03-27_04-22-EST
**Component**: `internal/mcp/analyzer.go`

## Subsystem Overview
The MCP Tool Analyzer is responsible for extracting semantic metadata from Model Context Protocol (MCP) tool schemas. It primarily utilizes an LLM to generate analysis details like capabilities, categories, domains, and shard affinities based on the tool's description and JSON schemas. This derived intelligence is crucial for context routing, dynamic JIT tool assignments, and efficient semantic searches over available tools.

---

## 1. Null/Undefined/Empty Input Vectors

### 1.1 Empty/Nil LLM Responses
In `Analyze`, the system relies on the LLM to complete a prompt with tool analysis metadata.
- **Vulnerability**: If the LLM returns an empty string `""` or a nil response (if the client allows it), `parseAnalysisResponse` passes this empty string to `extractJSON`. `extractJSON` falls through its markdown and brace-parsing logic, returning the empty string. The subsequent `json.Unmarshal([]byte(""), &result)` will immediately return a `json.SyntaxError: unexpected end of JSON input`.
- **Mitigation**: The code correctly catches the unmarshal error and falls back to `a.analyzeWithoutLLM(schema)`. However, there are no tests ensuring this fallback behaves correctly when presented with empty strings.
- **Performance**: Very performant. The error is caught at the `json.Unmarshal` step and falls back to deterministic rule-based analysis, avoiding retries or panics.
- **Test Plan**:
  ```go
  func TestAnalyze_EmptyLLMResponse(t *testing.T) {
      // 1. Mock LLMClient to return ("", nil)
      mockLLM := &MockLLMClient{Response: ""}
      analyzer := NewToolAnalyzer(mockLLM, nil)

      // 2. Pass a basic schema
      schema := MCPToolSchema{Name: "test_tool", Description: "test desc"}

      // 3. Ensure no panic and fallback is triggered
      result, err := analyzer.Analyze(context.Background(), schema)
      if err != nil {
          t.Fatalf("expected nil error, got %v", err)
      }

      // 4. Verify fallback state
      if result.Capabilities[0] != "/read" {
          t.Errorf("expected fallback capability /read, got %v", result.Capabilities)
      }
  }
  ```
  ```go
  func TestAnalyze_NilLLMResponse(t *testing.T) {
      // Simulate LLM client panicking or returning weird nils
      // Ensure analyzer handles interface nil gracefully
      analyzer := NewToolAnalyzer(nil, nil)

      schema := MCPToolSchema{Name: "test_tool"}
      result, _ := analyzer.Analyze(context.Background(), schema)
      if result == nil {
          t.Fatal("expected fallback result, got nil")
      }
  }
  ```

### 1.2 Nil/Empty Schemas
The `MCPToolSchema` passed into `Analyze` can contain nil or empty `json.RawMessage` for `InputSchema` or `OutputSchema`.
- **Vulnerability**: `formatJSON(schema.InputSchema)` uses `json.Indent`. If `InputSchema` is nil or an empty array, `json.Indent` might fail or return an empty string. The templating engine in `buildAnalysisPrompt` will insert this empty string. While this shouldn't crash, the LLM might hallucinate schemas or error out because the prompt implies schemas should be present.
- **Performance**: High. An empty schema means a very small prompt payload, fast LLM turnaround, and deterministic processing. However, the quality of the inference may degrade.
- **Test Plan**:
  ```go
  func TestAnalyze_NilSchemas(t *testing.T) {
      // 1. Mock LLM to assert prompt contains empty schema placeholders safely
      mockLLM := &MockLLMClient{Response: "{}"}
      analyzer := NewToolAnalyzer(mockLLM, nil)

      // 2. Pass schema with nil Input/Output
      schema := MCPToolSchema{
          Name: "no_schema_tool",
          InputSchema: nil,
          OutputSchema: nil,
      }

      // 3. Ensure buildAnalysisPrompt handles it
      _, err := analyzer.Analyze(context.Background(), schema)
      if err != nil {
          t.Fatalf("Analyze failed with nil schemas: %v", err)
      }
  }
  ```
  ```go
  func TestBuildAnalysisPrompt_EmptyArrays(t *testing.T) {
      analyzer := NewToolAnalyzer(nil, nil)
      schema := MCPToolSchema{
          Name: "empty_arr_tool",
          InputSchema: json.RawMessage("[]"),
          OutputSchema: json.RawMessage("{}"),
      }
      prompt, err := analyzer.buildAnalysisPrompt(schema)
      if err != nil {
          t.Fatal(err)
      }
      if !strings.Contains(prompt, "[]") || !strings.Contains(prompt, "{}") {
          t.Error("prompt should preserve empty structures")
      }
  }
  ```

### 1.3 Empty Name/Description
- **Vulnerability**: If `schema.Name` is empty, `ToolAnalysis.ToolID` becomes empty. If `schema.Description` is also empty, the fallback for `analysis.Condensed` in `parseAnalysisResponse` uses `truncateDescription("", 80)`, which is safe. But an empty ToolID could cause issues downstream when this analysis is stored in the database.
- **Performance**: Excellent. Short strings process quickly. Database will accept an empty string as a primary key, but it leads to logical bugs.
- **Test Plan**:
  ```go
  func TestAnalyze_EmptyIdentity(t *testing.T) {
      analyzer := NewToolAnalyzer(nil, nil) // Force analyzeWithoutLLM
      schema := MCPToolSchema{Name: "", Description: ""}

      result, err := analyzer.Analyze(context.Background(), schema)
      if err != nil {
          t.Fatalf("Analyze failed: %v", err)
      }
      if result.ToolID != "" {
          t.Errorf("expected empty ToolID, got %s", result.ToolID)
      }
  }
  ```
  ```go
  func TestAnalyze_WhitespaceIdentity(t *testing.T) {
      analyzer := NewToolAnalyzer(nil, nil)
      schema := MCPToolSchema{Name: "   \n\t   ", Description: "\t\t"}

      result, err := analyzer.Analyze(context.Background(), schema)
      // The system should probably trim identities and reject empty ones before db insert
      if result.ToolID != "   \n\t   " { // Currently it preserves it
          t.Error("expected preserved whitespace identity")
      }
  }
  ```

---

## 2. Type Coercion & Malformed Data

### 2.1 Malformed JSON in Markdown
`extractJSON` attempts to pull JSON out of markdown blocks.
- **Vulnerability**: If the LLM produces a markdown block ````json` but the content inside is incomplete (e.g., truncated due to max tokens: `{"categories": ["file`), `extractJSON` will return the truncated string. `json.Unmarshal` will fail.
- **Vulnerability**: If the LLM produces a raw JSON object but it contains unescaped control characters or malformed types (e.g., `ShardAffinities` as `{"coder": "high"}` instead of `{"coder": 50}`), `json.Unmarshal` will fail with a type coercion error.
- **Mitigation**: The system safely falls back to `analyzeWithoutLLM`.
- **Performance**: Fast fail. Go's JSON parser is extremely efficient at finding structural and type errors.
- **Test Plan**:
  ```go
  func TestExtractJSON_Malformed(t *testing.T) {
      cases := []struct{
          name string
          input string
      }{
          {"Truncated Markdown", "```json\n{\"categories\": [\"test\"\n```"},
          {"Type Coercion", "```json\n{\"shard_affinities\": {\"coder\": \"high\"}}\n```"},
          {"Unbalanced Braces", "{\"capabilities\": [\"/read\"}"},
          {"Markdown Prefix Only", "```json\n"},
          {"Markdown Suffix Only", "```\n{\"categories\": []}\n```"},
          {"Invalid Escape Sequence", "```json\n{\"desc\": \"bad \\escape\"}\n```"},
          {"Double Nested JSON", "```json\n{\"inner\": \"```json\n{}\n```\"}\n```"},
      }

      for _, tc := range cases {
          t.Run(tc.name, func(t *testing.T) {
              analyzer := NewToolAnalyzer(&MockLLMClient{Response: tc.input}, nil)
              schema := MCPToolSchema{Name: "test"}

              // Should fall back gracefully without panic
              result, err := analyzer.Analyze(context.Background(), schema)
              if err != nil {
                  t.Errorf("unexpected error: %v", err)
              }
              if len(result.Capabilities) == 0 {
                  t.Error("expected fallback capabilities, got empty")
              }
          })
      }
  }
  ```

### 2.2 Invalid Capability/Category Normalization
- **Vulnerability**: `normalizeCapabilities` forces strings to lowercase and prepends a `/`. If the LLM returns `["/read/", "///write"]`, the normalizer might not handle it correctly (it uses `strings.TrimSpace`, but not `strings.Trim(..., "/")`). `///write` would be prepended again resulting in `////write` or similar, failing the `validCaps` lookup and being dropped.
- **Performance**: O(N) over a tiny array. Negligible impact.
- **Test Plan**:
  ```go
  func TestNormalizeCapabilities_EdgeCases(t *testing.T) {
      input := []string{" / r e a d ", "///write", "", "/read/", "\n\t/execute\n", "\\\\search", "r e a d"}
      expected := []string{"/read", "/execute"} // Assuming ///write fails and is dropped

      result := normalizeCapabilities(input)

      // Assert result matches expected lengths and values
      if len(result) != len(expected) {
          t.Errorf("expected %d caps, got %d", len(expected), len(result))
      }
  }
  ```
  ```go
  func TestNormalizeCategories_EdgeCases(t *testing.T) {
      input := []string{" file system ", "FILE_SYSTEM", "code analysis", "UNKNOWN", "12345", "<script>alert(1)</script>"}
      expected := []string{"filesystem", "code_analysis"} // assuming mapping works, unknown/XSS dropped

      result := normalizeCategories(input)
      if len(result) != len(expected) {
          t.Errorf("expected %d categories, got %d", len(expected), len(result))
      }
  }
  ```
  ```go
  func TestNormalizeAffinities_Extremes(t *testing.T) {
      input := map[string]int{
          "coder": 150, // Should cap at 100
          "tester": -10, // Should floor at 0
          "reviewer": 0,
          "unknown": 50, // Should be dropped
      }
      result := normalizeAffinities(input)

      if result["coder"] != 100 { t.Errorf("expected 100, got %d", result["coder"]) }
      if result["tester"] != 0 { t.Errorf("expected 0, got %d", result["tester"]) }
      if _, ok := result["unknown"]; ok { t.Error("unknown shard affinity was not dropped") }
  }
  ```

---

## 3. User Request Extremes

### 3.1 Massive Schemas (Token Exhaustion)
- **Vulnerability**: If an MCP server registers a tool with a massive input schema (e.g., a massive OpenAPI spec represented as JSON schema), `formatJSON` will attempt to format the entire thing. The resulting string could be megabytes in size.
- **Performance Impact**: Passing a massive formatted schema into `buildAnalysisPrompt` will blow past the LLM's token context limit. The LLM provider will reject the request. The analyzer will fail and fall back to `analyzeWithoutLLM`. This wastes latency, network bandwidth, and memory doing a massive `json.Indent`.
- **System Tolerance**: The system is performant enough to handle the JSON parsing, but the network call and token budget will suffer severely.
- **Test Plan**:
  ```go
  func TestBuildAnalysisPrompt_MassiveSchema(t *testing.T) {
      // 1. Generate a 5MB JSON string representing a deep nested schema
      massiveJSON := generateMassiveJSON(5 * 1024 * 1024)

      schema := MCPToolSchema{
          Name: "god_tool",
          InputSchema: massiveJSON,
      }

      // 2. Ensure buildAnalysisPrompt doesn't OOM and completes quickly
      analyzer := NewToolAnalyzer(nil, nil)
      prompt, err := analyzer.buildAnalysisPrompt(schema)

      // 3. Ideally, assert that the prompt is truncated to a safe token limit
      // (Currently it is NOT, which is a bug this test will expose)
      if len(prompt) > 100000 {
          t.Logf("Warning: Prompt is too large (%d bytes), risk of token exhaustion", len(prompt))
      }
  }
  ```
  ```go
  func TestAnalyze_MassiveLLMResponse(t *testing.T) {
      // 1. Mock LLM returning 5MB of prose to test JSON extractor limits
      massiveResponse := strings.Repeat("This tool does file operations. ", 200000)
      analyzer := NewToolAnalyzer(&MockLLMClient{Response: massiveResponse}, nil)

      schema := MCPToolSchema{Name: "spam_tool"}
      start := time.Now()

      _, err := analyzer.Analyze(context.Background(), schema)

      // 2. Performance check: JSON extraction shouldn't hang on massive non-JSON blocks
      if time.Since(start) > 200 * time.Millisecond {
          t.Errorf("Analyze took too long on massive response: %v", time.Since(start))
      }
  }
  ```

### 3.2 Extreme Descriptions (Bounds Checking)
- **Vulnerability**: `truncateDescription` has an edge case. `if maxLen <= 3 { return s[:maxLen] }` followed by `return s[:maxLen-3] + "..."`. If a user passes `maxLen = 2` and `s = "a"`, `s[:maxLen]` will panic with a bounds out of range error.
- **Code context**:
  ```go
  func truncateDescription(s string, maxLen int) string {
      s = strings.TrimSpace(s)
      if len(s) <= maxLen {
          return s
      }
      if maxLen <= 3 {
          return s[:maxLen] // Safe because len(s) > maxLen due to prior check
      }
      return s[:maxLen-3] + "..."
  }
  ```
- **Performance**: O(1) string slicing. Extremely fast.
- **Test Plan**:
  ```go
  func TestTruncateDescription_Extremes(t *testing.T) {
      cases := []struct{
          input string
          maxLen int
          expected string
      }{
          {"a", 2, "a"},
          {"abcdef", 2, "ab"}, // hits maxLen <= 3 branch
          {"abcdef", 5, "ab..."},
          {"", 10, ""},
          {"", -1, ""}, // Panic check
          {"ab", 1, "a"}, // Panic check
          {"ab", 0, ""},  // Empty truncation check
          {"   \n\t   ", 5, ""}, // Whitespace truncation check
      }

      for _, tc := range cases {
          if got := truncateDescription(tc.input, tc.maxLen); got != tc.expected {
              t.Errorf("truncateDescription(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.expected)
          }
      }
  }
  ```

### 3.3 Exhaustive String Search in extractJSON
- **Vulnerability**: `extractJSON` scans character-by-character if it doesn't find markdown tags:
  ```go
  if start := strings.Index(response, "{"); start != -1 {
      depth := 0
      for i := start; i < len(response); i++ {
          switch response[i] {
          case '{': depth++
          case '}': depth--
          // ...
  ```
  If an LLM returns a massive 10MB string of prose that happens to have a single `{` at the beginning and never a matching `}`, the loop scans the entire 10MB.
- **Performance Impact**: O(N) scan. Performant enough for Go, but wastes CPU cycles on bad data.
- **Test Plan**:
  ```go
  func TestExtractJSON_ExhaustiveSearch(t *testing.T) {
      // 1. Create a 10MB string starting with '{' but no '}'
      massiveProse := "{" + strings.Repeat("A very long hallucinated response ", 500000)

      start := time.Now()
      result := extractJSON(massiveProse)
      duration := time.Since(start)

      // 2. Verify it completes reasonably fast (e.g., < 50ms)
      if duration > 50*time.Millisecond {
          t.Errorf("extractJSON took too long: %v", duration)
      }

      // 3. Verify it returns the original string (fallback behavior)
      if result != massiveProse {
          t.Error("expected fallback to original string")
      }
  }
  ```

---

## 4. State Conflicts & Race Conditions

### 4.1 Concurrent Analysis (Thread Safety)
The `ToolAnalyzer` is stateless (it only holds `llm` and `embedder` interfaces).
- **Vulnerability**: If multiple goroutines call `Analyze` simultaneously for different tools, there is no shared state mutation within the analyzer itself. However, the underlying `LLMClient` and `EmbeddingEngine` must be thread-safe.
- **Performance**: High concurrency is possible, limited only by the LLM client's connection pooling and rate limits.
- **Test Plan**:
  ```go
  func TestAnalyze_Concurrency(t *testing.T) {
      analyzer := NewToolAnalyzer(&MockLLMClient{Response: "{}"}, nil)
      var wg sync.WaitGroup

      for i := 0; i < 100; i++ {
          wg.Add(1)
          go func(id int) {
              defer wg.Done()
              schema := MCPToolSchema{Name: fmt.Sprintf("tool_%d", id)}
              _, err := analyzer.Analyze(context.Background(), schema)
              if err != nil {
                  t.Errorf("Concurrent Analyze failed: %v", err)
              }
          }(i)
      }
      wg.Wait()
  }
  ```

### 4.2 Context Cancellation (Masked Errors)
- **Vulnerability**: `Analyze` takes a `context.Context` and passes it to `llm.Complete(ctx, prompt)`. If the context is cancelled, `llm.Complete` should return an error.
- **Mitigation**: The code handles the error and falls back to `analyzeWithoutLLM(schema)`. This is highly problematic. If a user cancels the operation (e.g., closing the CLI session, or a timeout occurs), the system shouldn't fall back to a dumb analysis and store it; it should abort the analysis entirely.
- **Code context**:
  ```go
  response, err := a.llm.Complete(ctx, prompt)
  if err != nil {
      logging.Get(logging.CategoryTools).Warn("LLM analysis failed: %v", err)
      return a.analyzeWithoutLLM(schema) // <-- Ignores context.Canceled!
  }
  ```
- **Performance/State Impact**: The system does unnecessary work (`analyzeWithoutLLM`) and stores an incomplete analysis in the database, requiring future garbage collection or manual intervention.
- **Test Plan**:
  ```go
  func TestAnalyze_ContextCancellation(t *testing.T) {
      // 1. Mock LLM that respects context
      mockLLM := &ContextAwareMockLLM{}
      analyzer := NewToolAnalyzer(mockLLM, nil)

      ctx, cancel := context.WithCancel(context.Background())
      cancel() // Cancel immediately

      schema := MCPToolSchema{Name: "test"}

      // 2. Expect an error, NOT a fallback analysis
      result, err := analyzer.Analyze(ctx, schema)

      if err == nil {
          t.Error("expected error due to canceled context, got nil")
      }
      if !errors.Is(err, context.Canceled) {
          t.Errorf("expected context.Canceled error, got: %v", err)
      }
      if result != nil {
          t.Error("expected nil result on cancellation")
      }
  }
  ```
  ```go
  func TestAnalyze_ContextDeadlineExceeded(t *testing.T) {
      // 1. Mock LLM that hangs forever
      mockLLM := &HangingMockLLM{}
      analyzer := NewToolAnalyzer(mockLLM, nil)

      // 2. Set strict timeout
      ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
      defer cancel()

      schema := MCPToolSchema{Name: "slow_tool"}

      result, err := analyzer.Analyze(ctx, schema)
      if err == nil {
          t.Error("expected timeout error")
      }
  }
  ```

---

## Conclusion & Actionable Recommendations

The `ToolAnalyzer` subsystem is generally robust and handles runtime errors by falling back to a rule-based deterministic analysis (`analyzeWithoutLLM`). However, this fallback mechanism acts as a catch-all that obscures underlying systemic issues.

1. **Context Cancellation Propagation**: Update `Analyze` to explicitly check `errors.Is(err, context.Canceled)` or `errors.Is(err, context.DeadlineExceeded)`. If true, return the error rather than executing the fallback. This prevents "zombie" operations and bad database writes when sessions terminate early.
2. **Token Limit Protections**: Implement a `truncateJSON` or schema summarization step before `buildAnalysisPrompt`. Massive OpenAPI specs will consistently fail against the LLM, burning budget and network time just to fall back to the dumb analyzer.
3. **Type Coercion Logging**: When `json.Unmarshal` fails in `parseAnalysisResponse`, the system logs a generic warning. It should log the raw LLM response (at a trace level) to help diagnose prompt drift or LLM hallucination issues.
4. **Enhanced Capability Normalization**: `normalizeCapabilities` should employ regex or more robust string cleaning (`strings.Trim(..., "/ ")`) to handle adversarial or hallucinated string formatting from the LLM.
