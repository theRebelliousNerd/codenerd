package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// QUALITY PROFILE WRAPPERS
// =============================================================================
// These methods expose the tool-specific quality profile system.

// GetToolProfile retrieves the quality profile for a tool
func (o *Orchestrator) GetToolProfile(toolName string) *ToolQualityProfile {
	return o.profiles.GetProfile(toolName)
}

// SetToolProfile stores a quality profile for a tool
func (o *Orchestrator) SetToolProfile(profile *ToolQualityProfile) {
	o.profiles.SetProfile(profile)
}

// GetDefaultToolProfile returns a default profile based on tool type
func (o *Orchestrator) GetDefaultToolProfile(toolName string, toolType ToolType) *ToolQualityProfile {
	return GetDefaultProfile(toolName, toolType)
}

// EvaluateWithProfile performs profile-aware quality evaluation
func (o *Orchestrator) EvaluateWithProfile(ctx context.Context, feedback *ExecutionFeedback) *QualityAssessment {
	profile := o.profiles.GetProfile(feedback.ToolName)
	if profile == nil {
		// Fall back to default evaluation
		return o.evaluator.Evaluate(ctx, feedback)
	}
	return o.evaluator.EvaluateWithProfile(ctx, feedback, profile)
}

// ExecuteAndEvaluateWithProfile runs a tool and evaluates using its quality profile
func (o *Orchestrator) ExecuteAndEvaluateWithProfile(ctx context.Context, toolName string, input string) (string, *QualityAssessment, error) {
	start := time.Now()

	output, err := o.ouroboros.ExecuteTool(ctx, toolName, input)

	feedback := &ExecutionFeedback{
		ToolName:   toolName,
		Timestamp:  start,
		Input:      input,
		Output:     output,
		OutputSize: len(output),
		Duration:   time.Since(start),
		Success:    err == nil,
	}

	if err != nil {
		feedback.ErrorMsg = err.Error()
	}

	// Profile-aware evaluation
	profile := o.profiles.GetProfile(toolName)
	if profile != nil {
		feedback.Quality = o.evaluator.EvaluateWithProfile(ctx, feedback, profile)
	} else {
		feedback.Quality = o.evaluator.Evaluate(ctx, feedback)
	}

	// Record for learning
	o.patterns.RecordExecution(*feedback)
	patterns := o.patterns.GetToolPatterns(feedback.ToolName)
	o.learnings.RecordLearning(feedback.ToolName, feedback, patterns)

	return output, feedback.Quality, err
}

// GenerateToolProfile uses LLM to generate a quality profile during tool creation
func (o *Orchestrator) GenerateToolProfile(ctx context.Context, toolName string, description string, toolCode string) (*ToolQualityProfile, error) {
	prompt := fmt.Sprintf(`Generate a quality profile for this tool. The profile defines expectations for how this tool should perform.

Tool Name: %s
Description: %s
Code (abbreviated):
%s

Based on the tool's purpose and implementation, determine:

1. **Tool Type** - One of:
   - quick_calculation: < 1s, simple computation (e.g., calculator, converter)
   - data_fetch: API call, may paginate (e.g., fetch docs, query database)
   - background_task: Long-running, minutes OK (e.g., indexer, importer)
   - recursive_analysis: Codebase traversal, slow OK (e.g., code analyzer)
   - realtime_query: Must be fast, frequent (e.g., status check, health ping)
   - one_time_setup: Run once, can be slow (e.g., initialization, migration)
   - batch_processor: Processes many items (e.g., bulk update, mass import)
   - monitor: Called repeatedly for status (e.g., metrics collector)

2. **Performance Expectations**:
   - expected_duration_min: Faster than this is suspicious (e.g., didn't do work)
   - expected_duration_max: Slower than this is a problem
   - acceptable_duration: Target duration for good performance
   - timeout_duration: When to give up
   - max_retries: How many retries are acceptable

3. **Output Expectations**:
   - expected_min_size: Smaller output is suspicious
   - expected_max_size: Larger might indicate issue
   - expected_typical_size: Normal output size in bytes
   - expected_format: json, text, csv, etc.
   - expects_pagination: Should we paginate?
   - required_fields: Fields that must be in output (for JSON)
   - must_contain: Strings that must appear
   - must_not_contain: Strings that indicate failure

4. **Usage Pattern**:
   - frequency: once, occasional, frequent, constant
   - is_idempotent: Same input = same output?
   - has_side_effects: Modifies external state?
   - depends_on_external: Needs external service?

5. **Caching**:
   - cacheable: Can results be cached?
   - cache_duration: How long to cache (e.g., "15m", "1h")
   - cache_key: What makes cache key unique (e.g., "input_hash")

Return JSON:
{
  "tool_type": "data_fetch",
  "description": "Brief description of what tool does",
  "performance": {
    "expected_duration_min_ms": 100,
    "expected_duration_max_ms": 30000,
    "acceptable_duration_ms": 5000,
    "timeout_duration_ms": 60000,
    "max_retries": 3,
    "expected_api_calls": 1,
    "scales_with_input_size": false
  },
  "output": {
    "expected_min_size": 100,
    "expected_max_size": 1048576,
    "expected_typical_size": 10240,
    "expected_format": "json",
    "expects_pagination": true,
    "expected_pages": 5,
    "required_fields": ["data", "status"],
    "must_contain": [],
    "must_not_contain": ["error", "failed"]
  },
  "usage_pattern": {
    "frequency": "occasional",
    "calls_per_session": 5,
    "is_idempotent": true,
    "has_side_effects": false,
    "depends_on_external": true
  },
  "caching": {
    "cacheable": true,
    "cache_duration": "15m",
    "cache_key": "input_url"
  },
  "custom_dimensions": [
    {
      "name": "items_fetched",
      "description": "Number of items in response",
      "expected_value": 100,
      "tolerance": 50,
      "weight": 0.3,
      "extract_pattern": "\"count\":\\s*(\\d+)"
    }
  ]
}`,
		toolName,
		description,
		truncateCode(toolCode, 2000),
	)

	resp, err := o.client.Complete(ctx, prompt)
	if err != nil {
		// Return default profile on error
		return GetDefaultProfile(toolName, ToolTypeDataFetch), nil
	}

	// Parse response
	profile, parseErr := parseProfileResponse(toolName, resp)
	if parseErr != nil {
		// Return default profile on parse error
		logging.AutopoiesisDebug("Failed to parse profile response: %v", parseErr)
		return GetDefaultProfile(toolName, ToolTypeDataFetch), nil
	}

	// Store the profile
	o.profiles.SetProfile(profile)

	return profile, nil
}

// GenerateToolWithProfile generates a tool and its quality profile together
func (o *Orchestrator) GenerateToolWithProfile(ctx context.Context, need *ToolNeed, userRequest string) (*GeneratedTool, *ToolQualityProfile, *ReasoningTrace, error) {
	// Generate tool with tracing
	tool, trace, err := o.GenerateToolWithTracing(ctx, need, userRequest)
	if err != nil {
		return nil, nil, trace, err
	}

	// Generate quality profile for the tool
	profile, profileErr := o.GenerateToolProfile(ctx, tool.Name, need.Purpose, tool.Code)
	if profileErr != nil {
		// Non-fatal - use default profile
		profile = GetDefaultProfile(tool.Name, ToolTypeDataFetch)
	}

	// Store profile
	o.profiles.SetProfile(profile)

	// Add profile info to trace notes
	trace.PostExecutionNotes = append(trace.PostExecutionNotes,
		fmt.Sprintf("Generated quality profile: type=%s, acceptable_duration=%v",
			profile.ToolType, profile.Performance.AcceptableDuration))

	return tool, profile, trace, nil
}

// parseProfileResponse parses LLM response into a ToolQualityProfile
func parseProfileResponse(toolName string, response string) (*ToolQualityProfile, error) {
	// Extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	// Parse into intermediate struct
	var raw struct {
		ToolType    string `json:"tool_type"`
		Description string `json:"description"`
		Performance struct {
			ExpectedDurationMinMS int64   `json:"expected_duration_min_ms"`
			ExpectedDurationMaxMS int64   `json:"expected_duration_max_ms"`
			AcceptableDurationMS  int64   `json:"acceptable_duration_ms"`
			TimeoutDurationMS     int64   `json:"timeout_duration_ms"`
			MaxRetries            int     `json:"max_retries"`
			ExpectedAPICalls      int     `json:"expected_api_calls"`
			MaxMemoryMB           int64   `json:"max_memory_mb"`
			ScalesWithInputSize   bool    `json:"scales_with_input_size"`
			ScalingFactor         float64 `json:"scaling_factor"`
		} `json:"performance"`
		Output struct {
			ExpectedMinSize     int      `json:"expected_min_size"`
			ExpectedMaxSize     int      `json:"expected_max_size"`
			ExpectedTypicalSize int      `json:"expected_typical_size"`
			ExpectedFormat      string   `json:"expected_format"`
			ExpectsPagination   bool     `json:"expects_pagination"`
			ExpectedPages       int      `json:"expected_pages"`
			RequiredFields      []string `json:"required_fields"`
			MustContain         []string `json:"must_contain"`
			MustNotContain      []string `json:"must_not_contain"`
			CompletenessCheck   string   `json:"completeness_check"`
		} `json:"output"`
		UsagePattern struct {
			Frequency         string `json:"frequency"`
			CallsPerSession   int    `json:"calls_per_session"`
			IsIdempotent      bool   `json:"is_idempotent"`
			HasSideEffects    bool   `json:"has_side_effects"`
			DependsOnExternal bool   `json:"depends_on_external"`
		} `json:"usage_pattern"`
		Caching struct {
			Cacheable     bool     `json:"cacheable"`
			CacheDuration string   `json:"cache_duration"`
			CacheKey      string   `json:"cache_key"`
			InvalidateOn  []string `json:"invalidate_on"`
		} `json:"caching"`
		CustomDimensions []struct {
			Name           string  `json:"name"`
			Description    string  `json:"description"`
			ExpectedValue  float64 `json:"expected_value"`
			Tolerance      float64 `json:"tolerance"`
			Weight         float64 `json:"weight"`
			ExtractPattern string  `json:"extract_pattern"`
		} `json:"custom_dimensions"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse profile JSON: %w", err)
	}

	// Convert to ToolQualityProfile
	profile := &ToolQualityProfile{
		ToolName:    toolName,
		ToolType:    ToolType(raw.ToolType),
		Description: raw.Description,
		CreatedAt:   time.Now(),
		Performance: PerformanceExpectations{
			ExpectedDurationMin: time.Duration(raw.Performance.ExpectedDurationMinMS) * time.Millisecond,
			ExpectedDurationMax: time.Duration(raw.Performance.ExpectedDurationMaxMS) * time.Millisecond,
			AcceptableDuration:  time.Duration(raw.Performance.AcceptableDurationMS) * time.Millisecond,
			TimeoutDuration:     time.Duration(raw.Performance.TimeoutDurationMS) * time.Millisecond,
			MaxRetries:          raw.Performance.MaxRetries,
			ExpectedAPIcalls:    raw.Performance.ExpectedAPICalls,
			MaxMemoryMB:         raw.Performance.MaxMemoryMB,
			ScalesWithInputSize: raw.Performance.ScalesWithInputSize,
			ScalingFactor:       raw.Performance.ScalingFactor,
		},
		Output: OutputExpectations{
			ExpectedMinSize:     raw.Output.ExpectedMinSize,
			ExpectedMaxSize:     raw.Output.ExpectedMaxSize,
			ExpectedTypicalSize: raw.Output.ExpectedTypicalSize,
			ExpectedFormat:      raw.Output.ExpectedFormat,
			ExpectsPagination:   raw.Output.ExpectsPagination,
			ExpectedPages:       raw.Output.ExpectedPages,
			RequiredFields:      raw.Output.RequiredFields,
			MustContain:         raw.Output.MustContain,
			MustNotContain:      raw.Output.MustNotContain,
			CompletenessCheck:   raw.Output.CompletenessCheck,
		},
		UsagePattern: UsagePattern{
			Frequency:         UsageFrequency(raw.UsagePattern.Frequency),
			CallsPerSession:   raw.UsagePattern.CallsPerSession,
			IsIdempotent:      raw.UsagePattern.IsIdempotent,
			HasSideEffects:    raw.UsagePattern.HasSideEffects,
			DependsOnExternal: raw.UsagePattern.DependsOnExternal,
		},
		Caching: CachingConfig{
			Cacheable:    raw.Caching.Cacheable,
			CacheKey:     raw.Caching.CacheKey,
			InvalidateOn: raw.Caching.InvalidateOn,
		},
	}

	// Parse cache duration
	if raw.Caching.CacheDuration != "" {
		if dur, err := time.ParseDuration(raw.Caching.CacheDuration); err == nil {
			profile.Caching.CacheDuration = dur
		}
	}

	// Convert custom dimensions
	for _, dim := range raw.CustomDimensions {
		profile.CustomDimensions = append(profile.CustomDimensions, CustomDimension{
			Name:           dim.Name,
			Description:    dim.Description,
			ExpectedValue:  dim.ExpectedValue,
			Tolerance:      dim.Tolerance,
			Weight:         dim.Weight,
			ExtractPattern: dim.ExtractPattern,
		})
	}

	return profile, nil
}
